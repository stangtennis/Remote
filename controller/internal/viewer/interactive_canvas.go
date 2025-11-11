package viewer

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

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
func (ic *InteractiveCanvas) MouseOut() {}

func (ic *InteractiveCanvas) MouseMoved(event *desktop.MouseEvent) {
	ic.lastMouseX = event.Position.X
	ic.lastMouseY = event.Position.Y
	if ic.onMouseMove != nil {
		ic.onMouseMove(event.Position.X, event.Position.Y)
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
	if ic.onKeyPress != nil {
		ic.onKeyPress(event)
	}
}

// Focusable
func (ic *InteractiveCanvas) FocusGained() {}
func (ic *InteractiveCanvas) FocusLost()   {}

// Tappable (for mobile/touch)
func (ic *InteractiveCanvas) Tapped(*fyne.PointEvent) {}
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

// Renderer
type interactiveCanvasRenderer struct {
	canvas *InteractiveCanvas
	image  *canvas.Image
}

func (r *interactiveCanvasRenderer) Layout(size fyne.Size) {
	r.image.Resize(size)
}

func (r *interactiveCanvasRenderer) MinSize() fyne.Size {
	return fyne.NewSize(640, 480)
}

func (r *interactiveCanvasRenderer) Refresh() {
	r.image.Refresh()
}

func (r *interactiveCanvasRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.image}
}

func (r *interactiveCanvasRenderer) Destroy() {}
