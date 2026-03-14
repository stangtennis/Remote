package input

import "testing"

func TestMapKeyCode(t *testing.T) {
	tests := []struct {
		code string
		want string
	}{
		{"KeyA", "a"},
		{"KeyZ", "z"},
		{"Digit0", "0"},
		{"Digit9", "9"},
		{"Enter", "enter"},
		{"Space", "space"},
		{"Backspace", "backspace"},
		{"Tab", "tab"},
		{"Escape", "esc"},
		{"ArrowUp", "up"},
		{"ArrowDown", "down"},
		{"ArrowLeft", "left"},
		{"ArrowRight", "right"},
		{"F1", "f1"},
		{"F12", "f12"},
		{"ShiftLeft", "shift"},
		{"ControlLeft", "ctrl"},
		{"AltLeft", "alt"},
		{"MetaLeft", "cmd"},
		{"Delete", "delete"},
		{"Home", "home"},
		{"End", "end"},
		{"PageUp", "pageup"},
		{"PageDown", "pagedown"},
		{"Insert", "insert"},
		// Numpad keys (#10 improvement)
		{"Numpad0", "num0"},
		{"Numpad9", "num9"},
		{"NumpadMultiply", "num*"},
		{"NumpadAdd", "num+"},
		{"NumpadSubtract", "num-"},
		{"NumpadDecimal", "num."},
		{"NumpadDivide", "num/"},
		{"NumpadEnter", "enter"},
		{"NumLock", "num_lock"},
		{"CapsLock", "caps_lock"},
		{"ScrollLock", "scroll_lock"},
		{"PrintScreen", "print_screen"},
		{"ContextMenu", "menu"},
		{"Backquote", "`"},
		{"IntlBackslash", "\\"},
		// Punctuation
		{"Comma", ","},
		{"Period", "."},
		{"Semicolon", ";"},
		{"Minus", "-"},
		{"Equal", "="},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			got := mapKeyCode(tt.code)
			if got != tt.want {
				t.Errorf("mapKeyCode(%q) = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

func TestMapKeyCodeUnknown(t *testing.T) {
	// Unknown keys should return lowercase version
	got := mapKeyCode("UnknownKey123")
	if got != "unknownkey123" {
		t.Errorf("mapKeyCode(unknown) = %q, want %q", got, "unknownkey123")
	}
}
