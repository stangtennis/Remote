//go:build darwin

package input

import "testing"

func TestNewMouseController(t *testing.T) {
	mc := NewMouseController(1920, 1080)
	if mc == nil {
		t.Fatal("NewMouseController() returned nil")
	}
	if mc.screenWidth != 1920 {
		t.Errorf("screenWidth = %d, want 1920", mc.screenWidth)
	}
	if mc.screenHeight != 1080 {
		t.Errorf("screenHeight = %d, want 1080", mc.screenHeight)
	}
}

func TestSetMonitorOffset(t *testing.T) {
	mc := NewMouseController(1920, 1080)
	mc.SetMonitorOffset(100, 200)
	if mc.offsetX != 100 {
		t.Errorf("offsetX = %d, want 100", mc.offsetX)
	}
	if mc.offsetY != 200 {
		t.Errorf("offsetY = %d, want 200", mc.offsetY)
	}
}

func TestSetResolution(t *testing.T) {
	mc := NewMouseController(1920, 1080)
	mc.SetResolution(3840, 2160)
	if mc.screenWidth != 3840 {
		t.Errorf("screenWidth = %d, want 3840", mc.screenWidth)
	}
	if mc.screenHeight != 2160 {
		t.Errorf("screenHeight = %d, want 2160", mc.screenHeight)
	}
}

func TestClamp(t *testing.T) {
	tests := []struct {
		val, min, max, expected int
	}{
		{50, 0, 100, 50},
		{-10, 0, 100, 0},
		{150, 0, 100, 100},
		{0, 0, 100, 0},
		{100, 0, 100, 100},
	}

	for _, tt := range tests {
		got := clamp(tt.val, tt.min, tt.max)
		if got != tt.expected {
			t.Errorf("clamp(%d, %d, %d) = %d, want %d", tt.val, tt.min, tt.max, got, tt.expected)
		}
	}
}

func TestClampFloat(t *testing.T) {
	tests := []struct {
		val, min, max, expected float64
	}{
		{50.5, 0, 100, 50.5},
		{-10.0, 0, 100, 0},
		{150.0, 0, 100, 100},
		{0, 0, 100, 0},
		{100.0, 0, 100, 100.0},
	}

	for _, tt := range tests {
		got := clampFloat(tt.val, tt.min, tt.max)
		if got != tt.expected {
			t.Errorf("clampFloat(%f, %f, %f) = %f, want %f", tt.val, tt.min, tt.max, got, tt.expected)
		}
	}
}

func TestCursorHidden(t *testing.T) {
	mc := NewMouseController(1920, 1080)

	if mc.IsCursorHidden() {
		t.Error("cursor should not be hidden initially")
	}

	mc.HideCursor()
	if !mc.IsCursorHidden() {
		t.Error("cursor should be hidden after HideCursor()")
	}

	mc.ShowCursor()
	if mc.IsCursorHidden() {
		t.Error("cursor should not be hidden after ShowCursor()")
	}
}
