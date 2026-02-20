//go:build darwin

package screen

/*
#cgo LDFLAGS: -framework CoreGraphics -framework CoreFoundation -framework ImageIO
#include <CoreGraphics/CoreGraphics.h>
#include <CoreFoundation/CoreFoundation.h>
#include <ImageIO/ImageIO.h>
#include <stdlib.h>

// Capture the main display as BGRA pixel data
// Returns pixel data that caller must free, sets width/height/bytesPerRow
static unsigned char* captureDisplay(int displayIndex, int* outWidth, int* outHeight, int* outBytesPerRow) {
    // Get display ID
    CGDirectDisplayID displays[16];
    uint32_t displayCount;
    CGGetActiveDisplayList(16, displays, &displayCount);

    if (displayIndex >= (int)displayCount) {
        return NULL;
    }

    CGDirectDisplayID displayID = displays[displayIndex];
    CGImageRef image = CGDisplayCreateImage(displayID);
    if (!image) {
        return NULL;
    }

    size_t width = CGImageGetWidth(image);
    size_t height = CGImageGetHeight(image);

    // Create bitmap context to get raw RGBA pixels
    size_t bytesPerRow = width * 4;
    unsigned char* pixels = (unsigned char*)malloc(bytesPerRow * height);
    if (!pixels) {
        CGImageRelease(image);
        return NULL;
    }

    CGColorSpaceRef colorSpace = CGColorSpaceCreateDeviceRGB();
    CGContextRef ctx = CGBitmapContextCreate(
        pixels, width, height, 8, bytesPerRow,
        colorSpace,
        kCGImageAlphaPremultipliedLast | kCGBitmapByteOrder32Big // RGBA
    );
    CGColorSpaceRelease(colorSpace);

    if (!ctx) {
        free(pixels);
        CGImageRelease(image);
        return NULL;
    }

    CGContextDrawImage(ctx, CGRectMake(0, 0, width, height), image);
    CGContextRelease(ctx);
    CGImageRelease(image);

    *outWidth = (int)width;
    *outHeight = (int)height;
    *outBytesPerRow = (int)bytesPerRow;

    return pixels;
}

// Get display count
static int getDisplayCount() {
    uint32_t count;
    CGGetActiveDisplayList(0, NULL, &count);
    return (int)count;
}

// Get display bounds
static void getDisplayBounds(int displayIndex, int* x, int* y, int* w, int* h) {
    CGDirectDisplayID displays[16];
    uint32_t displayCount;
    CGGetActiveDisplayList(16, displays, &displayCount);
    if (displayIndex >= (int)displayCount) {
        *x = 0; *y = 0; *w = 0; *h = 0;
        return;
    }
    CGRect bounds = CGDisplayBounds(displays[displayIndex]);
    *x = (int)bounds.origin.x;
    *y = (int)bounds.origin.y;
    *w = (int)bounds.size.width;
    *h = (int)bounds.size.height;
}
*/
import "C"
import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"sync"
	"unsafe"

	"github.com/nfnt/resize"
)

type Capturer struct {
	displayIndex int
	bounds       image.Rectangle
	lastHash     []byte
	mu           sync.Mutex
}

func NewCapturer() (*Capturer, error) {
	return NewCapturerWithMode(false)
}

func NewCapturerForSession0() (*Capturer, error) {
	// macOS har ikke Session 0 koncept — brug normal capturer
	return NewCapturer()
}

func NewCapturerWithMode(forceGDI bool) (*Capturer, error) {
	count := int(C.getDisplayCount())
	if count == 0 {
		return nil, fmt.Errorf("no active displays found")
	}

	var x, y, w, h C.int
	C.getDisplayBounds(0, &x, &y, &w, &h)

	bounds := image.Rect(int(x), int(y), int(x)+int(w), int(y)+int(h))
	log.Printf("Quartz capturer ready: %dx%d", bounds.Dx(), bounds.Dy())

	return &Capturer{
		displayIndex: 0,
		bounds:       bounds,
	}, nil
}

func (c *Capturer) CaptureJPEG(quality int) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	img, err := c.captureRGBAInternal()
	if err != nil {
		return nil, err
	}

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

func (c *Capturer) CaptureJPEGIfChanged(quality int) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	img, err := c.captureRGBAInternal()
	if err != nil {
		return nil, fmt.Errorf("failed to capture screen: %w", err)
	}

	// Quick hash for change detection (sample every 10th pixel)
	hash := sha256.New()
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y += 10 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 10 {
			r, g, b, _ := img.At(x, y).RGBA()
			hash.Write([]byte{byte(r >> 8), byte(g >> 8), byte(b >> 8)})
		}
	}
	currentHash := hash.Sum(nil)

	if c.lastHash != nil && bytes.Equal(currentHash, c.lastHash) {
		return nil, nil
	}
	c.lastHash = currentHash

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

func (c *Capturer) Close() error {
	return nil
}

func (c *Capturer) Reinitialize(forceGDI bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var x, y, w, h C.int
	C.getDisplayBounds(C.int(c.displayIndex), &x, &y, &w, &h)

	c.bounds = image.Rect(int(x), int(y), int(x)+int(w), int(y)+int(h))
	c.lastHash = nil
	log.Printf("Reinitialized Quartz capturer: %dx%d", c.bounds.Dx(), c.bounds.Dy())
	return nil
}

func (c *Capturer) IsGDIMode() bool {
	return false
}

func (c *Capturer) SwitchDisplay(displayIndex int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := int(C.getDisplayCount())
	if displayIndex >= count {
		return fmt.Errorf("display %d not found (only %d displays)", displayIndex, count)
	}

	var x, y, w, h C.int
	C.getDisplayBounds(C.int(displayIndex), &x, &y, &w, &h)

	c.displayIndex = displayIndex
	c.bounds = image.Rect(int(x), int(y), int(x)+int(w), int(y)+int(h))
	c.lastHash = nil

	log.Printf("Switched to display %d: %dx%d", displayIndex, c.bounds.Dx(), c.bounds.Dy())
	return nil
}

func (c *Capturer) GetDisplayIndex() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.displayIndex
}

func (c *Capturer) CaptureJPEGScaled(quality int, scale float64) ([]byte, int, int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if scale < 0.25 {
		scale = 0.25
	}
	if scale > 1.0 {
		scale = 1.0
	}

	img, err := c.captureRGBAInternal()
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to capture screen: %w", err)
	}

	origWidth := img.Bounds().Dx()
	origHeight := img.Bounds().Dy()
	targetWidth := uint(float64(origWidth) * scale)
	targetHeight := uint(float64(origHeight) * scale)

	var finalImg image.Image = img
	if scale < 1.0 {
		finalImg = resize.Resize(targetWidth, targetHeight, img, resize.Bilinear)
	}

	var buf bytes.Buffer
	opts := &jpeg.Options{Quality: quality}
	if err := jpeg.Encode(&buf, finalImg, opts); err != nil {
		return nil, 0, 0, fmt.Errorf("failed to encode JPEG: %w", err)
	}

	return buf.Bytes(), int(targetWidth), int(targetHeight), nil
}

func (c *Capturer) EncodeRGBAToJPEG(img *image.RGBA, quality int, scale float64) ([]byte, int, int, error) {
	if scale < 0.25 {
		scale = 0.25
	}
	if scale > 1.0 {
		scale = 1.0
	}

	origWidth := img.Bounds().Dx()
	origHeight := img.Bounds().Dy()
	targetWidth := uint(float64(origWidth) * scale)
	targetHeight := uint(float64(origHeight) * scale)

	var finalImg image.Image = img
	if scale < 1.0 {
		finalImg = resize.Resize(targetWidth, targetHeight, img, resize.Bilinear)
	}

	var buf bytes.Buffer
	opts := &jpeg.Options{Quality: quality}
	if err := jpeg.Encode(&buf, finalImg, opts); err != nil {
		return nil, 0, 0, fmt.Errorf("failed to encode JPEG: %w", err)
	}

	return buf.Bytes(), int(targetWidth), int(targetHeight), nil
}

func (c *Capturer) CaptureRGBA() (*image.RGBA, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.captureRGBAInternal()
}

// captureRGBAInternal captures without locking (caller must hold lock)
func (c *Capturer) captureRGBAInternal() (*image.RGBA, error) {
	var width, height, bytesPerRow C.int
	pixels := C.captureDisplay(C.int(c.displayIndex), &width, &height, &bytesPerRow)
	if pixels == nil {
		return nil, fmt.Errorf("CGDisplayCreateImage failed for display %d", c.displayIndex)
	}
	defer C.free(unsafe.Pointer(pixels))

	w := int(width)
	h := int(height)
	bpr := int(bytesPerRow)

	// Copy C pixel data to Go image
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	// CGBitmapContext with kCGImageAlphaPremultipliedLast gives RGBA order
	src := C.GoBytes(unsafe.Pointer(pixels), C.int(bpr*h))

	if bpr == img.Stride {
		copy(img.Pix, src)
	} else {
		for y := 0; y < h; y++ {
			srcOff := y * bpr
			dstOff := y * img.Stride
			copy(img.Pix[dstOff:dstOff+w*4], src[srcOff:srcOff+w*4])
		}
	}

	// Update bounds
	c.bounds = image.Rect(0, 0, w, h)

	return img, nil
}
