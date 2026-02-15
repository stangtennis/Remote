package webrtc

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	// FFmpeg essentials build - small (~30MB zip, ~80MB exe) with all codecs
	ffmpegDownloadURL = "https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip"
	ffmpegExeName     = "ffmpeg.exe"
)

// EnsureFFmpeg checks if FFmpeg exists, downloads if not (Windows only)
// Returns the path to ffmpeg.exe
func EnsureFFmpeg() (string, error) {
	// Get target directory (next to controller.exe)
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}
	exeDir := filepath.Dir(exePath)
	targetPath := filepath.Join(exeDir, ffmpegExeName)

	// Check if already exists next to controller
	if _, err := os.Stat(targetPath); err == nil {
		return targetPath, nil
	}

	// Download FFmpeg
	log.Println("🎬 FFmpeg not found - downloading FFmpeg essentials...")
	log.Println("   This is a one-time download for H.264 video decoding")

	zipPath := filepath.Join(exeDir, "ffmpeg-download.zip")
	defer os.Remove(zipPath) // Clean up zip after extraction

	// Download zip
	resp, err := http.Get(ffmpegDownloadURL)
	if err != nil {
		return "", fmt.Errorf("failed to download FFmpeg: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download FFmpeg: HTTP %d", resp.StatusCode)
	}

	outFile, err := os.Create(zipPath)
	if err != nil {
		return "", fmt.Errorf("failed to create download file: %w", err)
	}

	written, err := io.Copy(outFile, resp.Body)
	outFile.Close()
	if err != nil {
		os.Remove(zipPath)
		return "", fmt.Errorf("failed to save FFmpeg download: %w", err)
	}

	log.Printf("🎬 Downloaded FFmpeg (%.1f MB) - extracting...", float64(written)/1024/1024)

	// Extract only ffmpeg.exe from the zip
	if err := extractFFmpegFromZip(zipPath, targetPath); err != nil {
		os.Remove(zipPath)
		return "", fmt.Errorf("failed to extract FFmpeg: %w", err)
	}

	log.Printf("🎬 FFmpeg installed: %s", targetPath)
	return targetPath, nil
}

// extractFFmpegFromZip extracts only ffmpeg.exe from the zip archive
func extractFFmpegFromZip(zipPath, targetPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		// Look for ffmpeg.exe in any subdirectory
		name := filepath.Base(f.Name)
		if !strings.EqualFold(name, ffmpegExeName) {
			continue
		}

		// Found ffmpeg.exe
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("failed to open ffmpeg.exe in zip: %w", err)
		}
		defer rc.Close()

		outFile, err := os.Create(targetPath)
		if err != nil {
			return fmt.Errorf("failed to create ffmpeg.exe: %w", err)
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		if err != nil {
			os.Remove(targetPath)
			return fmt.Errorf("failed to extract ffmpeg.exe: %w", err)
		}

		return nil
	}

	return fmt.Errorf("ffmpeg.exe not found in zip archive")
}
