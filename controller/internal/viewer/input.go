package viewer

import (
	"log"
)

// InputHandler manages keyboard and mouse input
type InputHandler struct {
	viewer      *Viewer
	enabled     bool
	mouseX      float32
	mouseY      float32
	
	// Callbacks for sending input to remote
	onMouseMove   func(x, y float32)
	onMouseButton func(button int, pressed bool)
	onMouseScroll func(deltaX, deltaY float32)
	onKeyPress    func(key string, pressed bool)
}

// NewInputHandler creates a new input handler
func NewInputHandler(viewer *Viewer) *InputHandler {
	return &InputHandler{
		viewer:  viewer,
		enabled: true,
	}
}

// Enable enables input capture
func (h *InputHandler) Enable() {
	h.enabled = true
	log.Println("Input capture enabled")
}

// Disable disables input capture
func (h *InputHandler) Disable() {
	h.enabled = false
	log.Println("Input capture disabled")
}

// AttachToCanvas attaches input handlers to the video canvas
// Note: Full input capture requires custom widget implementation
// This will be completed when WebRTC connection is implemented
func (h *InputHandler) AttachToCanvas() {
	// TODO: Implement custom tappable widget for full mouse/keyboard capture
	// For now, input will be captured at window level
	log.Println("Input handler attached (window-level capture)")
	
	// Window-level keyboard capture will be implemented in WebRTC integration
}

// Mouse event handlers
func (h *InputHandler) handleMouseMove(x, y float32) {
	h.mouseX = x
	h.mouseY = y
	
	if h.onMouseMove != nil {
		// Convert to remote screen coordinates
		remoteX, remoteY := h.convertToRemoteCoords(x, y)
		h.onMouseMove(remoteX, remoteY)
	}
}

func (h *InputHandler) handleMouseButton(button int, pressed bool) {
	log.Printf("Mouse button %d %s", button, map[bool]string{true: "pressed", false: "released"}[pressed])
	
	if h.onMouseButton != nil {
		h.onMouseButton(button, pressed)
	}
}

func (h *InputHandler) handleMouseScroll(deltaX, deltaY float32) {
	log.Printf("Mouse scroll: dx=%.2f, dy=%.2f", deltaX, deltaY)
	
	if h.onMouseScroll != nil {
		h.onMouseScroll(deltaX, deltaY)
	}
}

// Keyboard event handlers
func (h *InputHandler) HandleKeyPress(key string, pressed bool) {
	if !h.enabled {
		return
	}
	
	log.Printf("Key %s %s", key, map[bool]string{true: "pressed", false: "released"}[pressed])
	
	if h.onKeyPress != nil {
		h.onKeyPress(key, pressed)
	}
}

// Coordinate conversion
func (h *InputHandler) convertToRemoteCoords(localX, localY float32) (float32, float32) {
	// Get canvas size
	canvasSize := h.viewer.videoCanvas.Size()
	
	// Assume remote is 1920x1080 (Full HD)
	remoteWidth := float32(1920)
	remoteHeight := float32(1080)
	
	// Calculate scale
	scaleX := remoteWidth / canvasSize.Width
	scaleY := remoteHeight / canvasSize.Height
	
	// Convert coordinates
	remoteX := localX * scaleX
	remoteY := localY * scaleY
	
	return remoteX, remoteY
}

// Callback setters
func (h *InputHandler) SetOnMouseMove(callback func(x, y float32)) {
	h.onMouseMove = callback
}

func (h *InputHandler) SetOnMouseButton(callback func(button int, pressed bool)) {
	h.onMouseButton = callback
}

func (h *InputHandler) SetOnMouseScroll(callback func(deltaX, deltaY float32)) {
	h.onMouseScroll = callback
}

func (h *InputHandler) SetOnKeyPress(callback func(key string, pressed bool)) {
	h.onKeyPress = callback
}
