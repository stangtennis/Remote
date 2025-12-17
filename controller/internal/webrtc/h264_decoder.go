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
)

// H264Decoder decodes H.264 NAL units to frames using FFmpeg subprocess
type H264Decoder struct {
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	stdout   io.ReadCloser
	stderr   io.ReadCloser
	onFrame  func([]byte) // Callback for decoded JPEG frames
	running  bool
	mu       sync.Mutex
	stopChan chan struct{}
	width    int
	height   int
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

	// Find FFmpeg executable
	ffmpegPath := findFFmpeg()

	// FFmpeg command: read H.264 Annex-B from stdin, output MJPEG to stdout
	// Using hardware acceleration (DXVA2) on Windows for fast decoding
	// -hwaccel dxva2: use DirectX Video Acceleration
	// -hwaccel_output_format nv12: hardware output format
	// -threads 0: use all available CPU cores
	// -flags low_delay: minimize decoding latency
	// -fflags nobuffer+discardcorrupt: disable buffering, discard corrupt frames
	// -probesize 32: minimal probing for faster start
	// -analyzeduration 0: skip analysis for faster start
	// -q:v 10: lower quality JPEG for speed (range 2-31, higher=faster)
	// -vsync 0: no frame sync, output as fast as possible
	d.cmd = exec.Command(ffmpegPath,
		"-hide_banner",
		"-loglevel", "warning",
		"-hwaccel", "dxva2",
		"-threads", "4",
		"-flags", "low_delay",
		"-fflags", "nobuffer+discardcorrupt",
		"-probesize", "32",
		"-analyzeduration", "0",
		"-f", "h264",
		"-i", "pipe:0",
		"-vsync", "0",
		"-f", "image2pipe",
		"-vcodec", "mjpeg",
		"-q:v", "10",
		"pipe:1",
	)
	configureFFmpegCmd(d.cmd)

	var err error
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
			// Parse resolution from FFmpeg output (e.g., "Stream #0:0: Video: h264, 1920x1080")
			d.parseResolution(line)
		}
		if err != nil {
			return
		}
	}
}

// parseResolution extracts video resolution from FFmpeg stderr output
func (d *H264Decoder) parseResolution(line string) {
	// Look for resolution pattern like "1920x1080" or "3840x2160"
	// FFmpeg outputs: "Stream #0:0: Video: h264 (High), yuv420p, 1920x1080, 30 fps"
	if d.width > 0 && d.height > 0 {
		return // Already have resolution
	}

	// Simple pattern matching for WxH
	for i := 0; i < len(line)-4; i++ {
		if line[i] >= '0' && line[i] <= '9' {
			// Found a digit, try to parse WxH
			j := i
			for j < len(line) && line[j] >= '0' && line[j] <= '9' {
				j++
			}
			if j < len(line) && line[j] == 'x' {
				k := j + 1
				for k < len(line) && line[k] >= '0' && line[k] <= '9' {
					k++
				}
				if k > j+1 {
					var w, h int
					fmt.Sscanf(line[i:k], "%dx%d", &w, &h)
					if w >= 320 && w <= 7680 && h >= 240 && h <= 4320 {
						d.mu.Lock()
						d.width = w
						d.height = h
						d.mu.Unlock()
						log.Printf("🎬 Detected video resolution: %dx%d", w, h)
						return
					}
				}
			}
		}
	}
}

// readFrames reads MJPEG frames from FFmpeg stdout
func (d *H264Decoder) readFrames() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("❌ H264Decoder readFrames panic: %v", r)
		}
	}()

	buf := make([]byte, 512*1024) // 512KB buffer
	var frameBuf bytes.Buffer
	frameCount := 0

	for {
		select {
		case <-d.stopChan:
			return
		default:
		}

		n, err := d.stdout.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Printf("⚠️ FFmpeg stdout read error: %v", err)
			}
			return
		}

		// Append to frame buffer
		frameBuf.Write(buf[:n])

		// Look for complete JPEG frames (SOI...EOI)
		data := frameBuf.Bytes()
		for {
			// Find SOI marker (0xFFD8)
			soiIdx := bytes.Index(data, []byte{0xFF, 0xD8})
			if soiIdx == -1 {
				break
			}

			// Find EOI marker (0xFFD9) after SOI
			eoiIdx := bytes.Index(data[soiIdx+2:], []byte{0xFF, 0xD9})
			if eoiIdx == -1 {
				break
			}
			eoiIdx += soiIdx + 2 + 2 // Include the EOI marker itself

			// Extract complete JPEG frame
			jpegData := make([]byte, eoiIdx-soiIdx)
			copy(jpegData, data[soiIdx:eoiIdx])

			frameCount++
			if frameCount%30 == 0 {
				log.Printf("🎬 H.264 decoded frames: %d", frameCount)
			}

			// Send to callback
			if d.onFrame != nil {
				d.onFrame(jpegData)
			}

			// Remove processed data from buffer
			data = data[eoiIdx:]
		}

		// Keep remaining data in buffer
		frameBuf.Reset()
		frameBuf.Write(data)
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
