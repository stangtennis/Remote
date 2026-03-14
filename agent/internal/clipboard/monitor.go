package clipboard

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/png"
	"log"

	"golang.design/x/clipboard"
)

// Monitor watches the system clipboard for changes using native events
type Monitor struct {
	lastTextHash  string
	lastImageHash string
	onTextChange  func(text string)
	onImageChange func(imageData []byte)
	cancelFunc    context.CancelFunc
	running       bool
}

// NewMonitor creates a new clipboard monitor
func NewMonitor() *Monitor {
	return &Monitor{}
}

// SetOnTextChange sets the callback for text clipboard changes
func (m *Monitor) SetOnTextChange(callback func(text string)) {
	m.onTextChange = callback
}

// SetOnImageChange sets the callback for image clipboard changes
func (m *Monitor) SetOnImageChange(callback func(imageData []byte)) {
	m.onImageChange = callback
}

// Start begins monitoring the clipboard using native OS events (no polling)
func (m *Monitor) Start() error {
	err := clipboard.Init()
	if err != nil {
		return err
	}

	m.running = true

	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFunc = cancel

	// Use clipboard.Watch for native event-driven monitoring (uses WM_CLIPBOARDUPDATE on Windows)
	textCh := clipboard.Watch(ctx, clipboard.FmtText)
	imgCh := clipboard.Watch(ctx, clipboard.FmtImage)

	go m.watchText(textCh)
	go m.watchImage(imgCh)

	log.Println("📋 Clipboard monitor started (native events)")
	return nil
}

// Stop stops monitoring the clipboard
func (m *Monitor) Stop() {
	if m.running {
		m.running = false
		if m.cancelFunc != nil {
			m.cancelFunc()
		}
		log.Println("📋 Clipboard monitor stopped")
	}
}

// watchText monitors text clipboard changes via native events
func (m *Monitor) watchText(ch <-chan []byte) {
	for data := range ch {
		if !m.running {
			return
		}
		if len(data) == 0 || len(data) > 10*1024*1024 {
			continue
		}
		text := string(data)
		hash := hashString(text)
		if hash != m.lastTextHash {
			m.lastTextHash = hash
			log.Printf("📋 Text clipboard changed (%d bytes)", len(text))
			if m.onTextChange != nil {
				m.onTextChange(text)
			}
		}
	}
}

// watchImage monitors image clipboard changes via native events
func (m *Monitor) watchImage(ch <-chan []byte) {
	for data := range ch {
		if !m.running {
			return
		}
		if len(data) == 0 || len(data) > 50*1024*1024 {
			continue
		}
		hash := hashBytes(data)
		if hash != m.lastImageHash && m.onImageChange != nil {
			m.lastImageHash = hash
			pngData, err := convertImageToPNG(data)
			if err != nil {
				log.Printf("❌ Failed to convert image: %v", err)
				continue
			}
			log.Printf("📋 Image clipboard changed (%d bytes)", len(pngData))
			m.onImageChange(pngData)
		}
	}
}

// RememberText marks text as handled to avoid echo loops when we just set the clipboard ourselves.
func (m *Monitor) RememberText(text string) {
	m.lastTextHash = hashString(text)
}

// RememberImage marks image data as handled to avoid echo loops.
func (m *Monitor) RememberImage(data []byte) {
	m.lastImageHash = hashBytes(data)
}

// hashString creates a hash of a string
func hashString(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

// hashBytes creates a hash of byte data
func hashBytes(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// convertImageToPNG converts image data to PNG format
func convertImageToPNG(data []byte) ([]byte, error) {
	// Try to decode the image
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		// If it's already in a format we can't decode, just return as-is
		return data, nil
	}

	// Encode to PNG
	var buf bytes.Buffer
	err = png.Encode(&buf, img)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
