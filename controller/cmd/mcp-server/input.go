package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// buildMouseMove creates a mouse_move input event
func buildMouseMove(x, y int) string {
	msg := map[string]interface{}{
		"t": "mouse_move",
		"x": x,
		"y": y,
	}
	data, _ := json.Marshal(msg)
	return string(data)
}

// buildMouseClick creates mouse click events (down + up)
func buildMouseClick(x, y int, button string) []string {
	down := map[string]interface{}{
		"t":      "mouse_click",
		"button": button,
		"down":   true,
		"x":      x,
		"y":      y,
	}
	up := map[string]interface{}{
		"t":      "mouse_click",
		"button": button,
		"down":   false,
		"x":      x,
		"y":      y,
	}
	downJSON, _ := json.Marshal(down)
	upJSON, _ := json.Marshal(up)
	return []string{string(downJSON), string(upJSON)}
}

// buildMouseDoubleClick creates double-click events
func buildMouseDoubleClick(x, y int, button string) []string {
	events := buildMouseClick(x, y, button)
	events = append(events, buildMouseClick(x, y, button)...)
	return events
}

// buildScroll creates a mouse_scroll event
func buildScroll(delta int) string {
	msg := map[string]interface{}{
		"t":     "mouse_scroll",
		"delta": delta,
	}
	data, _ := json.Marshal(msg)
	return string(data)
}

// buildKeyPress creates key down + up events
func buildKeyPress(code string, ctrl, shift, alt bool) []string {
	down := map[string]interface{}{
		"t":     "key",
		"code":  code,
		"down":  true,
		"ctrl":  ctrl,
		"shift": shift,
		"alt":   alt,
	}
	up := map[string]interface{}{
		"t":     "key",
		"code":  code,
		"down":  false,
		"ctrl":  ctrl,
		"shift": shift,
		"alt":   alt,
	}
	downJSON, _ := json.Marshal(down)
	upJSON, _ := json.Marshal(up)
	return []string{string(downJSON), string(upJSON)}
}

// charToKeyCode converts a character to its key code
func charToKeyCode(c rune) (string, bool) {
	// Letters
	if c >= 'a' && c <= 'z' {
		return fmt.Sprintf("Key%c", c-32), false // KeyA, KeyB, etc.
	}
	if c >= 'A' && c <= 'Z' {
		return fmt.Sprintf("Key%c", c), true // Shift needed
	}
	// Digits
	if c >= '0' && c <= '9' {
		return fmt.Sprintf("Digit%c", c), false
	}
	// Common symbols
	switch c {
	case ' ':
		return "Space", false
	case '\n', '\r':
		return "Enter", false
	case '\t':
		return "Tab", false
	case '.':
		return "Period", false
	case ',':
		return "Comma", false
	case '/':
		return "Slash", false
	case '\\':
		return "Backslash", false
	case '-':
		return "Minus", false
	case '=':
		return "Equal", false
	case '[':
		return "BracketLeft", false
	case ']':
		return "BracketRight", false
	case ';':
		return "Semicolon", false
	case '\'':
		return "Quote", false
	case '`':
		return "Backquote", false
	// Shifted symbols
	case '!':
		return "Digit1", true
	case '@':
		return "Digit2", true
	case '#':
		return "Digit3", true
	case '$':
		return "Digit4", true
	case '%':
		return "Digit5", true
	case '^':
		return "Digit6", true
	case '&':
		return "Digit7", true
	case '*':
		return "Digit8", true
	case '(':
		return "Digit9", true
	case ')':
		return "Digit0", true
	case '_':
		return "Minus", true
	case '+':
		return "Equal", true
	case '{':
		return "BracketLeft", true
	case '}':
		return "BracketRight", true
	case ':':
		return "Semicolon", true
	case '"':
		return "Quote", true
	case '<':
		return "Comma", true
	case '>':
		return "Period", true
	case '?':
		return "Slash", true
	case '|':
		return "Backslash", true
	case '~':
		return "Backquote", true
	}
	return "", false
}

// buildTypeText generates key events for typing a string
func buildTypeText(text string) []string {
	var events []string
	for _, c := range text {
		code, needShift := charToKeyCode(c)
		if code == "" {
			continue // Skip unsupported characters
		}
		events = append(events, buildKeyPress(code, false, needShift, false)...)
	}
	return events
}

// parseKeyName converts a human-readable key name to a KeyCode
func parseKeyName(name string) string {
	lower := strings.ToLower(name)
	switch lower {
	case "enter", "return":
		return "Enter"
	case "tab":
		return "Tab"
	case "escape", "esc":
		return "Escape"
	case "backspace":
		return "Backspace"
	case "delete", "del":
		return "Delete"
	case "space":
		return "Space"
	case "up", "arrowup":
		return "ArrowUp"
	case "down", "arrowdown":
		return "ArrowDown"
	case "left", "arrowleft":
		return "ArrowLeft"
	case "right", "arrowright":
		return "ArrowRight"
	case "home":
		return "Home"
	case "end":
		return "End"
	case "pageup":
		return "PageUp"
	case "pagedown":
		return "PageDown"
	case "f1":
		return "F1"
	case "f2":
		return "F2"
	case "f3":
		return "F3"
	case "f4":
		return "F4"
	case "f5":
		return "F5"
	case "f6":
		return "F6"
	case "f7":
		return "F7"
	case "f8":
		return "F8"
	case "f9":
		return "F9"
	case "f10":
		return "F10"
	case "f11":
		return "F11"
	case "f12":
		return "F12"
	}
	// Single letter
	if len(name) == 1 {
		c := rune(name[0])
		code, _ := charToKeyCode(c)
		if code != "" {
			return code
		}
	}
	// Fallback: return as-is (e.g., "KeyA")
	return name
}
