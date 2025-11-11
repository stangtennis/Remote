package screen

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	"image/jpeg"

	"github.com/kbinani/screenshot"
	"github.com/nfnt/resize"
)

type Capturer struct {
	displayIndex int
	bounds       image.Rectangle
	lastHash     []byte // Hash of last frame for change detection
	dxgiCapturer *DXGICapturer // DXGI capturer if available (works better with RDP)
}

func NewCapturer() (*Capturer, error) {
	// Try DXGI first (works better with RDP and modern Windows)
	dxgi, err := NewDXGICapturer()
	if err == nil {
		// DXGI available - wrap it in a Capturer interface
		bounds := dxgi.GetBounds()
		return &Capturer{
			displayIndex: 0,
			bounds:       bounds,
			dxgiCapturer: dxgi,
		}, nil
	}
	
	// Fallback to screenshot library (GDI-based)
	n := screenshot.NumActiveDisplays()
	if n == 0 {
		return nil, fmt.Errorf("no active displays found")
	}

	bounds := screenshot.GetDisplayBounds(0)

	return &Capturer{
		displayIndex: 0,
		bounds:       bounds,
	}, nil
}

func (c *Capturer) CaptureJPEG(quality int) ([]byte, error) {
	// Use DXGI if available (better for RDP)
	if c.dxgiCapturer != nil {
		return c.dxgiCapturer.CaptureJPEG(quality)
	}
	
	// Fallback to screenshot library
	img, err := screenshot.CaptureRect(c.bounds)
	if err != nil {
		return nil, fmt.Errorf("failed to capture screen: %w", err)
	}

	// Keep full resolution up to 4K (3840px) for MAXIMUM quality
	var finalImg image.Image = img
	maxWidth := uint(3840)
	if img.Bounds().Dx() > int(maxWidth) {
		// Use Lanczos3 for highest quality scaling
		finalImg = resize.Resize(maxWidth, 0, img, resize.Lanczos3)
	}

	// Encode as JPEG
	var buf bytes.Buffer
	opts := &jpeg.Options{Quality: quality}
	
	if err := jpeg.Encode(&buf, finalImg, opts); err != nil {
		return nil, fmt.Errorf("failed to encode JPEG: %w", err)
	}

	return buf.Bytes(), nil
}

// CaptureJPEGIfChanged only returns a frame if the screen has changed
// Returns (nil, nil) if no change detected
func (c *Capturer) CaptureJPEGIfChanged(quality int) ([]byte, error) {
	// Use DXGI if available (better for RDP)
	if c.dxgiCapturer != nil {
		// For DXGI, just capture every time (it's fast enough)
		return c.dxgiCapturer.CaptureJPEG(quality)
	}
	
	// Fallback to screenshot library with change detection
	img, err := screenshot.CaptureRect(c.bounds)
	if err != nil {
		return nil, fmt.Errorf("failed to capture screen: %w", err)
	}

	// Create a quick hash to detect changes (sample every 10th pixel for speed)
	hash := sha256.New()
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y += 10 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 10 {
			r, g, b, _ := img.At(x, y).RGBA()
			hash.Write([]byte{byte(r >> 8), byte(g >> 8), byte(b >> 8)})
		}
	}
	currentHash := hash.Sum(nil)

	// Compare with last frame
	if c.lastHash != nil && bytes.Equal(currentHash, c.lastHash) {
		// No change detected
		return nil, nil
	}
	c.lastHash = currentHash

	// Screen changed - encode and return
	var finalImg image.Image = img
	maxWidth := uint(3840)
	if img.Bounds().Dx() > int(maxWidth) {
		finalImg = resize.Resize(maxWidth, 0, img, resize.Lanczos3)
	}

	var buf bytes.Buffer
	opts := &jpeg.Options{Quality: quality}
	if err := jpeg.Encode(&buf, finalImg, opts); err != nil {
		return nil, fmt.Errorf("failed to encode JPEG: %w", err)
	}

	return buf.Bytes(), nil
}

func (c *Capturer) GetBounds() image.Rectangle {
	return c.bounds
}

func (c *Capturer) GetResolution() (int, int) {
	return c.bounds.Dx(), c.bounds.Dy()
}
