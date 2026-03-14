package input

import "testing"

func TestMapKeyCode(t *testing.T) {
	tests := []struct {
		code string
		want uint16
	}{
		{"KeyA", 0x41},
		{"KeyZ", 0x5A},
		{"Digit0", 0x30},
		{"Digit9", 0x39},
		{"Enter", 0x0D},
		{"Space", 0x20},
		{"Backspace", 0x08},
		{"Tab", 0x09},
		{"Escape", 0x1B},
		{"ArrowUp", 0x26},
		{"ArrowDown", 0x28},
		{"ArrowLeft", 0x25},
		{"ArrowRight", 0x27},
		{"F1", 0x70},
		{"F12", 0x7B},
		{"ShiftLeft", 0xA0},
		{"ControlLeft", 0xA2},
		{"AltLeft", 0xA4},
		{"MetaLeft", 0x5B},
		{"Delete", 0x2E},
		{"Home", 0x24},
		{"End", 0x23},
		{"PageUp", 0x21},
		{"PageDown", 0x22},
		{"Insert", 0x2D},
		// Numpad keys (#10 improvement)
		{"Numpad0", 0x60},
		{"Numpad9", 0x69},
		{"NumpadMultiply", 0x6A},
		{"NumpadAdd", 0x6B},
		{"NumpadSubtract", 0x6D},
		{"NumpadDecimal", 0x6E},
		{"NumpadDivide", 0x6F},
		{"NumpadEnter", 0x0D},
		{"NumLock", 0x90},
		{"CapsLock", 0x14},
		{"ScrollLock", 0x91},
		{"PrintScreen", 0x2C},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			got := mapKeyCode(tt.code)
			if got != tt.want {
				t.Errorf("mapKeyCode(%q) = 0x%X, want 0x%X", tt.code, got, tt.want)
			}
		})
	}
}

func TestMapKeyCodeUnknown(t *testing.T) {
	got := mapKeyCode("UnknownKey123")
	if got != 0 {
		t.Errorf("mapKeyCode(unknown) = 0x%X, want 0", got)
	}
}
