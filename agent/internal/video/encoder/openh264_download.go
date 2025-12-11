package encoder

import (
	"compress/bzip2"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

const (
	// OpenH264 version and download URL
	openH264Version = "2.4.1"
	openH264URL     = "https://github.com/cisco/openh264/releases/download/v2.4.1/openh264-2.4.1-win64.dll.bz2"
	openH264DLLName = "openh264-2.4.1-win64.dll"
)

// EnsureOpenH264DLL checks if OpenH264 DLL exists, downloads if not
func EnsureOpenH264DLL() (string, error) {
	// Get executable directory
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}
	exeDir := filepath.Dir(exePath)
	dllPath := filepath.Join(exeDir, openH264DLLName)

	// Check if DLL already exists
	if _, err := os.Stat(dllPath); err == nil {
		log.Printf("🎬 OpenH264 DLL found: %s", dllPath)
		return dllPath, nil
	}

	// Download DLL
	log.Printf("🎬 Downloading OpenH264 DLL v%s...", openH264Version)

	resp, err := http.Get(openH264URL)
	if err != nil {
		return "", fmt.Errorf("failed to download OpenH264: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download OpenH264: HTTP %d", resp.StatusCode)
	}

	// Decompress bz2
	bzReader := bzip2.NewReader(resp.Body)

	// Create temp file first
	tmpPath := dllPath + ".tmp"
	outFile, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("failed to create DLL file: %w", err)
	}

	written, err := io.Copy(outFile, bzReader)
	outFile.Close()
	if err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("failed to decompress OpenH264: %w", err)
	}

	// Rename to final path
	if err := os.Rename(tmpPath, dllPath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("failed to rename DLL: %w", err)
	}

	log.Printf("🎬 OpenH264 DLL downloaded: %s (%.2f MB)", dllPath, float64(written)/1024/1024)
	return dllPath, nil
}
