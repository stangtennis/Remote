//go:build darwin

package input

/*
#cgo LDFLAGS: -framework CoreGraphics -framework ApplicationServices
#include <CoreGraphics/CoreGraphics.h>
#include <ApplicationServices/ApplicationServices.h>

static void keyEvent(int keyCode, int down) {
    CGEventRef event = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)keyCode, down ? true : false);
    CGEventPost(kCGSessionEventTap, event);
    CFRelease(event);
}

static void keyEventWithFlags(int keyCode, int down, uint64_t flags) {
    CGEventRef event = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)keyCode, down ? true : false);
    CGEventSetFlags(event, (CGEventFlags)flags);
    CGEventPost(kCGSessionEventTap, event);
    CFRelease(event);
}
*/
import "C"
import (
	"fmt"
	"log"
	"strings"
)

type KeyboardController struct{}

func NewKeyboardController() *KeyboardController {
	return &KeyboardController{}
}

func (k *KeyboardController) SendKey(code string, down bool) error {
	keyCode := mapKeyCodeToMac(code)
	if keyCode < 0 {
		return fmt.Errorf("unknown key code: %s", code)
	}

	if down {
		C.keyEvent(C.int(keyCode), 1)
	} else {
		C.keyEvent(C.int(keyCode), 0)
	}

	return nil
}

func (k *KeyboardController) SendKeyWithModifiers(code string, down bool, ctrl, shift, alt bool) error {
	keyCode := mapKeyCodeToMac(code)
	if keyCode < 0 {
		return fmt.Errorf("unknown key code: %s", code)
	}

	log.Printf("⌨️ Key: %s (0x%02X) down=%v ctrl=%v shift=%v alt=%v", code, keyCode, down, ctrl, shift, alt)

	// Build modifier flags (macOS CGEvent flag masks)
	var flags C.uint64_t = 0
	if ctrl {
		flags |= 0x100000 // kCGEventFlagMaskCommand (Ctrl maps to Cmd on macOS)
	}
	if shift {
		flags |= 0x20000 // kCGEventFlagMaskShift
	}
	if alt {
		flags |= 0x80000 // kCGEventFlagMaskAlternate
	}

	if flags != 0 {
		C.keyEventWithFlags(C.int(keyCode), boolToInt(down), flags)
	} else {
		C.keyEvent(C.int(keyCode), boolToInt(down))
	}

	return nil
}

// ClearModifiers releases all modifier keys to prevent stuck modifier state
// (e.g., after a session drops while modifier keys were held)
func (k *KeyboardController) ClearModifiers() {
	modifiers := []int{
		0x37, // Left Command
		0x36, // Right Command
		0x38, // Left Shift
		0x3C, // Right Shift
		0x3B, // Left Control
		0x3E, // Right Control
		0x3A, // Left Alt/Option
		0x3D, // Right Alt/Option
	}
	for _, code := range modifiers {
		C.keyEvent(C.int(code), 0) // keyup
	}
}

func boolToInt(b bool) C.int {
	if b {
		return 1
	}
	return 0
}

// mapKeyCodeToMac maps JavaScript KeyboardEvent.code to macOS virtual key codes
func mapKeyCodeToMac(code string) int {
	keyMap := map[string]int{
		// Letters (macOS virtual key codes)
		"KeyA": 0x00, "KeyB": 0x0B, "KeyC": 0x08, "KeyD": 0x02, "KeyE": 0x0E,
		"KeyF": 0x03, "KeyG": 0x05, "KeyH": 0x04, "KeyI": 0x22, "KeyJ": 0x26,
		"KeyK": 0x28, "KeyL": 0x25, "KeyM": 0x2E, "KeyN": 0x2D, "KeyO": 0x1F,
		"KeyP": 0x23, "KeyQ": 0x0C, "KeyR": 0x0F, "KeyS": 0x01, "KeyT": 0x11,
		"KeyU": 0x20, "KeyV": 0x09, "KeyW": 0x0D, "KeyX": 0x07, "KeyY": 0x10,
		"KeyZ": 0x06,

		// Numbers
		"Digit0": 0x1D, "Digit1": 0x12, "Digit2": 0x13, "Digit3": 0x14, "Digit4": 0x15,
		"Digit5": 0x17, "Digit6": 0x16, "Digit7": 0x1A, "Digit8": 0x1C, "Digit9": 0x19,

		// Function keys
		"F1": 0x7A, "F2": 0x78, "F3": 0x63, "F4": 0x76, "F5": 0x60, "F6": 0x61,
		"F7": 0x62, "F8": 0x64, "F9": 0x65, "F10": 0x6D, "F11": 0x67, "F12": 0x6F,

		// Special keys
		"Enter":     0x24,
		"Space":     0x31,
		"Backspace": 0x33,
		"Tab":       0x30,
		"Escape":    0x35,
		"Delete":    0x75,
		"Home":      0x73,
		"End":       0x77,
		"PageUp":    0x74,
		"PageDown":  0x79,

		// Arrow keys
		"ArrowUp":    0x7E,
		"ArrowDown":  0x7D,
		"ArrowLeft":  0x7B,
		"ArrowRight": 0x7C,

		// Modifiers
		"ShiftLeft":    0x38,
		"ShiftRight":   0x3C,
		"ControlLeft":  0x3B,
		"ControlRight": 0x3E,
		"AltLeft":      0x3A,
		"AltRight":     0x3D,
		"MetaLeft":     0x37, // Left Command
		"MetaRight":    0x36, // Right Command

		// Punctuation
		"Comma":        0x2B,
		"Period":       0x2F,
		"Slash":        0x2C,
		"Semicolon":    0x29,
		"Quote":        0x27,
		"BracketLeft":  0x21,
		"BracketRight": 0x1E,
		"Backslash":    0x2A,
		"Minus":        0x1B,
		"Equal":        0x18,
		"Backquote":    0x32,
	}

	if code, ok := keyMap[code]; ok {
		return code
	}

	// Try case-insensitive match
	for k, v := range keyMap {
		if strings.EqualFold(k, code) {
			return v
		}
	}

	return -1
}
