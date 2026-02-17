package screen

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"sync"

	"github.com/kbinani/screenshot"
	"github.com/nfnt/resize"
)

type Capturer struct {
	displayIndex int
	bounds       image.Rectangle
	lastHash     []byte // Hash of last frame for change detection
	dxgiCapturer *DXGICapturer // DXGI capturer if available (works better with RDP)
	gdiCapturer  *GDICapturer  // GDI capturer for Session 0 / login screen
	useGDI       bool          // Force GDI mode (for Session 0)
	mu           sync.Mutex    // Protect capturer switching
}

func NewCapturer() (*Capturer, error) {
	return NewCapturerWithMode(false)
}

// NewCapturerForSession0 creates a capturer specifically for Session 0 (login screen)
// Uses GDI capture which works better in Session 0
func NewCapturerForSession0() (*Capturer, error) {
	return NewCapturerWithMode(true)
}

// NewCapturerWithMode creates a capturer with optional GDI-only mode
func NewCapturerWithMode(forceGDI bool) (*Capturer, error) {
	// For Session 0 / login screen, use GDI directly
	if forceGDI {
		log.Println("🔧 Initializing GDI capturer for Session 0...")
		gdi, err := NewGDICapturer()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize GDI capturer: %w", err)
		}
		bounds := gdi.GetBounds()
		log.Printf("✅ GDI capturer ready: %dx%d", bounds.Dx(), bounds.Dy())
		return &Capturer{
			displayIndex: 0,
			bounds:       bounds,
			gdiCapturer:  gdi,
			useGDI:       true,
		}, nil
	}

	// Try DXGI first (works better with RDP and modern Windows)
	dxgi, err := NewDXGICapturer()
	if err == nil {
		// DXGI available - wrap it in a Capturer interface
		bounds := dxgi.GetBounds()
		log.Printf("✅ DXGI capturer ready: %dx%d", bounds.Dx(), bounds.Dy())
		return &Capturer{
			displayIndex: 0,
			bounds:       bounds,
			dxgiCapturer: dxgi,
		}, nil
	}
	log.Printf("⚠️  DXGI not available: %v, trying GDI...", err)

	// Try GDI next (works in more scenarios including Session 0)
	gdi, err := NewGDICapturer()
	if err == nil {
		bounds := gdi.GetBounds()
		log.Printf("✅ GDI capturer ready: %dx%d", bounds.Dx(), bounds.Dy())
		return &Capturer{
			displayIndex: 0,
			bounds:       bounds,
			gdiCapturer:  gdi,
			useGDI:       true,
		}, nil
	}
	log.Printf("⚠️  GDI not available: %v, trying screenshot library...", err)
	
	// Fallback to screenshot library (GDI-based)
	n := screenshot.NumActiveDisplays()
	if n == 0 {
		return nil, fmt.Errorf("no active displays found (DXGI, GDI, and screenshot all failed)")
	}

	bounds := screenshot.GetDisplayBounds(0)
	log.Printf("✅ Screenshot library capturer ready: %dx%d", bounds.Dx(), bounds.Dy())

	return &Capturer{
		displayIndex: 0,
		bounds:       bounds,
	}, nil
}

func (c *Capturer) CaptureJPEG(quality int) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Use GDI if in GDI mode (Session 0 / login screen)
	if c.useGDI && c.gdiCapturer != nil {
		return c.gdiCapturer.CaptureJPEG(quality)
	}

	// Use DXGI if available (better for RDP)
	if c.dxgiCapturer != nil {
		return c.dxgiCapturer.CaptureJPEG(quality)
	}
	
	// Fallback to screenshot library
	img, err := screenshot.CaptureRect(c.bounds)
	if err != nil {
		return nil, fmt.Errorf("failed to capture screen: %w", err)
	}

	// Keep full resolution up to 4K (3840px) for MAXIMUM quality
	var finalImg image.Image = img
	maxWidth := uint(3840)
	if img.Bounds().Dx() > int(maxWidth) {
		// Use Lanczos3 for highest quality scaling
		finalImg = resize.Resize(maxWidth, 0, img, resize.Lanczos3)
	}

	// Encode as JPEG
	var buf bytes.Buffer
	opts := &jpeg.Options{Quality: quality}
	
	if err := jpeg.Encode(&buf, finalImg, opts); err != nil {
		return nil, fmt.Errorf("failed to encode JPEG: %w", err)
	}

	return buf.Bytes(), nil
}

// CaptureJPEGIfChanged only returns a frame if the screen has changed
// Returns (nil, nil) if no change detected
func (c *Capturer) CaptureJPEGIfChanged(quality int) ([]byte, error) {
	// Use DXGI if available (better for RDP)
	if c.dxgiCapturer != nil {
		// For DXGI, just capture every time (it's fast enough)
		return c.dxgiCapturer.CaptureJPEG(quality)
	}
	
	// Fallback to screenshot library with change detection
	img, err := screenshot.CaptureRect(c.bounds)
	if err != nil {
		return nil, fmt.Errorf("failed to capture screen: %w", err)
	}

	// Create a quick hash to detect changes (sample every 10th pixel for speed)
	hash := sha256.New()
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y += 10 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 10 {
			r, g, b, _ := img.At(x, y).RGBA()
			hash.Write([]byte{byte(r >> 8), byte(g >> 8), byte(b >> 8)})
		}
	}
	currentHash := hash.Sum(nil)

	// Compare with last frame
	if c.lastHash != nil && bytes.Equal(currentHash, c.lastHash) {
		// No change detected
		return nil, nil
	}
	c.lastHash = currentHash

	// Screen changed - encode and return
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

// Close releases capturer resources
func (c *Capturer) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.dxgiCapturer != nil {
		c.dxgiCapturer.Close()
		c.dxgiCapturer = nil
	}
	if c.gdiCapturer != nil {
		c.gdiCapturer.Close()
		c.gdiCapturer = nil
	}
	return nil
}

// Reinitialize recreates the capturer (useful after desktop switch)
func (c *Capturer) Reinitialize(forceGDI bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Close existing capturers
	if c.dxgiCapturer != nil {
		c.dxgiCapturer.Close()
		c.dxgiCapturer = nil
	}
	if c.gdiCapturer != nil {
		c.gdiCapturer.Close()
		c.gdiCapturer = nil
	}

	// Reinitialize based on mode
	if forceGDI {
		gdi, err := NewGDICapturer()
		if err != nil {
			return fmt.Errorf("failed to reinitialize GDI capturer: %w", err)
		}
		c.gdiCapturer = gdi
		c.bounds = gdi.GetBounds()
		c.useGDI = true
		log.Printf("✅ Reinitialized GDI capturer: %dx%d", c.bounds.Dx(), c.bounds.Dy())
		return nil
	}

	// Try DXGI first
	dxgi, err := NewDXGICapturer()
	if err == nil {
		c.dxgiCapturer = dxgi
		c.bounds = dxgi.GetBounds()
		c.useGDI = false
		log.Printf("✅ Reinitialized DXGI capturer: %dx%d", c.bounds.Dx(), c.bounds.Dy())
		return nil
	}

	// Fallback to GDI
	gdi, err := NewGDICapturer()
	if err != nil {
		return fmt.Errorf("failed to reinitialize any capturer: %w", err)
	}
	c.gdiCapturer = gdi
	c.bounds = gdi.GetBounds()
	c.useGDI = true
	log.Printf("✅ Reinitialized GDI capturer (fallback): %dx%d", c.bounds.Dx(), c.bounds.Dy())
	return nil
}

// IsGDIMode returns true if using GDI capture mode
func (c *Capturer) IsGDIMode() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.useGDI
}

// SwitchDisplay switches to a different monitor for capture
// Tears down the existing capturer and recreates it for the target output
func (c *Capturer) SwitchDisplay(displayIndex int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	log.Printf("🖥️ Switching to display %d...", displayIndex)

	// Close existing capturer
	if c.dxgiCapturer != nil {
		c.dxgiCapturer.Close()
		c.dxgiCapturer = nil
	}

	// Create new DXGI capturer for the target output
	dxgi, err := NewDXGICapturerForOutput(displayIndex)
	if err != nil {
		return fmt.Errorf("failed to switch to display %d: %w", displayIndex, err)
	}

	c.dxgiCapturer = dxgi
	c.bounds = dxgi.GetBounds()
	c.displayIndex = displayIndex
	c.useGDI = false
	c.lastHash = nil // Reset change detection

	log.Printf("✅ Switched to display %d: %dx%d", displayIndex, c.bounds.Dx(), c.bounds.Dy())
	return nil
}

// GetDisplayIndex returns the current display index
func (c *Capturer) GetDisplayIndex() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.displayIndex
}

// CaptureJPEGScaled captures and scales the screen to target width
// scale should be 0.5-1.0 (e.g., 0.75 = 75% of original size)
func (c *Capturer) CaptureJPEGScaled(quality int, scale float64) ([]byte, int, int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Clamp scale to valid range
	if scale < 0.25 {
		scale = 0.25
	}
	if scale > 1.0 {
		scale = 1.0
	}

	var img *image.RGBA
	var err error

	// Capture based on mode
	if c.useGDI && c.gdiCapturer != nil {
		img, err = c.gdiCapturer.CaptureRGBA()
	} else if c.dxgiCapturer != nil {
		img, err = c.dxgiCapturer.CaptureRGBA()
	} else {
		img, err = screenshot.CaptureRect(c.bounds)
	}

	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to capture screen: %w", err)
	}

	// Calculate target dimensions
	origWidth := img.Bounds().Dx()
	origHeight := img.Bounds().Dy()
	targetWidth := uint(float64(origWidth) * scale)
	targetHeight := uint(float64(origHeight) * scale)

	// Scale if needed
	var finalImg image.Image = img
	if scale < 1.0 {
		// Use Bilinear for speed (Lanczos3 is too slow for real-time)
		finalImg = resize.Resize(targetWidth, targetHeight, img, resize.Bilinear)
	}

	// Encode as JPEG
	var buf bytes.Buffer
	opts := &jpeg.Options{Quality: quality}
	if err := jpeg.Encode(&buf, finalImg, opts); err != nil {
		return nil, 0, 0, fmt.Errorf("failed to encode JPEG: %w", err)
	}

	// Return the SCALED dimensions (what the client will see)
	return buf.Bytes(), int(targetWidth), int(targetHeight), nil
}

// EncodeRGBAToJPEG encodes an existing RGBA frame to JPEG with scaling
// This avoids double-capture by reusing the RGBA frame from motion detection
func (c *Capturer) EncodeRGBAToJPEG(img *image.RGBA, quality int, scale float64) ([]byte, int, int, error) {
	// Clamp scale to valid range
	if scale < 0.25 {
		scale = 0.25
	}
	if scale > 1.0 {
		scale = 1.0
	}

	// Calculate target dimensions
	origWidth := img.Bounds().Dx()
	origHeight := img.Bounds().Dy()
	targetWidth := uint(float64(origWidth) * scale)
	targetHeight := uint(float64(origHeight) * scale)

	// Scale if needed
	var finalImg image.Image = img
	if scale < 1.0 {
		finalImg = resize.Resize(targetWidth, targetHeight, img, resize.Bilinear)
	}

	// Encode as JPEG
	var buf bytes.Buffer
	opts := &jpeg.Options{Quality: quality}
	if err := jpeg.Encode(&buf, finalImg, opts); err != nil {
		return nil, 0, 0, fmt.Errorf("failed to encode JPEG: %w", err)
	}

	return buf.Bytes(), int(targetWidth), int(targetHeight), nil
}

// CaptureRGBA captures the screen as RGBA image (for dirty region detection)
func (c *Capturer) CaptureRGBA() (*image.RGBA, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Use GDI if in GDI mode
	if c.useGDI && c.gdiCapturer != nil {
		return c.gdiCapturer.CaptureRGBA()
	}

	// Use DXGI if available
	if c.dxgiCapturer != nil {
		return c.dxgiCapturer.CaptureRGBA()
	}

	// Fallback to screenshot library
	img, err := screenshot.CaptureRect(c.bounds)
	if err != nil {
		return nil, fmt.Errorf("failed to capture screen: %w", err)
	}

	// screenshot.CaptureRect returns *image.RGBA directly
	return img, nil
}
