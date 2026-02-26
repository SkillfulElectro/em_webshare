package main

import (
	"em_webshare/core"
	"fmt"
	"io"
	"net"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
)

type uiWriter struct {
	logText binding.String
	scroll  *container.Scroll
}

func (w *uiWriter) Write(p []byte) (n int, err error) {
	current, _ := w.logText.Get()
	w.logText.Set(current + string(p))
	w.scroll.ScrollToBottom()
	return len(p), nil
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "unknown"
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "unknown"
}

func main() {
	myApp := app.New()
	myWindow := myApp.NewWindow("EM WebShare Mobile")

	logText := binding.NewString()
	logText.Set("EM WebShare Mobile starting...\n")
	logLabel := widget.NewLabelWithData(logText)
	logLabel.Wrapping = fyne.TextWrapBreak
	logScroll := container.NewVScroll(logLabel)

	writer := &uiWriter{logText: logText, scroll: logScroll}

	// Redirect stdout/stderr to UI
	r, w, _ := os.Pipe()
	os.Stdout = w
	os.Stderr = w
	go io.Copy(writer, r)

	// Use app-specific storage for Android compatibility
	storagePath := ""
	if myApp.Storage().RootURI() != nil {
		storagePath = myApp.Storage().RootURI().Path()
	}
	core.Init(storagePath)

	port := core.FindAvailablePort()
	if port == -1 {
		fmt.Fprintln(writer, "No available ports.")
	} else {
		go core.StartServer(port)
		ip := getLocalIP()
		fmt.Fprintf(writer, "Web Interface: http://%s:%d\n", ip, port)
	}

	commandEntry := widget.NewEntry()
	commandEntry.SetPlaceHolder("Enter command (e.g. upload /sdcard/...)")
	commandEntry.OnSubmitted = func(s string) {
		if s != "" {
			fmt.Fprintf(writer, "> %s\n", s)
			core.HandleCommand(s, writer)
			commandEntry.SetText("")
		}
	}

	sendButton := widget.NewButton("Send", func() {
		commandEntry.OnSubmitted(commandEntry.Text)
	})

	inputContainer := container.NewBorder(nil, nil, nil, sendButton, commandEntry)

	content := container.NewBorder(nil, inputContainer, nil, nil, logScroll)
	myWindow.SetContent(content)
	myWindow.Resize(fyne.NewSize(400, 600))
	myWindow.ShowAndRun()
}
