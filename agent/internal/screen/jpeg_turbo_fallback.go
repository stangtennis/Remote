//go:build !turbo

package screen

import (
	"bytes"
	"image"
	"image/jpeg"
)

// EncodeJPEG encodes raw pixel data to JPEG using the standard library.
// If bgra is true, input is BGRA format and will be converted to RGBA first.
func EncodeJPEG(pix []byte, width, height, stride, quality int, bgra bool) ([]byte, error) {
	return encodeJPEGStandard(pix, width, height, stride, quality, bgra)
}

// EncodeImageJPEG encodes any image.Image to JPEG using the standard library.
func EncodeImageJPEG(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// IsTurboAvailable returns false when libjpeg-turbo is not linked
func IsTurboAvailable() bool {
	return false
}
