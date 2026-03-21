//go:build turbo

package screen

/*
#cgo linux LDFLAGS: -lturbojpeg
#cgo darwin LDFLAGS: -L/opt/homebrew/opt/libjpeg-turbo/lib -L/usr/local/opt/libjpeg-turbo/lib -lturbojpeg
#cgo windows LDFLAGS: -L${SRCDIR}/../../../deps/libjpeg-turbo-win64/lib -lturbojpeg

// TurboJPEG function declarations (stable ABI — no header needed)
typedef void* tjhandle;

extern tjhandle tjInitCompress(void);
extern int tjCompress2(tjhandle handle, const unsigned char *srcBuf,
    int width, int pitch, int height, int pixelFormat,
    unsigned char **jpegBuf, unsigned long *jpegSize,
    int jpegSubsamp, int jpegQual, int flags);
extern void tjFree(unsigned char *buffer);
extern int tjDestroy(tjhandle handle);
extern char* tjGetErrorStr2(tjhandle handle);

// Pixel formats (from turbojpeg.h enum TJPF)
#define TJ_PF_RGBA 7
#define TJ_PF_BGRA 8

// Chroma subsampling
#define TJ_SAMP_420 2

// Flags
#define TJ_FLAG_FASTDCT 2048
*/
import "C"
import (
	"fmt"
	"image"
	"image/draw"
	"sync"
	"unsafe"
)

var (
	turboCompressor C.tjhandle
	turboMu         sync.Mutex
	turboInitOnce   sync.Once
)

func initTurboCompressor() {
	turboCompressor = C.tjInitCompress()
	if turboCompressor == nil {
		panic("tjInitCompress failed")
	}
}

// EncodeJPEG encodes raw pixel data to JPEG using libjpeg-turbo.
// If bgra is true, input is BGRA format; otherwise RGBA.
func EncodeJPEG(pix []byte, width, height, stride, quality int, bgra bool) ([]byte, error) {
	if len(pix) < height*stride {
		return nil, fmt.Errorf("pixel buffer too small: need %d, got %d", height*stride, len(pix))
	}

	turboInitOnce.Do(initTurboCompressor)

	turboMu.Lock()
	defer turboMu.Unlock()

	pixFmt := C.int(C.TJ_PF_RGBA)
	if bgra {
		pixFmt = C.int(C.TJ_PF_BGRA)
	}

	var jpegBuf *C.uchar
	var jpegSize C.ulong

	ret := C.tjCompress2(
		turboCompressor,
		(*C.uchar)(unsafe.Pointer(&pix[0])),
		C.int(width),
		C.int(stride),
		C.int(height),
		pixFmt,
		&jpegBuf,
		&jpegSize,
		C.int(C.TJ_SAMP_420),
		C.int(quality),
		C.int(C.TJ_FLAG_FASTDCT),
	)

	if ret != 0 {
		errStr := C.GoString(C.tjGetErrorStr2(turboCompressor))
		return nil, fmt.Errorf("tjCompress2 failed: %s", errStr)
	}
	defer C.tjFree(jpegBuf)

	result := C.GoBytes(unsafe.Pointer(jpegBuf), C.int(jpegSize))
	return result, nil
}

// EncodeImageJPEG encodes any image.Image to JPEG using libjpeg-turbo.
// Converts to RGBA if the image is not already *image.RGBA or *image.NRGBA.
func EncodeImageJPEG(img image.Image, quality int) ([]byte, error) {
	switch v := img.(type) {
	case *image.RGBA:
		return EncodeJPEG(v.Pix, v.Bounds().Dx(), v.Bounds().Dy(), v.Stride, quality, false)
	case *image.NRGBA:
		// NRGBA has same byte layout as RGBA (R,G,B,A per pixel)
		return EncodeJPEG(v.Pix, v.Bounds().Dx(), v.Bounds().Dy(), v.Stride, quality, false)
	default:
		// Convert to RGBA using draw.Draw (handles all image types)
		bounds := img.Bounds()
		rgba := image.NewRGBA(bounds)
		draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)
		return EncodeJPEG(rgba.Pix, bounds.Dx(), bounds.Dy(), rgba.Stride, quality, false)
	}
}

// IsTurboAvailable returns true when libjpeg-turbo is linked
func IsTurboAvailable() bool {
	return true
}
