package encoder

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"sync"
)

// SoftwareEncoder implements H.264 encoding using software
// Note: This is a placeholder that outputs JPEG for now
// Real H.264 encoding requires x264 or ffmpeg bindings
type SoftwareEncoder struct {
	config  Config
	mu      sync.Mutex
	quality int
}

// NewSoftwareEncoder creates a new software encoder
func NewSoftwareEncoder() *SoftwareEncoder {
	return &SoftwareEncoder{
		quality: 75,
	}
}

// Init initializes the software encoder
func (e *SoftwareEncoder) Init(cfg Config) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.config = cfg
	// Map bitrate to JPEG quality (rough approximation)
	// 500 kbps -> Q50, 4000 kbps -> Q90
	e.quality = 50 + (cfg.Bitrate-500)*40/3500
	if e.quality < 50 {
		e.quality = 50
	}
	if e.quality > 90 {
		e.quality = 90
	}

	return nil
}

// Encode encodes an RGBA frame
// Note: Returns JPEG for now, will be replaced with H.264 NAL units
func (e *SoftwareEncoder) Encode(frame *image.RGBA, forceKeyframe bool) ([]byte, error) {
	e.mu.Lock()
	quality := e.quality
	e.mu.Unlock()

	if frame == nil {
		return nil, fmt.Errorf("nil frame")
	}

	// For now, encode as JPEG (placeholder for H.264)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, frame, &jpeg.Options{Quality: quality}); err != nil {
		return nil, fmt.Errorf("jpeg encode failed: %w", err)
	}

	return buf.Bytes(), nil
}

// SetBitrate adjusts the encoding bitrate
func (e *SoftwareEncoder) SetBitrate(kbps int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.config.Bitrate = kbps
	// Recalculate quality
	e.quality = 50 + (kbps-500)*40/3500
	if e.quality < 50 {
		e.quality = 50
	}
	if e.quality > 90 {
		e.quality = 90
	}

	return nil
}

// Close releases encoder resources
func (e *SoftwareEncoder) Close() error {
	return nil
}

// Name returns the encoder name
func (e *SoftwareEncoder) Name() string {
	return "software-jpeg"
}
