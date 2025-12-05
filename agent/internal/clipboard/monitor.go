package clipboard

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/png"
	"log"
	"time"

	"golang.design/x/clipboard"
)

// Monitor watches the system clipboard for changes
type Monitor struct {
	lastTextHash  string
	lastImageHash string
	onTextChange  func(text string)
	onImageChange func(imageData []byte)
	stopChan      chan bool
	running       bool
}

// NewMonitor creates a new clipboard monitor
func NewMonitor() *Monitor {
	return &Monitor{
		stopChan: make(chan bool),
	}
}

// SetOnTextChange sets the callback for text clipboard changes
func (m *Monitor) SetOnTextChange(callback func(text string)) {
	m.onTextChange = callback
}

// SetOnImageChange sets the callback for image clipboard changes
func (m *Monitor) SetOnImageChange(callback func(imageData []byte)) {
	m.onImageChange = callback
}

// Start begins monitoring the clipboard
func (m *Monitor) Start() error {
	// Initialize clipboard
	err := clipboard.Init()
	if err != nil {
		return err
	}

	m.running = true
	go m.monitorLoop()
	log.Println("📋 Clipboard monitor started")
	return nil
}

// Stop stops monitoring the clipboard
func (m *Monitor) Stop() {
	if m.running {
		m.running = false
		close(m.stopChan)
		log.Println("📋 Clipboard monitor stopped")
	}
}

// monitorLoop continuously checks the clipboard for changes
func (m *Monitor) monitorLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.checkClipboard()
		}
	}
}

// checkClipboard checks for clipboard changes and triggers callbacks
func (m *Monitor) checkClipboard() {
	// Check for text
	textData := clipboard.Read(clipboard.FmtText)
	if len(textData) > 0 {
		text := string(textData)
		hash := hashString(text)

		if hash != m.lastTextHash {
			m.lastTextHash = hash

			// Size limit: 10MB
			if len(text) > 10*1024*1024 {
				log.Println("⚠️ Clipboard text too large (>10MB), skipping")
				return
			}

			log.Printf("📋 Text clipboard changed (%d bytes): %s...", len(text), truncateString(text, 50))
			if m.onTextChange != nil {
				m.onTextChange(text)
			} else {
				log.Println("⚠️ No text change callback set!")
			}
		}
	}

	// Check for image
	imageData := clipboard.Read(clipboard.FmtImage)
	if len(imageData) > 0 {
		hash := hashBytes(imageData)

		if hash != m.lastImageHash && m.onImageChange != nil {
			m.lastImageHash = hash

			// Size limit: 50MB
			if len(imageData) > 50*1024*1024 {
				log.Println("⚠️ Clipboard image too large (>50MB), skipping")
				return
			}

			// Convert to PNG format for consistent transmission
			pngData, err := convertImageToPNG(imageData)
			if err != nil {
				log.Printf("❌ Failed to convert image: %v", err)
				return
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
