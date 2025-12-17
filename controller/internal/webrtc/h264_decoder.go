package webrtc

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
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

	// FFmpeg command: read H.264 Annex-B from stdin, output raw NV12 to stdout
	// Using hardware acceleration (DXVA2) on Windows for fast decoding
	// NV12 is the native GPU format - much faster than MJPEG re-encoding!
	// -hwaccel dxva2: use DirectX Video Acceleration (GPU decode)
	// -hwaccel_output_format nv12: keep frames in GPU-native format
	// -flags low_delay: minimize decoding latency
	// -fflags nobuffer+discardcorrupt: disable buffering, discard corrupt frames
	// -probesize 32: minimal probing for faster start
	// -analyzeduration 0: skip analysis for faster start
	// -vsync 0: no frame sync, output as fast as possible
	// -f rawvideo -pix_fmt nv12: output raw NV12 frames (Y plane + interleaved UV)
	d.cmd = exec.Command(ffmpegPath,
		"-hide_banner",
		"-loglevel", "info",
		"-hwaccel", "dxva2",
		"-hwaccel_output_format", "nv12",
		"-flags", "low_delay",
		"-fflags", "nobuffer+discardcorrupt",
		"-probesize", "32",
		"-analyzeduration", "0",
		"-f", "h264",
		"-i", "pipe:0",
		"-vsync", "0",
		"-f", "rawvideo",
		"-pix_fmt", "nv12",
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

// readFrames reads raw NV12 frames from FFmpeg stdout and converts to JPEG
func (d *H264Decoder) readFrames() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("❌ H264Decoder readFrames panic: %v", r)
		}
	}()

	var frameBuf bytes.Buffer
	frameCount := 0
	lastLogTime := time.Now()
	var lastFPS float64

	for {
		select {
		case <-d.stopChan:
			return
		default:
		}

		// Get current resolution
		d.mu.Lock()
		width := d.width
		height := d.height
		d.mu.Unlock()

		// If we don't have resolution yet, read small chunks and wait
		if width == 0 || height == 0 {
			buf := make([]byte, 4096)
			n, err := d.stdout.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Printf("⚠️ FFmpeg stdout read error: %v", err)
				}
				return
			}
			frameBuf.Write(buf[:n])
			continue
		}

		// Calculate NV12 frame size: Y plane (width*height) + UV plane (width*height/2)
		frameSize := width*height + width*height/2

		// Read until we have a complete frame
		for frameBuf.Len() < frameSize {
			buf := make([]byte, 65536) // 64KB chunks
			n, err := d.stdout.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Printf("⚠️ FFmpeg stdout read error: %v", err)
				}
				return
			}
			frameBuf.Write(buf[:n])
		}

		// Extract one frame
		frameData := frameBuf.Next(frameSize)
		frameCount++

		// Log FPS periodically
		now := time.Now()
		elapsed := now.Sub(lastLogTime).Seconds()
		if elapsed >= 1.0 {
			fps := float64(frameCount) / elapsed
			if fps != lastFPS {
				log.Printf("🎬 H.264 NV12 frames: %d (%.1f fps, %dx%d)", frameCount, fps, width, height)
				lastFPS = fps
			}
			lastLogTime = now
			frameCount = 0
		}

		// Convert NV12 to JPEG (fast, in-memory)
		jpegData := nv12ToJPEG(frameData, width, height)
		if jpegData != nil && d.onFrame != nil {
			d.onFrame(jpegData)
		}
	}
}

// nv12ToJPEG converts NV12 raw frame to JPEG
// NV12 format: Y plane (width*height bytes) followed by interleaved UV plane (width*height/2 bytes)
func nv12ToJPEG(nv12 []byte, width, height int) []byte {
	ySize := width * height
	if len(nv12) < ySize+ySize/2 {
		return nil
	}

	// Create RGB image
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Y plane
	yPlane := nv12[:ySize]
	// UV plane (interleaved)
	uvPlane := nv12[ySize:]

	// Convert NV12 to RGB
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Get Y value
			yIdx := y*width + x
			yVal := int(yPlane[yIdx])

			// Get UV values (subsampled 2x2)
			uvIdx := (y/2)*(width) + (x/2)*2
			if uvIdx+1 >= len(uvPlane) {
				continue
			}
			uVal := int(uvPlane[uvIdx]) - 128
			vVal := int(uvPlane[uvIdx+1]) - 128

			// YUV to RGB conversion (BT.601)
			r := yVal + (359*vVal)>>8
			g := yVal - (88*uVal+183*vVal)>>8
			b := yVal + (454*uVal)>>8

			// Clamp values
			if r < 0 {
				r = 0
			} else if r > 255 {
				r = 255
			}
			if g < 0 {
				g = 0
			} else if g > 255 {
				g = 255
			}
			if b < 0 {
				b = 0
			} else if b > 255 {
				b = 255
			}

			// Set pixel
			idx := (y*width + x) * 4
			img.Pix[idx+0] = uint8(r)
			img.Pix[idx+1] = uint8(g)
			img.Pix[idx+2] = uint8(b)
			img.Pix[idx+3] = 255
		}
	}

	// Encode to JPEG with medium quality (faster)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 75}); err != nil {
		return nil
	}

	return buf.Bytes()
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
