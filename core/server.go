package core

import (
	"archive/tar"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

var UploadDir string
var workingDir string

func getDownloadDir() string {
	if runtime.GOOS == "android" {
		return "/sdcard/Download"
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Downloads")
}
var UploadDir_tex sync.Mutex

//go:embed statics/index.html
var index_html string

//go:embed statics/scripts.js
var scripts_js string

//go:embed statics/styles.css
var styles_css string

var DownloadQueue []string

var Mu sync.Mutex

func Init(baseDir string) {
	UploadDir_tex.Lock()
	defer UploadDir_tex.Unlock()

	dlDir := getDownloadDir()
	if dlDir != "" {
		UploadDir = filepath.Join(dlDir, "em_webshare")
	} else {
		if baseDir == "" {
			execPath, err := os.Executable()
			if err != nil {
				fmt.Println("Error getting executable path:", err)
				return
			}
			baseDir = filepath.Dir(execPath)
		}
		UploadDir = filepath.Join(baseDir, "uploads")
	}

	if err := os.MkdirAll(UploadDir, os.ModePerm); err != nil {
		fmt.Println("Error creating upload directory:", err)
	}

	if runtime.GOOS == "android" {
		workingDir = "/sdcard/Download"
	} else {
		workingDir, _ = os.Getwd()
	}
}

type Response struct {
	Message       string `json:"message,omitempty"`
	Filename      string `json:"filename,omitempty"`
	FileAvailable bool   `json:"fileAvailable,omitempty"`
	File          string `json:"file,omitempty"`
}

func FindAvailablePort() int {
	for port := 8000; port <= 60000; port++ {
		address := fmt.Sprintf("0.0.0.0:%d", port)
		ln, err := net.Listen("tcp", address)
		if err == nil {
			ln.Close()
			return port
		}
	}
	return -1
}

func ServeStaticFiles() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, index_html)
	})

	http.HandleFunc("/styles.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		fmt.Fprint(w, styles_css)
	})

	http.HandleFunc("/scripts.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		fmt.Fprint(w, scripts_js)
	})
}

func UploadChunkHandler(w http.ResponseWriter, r *http.Request) {
	UploadDir_tex.Lock()
	defer UploadDir_tex.Unlock()

	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseMultipartForm(20 << 20) // 20 MB
	if err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	fileName := r.FormValue("filename")
	chunkNumberStr := r.FormValue("chunkNumber")
	totalChunksStr := r.FormValue("totalChunks")

	if fileName == "" || chunkNumberStr == "" || totalChunksStr == "" {
		http.Error(w, "Missing metadata", http.StatusBadRequest)
		return
	}

	chunkNumber, err := strconv.Atoi(chunkNumberStr)
	if err != nil || chunkNumber < 1 {
		http.Error(w, "Invalid chunk number", http.StatusBadRequest)
		return
	}

	totalChunks, err := strconv.Atoi(totalChunksStr)
	if err != nil || totalChunks < 1 {
		http.Error(w, "Invalid total chunks", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Unable to retrieve chunk file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	chunkDir := filepath.Join(UploadDir, fileName+"_chunks")
	if err := os.MkdirAll(chunkDir, os.ModePerm); err != nil {
		http.Error(w, "Error creating chunk directory", http.StatusInternalServerError)
		return
	}

	chunkPath := filepath.Join(chunkDir, fmt.Sprintf("chunk_%d", chunkNumber))
	dst, err := os.Create(chunkPath)
	if err != nil {
		http.Error(w, "Error creating chunk file", http.StatusInternalServerError)
		return
	}

	_, err = io.Copy(dst, file)
	if err != nil {
		http.Error(w, "Error saving chunk", http.StatusInternalServerError)
		return
	}

	dst.Close()

	if allChunksUploaded(chunkDir, totalChunks) {
		err := mergeChunks(chunkDir, filepath.Join(UploadDir, fileName), totalChunks)
		if err != nil {
			http.Error(w, "Error merging chunks", http.StatusInternalServerError)
			return
		}

		os.RemoveAll(chunkDir)
	}

	response := Response{
		Message: fmt.Sprintf("Chunk %d uploaded successfully", chunkNumber),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func allChunksUploaded(dir string, totalChunks int) bool {
	files, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	return len(files) == totalChunks
}

func mergeChunks(chunkDir, outputFile string, totalChunks int) error {
	dst, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer dst.Close()

	for i := 1; i <= totalChunks; i++ {
		chunkPath := filepath.Join(chunkDir, fmt.Sprintf("chunk_%d", i))
		src, err := os.Open(chunkPath)
		if err != nil {
			return err
		}

		_, err = io.Copy(dst, src)
		src.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func CheckFileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	Mu.Lock()
	defer Mu.Unlock()

	if len(DownloadQueue) > 0 {
		filePath := DownloadQueue[0]
		response := Response{
			FileAvailable: true,
			File:          filePath,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	} else {
		response := Response{
			FileAvailable: false,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

func DownloadFileHandler(w http.ResponseWriter, r *http.Request) {
	UploadDir_tex.Lock()
	defer UploadDir_tex.Unlock()

	if r.Method != http.MethodGet {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	Mu.Lock()
	if len(DownloadQueue) == 0 {
		Mu.Unlock()
		http.Error(w, "No file available for download", http.StatusNotFound)
		return
	}

	dirPath := DownloadQueue[0]
	DownloadQueue = DownloadQueue[1:]
	Mu.Unlock()

	info, err := os.Stat(dirPath)
	if err != nil {
		http.Error(w, "File or directory does not exist", http.StatusNotFound)
		return
	}

	if info.IsDir() {
		streamAsTar(w, dirPath)
	} else {
		serveFile(w, dirPath)
	}
}

func serveFile(w http.ResponseWriter, filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "Error opening file", http.StatusInternalServerError)
		fmt.Printf("Error opening file %s: %v\n", filePath, err)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(filePath))
	w.Header().Set("Content-Type", "application/octet-stream")

	if _, err := io.Copy(w, file); err != nil {
		fmt.Printf("Error sending file %s: %v\n", filePath, err)
	}
}

func streamAsTar(w http.ResponseWriter, dirPath string) {
	w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(dirPath)+".tar")
	w.Header().Set("Content-Type", "application/x-tar")

	tarWriter := tar.NewWriter(w)
	defer tarWriter.Close()

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("Error walking path %s: %v\n", path, err)
			return err
		}

		if path == dirPath {
			return nil
		}

		relPath, err := filepath.Rel(dirPath, path)
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, path)
		if err != nil {
			return err
		}
		header.Name = relPath

		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		if !info.IsDir() {
			return writeFileToTar(tarWriter, path)
		}

		return nil
	})
	if err != nil {
		fmt.Printf("Error streaming directory %s: %v\n", dirPath, err)
	}
}

func writeFileToTar(tarWriter *tar.Writer, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(tarWriter, file)
	return err
}

func UploadFileHandler(w http.ResponseWriter, r *http.Request) {
	UploadDir_tex.Lock()
	defer UploadDir_tex.Unlock()

	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		http.Error(w, "Unable to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Unable to retrieve file: "+err.Error(), http.StatusBadRequest)
		fmt.Println("Unable to retrieve file: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	originalFileName := fileHeader.Filename
	relativePath := r.FormValue("relativePath")

	var filePath string
	if len(relativePath) == 0 {
		filePath = filepath.Join(UploadDir, originalFileName)
	} else {
		// Security: Sanitize relativePath to prevent path traversal
		cleanRelPath := filepath.Clean(filepath.FromSlash(relativePath))
		if strings.HasPrefix(cleanRelPath, "..") || filepath.IsAbs(cleanRelPath) {
			http.Error(w, "Invalid relative path", http.StatusBadRequest)
			return
		}
		filePath = filepath.Join(UploadDir, cleanRelPath)
	}

	// Ensure the final path is still within UploadDir
	absUploadDir, _ := filepath.Abs(UploadDir)
	absFilePath, _ := filepath.Abs(filePath)
	if !strings.HasPrefix(absFilePath, absUploadDir) {
		http.Error(w, "Forbidden path", http.StatusForbidden)
		return
	}

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		http.Error(w, "Unable to create directory: "+err.Error(), http.StatusInternalServerError)
		return
	}

	dst, err := os.Create(filePath)
	if err != nil {
		http.Error(w, "Unable to create file on server: "+err.Error(), http.StatusInternalServerError)
		fmt.Println("Unable to create file on server: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		http.Error(w, "Error saving file: "+err.Error(), http.StatusInternalServerError)
		fmt.Println("Error saving file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"message": "File uploaded successfully", "filename": "%s"}`, originalFileName)
}

func StartServer(port int) {
	ServeStaticFiles()

	http.HandleFunc("/upload-chunk", UploadChunkHandler)
	http.HandleFunc("/check-file", CheckFileHandler)
	http.HandleFunc("/download", DownloadFileHandler)

	http.HandleFunc("/upload", UploadFileHandler)

	address := fmt.Sprintf("0.0.0.0:%d", port)
	fmt.Printf("EM WebShare is ready on port %d\n", port)
	if err := http.ListenAndServe(address, nil); err != nil {
		fmt.Println("Error starting server:", err)
	}
}

func HandleCommand(command string, out io.Writer) bool {
	command = strings.TrimSpace(command)

	if len(command) == 0 {
		return true
	}

	if command == "exit" {
		fmt.Fprintln(out, "Exiting...")
		return false
	}

	parts := strings.Fields(command)

	if len(parts) < 1 {
		fmt.Fprintln(out, "Invalid command. Please enter a valid command.")
		return true
	}

	switch parts[0] {
	case "pwd":
		fmt.Fprintln(out, workingDir)

	case "cd":
		if len(parts) < 2 {
			fmt.Fprintln(out, "Please provide a path.")
			return true
		}
		newDir := strings.Join(parts[1:], " ")
		if !filepath.IsAbs(newDir) {
			newDir = filepath.Join(workingDir, newDir)
		}
		if info, err := os.Stat(newDir); err == nil && info.IsDir() {
			workingDir = newDir
			fmt.Fprintf(out, "Changed directory to %s\n", workingDir)
		} else {
			fmt.Fprintf(out, "Directory %s not found.\n", newDir)
		}

	case "ls":
		dir := workingDir
		if len(parts) > 1 {
			dir = strings.Join(parts[1:], " ")
			if !filepath.IsAbs(dir) {
				dir = filepath.Join(workingDir, dir)
			}
		}
		files, err := os.ReadDir(dir)
		if err != nil {
			fmt.Fprintf(out, "Error reading directory: %v\n", err)
			return true
		}
		for _, f := range files {
			suffix := ""
			if f.IsDir() {
				suffix = "/"
			}
			fmt.Fprintf(out, "%s%s\n", f.Name(), suffix)
		}

	case "up_dir":
		if len(parts) < 2 {
			fmt.Fprintln(out, "Please provide the path to directory for saving client uploads after up_dir command !")
			return true
		}

		path := strings.Join(parts[1:], " ")
		if !filepath.IsAbs(path) {
			path = filepath.Join(workingDir, path)
		}

		fmt.Fprintln(out, "Changing the Upload dir ... (it might take while)")
		UploadDir_tex.Lock()
		UploadDir = path
		UploadDir_tex.Unlock()
		fmt.Fprintf(out, "Changed Upload directory to %s\n", UploadDir)

	case "upload":
		if len(parts) < 2 {
			fmt.Fprintln(out, "Please provide the file path to upload.")
			return true
		}

		filePath := strings.Join(parts[1:], " ")

		if !filepath.IsAbs(filePath) {
			// Try relative to workingDir
			tempPath := filepath.Join(workingDir, filePath)
			if _, err := os.Stat(tempPath); err == nil {
				filePath = tempPath
			} else {
				// Try relative to Download folder if not already there
				dlDir := getDownloadDir()
				if dlDir != "" && workingDir != dlDir {
					tempPath = filepath.Join(dlDir, filePath)
					if _, err := os.Stat(tempPath); err == nil {
						filePath = tempPath
					}
				}
			}
		}

		if _, err := os.Stat(filePath); err != nil {
			fmt.Fprintf(out, "File %s not found.\n", filePath)
			return true
		}

		Mu.Lock()
		DownloadQueue = append(DownloadQueue, filePath)
		Mu.Unlock()

		fmt.Fprintf(out, "File %s added to download queue.\n", filePath)

	default:
		fmt.Fprintln(out, "Unknown command:", command)
	}
	return true
}
