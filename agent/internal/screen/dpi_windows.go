//go:build windows

package screen

import (
	"log"
	"syscall"
)

// DPI_AWARENESS_CONTEXT-konstanter er HANDLE-værdier (signed -1..-5).
// Vi udtrykker dem som two's-complement uintptr så de kan castes uden
// "overflows uintptr" compile-fejl på 64-bit Go.
const (
	dpiAwarenessContextPerMonitorAwareV2 = ^uintptr(3) // -4
)

// EnableDPIAwareness sætter processen til Per-Monitor V2 DPI-awareness.
//
// Hvorfor: hvis ikke, ser GetSystemMetrics(SM_CXSCREEN) og DXGI/GDI capture
// en virtualiseret (mindre) skærm når brugeren kører display-scaling > 100%
// (typisk på laptops + 4K). Resultat: taskbar / start-menu / højre-side
// bliver klippet væk i den capture vi sender til controlleren.
//
// Manifestet sætter samme awareness statisk, men servicen kan i visse
// loadere ignorere manifest — vi kalder API'et runtime som backup.
// Idempotent: hvis allerede sat, returnerer Windows S_OK eller E_ACCESSDENIED
// som vi ignorerer.
func EnableDPIAwareness() {
	user32 := syscall.NewLazyDLL("user32.dll")

	// Per-Monitor V2 (Windows 10 1703+) — bedste mulighed.
	// SetProcessDpiAwarenessContext(DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = -4)
	if proc := user32.NewProc("SetProcessDpiAwarenessContext"); proc.Find() == nil {
		ret, _, _ := proc.Call(dpiAwarenessContextPerMonitorAwareV2)
		if ret != 0 {
			log.Println("📐 DPI awareness: Per-Monitor V2 (set via user32)")
			return
		}
	}

	// Fallback: shcore.SetProcessDpiAwareness (Windows 8.1+)
	// PROCESS_PER_MONITOR_DPI_AWARE = 2
	shcore := syscall.NewLazyDLL("shcore.dll")
	if proc := shcore.NewProc("SetProcessDpiAwareness"); proc.Find() == nil {
		ret, _, _ := proc.Call(uintptr(2))
		if ret == 0 { // S_OK
			log.Println("📐 DPI awareness: Per-Monitor (set via shcore)")
			return
		}
	}

	// Last resort: user32.SetProcessDPIAware (system-DPI-aware, no per-monitor)
	if proc := user32.NewProc("SetProcessDPIAware"); proc.Find() == nil {
		ret, _, _ := proc.Call()
		if ret != 0 {
			log.Println("📐 DPI awareness: System-aware (set via user32 legacy)")
			return
		}
	}

	log.Println("⚠️  Kunne ikke sætte DPI-awareness — capture kan blive klippet ved scaling > 100%")
}
