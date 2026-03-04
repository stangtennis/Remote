package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// CyberTheme is a hi-tech dark theme with electric cyan accents.
type CyberTheme struct{}

var _ fyne.Theme = (*CyberTheme)(nil)

func (t *CyberTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.NRGBA{R: 0x0d, G: 0x11, B: 0x17, A: 0xff} // #0d1117
	case theme.ColorNameButton:
		return color.NRGBA{R: 0x1a, G: 0x1f, B: 0x2e, A: 0xff} // #1a1f2e
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 0x00, G: 0xd4, B: 0xff, A: 0xff} // #00d4ff
	case theme.ColorNameForeground:
		return color.NRGBA{R: 0xe0, G: 0xe8, B: 0xf0, A: 0xff} // #e0e8f0
	case theme.ColorNameInputBackground:
		return color.NRGBA{R: 0x12, G: 0x17, B: 0x20, A: 0xff} // #121720
	case theme.ColorNameInputBorder:
		return color.NRGBA{R: 0x2a, G: 0x3a, B: 0x4a, A: 0xff} // #2a3a4a
	case theme.ColorNameSeparator:
		return color.NRGBA{R: 0x1e, G: 0x2d, B: 0x3d, A: 0xff} // #1e2d3d
	case theme.ColorNameSuccess:
		return color.NRGBA{R: 0x00, G: 0xe6, B: 0x76, A: 0xff} // #00e676
	case theme.ColorNameError:
		return color.NRGBA{R: 0xff, G: 0x45, B: 0x57, A: 0xff} // #ff4557
	case theme.ColorNameWarning:
		return color.NRGBA{R: 0xff, G: 0xb8, B: 0x00, A: 0xff} // #ffb800
	case theme.ColorNameScrollBar:
		return color.NRGBA{R: 0x00, G: 0xd4, B: 0xff, A: 0x66} // #00d4ff66
	case theme.ColorNameDisabled:
		return color.NRGBA{R: 0x50, G: 0x58, B: 0x68, A: 0xff}
	case theme.ColorNameDisabledButton:
		return color.NRGBA{R: 0x14, G: 0x18, B: 0x22, A: 0xff}
	case theme.ColorNamePlaceHolder:
		return color.NRGBA{R: 0x60, G: 0x70, B: 0x80, A: 0xff}
	case theme.ColorNameHover:
		return color.NRGBA{R: 0x00, G: 0xd4, B: 0xff, A: 0x22}
	case theme.ColorNameFocus:
		return color.NRGBA{R: 0x00, G: 0xd4, B: 0xff, A: 0x44}
	case theme.ColorNameSelection:
		return color.NRGBA{R: 0x00, G: 0xd4, B: 0xff, A: 0x33}
	case theme.ColorNameHeaderBackground:
		return color.NRGBA{R: 0x10, G: 0x16, B: 0x1e, A: 0xff}
	case theme.ColorNameMenuBackground:
		return color.NRGBA{R: 0x10, G: 0x16, B: 0x1e, A: 0xff}
	case theme.ColorNameOverlayBackground:
		return color.NRGBA{R: 0x0d, G: 0x11, B: 0x17, A: 0xee}
	case theme.ColorNameShadow:
		return color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x66}
	}
	// Fallback to default dark theme
	return theme.DarkTheme().Color(name, variant)
}

func (t *CyberTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DarkTheme().Font(style)
}

func (t *CyberTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DarkTheme().Icon(name)
}

func (t *CyberTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 6
	case theme.SizeNameInputRadius:
		return 8
	case theme.SizeNameSelectionRadius:
		return 6
	}
	return theme.DarkTheme().Size(name)
}
