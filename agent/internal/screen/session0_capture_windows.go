//go:build windows
// +build windows

package screen

import (
	"encoding/binary"
	"fmt"
	"image"
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
	_MAXIMUM_ALLOWED         = 0x02000000
	_WAIT_TIMEOUT            = 0x00000102
	_SE_PRIVILEGE_ENABLED    = 0x00000002
	_TOKEN_ADJUST_PRIVILEGES = 0x0020
	_TOKEN_QUERY             = 0x0008
	_ERROR_NOT_ALL_ASSIGNED  = 1300

	// Pipe command bytes (service → helper)
	cmdCapture    = 0x01
	cmdMouseMove  = 0x02
	cmdMouseClick = 0x03
	cmdScroll     = 0x04
	cmdKeyEvent   = 0x05
	cmdUnicode    = 0x06
	cmdQuit       = 0xFF
)

var (
	modKernel32s = windows.NewLazySystemDLL("kernel32.dll")
	modWtsapi32  = windows.NewLazySystemDLL("wtsapi32.dll")
	modUserenv   = windows.NewLazySystemDLL("userenv.dll")
	modAdvapi32s = windows.NewLazySystemDLL("advapi32.dll")

	procWTSGetActiveConsoleSessionId = modKernel32s.NewProc("WTSGetActiveConsoleSessionId")
	procWTSQueryUserToken            = modWtsapi32.NewProc("WTSQueryUserToken")
	procWTSEnumerateSessionsW        = modWtsapi32.NewProc("WTSEnumerateSessionsW")
	procWTSFreeMemory                = modWtsapi32.NewProc("WTSFreeMemory")
	procCreateEnvironmentBlock       = modUserenv.NewProc("CreateEnvironmentBlock")
	procDestroyEnvironmentBlock      = modUserenv.NewProc("DestroyEnvironmentBlock")
	procLookupPrivilegeValueW        = modAdvapi32s.NewProc("LookupPrivilegeValueW")
	procAdjustTokenPrivileges        = modAdvapi32s.NewProc("AdjustTokenPrivileges")
)

// WTS session state-konstanter (https://learn.microsoft.com/en-us/windows/win32/api/wtsapi32/ne-wtsapi32-wts_connectstate_class)
const (
	wtsActive       = 0 // Active user session (RDP eller console)
	wtsConnected    = 1
	wtsConnectQuery = 2
	wtsShadow       = 3
	wtsDisconnected = 4 // RDP disconnected, session frozen
	wtsIdle         = 5
	wtsListen       = 6
	wtsReset        = 7
	wtsDown         = 8
	wtsInit         = 9
)

type wtsSessionInfo struct {
	SessionID      uint32
	WinStationName *uint16
	State          int32 // WTS_CONNECTSTATE_CLASS
}

func enumerateWTSSessions() ([]wtsSessionInfo, error) {
	var pInfo uintptr
	var count uint32
	const wtsCurrentServer = 0
	ret, _, _ := procWTSEnumerateSessionsW.Call(
		uintptr(wtsCurrentServer),
		0, 1,
		uintptr(unsafe.Pointer(&pInfo)),
		uintptr(unsafe.Pointer(&count)),
	)
	if ret == 0 || pInfo == 0 {
		return nil, fmt.Errorf("WTSEnumerateSessionsW failed")
	}
	defer procWTSFreeMemory.Call(pInfo)

	const entrySize = 24
	sessions := make([]wtsSessionInfo, 0, count)
	for i := uint32(0); i < count; i++ {
		entry := unsafe.Pointer(pInfo + uintptr(i)*entrySize)
		sessions = append(sessions, wtsSessionInfo{
			SessionID:      *(*uint32)(entry),
			WinStationName: *(**uint16)(unsafe.Pointer(uintptr(entry) + 8)),
			State:          *(*int32)(unsafe.Pointer(uintptr(entry) + 16)),
		})
	}
	return sessions, nil
}

func sessionHasUserToken(sessionID uint32) bool {
	if sessionID == 0 || sessionID == 0xFFFFFFFF {
		return false
	}

	var token windows.Token
	ret, _, _ := procWTSQueryUserToken.Call(uintptr(sessionID), uintptr(unsafe.Pointer(&token)))
	if ret == 0 {
		return false
	}
	token.Close()
	return true
}

func getSessionState(sessionID uint32) int32 {
	sessions, err := enumerateWTSSessions()
	if err != nil {
		return wtsInit
	}
	for _, session := range sessions {
		if session.SessionID == sessionID {
			return session.State
		}
	}
	return wtsInit
}

func findBestUserSessionState() (uint32, int32) {
	sessionID := findBestUserSession()
	if sessionID == 0xFFFFFFFF {
		return sessionID, wtsInit
	}
	return sessionID, getSessionState(sessionID)
}

func resolveCaptureSessionTarget() (uint32, int32, uint32, int32) {
	rawSessionID, rawState := findBestUserSessionState()
	if rawSessionID == 0xFFFFFFFF {
		return rawSessionID, rawState, rawSessionID, rawState
	}
	if rawState == wtsDisconnected {
		if consoleSessionID, consoleState, ok := findConsoleLoginSession(); ok {
			return rawSessionID, rawState, consoleSessionID, consoleState
		}
	}
	return rawSessionID, rawState, rawSessionID, rawState
}

func preferredDesktopForSessionState(sessionState int32, hasUserToken bool) string {
	if hasUserToken {
		return "winsta0\\default"
	}
	return "winsta0\\Winlogon"
}

func findConsoleLoginSession() (uint32, int32, bool) {
	consoleSession, _, _ := procWTSGetActiveConsoleSessionId.Call()
	consoleSessionID := uint32(consoleSession)
	if consoleSessionID == 0 || consoleSessionID == 0xFFFFFFFF {
		return 0xFFFFFFFF, wtsInit, false
	}
	state := getSessionState(consoleSessionID)
	if state == wtsConnected && !sessionHasUserToken(consoleSessionID) {
		return consoleSessionID, state, true
	}
	return consoleSessionID, state, false
}

// findBestUserSession returnerer den bedste session at capture'r fra:
//  1. ACTIVE user session med token (console eller RDP)
//  2. CONNECTED user session med token
//  3. DISCONNECTED user session med token (typisk efter RDP-disconnect)
//  4. CONNECTED console/login-session uden bruger-token (Winlogon/pre-login)
//  5. WTSGetActiveConsoleSessionId fallback (legacy)
//
// Det vigtige edge-case er en Windows host efter RDP-disconnect:
// brugersessionen kan stå som DISCONNECTED, mens physical console står som
// CONNECTED på Winlogon. Hvis vi vælger console-sessionen her, får vi ofte
// sort skærm. Vi skal derfor foretrække den rigtige brugersession så længe
// den stadig har et gyldigt user token.
func findBestUserSession() uint32 {
	// Default: WTSGetActiveConsoleSessionId
	consoleSession, _, _ := procWTSGetActiveConsoleSessionId.Call()
	consoleSessionID := uint32(consoleSession)

	// Enumerér alle sessions og find ACTIVE (en bruger er logget ind)
	sessions, err := enumerateWTSSessions()
	if err != nil {
		return consoleSessionID // fallback
	}

	var bestActive uint32 = 0xFFFFFFFF
	var bestConnected uint32 = 0xFFFFFFFF
	var bestDisconnected uint32 = 0xFFFFFFFF
	var bestLoginScreen uint32 = 0xFFFFFFFF
	for _, session := range sessions {
		sessionID := session.SessionID
		state := session.State
		if sessionID == 0 {
			continue
		}

		hasUserToken := sessionHasUserToken(sessionID)
		switch state {
		case wtsActive:
			// Foretrukket: en aktiv brugersession, også når den er via RDP.
			if hasUserToken {
				bestActive = sessionID
				if sessionID == consoleSessionID {
					return sessionID
				}
			}
		case wtsConnected:
			if hasUserToken {
				// Brugersession findes, men er ikke helt ACTIVE endnu.
				if sessionID == consoleSessionID {
					bestConnected = sessionID
				} else if bestConnected == 0xFFFFFFFF {
					bestConnected = sessionID
				}
			} else {
				// Pre-login / Winlogon uden bruger-token.
				if sessionID == consoleSessionID {
					bestLoginScreen = sessionID
				} else if bestLoginScreen == 0xFFFFFFFF {
					bestLoginScreen = sessionID
				}
			}
		case wtsDisconnected:
			// RDP-brugersession efter disconnect. Hvis user token stadig findes,
			// er det som regel den session vi skal overtage i stedet for den
			// tomme Winlogon console-session.
			if hasUserToken {
				if sessionID == consoleSessionID {
					bestDisconnected = sessionID
				} else if bestDisconnected == 0xFFFFFFFF {
					bestDisconnected = sessionID
				}
			}
		}
	}

	if bestActive != 0xFFFFFFFF {
		return bestActive
	}
	if bestConnected != 0xFFFFFFFF {
		return bestConnected
	}
	if bestDisconnected != 0xFFFFFFFF {
		return bestDisconnected
	}
	if bestLoginScreen != 0xFFFFFFFF {
		return bestLoginScreen
	}
	return consoleSessionID
}

func shouldDebounceSessionSwitch(currentSession uintptr, currentState int32, newSession uintptr, newState int32) bool {
	if currentSession == 0 || currentSession == newSession {
		return false
	}
	if newState == wtsDisconnected {
		// A downgrade to a disconnected session is often transient after
		// RDP/input-desktop transitions; require confirmation before relaunch.
		return currentState == wtsActive || currentState == wtsConnected
	}
	if newState == wtsConnected && !sessionHasUserToken(uint32(newSession)) {
		// Connected-without-token is typically Winlogon/login; don't immediately
		// abandon an active/connected user session without seeing it persist.
		return currentState == wtsActive || currentState == wtsConnected
	}
	return false
}

func requiredSessionSwitchConfirmations(currentState int32, newState int32) int {
	if newState == wtsDisconnected && (currentState == wtsActive || currentState == wtsConnected) {
		return 4 // 4 polls * 3s = ~12s stable before helper relaunch
	}
	if newState == wtsConnected && (currentState == wtsActive || currentState == wtsConnected) {
		return 3 // ~9s for potential Winlogon fallback targets
	}
	return 2
}

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

// enablePrivilege enables a named privilege on the current process token.
// Required for SeTcbPrivilege which allows CreateProcessAsUser to work across sessions.
func enablePrivilege(name string) error {
	var token windows.Token
	process, _ := windows.GetCurrentProcess()
	err := windows.OpenProcessToken(process, _TOKEN_ADJUST_PRIVILEGES|_TOKEN_QUERY, &token)
	if err != nil {
		return fmt.Errorf("OpenProcessToken: %w", err)
	}
	defer token.Close()

	nameUTF16, _ := windows.UTF16PtrFromString(name)
	var luid [2]uint32 // LUID is 8 bytes (LowPart + HighPart)
	ret, _, err := procLookupPrivilegeValueW.Call(
		0,
		uintptr(unsafe.Pointer(nameUTF16)),
		uintptr(unsafe.Pointer(&luid[0])),
	)
	if ret == 0 {
		return fmt.Errorf("LookupPrivilegeValue(%s): %v", name, err)
	}

	// TOKEN_PRIVILEGES struct: Count(4) + LUID(8) + Attributes(4) = 16 bytes
	var tp [16]byte
	binary.LittleEndian.PutUint32(tp[0:4], 1)                       // PrivilegeCount
	binary.LittleEndian.PutUint32(tp[4:8], luid[0])                 // LUID.LowPart
	binary.LittleEndian.PutUint32(tp[8:12], luid[1])                // LUID.HighPart
	binary.LittleEndian.PutUint32(tp[12:16], _SE_PRIVILEGE_ENABLED) // Attributes

	var lastErr error
	ret, _, lastErr = procAdjustTokenPrivileges.Call(
		uintptr(token),
		0, // DisableAllPrivileges = FALSE
		uintptr(unsafe.Pointer(&tp[0])),
		0, 0, 0,
	)
	if ret == 0 {
		return fmt.Errorf("AdjustTokenPrivileges: %v", lastErr)
	}

	// Check if privilege was actually assigned (AdjustTokenPrivileges returns TRUE
	// but sets ERROR_NOT_ALL_ASSIGNED if the privilege couldn't be enabled)
	if errno, ok := lastErr.(windows.Errno); ok && errno == _ERROR_NOT_ALL_ASSIGNED {
		return fmt.Errorf("privilege %s not held by process (not running as LocalSystem?)", name)
	}

	log.Printf("✅ Privilege enabled: %s", name)
	return nil
}

// Session0PipeCapturer captures screen from a helper process running in the user's session.
// Services run in Session 0 which has no physical display — GDI/DXGI capture always fails.
// This capturer launches a helper process in the user's session via CreateProcessAsUser,
// which captures the screen via GDI and sends raw BGRA frames back through a named pipe.
type Session0PipeCapturer struct {
	pipeName     string
	pipeHandle   windows.Handle
	pipe         *pipeRW
	helperProc   windows.Handle
	width        int
	height       int
	sessionID    uintptr // Current target session ID
	sessionState int32
	mu           sync.Mutex
	stopCh       chan struct{} // For graceful shutdown of session monitor
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
		stopCh:     make(chan struct{}),
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
	w := int(binary.LittleEndian.Uint32(resoBuf[0:4]))
	h := int(binary.LittleEndian.Uint32(resoBuf[4:8]))
	// Validate resolution bounds (max 8K to prevent OOM from corrupt pipe data)
	if w <= 0 || h <= 0 || w > 7680 || h > 4320 {
		c.Close()
		return nil, fmt.Errorf("invalid resolution from helper: %dx%d", w, h)
	}
	c.width = w
	c.height = h

	log.Printf("✅ Session 0 pipe capturer ready: %dx%d (helper in user session %d)", c.width, c.height, c.sessionID)

	// Start session monitor goroutine (also monitors helper process health)
	go c.monitorSession()

	return c, nil
}

func (c *Session0PipeCapturer) launchHelper() error {
	// Enable required privileges for cross-session process creation
	// SeTcbPrivilege: required for WTSQueryUserToken and SetTokenInformation across sessions
	// SeAssignPrimaryTokenPrivilege: required for CreateProcessAsUser with a different user's token
	// SeIncreaseQuotaPrivilege: required for CreateProcessAsUser to assign process quotas
	for _, priv := range []string{"SeTcbPrivilege", "SeAssignPrimaryTokenPrivilege", "SeIncreaseQuotaPrivilege"} {
		if err := enablePrivilege(priv); err != nil {
			log.Printf("⚠️ enablePrivilege(%s): %v", priv, err)
		}
	}

	// Find the best user session — først ACTIVE (RDP eller console),
	// derefter physical console fallback. Tidligere brugte vi kun
	// WTSGetActiveConsoleSessionId men den fanger kun physical display
	// og kan returnere stale session-IDs efter RDP-disconnects.
	rawSessionIDu, rawSessionState, sessionIDu, sessionState := resolveCaptureSessionTarget()
	if sessionIDu == 0xFFFFFFFF {
		return fmt.Errorf("no active user session (nobody is logged in)")
	}
	if rawSessionState == wtsDisconnected {
		if sessionIDu != rawSessionIDu {
			log.Printf("🔁 Disconnected user session %d detected; preferring console login session %d (state=%d)", rawSessionIDu, sessionIDu, sessionState)
		} else {
			log.Printf("🔒 Disconnected user session %d selected directly", rawSessionIDu)
		}
	}
	sessionID := uintptr(sessionIDu)
	log.Printf("📋 Capture target session: %d (via session enum, state=%d)", sessionID, sessionState)

	// Try to get a token for the console session.
	// Method 1: WTSQueryUserToken (works when a user is logged in)
	// Method 2: Duplicate our SYSTEM token with the session ID changed (works at login screen)
	var dupToken windows.Token
	var tokenMethod string
	helperDesktop := "winsta0\\default"
	captureMode := "follow-input"

	// Always prefer SYSTEM token — it provides SYSTEM integrity which:
	// 1. Bypasses UIPI for ALL windows (admin, elevated, Winlogon/lock screen)
	// 2. Allows SeTcbPrivilege (required for SendInput on secure desktop)
	// 3. Can access Winlogon desktop DACL (which blocks non-SYSTEM processes)
	// The elevated linked token (High integrity) bypasses UIPI for admin windows
	// but NOT for the Winlogon desktop (lock screen), so SYSTEM is always preferred.

	var userToken windows.Token
	ret, _, wtsErr := procWTSQueryUserToken.Call(sessionID, uintptr(unsafe.Pointer(&userToken)))
	if ret != 0 {
		defer userToken.Close()
		log.Printf("📋 User is logged in (session %d, state=%d)", sessionID, sessionState)
		helperDesktop = preferredDesktopForSessionState(sessionState, true)
		if sessionState == wtsDisconnected {
			captureMode = "fixed"
		}

		// Always use SYSTEM token with session ID — highest privilege level
		log.Printf("🛡️ Using SYSTEM token for session %d (UIPI bypass + Winlogon desktop access)", sessionID)
		var processToken windows.Token
		process, _ := windows.GetCurrentProcess()
		if err := windows.OpenProcessToken(process, windows.TOKEN_ALL_ACCESS, &processToken); err == nil {
			defer processToken.Close()
			if err := windows.DuplicateTokenEx(
				processToken,
				_MAXIMUM_ALLOWED,
				nil,
				windows.SecurityImpersonation,
				windows.TokenPrimary,
				&dupToken,
			); err == nil {
				sid := uint32(sessionID)
				if err := windows.SetTokenInformation(
					dupToken,
					windows.TokenSessionId,
					(*byte)(unsafe.Pointer(&sid)),
					uint32(unsafe.Sizeof(sid)),
				); err == nil {
					tokenMethod = "system"
				} else {
					log.Printf("⚠️ SetTokenInformation(SessionId): %v — falling back to user token", err)
					dupToken.Close()
					dupToken = 0
				}
			} else {
				log.Printf("⚠️ DuplicateTokenEx (system): %v — falling back to user token", err)
			}
		}

		// Fallback: regular user token (medium integrity — can't interact with admin/Winlogon)
		if dupToken == 0 {
			tokenMethod = "user"
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
		}
	} else {
		// No user logged in (login screen) — use SYSTEM token with session ID changed
		log.Printf("⚠️ WTSQueryUserToken failed: %v — using SYSTEM token fallback", wtsErr)
		tokenMethod = "system"
		helperDesktop = preferredDesktopForSessionState(sessionState, false)
		captureMode = "fixed"

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
	cmdLine := fmt.Sprintf(`"%s" --capture-helper "%s" "%s"`, exePath, c.pipeName, captureMode)
	cmdLineUTF16, _ := windows.UTF16PtrFromString(cmdLine)

	createFlags := uint32(windows.CREATE_NO_WINDOW)
	if envBlock != 0 {
		createFlags |= windows.CREATE_UNICODE_ENVIRONMENT
	}

	desktops := []string{helperDesktop}
	if helperDesktop != "winsta0\\default" {
		desktops = append(desktops, "winsta0\\default")
	}
	var lastErr error
	for _, desktopName := range desktops {
		log.Printf("🚀 Launching capture helper (token: %s, desktop: %s, mode: %s): %s", tokenMethod, desktopName, captureMode, cmdLine)
		desktopUTF16, _ := windows.UTF16PtrFromString(desktopName)
		si := windows.StartupInfo{
			Cb:      uint32(unsafe.Sizeof(windows.StartupInfo{})),
			Desktop: desktopUTF16,
		}
		var pi windows.ProcessInformation
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
			lastErr = err
			log.Printf("⚠️ CreateProcessAsUser failed on desktop %s: %v", desktopName, err)
			continue
		}

		windows.CloseHandle(pi.Thread)
		c.helperProc = pi.Process
		c.sessionID = sessionID
		c.sessionState = sessionState
		log.Printf("✅ Capture helper launched (PID: %d, session: %d, token: %s, desktop: %s, mode: %s)", pi.ProcessId, sessionID, tokenMethod, desktopName, captureMode)
		return nil
	}
	return fmt.Errorf("CreateProcessAsUser (token: %s): %w", tokenMethod, lastErr)
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
		if w <= 0 || h <= 0 {
			resultCh <- readResult{err: fmt.Errorf("capture helper reported capture error")}
			return
		}
		if w > 7680 || h > 4320 {
			resultCh <- readResult{err: fmt.Errorf("invalid resolution from helper: %dx%d (max 8K)", w, h)}
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

	return EncodeJPEG(img.Pix, img.Bounds().Dx(), img.Bounds().Dy(), img.Stride, quality, false)
}

func (c *Session0PipeCapturer) GetBounds() image.Rectangle {
	return image.Rect(0, 0, c.width, c.height)
}

func (c *Session0PipeCapturer) GetResolution() (int, int) {
	return c.width, c.height
}

// --- Input forwarding methods (service → helper via pipe) ---

// SendMouseMove sends a mouse move command to the helper process.
func (c *Session0PipeCapturer) SendMouseMove(x, y int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pipe == nil {
		return fmt.Errorf("pipe not connected")
	}
	var buf [5]byte
	buf[0] = cmdMouseMove
	binary.LittleEndian.PutUint16(buf[1:3], uint16(x))
	binary.LittleEndian.PutUint16(buf[3:5], uint16(y))
	_, err := c.pipe.Write(buf[:])
	return err
}

// SendMouseClick sends a mouse click command to the helper process.
func (c *Session0PipeCapturer) SendMouseClick(button, down int, x, y int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pipe == nil {
		return fmt.Errorf("pipe not connected")
	}
	var buf [7]byte
	buf[0] = cmdMouseClick
	buf[1] = byte(button)
	buf[2] = byte(down)
	binary.LittleEndian.PutUint16(buf[3:5], uint16(x))
	binary.LittleEndian.PutUint16(buf[5:7], uint16(y))
	_, err := c.pipe.Write(buf[:])
	return err
}

// SendScroll sends a scroll command to the helper process.
func (c *Session0PipeCapturer) SendScroll(delta, x, y int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pipe == nil {
		return fmt.Errorf("pipe not connected")
	}
	var buf [7]byte
	buf[0] = cmdScroll
	binary.LittleEndian.PutUint16(buf[1:3], uint16(int16(delta)))
	binary.LittleEndian.PutUint16(buf[3:5], uint16(x))
	binary.LittleEndian.PutUint16(buf[5:7], uint16(y))
	_, err := c.pipe.Write(buf[:])
	return err
}

// SendKeyEvent sends a key event command to the helper process.
func (c *Session0PipeCapturer) SendKeyEvent(code string, down bool, ctrl, shift, alt, meta bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pipe == nil {
		return fmt.Errorf("pipe not connected")
	}
	keyBytes := []byte(code)
	if len(keyBytes) > 255 {
		keyBytes = keyBytes[:255]
	}
	buf := make([]byte, 7+len(keyBytes))
	buf[0] = cmdKeyEvent
	if down {
		buf[1] = 1
	}
	if ctrl {
		buf[2] = 1
	}
	if shift {
		buf[3] = 1
	}
	if alt {
		buf[4] = 1
	}
	if meta {
		buf[5] = 1
	}
	buf[6] = byte(len(keyBytes))
	copy(buf[7:], keyBytes)
	_, err := c.pipe.Write(buf)
	return err
}

// SendUnicodeChar sends a unicode character to the helper process.
func (c *Session0PipeCapturer) SendUnicodeChar(char rune) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pipe == nil {
		return fmt.Errorf("pipe not connected")
	}
	var buf [3]byte
	buf[0] = cmdUnicode
	binary.LittleEndian.PutUint16(buf[1:3], uint16(char))
	_, err := c.pipe.Write(buf[:])
	return err
}

// --- Session monitor ---

// monitorSession polls the active console session and relaunches the helper if it changes.
// Also monitors the helper process health and auto-restarts on crash.
func (c *Session0PipeCapturer) monitorSession() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	var pendingSession uintptr
	var pendingState int32
	var pendingCount int

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			// Brug samme target-resolution som launchHelper(), ellers kan vi flappe
			// mellem disconnected user session og console login session efter RDP-disconnect.
			_, _, newSessionU, newSessionState := resolveCaptureSessionTarget()
			newSession := uintptr(newSessionU)
			if newSessionU == 0xFFFFFFFF {
				continue // No session available
			}
			c.mu.Lock()
			currentSession := c.sessionID
			currentState := c.sessionState
			c.mu.Unlock()

			if newSession != currentSession || newSessionState != currentState {
				if shouldDebounceSessionSwitch(currentSession, currentState, newSession, newSessionState) {
					minConfirmations := requiredSessionSwitchConfirmations(currentState, newSessionState)
					if pendingSession != newSession || pendingState != newSessionState {
						pendingSession = newSession
						pendingState = newSessionState
						pendingCount = 1
						log.Printf("⏳ Session switch candidate %d/%d → %d/%d; waiting for confirmation (1/%d)...", currentSession, currentState, newSession, newSessionState, minConfirmations)
						continue
					}
					pendingCount++
					if pendingCount < minConfirmations {
						log.Printf("⏳ Session switch candidate still pending (%d/%d): %d/%d → %d/%d", pendingCount, minConfirmations, currentSession, currentState, newSession, newSessionState)
						continue
					}
				}
				pendingSession = 0
				pendingState = 0
				pendingCount = 0
				log.Printf("🔄 Target session changed: %d/%d → %d/%d, relaunching helper...", currentSession, currentState, newSession, newSessionState)
				c.relaunchHelper(newSession, newSessionState)
				continue
			}
			pendingSession = 0
			pendingState = 0
			pendingCount = 0

			// Check if helper process is still alive
			c.mu.Lock()
			helperProc := c.helperProc
			c.mu.Unlock()
			if helperProc != 0 {
				exitEvent, _ := windows.WaitForSingleObject(helperProc, 0)
				if exitEvent == 0 { // WAIT_OBJECT_0 = already exited
					var exitCode uint32
					windows.GetExitCodeProcess(helperProc, &exitCode)
					log.Printf("⚠️ Capture helper crashed (exit code %d), auto-restarting...", exitCode)
					c.relaunchHelper(currentSession, currentState)
				}
			}
		}
	}
}

// relaunchHelper kills the current helper and launches a new one in the given session.
func (c *Session0PipeCapturer) relaunchHelper(newSession uintptr, newSessionState int32) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Close existing pipe and kill helper
	if c.pipe != nil {
		c.pipe.Write([]byte{cmdQuit})
		c.pipe = nil
	}
	if c.pipeHandle != 0 {
		windows.CloseHandle(c.pipeHandle)
		c.pipeHandle = 0
	}
	if c.helperProc != 0 {
		event, _ := windows.WaitForSingleObject(c.helperProc, 1000)
		if event == _WAIT_TIMEOUT {
			windows.TerminateProcess(c.helperProc, 1)
		}
		windows.CloseHandle(c.helperProc)
		c.helperProc = 0
	}

	// Create new pipe
	pipeName := fmt.Sprintf(`\\.\pipe\RemoteDesktopCapture-%d`, rand.Intn(999999))
	sd, err := windows.SecurityDescriptorFromString("D:(A;;GA;;;WD)")
	if err != nil {
		log.Printf("❌ relaunchHelper: SecurityDescriptorFromString: %v", err)
		return
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
		1, 65536, 65536, 30000, sa,
	)
	if err != nil {
		log.Printf("❌ relaunchHelper: CreateNamedPipe: %v", err)
		return
	}

	c.pipeName = pipeName
	c.pipeHandle = pipeHandle
	c.sessionID = newSession
	c.sessionState = newSessionState

	// Launch helper in new session
	if err := c.launchHelper(); err != nil {
		log.Printf("❌ relaunchHelper: launchHelper: %v", err)
		windows.CloseHandle(pipeHandle)
		c.pipeHandle = 0
		return
	}

	// Wait for helper to connect
	time.Sleep(2 * time.Second)
	connectDone := make(chan error, 1)
	go func() {
		connectDone <- windows.ConnectNamedPipe(pipeHandle, nil)
	}()

	select {
	case err := <-connectDone:
		if err != nil && err != windows.ERROR_PIPE_CONNECTED {
			log.Printf("❌ relaunchHelper: ConnectNamedPipe: %v", err)
			return
		}
	case <-time.After(10 * time.Second):
		log.Println("❌ relaunchHelper: timeout waiting for helper")
		windows.CloseHandle(pipeHandle)
		c.pipeHandle = 0
		<-connectDone
		if c.helperProc != 0 {
			windows.TerminateProcess(c.helperProc, 1)
			windows.CloseHandle(c.helperProc)
			c.helperProc = 0
		}
		return
	}

	c.pipe = &pipeRW{handle: pipeHandle}

	// Read new resolution
	resoBuf := make([]byte, 8)
	if _, err := io.ReadFull(c.pipe, resoBuf); err != nil {
		log.Printf("❌ relaunchHelper: failed to read resolution: %v", err)
		return
	}
	w := int(binary.LittleEndian.Uint32(resoBuf[0:4]))
	h := int(binary.LittleEndian.Uint32(resoBuf[4:8]))
	if w <= 0 || h <= 0 || w > 7680 || h > 4320 {
		log.Printf("❌ relaunchHelper: invalid resolution: %dx%d", w, h)
		return
	}
	c.width = w
	c.height = h

	log.Printf("✅ Helper relaunched in session %d (state=%d): %dx%d", c.sessionID, c.sessionState, c.width, c.height)
}

func (c *Session0PipeCapturer) Close() error {
	// Stop session monitor
	select {
	case <-c.stopCh:
		// Already closed
	default:
		close(c.stopCh)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.pipe != nil {
		c.pipe.Write([]byte{cmdQuit})
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
