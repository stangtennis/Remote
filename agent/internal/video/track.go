// Package video provides video streaming capabilities
package video

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
)

// Track manages a WebRTC video track for H.264 streaming
type Track struct {
	track     *webrtc.TrackLocalStaticSample
	mu        sync.Mutex
	running   bool
	frameRate int
	bitrate   int // kbps
}

// NewTrack creates a new video track
func NewTrack() (*Track, error) {
	// Create H.264 track
	track, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{
			MimeType:  webrtc.MimeTypeH264,
			ClockRate: 90000,
		},
		"video",
		"remote-desktop",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create video track: %w", err)
	}

	return &Track{
		track:     track,
		frameRate: 30,
		bitrate:   2000,
	}, nil
}

// GetTrack returns the underlying WebRTC track for adding to peer connection
func (t *Track) GetTrack() *webrtc.TrackLocalStaticSample {
	return t.track
}

// WriteFrame writes encoded video data to the track
func (t *Track) WriteFrame(data []byte, duration time.Duration) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.running {
		return fmt.Errorf("track not running")
	}

	return t.track.WriteSample(media.Sample{
		Data:     data,
		Duration: duration,
	})
}

// Start starts the video track
func (t *Track) Start() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.running = true
	log.Println("🎬 Video track started")
}

// Stop stops the video track
func (t *Track) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.running = false
	log.Println("🎬 Video track stopped")
}

// SetBitrate sets the target bitrate
func (t *Track) SetBitrate(kbps int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.bitrate = kbps
}

// SetFrameRate sets the target frame rate
func (t *Track) SetFrameRate(fps int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.frameRate = fps
}

// IsRunning returns whether the track is running
func (t *Track) IsRunning() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.running
}
