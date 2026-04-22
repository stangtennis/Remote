package screen

import (
	"bytes"
	"image"
	"image/jpeg"
	"testing"

	"github.com/stangtennis/remote-agent/internal/state"
)

func TestBlackJPEG_ProducesValidJPEG(t *testing.T) {
	bounds := image.Rect(0, 0, 320, 240)
	data, err := blackJPEG(bounds, 75)
	if err != nil {
		t.Fatalf("blackJPEG: %v", err)
	}
	if len(data) < 100 {
		t.Fatalf("expected non-trivial jpeg data, got %d bytes", len(data))
	}

	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decoded jpeg invalid: %v", err)
	}
	if img.Bounds().Dx() != 320 || img.Bounds().Dy() != 240 {
		t.Fatalf("wrong dimensions: %v", img.Bounds())
	}

	// Verify center pixel is black
	r, g, b, _ := img.At(160, 120).RGBA()
	if r>>8 > 10 || g>>8 > 10 || b>>8 > 10 {
		t.Fatalf("expected near-black pixel, got rgb(%d,%d,%d)", r>>8, g>>8, b>>8)
	}
}

func TestBlackJPEG_CachesPerBoundsAndQuality(t *testing.T) {
	b1 := image.Rect(0, 0, 100, 100)
	b2 := image.Rect(0, 0, 200, 200)

	d1a, err := blackJPEG(b1, 70)
	if err != nil {
		t.Fatal(err)
	}
	d1b, err := blackJPEG(b1, 70)
	if err != nil {
		t.Fatal(err)
	}
	// Same slice reference means cache hit
	if &d1a[0] != &d1b[0] {
		t.Fatalf("expected cache hit for identical bounds+quality")
	}

	d2, err := blackJPEG(b2, 70)
	if err != nil {
		t.Fatal(err)
	}
	if len(d2) == len(d1a) && bytes.Equal(d2, d1a) {
		t.Fatalf("expected different JPEG for different bounds")
	}

	d3, err := blackJPEG(b1, 50)
	if err != nil {
		t.Fatal(err)
	}
	if &d3[0] == &d1a[0] {
		t.Fatalf("expected cache miss for different quality")
	}
}

func TestPrivacyOverride_DisabledByDefault(t *testing.T) {
	state.SetPrivacyMode(false)
	_, override := privacyOverride(image.Rect(0, 0, 100, 100), 80)
	if override {
		t.Fatal("expected no override when privacy mode is off")
	}
}

func TestPrivacyOverride_EnabledReturnsBlackJPEG(t *testing.T) {
	state.SetPrivacyMode(true)
	defer state.SetPrivacyMode(false)

	bounds := image.Rect(0, 0, 640, 480)
	data, override := privacyOverride(bounds, 80)
	if !override {
		t.Fatal("expected override when privacy mode is on")
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty black frame")
	}
	if _, err := jpeg.Decode(bytes.NewReader(data)); err != nil {
		t.Fatalf("privacy frame is not valid JPEG: %v", err)
	}
}
