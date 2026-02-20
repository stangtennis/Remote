//go:build windows

package filebrowser

import "os"

const defaultRemotePath = "C:\\"

func getLocalDrives() []string {
	drives := []string{}
	for _, letter := range "CDEFGH" {
		drive := string(letter) + ":\\"
		if _, err := os.Stat(drive); err == nil {
			drives = append(drives, drive)
		}
	}
	if len(drives) == 0 {
		drives = []string{"C:\\"}
	}
	return drives
}
