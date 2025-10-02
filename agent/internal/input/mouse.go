package input

import (
	"fmt"

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
	// Convert normalized coordinates (0-1) to screen coordinates
	screenX := int(x * float64(m.screenWidth))
	screenY := int(y * float64(m.screenHeight))

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
