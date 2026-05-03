package encoder

import (
	"fmt"
	"image"
	"io"
	"log"
	"os/exec"
	"runtime"
	"sync"
)

// NVENCEncoder implements H.264 encoding using FFmpeg's NVENC backend
type NVENCEncoder struct {
	config  Config
	mu      sync.Mutex
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	buf     []byte
	started bool
}

// NewNVENCEncoder creates a new NVENC encoder
func NewNVENCEncoder() *NVENCEncoder {
	return &NVENCEncoder{}
}

// IsNVENCAvailable checks if NVENC encoding is available
func IsNVENCAvailable() bool {
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		return false
	}

	// Check if ffmpeg is available
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return false
	}

	// Probe for NVENC support
	cmd := exec.Command("ffmpeg", "-hide_banner", "-encoders")
	out, err := cmd.Output()
	if err != nil {
		return false
	}

	return containsBytes(out, []byte("h264_nvenc"))
}

func containsBytes(haystack, needle []byte) bool {
	for i := 0; i <= len(haystack)-len(needle); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// Init initializes the NVENC encoder via FFmpeg subprocess
func (e *NVENCEncoder) Init(cfg Config) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !IsNVENCAvailable() {
		return fmt.Errorf("NVENC not available (ffmpeg or h264_nvenc not found)")
	}

	e.config = cfg
	return e.startFFmpeg()
}

func (e *NVENCEncoder) startFFmpeg() error {
	if e.cmd != nil {
		e.cmd.Process.Kill()
		e.cmd.Wait()
	}

	// FFmpeg command: read raw RGBA from stdin, encode to H.264 NVENC, output raw H.264 to stdout
	//
	// VIGTIGT: forceintraidr+repeat-spspps er KRITISK for WebRTC.
	// Uden disse flags sender NVENC kun SPS+PPS én gang ved start, og
	// browser-side decoder (WebView2/Chromium) kan ikke initialisere
	// dekoderen senere → black screen i controller selvom frames flyder.
	// `-bsf h264_metadata` repeats SPS+PPS i hver keyframe → decoder
	// kan altid samle initialiserings-data uanset hvor i streamen den
	// joiner.
	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-f", "rawvideo",
		"-pix_fmt", "rgba",
		"-s", fmt.Sprintf("%dx%d", e.config.Width, e.config.Height),
		"-r", fmt.Sprintf("%d", e.config.Framerate),
		"-i", "pipe:0",
		"-c:v", "h264_nvenc",
		"-preset", "p1",          // Fastest preset (latency)
		"-tune", "ull",           // Ultra-low latency
		"-rc", "cbr",             // Constant bitrate
		"-b:v", fmt.Sprintf("%dk", e.config.Bitrate),
		"-maxrate", fmt.Sprintf("%dk", e.config.Bitrate),
		"-bufsize", fmt.Sprintf("%dk", e.config.Bitrate/2),
		"-profile:v", "baseline", // Mest kompatibel — ingen B-frames, simpel decoder
		"-level", "4.1",          // Op til 1080p60 / 2K30
		"-g", fmt.Sprintf("%d", e.config.KeyframeInterval),
		"-bf", "0",               // No B-frames for low latency
		"-forced-idr", "1",       // Tving IDR (ikke kun I-frame) ved keyframes
		"-spatial_aq", "0",       // Slå adaptive quantization fra (encoder-bug i visse drivere)
		"-flags", "+low_delay",
		// Repeat SPS+PPS før hver IDR-frame — KRÆVET for WebRTC join-late
		// så browser-decoder kan re-initialiseres når en ny keyframe kommer.
		"-bsf:v", "dump_extra=freq=keyframe",
		"-f", "h264",
		"pipe:1",
	}

	e.cmd = exec.Command("ffmpeg", args...)

	var err error
	e.stdin, err = e.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	e.stdout, err = e.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	if err := e.cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg start: %w", err)
	}

	e.started = true
	log.Printf("NVENC encoder started (FFmpeg PID: %d, %dx%d @ %d kbps)",
		e.cmd.Process.Pid, e.config.Width, e.config.Height, e.config.Bitrate)

	return nil
}

// Encode encodes an RGBA frame to H.264 NAL units via NVENC
func (e *NVENCEncoder) Encode(frame *image.RGBA, forceKeyframe bool) ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.started || e.stdin == nil {
		return nil, fmt.Errorf("encoder not started")
	}

	// Check if frame dimensions changed
	bounds := frame.Bounds()
	if bounds.Dx() != e.config.Width || bounds.Dy() != e.config.Height {
		e.config.Width = bounds.Dx()
		e.config.Height = bounds.Dy()
		log.Printf("NVENC: resolution changed to %dx%d, restarting ffmpeg", e.config.Width, e.config.Height)
		if err := e.startFFmpeg(); err != nil {
			return nil, err
		}
	}

	// Write raw RGBA frame to FFmpeg stdin
	_, err := e.stdin.Write(frame.Pix)
	if err != nil {
		return nil, fmt.Errorf("write frame: %w", err)
	}

	// Read encoded output (non-blocking with buffer)
	if e.buf == nil {
		e.buf = make([]byte, 256*1024) // 256KB read buffer
	}

	n, err := e.stdout.Read(e.buf)
	if err != nil {
		return nil, fmt.Errorf("read output: %w", err)
	}

	result := make([]byte, n)
	copy(result, e.buf[:n])
	return result, nil
}

// SetBitrate adjusts the encoding bitrate (requires FFmpeg restart)
func (e *NVENCEncoder) SetBitrate(kbps int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.config.Bitrate = kbps
	// FFmpeg doesn't support dynamic bitrate change, would need restart
	// For now just update config — restart on next resolution change
	log.Printf("NVENC: bitrate updated to %d kbps (effective on next restart)", kbps)
	return nil
}

// Close releases encoder resources
func (e *NVENCEncoder) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.started = false
	if e.stdin != nil {
		e.stdin.Close()
	}
	if e.cmd != nil && e.cmd.Process != nil {
		e.cmd.Process.Kill()
		e.cmd.Wait()
	}
	log.Println("NVENC encoder closed")
	return nil
}

// Name returns the encoder name
func (e *NVENCEncoder) Name() string {
	return "nvenc"
}
