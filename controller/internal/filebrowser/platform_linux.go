//go:build linux

package filebrowser

import "os"

const defaultRemotePath = "/"

func getLocalDrives() []string {
	drives := []string{"/"}
	if home, err := os.UserHomeDir(); err == nil {
		drives = append(drives, home)
	}
	return drives
}
