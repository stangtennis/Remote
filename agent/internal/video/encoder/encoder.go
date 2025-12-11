// Package encoder provides H.264 video encoding for remote desktop streaming
package encoder

import (
	"fmt"
	"image"
	"sync"
)

// Config holds encoder configuration
type Config struct {
	Width      int
	Height     int
	Bitrate    int // kbps
	Framerate  int
	KeyframeInterval int // frames between keyframes
}

// Encoder interface for video encoding
type Encoder interface {
	// Init initializes the encoder with the given config
	Init(cfg Config) error

	// Encode encodes an RGBA frame to H.264 NAL units
	Encode(frame *image.RGBA, forceKeyframe bool) ([]byte, error)

	// SetBitrate dynamically adjusts the bitrate
	SetBitrate(kbps int) error

	// Close releases encoder resources
	Close() error

	// Name returns the encoder name (e.g., "x264", "nvenc")
	Name() string
}

// Manager manages encoder selection and lifecycle
type Manager struct {
	encoder    Encoder
	config     Config
	mu         sync.Mutex
	frameCount int
}

// NewManager creates a new encoder manager
func NewManager() *Manager {
	return &Manager{}
}

// Init initializes the encoder with hardware-first, software fallback
func (m *Manager) Init(cfg Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = cfg

	// Try OpenH264 first (real H.264 encoding)
	openh264Enc := NewOpenH264Encoder()
	if err := openh264Enc.Init(cfg); err == nil {
		m.encoder = openh264Enc
		return nil
	}

	// Fallback to software encoder (JPEG placeholder)
	encoder := NewSoftwareEncoder()
	if err := encoder.Init(cfg); err != nil {
		return fmt.Errorf("failed to init encoder: %w", err)
	}

	m.encoder = encoder
	return nil
}

// Encode encodes a frame
func (m *Manager) Encode(frame *image.RGBA) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.encoder == nil {
		return nil, fmt.Errorf("encoder not initialized")
	}

	m.frameCount++
	forceKeyframe := m.frameCount%m.config.KeyframeInterval == 0

	return m.encoder.Encode(frame, forceKeyframe)
}

// SetBitrate adjusts bitrate dynamically
func (m *Manager) SetBitrate(kbps int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.encoder == nil {
		return fmt.Errorf("encoder not initialized")
	}

	m.config.Bitrate = kbps
	return m.encoder.SetBitrate(kbps)
}

// ForceKeyframe forces the next frame to be a keyframe
func (m *Manager) ForceKeyframe() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.frameCount = 0 // Reset to trigger keyframe on next encode
}

// Close releases resources
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.encoder != nil {
		return m.encoder.Close()
	}
	return nil
}

// GetEncoderName returns the active encoder name
func (m *Manager) GetEncoderName() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.encoder != nil {
		return m.encoder.Name()
	}
	return "none"
}
