package clipboard

import (
	"bytes"
	"image/png"
	"log"

	"golang.design/x/clipboard"
)

// Receiver applies incoming clipboard data locally on the agent.
type Receiver struct {
	initialized bool
}

func NewReceiver() *Receiver {
	return &Receiver{}
}

func (r *Receiver) ensureInit() error {
	if r.initialized {
		return nil
	}
	if err := clipboard.Init(); err != nil {
		return err
	}
	r.initialized = true
	log.Println("?? Clipboard receiver (agent) initialized")
	return nil
}

func (r *Receiver) SetText(text string) error {
	if err := r.ensureInit(); err != nil {
		return err
	}
	clipboard.Write(clipboard.FmtText, []byte(text))
	log.Printf("?? Agent clipboard updated with text (%d bytes)", len(text))
	return nil
}

func (r *Receiver) SetImage(imageData []byte) error {
	if err := r.ensureInit(); err != nil {
		return err
	}

	// Validate/normalize to PNG
	img, err := png.Decode(bytes.NewReader(imageData))
	if err != nil {
		// If decode fails, try raw write
		clipboard.Write(clipboard.FmtImage, imageData)
		log.Printf("?? Agent clipboard set with raw image (%d bytes)", len(imageData))
		return nil
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}

	clipboard.Write(clipboard.FmtImage, buf.Bytes())
	log.Printf("?? Agent clipboard updated with image (%d bytes)", buf.Len())
	return nil
}
