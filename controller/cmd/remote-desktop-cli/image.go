package main

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"

	_ "image/jpeg"

	"github.com/nfnt/resize"
)

// downscaleJPEG takes a JPEG byte slice and returns a downscaled version
func downscaleJPEG(jpegData []byte, maxWidth int, quality int) ([]byte, int, int, error) {
	img, _, err := image.Decode(bytes.NewReader(jpegData))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to decode JPEG: %w", err)
	}

	bounds := img.Bounds()
	origW := bounds.Dx()

	var result image.Image
	if origW > maxWidth {
		result = resize.Resize(uint(maxWidth), 0, img, resize.Lanczos3)
	} else {
		result = img
	}

	resultBounds := result.Bounds()

	var buf bytes.Buffer
	err = jpeg.Encode(&buf, result, &jpeg.Options{Quality: quality})
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to encode JPEG: %w", err)
	}

	return buf.Bytes(), resultBounds.Dx(), resultBounds.Dy(), nil
}
