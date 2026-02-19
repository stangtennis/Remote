//go:build windows
// +build windows

package screen

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log"
	"math/rand"
	"os"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	// Windows API constants
	_MAXIMUM_ALLOWED = 0x02000000
	_WAIT_TIMEOUT    = 0x00000102
)

var (
	modKernel32s = windows.NewLazySystemDLL("kernel32.dll")
	modWtsapi32  = windows.NewLazySystemDLL("wtsapi32.dll")
	modUserenv   = windows.NewLazySystemDLL("userenv.dll")

	procWTSGetActiveConsoleSessionId = modKernel32s.NewProc("WTSGetActiveConsoleSessionId")
	procWTSQueryUserToken            = modWtsapi32.NewProc("WTSQueryUserToken")
	procCreateEnvironmentBlock       = modUserenv.NewProc("CreateEnvironmentBlock")
	procDestroyEnvironmentBlock      = modUserenv.NewProc("DestroyEnvironmentBlock")
)

// pipeRW wraps a Windows named pipe handle as io.ReadWriter.
type pipeRW struct {
	handle windows.Handle
}

func (p *pipeRW) Read(b []byte) (int, error) {
	var n uint32
	err := windows.ReadFile(p.handle, b, &n, nil)
	if err != nil {
		return int(n), err
	}
	if n == 0 {
		return 0, io.EOF
	}
	return int(n), nil
}

func (p *pipeRW) Write(b []byte) (int, error) {
	total := 0
	for total < len(b) {
		var n uint32
		err := windows.WriteFile(p.handle, b[total:], &n, nil)
		if err != nil {
			return total + int(n), err
		}
		total += int(n)
	}
	return total, nil
}

// Session0PipeCapturer captures screen from a helper process running in the user's session.
// Services run in Session 0 which has no physical display — GDI/DXGI capture always fails.
// This capturer launches a helper process in the user's session via CreateProcessAsUser,
// which captures the screen via GDI and sends raw BGRA frames back through a named pipe.
type Session0PipeCapturer struct {
	pipeName   string
	pipeHandle windows.Handle
	pipe       *pipeRW
	helperProc windows.Handle
	width      int
	height     int
	mu         sync.Mutex
}

// NewSession0PipeCapturer creates a pipe-based capturer for Session 0.
func NewSession0PipeCapturer() (*Session0PipeCapturer, error) {
	pipeName := fmt.Sprintf(`\\.\pipe\RemoteDesktopCapture-%d`, rand.Intn(999999))
	log.Printf("🔧 Creating Session 0 capture pipe: %s", pipeName)

	// Create security descriptor allowing cross-session access (Everyone: Full Control)
	sd, err := windows.SecurityDescriptorFromString("D:(A;;GA;;;WD)")
	if err != nil {
		return nil, fmt.Errorf("SecurityDescriptorFromString: %w", err)
	}
	sa := &windows.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		SecurityDescriptor: sd,
	}

	pipeNameUTF16, _ := windows.UTF16PtrFromString(pipeName)
	pipeHandle, err := windows.CreateNamedPipe(
		pipeNameUTF16,
		windows.PIPE_ACCESS_DUPLEX,
		windows.PIPE_TYPE_BYTE|windows.PIPE_READMODE_BYTE|windows.PIPE_WAIT,
		1,     // max instances
		65536, // out buffer
		65536, // in buffer
		30000, // default timeout ms
		sa,
	)
	if err != nil {
		return nil, fmt.Errorf("CreateNamedPipe: %w", err)
	}

	c := &Session0PipeCapturer{
		pipeName:   pipeName,
		pipeHandle: pipeHandle,
		width:      1920,
		height:     1080,
	}

	// Launch helper process in user's session
	if err := c.launchHelper(); err != nil {
		windows.CloseHandle(pipeHandle)
		return nil, fmt.Errorf("launchHelper: %w", err)
	}

	// Brief wait, then verify helper is still alive
	time.Sleep(2 * time.Second)
	exitEvent, _ := windows.WaitForSingleObject(c.helperProc, 0)
	if exitEvent == 0 { // WAIT_OBJECT_0 = already exited
		var exitCode uint32
		windows.GetExitCodeProcess(c.helperProc, &exitCode)
		windows.CloseHandle(c.helperProc)
		windows.CloseHandle(pipeHandle)
		return nil, fmt.Errorf("capture helper exited immediately (exit code %d)", exitCode)
	}

	// Wait for helper to connect to pipe (with timeout to prevent blocking forever)
	log.Println("⏳ Waiting for capture helper to connect (max 10s)...")
	connectDone := make(chan error, 1)
	go func() {
		connectDone <- windows.ConnectNamedPipe(pipeHandle, nil)
	}()

	select {
	case err = <-connectDone:
		if err != nil && err != windows.ERROR_PIPE_CONNECTED {
			c.Close()
			return nil, fmt.Errorf("ConnectNamedPipe: %w", err)
		}
	case <-time.After(10 * time.Second):
		log.Println("❌ Timeout: capture helper didn't connect within 10s")
		// Close pipe handle to unblock the ConnectNamedPipe goroutine
		windows.CloseHandle(pipeHandle)
		c.pipeHandle = 0
		<-connectDone // Wait for goroutine to finish (returns ERROR_INVALID_HANDLE)
		// Terminate helper process
		if c.helperProc != 0 {
			windows.TerminateProcess(c.helperProc, 1)
			windows.CloseHandle(c.helperProc)
			c.helperProc = 0
		}
		return nil, fmt.Errorf("timeout: capture helper didn't connect within 10s")
	}

	c.pipe = &pipeRW{handle: pipeHandle}
	log.Println("✅ Capture helper connected")

	// Read initial resolution (8 bytes: uint32 width + uint32 height)
	resoBuf := make([]byte, 8)
	if _, err := io.ReadFull(c.pipe, resoBuf); err != nil {
		c.Close()
		return nil, fmt.Errorf("failed to read initial resolution: %w", err)
	}
	c.width = int(binary.LittleEndian.Uint32(resoBuf[0:4]))
	c.height = int(binary.LittleEndian.Uint32(resoBuf[4:8]))

	log.Printf("✅ Session 0 pipe capturer ready: %dx%d (helper in user session)", c.width, c.height)
	return c, nil
}

func (c *Session0PipeCapturer) launchHelper() error {
	// Get active console session (the session with the physical display)
	sessionID, _, _ := procWTSGetActiveConsoleSessionId.Call()
	if sessionID == 0xFFFFFFFF {
		return fmt.Errorf("no active user session (nobody is logged in)")
	}
	log.Printf("📋 Active console session: %d", sessionID)

	// Try to get a token for the console session.
	// Method 1: WTSQueryUserToken (works when a user is logged in)
	// Method 2: Duplicate our SYSTEM token with the session ID changed (works at login screen)
	var dupToken windows.Token
	var tokenMethod string

	var userToken windows.Token
	ret, _, wtsErr := procWTSQueryUserToken.Call(sessionID, uintptr(unsafe.Pointer(&userToken)))
	if ret != 0 {
		// User is logged in — use their token
		tokenMethod = "user"
		defer userToken.Close()
		if err := windows.DuplicateTokenEx(
			userToken,
			_MAXIMUM_ALLOWED,
			nil,
			windows.SecurityImpersonation,
			windows.TokenPrimary,
			&dupToken,
		); err != nil {
			return fmt.Errorf("DuplicateTokenEx (user token): %w", err)
		}
	} else {
		// No user logged in (login screen) — use SYSTEM token with session ID changed
		log.Printf("⚠️ WTSQueryUserToken failed: %v — using SYSTEM token fallback", wtsErr)
		tokenMethod = "system"

		// Get our own process token (we run as SYSTEM)
		var processToken windows.Token
		process, _ := windows.GetCurrentProcess()
		if err := windows.OpenProcessToken(process, windows.TOKEN_ALL_ACCESS, &processToken); err != nil {
			return fmt.Errorf("OpenProcessToken: %w", err)
		}
		defer processToken.Close()

		// Duplicate as primary token
		if err := windows.DuplicateTokenEx(
			processToken,
			_MAXIMUM_ALLOWED,
			nil,
			windows.SecurityImpersonation,
			windows.TokenPrimary,
			&dupToken,
		); err != nil {
			return fmt.Errorf("DuplicateTokenEx (system token): %w", err)
		}

		// Change the session ID on the duplicated token to the console session
		sid := uint32(sessionID)
		if err := windows.SetTokenInformation(
			dupToken,
			windows.TokenSessionId,
			(*byte)(unsafe.Pointer(&sid)),
			uint32(unsafe.Sizeof(sid)),
		); err != nil {
			dupToken.Close()
			return fmt.Errorf("SetTokenInformation(TokenSessionId=%d): %w", sessionID, err)
		}
		log.Printf("✅ SYSTEM token configured for session %d", sessionID)
	}
	defer dupToken.Close()

	// Create environment block
	var envBlock uintptr
	ret, _, _ = procCreateEnvironmentBlock.Call(
		uintptr(unsafe.Pointer(&envBlock)),
		uintptr(dupToken),
		0,
	)
	if ret != 0 && envBlock != 0 {
		defer procDestroyEnvironmentBlock.Call(envBlock)
	}

	// Build command line: launch ourselves with --capture-helper flag
	exePath, err2 := os.Executable()
	if err2 != nil {
		return fmt.Errorf("os.Executable: %w", err2)
	}
	cmdLine := fmt.Sprintf(`"%s" --capture-helper "%s"`, exePath, c.pipeName)
	log.Printf("🚀 Launching capture helper (token: %s): %s", tokenMethod, cmdLine)

	cmdLineUTF16, _ := windows.UTF16PtrFromString(cmdLine)
	desktopUTF16, _ := windows.UTF16PtrFromString("winsta0\\default")

	si := windows.StartupInfo{
		Cb:      uint32(unsafe.Sizeof(windows.StartupInfo{})),
		Desktop: desktopUTF16,
	}
	var pi windows.ProcessInformation

	createFlags := uint32(windows.CREATE_NO_WINDOW)
	if envBlock != 0 {
		createFlags |= windows.CREATE_UNICODE_ENVIRONMENT
	}

	if err := windows.CreateProcessAsUser(
		dupToken,
		nil, cmdLineUTF16,
		nil, nil,
		false,
		createFlags,
		(*uint16)(unsafe.Pointer(envBlock)),
		nil,
		&si, &pi,
	); err != nil {
		return fmt.Errorf("CreateProcessAsUser (token: %s): %w", tokenMethod, err)
	}

	windows.CloseHandle(pi.Thread)
	c.helperProc = pi.Process

	log.Printf("✅ Capture helper launched (PID: %d, session: %d, token: %s)", pi.ProcessId, sessionID, tokenMethod)
	return nil
}

// CaptureRGBA sends a capture request to the helper and returns the frame as RGBA.
func (c *Session0PipeCapturer) CaptureRGBA() (*image.RGBA, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.pipe == nil {
		return nil, fmt.Errorf("capture helper not connected")
	}

	// Check if helper is still alive
	if c.helperProc != 0 {
		exitEvent, _ := windows.WaitForSingleObject(c.helperProc, 0)
		if exitEvent == 0 { // WAIT_OBJECT_0 = already exited
			var exitCode uint32
			windows.GetExitCodeProcess(c.helperProc, &exitCode)
			return nil, fmt.Errorf("capture helper has exited (code %d)", exitCode)
		}
	}

	// Send capture command (0x01)
	if _, err := c.pipe.Write([]byte{0x01}); err != nil {
		return nil, fmt.Errorf("capture helper pipe write error: %w", err)
	}

	// Read response with timeout (goroutine + channel)
	type readResult struct {
		hdr  []byte
		bgra []byte
		w, h int
		err  error
	}
	resultCh := make(chan readResult, 1)
	go func() {
		// Read header: uint32(width) + uint32(height)
		hdr := make([]byte, 8)
		if _, err := io.ReadFull(c.pipe, hdr); err != nil {
			resultCh <- readResult{err: fmt.Errorf("capture helper pipe read error: %w", err)}
			return
		}

		w := int(binary.LittleEndian.Uint32(hdr[0:4]))
		h := int(binary.LittleEndian.Uint32(hdr[4:8]))
		if w == 0 || h == 0 {
			resultCh <- readResult{err: fmt.Errorf("capture helper reported capture error")}
			return
		}

		// Read BGRA pixel data
		dataLen := w * h * 4
		bgra := make([]byte, dataLen)
		if _, err := io.ReadFull(c.pipe, bgra); err != nil {
			resultCh <- readResult{err: fmt.Errorf("capture helper frame read error (%d bytes): %w", dataLen, err)}
			return
		}
		resultCh <- readResult{bgra: bgra, w: w, h: h}
	}()

	// Wait for result with 5s timeout
	select {
	case res := <-resultCh:
		if res.err != nil {
			return nil, res.err
		}

		c.width = res.w
		c.height = res.h

		// Convert BGRA → RGBA
		dataLen := res.w * res.h * 4
		img := image.NewRGBA(image.Rect(0, 0, res.w, res.h))
		for i := 0; i < dataLen; i += 4 {
			img.Pix[i] = res.bgra[i+2]   // R
			img.Pix[i+1] = res.bgra[i+1] // G
			img.Pix[i+2] = res.bgra[i]   // B
			img.Pix[i+3] = 255           // A
		}
		return img, nil

	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("capture helper read timeout (5s)")
	}
}

// CaptureJPEG captures a frame and encodes it as JPEG.
func (c *Session0PipeCapturer) CaptureJPEG(quality int) ([]byte, error) {
	img, err := c.CaptureRGBA()
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, fmt.Errorf("JPEG encode: %w", err)
	}
	return buf.Bytes(), nil
}

func (c *Session0PipeCapturer) GetBounds() image.Rectangle {
	return image.Rect(0, 0, c.width, c.height)
}

func (c *Session0PipeCapturer) GetResolution() (int, int) {
	return c.width, c.height
}

func (c *Session0PipeCapturer) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.pipe != nil {
		c.pipe.Write([]byte{0xFF}) // quit command (best effort)
		c.pipe = nil
	}
	if c.pipeHandle != 0 {
		windows.CloseHandle(c.pipeHandle)
		c.pipeHandle = 0
	}
	if c.helperProc != 0 {
		// Wait up to 3s for helper to exit gracefully, then terminate
		event, _ := windows.WaitForSingleObject(c.helperProc, 3000)
		if event == _WAIT_TIMEOUT {
			log.Println("⚠️ Terminating capture helper (didn't exit in 3s)")
			windows.TerminateProcess(c.helperProc, 1)
		}
		windows.CloseHandle(c.helperProc)
		c.helperProc = 0
	}
	return nil
}
