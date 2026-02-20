//go:build darwin

package desktop

import "testing"

func TestGetCurrentDesktop(t *testing.T) {
	name, err := GetCurrentDesktop()
	if err != nil {
		t.Fatalf("GetCurrentDesktop() error = %v", err)
	}
	if name != "Default" {
		t.Errorf("GetCurrentDesktop() = %q, want \"Default\"", name)
	}
}

func TestGetInputDesktop(t *testing.T) {
	name, err := GetInputDesktop()
	if err != nil {
		t.Fatalf("GetInputDesktop() error = %v", err)
	}
	if name != "Default" {
		t.Errorf("GetInputDesktop() = %q, want \"Default\"", name)
	}
}

func TestGetDesktopType(t *testing.T) {
	tests := []struct {
		name     string
		expected DesktopType
	}{
		{"Default", DesktopDefault},
		{"ScreenSaver", DesktopScreenSaver},
		{"Unknown", DesktopUnknown},
		{"", DesktopUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetDesktopType(tt.name)
			if got != tt.expected {
				t.Errorf("GetDesktopType(%q) = %d, want %d", tt.name, got, tt.expected)
			}
		})
	}
}

func TestIsOnLoginScreen(t *testing.T) {
	// Stub should always return false
	if IsOnLoginScreen() {
		t.Error("IsOnLoginScreen() = true, want false (stub)")
	}
}

func TestSwitchToInputDesktop(t *testing.T) {
	err := SwitchToInputDesktop()
	if err != nil {
		t.Errorf("SwitchToInputDesktop() error = %v, want nil", err)
	}
}
