package core

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGetDownloadDir(t *testing.T) {
	dir := getDownloadDir()
	if runtime.GOOS == "android" {
		if dir != "/sdcard/Download" {
			t.Errorf("Expected /sdcard/Download, got %s", dir)
		}
	} else {
		home, _ := os.UserHomeDir()
		expected := filepath.Join(home, "Downloads")
		if dir != expected {
			t.Errorf("Expected %s, got %s", expected, dir)
		}
	}
}

func TestHandleCommand(t *testing.T) {
	// Setup
	Init("")

	var out bytes.Buffer

	// Test pwd
	out.Reset()
	HandleCommand("pwd", &out)
	if !strings.Contains(out.String(), workingDir) {
		t.Errorf("pwd output should contain %s, got %s", workingDir, out.String())
	}

	// Test cd
	tempDir, _ := os.MkdirTemp("", "test_cd")
	defer os.RemoveAll(tempDir)

	out.Reset()
	HandleCommand("cd "+tempDir, &out)
	if !strings.Contains(out.String(), "Changed directory to") {
		t.Errorf("cd output should indicate success, got %s", out.String())
	}
	if workingDir != tempDir {
		t.Errorf("workingDir should be %s, got %s", tempDir, workingDir)
	}

	// Test ls
	out.Reset()
	HandleCommand("ls", &out)
	// Just check if it doesn't error out
	if strings.Contains(out.String(), "Error") {
		t.Errorf("ls should not return error, got %s", out.String())
	}

	// Test upload (failure case)
	out.Reset()
	HandleCommand("upload non_existent_file_xyz", &out)
	if !strings.Contains(out.String(), "not found") {
		t.Errorf("upload should report file not found, got %s", out.String())
	}
}
