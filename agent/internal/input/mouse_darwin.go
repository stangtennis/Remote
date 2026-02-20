//go:build darwin

package input

/*
#cgo LDFLAGS: -framework CoreGraphics -framework ApplicationServices
#include <CoreGraphics/CoreGraphics.h>
#include <ApplicationServices/ApplicationServices.h>

static void mouseMove(double x, double y) {
    CGEventRef event = CGEventCreateMouseEvent(NULL, kCGEventMouseMoved,
        CGPointMake(x, y), kCGMouseButtonLeft);
    CGEventPost(kCGHIDEventTap, event);
    CFRelease(event);
}

static void mouseDown(double x, double y, int button) {
    CGEventType type;
    CGMouseButton btn;
    switch (button) {
    case 1:
        type = kCGEventRightMouseDown;
        btn = kCGMouseButtonRight;
        break;
    case 2:
        type = kCGEventOtherMouseDown;
        btn = kCGMouseButtonCenter;
        break;
    default:
        type = kCGEventLeftMouseDown;
        btn = kCGMouseButtonLeft;
        break;
    }
    CGEventRef event = CGEventCreateMouseEvent(NULL, type, CGPointMake(x, y), btn);
    CGEventPost(kCGHIDEventTap, event);
    CFRelease(event);
}

static void mouseUp(double x, double y, int button) {
    CGEventType type;
    CGMouseButton btn;
    switch (button) {
    case 1:
        type = kCGEventRightMouseUp;
        btn = kCGMouseButtonRight;
        break;
    case 2:
        type = kCGEventOtherMouseUp;
        btn = kCGMouseButtonCenter;
        break;
    default:
        type = kCGEventLeftMouseUp;
        btn = kCGMouseButtonLeft;
        break;
    }
    CGEventRef event = CGEventCreateMouseEvent(NULL, type, CGPointMake(x, y), btn);
    CGEventPost(kCGHIDEventTap, event);
    CFRelease(event);
}

static void mouseScroll(int dy) {
    CGEventRef event = CGEventCreateScrollWheelEvent(NULL, kCGScrollEventUnitLine, 1, dy);
    CGEventPost(kCGHIDEventTap, event);
    CFRelease(event);
}
*/
import "C"
import (
	"fmt"
	"math"
)

type MouseController struct {
	screenWidth  int
	screenHeight int
	offsetX      int
	offsetY      int
	cursorHidden bool
	lastX        float64
	lastY        float64
}

func NewMouseController(width, height int) *MouseController {
	return &MouseController{
		screenWidth:  width,
		screenHeight: height,
	}
}

func (m *MouseController) SetMonitorOffset(offsetX, offsetY int) {
	m.offsetX = offsetX
	m.offsetY = offsetY
}

func (m *MouseController) SetResolution(width, height int) {
	m.screenWidth = width
	m.screenHeight = height
}

func (m *MouseController) Move(x, y float64) error {
	screenX := math.Round(x)
	screenY := math.Round(y)

	screenX = clampFloat(screenX, 0, float64(m.screenWidth-1))
	screenY = clampFloat(screenY, 0, float64(m.screenHeight-1))

	m.lastX = screenX
	m.lastY = screenY

	C.mouseMove(C.double(screenX), C.double(screenY))
	return nil
}

func (m *MouseController) MoveRelative(x, y float64) error {
	screenX := x * float64(m.screenWidth)
	screenY := y * float64(m.screenHeight)

	screenX = clampFloat(screenX, 0, float64(m.screenWidth-1))
	screenY = clampFloat(screenY, 0, float64(m.screenHeight-1))

	screenX += float64(m.offsetX)
	screenY += float64(m.offsetY)

	m.lastX = screenX
	m.lastY = screenY

	C.mouseMove(C.double(screenX), C.double(screenY))
	return nil
}

func clamp(val, min, max int) int {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

func clampFloat(val, min, max float64) float64 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

func (m *MouseController) Click(button string, down bool) error {
	btn := 0
	switch button {
	case "left":
		btn = 0
	case "right":
		btn = 1
	case "middle":
		btn = 2
	default:
		return fmt.Errorf("unknown button: %s", button)
	}

	if down {
		C.mouseDown(C.double(m.lastX), C.double(m.lastY), C.int(btn))
	} else {
		C.mouseUp(C.double(m.lastX), C.double(m.lastY), C.int(btn))
	}

	return nil
}

func (m *MouseController) Scroll(delta int) error {
	// macOS scroll direction: positive = up, negative = down
	C.mouseScroll(C.int(delta))
	return nil
}

func (m *MouseController) HideCursor() {
	// macOS cursor hiding is handled differently (CGDisplayHideCursor)
	// Stub for now — remote cursor overlay is drawn by dashboard
	m.cursorHidden = true
}

func (m *MouseController) ShowCursor() {
	m.cursorHidden = false
}

func (m *MouseController) IsCursorHidden() bool {
	return m.cursorHidden
}
