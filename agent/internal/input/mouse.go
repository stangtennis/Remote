package input

import (
	"fmt"
	"math"

	"github.com/go-vgo/robotgo"
)

type MouseController struct {
	screenWidth  int
	screenHeight int
}

func NewMouseController(width, height int) *MouseController {
	return &MouseController{
		screenWidth:  width,
		screenHeight: height,
	}
}

func (m *MouseController) Move(x, y float64) error {
	// Use proper rounding instead of truncation to reduce drift
	screenX := int(math.Round(x))
	screenY := int(math.Round(y))

	// Clamp to screen bounds to prevent out-of-bounds issues
	if screenX < 0 {
		screenX = 0
	}
	if screenY < 0 {
		screenY = 0
	}
	if m.screenWidth > 0 && screenX >= m.screenWidth {
		screenX = m.screenWidth - 1
	}
	if m.screenHeight > 0 && screenY >= m.screenHeight {
		screenY = m.screenHeight - 1
	}

	robotgo.Move(screenX, screenY)
	return nil
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
