//go:build !windows

package main

import "log"

// InputInjector stub for non-Windows platforms
type InputInjector struct{}

func NewInputInjector() *InputInjector {
	log.Println("⚠️ Input injection only supported on Windows")
	return &InputInjector{}
}

func (i *InputInjector) MouseMoveRelative(dx, dy int) {
	log.Printf("MouseMoveRelative: dx=%d dy=%d (stub)", dx, dy)
}

func (i *InputInjector) MouseMoveAbsolute(x, y float64) {
	log.Printf("MouseMoveAbsolute: x=%.4f y=%.4f (stub)", x, y)
}

func (i *InputInjector) MouseButton(button string, down bool) {
	log.Printf("MouseButton: %s down=%v (stub)", button, down)
}

func (i *InputInjector) MouseWheel(dx, dy int) {
	log.Printf("MouseWheel: dx=%d dy=%d (stub)", dx, dy)
}

func (i *InputInjector) KeyEvent(code string, down bool, ctrl, shift, alt bool) {
	log.Printf("KeyEvent: %s down=%v ctrl=%v shift=%v alt=%v (stub)", code, down, ctrl, shift, alt)
}

func (i *InputInjector) SetClipboard(text string) {
	log.Printf("SetClipboard: %d chars (stub)", len(text))
}

func (i *InputInjector) GetClipboard() string {
	log.Println("GetClipboard (stub)")
	return ""
}
