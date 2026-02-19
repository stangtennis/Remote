//go:build windows
// +build windows

package screen

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows"
)

// RunCaptureHelper runs as a capture helper process in the user's session.
// It connects to the named pipe created by the service, captures the screen
// via GDI (which works in the user session), and sends frames on demand.
func RunCaptureHelper(pipeName string) error {
	// Set up logging to temp file
	logDir := filepath.Join(os.TempDir(), "RemoteDesktopAgent")
	os.MkdirAll(logDir, 0755)
	logFile, err := os.OpenFile(
		filepath.Join(logDir, "capture-helper.log"),
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		0644,
	)
	if err == nil {
		log.SetOutput(logFile)
		defer logFile.Close()
	}

	log.Printf("🎬 Capture helper starting, pipe: %s", pipeName)
	log.Printf("   PID: %d, Session: user session", os.Getpid())

	// Connect to the named pipe (retry for up to 30 seconds)
	pipeNameUTF16, _ := windows.UTF16PtrFromString(pipeName)
	var pipeHandle windows.Handle
	for i := 0; i < 30; i++ {
		pipeHandle, err = windows.CreateFile(
			pipeNameUTF16,
			windows.GENERIC_READ|windows.GENERIC_WRITE,
			0, nil,
			windows.OPEN_EXISTING,
			0, 0,
		)
		if err == nil {
			break
		}
		log.Printf("⏳ Waiting for pipe... attempt %d (%v)", i+1, err)
		time.Sleep(time.Second)
	}
	if err != nil {
		return fmt.Errorf("failed to connect to pipe after 30 attempts: %w", err)
	}
	defer windows.CloseHandle(pipeHandle)

	pipe := &pipeRW{handle: pipeHandle}
	log.Println("✅ Connected to capture pipe")

	// Initialize GDI capturer (we're in the user's session, so GDI works!)
	gdi, err := NewGDICapturer()
	if err != nil {
		return fmt.Errorf("GDI init failed: %w", err)
	}
	defer gdi.Close()
	log.Printf("✅ GDI capturer ready: %dx%d", gdi.width, gdi.height)

	// Send initial resolution (8 bytes: uint32 width + uint32 height)
	resoBuf := make([]byte, 8)
	binary.LittleEndian.PutUint32(resoBuf[0:4], uint32(gdi.width))
	binary.LittleEndian.PutUint32(resoBuf[4:8], uint32(gdi.height))
	if _, err := pipe.Write(resoBuf); err != nil {
		return fmt.Errorf("failed to send initial resolution: %w", err)
	}
	log.Println("📐 Sent initial resolution")

	// Main loop: read commands, capture and send frames
	cmdBuf := make([]byte, 1)
	frameCount := 0
	errorCount := 0
	startTime := time.Now()

	for {
		// Wait for command from service
		if _, err := io.ReadFull(pipe, cmdBuf); err != nil {
			log.Printf("❌ Pipe read error: %v", err)
			return err
		}

		switch cmdBuf[0] {
		case 0x01: // Capture BGRA frame
			bgra, w, h, err := gdi.CaptureBGRA()
			if err != nil {
				errorCount++
				if errorCount%100 == 1 {
					log.Printf("⚠️ Capture error #%d: %v", errorCount, err)
				}
				// Send error response (0x0 dimensions = error)
				errBuf := make([]byte, 8)
				pipe.Write(errBuf)
				continue
			}

			// Send frame: uint32(width) + uint32(height) + BGRA data
			hdr := make([]byte, 8)
			binary.LittleEndian.PutUint32(hdr[0:4], uint32(w))
			binary.LittleEndian.PutUint32(hdr[4:8], uint32(h))
			if _, err := pipe.Write(hdr); err != nil {
				return fmt.Errorf("pipe write header: %w", err)
			}
			if _, err := pipe.Write(bgra); err != nil {
				return fmt.Errorf("pipe write frame: %w", err)
			}

			frameCount++
			if frameCount%300 == 0 {
				elapsed := time.Since(startTime).Seconds()
				fps := float64(frameCount) / elapsed
				log.Printf("📊 Frames: %d (%.1f fps, %dx%d, errors: %d)", frameCount, fps, w, h, errorCount)
			}

		case 0xFF: // Quit
			log.Printf("👋 Quit command received (sent %d frames)", frameCount)
			return nil
		}
	}
}
