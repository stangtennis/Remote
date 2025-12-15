package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// DownloadProgress represents download progress
type DownloadProgress struct {
	TotalBytes      int64
	DownloadedBytes int64
	Percent         float64
}

// Downloader handles file downloads with progress tracking
type Downloader struct {
	httpClient *http.Client
	userAgent  string
	onProgress func(DownloadProgress)
}

// NewDownloader creates a new downloader
func NewDownloader() *Downloader {
	return &Downloader{
		httpClient: &http.Client{
			Timeout: 10 * time.Minute,
		},
		userAgent: "RemoteDesktop-Agent-Updater/1.0",
	}
}

// SetProgressCallback sets the progress callback
func (d *Downloader) SetProgressCallback(callback func(DownloadProgress)) {
	d.onProgress = callback
}

// DownloadFile downloads a file to the specified path
func (d *Downloader) DownloadFile(url string, destPath string, expectedSize int64) error {
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	tempPath := destPath + ".tmp"
	file, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer file.Close()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		os.Remove(tempPath)
		return err
	}
	req.Header.Set("User-Agent", d.userAgent)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		os.Remove(tempPath)
		return fmt.Errorf("download failed: status %d", resp.StatusCode)
	}

	totalSize := resp.ContentLength
	if totalSize <= 0 && expectedSize > 0 {
		totalSize = expectedSize
	}

	var downloaded int64
	buf := make([]byte, 32*1024)

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			_, writeErr := file.Write(buf[:n])
			if writeErr != nil {
				os.Remove(tempPath)
				return fmt.Errorf("failed to write file: %w", writeErr)
			}
			downloaded += int64(n)

			if d.onProgress != nil && totalSize > 0 {
				d.onProgress(DownloadProgress{
					TotalBytes:      totalSize,
					DownloadedBytes: downloaded,
					Percent:         float64(downloaded) / float64(totalSize) * 100,
				})
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			os.Remove(tempPath)
			return fmt.Errorf("download error: %w", err)
		}
	}

	file.Close()

	if err := os.Rename(tempPath, destPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to rename file: %w", err)
	}

	log.Printf("✅ Downloaded %d bytes to %s", downloaded, destPath)
	return nil
}

// VerifySHA256 verifies a file's SHA256 checksum
func VerifySHA256(filePath string, expectedHash string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return fmt.Errorf("failed to hash file: %w", err)
	}

	actualHash := hex.EncodeToString(hasher.Sum(nil))
	if actualHash != expectedHash {
		return fmt.Errorf("SHA256 mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	log.Printf("✅ SHA256 verified: %s", actualHash[:16]+"...")
	return nil
}

// GetUpdateDirectory returns the directory for storing updates
func GetUpdateDirectory() (string, error) {
	// Use %PROGRAMDATA%\RemoteDesktopAgent\updates for service-friendly access
	programData := os.Getenv("PROGRAMDATA")
	if programData != "" {
		updateDir := filepath.Join(programData, "RemoteDesktopAgent", "updates")
		if err := os.MkdirAll(updateDir, 0755); err == nil {
			return updateDir, nil
		}
	}

	// Fallback to exe directory
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	updateDir := filepath.Join(filepath.Dir(exePath), "updates")
	if err := os.MkdirAll(updateDir, 0755); err != nil {
		return "", err
	}

	return updateDir, nil
}
