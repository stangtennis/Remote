//go:build windows
// +build windows

package desktop

import (
	"fmt"
	"log"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                        = windows.NewLazySystemDLL("user32.dll")
	procGetThreadDesktop          = user32.NewProc("GetThreadDesktop")
	procGetUserObjectInformationW = user32.NewProc("GetUserObjectInformationW")
	procOpenInputDesktop          = user32.NewProc("OpenInputDesktop")
	procCloseDesktop              = user32.NewProc("CloseDesktop")
	procSwitchDesktop             = user32.NewProc("SwitchDesktop")

	kernel32                         = windows.NewLazySystemDLL("kernel32.dll")
	procGetCurrentThreadId           = kernel32.NewProc("GetCurrentThreadId")
	procWTSGetActiveConsoleSessionId = kernel32.NewProc("WTSGetActiveConsoleSessionId")
)

const (
	UOI_NAME = 2
)

type DesktopType int

const (
	DesktopUnknown     DesktopType = iota
	DesktopDefault                 // Normal user desktop
	DesktopWinlogon                // Windows login screen
	DesktopScreenSaver             // Screen saver
)

// GetCurrentDesktop returns the name of the current desktop
func GetCurrentDesktop() (string, error) {
	threadID, _, _ := procGetCurrentThreadId.Call()
	if threadID == 0 {
		return "", fmt.Errorf("failed to get thread ID")
	}

	hDesktop, _, _ := procGetThreadDesktop.Call(threadID)
	if hDesktop == 0 {
		return "", fmt.Errorf("failed to get thread desktop")
	}

	return getDesktopName(hDesktop)
}

// GetInputDesktop returns the name of the current input desktop
func GetInputDesktop() (string, error) {
	hDesktop, _, err := procOpenInputDesktop.Call(
		0,      // dwFlags
		0,      // fInherit
		0x0001, // DESKTOP_READOBJECTS
	)
	if hDesktop == 0 {
		return "", fmt.Errorf("failed to open input desktop: %w", err)
	}
	defer procCloseDesktop.Call(hDesktop)

	return getDesktopName(hDesktop)
}

func getDesktopName(hDesktop uintptr) (string, error) {
	var nameLen uint32
	procGetUserObjectInformationW.Call(
		hDesktop,
		UOI_NAME,
		0,
		0,
		uintptr(unsafe.Pointer(&nameLen)),
	)

	if nameLen == 0 {
		return "", fmt.Errorf("failed to get desktop name length")
	}

	nameBuf := make([]uint16, nameLen)
	ret, _, err := procGetUserObjectInformationW.Call(
		hDesktop,
		UOI_NAME,
		uintptr(unsafe.Pointer(&nameBuf[0])),
		uintptr(nameLen*2),
		uintptr(unsafe.Pointer(&nameLen)),
	)

	if ret == 0 {
		return "", fmt.Errorf("failed to get desktop name: %w", err)
	}

	return syscall.UTF16ToString(nameBuf), nil
}

// GetDesktopType determines the type of desktop based on name
func GetDesktopType(desktopName string) DesktopType {
	switch desktopName {
	case "Winlogon":
		return DesktopWinlogon
	case "Screen-saver":
		return DesktopScreenSaver
	case "Default":
		return DesktopDefault
	default:
		return DesktopUnknown
	}
}

// IsOnLoginScreen checks if currently on Windows login screen
func IsOnLoginScreen() bool {
	desktop, err := GetInputDesktop()
	if err != nil {
		log.Printf("Failed to get input desktop: %v", err)
		return false
	}

	return GetDesktopType(desktop) == DesktopWinlogon
}

// GetActiveConsoleSession returns the session ID of the user logged in at the physical console.
// Returns 0xFFFFFFFF if no user is logged in at the console.
// This works from Session 0 (services) where OpenInputDesktop() fails.
func GetActiveConsoleSession() uint32 {
	ret, _, _ := procWTSGetActiveConsoleSessionId.Call()
	return uint32(ret)
}

// MonitorDesktopSwitch monitors for desktop changes
func MonitorDesktopSwitch(onChange func(DesktopType)) {
	var currentDesktop string
	// Track WTS session for Session 0 fallback (initialize to 0xFFFFFFFF = no user)
	var lastWTSSession uint32 = 0xFFFFFFFF

	for {
		desktop, err := GetInputDesktop()
		if err == nil {
			// OpenInputDesktop succeeded — normal mode
			if desktop != currentDesktop {
				currentDesktop = desktop
				desktopType := GetDesktopType(desktop)

				log.Printf("Desktop switched to: %s (type: %d)", desktop, desktopType)
				if onChange != nil {
					onChange(desktopType)
				}
			}
		} else {
			// OpenInputDesktop failed — Session 0 mode
			// Use WTSGetActiveConsoleSessionId as fallback to detect user login/logout
			sessionID := GetActiveConsoleSession()
			if sessionID != lastWTSSession {
				lastWTSSession = sessionID
				if sessionID > 0 && sessionID != 0xFFFFFFFF {
					log.Printf("👤 User session %d detected via WTS (Session 0 fallback)", sessionID)
					currentDesktop = "Default" // Sync state
					if onChange != nil {
						onChange(DesktopDefault)
					}
				} else {
					log.Println("🔒 No active user session (WTS fallback)")
					currentDesktop = "Winlogon"
					if onChange != nil {
						onChange(DesktopWinlogon)
					}
				}
			}
		}

		// Check every 2 seconds
		time.Sleep(2 * time.Second)
	}
}

var (
	procSetThreadDesktop = user32.NewProc("SetThreadDesktop")
)

// SwitchToInputDesktop switches the current thread to the input desktop
// This is required for Session 0 / login screen interaction
func SwitchToInputDesktop() error {
	// Open the input desktop with full access
	hDesktop, _, err := procOpenInputDesktop.Call(
		0,          // dwFlags
		uintptr(1), // fInherit = TRUE
		0x0001|0x0002|0x0004| // DESKTOP_READOBJECTS | DESKTOP_CREATEWINDOW | DESKTOP_CREATEMENU
			0x0008|0x0010|0x0020| // DESKTOP_HOOKCONTROL | DESKTOP_JOURNALRECORD | DESKTOP_JOURNALPLAYBACK
			0x0040|0x0080|0x0100, // DESKTOP_ENUMERATE | DESKTOP_WRITEOBJECTS | DESKTOP_SWITCHDESKTOP
	)
	if hDesktop == 0 {
		return fmt.Errorf("failed to open input desktop: %w", err)
	}

	// Set this thread's desktop to the input desktop
	ret, _, err := procSetThreadDesktop.Call(hDesktop)
	if ret == 0 {
		// SetThreadDesktop failed - this can happen if thread has windows
		// Try SwitchDesktop instead
		ret, _, err = procSwitchDesktop.Call(hDesktop)
		if ret == 0 {
			procCloseDesktop.Call(hDesktop)
			return fmt.Errorf("failed to switch desktop: %w", err)
		}
	}

	// Don't close the desktop handle - we need it for the thread
	return nil
}
