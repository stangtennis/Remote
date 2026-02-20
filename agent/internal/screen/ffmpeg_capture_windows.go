//go:build windows

package screen

import (
	"bufio"
	"bytes"
	"fmt"
	"image"
	"io"
	"log"
	"os/exec"
	"sync"
	"time"
)

type FFmpegCapturer struct {
	ffmpegPath  string
	width       int
	height      int
	cmd         *exec.Cmd
	stdout      io.ReadCloser
	latestFrame []byte
	frameMutex  sync.RWMutex
	stopChan    chan struct{}
	running     bool
}

func NewFFmpegCapturer() (*FFmpegCapturer, error) {
	// Check if ffmpeg is available
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		// Try common installation paths
		paths := []string{
			"C:\\ffmpeg\\bin\\ffmpeg.exe",
			"C:\\Program Files\\ffmpeg\\bin\\ffmpeg.exe",
			".\\ffmpeg.exe",
		}
		
		found := false
		for _, path := range paths {
			if _, err := exec.LookPath(path); err == nil {
				ffmpegPath = path
				found = true
				break
			}
		}
		
		if !found {
			return nil, fmt.Errorf("ffmpeg not found in PATH or common locations")
		}
	}

	// Detect screen resolution
	width, height := getScreenResolution()

	capturer := &FFmpegCapturer{
		ffmpegPath: ffmpegPath,
		width:      width,
		height:     height,
		stopChan:   make(chan struct{}),
	}

	// Start the persistent FFmpeg process
	if err := capturer.start(); err != nil {
		return nil, fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	return capturer, nil
}

func (c *FFmpegCapturer) start() error {
	// Start FFmpeg in continuous capture mode
	c.cmd = exec.Command(
		c.ffmpegPath,
		"-f", "gdigrab",        // GDI capture (works with RDP)
		"-framerate", "10",     // 10 FPS
		"-i", "desktop",        // Capture desktop
		"-vcodec", "mjpeg",     // MJPEG codec
		"-q:v", "5",            // Quality (2-31, lower is better)
		"-f", "image2pipe",     // Output as image stream
		"-vcodec", "mjpeg",     // MJPEG output
		"-",                    // Output to stdout
	)

	// Get stdout pipe
	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	c.stdout = stdout

	// Get stderr for error logging
	stderr, err := c.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	c.running = true
	log.Println("✅ FFmpeg streaming started at 10 FPS")

	// Log stderr in background
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			// Only log important errors, not info messages
			if len(line) > 0 && (bytes.Contains([]byte(line), []byte("error")) || 
				bytes.Contains([]byte(line), []byte("Error")) ||
				bytes.Contains([]byte(line), []byte("failed"))) {
				log.Printf("FFmpeg: %s", line)
			}
		}
	}()

	// Start frame reader
	go c.readFrames()

	return nil
}

func (c *FFmpegCapturer) readFrames() {
	reader := bufio.NewReader(c.stdout)
	
	for c.running {
		// Read a JPEG frame
		// JPEG starts with 0xFFD8 and ends with 0xFFD9
		frame, err := c.readJPEGFrame(reader)
		if err != nil {
			if err == io.EOF {
				log.Println("⚠️ FFmpeg stream ended")
				c.running = false
				break
			}
			// Log error but continue
			if c.running {
				log.Printf("⚠️ Error reading frame: %v", err)
				time.Sleep(100 * time.Millisecond)
			}
			continue
		}

		// Store the latest frame
		c.frameMutex.Lock()
		c.latestFrame = frame
		c.frameMutex.Unlock()
	}
}

func (c *FFmpegCapturer) readJPEGFrame(reader *bufio.Reader) ([]byte, error) {
	// JPEG magic bytes
	const jpegStart1 = 0xFF
	const jpegStart2 = 0xD8
	const jpegEnd1 = 0xFF
	const jpegEnd2 = 0xD9

	var frame bytes.Buffer

	// Find JPEG start marker (0xFFD8)
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}

		if b == jpegStart1 {
			next, err := reader.ReadByte()
			if err != nil {
				return nil, err
			}

			if next == jpegStart2 {
				// Found start marker
				frame.WriteByte(jpegStart1)
				frame.WriteByte(jpegStart2)
				break
			}
		}
	}

	// Read until JPEG end marker (0xFFD9)
	prevByte := byte(0)
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}

		frame.WriteByte(b)

		if prevByte == jpegEnd1 && b == jpegEnd2 {
			// Found end marker
			break
		}

		prevByte = b
	}

	return frame.Bytes(), nil
}

func (c *FFmpegCapturer) CaptureJPEG(quality int) ([]byte, error) {
	// Return the latest frame from the stream
	// Quality parameter is ignored since FFmpeg is already running
	
	c.frameMutex.RLock()
	frame := c.latestFrame
	c.frameMutex.RUnlock()

	if frame == nil {
		return nil, fmt.Errorf("no frame available yet")
	}

	// Return a copy to avoid data races
	frameCopy := make([]byte, len(frame))
	copy(frameCopy, frame)
	
	return frameCopy, nil
}

func (c *FFmpegCapturer) GetBounds() image.Rectangle {
	return image.Rect(0, 0, c.width, c.height)
}

func (c *FFmpegCapturer) GetResolution() (int, int) {
	return c.width, c.height
}

func (c *FFmpegCapturer) Close() error {
	c.running = false
	
	if c.cmd != nil && c.cmd.Process != nil {
		// Kill the FFmpeg process
		if err := c.cmd.Process.Kill(); err != nil {
			log.Printf("⚠️ Failed to kill FFmpeg process: %v", err)
		}
		c.cmd.Wait()
	}
	
	if c.stdout != nil {
		c.stdout.Close()
	}
	
	close(c.stopChan)
	log.Println("🛑 FFmpeg capturer closed")
	
	return nil
}

// getScreenResolution gets the primary screen resolution on Windows
func getScreenResolution() (int, int) {
	// Try to detect via PowerShell (more reliable)
	cmd := exec.Command("powershell", "-Command",
		"Add-Type -AssemblyName System.Windows.Forms; "+
			"[System.Windows.Forms.Screen]::PrimaryScreen.Bounds.Width; "+
			"[System.Windows.Forms.Screen]::PrimaryScreen.Bounds.Height")
	
	output, err := cmd.Output()
	if err == nil {
		var w, h int
		if _, err := fmt.Sscanf(string(output), "%d\n%d", &w, &h); err == nil {
			return w, h
		}
	}

	// Default to 1920x1080 if detection fails
	return 1920, 1080
}
