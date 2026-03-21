//go:build !turbo

package screen

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
)

// EncodeJPEG encodes raw pixel data to JPEG using the standard library.
// If bgra is true, input is BGRA format and will be converted to RGBA first.
func EncodeJPEG(pix []byte, width, height, stride, quality int, bgra bool) ([]byte, error) {
	if len(pix) < height*stride {
		return nil, fmt.Errorf("pixel buffer too small: need %d, got %d", height*stride, len(pix))
	}

	img := &image.RGBA{
		Pix:    make([]byte, height*width*4),
		Stride: width * 4,
		Rect:   image.Rect(0, 0, width, height),
	}

	if bgra {
		// Convert BGRA to RGBA
		for y := 0; y < height; y++ {
			srcOff := y * stride
			dstOff := y * img.Stride
			for x := 0; x < width; x++ {
				si := srcOff + x*4
				di := dstOff + x*4
				img.Pix[di] = pix[si+2]   // R
				img.Pix[di+1] = pix[si+1] // G
				img.Pix[di+2] = pix[si]   // B
				img.Pix[di+3] = pix[si+3] // A
			}
		}
	} else {
		// RGBA — copy rows (handles stride mismatch)
		for y := 0; y < height; y++ {
			srcOff := y * stride
			dstOff := y * img.Stride
			copy(img.Pix[dstOff:dstOff+width*4], pix[srcOff:srcOff+width*4])
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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
