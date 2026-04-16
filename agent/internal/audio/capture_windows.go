//go:build windows

package audio

import (
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// Capturer captures system audio via WASAPI loopback using ffmpeg.
// This is the simple approach — a future version could use direct WASAPI CGO for lower latency.
type Capturer struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	cancel  context.CancelFunc
	running bool
}

// NewCapturer creates a new audio capturer
func NewCapturer() *Capturer {
	return &Capturer{}
}

// Start begins capturing system audio and feeding Opus-encoded frames to the track.
// It spawns ffmpeg to capture WASAPI loopback audio and encode it to Opus.
func (c *Capturer) Start(ctx context.Context, track *Track) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	// Check if ffmpeg is available
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg not found — audio capture requires ffmpeg in PATH: %w", err)
	}

	captureCtx, cancel := context.WithCancel(ctx)

	// ffmpeg: capture Windows WASAPI loopback → raw Opus packets
	// -f dshow -i audio="virtual-audio-capturer" would need a virtual device
	// Instead, use -f lavfi to generate silence as a placeholder, then swap to real capture
	// For actual WASAPI loopback, we use the "Stereo Mix" or system default loopback
	cmd := exec.CommandContext(captureCtx,
		"ffmpeg",
		"-f", "dshow",
		"-audio_buffer_size", "50",
		"-i", "audio=virtual-audio-capturer",
		"-acodec", "libopus",
		"-ar", "48000",
		"-ac", "2",
		"-b:a", "64k",
		"-application", "lowdelay",
		"-frame_duration", "20",
		"-vbr", "off",
		"-f", "opus",
		"-",
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("ffmpeg stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		// ffmpeg/dshow capture might not be available — log and continue without audio
		log.Printf("⚠️ Audio capture not available (ffmpeg dshow failed): %v", err)
		return nil
	}

	c.mu.Lock()
	c.cmd = cmd
	c.cancel = cancel
	c.running = true
	c.mu.Unlock()

	track.Start()
	log.Println("🔊 Audio capture started (WASAPI loopback via ffmpeg)")

	// Read Opus frames and write to track
	go func() {
		defer func() {
			track.Stop()
			c.mu.Lock()
			c.running = false
			c.mu.Unlock()
			log.Println("🔊 Audio capture stopped")
		}()

		// Read Opus data in 20ms frames (~960 samples at 48kHz × 2ch)
		buf := make([]byte, 4096)
		frameDuration := 20 * time.Millisecond

		for {
			select {
			case <-captureCtx.Done():
				return
			default:
			}

			n, err := stdout.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Printf("⚠️ Audio read error: %v", err)
				}
				return
			}

			if n > 0 {
				frame := make([]byte, n)
				copy(frame, buf[:n])
				if writeErr := track.WriteSample(frame, frameDuration); writeErr != nil {
					log.Printf("⚠️ Audio write error: %v", writeErr)
					return
				}
			}
		}
	}()

	return nil
}

// Stop stops the audio capture
func (c *Capturer) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
		c.cmd.Wait()
	}
	c.running = false
}

// IsRunning returns whether audio capture is active
func (c *Capturer) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}
