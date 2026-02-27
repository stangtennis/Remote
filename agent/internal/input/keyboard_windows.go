//go:build windows

package input

import (
	"encoding/binary"
	"fmt"
	"strings"
	"unsafe"

	"github.com/go-vgo/robotgo"
)

// SendInput API constants
const (
	inputKeyboard    = 1
	keyeventfUnicode = 0x0004
	keyeventfKeyup   = 0x0002
	cbSize           = 40 // sizeof(INPUT) on 64-bit Windows
)

var (
	procSendInput = user32.NewProc("SendInput")
)

type KeyboardController struct{}

func NewKeyboardController() *KeyboardController {
	return &KeyboardController{}
}

// makeKeyboardInput builds a raw 40-byte INPUT struct for SendInput.
// Layout on 64-bit Windows:
//
//	Offset  0: type      (DWORD, 4 bytes) = INPUT_KEYBOARD (1)
//	Offset  4: padding   (4 bytes)
//	Offset  8: wVk       (WORD,  2 bytes)
//	Offset 10: wScan     (WORD,  2 bytes)
//	Offset 12: dwFlags   (DWORD, 4 bytes)
//	Offset 16: time      (DWORD, 4 bytes)
//	Offset 20: padding   (4 bytes)
//	Offset 24: dwExtraInfo (ULONG_PTR, 8 bytes)
//	Offset 32: padding   (8 bytes, union filler)
//	Total: 40 bytes
func makeKeyboardInput(wVk, wScan uint16, dwFlags uint32) [cbSize]byte {
	var buf [cbSize]byte
	binary.LittleEndian.PutUint32(buf[0:4], inputKeyboard) // type
	// buf[4:8] = padding (zero)
	binary.LittleEndian.PutUint16(buf[8:10], wVk)    // wVk
	binary.LittleEndian.PutUint16(buf[10:12], wScan)  // wScan
	binary.LittleEndian.PutUint32(buf[12:16], dwFlags) // dwFlags
	// buf[16:40] = zero (time, padding, dwExtraInfo, union filler)
	return buf
}

// SendUnicodeChar sends a Unicode character using SendInput with KEYEVENTF_UNICODE.
// This bypasses keyboard layout issues — the character is sent directly regardless
// of what keyboard layout is active on the target machine.
func (k *KeyboardController) SendUnicodeChar(char rune) error {
	down := makeKeyboardInput(0, uint16(char), keyeventfUnicode)
	up := makeKeyboardInput(0, uint16(char), keyeventfUnicode|keyeventfKeyup)

	// Build contiguous array of 2 INPUT structs (80 bytes total)
	var inputs [2 * cbSize]byte
	copy(inputs[0:cbSize], down[:])
	copy(inputs[cbSize:2*cbSize], up[:])

	n, _, err := procSendInput.Call(
		uintptr(2),
		uintptr(unsafe.Pointer(&inputs[0])),
		uintptr(cbSize),
	)
	if n == 0 {
		return fmt.Errorf("SendInput failed: %v", err)
	}
	return nil
}

func (k *KeyboardController) SendKey(code string, down bool) error {
	// Map JavaScript KeyboardEvent.code to robotgo key codes
	key := mapKeyCode(code)
	if key == "" {
		return fmt.Errorf("unknown key code: %s", code)
	}

	if down {
		robotgo.KeyDown(key)
	} else {
		robotgo.KeyUp(key)
	}

	return nil
}

// SendKeyWithModifiers sends a key with modifier keys (Ctrl, Shift, Alt, Meta/Win)
func (k *KeyboardController) SendKeyWithModifiers(code string, down bool, ctrl, shift, alt, meta bool) error {
	// Map JavaScript KeyboardEvent.code to robotgo key codes
	key := mapKeyCode(code)
	if key == "" {
		return fmt.Errorf("unknown key code: %s", code)
	}

	// Skip if this is a modifier key itself (they're handled separately)
	if key == "ctrl" || key == "shift" || key == "alt" || key == "cmd" {
		if down {
			robotgo.KeyDown(key)
		} else {
			robotgo.KeyUp(key)
		}
		return nil
	}

	// For non-modifier keys with modifiers pressed, use robotgo.KeyTap with modifiers
	if down {
		if ctrl || shift || alt || meta {
			// Build modifier list
			var modifiers []interface{}
			if ctrl {
				modifiers = append(modifiers, "ctrl")
			}
			if shift {
				modifiers = append(modifiers, "shift")
			}
			if alt {
				modifiers = append(modifiers, "alt")
			}
			if meta {
				modifiers = append(modifiers, "cmd")
			}
			// Use KeyTap for key combinations (sends down+up)
			robotgo.KeyTap(key, modifiers...)
		} else {
			robotgo.KeyDown(key)
		}
	} else {
		// Only send key up if no modifiers (modifiers use KeyTap which includes up)
		if !ctrl && !shift && !alt && !meta {
			robotgo.KeyUp(key)
		}
	}

	return nil
}

// ClearModifiers releases all modifier keys to prevent stuck modifier state
func (k *KeyboardController) ClearModifiers() {
	for _, mod := range []string{"ctrl", "shift", "alt", "cmd"} {
		robotgo.KeyUp(mod)
	}
}

func mapKeyCode(code string) string {
	// Common key mappings from JavaScript to robotgo
	keyMap := map[string]string{
		// Letters
		"KeyA": "a", "KeyB": "b", "KeyC": "c", "KeyD": "d", "KeyE": "e",
		"KeyF": "f", "KeyG": "g", "KeyH": "h", "KeyI": "i", "KeyJ": "j",
		"KeyK": "k", "KeyL": "l", "KeyM": "m", "KeyN": "n", "KeyO": "o",
		"KeyP": "p", "KeyQ": "q", "KeyR": "r", "KeyS": "s", "KeyT": "t",
		"KeyU": "u", "KeyV": "v", "KeyW": "w", "KeyX": "x", "KeyY": "y",
		"KeyZ": "z",

		// Numbers
		"Digit0": "0", "Digit1": "1", "Digit2": "2", "Digit3": "3", "Digit4": "4",
		"Digit5": "5", "Digit6": "6", "Digit7": "7", "Digit8": "8", "Digit9": "9",

		// Function keys
		"F1": "f1", "F2": "f2", "F3": "f3", "F4": "f4", "F5": "f5", "F6": "f6",
		"F7": "f7", "F8": "f8", "F9": "f9", "F10": "f10", "F11": "f11", "F12": "f12",

		// Special keys
		"Enter":     "enter",
		"Space":     "space",
		"Backspace": "backspace",
		"Tab":       "tab",
		"Escape":    "esc",
		"Delete":    "delete",
		"Insert":    "insert",
		"Home":      "home",
		"End":       "end",
		"PageUp":    "pageup",
		"PageDown":  "pagedown",

		// Arrow keys
		"ArrowUp":    "up",
		"ArrowDown":  "down",
		"ArrowLeft":  "left",
		"ArrowRight": "right",

		// Modifiers
		"ShiftLeft":    "shift",
		"ShiftRight":   "shift",
		"ControlLeft":  "ctrl",
		"ControlRight": "ctrl",
		"AltLeft":      "alt",
		"AltRight":     "alt",
		"MetaLeft":     "cmd",
		"MetaRight":    "cmd",

		// Punctuation
		"Comma":        ",",
		"Period":       ".",
		"Slash":        "/",
		"Semicolon":    ";",
		"Quote":        "'",
		"BracketLeft":  "[",
		"BracketRight": "]",
		"Backslash":    "\\",
		"Minus":        "-",
		"Equal":        "=",
	}

	if mapped, ok := keyMap[code]; ok {
		return mapped
	}

	// Try lowercase of the key
	return strings.ToLower(code)
}
