//go:build windows

package main

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"
)

const (
	wmCommand = 0x0111
	wmClose   = 0x0010
	wmDestroy = 0x0002
	wmApp     = 0x8000

	wsOverlapped = 0x00000000
	wsCaption    = 0x00C00000
	wsSysMenu    = 0x00080000
	wsMinimize   = 0x00020000
	wsVisible    = 0x10000000
	wsChild      = 0x40000000

	ssLeft      = 0x00000000
	editStyle   = 0x0001 | 0x0004 | 0x0080
	buttonStyle = 0x00000000
	colorWindow = 5

	supportCloseButtonID = 1001
)

var (
	supportUser32   = syscall.NewLazyDLL("user32.dll")
	supportKernel32 = syscall.NewLazyDLL("kernel32.dll")

	supportRegisterClassEx  = supportUser32.NewProc("RegisterClassExW")
	supportCreateWindowEx   = supportUser32.NewProc("CreateWindowExW")
	supportDefWindowProc    = supportUser32.NewProc("DefWindowProcW")
	supportDestroyWindow    = supportUser32.NewProc("DestroyWindow")
	supportShowWindow       = supportUser32.NewProc("ShowWindow")
	supportUpdateWindow     = supportUser32.NewProc("UpdateWindow")
	supportGetMessage       = supportUser32.NewProc("GetMessageW")
	supportTranslateMessage = supportUser32.NewProc("TranslateMessage")
	supportDispatchMessage  = supportUser32.NewProc("DispatchMessageW")
	supportPostQuitMessage  = supportUser32.NewProc("PostQuitMessage")
	supportPostMessage      = supportUser32.NewProc("PostMessageW")
	supportSetWindowText    = supportUser32.NewProc("SetWindowTextW")
	supportGetModuleHandle  = supportKernel32.NewProc("GetModuleHandleW")
	supportCreateMutex      = supportKernel32.NewProc("CreateMutexW")
	supportCloseHandle      = supportKernel32.NewProc("CloseHandle")
	supportGetLastError     = supportKernel32.NewProc("GetLastError")

	supportWindowProc   = syscall.NewCallback(supportWindowWndProc)
	activeSupportWindow *supportWindow
)

const errorAlreadyExists = 183

var portableSupportMutex uintptr

type supportWindow struct {
	hwnd     uintptr
	status   uintptr
	stop     chan struct{}
	stopOnce sync.Once
}

// acquirePortableSupportMutex prevents duplicate PIN dialogs and support
// sessions when the portable EXE is launched twice or restarted through UAC.
func acquirePortableSupportMutex() bool {
	name := supportUTF16("Global\\RemoteDesktopPortableSupport")
	handle, _, _ := supportCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if handle == 0 {
		return true
	}
	portableSupportMutex = handle
	lastError, _, _ := supportGetLastError.Call()
	if lastError == errorAlreadyExists {
		supportCloseHandle.Call(handle)
		portableSupportMutex = 0
		return false
	}
	return true
}

func releasePortableSupportMutex() {
	if portableSupportMutex != 0 {
		supportCloseHandle.Call(portableSupportMutex)
		portableSupportMutex = 0
	}
}

type supportWndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type supportPoint struct {
	x int32
	y int32
}

type supportMsg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      supportPoint
	private uint32
}

func supportUTF16(value string) *uint16 {
	ptr, _ := syscall.UTF16PtrFromString(value)
	return ptr
}

func createSupportControl(className, title string, style, exStyle uint32, x, y, width, height int32, parent, menu, instance uintptr) uintptr {
	hwnd, _, _ := supportCreateWindowEx.Call(
		uintptr(exStyle),
		uintptr(unsafe.Pointer(supportUTF16(className))),
		uintptr(unsafe.Pointer(supportUTF16(title))),
		uintptr(style),
		uintptr(x), uintptr(y), uintptr(width), uintptr(height),
		parent, menu, instance, 0,
	)
	return hwnd
}

func newSupportWindow(stop chan struct{}) (*supportWindow, error) {
	instance, _, _ := supportGetModuleHandle.Call(0)
	className := supportUTF16("RemoteDesktopPortableSupportWindow")
	class := supportWndClassEx{
		cbSize:        uint32(unsafe.Sizeof(supportWndClassEx{})),
		lpfnWndProc:   supportWindowProc,
		hInstance:     instance,
		hbrBackground: colorWindow + 1,
		lpszClassName: className,
	}
	// RegisterClassEx returns zero when the class already exists; that is safe
	// because only one portable support window is allowed per process.
	supportRegisterClassEx.Call(uintptr(unsafe.Pointer(&class)))

	hwnd, _, _ := supportCreateWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(supportUTF16("Remote Desktop AI Support"))),
		wsOverlapped|wsCaption|wsSysMenu|wsMinimize,
		uintptr(200), uintptr(160), uintptr(500), uintptr(250),
		0, 0, instance, 0,
	)
	if hwnd == 0 {
		return nil, fmt.Errorf("could not create support window")
	}

	createSupportControl("STATIC", "Denne computer er midlertidigt delt med AI-support.", wsVisible|wsChild|ssLeft, 0, 24, 24, 440, 32, hwnd, 0, instance)
	status := createSupportControl("STATIC", "Starter support...", wsVisible|wsChild|ssLeft, 0, 24, 70, 440, 48, hwnd, 0, instance)
	createSupportControl("STATIC", "Luk vinduet for at afslutte sessionen.", wsVisible|wsChild|ssLeft, 0, 24, 130, 440, 28, hwnd, 0, instance)
	createSupportControl("BUTTON", "Afslut support", wsVisible|wsChild|buttonStyle, 0, 160, 170, 180, 34, hwnd, supportCloseButtonID, instance)

	window := &supportWindow{hwnd: hwnd, status: status, stop: stop}
	activeSupportWindow = window
	supportShowWindow.Call(hwnd, 5)
	supportUpdateWindow.Call(hwnd)
	return window, nil
}

func (w *supportWindow) setStatus(message string) {
	if w == nil || w.status == 0 {
		return
	}
	supportSetWindowText.Call(w.status, uintptr(unsafe.Pointer(supportUTF16(message))))
}

func (w *supportWindow) close() {
	if w == nil {
		return
	}
	w.stopOnce.Do(func() { close(w.stop) })
	supportPostMessage.Call(w.hwnd, wmClose, 0, 0)
}

func (w *supportWindow) run() {
	var msg supportMsg
	for {
		result, _, _ := supportGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(result) <= 0 {
			break
		}
		supportTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		supportDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
	activeSupportWindow = nil
}

func supportWindowWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case wmCommand:
		if uint32(wParam)&0xffff == supportCloseButtonID && activeSupportWindow != nil {
			activeSupportWindow.close()
		}
	case wmClose:
		if activeSupportWindow != nil {
			activeSupportWindow.stopOnce.Do(func() { close(activeSupportWindow.stop) })
		}
		supportDestroyWindow.Call(hwnd)
	case wmDestroy:
		supportPostQuitMessage.Call(0)
	case wmApp:
		// Reserved for future UI-thread status marshaling.
	default:
		result, _, _ := supportDefWindowProc.Call(hwnd, uintptr(message), wParam, lParam)
		return result
	}
	return 0
}
