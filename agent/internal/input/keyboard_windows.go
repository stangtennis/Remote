//go:build windows

package input

import (
	"fmt"
	"strings"

	"github.com/go-vgo/robotgo"
)

type KeyboardController struct{}

func NewKeyboardController() *KeyboardController {
	return &KeyboardController{}
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

// SendKeyWithModifiers sends a key with modifier keys (Ctrl, Shift, Alt)
func (k *KeyboardController) SendKeyWithModifiers(code string, down bool, ctrl, shift, alt bool) error {
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
		if ctrl || shift || alt {
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
			// Use KeyTap for key combinations (sends down+up)
			robotgo.KeyTap(key, modifiers...)
		} else {
			robotgo.KeyDown(key)
		}
	} else {
		// Only send key up if no modifiers (modifiers use KeyTap which includes up)
		if !ctrl && !shift && !alt {
			robotgo.KeyUp(key)
		}
	}

	return nil
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
