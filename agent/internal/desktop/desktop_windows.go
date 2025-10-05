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
	user32                   = windows.NewLazySystemDLL("user32.dll")
	procGetThreadDesktop     = user32.NewProc("GetThreadDesktop")
	procGetUserObjectInformationW = user32.NewProc("GetUserObjectInformationW")
	procOpenInputDesktop     = user32.NewProc("OpenInputDesktop")
	procCloseDesktop         = user32.NewProc("CloseDesktop")
	procSwitchDesktop        = user32.NewProc("SwitchDesktop")
	procGetCurrentThreadId   = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetCurrentThreadId")
)

const (
	UOI_NAME = 2
)

type DesktopType int

const (
	DesktopUnknown DesktopType = iota
	DesktopDefault              // Normal user desktop
	DesktopWinlogon             // Windows login screen
	DesktopScreenSaver          // Screen saver
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
		0,     // dwFlags
		0,     // fInherit
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

// MonitorDesktopSwitch monitors for desktop changes
func MonitorDesktopSwitch(onChange func(DesktopType)) {
	var currentDesktop string
	
	for {
		desktop, err := GetInputDesktop()
		if err == nil && desktop != currentDesktop {
			currentDesktop = desktop
			desktopType := GetDesktopType(desktop)
			
			log.Printf("Desktop switched to: %s (type: %d)", desktop, desktopType)
			if onChange != nil {
				onChange(desktopType)
			}
		}
		
		// Check every 2 seconds
		time.Sleep(2 * time.Second)
	}
}

// SwitchToInputDesktop switches the thread to the input desktop
func SwitchToInputDesktop() error {
	hDesktop, _, err := procOpenInputDesktop.Call(
		0,      // dwFlags
		0,      // fInherit
		0x01FF, // GENERIC_ALL
	)
	if hDesktop == 0 {
		return fmt.Errorf("failed to open input desktop: %w", err)
	}

	ret, _, err := procSwitchDesktop.Call(hDesktop)
	if ret == 0 {
		procCloseDesktop.Call(hDesktop)
		return fmt.Errorf("failed to switch desktop: %w", err)
	}

	return nil
}
