package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// Status dot colors matching the CyberTheme palette.
var (
	ColorOnline  = color.NRGBA{R: 0x00, G: 0xe6, B: 0x76, A: 0xff} // green
	ColorAway    = color.NRGBA{R: 0xff, G: 0xb8, B: 0x00, A: 0xff} // amber
	ColorOffline = color.NRGBA{R: 0xff, G: 0x45, B: 0x57, A: 0xff} // red
	ColorInfo    = color.NRGBA{R: 0x00, G: 0xd4, B: 0xff, A: 0xff} // cyan
)

// NewStatusDot creates a small colored circle indicator.
func NewStatusDot(c color.Color, size float32) *canvas.Circle {
	dot := canvas.NewCircle(c)
	dot.StrokeWidth = 0
	dot.Resize(fyne.NewSize(size, size))
	return dot
}

// StatusBadge returns an HBox with a colored dot and a label.
func StatusBadge(c color.Color, text string) *fyne.Container {
	dot := NewStatusDot(c, 10)
	label := widget.NewLabel(text)
	return container.NewHBox(dot, label)
}
