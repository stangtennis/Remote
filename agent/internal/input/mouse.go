package input

import (
	"fmt"
	"log"
	"math"
	"syscall"

	"github.com/go-vgo/robotgo"
)

var (
	user32                    = syscall.NewLazyDLL("user32.dll")
	procShowCursor            = user32.NewProc("ShowCursor")
	procSetCursorPos          = user32.NewProc("SetCursorPos")
	procSystemParametersInfoW = user32.NewProc("SystemParametersInfoW")
)

const (
	SPI_SETCURSORS = 0x0057
)

type MouseController struct {
	screenWidth  int
	screenHeight int
	cursorHidden bool
}

func NewMouseController(width, height int) *MouseController {
	return &MouseController{
		screenWidth:  width,
		screenHeight: height,
	}
}

func (m *MouseController) Move(x, y float64) error {
	// Absolute coordinates
	screenX := int(math.Round(x))
	screenY := int(math.Round(y))

	// Clamp to screen bounds
	screenX = clamp(screenX, 0, m.screenWidth-1)
	screenY = clamp(screenY, 0, m.screenHeight-1)

	// Use Windows API directly to avoid robotgo DPI scaling issues
	m.setCursorPos(screenX, screenY)
	return nil
}

// MoveRelative moves mouse using relative coordinates (0.0-1.0)
func (m *MouseController) MoveRelative(x, y float64) error {
	// Convert relative (0-1) to absolute screen coordinates
	screenX := int(math.Round(x * float64(m.screenWidth)))
	screenY := int(math.Round(y * float64(m.screenHeight)))

	// Clamp to screen bounds
	screenX = clamp(screenX, 0, m.screenWidth-1)
	screenY = clamp(screenY, 0, m.screenHeight-1)

	// Use Windows API directly to avoid robotgo DPI scaling issues
	m.setCursorPos(screenX, screenY)
	return nil
}

// setCursorPos uses Windows API directly to set cursor position
// This avoids DPI scaling issues that robotgo has
func (m *MouseController) setCursorPos(x, y int) {
	procSetCursorPos.Call(uintptr(x), uintptr(y))
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

func (m *MouseController) Click(button string, down bool) error {
	if down {
		switch button {
		case "left":
			robotgo.MouseDown("left")
		case "right":
			robotgo.MouseDown("right")
		case "middle":
			robotgo.MouseDown("center")
		default:
			return fmt.Errorf("unknown button: %s", button)
		}
	} else {
		switch button {
		case "left":
			robotgo.MouseUp("left")
		case "right":
			robotgo.MouseUp("right")
		case "middle":
			robotgo.MouseUp("center")
		default:
			return fmt.Errorf("unknown button: %s", button)
		}
	}

	return nil
}

func (m *MouseController) Scroll(delta int) error {
	if delta > 0 {
		robotgo.ScrollDir(1, "up")
	} else if delta < 0 {
		robotgo.ScrollDir(1, "down")
	}
	return nil
}

// HideCursor hides the local mouse cursor during remote session
func (m *MouseController) HideCursor() {
	if m.cursorHidden {
		return
	}
	// ShowCursor decrements counter, hide when < 0
	for i := 0; i < 10; i++ {
		ret, _, _ := procShowCursor.Call(0) // FALSE = hide
		if int32(ret) < 0 {
			break
		}
	}
	m.cursorHidden = true
	log.Println("🖱️ Local cursor hidden")
}

// ShowCursor restores the local mouse cursor
func (m *MouseController) ShowCursor() {
	if !m.cursorHidden {
		return
	}
	// ShowCursor increments counter, show when >= 0
	for i := 0; i < 10; i++ {
		ret, _, _ := procShowCursor.Call(1) // TRUE = show
		if int32(ret) >= 0 {
			break
		}
	}
	m.cursorHidden = false
	log.Println("🖱️ Local cursor restored")
}

// IsCursorHidden returns whether cursor is currently hidden
func (m *MouseController) IsCursorHidden() bool {
	return m.cursorHidden
}
