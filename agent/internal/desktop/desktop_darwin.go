//go:build darwin

package desktop

import (
	"log"
	"time"
)

type DesktopType int

const (
	DesktopUnknown     DesktopType = iota
	DesktopDefault                 // Normal user desktop
	DesktopWinlogon                // Windows login screen (N/A on macOS)
	DesktopScreenSaver             // Screen saver
)

// GetCurrentDesktop returns the name of the current desktop.
// macOS does not have separate desktops like Windows (Winlogon, Default, etc.)
func GetCurrentDesktop() (string, error) {
	return "Default", nil
}

// GetInputDesktop returns the name of the current input desktop.
func GetInputDesktop() (string, error) {
	return "Default", nil
}

// GetDesktopType determines the type of desktop based on name.
func GetDesktopType(desktopName string) DesktopType {
	switch desktopName {
	case "Default":
		return DesktopDefault
	case "ScreenSaver":
		return DesktopScreenSaver
	default:
		return DesktopUnknown
	}
}

// IsOnLoginScreen checks if currently on login screen.
// macOS login screen detection would use CGSessionCopyCurrentDictionary.
func IsOnLoginScreen() bool {
	// TODO: Implementer via CGSessionCopyCurrentDictionary
	// Tjek om "CGSSessionScreenIsLocked" key er true
	return false
}

// MonitorDesktopSwitch monitors for desktop changes.
// On macOS this monitors for screen lock/unlock events.
func MonitorDesktopSwitch(onChange func(DesktopType)) {
	// macOS har ikke desktop switching som Windows.
	// Kører som no-op polling loop for kompatibilitet.
	var currentDesktop string = "Default"
	_ = currentDesktop

	for {
		time.Sleep(2 * time.Second)
		// TODO: Brug NSDistributedNotificationCenter til at lytte efter
		// "com.apple.screenIsLocked" og "com.apple.screenIsUnlocked"
	}
}

// SwitchToInputDesktop switches the current thread to the input desktop.
// No-op on macOS since there are no separate desktops.
func SwitchToInputDesktop() error {
	log.Println("SwitchToInputDesktop: no-op on macOS")
	return nil
}
