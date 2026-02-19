//go:build windows

package main

import (
	"log"
	"strings"
	"syscall"
	"unsafe"
)

var (
	user32              = syscall.NewLazyDLL("user32.dll")
	procSendInput       = user32.NewProc("SendInput")
	procGetSystemMetrics = user32.NewProc("GetSystemMetrics")
	procSetClipboardData = user32.NewProc("SetClipboardData")
	procGetClipboardData = user32.NewProc("GetClipboardData")
	procOpenClipboard    = user32.NewProc("OpenClipboard")
	procCloseClipboard   = user32.NewProc("CloseClipboard")
	procEmptyClipboard   = user32.NewProc("EmptyClipboard")
	procGlobalAlloc      = syscall.NewLazyDLL("kernel32.dll").NewProc("GlobalAlloc")
	procGlobalLock       = syscall.NewLazyDLL("kernel32.dll").NewProc("GlobalLock")
	procGlobalUnlock     = syscall.NewLazyDLL("kernel32.dll").NewProc("GlobalUnlock")
)

const (
	INPUT_MOUSE    = 0
	INPUT_KEYBOARD = 1

	MOUSEEVENTF_MOVE        = 0x0001
	MOUSEEVENTF_LEFTDOWN    = 0x0002
	MOUSEEVENTF_LEFTUP      = 0x0004
	MOUSEEVENTF_RIGHTDOWN   = 0x0008
	MOUSEEVENTF_RIGHTUP     = 0x0010
	MOUSEEVENTF_MIDDLEDOWN  = 0x0020
	MOUSEEVENTF_MIDDLEUP    = 0x0040
	MOUSEEVENTF_WHEEL       = 0x0800
	MOUSEEVENTF_HWHEEL      = 0x1000
	MOUSEEVENTF_ABSOLUTE    = 0x8000

	KEYEVENTF_KEYUP         = 0x0002
	KEYEVENTF_EXTENDEDKEY   = 0x0001

	SM_CXSCREEN = 0
	SM_CYSCREEN = 1

	CF_UNICODETEXT = 13
	GMEM_MOVEABLE  = 0x0002
)

type MOUSEINPUT struct {
	Dx          int32
	Dy          int32
	MouseData   uint32
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
}

type KEYBDINPUT struct {
	WVk         uint16
	WScan       uint16
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
}

type INPUT struct {
	Type uint32
	Mi   MOUSEINPUT
}

type KEYINPUT struct {
	Type uint32
	Ki   KEYBDINPUT
	_    [8]byte // Padding to match INPUT union size
}

// InputInjector handles input injection on Windows
type InputInjector struct {
	screenWidth  int
	screenHeight int
}

func NewInputInjector() *InputInjector {
	w, _, _ := procGetSystemMetrics.Call(SM_CXSCREEN)
	h, _, _ := procGetSystemMetrics.Call(SM_CYSCREEN)

	log.Printf("🖥️ Screen resolution: %dx%d", w, h)

	return &InputInjector{
		screenWidth:  int(w),
		screenHeight: int(h),
	}
}

func (i *InputInjector) MouseMoveRelative(dx, dy int) {
	input := INPUT{
		Type: INPUT_MOUSE,
		Mi: MOUSEINPUT{
			Dx:      int32(dx),
			Dy:      int32(dy),
			DwFlags: MOUSEEVENTF_MOVE,
		},
	}

	procSendInput.Call(1, uintptr(unsafe.Pointer(&input)), unsafe.Sizeof(input))
}

func (i *InputInjector) MouseMoveAbsolute(x, y float64) {
	// Convert normalized coords (0-1) to absolute coords (0-65535)
	absX := int32(x * 65535)
	absY := int32(y * 65535)

	input := INPUT{
		Type: INPUT_MOUSE,
		Mi: MOUSEINPUT{
			Dx:      absX,
			Dy:      absY,
			DwFlags: MOUSEEVENTF_MOVE | MOUSEEVENTF_ABSOLUTE,
		},
	}

	procSendInput.Call(1, uintptr(unsafe.Pointer(&input)), unsafe.Sizeof(input))
}

func (i *InputInjector) MouseButton(button string, down bool) {
	var flags uint32

	switch button {
	case "left":
		if down {
			flags = MOUSEEVENTF_LEFTDOWN
		} else {
			flags = MOUSEEVENTF_LEFTUP
		}
	case "right":
		if down {
			flags = MOUSEEVENTF_RIGHTDOWN
		} else {
			flags = MOUSEEVENTF_RIGHTUP
		}
	case "middle":
		if down {
			flags = MOUSEEVENTF_MIDDLEDOWN
		} else {
			flags = MOUSEEVENTF_MIDDLEUP
		}
	default:
		return
	}

	input := INPUT{
		Type: INPUT_MOUSE,
		Mi: MOUSEINPUT{
			DwFlags: flags,
		},
	}

	procSendInput.Call(1, uintptr(unsafe.Pointer(&input)), unsafe.Sizeof(input))
}

func (i *InputInjector) MouseWheel(dx, dy int) {
	if dy != 0 {
		input := INPUT{
			Type: INPUT_MOUSE,
			Mi: MOUSEINPUT{
				MouseData: uint32(dy),
				DwFlags:   MOUSEEVENTF_WHEEL,
			},
		}
		procSendInput.Call(1, uintptr(unsafe.Pointer(&input)), unsafe.Sizeof(input))
	}

	if dx != 0 {
		input := INPUT{
			Type: INPUT_MOUSE,
			Mi: MOUSEINPUT{
				MouseData: uint32(dx),
				DwFlags:   MOUSEEVENTF_HWHEEL,
			},
		}
		procSendInput.Call(1, uintptr(unsafe.Pointer(&input)), unsafe.Sizeof(input))
	}
}

// Virtual key codes
var vkCodes = map[string]uint16{
	"KeyA": 0x41, "KeyB": 0x42, "KeyC": 0x43, "KeyD": 0x44, "KeyE": 0x45,
	"KeyF": 0x46, "KeyG": 0x47, "KeyH": 0x48, "KeyI": 0x49, "KeyJ": 0x4A,
	"KeyK": 0x4B, "KeyL": 0x4C, "KeyM": 0x4D, "KeyN": 0x4E, "KeyO": 0x4F,
	"KeyP": 0x50, "KeyQ": 0x51, "KeyR": 0x52, "KeyS": 0x53, "KeyT": 0x54,
	"KeyU": 0x55, "KeyV": 0x56, "KeyW": 0x57, "KeyX": 0x58, "KeyY": 0x59,
	"KeyZ": 0x5A,

	"Digit0": 0x30, "Digit1": 0x31, "Digit2": 0x32, "Digit3": 0x33, "Digit4": 0x34,
	"Digit5": 0x35, "Digit6": 0x36, "Digit7": 0x37, "Digit8": 0x38, "Digit9": 0x39,

	"F1": 0x70, "F2": 0x71, "F3": 0x72, "F4": 0x73, "F5": 0x74, "F6": 0x75,
	"F7": 0x76, "F8": 0x77, "F9": 0x78, "F10": 0x79, "F11": 0x7A, "F12": 0x7B,

	"Escape": 0x1B, "Tab": 0x09, "CapsLock": 0x14, "ShiftLeft": 0x10, "ShiftRight": 0x10,
	"ControlLeft": 0x11, "ControlRight": 0x11, "AltLeft": 0x12, "AltRight": 0x12,
	"Space": 0x20, "Enter": 0x0D, "Backspace": 0x08, "Delete": 0x2E, "Insert": 0x2D,
	"Home": 0x24, "End": 0x23, "PageUp": 0x21, "PageDown": 0x22,

	"ArrowUp": 0x26, "ArrowDown": 0x28, "ArrowLeft": 0x25, "ArrowRight": 0x27,

	"Numpad0": 0x60, "Numpad1": 0x61, "Numpad2": 0x62, "Numpad3": 0x63, "Numpad4": 0x64,
	"Numpad5": 0x65, "Numpad6": 0x66, "Numpad7": 0x67, "Numpad8": 0x68, "Numpad9": 0x69,
	"NumpadMultiply": 0x6A, "NumpadAdd": 0x6B, "NumpadSubtract": 0x6D,
	"NumpadDecimal": 0x6E, "NumpadDivide": 0x6F, "NumpadEnter": 0x0D, "NumLock": 0x90,

	"Semicolon": 0xBA, "Equal": 0xBB, "Comma": 0xBC, "Minus": 0xBD,
	"Period": 0xBE, "Slash": 0xBF, "Backquote": 0xC0,
	"BracketLeft": 0xDB, "Backslash": 0xDC, "BracketRight": 0xDD, "Quote": 0xDE,

	"PrintScreen": 0x2C, "ScrollLock": 0x91, "Pause": 0x13,
	"ContextMenu": 0x5D, "MetaLeft": 0x5B, "MetaRight": 0x5C,
}

// Extended keys that need KEYEVENTF_EXTENDEDKEY
var extendedKeys = map[string]bool{
	"ArrowUp": true, "ArrowDown": true, "ArrowLeft": true, "ArrowRight": true,
	"Home": true, "End": true, "PageUp": true, "PageDown": true,
	"Insert": true, "Delete": true, "NumLock": true,
	"ControlRight": true, "AltRight": true, "MetaLeft": true, "MetaRight": true,
}

func (i *InputInjector) KeyEvent(code string, down bool, ctrl, shift, alt bool) {
	vk, ok := vkCodes[code]
	if !ok {
		log.Printf("Unknown key code: %s", code)
		return
	}

	var flags uint32 = 0
	if !down {
		flags |= KEYEVENTF_KEYUP
	}
	if extendedKeys[code] {
		flags |= KEYEVENTF_EXTENDEDKEY
	}

	input := KEYINPUT{
		Type: INPUT_KEYBOARD,
		Ki: KEYBDINPUT{
			WVk:     vk,
			DwFlags: flags,
		},
	}

	procSendInput.Call(1, uintptr(unsafe.Pointer(&input)), unsafe.Sizeof(INPUT{}))
}

func (i *InputInjector) SetClipboard(text string) {
	procOpenClipboard.Call(0)
	defer procCloseClipboard.Call()

	procEmptyClipboard.Call()

	// Convert to UTF-16
	utf16 := syscall.StringToUTF16(text)
	size := len(utf16) * 2

	// Allocate global memory
	hMem, _, _ := procGlobalAlloc.Call(GMEM_MOVEABLE, uintptr(size))
	if hMem == 0 {
		return
	}

	// Lock and copy
	ptr, _, _ := procGlobalLock.Call(hMem)
	if ptr == 0 {
		return
	}

	for i, c := range utf16 {
		*(*uint16)(unsafe.Pointer(ptr + uintptr(i*2))) = c
	}

	procGlobalUnlock.Call(hMem)
	procSetClipboardData.Call(CF_UNICODETEXT, hMem)

	log.Printf("📋 Clipboard set: %d chars", len(text))
}

func (i *InputInjector) GetClipboard() string {
	procOpenClipboard.Call(0)
	defer procCloseClipboard.Call()

	hMem, _, _ := procGetClipboardData.Call(CF_UNICODETEXT)
	if hMem == 0 {
		return ""
	}

	ptr, _, _ := procGlobalLock.Call(hMem)
	if ptr == 0 {
		return ""
	}
	defer procGlobalUnlock.Call(hMem)

	// Read UTF-16 string
	var utf16 []uint16
	for i := 0; ; i++ {
		c := *(*uint16)(unsafe.Pointer(ptr + uintptr(i*2)))
		if c == 0 {
			break
		}
		utf16 = append(utf16, c)
	}

	return strings.TrimRight(syscall.UTF16ToString(utf16), "\x00")
}
