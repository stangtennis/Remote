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

// Monitor watches the system clipboard for changes (controller side).
type Monitor struct {
	lastTextHash  string
	lastImageHash string
	onTextChange  func(text string)
	onImageChange func(imageData []byte)
	stopChan      chan bool
	running       bool
}

func NewMonitor() *Monitor {
	return &Monitor{
		stopChan: make(chan bool),
	}
}

func (m *Monitor) SetOnTextChange(callback func(text string)) {
	m.onTextChange = callback
}

func (m *Monitor) SetOnImageChange(callback func(imageData []byte)) {
	m.onImageChange = callback
}

func (m *Monitor) Start() error {
	if m.running {
		return nil
	}

	if err := clipboard.Init(); err != nil {
		return err
	}

	m.running = true
	go m.monitorLoop()
	log.Println("?? Clipboard monitor (controller) started")
	return nil
}

func (m *Monitor) Stop() {
	if m.running {
		m.running = false
		close(m.stopChan)
		log.Println("?? Clipboard monitor (controller) stopped")
	}
}

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

func (m *Monitor) checkClipboard() {
	// Text
	textData := clipboard.Read(clipboard.FmtText)
	if len(textData) > 0 {
		text := string(textData)
		hash := hashString(text)

		if hash != m.lastTextHash && m.onTextChange != nil {
			m.lastTextHash = hash

			if len(text) > 10*1024*1024 {
				log.Println("?? Clipboard text too large (>10MB), skipping send")
				return
			}

			log.Printf("?? Local clipboard text changed (%d bytes)", len(text))
			m.onTextChange(text)
		}
	}

	// Image
	imageData := clipboard.Read(clipboard.FmtImage)
	if len(imageData) > 0 {
		hash := hashBytes(imageData)

		if hash != m.lastImageHash && m.onImageChange != nil {
			m.lastImageHash = hash

			if len(imageData) > 50*1024*1024 {
				log.Println("?? Clipboard image too large (>50MB), skipping send")
				return
			}

			pngData, err := convertImageToPNG(imageData)
			if err != nil {
				log.Printf("? Failed to convert image: %v", err)
				return
			}

			log.Printf("?? Local clipboard image changed (%d bytes)", len(pngData))
			m.onImageChange(pngData)
		}
	}
}

// RememberText marks the provided text as already seen to prevent echo loops.
func (m *Monitor) RememberText(text string) {
	m.lastTextHash = hashString(text)
}

// RememberImage marks the provided image as already seen to prevent echo loops.
func (m *Monitor) RememberImage(data []byte) {
	m.lastImageHash = hashBytes(data)
}

func hashString(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

func hashBytes(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func convertImageToPNG(data []byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data, nil
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
