package main

import (
	"archive/tar"
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const uploadDir = "./uploads"
const staticDir = "./statics"

var downloadQueue []string

var mu sync.Mutex

func init() {
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		fmt.Println("Error creating upload directory:", err)
	}
}

type Response struct {
	Message       string `json:"message,omitempty"`
	Filename      string `json:"filename,omitempty"`
	FileAvailable bool   `json:"fileAvailable,omitempty"`
	File          string `json:"file,omitempty"`
}

func findAvailablePort() int {
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

func serveStaticFiles() {
	fs := http.FileServer(http.Dir(staticDir))
	http.Handle("/", fs)
}

// will be used in future updates
func uploadChunkHandler(w http.ResponseWriter, r *http.Request) {
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

	chunkDir := filepath.Join(uploadDir, fileName+"_chunks")
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
		err := mergeChunks(chunkDir, filepath.Join(uploadDir, fileName), totalChunks)
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

func checkFileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if len(downloadQueue) > 0 {
		filePath := downloadQueue[0]
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

func downloadFileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	mu.Lock()
	if len(downloadQueue) == 0 {
		mu.Unlock()
		http.Error(w, "No file available for download", http.StatusNotFound)
		return
	}

	dirPath := downloadQueue[0]
	downloadQueue = downloadQueue[1:]
	mu.Unlock()

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

func uploadFileHandler(w http.ResponseWriter, r *http.Request) {
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
		filePath = filepath.Join(uploadDir, originalFileName)
	} else {
		filePath = filepath.Join(uploadDir, relativePath)
	}

	// fmt.Println(filePath)

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

func startServer(port int) {
	serveStaticFiles()

	http.HandleFunc("/upload-chunk", uploadChunkHandler)
	http.HandleFunc("/check-file", checkFileHandler)
	http.HandleFunc("/download", downloadFileHandler)

	http.HandleFunc("/upload", uploadFileHandler)

	address := fmt.Sprintf("0.0.0.0:%d", port)
	fmt.Printf("EM WebShare is ready on port %d\n", port)
	if err := http.ListenAndServe(address, nil); err != nil {
		fmt.Println("Error starting server:", err)
	}
}

func handleCLICommands() {
	for {
		fmt.Println("Enter command (e.g., 'upload /file/path' , 'upload /dir/path'): (If dir , press download multiple times in web side)")

		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()

		command := scanner.Text()

		if err := scanner.Err(); err != nil {
			fmt.Println("Error reading command:", err)
			continue
		}

		command = strings.TrimSpace(command)

		if len(command) == 0 {
			continue
		}

		if command == "exit" {
			fmt.Println("Exiting...")
			return
		}

		parts := strings.Fields(command)

		if len(parts) < 1 {
			fmt.Println("Invalid command. Please enter a valid command.")
			continue
		}

		switch parts[0] {
		case "upload":
			if len(parts) < 2 {
				fmt.Println("Please provide the file path to upload.")
				continue
			}

			filePath := strings.Join(parts[1:], " ")

			if _, err := os.Stat(filePath); err != nil {
				fmt.Printf("File %s not found.\n", filePath)
				return
			}

			mu.Lock()
			downloadQueue = append(downloadQueue, filePath)
			mu.Unlock()

			fmt.Printf("File %s added to download queue.\n", filePath)

		default:
			fmt.Println("Unknown command:", command)
		}
	}
}

func main() {
	fmt.Println("EM WebShare : Simple Web Based file sharing app")
	fmt.Print("contribute : https://github.com/SkillfulElectro/em_webshare.git\n\n")

	port := findAvailablePort()
	if port == -1 {
		fmt.Println("No available ports in the range 8000-60000.")
		return
	}

	go startServer(port)

	handleCLICommands()
}
