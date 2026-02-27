package main

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"

	// Register JPEG decoder
	_ "image/jpeg"

	"github.com/nfnt/resize"
)

// downscaleJPEG takes a JPEG byte slice and returns a downscaled version
// maxWidth is the maximum width in pixels, quality is JPEG quality (1-100)
func downscaleJPEG(jpegData []byte, maxWidth int, quality int) ([]byte, int, int, error) {
	// Decode JPEG
	img, _, err := image.Decode(bytes.NewReader(jpegData))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to decode JPEG: %w", err)
	}

	bounds := img.Bounds()
	origW := bounds.Dx()

	// Only downscale if wider than maxWidth
	var result image.Image
	if origW > maxWidth {
		// Resize preserving aspect ratio
		result = resize.Resize(uint(maxWidth), 0, img, resize.Lanczos3)
	} else {
		result = img
	}

	resultBounds := result.Bounds()

	// Encode to JPEG with specified quality
	var buf bytes.Buffer
	err = jpeg.Encode(&buf, result, &jpeg.Options{Quality: quality})
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to encode JPEG: %w", err)
	}

	return buf.Bytes(), resultBounds.Dx(), resultBounds.Dy(), nil
}
