package viewer

import (
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// Mouse move throttle interval (8ms = ~120 FPS for instant response)
const mouseThrottleInterval = 8 * time.Millisecond

// InteractiveCanvas is a canvas that captures mouse and keyboard events
type InteractiveCanvas struct {
	widget.BaseWidget
	image         *canvas.Image
	lastMouseX    float32
	lastMouseY    float32
	onMouseMove   func(x, y float32)
	onMouseButton func(button desktop.MouseButton, pressed bool, x, y float32)
	onMouseScroll func(deltaX, deltaY float32)
	onKeyPress    func(key *fyne.KeyEvent)
	onKeyDown     func(key *fyne.KeyEvent, modifier desktop.Modifier)
	onKeyUp       func(key *fyne.KeyEvent, modifier desktop.Modifier)

	// Throttling for mouse move
	lastMoveTime  time.Time
	pendingMove   bool
	pendingX      float32
	pendingY      float32
	throttleMu    sync.Mutex
	throttleTimer *time.Timer
}

// NewInteractiveCanvas creates a new interactive canvas
func NewInteractiveCanvas(img *canvas.Image) *InteractiveCanvas {
	ic := &InteractiveCanvas{
		image: img,
	}
	ic.ExtendBaseWidget(ic)
	return ic
}

// CreateRenderer creates the renderer for this widget
func (ic *InteractiveCanvas) CreateRenderer() fyne.WidgetRenderer {
	return &interactiveCanvasRenderer{
		canvas: ic,
		image:  ic.image,
	}
}

// Mouse events
func (ic *InteractiveCanvas) MouseIn(*desktop.MouseEvent) {}
func (ic *InteractiveCanvas) MouseOut()                   {}

func (ic *InteractiveCanvas) MouseMoved(event *desktop.MouseEvent) {
	ic.lastMouseX = event.Position.X
	ic.lastMouseY = event.Position.Y

	if ic.onMouseMove == nil {
		return
	}

	ic.throttleMu.Lock()
	defer ic.throttleMu.Unlock()

	now := time.Now()
	timeSinceLastMove := now.Sub(ic.lastMoveTime)

	// If enough time has passed, send immediately
	if timeSinceLastMove >= mouseThrottleInterval {
		ic.lastMoveTime = now
		ic.pendingMove = false
		if ic.throttleTimer != nil {
			ic.throttleTimer.Stop()
			ic.throttleTimer = nil
		}
		// Send the move event
		go ic.onMouseMove(event.Position.X, event.Position.Y)
		return
	}

	// Otherwise, store pending move and schedule send
	ic.pendingX = event.Position.X
	ic.pendingY = event.Position.Y
	ic.pendingMove = true

	// Schedule a timer to send the pending move if not already scheduled
	if ic.throttleTimer == nil {
		remaining := mouseThrottleInterval - timeSinceLastMove
		ic.throttleTimer = time.AfterFunc(remaining, func() {
			ic.throttleMu.Lock()
			defer ic.throttleMu.Unlock()

			if ic.pendingMove && ic.onMouseMove != nil {
				ic.lastMoveTime = time.Now()
				ic.pendingMove = false
				x, y := ic.pendingX, ic.pendingY
				go ic.onMouseMove(x, y)
			}
			ic.throttleTimer = nil
		})
	}
}

func (ic *InteractiveCanvas) MouseDown(event *desktop.MouseEvent) {
	ic.lastMouseX = event.Position.X
	ic.lastMouseY = event.Position.Y
	if ic.onMouseButton != nil {
		ic.onMouseButton(event.Button, true, event.Position.X, event.Position.Y)
	}
}

func (ic *InteractiveCanvas) MouseUp(event *desktop.MouseEvent) {
	ic.lastMouseX = event.Position.X
	ic.lastMouseY = event.Position.Y
	if ic.onMouseButton != nil {
		ic.onMouseButton(event.Button, false, event.Position.X, event.Position.Y)
	}
}

// Scroll events
func (ic *InteractiveCanvas) Scrolled(event *fyne.ScrollEvent) {
	if ic.onMouseScroll != nil {
		ic.onMouseScroll(event.Scrolled.DX, event.Scrolled.DY)
	}
}

// Keyboard events
func (ic *InteractiveCanvas) TypedRune(rune) {}

func (ic *InteractiveCanvas) TypedKey(event *fyne.KeyEvent) {
	// Only use TypedKey as fallback if KeyDown/KeyUp not set
	if ic.onKeyDown == nil && ic.onKeyPress != nil {
		ic.onKeyPress(event)
	}
}

// desktop.Keyable interface — provides modifier state and separate down/up
func (ic *InteractiveCanvas) KeyDown(event *fyne.KeyEvent, modifier desktop.Modifier) {
	if ic.onKeyDown != nil {
		ic.onKeyDown(event, modifier)
	}
}

func (ic *InteractiveCanvas) KeyUp(event *fyne.KeyEvent, modifier desktop.Modifier) {
	if ic.onKeyUp != nil {
		ic.onKeyUp(event, modifier)
	}
}

// Focusable
func (ic *InteractiveCanvas) FocusGained() {}
func (ic *InteractiveCanvas) FocusLost()   {}

// Tappable (for mobile/touch)
func (ic *InteractiveCanvas) Tapped(*fyne.PointEvent)          {}
func (ic *InteractiveCanvas) TappedSecondary(*fyne.PointEvent) {}

// Setters for callbacks
func (ic *InteractiveCanvas) SetOnMouseMove(callback func(x, y float32)) {
	ic.onMouseMove = callback
}

func (ic *InteractiveCanvas) SetOnMouseButton(callback func(button desktop.MouseButton, pressed bool, x, y float32)) {
	ic.onMouseButton = callback
}

func (ic *InteractiveCanvas) SetOnMouseScroll(callback func(deltaX, deltaY float32)) {
	ic.onMouseScroll = callback
}

func (ic *InteractiveCanvas) SetOnKeyPress(callback func(key *fyne.KeyEvent)) {
	ic.onKeyPress = callback
}

func (ic *InteractiveCanvas) SetOnKeyDown(callback func(key *fyne.KeyEvent, modifier desktop.Modifier)) {
	ic.onKeyDown = callback
}

func (ic *InteractiveCanvas) SetOnKeyUp(callback func(key *fyne.KeyEvent, modifier desktop.Modifier)) {
	ic.onKeyUp = callback
}

// Renderer
type interactiveCanvasRenderer struct {
	canvas *InteractiveCanvas
	image  *canvas.Image
}

func (r *interactiveCanvasRenderer) Layout(size fyne.Size) {
	r.image.Resize(size)
}

func (r *interactiveCanvasRenderer) MinSize() fyne.Size {
	return fyne.NewSize(320, 240) // Allow smaller window sizes
}

func (r *interactiveCanvasRenderer) Refresh() {
	r.image.Refresh()
}

func (r *interactiveCanvasRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.image}
}

func (r *interactiveCanvasRenderer) Destroy() {}
