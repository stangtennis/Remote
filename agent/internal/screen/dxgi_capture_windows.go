//go:build windows

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

typedef struct {
    int index;
    int width;
    int height;
    int offsetX;
    int offsetY;
    int isPrimary;
    char name[64];
} MonitorInfoC;

DXGICapture* InitDXGI();
DXGICapture* InitDXGIForOutput(int outputIndex);
int EnumDXGIOutputs(MonitorInfoC* infos, int maxCount);
int CaptureDXGI(DXGICapture* cap, unsigned char* buffer, int bufferSize);
void CloseDXGI(DXGICapture* cap);
*/
import "C"
import (
	"fmt"
	"image"
	"unsafe"
)

type DXGICapturer struct {
	handle    *C.DXGICapture
	width     int
	height    int
	lastFrame *image.RGBA // Cache last frame for timeout cases
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

// NewDXGICapturerForOutput creates a DXGI capturer for a specific monitor output
func NewDXGICapturerForOutput(outputIndex int) (*DXGICapturer, error) {
	handle := C.InitDXGIForOutput(C.int(outputIndex))
	if handle == nil {
		return nil, fmt.Errorf("failed to initialize DXGI capture for output %d", outputIndex)
	}

	return &DXGICapturer{
		handle: handle,
		width:  int(handle.width),
		height: int(handle.height),
	}, nil
}

// MonitorInfo describes a connected display
type MonitorInfo struct {
	Index   int
	Name    string
	Width   int
	Height  int
	OffsetX int
	OffsetY int
	Primary bool
}

// EnumerateDisplays returns info about all connected monitors via DXGI
func EnumerateDisplays() []MonitorInfo {
	const maxMonitors = 16
	var infos [maxMonitors]C.MonitorInfoC

	count := int(C.EnumDXGIOutputs(&infos[0], C.int(maxMonitors)))
	if count <= 0 {
		return nil
	}

	result := make([]MonitorInfo, count)
	for i := 0; i < count; i++ {
		result[i] = MonitorInfo{
			Index:   int(infos[i].index),
			Name:    C.GoString(&infos[i].name[0]),
			Width:   int(infos[i].width),
			Height:  int(infos[i].height),
			OffsetX: int(infos[i].offsetX),
			OffsetY: int(infos[i].offsetY),
			Primary: infos[i].isPrimary != 0,
		}
	}
	return result
}

func (c *DXGICapturer) CaptureJPEG(quality int) ([]byte, error) {
	// Calculate buffer size for BGRA (4 bytes per pixel)
	bufferSize := c.width * c.height * 4
	buffer := getPixelBuffer(bufferSize)
	defer putPixelBuffer(buffer)

	// Capture frame from DXGI
	result := C.CaptureDXGI(c.handle, (*C.uchar)(unsafe.Pointer(&buffer[0])), C.int(bufferSize))
	if result != 0 {
		// Timeout (code 1) means no new frame - use cached frame if available
		if result == 1 && c.lastFrame != nil {
			data, err := EncodeJPEG(c.lastFrame.Pix, c.lastFrame.Bounds().Dx(), c.lastFrame.Bounds().Dy(), c.lastFrame.Stride, quality, false)
			if err != nil {
				return nil, fmt.Errorf("failed to encode cached JPEG: %w", err)
			}
			return data, nil
		}

		// Detailed error codes for real errors
		var errMsg string
		switch result {
		case 1:
			errMsg = "timeout (no new frame, no cache)"
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

	// Encode BGRA directly to JPEG — no pixel swap needed!
	data, err := EncodeJPEG(buffer, c.width, c.height, c.width*4, quality, true)
	if err != nil {
		return nil, fmt.Errorf("failed to encode JPEG: %w", err)
	}

	// Update cached RGBA frame for timeout cases (BGRA→RGBA conversion only for cache)
	if c.lastFrame == nil || c.lastFrame.Bounds().Dx() != c.width || c.lastFrame.Bounds().Dy() != c.height {
		c.lastFrame = image.NewRGBA(image.Rect(0, 0, c.width, c.height))
	}
	for i := 0; i < bufferSize; i += 4 {
		c.lastFrame.Pix[i] = buffer[i+2]   // R
		c.lastFrame.Pix[i+1] = buffer[i+1] // G
		c.lastFrame.Pix[i+2] = buffer[i]   // B
		c.lastFrame.Pix[i+3] = buffer[i+3] // A
	}

	return data, nil
}

// CaptureRGBA captures the screen as RGBA image (for dirty region detection)
func (c *DXGICapturer) CaptureRGBA() (*image.RGBA, error) {
	// Calculate buffer size for BGRA (4 bytes per pixel)
	bufferSize := c.width * c.height * 4
	buffer := make([]byte, bufferSize)

	// Capture frame from DXGI
	result := C.CaptureDXGI(c.handle, (*C.uchar)(unsafe.Pointer(&buffer[0])), C.int(bufferSize))
	if result != 0 {
		// Timeout (code 1) means no new frame - return cached frame if available
		if result == 1 && c.lastFrame != nil {
			return c.lastFrame, nil
		}
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

	// Cache frame for timeout cases
	c.lastFrame = img

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

// Reinitialize recreates the DXGI capture after desktop change (screensaver, lock, etc.)
func (c *DXGICapturer) Reinitialize() error {
	// Close existing handle
	if c.handle != nil {
		C.CloseDXGI(c.handle)
		c.handle = nil
	}

	// Create new handle
	handle := C.InitDXGI()
	if handle == nil {
		return fmt.Errorf("failed to reinitialize DXGI capture")
	}

	c.handle = handle
	c.width = int(handle.width)
	c.height = int(handle.height)
	// Keep lastFrame cache for smooth transition

	return nil
}

// NeedsReinit returns true if error code indicates DXGI needs reinitialization
func NeedsReinit(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// Error -2 is AcquireNextFrame failed (desktop changed)
	return contains(errStr, "AcquireNextFrame failed") || contains(errStr, "error -2")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
