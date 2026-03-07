//go:build windows

package main

import (
	"encoding/binary"
	"log"
	"strings"
	"syscall"
	"unsafe"
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procSendInput        = user32.NewProc("SendInput")
	procGetSystemMetrics = user32.NewProc("GetSystemMetrics")
	procVkKeyScanW       = user32.NewProc("VkKeyScanW")
	procGetForegroundWnd = user32.NewProc("GetForegroundWindow")
	procGetClassNameW    = user32.NewProc("GetClassNameW")
	procSetClipboardData = user32.NewProc("SetClipboardData")
	procGetClipboardData = user32.NewProc("GetClipboardData")
	procOpenClipboard    = user32.NewProc("OpenClipboard")
	procCloseClipboard   = user32.NewProc("CloseClipboard")
	procEmptyClipboard   = user32.NewProc("EmptyClipboard")
	procGlobalAlloc      = kernel32.NewProc("GlobalAlloc")
	procGlobalLock       = kernel32.NewProc("GlobalLock")
	procGlobalUnlock     = kernel32.NewProc("GlobalUnlock")
)

const (
	INPUT_MOUSE    = 0
	INPUT_KEYBOARD = 1

	MOUSEEVENTF_MOVE     = 0x0001
	MOUSEEVENTF_LEFTDOWN = 0x0002
	MOUSEEVENTF_LEFTUP   = 0x0004
	MOUSEEVENTF_RIGHTDOWN  = 0x0008
	MOUSEEVENTF_RIGHTUP    = 0x0010
	MOUSEEVENTF_MIDDLEDOWN = 0x0020
	MOUSEEVENTF_MIDDLEUP   = 0x0040
	MOUSEEVENTF_WHEEL    = 0x0800
	MOUSEEVENTF_HWHEEL   = 0x1000
	MOUSEEVENTF_ABSOLUTE = 0x8000

	KEYEVENTF_EXTENDEDKEY = 0x0001
	KEYEVENTF_KEYUP       = 0x0002
	KEYEVENTF_UNICODE     = 0x0004

	SM_CXSCREEN = 0
	SM_CYSCREEN = 1

	CF_UNICODETEXT = 13
	GMEM_MOVEABLE  = 0x0002

	cbSizeInput = 40 // sizeof(INPUT) on 64-bit Windows
)

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

// --- Low-level helpers (match Session 0 helper) ---

// makeKeyInput builds a 40-byte INPUT struct for keyboard events.
func makeKeyInput(wVk, wScan uint16, dwFlags uint32) [cbSizeInput]byte {
	var buf [cbSizeInput]byte
	binary.LittleEndian.PutUint32(buf[0:4], INPUT_KEYBOARD)
	binary.LittleEndian.PutUint16(buf[8:10], wVk)
	binary.LittleEndian.PutUint16(buf[10:12], wScan)
	binary.LittleEndian.PutUint32(buf[12:16], dwFlags)
	return buf
}

// sendInputRaw wraps the SendInput call.
func sendInputRaw(inputs []byte, count int) {
	if count == 0 {
		return
	}
	n, _, err := procSendInput.Call(
		uintptr(count),
		uintptr(unsafe.Pointer(&inputs[0])),
		uintptr(cbSizeInput),
	)
	if n == 0 {
		log.Printf("SendInput failed: %v", err)
	}
}

// charToVK maps a Unicode character to a Windows virtual key code and modifier state
// using VkKeyScanW, which respects the current keyboard layout.
// Returns (0, 0) if the character can't be mapped.
// Modifier bits: 0x01=Shift, 0x02=Ctrl, 0x04=Alt
// AltGr is represented as Ctrl+Alt (0x06) together.
func charToVK(char uint16) (vk uint16, mods byte) {
	ret, _, _ := procVkKeyScanW.Call(uintptr(char))
	result := int16(ret)
	if result == -1 {
		return 0, 0
	}
	vkCode := uint16(result & 0xFF)
	modifiers := byte((result >> 8) & 0xFF)
	ctrl := modifiers & 0x02
	alt := modifiers & 0x04
	// Allow: no modifier, Shift only, or AltGr (Ctrl+Alt together).
	// Reject: only Ctrl or only Alt (unusual, fall back to Unicode).
	if (ctrl != 0) != (alt != 0) {
		return 0, 0
	}
	return vkCode, modifiers
}

// isForegroundConsole returns true if the foreground window is a console window.
func isForegroundConsole() bool {
	hwnd, _, _ := procGetForegroundWnd.Call()
	if hwnd == 0 {
		return false
	}
	var className [256]uint16
	ret, _, _ := procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&className[0])), 256)
	if ret == 0 {
		return false
	}
	name := syscall.UTF16ToString(className[:ret])
	return name == "ConsoleWindowClass" || name == "CASCADIA_HOSTING_WINDOW_CLASS"
}

// Extended keys that need KEYEVENTF_EXTENDEDKEY
var extendedKeys = map[string]bool{
	"ArrowUp": true, "ArrowDown": true, "ArrowLeft": true, "ArrowRight": true,
	"Home": true, "End": true, "PageUp": true, "PageDown": true,
	"Insert": true, "Delete": true, "NumLock": true,
	"ControlRight": true, "AltRight": true, "MetaLeft": true, "MetaRight": true,
	"NumpadEnter": true, "NumpadDivide": true,
}

// Virtual key codes (JS KeyboardEvent.code → Windows VK)
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

	"IntlBackslash": 0xE2,
}

// --- Mouse ---

func (i *InputInjector) MouseMoveRelative(dx, dy int) {
	var buf [cbSizeInput]byte
	binary.LittleEndian.PutUint32(buf[0:4], INPUT_MOUSE)
	binary.LittleEndian.PutUint32(buf[8:12], uint32(int32(dx)))
	binary.LittleEndian.PutUint32(buf[12:16], uint32(int32(dy)))
	binary.LittleEndian.PutUint32(buf[20:24], MOUSEEVENTF_MOVE)
	sendInputRaw(buf[:], 1)
}

func (i *InputInjector) MouseMoveAbsolute(x, y float64) {
	absX := int32(x * 65535)
	absY := int32(y * 65535)

	var buf [cbSizeInput]byte
	binary.LittleEndian.PutUint32(buf[0:4], INPUT_MOUSE)
	binary.LittleEndian.PutUint32(buf[8:12], uint32(absX))
	binary.LittleEndian.PutUint32(buf[12:16], uint32(absY))
	binary.LittleEndian.PutUint32(buf[20:24], MOUSEEVENTF_MOVE|MOUSEEVENTF_ABSOLUTE)
	sendInputRaw(buf[:], 1)
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

	var buf [cbSizeInput]byte
	binary.LittleEndian.PutUint32(buf[0:4], INPUT_MOUSE)
	binary.LittleEndian.PutUint32(buf[20:24], flags)
	sendInputRaw(buf[:], 1)
}

func (i *InputInjector) MouseWheel(dx, dy int) {
	if dy != 0 {
		var buf [cbSizeInput]byte
		binary.LittleEndian.PutUint32(buf[0:4], INPUT_MOUSE)
		binary.LittleEndian.PutUint32(buf[16:20], uint32(int32(dy)))
		binary.LittleEndian.PutUint32(buf[20:24], MOUSEEVENTF_WHEEL)
		sendInputRaw(buf[:], 1)
	}
	if dx != 0 {
		var buf [cbSizeInput]byte
		binary.LittleEndian.PutUint32(buf[0:4], INPUT_MOUSE)
		binary.LittleEndian.PutUint32(buf[16:20], uint32(int32(dx)))
		binary.LittleEndian.PutUint32(buf[20:24], MOUSEEVENTF_HWHEEL)
		sendInputRaw(buf[:], 1)
	}
}

// --- Keyboard ---

// KeyEvent handles key down/up events using JS KeyboardEvent.code.
// This is used for modifier keys and special keys (arrows, F-keys, etc.)
func (i *InputInjector) KeyEvent(code string, down bool, ctrl, shift, alt, meta bool) {
	vk, ok := vkCodes[code]
	if !ok {
		log.Printf("Unknown key code: %s", code)
		return
	}

	var flags uint32
	if !down {
		flags |= KEYEVENTF_KEYUP
	}
	if extendedKeys[code] {
		flags |= KEYEVENTF_EXTENDEDKEY
	}

	// Build input array with modifiers (press mod → key → release mod)
	var inputs []byte

	if down {
		if ctrl {
			inp := makeKeyInput(0x11, 0, 0)
			inputs = append(inputs, inp[:]...)
		}
		if shift {
			inp := makeKeyInput(0x10, 0, 0)
			inputs = append(inputs, inp[:]...)
		}
		if alt {
			inp := makeKeyInput(0x12, 0, 0)
			inputs = append(inputs, inp[:]...)
		}
		if meta {
			inp := makeKeyInput(0x5B, 0, 0)
			inputs = append(inputs, inp[:]...)
		}

		inp := makeKeyInput(vk, 0, flags)
		inputs = append(inputs, inp[:]...)

		inp = makeKeyInput(vk, 0, flags|KEYEVENTF_KEYUP)
		inputs = append(inputs, inp[:]...)

		if meta {
			inp := makeKeyInput(0x5B, 0, KEYEVENTF_KEYUP)
			inputs = append(inputs, inp[:]...)
		}
		if alt {
			inp := makeKeyInput(0x12, 0, KEYEVENTF_KEYUP)
			inputs = append(inputs, inp[:]...)
		}
		if shift {
			inp := makeKeyInput(0x10, 0, KEYEVENTF_KEYUP)
			inputs = append(inputs, inp[:]...)
		}
		if ctrl {
			inp := makeKeyInput(0x11, 0, KEYEVENTF_KEYUP)
			inputs = append(inputs, inp[:]...)
		}
	} else {
		inp := makeKeyInput(vk, 0, flags)
		inputs = append(inputs, inp[:]...)
	}

	if count := len(inputs) / cbSizeInput; count > 0 {
		sendInputRaw(inputs, count)
	}
}

// TypeUnicode types a Unicode character using the same logic as Session 0 helper.
// For mappable characters, uses VK codes (works in elevated consoles).
// For AltGr chars in GUI apps, uses KEYEVENTF_UNICODE (avoids Ctrl+key conflicts).
// For non-mappable chars (emoji, CJK), uses KEYEVENTF_UNICODE.
func (i *InputInjector) TypeUnicode(char uint16) {
	if vk, mods := charToVK(char); vk != 0 {
		shiftNeeded := mods&0x01 != 0
		altGr := mods&0x06 == 0x06

		// AltGr: use KEYEVENTF_UNICODE for GUI apps (avoids Ctrl+key shortcuts)
		if altGr && !isForegroundConsole() {
			down := makeKeyInput(0, char, KEYEVENTF_UNICODE)
			up := makeKeyInput(0, char, KEYEVENTF_UNICODE|KEYEVENTF_KEYUP)
			var inputs [2 * cbSizeInput]byte
			copy(inputs[0:cbSizeInput], down[:])
			copy(inputs[cbSizeInput:2*cbSizeInput], up[:])
			sendInputRaw(inputs[:], 2)
			return
		}

		// Console or non-AltGr: use VK codes with modifiers
		var inputs []byte
		if altGr {
			inp := makeKeyInput(0xA2, 0, KEYEVENTF_EXTENDEDKEY) // VK_LCONTROL
			inputs = append(inputs, inp[:]...)
			inp = makeKeyInput(0xA5, 0, 0) // VK_RMENU
			inputs = append(inputs, inp[:]...)
		}
		if shiftNeeded {
			inp := makeKeyInput(0x10, 0, 0)
			inputs = append(inputs, inp[:]...)
		}
		inp := makeKeyInput(vk, 0, 0)
		inputs = append(inputs, inp[:]...)
		inp = makeKeyInput(vk, 0, KEYEVENTF_KEYUP)
		inputs = append(inputs, inp[:]...)
		if shiftNeeded {
			inp := makeKeyInput(0x10, 0, KEYEVENTF_KEYUP)
			inputs = append(inputs, inp[:]...)
		}
		if altGr {
			inp := makeKeyInput(0xA5, 0, KEYEVENTF_KEYUP)
			inputs = append(inputs, inp[:]...)
			inp = makeKeyInput(0xA2, 0, KEYEVENTF_EXTENDEDKEY|KEYEVENTF_KEYUP)
			inputs = append(inputs, inp[:]...)
		}
		sendInputRaw(inputs, len(inputs)/cbSizeInput)
		return
	}

	// Non-mappable: use KEYEVENTF_UNICODE (emoji, CJK, accented chars, etc.)
	down := makeKeyInput(0, char, KEYEVENTF_UNICODE)
	up := makeKeyInput(0, char, KEYEVENTF_UNICODE|KEYEVENTF_KEYUP)
	var inputs [2 * cbSizeInput]byte
	copy(inputs[0:cbSizeInput], down[:])
	copy(inputs[cbSizeInput:2*cbSizeInput], up[:])
	sendInputRaw(inputs[:], 2)
}

// --- Clipboard ---

func (i *InputInjector) SetClipboard(text string) {
	procOpenClipboard.Call(0)
	defer procCloseClipboard.Call()

	procEmptyClipboard.Call()

	utf16 := syscall.StringToUTF16(text)
	size := len(utf16) * 2

	hMem, _, _ := procGlobalAlloc.Call(GMEM_MOVEABLE, uintptr(size))
	if hMem == 0 {
		return
	}

	ptr, _, _ := procGlobalLock.Call(hMem)
	if ptr == 0 {
		return
	}

	for j, c := range utf16 {
		*(*uint16)(unsafe.Pointer(ptr + uintptr(j*2))) = c
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

	var utf16 []uint16
	for j := 0; ; j++ {
		c := *(*uint16)(unsafe.Pointer(ptr + uintptr(j*2)))
		if c == 0 {
			break
		}
		utf16 = append(utf16, c)
	}

	return strings.TrimRight(syscall.UTF16ToString(utf16), "\x00")
}
