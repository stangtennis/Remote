// Package audio provides audio streaming capabilities via WebRTC
package audio

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
)

// Track manages a WebRTC audio track for Opus streaming
type Track struct {
	track   *webrtc.TrackLocalStaticSample
	mu      sync.Mutex
	running bool
}

// NewTrack creates a new Opus audio track
func NewTrack() (*Track, error) {
	track, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{
			MimeType:  webrtc.MimeTypeOpus,
			ClockRate: 48000,
			Channels:  2,
		},
		"audio",
		"remote-desktop-audio",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create audio track: %w", err)
	}

	return &Track{track: track}, nil
}

// GetTrack returns the underlying WebRTC track for adding to peer connection
func (t *Track) GetTrack() *webrtc.TrackLocalStaticSample {
	return t.track
}

// WriteSample writes encoded Opus audio data to the track
func (t *Track) WriteSample(data []byte, duration time.Duration) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.running {
		return nil // silently drop when not running
	}

	return t.track.WriteSample(media.Sample{
		Data:     data,
		Duration: duration,
	})
}

// Start starts the audio track
func (t *Track) Start() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.running = true
	log.Println("🔊 Audio track started")
}

// Stop stops the audio track
func (t *Track) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.running = false
	log.Println("🔊 Audio track stopped")
}

// IsRunning returns whether the track is running
func (t *Track) IsRunning() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.running
}
