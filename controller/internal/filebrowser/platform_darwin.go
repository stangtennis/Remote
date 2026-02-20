//go:build darwin

package filebrowser

import (
	"os"
	"path/filepath"
)

const defaultRemotePath = "/"

func getLocalDrives() []string {
	drives := []string{"/"}
	// Add mounted volumes
	entries, err := os.ReadDir("/Volumes")
	if err == nil {
		for _, e := range entries {
			drives = append(drives, filepath.Join("/Volumes", e.Name()))
		}
	}
	// Add home directory
	if home, err := os.UserHomeDir(); err == nil {
		drives = append(drives, home)
	}
	return drives
}
