//go:build windows
// +build windows

package screen

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows API constants for input injection
const (
	_INPUT_MOUSE    = 0
	_INPUT_KEYBOARD = 1

	_MOUSEEVENTF_MOVE       = 0x0001
	_MOUSEEVENTF_LEFTDOWN   = 0x0002
	_MOUSEEVENTF_LEFTUP     = 0x0004
	_MOUSEEVENTF_RIGHTDOWN  = 0x0008
	_MOUSEEVENTF_RIGHTUP    = 0x0010
	_MOUSEEVENTF_MIDDLEDOWN = 0x0020
	_MOUSEEVENTF_MIDDLEUP   = 0x0040
	_MOUSEEVENTF_WHEEL      = 0x0800
	_MOUSEEVENTF_ABSOLUTE   = 0x8000

	_KEYEVENTF_KEYUP   = 0x0002
	_KEYEVENTF_UNICODE = 0x0004

	_WHEEL_DELTA = 120

	_cbSizeInput = 40 // sizeof(INPUT) on 64-bit Windows
)

var (
	helperUser32        = syscall.NewLazyDLL("user32.dll")
	procSetCursorPosH   = helperUser32.NewProc("SetCursorPos")
	procSendInputH      = helperUser32.NewProc("SendInput")
	procOpenInputDesktop = helperUser32.NewProc("OpenInputDesktop")
	procSetThreadDesktop = helperUser32.NewProc("SetThreadDesktop")
	procCloseDesktop     = helperUser32.NewProc("CloseDesktop")
)

// switchToInputDesktop attaches the calling thread to the current input desktop.
// This ensures input events reach the correct desktop (Winlogon at login screen, Default after login).
func switchToInputDesktop() {
	desk, _, _ := procOpenInputDesktop.Call(0, 0, 0x10000000) // GENERIC_ALL
	if desk == 0 {
		return
	}
	procSetThreadDesktop.Call(desk)
	procCloseDesktop.Call(desk)
}

// makeMouseInput builds a 40-byte INPUT struct for mouse events.
func makeMouseInput(dx, dy int32, mouseData uint32, flags uint32) [_cbSizeInput]byte {
	var buf [_cbSizeInput]byte
	binary.LittleEndian.PutUint32(buf[0:4], _INPUT_MOUSE) // type = INPUT_MOUSE
	// MOUSEINPUT starts at offset 8 (after 4 bytes padding)
	binary.LittleEndian.PutUint32(buf[8:12], uint32(dx))        // dx
	binary.LittleEndian.PutUint32(buf[12:16], uint32(dy))       // dy
	binary.LittleEndian.PutUint32(buf[16:20], mouseData)        // mouseData
	binary.LittleEndian.PutUint32(buf[20:24], flags)            // dwFlags
	return buf
}

// makeKeyInput builds a 40-byte INPUT struct for keyboard events.
func makeKeyInput(wVk, wScan uint16, dwFlags uint32) [_cbSizeInput]byte {
	var buf [_cbSizeInput]byte
	binary.LittleEndian.PutUint32(buf[0:4], _INPUT_KEYBOARD)
	binary.LittleEndian.PutUint16(buf[8:10], wVk)
	binary.LittleEndian.PutUint16(buf[10:12], wScan)
	binary.LittleEndian.PutUint32(buf[12:16], dwFlags)
	return buf
}

// helperSendInput wraps the SendInput call for the helper process.
func helperSendInput(inputs []byte, count int) error {
	n, _, err := procSendInputH.Call(
		uintptr(count),
		uintptr(unsafe.Pointer(&inputs[0])),
		uintptr(_cbSizeInput),
	)
	if n == 0 {
		return fmt.Errorf("SendInput failed: %v", err)
	}
	return nil
}

// handleMouseMove handles mouse move commands from the service.
func handleMouseMove(pipe *pipeRW) error {
	var buf [4]byte
	if _, err := io.ReadFull(pipe, buf[:]); err != nil {
		return err
	}
	x := int(binary.LittleEndian.Uint16(buf[0:2]))
	y := int(binary.LittleEndian.Uint16(buf[2:4]))

	switchToInputDesktop()
	procSetCursorPosH.Call(uintptr(x), uintptr(y))
	return nil
}

// handleMouseClick handles mouse click commands from the service.
func handleMouseClick(pipe *pipeRW) error {
	var buf [6]byte
	if _, err := io.ReadFull(pipe, buf[:]); err != nil {
		return err
	}
	button := buf[0]
	down := buf[1]
	x := int(binary.LittleEndian.Uint16(buf[2:4]))
	y := int(binary.LittleEndian.Uint16(buf[4:6]))

	switchToInputDesktop()

	// Move cursor first
	procSetCursorPosH.Call(uintptr(x), uintptr(y))

	// Determine mouse event flags
	var flags uint32
	switch button {
	case 0: // left
		if down != 0 {
			flags = _MOUSEEVENTF_LEFTDOWN
		} else {
			flags = _MOUSEEVENTF_LEFTUP
		}
	case 1: // right
		if down != 0 {
			flags = _MOUSEEVENTF_RIGHTDOWN
		} else {
			flags = _MOUSEEVENTF_RIGHTUP
		}
	case 2: // middle
		if down != 0 {
			flags = _MOUSEEVENTF_MIDDLEDOWN
		} else {
			flags = _MOUSEEVENTF_MIDDLEUP
		}
	default:
		return nil
	}

	inp := makeMouseInput(0, 0, 0, flags)
	return helperSendInput(inp[:], 1)
}

// handleScroll handles scroll commands from the service.
func handleScroll(pipe *pipeRW) error {
	var buf [6]byte
	if _, err := io.ReadFull(pipe, buf[:]); err != nil {
		return err
	}
	delta := int16(binary.LittleEndian.Uint16(buf[0:2]))
	x := int(binary.LittleEndian.Uint16(buf[2:4]))
	y := int(binary.LittleEndian.Uint16(buf[4:6]))

	switchToInputDesktop()

	// Move cursor to position
	procSetCursorPosH.Call(uintptr(x), uintptr(y))

	// Send wheel event
	inp := makeMouseInput(0, 0, uint32(int32(delta)*_WHEEL_DELTA), _MOUSEEVENTF_WHEEL)
	return helperSendInput(inp[:], 1)
}

// handleKeyEvent handles key event commands from the service.
func handleKeyEvent(pipe *pipeRW) error {
	var hdr [6]byte
	if _, err := io.ReadFull(pipe, hdr[:]); err != nil {
		return err
	}
	down := hdr[0] != 0
	ctrl := hdr[1] != 0
	shift := hdr[2] != 0
	alt := hdr[3] != 0
	meta := hdr[4] != 0
	keyLen := int(hdr[5])

	keyBuf := make([]byte, keyLen)
	if keyLen > 0 {
		if _, err := io.ReadFull(pipe, keyBuf); err != nil {
			return err
		}
	}
	code := string(keyBuf)

	switchToInputDesktop()

	// Map JS key code to virtual key code
	vk := mapCodeToVK(code)
	if vk == 0 {
		return nil
	}

	// Build input array with modifiers
	var inputs []byte

	if down {
		// Press modifiers first
		if ctrl {
			inp := makeKeyInput(0x11, 0, 0) // VK_CONTROL
			inputs = append(inputs, inp[:]...)
		}
		if shift {
			inp := makeKeyInput(0x10, 0, 0) // VK_SHIFT
			inputs = append(inputs, inp[:]...)
		}
		if alt {
			inp := makeKeyInput(0x12, 0, 0) // VK_MENU
			inputs = append(inputs, inp[:]...)
		}
		if meta {
			inp := makeKeyInput(0x5B, 0, 0) // VK_LWIN
			inputs = append(inputs, inp[:]...)
		}

		// Press key
		inp := makeKeyInput(vk, 0, 0)
		inputs = append(inputs, inp[:]...)

		// Release key
		inp = makeKeyInput(vk, 0, _KEYEVENTF_KEYUP)
		inputs = append(inputs, inp[:]...)

		// Release modifiers in reverse
		if meta {
			inp := makeKeyInput(0x5B, 0, _KEYEVENTF_KEYUP)
			inputs = append(inputs, inp[:]...)
		}
		if alt {
			inp := makeKeyInput(0x12, 0, _KEYEVENTF_KEYUP)
			inputs = append(inputs, inp[:]...)
		}
		if shift {
			inp := makeKeyInput(0x10, 0, _KEYEVENTF_KEYUP)
			inputs = append(inputs, inp[:]...)
		}
		if ctrl {
			inp := makeKeyInput(0x11, 0, _KEYEVENTF_KEYUP)
			inputs = append(inputs, inp[:]...)
		}
	} else {
		// Key up only (for held modifiers)
		inp := makeKeyInput(vk, 0, _KEYEVENTF_KEYUP)
		inputs = append(inputs, inp[:]...)
	}

	count := len(inputs) / _cbSizeInput
	if count > 0 {
		return helperSendInput(inputs, count)
	}
	return nil
}

// handleUnicode handles unicode character commands from the service.
func handleUnicode(pipe *pipeRW) error {
	var buf [2]byte
	if _, err := io.ReadFull(pipe, buf[:]); err != nil {
		return err
	}
	char := binary.LittleEndian.Uint16(buf[0:2])

	switchToInputDesktop()

	down := makeKeyInput(0, char, _KEYEVENTF_UNICODE)
	up := makeKeyInput(0, char, _KEYEVENTF_UNICODE|_KEYEVENTF_KEYUP)

	var inputs [2 * _cbSizeInput]byte
	copy(inputs[0:_cbSizeInput], down[:])
	copy(inputs[_cbSizeInput:2*_cbSizeInput], up[:])

	return helperSendInput(inputs[:], 2)
}

// mapCodeToVK maps JavaScript KeyboardEvent.code to Windows Virtual Key codes.
func mapCodeToVK(code string) uint16 {
	vkMap := map[string]uint16{
		// Letters
		"KeyA": 0x41, "KeyB": 0x42, "KeyC": 0x43, "KeyD": 0x44, "KeyE": 0x45,
		"KeyF": 0x46, "KeyG": 0x47, "KeyH": 0x48, "KeyI": 0x49, "KeyJ": 0x4A,
		"KeyK": 0x4B, "KeyL": 0x4C, "KeyM": 0x4D, "KeyN": 0x4E, "KeyO": 0x4F,
		"KeyP": 0x50, "KeyQ": 0x51, "KeyR": 0x52, "KeyS": 0x53, "KeyT": 0x54,
		"KeyU": 0x55, "KeyV": 0x56, "KeyW": 0x57, "KeyX": 0x58, "KeyY": 0x59,
		"KeyZ": 0x5A,

		// Numbers
		"Digit0": 0x30, "Digit1": 0x31, "Digit2": 0x32, "Digit3": 0x33, "Digit4": 0x34,
		"Digit5": 0x35, "Digit6": 0x36, "Digit7": 0x37, "Digit8": 0x38, "Digit9": 0x39,

		// Function keys
		"F1": 0x70, "F2": 0x71, "F3": 0x72, "F4": 0x73, "F5": 0x74, "F6": 0x75,
		"F7": 0x76, "F8": 0x77, "F9": 0x78, "F10": 0x79, "F11": 0x7A, "F12": 0x7B,

		// Special keys
		"Enter": 0x0D, "Space": 0x20, "Backspace": 0x08, "Tab": 0x09,
		"Escape": 0x1B, "Delete": 0x2E, "Insert": 0x2D,
		"Home": 0x24, "End": 0x23, "PageUp": 0x21, "PageDown": 0x22,

		// Arrow keys
		"ArrowUp": 0x26, "ArrowDown": 0x28, "ArrowLeft": 0x25, "ArrowRight": 0x27,

		// Modifiers
		"ShiftLeft": 0x10, "ShiftRight": 0x10,
		"ControlLeft": 0x11, "ControlRight": 0x11,
		"AltLeft": 0x12, "AltRight": 0x12,
		"MetaLeft": 0x5B, "MetaRight": 0x5C,

		// Punctuation
		"Comma": 0xBC, "Period": 0xBE, "Slash": 0xBF,
		"Semicolon": 0xBA, "Quote": 0xDE,
		"BracketLeft": 0xDB, "BracketRight": 0xDD,
		"Backslash": 0xDC, "Minus": 0xBD, "Equal": 0xBB,
		"Backquote": 0xC0,
	}

	if vk, ok := vkMap[code]; ok {
		return vk
	}
	return 0
}

// RunCaptureHelper runs as a capture helper process in the user's session.
// It connects to the named pipe created by the service, captures the screen
// via GDI (which works in the user session), and sends frames on demand.
// It also handles input events forwarded from the service.
func RunCaptureHelper(pipeName string) error {
	// Set up logging to temp file
	logDir := filepath.Join(os.TempDir(), "RemoteDesktopAgent")
	os.MkdirAll(logDir, 0755)
	logFile, err := os.OpenFile(
		filepath.Join(logDir, "capture-helper.log"),
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		0644,
	)
	if err == nil {
		log.SetOutput(logFile)
		defer logFile.Close()
	}

	log.Printf("Capture helper starting, pipe: %s", pipeName)
	log.Printf("   PID: %d, Session: user session", os.Getpid())

	// Connect to the named pipe (retry for up to 30 seconds)
	pipeNameUTF16, _ := windows.UTF16PtrFromString(pipeName)
	var pipeHandle windows.Handle
	for i := 0; i < 30; i++ {
		pipeHandle, err = windows.CreateFile(
			pipeNameUTF16,
			windows.GENERIC_READ|windows.GENERIC_WRITE,
			0, nil,
			windows.OPEN_EXISTING,
			0, 0,
		)
		if err == nil {
			break
		}
		log.Printf("Waiting for pipe... attempt %d (%v)", i+1, err)
		time.Sleep(time.Second)
	}
	if err != nil {
		return fmt.Errorf("failed to connect to pipe after 30 attempts: %w", err)
	}
	defer windows.CloseHandle(pipeHandle)

	pipe := &pipeRW{handle: pipeHandle}
	log.Println("Connected to capture pipe")

	// Initialize GDI capturer (we're in the user's session, so GDI works!)
	gdi, err := NewGDICapturer()
	if err != nil {
		return fmt.Errorf("GDI init failed: %w", err)
	}
	defer gdi.Close()
	log.Printf("GDI capturer ready: %dx%d", gdi.width, gdi.height)

	// Send initial resolution (8 bytes: uint32 width + uint32 height)
	resoBuf := make([]byte, 8)
	binary.LittleEndian.PutUint32(resoBuf[0:4], uint32(gdi.width))
	binary.LittleEndian.PutUint32(resoBuf[4:8], uint32(gdi.height))
	if _, err := pipe.Write(resoBuf); err != nil {
		return fmt.Errorf("failed to send initial resolution: %w", err)
	}
	log.Println("Sent initial resolution")

	// Main loop: read commands, capture and send frames, handle input
	cmdBuf := make([]byte, 1)
	frameCount := 0
	errorCount := 0
	startTime := time.Now()

	for {
		// Wait for command from service
		if _, err := io.ReadFull(pipe, cmdBuf); err != nil {
			log.Printf("Pipe read error: %v", err)
			return err
		}

		switch cmdBuf[0] {
		case cmdCapture: // Capture BGRA frame
			bgra, w, h, err := gdi.CaptureBGRA()
			if err != nil {
				errorCount++
				if errorCount%100 == 1 {
					log.Printf("Capture error #%d: %v", errorCount, err)
				}
				// Send error response (0x0 dimensions = error)
				errBuf := make([]byte, 8)
				pipe.Write(errBuf)
				continue
			}

			// Send frame: uint32(width) + uint32(height) + BGRA data
			hdr := make([]byte, 8)
			binary.LittleEndian.PutUint32(hdr[0:4], uint32(w))
			binary.LittleEndian.PutUint32(hdr[4:8], uint32(h))
			if _, err := pipe.Write(hdr); err != nil {
				return fmt.Errorf("pipe write header: %w", err)
			}
			if _, err := pipe.Write(bgra); err != nil {
				return fmt.Errorf("pipe write frame: %w", err)
			}

			frameCount++
			if frameCount%300 == 0 {
				elapsed := time.Since(startTime).Seconds()
				fps := float64(frameCount) / elapsed
				log.Printf("Frames: %d (%.1f fps, %dx%d, errors: %d)", frameCount, fps, w, h, errorCount)
			}

		case cmdMouseMove:
			if err := handleMouseMove(pipe); err != nil {
				log.Printf("Mouse move error: %v", err)
			}

		case cmdMouseClick:
			if err := handleMouseClick(pipe); err != nil {
				log.Printf("Mouse click error: %v", err)
			}

		case cmdScroll:
			if err := handleScroll(pipe); err != nil {
				log.Printf("Scroll error: %v", err)
			}

		case cmdKeyEvent:
			if err := handleKeyEvent(pipe); err != nil {
				log.Printf("Key event error: %v", err)
			}

		case cmdUnicode:
			if err := handleUnicode(pipe); err != nil {
				log.Printf("Unicode error: %v", err)
			}

		case cmdQuit:
			log.Printf("Quit command received (sent %d frames)", frameCount)
			return nil
		}
	}
}
