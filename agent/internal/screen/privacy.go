package screen

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"sync"

	"github.com/stangtennis/remote-agent/internal/state"
)

// privacyFrameCache holds a pre-encoded black JPEG for the agent's current bounds.
// Re-encoded when bounds or quality change.
type privacyFrameCache struct {
	mu        sync.Mutex
	bounds    image.Rectangle
	quality   int
	cached    []byte
}

var privacyCache privacyFrameCache

// blackJPEG returns a cached black JPEG matching the given bounds+quality.
// Used when privacy mode is enabled.
func blackJPEG(bounds image.Rectangle, quality int) ([]byte, error) {
	privacyCache.mu.Lock()
	defer privacyCache.mu.Unlock()

	if privacyCache.cached != nil && privacyCache.bounds == bounds && privacyCache.quality == quality {
		return privacyCache.cached, nil
	}

	img := image.NewRGBA(bounds)
	black := color.RGBA{R: 0, G: 0, B: 0, A: 255}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			img.SetRGBA(x, y, black)
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}

	privacyCache.bounds = bounds
	privacyCache.quality = quality
	privacyCache.cached = buf.Bytes()
	return privacyCache.cached, nil
}

// privacyOverride returns (data, true) if privacy mode is active and a black
// frame should be used instead of a real capture. Otherwise returns (nil, false).
func privacyOverride(bounds image.Rectangle, quality int) ([]byte, bool) {
	if !state.IsPrivacyModeEnabled() {
		return nil, false
	}
	data, err := blackJPEG(bounds, quality)
	if err != nil {
		return nil, false
	}
	return data, true
}
