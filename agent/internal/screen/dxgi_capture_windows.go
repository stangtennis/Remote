// +build windows

package screen

/*
#cgo LDFLAGS: -ld3d11 -ldxgi -lgdi32 -lole32

#include <d3d11.h>
#include <dxgi1_2.h>
#include <windows.h>

typedef struct {
    ID3D11Device* device;
    ID3D11DeviceContext* context;
    IDXGIOutputDuplication* duplication;
    ID3D11Texture2D* staging;
    int width;
    int height;
} DXGICapture;

DXGICapture* InitDXGI();
int CaptureDXGI(DXGICapture* cap, unsigned char* buffer, int bufferSize);
void CloseDXGI(DXGICapture* cap);
*/
import "C"
import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"unsafe"
)

type DXGICapturer struct {
	handle *C.DXGICapture
	width  int
	height int
}

func NewDXGICapturer() (*DXGICapturer, error) {
	handle := C.InitDXGI()
	if handle == nil {
		return nil, fmt.Errorf("failed to initialize DXGI capture")
	}

	return &DXGICapturer{
		handle: handle,
		width:  int(handle.width),
		height: int(handle.height),
	}, nil
}

func (c *DXGICapturer) CaptureJPEG(quality int) ([]byte, error) {
	// Calculate buffer size for BGRA (4 bytes per pixel)
	bufferSize := c.width * c.height * 4
	buffer := make([]byte, bufferSize)

	// Capture frame from DXGI
	result := C.CaptureDXGI(c.handle, (*C.uchar)(unsafe.Pointer(&buffer[0])), C.int(bufferSize))
	if result != 0 {
		// Detailed error codes
		var errMsg string
		switch result {
		case 1:
			errMsg = "timeout (no new frame)"
		case -1:
			errMsg = "invalid parameters"
		case -2:
			errMsg = "AcquireNextFrame failed"
		case -3:
			errMsg = "QueryInterface failed"
		case -4:
			errMsg = "Map failed"
		case -5:
			errMsg = "buffer too small"
		default:
			errMsg = fmt.Sprintf("unknown error code %d", result)
		}
		return nil, fmt.Errorf("DXGI capture failed: %s", errMsg)
	}

	// Convert BGRA to RGBA and create image
	img := image.NewRGBA(image.Rect(0, 0, c.width, c.height))
	for i := 0; i < len(buffer); i += 4 {
		// BGRA -> RGBA
		img.Pix[i] = buffer[i+2]   // R
		img.Pix[i+1] = buffer[i+1] // G
		img.Pix[i+2] = buffer[i]   // B
		img.Pix[i+3] = buffer[i+3] // A
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
func (c *DXGICapturer) CaptureRGBA() (*image.RGBA, error) {
	// Calculate buffer size for BGRA (4 bytes per pixel)
	bufferSize := c.width * c.height * 4
	buffer := make([]byte, bufferSize)

	// Capture frame from DXGI
	result := C.CaptureDXGI(c.handle, (*C.uchar)(unsafe.Pointer(&buffer[0])), C.int(bufferSize))
	if result != 0 {
		return nil, fmt.Errorf("DXGI capture failed: error %d", result)
	}

	// Convert BGRA to RGBA and create image
	img := image.NewRGBA(image.Rect(0, 0, c.width, c.height))
	for i := 0; i < len(buffer); i += 4 {
		img.Pix[i] = buffer[i+2]   // R
		img.Pix[i+1] = buffer[i+1] // G
		img.Pix[i+2] = buffer[i]   // B
		img.Pix[i+3] = buffer[i+3] // A
	}

	return img, nil
}

func (c *DXGICapturer) GetBounds() image.Rectangle {
	return image.Rect(0, 0, c.width, c.height)
}

func (c *DXGICapturer) GetResolution() (int, int) {
	return c.width, c.height
}

func (c *DXGICapturer) Close() error {
	if c.handle != nil {
		C.CloseDXGI(c.handle)
		c.handle = nil
	}
	return nil
}
