//go:build darwin

package audio

import (
	"context"
	"log"
)

// Capturer is a stub for macOS audio capture.
// macOS system audio loopback requires an aggregate audio device or
// ScreenCaptureKit (macOS 12.3+), which will be implemented in a future version.
type Capturer struct{}

// NewCapturer creates a new audio capturer (stub on macOS)
func NewCapturer() *Capturer {
	return &Capturer{}
}

// Start is a no-op on macOS — audio capture not yet implemented
func (c *Capturer) Start(ctx context.Context, track *Track) error {
	log.Println("⚠️ Audio capture not available on macOS (requires ScreenCaptureKit)")
	return nil
}

// Stop is a no-op on macOS
func (c *Capturer) Stop() {}

// IsRunning always returns false on macOS
func (c *Capturer) IsRunning() bool {
	return false
}
