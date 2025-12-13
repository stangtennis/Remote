package webrtc

import (
	"bytes"
	"fmt"
	"image/jpeg"
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
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	onFrame   func([]byte) // Callback for decoded JPEG frames
	running   bool
	mu        sync.Mutex
	stopChan  chan struct{}
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
	// -f h264: input format is raw H.264
	// -i pipe:0: read from stdin
	// -f image2pipe: output as image stream
	// -vcodec mjpeg: encode output as MJPEG (easy to parse)
	// -q:v 2: high quality JPEG (1-31, lower is better)
	// pipe:1: write to stdout
	d.cmd = exec.Command(ffmpegPath,
		"-hide_banner",
		"-loglevel", "error",
		"-f", "h264",
		"-i", "pipe:0",
		"-f", "image2pipe",
		"-vcodec", "mjpeg",
		"-q:v", "2",
		"pipe:1",
	)

	var err error
	d.stdin, err = d.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	d.stdout, err = d.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err := d.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	d.running = true
	log.Println("🎬 FFmpeg H.264 decoder started")

	// Start goroutine to read decoded frames
	go d.readFrames()

	return nil
}

// readFrames reads MJPEG frames from FFmpeg stdout
func (d *H264Decoder) readFrames() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("❌ H264Decoder readFrames panic: %v", r)
		}
	}()

	// JPEG markers
	const (
		jpegSOI = 0xFFD8 // Start of Image
		jpegEOI = 0xFFD9 // End of Image
	)

	buf := make([]byte, 1024*1024) // 1MB buffer
	var frameBuf bytes.Buffer

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
			// Find SOI marker
			soiIdx := bytes.Index(data, []byte{0xFF, 0xD8})
			if soiIdx == -1 {
				break
			}

			// Find EOI marker after SOI
			eoiIdx := bytes.Index(data[soiIdx+2:], []byte{0xFF, 0xD9})
			if eoiIdx == -1 {
				break
			}
			eoiIdx += soiIdx + 2 + 2 // Include the EOI marker itself

			// Extract complete JPEG frame
			jpegData := data[soiIdx:eoiIdx]

			// Verify it's a valid JPEG
			if _, err := jpeg.DecodeConfig(bytes.NewReader(jpegData)); err == nil {
				// Send to callback
				if d.onFrame != nil {
					d.onFrame(jpegData)
				}
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
func EnsureAnnexB(nalUnits []byte) []byte {
	// Check if already has start code
	if len(nalUnits) >= 4 && nalUnits[0] == 0 && nalUnits[1] == 0 && nalUnits[2] == 0 && nalUnits[3] == 1 {
		return nalUnits
	}
	if len(nalUnits) >= 3 && nalUnits[0] == 0 && nalUnits[1] == 0 && nalUnits[2] == 1 {
		return nalUnits
	}

	// Add start code
	result := make([]byte, 4+len(nalUnits))
	result[0] = 0
	result[1] = 0
	result[2] = 0
	result[3] = 1
	copy(result[4:], nalUnits)
	return result
}
