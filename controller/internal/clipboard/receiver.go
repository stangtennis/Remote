package clipboard

import (
	"bytes"
	"image"
	"image/png"
	"log"

	"golang.design/x/clipboard"
)

// Receiver handles incoming clipboard data from the remote agent
type Receiver struct {
	initialized bool
}

// NewReceiver creates a new clipboard receiver
func NewReceiver() *Receiver {
	return &Receiver{}
}

// Initialize initializes the clipboard system
func (r *Receiver) Initialize() error {
	if !r.initialized {
		err := clipboard.Init()
		if err != nil {
			return err
		}
		r.initialized = true
		log.Println("📋 Clipboard receiver initialized")
	}
	return nil
}

// SetText sets the local clipboard to the given text
func (r *Receiver) SetText(text string) error {
	if !r.initialized {
		if err := r.Initialize(); err != nil {
			return err
		}
	}

	clipboard.Write(clipboard.FmtText, []byte(text))
	log.Printf("📋 Clipboard updated with text (%d bytes)", len(text))
	return nil
}

// SetImage sets the local clipboard to the given image data
func (r *Receiver) SetImage(imageData []byte) error {
	if !r.initialized {
		if err := r.Initialize(); err != nil {
			return err
		}
	}

	// Decode PNG to ensure it's valid
	img, err := png.Decode(bytes.NewReader(imageData))
	if err != nil {
		log.Printf("❌ Failed to decode image: %v", err)
		return err
	}

	// Re-encode to ensure proper format
	var buf bytes.Buffer
	err = png.Encode(&buf, img)
	if err != nil {
		log.Printf("❌ Failed to encode image: %v", err)
		return err
	}

	clipboard.Write(clipboard.FmtImage, buf.Bytes())
	log.Printf("📋 Clipboard updated with image (%d bytes)", buf.Len())
	return nil
}

// SetImageRaw sets the local clipboard to raw image data without validation
func (r *Receiver) SetImageRaw(imageData []byte) error {
	if !r.initialized {
		if err := r.Initialize(); err != nil {
			return err
		}
	}

	clipboard.Write(clipboard.FmtImage, imageData)
	log.Printf("📋 Clipboard updated with raw image (%d bytes)", len(imageData))
	return nil
}

// GetText retrieves text from the local clipboard
func (r *Receiver) GetText() (string, error) {
	if !r.initialized {
		if err := r.Initialize(); err != nil {
			return "", err
		}
	}

	data := clipboard.Read(clipboard.FmtText)
	return string(data), nil
}

// GetImage retrieves image data from the local clipboard
func (r *Receiver) GetImage() (image.Image, error) {
	if !r.initialized {
		if err := r.Initialize(); err != nil {
			return nil, err
		}
	}

	data := clipboard.Read(clipboard.FmtImage)
	if len(data) == 0 {
		return nil, nil
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	return img, err
}
