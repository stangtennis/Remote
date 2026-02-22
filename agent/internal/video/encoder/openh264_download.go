package encoder

import (
	"compress/bzip2"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
)

const (
	// OpenH264 version (v2.1.1 is latest available on GitHub)
	openH264Version = "2.1.1"
)

// Platform-specific OpenH264 download URLs and library names
func openH264Config() (url, libName string) {
	switch runtime.GOOS {
	case "darwin":
		return "https://github.com/cisco/openh264/releases/download/v2.1.1/libopenh264-2.1.1-osx64.6.dylib.bz2",
			"libopenh264-2.1.1-osx64.6.dylib"
	case "linux":
		return "https://github.com/cisco/openh264/releases/download/v2.1.1/libopenh264-2.1.1-linux64.6.so.bz2",
			"libopenh264-2.1.1-linux64.6.so"
	default: // windows
		return "https://github.com/cisco/openh264/releases/download/v2.1.1/openh264-2.1.1-win64.dll.bz2",
			"openh264-2.1.1-win64.dll"
	}
}

// EnsureOpenH264DLL checks if OpenH264 library exists, downloads if not
func EnsureOpenH264DLL() (string, error) {
	openH264URL, openH264LibName := openH264Config()

	// Get executable directory
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}
	exeDir := filepath.Dir(exePath)

	// Prefer placing the library next to the executable (best for portability),
	// but fall back to a writable per-machine/per-user location if exeDir isn't writable.
	primaryPath := filepath.Join(exeDir, openH264LibName)
	paths := []string{primaryPath}
	if cacheDir, err := os.UserCacheDir(); err == nil && cacheDir != "" {
		paths = append(paths, filepath.Join(cacheDir, "RemoteDesktopAgent", "openh264", openH264LibName))
	}
	if programData := os.Getenv("PROGRAMDATA"); programData != "" {
		paths = append(paths, filepath.Join(programData, "RemoteDesktopAgent", "openh264", openH264LibName))
	}

	// Check if DLL already exists in any candidate location
	for _, dllPath := range paths {
		if _, err := os.Stat(dllPath); err == nil {
			log.Printf("🎬 OpenH264 DLL found: %s", dllPath)
			return dllPath, nil
		}
	}

	// Pick a target path we can write to
	var dllPath string
	for _, p := range paths {
		dir := filepath.Dir(p)
		if err := os.MkdirAll(dir, 0700); err != nil {
			continue
		}
		// Try creating a temp file in the directory to ensure it's writable.
		f, err := os.CreateTemp(dir, "openh264-write-test-*")
		if err != nil {
			continue
		}
		f.Close()
		_ = os.Remove(f.Name())
		dllPath = p
		break
	}
	if dllPath == "" {
		return "", fmt.Errorf("no writable location found for OpenH264 DLL (tried: %v)", paths)
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
