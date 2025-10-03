package screen

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"

	"github.com/kbinani/screenshot"
	"github.com/nfnt/resize"
)

type Capturer struct {
	displayIndex int
	bounds       image.Rectangle
}

func NewCapturer() (*Capturer, error) {
	// Get primary display bounds
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
	// Capture screenshot
	img, err := screenshot.CaptureRect(c.bounds)
	if err != nil {
		return nil, fmt.Errorf("failed to capture screen: %w", err)
	}

	// Resize to max 1280 width for better performance
	var finalImg image.Image = img
	maxWidth := uint(1280)
	if img.Bounds().Dx() > int(maxWidth) {
		// Use Bilinear for faster resizing (vs Lanczos3)
		finalImg = resize.Resize(maxWidth, 0, img, resize.Bilinear)
	}

	// Encode as JPEG
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
