//go:build windows
// +build windows

package screen

/*
#cgo LDFLAGS: -lgdi32 -luser32

#include <windows.h>
#include <stdio.h>

typedef struct {
    int width;
    int height;
    HDC screenDC;
    HDC memDC;
    HBITMAP bitmap;
    BITMAPINFO bmi;
} GDICapture;

// Switch to input desktop before capture (for Session 0 support)
int SwitchToInputDesktopGDI() {
    HDESK hDesk = OpenInputDesktop(0, TRUE,
        DESKTOP_READOBJECTS | DESKTOP_CREATEWINDOW | DESKTOP_CREATEMENU |
        DESKTOP_HOOKCONTROL | DESKTOP_JOURNALRECORD | DESKTOP_JOURNALPLAYBACK |
        DESKTOP_ENUMERATE | DESKTOP_WRITEOBJECTS | DESKTOP_SWITCHDESKTOP);
    if (!hDesk) {
        return 0;
    }

    BOOL result = SetThreadDesktop(hDesk);
    // Don't close the desktop handle - we need it
    return result ? 1 : 0;
}

GDICapture* InitGDI() {
    // Switch to input desktop first (required for Session 0 / login screen)
    SwitchToInputDesktopGDI();

    // Get screen dimensions
    int width = GetSystemMetrics(SM_CXSCREEN);
    int height = GetSystemMetrics(SM_CYSCREEN);

    if (width == 0 || height == 0) {
        return NULL;
    }

    // Create screen DC using DISPLAY driver (more reliable than GetDC)
    HDC screenDC = CreateDC("DISPLAY", NULL, NULL, NULL);
    if (!screenDC) {
        // Fallback to GetDC
        screenDC = GetDC(NULL);
        if (!screenDC) {
            return NULL;
        }
    }

    // Create compatible DC
    HDC memDC = CreateCompatibleDC(screenDC);
    if (!memDC) {
        DeleteDC(screenDC);  // Use DeleteDC instead of ReleaseDC for CreateDC
        return NULL;
    }

    // Create compatible bitmap
    HBITMAP bitmap = CreateCompatibleBitmap(screenDC, width, height);
    if (!bitmap) {
        DeleteDC(memDC);
        DeleteDC(screenDC);
        return NULL;
    }

    // Select bitmap into DC
    SelectObject(memDC, bitmap);

    // Allocate structure
    GDICapture* cap = (GDICapture*)malloc(sizeof(GDICapture));
    cap->width = width;
    cap->height = height;
    cap->screenDC = screenDC;
    cap->memDC = memDC;
    cap->bitmap = bitmap;

    // Setup BITMAPINFO for GetDIBits
    ZeroMemory(&cap->bmi, sizeof(BITMAPINFO));
    cap->bmi.bmiHeader.biSize = sizeof(BITMAPINFOHEADER);
    cap->bmi.bmiHeader.biWidth = width;
    cap->bmi.bmiHeader.biHeight = -height; // Top-down
    cap->bmi.bmiHeader.biPlanes = 1;
    cap->bmi.bmiHeader.biBitCount = 32; // BGRA
    cap->bmi.bmiHeader.biCompression = BI_RGB;

    return cap;
}

int CaptureGDI(GDICapture* cap, unsigned char* buffer, int bufferSize) {
    if (!cap || !buffer) {
        return -1;
    }

    // Switch to input desktop before each capture (handles desktop switches)
    SwitchToInputDesktopGDI();

    // Always recreate screen DC after desktop switch.
    // In Session 0, the old DC points to the empty Session 0 desktop.
    // After SwitchToInputDesktopGDI(), we need a fresh DC for the user's desktop.
    if (cap->screenDC) {
        DeleteDC(cap->screenDC);
        cap->screenDC = NULL;
    }
    cap->screenDC = CreateDC("DISPLAY", NULL, NULL, NULL);
    if (!cap->screenDC) {
        cap->screenDC = GetDC(NULL);
        if (!cap->screenDC) {
            return -3;
        }
    }

    // Check if resolution changed (can happen when switching between desktops)
    int width = GetSystemMetrics(SM_CXSCREEN);
    int height = GetSystemMetrics(SM_CYSCREEN);
    if (width > 0 && height > 0 && (width != cap->width || height != cap->height)) {
        // Resolution changed - recreate memDC and bitmap
        if (cap->bitmap) { DeleteObject(cap->bitmap); cap->bitmap = NULL; }
        if (cap->memDC) { DeleteDC(cap->memDC); cap->memDC = NULL; }

        cap->width = width;
        cap->height = height;

        cap->memDC = CreateCompatibleDC(cap->screenDC);
        if (!cap->memDC) return -3;

        cap->bitmap = CreateCompatibleBitmap(cap->screenDC, width, height);
        if (!cap->bitmap) return -3;

        SelectObject(cap->memDC, cap->bitmap);

        cap->bmi.bmiHeader.biWidth = width;
        cap->bmi.bmiHeader.biHeight = -height;
    }

    int expectedSize = cap->width * cap->height * 4;
    if (bufferSize < expectedSize) {
        return -2;
    }

    // Direct BitBlt from screen DC to memory DC
    // SRCCOPY | CAPTUREBLT ensures we capture layered windows
    if (!BitBlt(cap->memDC, 0, 0, cap->width, cap->height,
                cap->screenDC, 0, 0, SRCCOPY | CAPTUREBLT)) {
        return -3;
    }

    // Get bitmap bits
    if (!GetDIBits(cap->memDC, cap->bitmap, 0, cap->height,
                   buffer, &cap->bmi, DIB_RGB_COLORS)) {
        return -4;
    }

    return 0;
}

void CloseGDI(GDICapture* cap) {
    if (!cap) return;

    if (cap->bitmap) DeleteObject(cap->bitmap);
    if (cap->memDC) DeleteDC(cap->memDC);
    if (cap->screenDC) DeleteDC(cap->screenDC);  // Use DeleteDC for CreateDC

    free(cap);
}
*/
import "C"
import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"unsafe"
)

type GDICapturer struct {
	handle unsafe.Pointer
	width  int
	height int
}

func NewGDICapturer() (*GDICapturer, error) {
	handle := C.InitGDI()
	if handle == nil {
		return nil, fmt.Errorf("failed to initialize GDI capture")
	}

	cap := (*C.GDICapture)(handle)

	return &GDICapturer{
		handle: unsafe.Pointer(handle),
		width:  int(cap.width),
		height: int(cap.height),
	}, nil
}

func (c *GDICapturer) refreshDimensions() {
	cap := (*C.GDICapture)(c.handle)
	c.width = int(cap.width)
	c.height = int(cap.height)
}

func (c *GDICapturer) CaptureJPEG(quality int) ([]byte, error) {
	// Allocate buffer for max possible BGRA data (use current dimensions)
	// C code may update dimensions after desktop switch, so use generous buffer
	bufferSize := c.width * c.height * 4
	if bufferSize == 0 {
		// Session 0 may start with 0x0 - try a 1920x1080 buffer
		bufferSize = 1920 * 1080 * 4
	}
	buffer := make([]byte, bufferSize)

	// Capture screen
	result := C.CaptureGDI((*C.GDICapture)(c.handle), (*C.uchar)(unsafe.Pointer(&buffer[0])), C.int(bufferSize))

	// Update Go dimensions from C struct (may have changed after desktop switch)
	c.refreshDimensions()

	if result == -2 {
		// Buffer too small after resolution change - retry with new size
		bufferSize = c.width * c.height * 4
		buffer = make([]byte, bufferSize)
		result = C.CaptureGDI((*C.GDICapture)(c.handle), (*C.uchar)(unsafe.Pointer(&buffer[0])), C.int(bufferSize))
		c.refreshDimensions()
	}

	if result != 0 {
		var errMsg string
		switch result {
		case -1:
			errMsg = "invalid parameters"
		case -2:
			errMsg = "buffer too small"
		case -3:
			errMsg = "BitBlt failed - screen may be locked or inaccessible"
		case -4:
			errMsg = "GetDIBits failed"
		case -5:
			errMsg = "GetDesktopWindow failed"
		default:
			errMsg = fmt.Sprintf("unknown error %d", result)
		}
		return nil, fmt.Errorf("GDI capture failed: %s (try running as Administrator/SYSTEM)", errMsg)
	}

	// Convert BGRA to RGBA
	img := image.NewRGBA(image.Rect(0, 0, c.width, c.height))
	pixelBytes := c.width * c.height * 4
	for i := 0; i < pixelBytes; i += 4 {
		img.Pix[i] = buffer[i+2]   // R
		img.Pix[i+1] = buffer[i+1] // G
		img.Pix[i+2] = buffer[i]   // B
		img.Pix[i+3] = 255         // A (opaque)
	}

	// Encode as JPEG
	var buf bytes.Buffer
	opts := &jpeg.Options{Quality: quality}
	if err := jpeg.Encode(&buf, img, opts); err != nil {
		return nil, fmt.Errorf("failed to encode JPEG: %w", err)
	}

	return buf.Bytes(), nil
}

// CaptureRGBA captures the screen as RGBA image (for dirty region detection)
func (c *GDICapturer) CaptureRGBA() (*image.RGBA, error) {
	// Allocate buffer for BGRA data
	bufferSize := c.width * c.height * 4
	if bufferSize == 0 {
		bufferSize = 1920 * 1080 * 4
	}
	buffer := make([]byte, bufferSize)

	// Capture screen
	result := C.CaptureGDI((*C.GDICapture)(c.handle), (*C.uchar)(unsafe.Pointer(&buffer[0])), C.int(bufferSize))

	// Update Go dimensions from C struct (may have changed after desktop switch)
	c.refreshDimensions()

	if result == -2 {
		// Buffer too small after resolution change - retry with new size
		bufferSize = c.width * c.height * 4
		buffer = make([]byte, bufferSize)
		result = C.CaptureGDI((*C.GDICapture)(c.handle), (*C.uchar)(unsafe.Pointer(&buffer[0])), C.int(bufferSize))
		c.refreshDimensions()
	}

	if result != 0 {
		return nil, fmt.Errorf("GDI capture failed: error %d", result)
	}

	// Convert BGRA to RGBA
	img := image.NewRGBA(image.Rect(0, 0, c.width, c.height))
	pixelBytes := c.width * c.height * 4
	for i := 0; i < pixelBytes; i += 4 {
		img.Pix[i] = buffer[i+2]   // R
		img.Pix[i+1] = buffer[i+1] // G
		img.Pix[i+2] = buffer[i]   // B
		img.Pix[i+3] = 255         // A (opaque)
	}

	return img, nil
}

// CaptureBGRA captures the screen as raw BGRA bytes (used by Session 0 pipe helper).
// Returns the raw pixel data without BGRA→RGBA conversion.
func (c *GDICapturer) CaptureBGRA() ([]byte, int, int, error) {
	bufferSize := c.width * c.height * 4
	if bufferSize == 0 {
		bufferSize = 1920 * 1080 * 4
	}
	buffer := make([]byte, bufferSize)

	result := C.CaptureGDI((*C.GDICapture)(c.handle), (*C.uchar)(unsafe.Pointer(&buffer[0])), C.int(bufferSize))
	c.refreshDimensions()

	if result == -2 {
		bufferSize = c.width * c.height * 4
		buffer = make([]byte, bufferSize)
		result = C.CaptureGDI((*C.GDICapture)(c.handle), (*C.uchar)(unsafe.Pointer(&buffer[0])), C.int(bufferSize))
		c.refreshDimensions()
	}

	if result != 0 {
		return nil, 0, 0, fmt.Errorf("GDI capture failed: error %d", result)
	}

	return buffer[:c.width*c.height*4], c.width, c.height, nil
}

func (c *GDICapturer) GetBounds() image.Rectangle {
	return image.Rect(0, 0, c.width, c.height)
}

func (c *GDICapturer) GetResolution() (int, int) {
	return c.width, c.height
}

func (c *GDICapturer) Close() error {
	if c.handle != nil {
		C.CloseGDI((*C.GDICapture)(c.handle))
		c.handle = nil
	}
	return nil
}
