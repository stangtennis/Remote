package updater

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	shell32          = syscall.NewLazyDLL("shell32.dll")
	procShellExecute = shell32.NewProc("ShellExecuteW")
)

// launchElevated launches an executable with admin privileges using ShellExecuteW "runas"
func launchElevated(exe string, args string) error {
	verb, _ := syscall.UTF16PtrFromString("runas")
	exePath, _ := syscall.UTF16PtrFromString(exe)
	argsPtr, _ := syscall.UTF16PtrFromString(args)

	ret, _, _ := procShellExecute.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(exePath)),
		uintptr(unsafe.Pointer(argsPtr)),
		0,
		1, // SW_SHOWNORMAL
	)

	// ShellExecute returns > 32 on success
	if ret <= 32 {
		return fmt.Errorf("ShellExecuteW failed with code %d", ret)
	}

	return nil
}
