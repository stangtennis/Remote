//go:build darwin

package screen

import "testing"

func TestEnumerateDisplays(t *testing.T) {
	displays := EnumerateDisplays()
	// On headless CI: 0 displays. On real Mac: 1+
	t.Logf("Found %d displays", len(displays))

	for _, d := range displays {
		t.Logf("Display %d: %s (%dx%d at %d,%d, primary=%v)",
			d.Index, d.Name, d.Width, d.Height, d.OffsetX, d.OffsetY, d.Primary)

		if d.Width <= 0 || d.Height <= 0 {
			t.Errorf("Display %d has invalid dimensions: %dx%d", d.Index, d.Width, d.Height)
		}
	}
}

func TestNewCapturer(t *testing.T) {
	cap, err := NewCapturer()
	if err != nil {
		// Expected on headless CI — no displays
		t.Logf("NewCapturer() error (expected on headless CI): %v", err)
		return
	}
	defer cap.Close()

	w, h := cap.GetResolution()
	t.Logf("Capturer resolution: %dx%d", w, h)

	if w <= 0 || h <= 0 {
		t.Errorf("Invalid resolution: %dx%d", w, h)
	}

	bounds := cap.GetBounds()
	if bounds.Dx() != w || bounds.Dy() != h {
		t.Errorf("GetBounds() = %v, inconsistent with GetResolution() %dx%d", bounds, w, h)
	}
}

func TestNewCapturerForSession0(t *testing.T) {
	// macOS Session 0 capturer should be same as normal
	cap, err := NewCapturerForSession0()
	if err != nil {
		t.Logf("NewCapturerForSession0() error (expected on headless CI): %v", err)
		return
	}
	defer cap.Close()
	t.Log("NewCapturerForSession0() succeeded")
}

func TestCapturerIsGDIMode(t *testing.T) {
	cap, err := NewCapturer()
	if err != nil {
		t.Skip("No display available")
	}
	defer cap.Close()

	if cap.IsGDIMode() {
		t.Error("IsGDIMode() should always return false on macOS")
	}
}

func TestCapturerGetDisplayIndex(t *testing.T) {
	cap, err := NewCapturer()
	if err != nil {
		t.Skip("No display available")
	}
	defer cap.Close()

	idx := cap.GetDisplayIndex()
	if idx != 0 {
		t.Errorf("GetDisplayIndex() = %d, want 0 (default)", idx)
	}
}

func TestCaptureJPEG(t *testing.T) {
	cap, err := NewCapturer()
	if err != nil {
		t.Skip("No display available")
	}
	defer cap.Close()

	data, err := cap.CaptureJPEG(75)
	if err != nil {
		t.Fatalf("CaptureJPEG() error = %v", err)
	}

	if len(data) < 100 {
		t.Errorf("CaptureJPEG() returned %d bytes, expected more", len(data))
	}

	// Verify JPEG magic bytes
	if data[0] != 0xFF || data[1] != 0xD8 {
		t.Errorf("CaptureJPEG() data doesn't start with JPEG magic (got %02x %02x)", data[0], data[1])
	}
}

func TestCaptureRGBA(t *testing.T) {
	cap, err := NewCapturer()
	if err != nil {
		t.Skip("No display available")
	}
	defer cap.Close()

	img, err := cap.CaptureRGBA()
	if err != nil {
		t.Fatalf("CaptureRGBA() error = %v", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		t.Errorf("CaptureRGBA() image has invalid dimensions: %v", bounds)
	}
	t.Logf("Captured RGBA image: %dx%d", bounds.Dx(), bounds.Dy())
}
