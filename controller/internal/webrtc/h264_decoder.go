package webrtc

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// H264Decoder decodes H.264 NAL units to frames using FFmpeg subprocess
// Uses hardware acceleration (DXVA2) and outputs raw NV12 frames for fast processing
type H264Decoder struct {
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	stdout   io.ReadCloser
	stderr   io.ReadCloser
	onFrame  func([]byte) // Callback for decoded JPEG frames
	running  bool
	mu       sync.Mutex
	stopChan chan struct{}
}

// NewH264Decoder creates a new FFmpeg-based H.264 decoder
func NewH264Decoder(onFrame func([]byte)) (*H264Decoder, error) {
	d := &H264Decoder{
		onFrame:  onFrame,
		stopChan: make(chan struct{}),
	}

	if err := d.start(); err != nil {
		return nil, err
	}

	return d, nil
}

// findFFmpeg locates the FFmpeg executable
// Priority: 1) Same directory as controller.exe, 2) ffmpeg subdirectory, 3) PATH
func findFFmpeg() string {
	ffmpegName := "ffmpeg"
	if runtime.GOOS == "windows" {
		ffmpegName = "ffmpeg.exe"
	}

	// Get executable directory
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)

		// Check same directory as controller.exe
		localFFmpeg := filepath.Join(exeDir, ffmpegName)
		if _, err := os.Stat(localFFmpeg); err == nil {
			log.Printf("🎬 Found FFmpeg at: %s", localFFmpeg)
			return localFFmpeg
		}

		// Check ffmpeg subdirectory
		subFFmpeg := filepath.Join(exeDir, "ffmpeg", ffmpegName)
		if _, err := os.Stat(subFFmpeg); err == nil {
			log.Printf("🎬 Found FFmpeg at: %s", subFFmpeg)
			return subFFmpeg
		}

		// Check bin subdirectory
		binFFmpeg := filepath.Join(exeDir, "bin", ffmpegName)
		if _, err := os.Stat(binFFmpeg); err == nil {
			log.Printf("🎬 Found FFmpeg at: %s", binFFmpeg)
			return binFFmpeg
		}
	}

	// macOS: check Homebrew paths (Apple Silicon + Intel)
	if runtime.GOOS == "darwin" || runtime.GOOS != "windows" {
		macPaths := []string{
			"/opt/homebrew/bin/ffmpeg", // Apple Silicon
			"/usr/local/bin/ffmpeg",    // Intel Mac / Homebrew on Linux
		}
		for _, p := range macPaths {
			if _, err := os.Stat(p); err == nil {
				log.Printf("🎬 Found FFmpeg at: %s", p)
				return p
			}
		}
	}

	// Fall back to PATH
	if path, err := exec.LookPath(ffmpegName); err == nil {
		log.Printf("🎬 Found FFmpeg in PATH: %s", path)
		return path
	}

	log.Println("⚠️ FFmpeg not found - H.264 decoding will not work")
	return ffmpegName // Try anyway, will fail with clear error
}

// start launches the FFmpeg subprocess
func (d *H264Decoder) start() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.running {
		return nil
	}

	// Find FFmpeg executable (auto-downloads if missing)
	ffmpegPath, err := EnsureFFmpeg()
	if err != nil {
		// EnsureFFmpeg failed, try findFFmpeg as last resort
		ffmpegPath = findFFmpeg()
	}

	// FFmpeg command: read H.264 Annex-B from stdin, output MJPEG to stdout
	// MJPEG output is self-framing (JPEG markers FFD8/FFD9) so we don't need
	// to know resolution upfront — eliminates the resolution-parsing dependency.
	// -flags low_delay: minimize decoding latency
	// -fflags nobuffer+discardcorrupt: disable buffering, discard corrupt frames
	// -probesize 32: minimal probing for faster start
	// -analyzeduration 0: skip analysis for faster start
	// -vsync 0: no frame sync, output as fast as possible
	// -vf ...: force consistent bt709 + full-range expansion + 4:4:4 output
	//   before MJPEG encode to reduce color shift and chroma blur vs JPEG mode.
	// -q:v 1: highest MJPEG quality (lower=better) to preserve H.264 detail.
	d.cmd = exec.Command(ffmpegPath,
		"-hide_banner",
		"-loglevel", "info",
		"-flags", "low_delay",
		"-fflags", "nobuffer+discardcorrupt",
		"-probesize", "32",
		"-analyzeduration", "0",
		"-f", "h264",
		"-i", "pipe:0",
		"-vsync", "0",
		"-sws_flags", "bicubic+accurate_rnd+full_chroma_int+full_chroma_inp",
		"-vf", "scale=in_color_matrix=bt709:out_color_matrix=bt709:in_range=tv:out_range=pc,format=yuvj444p",
		"-pix_fmt", "yuvj444p",
		"-color_range", "pc",
		"-colorspace", "bt709",
		"-color_primaries", "bt709",
		"-color_trc", "bt709",
		"-q:v", "1",
		"-f", "mjpeg",
		"pipe:1",
	)
	configureFFmpegCmd(d.cmd)

	d.stdin, err = d.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	d.stdout, err = d.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	d.stderr, _ = d.cmd.StderrPipe()

	if err := d.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	d.running = true
	log.Println("🎬 FFmpeg H.264 decoder started")

	// Start goroutine to read decoded frames
	go d.readFrames()
	go d.readStderr()

	return nil
}

func (d *H264Decoder) readStderr() {
	if d.stderr == nil {
		return
	}
	buf := make([]byte, 4096)
	for {
		select {
		case <-d.stopChan:
			return
		default:
		}
		n, err := d.stderr.Read(buf)
		if n > 0 {
			line := string(bytes.TrimSpace(buf[:n]))
			log.Printf("🎬 ffmpeg: %s", line)
		}
		if err != nil {
			return
		}
	}
}

// readFrames reads MJPEG frames from FFmpeg stdout
// MJPEG is self-framing: each JPEG starts with FFD8 and ends with FFD9
func (d *H264Decoder) readFrames() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("❌ H264Decoder readFrames panic: %v", r)
		}
	}()

	var buf bytes.Buffer
	readBuf := make([]byte, 65536) // 64KB read chunks
	frameCount := 0
	lastLogTime := time.Now()
	var lastFPS float64

	for {
		select {
		case <-d.stopChan:
			return
		default:
		}

		// Read data from FFmpeg stdout
		n, err := d.stdout.Read(readBuf)
		if err != nil {
			if err != io.EOF {
				log.Printf("⚠️ FFmpeg stdout read error: %v", err)
			}
			return
		}
		buf.Write(readBuf[:n])

		// Extract complete JPEG frames (FFD8...FFD9)
		for {
			data := buf.Bytes()
			if len(data) < 2 {
				break
			}

			// Find JPEG start marker (FFD8)
			startIdx := -1
			for i := 0; i < len(data)-1; i++ {
				if data[i] == 0xFF && data[i+1] == 0xD8 {
					startIdx = i
					break
				}
			}
			if startIdx < 0 {
				// No start marker found, discard all but last byte
				if buf.Len() > 1 {
					buf.Next(buf.Len() - 1)
				}
				break
			}

			// Discard data before start marker
			if startIdx > 0 {
				buf.Next(startIdx)
				data = buf.Bytes()
			}

			// Find JPEG end marker (FFD9) after start
			endIdx := -1
			for i := 2; i < len(data)-1; i++ {
				if data[i] == 0xFF && data[i+1] == 0xD9 {
					endIdx = i + 2 // Include the FFD9 marker
					break
				}
			}
			if endIdx < 0 {
				break // Incomplete frame, wait for more data
			}

			// Extract complete JPEG frame
			jpegData := make([]byte, endIdx)
			copy(jpegData, data[:endIdx])
			buf.Next(endIdx)

			frameCount++

			// Log FPS periodically
			now := time.Now()
			elapsed := now.Sub(lastLogTime).Seconds()
			if elapsed >= 1.0 {
				fps := float64(frameCount) / elapsed
				if fps != lastFPS {
					log.Printf("🎬 H.264 decoded: %d frames (%.1f fps, %d bytes/frame)", frameCount, fps, len(jpegData))
					lastFPS = fps
				}
				lastLogTime = now
				frameCount = 0
			}

			// Forward JPEG frame to callback
			if d.onFrame != nil {
				d.onFrame(jpegData)
			}
		}
	}
}

// Decode sends H.264 NAL units to the decoder
func (d *H264Decoder) Decode(nalUnits []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running || d.stdin == nil {
		return fmt.Errorf("decoder not running")
	}

	// Write NAL units to FFmpeg stdin
	_, err := d.stdin.Write(nalUnits)
	return err
}

// DecodeAnnexB sends Annex-B formatted H.264 data to the decoder
func (d *H264Decoder) DecodeAnnexB(data []byte) error {
	return d.Decode(data)
}

// Stop stops the decoder
func (d *H264Decoder) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running {
		return
	}

	close(d.stopChan)
	d.running = false

	if d.stdin != nil {
		d.stdin.Close()
	}

	if d.cmd != nil && d.cmd.Process != nil {
		d.cmd.Process.Kill()
		d.cmd.Wait()
	}

	log.Println("🎬 FFmpeg H.264 decoder stopped")
}

// IsRunning returns whether the decoder is running
func (d *H264Decoder) IsRunning() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.running
}

// EnsureAnnexB ensures NAL units have Annex-B start codes (00 00 00 01)
// Handles: already Annex-B, single NAL without start code, and AVCC (length-prefixed) format
func EnsureAnnexB(data []byte) []byte {
	if len(data) < 4 {
		return data
	}

	// Check if already has Annex-B start code (00 00 00 01 or 00 00 01)
	if data[0] == 0 && data[1] == 0 {
		if data[2] == 0 && data[3] == 1 {
			return data // Already 4-byte start code
		}
		if data[2] == 1 {
			return data // Already 3-byte start code
		}
	}

	// Check for AVCC format (4-byte length prefix)
	// First 4 bytes are big-endian NAL length
	nalLen := int(data[0])<<24 | int(data[1])<<16 | int(data[2])<<8 | int(data[3])
	if nalLen > 0 && nalLen <= len(data)-4 {
		// Likely AVCC format - convert to Annex-B
		var result []byte
		offset := 0
		for offset+4 <= len(data) {
			nalLen := int(data[offset])<<24 | int(data[offset+1])<<16 | int(data[offset+2])<<8 | int(data[offset+3])
			if nalLen <= 0 || offset+4+nalLen > len(data) {
				break
			}
			// Add start code + NAL data
			result = append(result, 0, 0, 0, 1)
			result = append(result, data[offset+4:offset+4+nalLen]...)
			offset += 4 + nalLen
		}
		if len(result) > 0 {
			return result
		}
	}

	// Single NAL without start code - add one
	result := make([]byte, 4+len(data))
	result[0] = 0
	result[1] = 0
	result[2] = 0
	result[3] = 1
	copy(result[4:], data)
	return result
}
