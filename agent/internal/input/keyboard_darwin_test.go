//go:build darwin

package input

import "testing"

func TestMapKeyCodeToMac(t *testing.T) {
	tests := []struct {
		code     string
		expected int
	}{
		// Letters
		{"KeyA", 0x00},
		{"KeyZ", 0x06},
		{"KeyM", 0x2E},

		// Digits
		{"Digit0", 0x1D},
		{"Digit1", 0x12},
		{"Digit9", 0x19},

		// Function keys
		{"F1", 0x7A},
		{"F12", 0x6F},

		// Special keys
		{"Enter", 0x24},
		{"Space", 0x31},
		{"Backspace", 0x33},
		{"Tab", 0x30},
		{"Escape", 0x35},
		{"Delete", 0x75},

		// Arrow keys
		{"ArrowUp", 0x7E},
		{"ArrowDown", 0x7D},
		{"ArrowLeft", 0x7B},
		{"ArrowRight", 0x7C},

		// Modifiers
		{"ShiftLeft", 0x38},
		{"ControlLeft", 0x3B},
		{"AltLeft", 0x3A},
		{"MetaLeft", 0x37},

		// Unknown key
		{"UnknownKey", -1},
		{"", -1},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			got := mapKeyCodeToMac(tt.code)
			if got != tt.expected {
				t.Errorf("mapKeyCodeToMac(%q) = 0x%X, want 0x%X", tt.code, got, tt.expected)
			}
		})
	}
}

func TestMapKeyCodeToMacCaseInsensitive(t *testing.T) {
	// Case-insensitive fallback
	got := mapKeyCodeToMac("keya")
	if got != 0x00 {
		t.Errorf("mapKeyCodeToMac(\"keya\") = %d, want 0 (case-insensitive)", got)
	}

	got = mapKeyCodeToMac("ENTER")
	if got != 0x24 {
		t.Errorf("mapKeyCodeToMac(\"ENTER\") = %d, want 0x24 (case-insensitive)", got)
	}
}

func TestNewKeyboardController(t *testing.T) {
	kc := NewKeyboardController()
	if kc == nil {
		t.Fatal("NewKeyboardController() returned nil")
	}
}
