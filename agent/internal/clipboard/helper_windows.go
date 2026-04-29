//go:build windows

package clipboard

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func b64Encode(data []byte) string         { return base64.StdEncoding.EncodeToString(data) }
func b64Decode(s string) ([]byte, error)   { return base64.StdEncoding.DecodeString(s) }

// Windows clipboards are per-session. The agent runs as SYSTEM in Session 0;
// the user is in Session 1+. A clipboard.Watch from Session 0 NEVER sees the
// user's copies. Fix: spawn a tiny helper process in the active user session
// via CreateProcessAsUser, run clipboard.Watch there, forward events back to
// the SYSTEM service over a named pipe.
//
// Wire format is JSON-per-line — simple enough that the helper can be
// re-used from any tool that wants to bridge the per-session clipboard.

// SessionHelper runs the service-side of the bridge: it spawns the helper
// in the user session and demuxes its messages onto callbacks.
type SessionHelper struct {
	mu            sync.Mutex
	pipeListener  windows.Handle
	pipeWriter    *pipeWriter
	helperProc    windows.Handle
	cancelCtx     context.CancelFunc
	onTextChange  func(text string)
	onImageChange func(imageData []byte)
	pipeName      string
	closed        bool
}

// pipeWriter is a thread-safe wrapper for pipe writes from the service side
// (used to send setText/setImage commands to the helper).
type pipeWriter struct {
	mu     sync.Mutex
	handle windows.Handle
	closed bool
}

func (p *pipeWriter) Write(data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return fmt.Errorf("pipe closed")
	}
	// Frame: 4-byte length + payload, written as a single WriteFile so the
	// counterpart reader sees one contiguous block. Some versions of
	// Windows hang on partial writes when both ends are in PIPE_WAIT mode
	// with small buffers, so we coalesce here.
	buf := make([]byte, 4+len(data))
	binary.LittleEndian.PutUint32(buf[:4], uint32(len(data)))
	copy(buf[4:], data)
	var n uint32
	if err := windows.WriteFile(p.handle, buf, &n, nil); err != nil {
		return err
	}
	return nil
}

func (p *pipeWriter) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.closed {
		windows.CloseHandle(p.handle)
		p.closed = true
	}
}

// NewSessionHelper creates an instance that will spawn a helper on Start.
func NewSessionHelper() *SessionHelper {
	return &SessionHelper{
		pipeName: fmt.Sprintf(`\\.\pipe\RemoteDesktopClipboard-%d`, time.Now().UnixNano()),
	}
}

func (h *SessionHelper) SetOnTextChange(cb func(text string))         { h.onTextChange = cb }
func (h *SessionHelper) SetOnImageChange(cb func(imageData []byte))   { h.onImageChange = cb }

// Start brings up the named pipe server, launches the helper as the active
// console user, and begins consuming messages.
func (h *SessionHelper) Start() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return fmt.Errorf("helper already closed")
	}

	// Create the named pipe in byte-stream duplex mode with a permissive
	// DACL. Default DACL on a SYSTEM-created pipe denies non-elevated user
	// access, which would break the helper running in the interactive
	// session. SDDL "D:(A;;GA;;;WD)" = generic-all to Everyone, matching
	// the capture pipe.
	sd, err := windows.SecurityDescriptorFromString("D:(A;;GA;;;WD)")
	if err != nil {
		return fmt.Errorf("SecurityDescriptorFromString: %w", err)
	}
	sa := &windows.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		SecurityDescriptor: sd,
	}
	pipeNameUTF16, _ := windows.UTF16PtrFromString(h.pipeName)
	// One-way pipe: service is the inbound reader, helper is the outbound
	// writer. Bidirectional duplex pipes serialize concurrent ReadFile +
	// WriteFile from different goroutines on the same handle, which dead-
	// locked the helper. For "set clipboard from controller" we go
	// through the shell.Run user-session bridge instead.
	pipe, err := windows.CreateNamedPipe(pipeNameUTF16,
		windows.PIPE_ACCESS_INBOUND,
		windows.PIPE_TYPE_BYTE|windows.PIPE_WAIT,
		1, 65536, 65536, 0, sa)
	if err != nil {
		return fmt.Errorf("CreateNamedPipe: %w", err)
	}
	h.pipeListener = pipe

	if err := h.spawnHelper(); err != nil {
		windows.CloseHandle(pipe)
		return err
	}

	// Block until the helper connects (it does so within seconds).
	if err := windows.ConnectNamedPipe(pipe, nil); err != nil && err != windows.ERROR_PIPE_CONNECTED {
		windows.CloseHandle(pipe)
		return fmt.Errorf("ConnectNamedPipe: %w", err)
	}
	log.Println("📋 Clipboard helper connected to pipe")

	h.pipeWriter = &pipeWriter{handle: pipe}

	ctx, cancel := context.WithCancel(context.Background())
	h.cancelCtx = cancel
	go h.readLoop(ctx, pipe)
	return nil
}

// readLoop consumes length-prefixed JSON messages from the helper.
func (h *SessionHelper) readLoop(ctx context.Context, pipe windows.Handle) {
	log.Println("📋 Service-side readLoop started")
	// Read in chunks; assemble length-prefixed messages from the byte stream.
	var carry []byte
	chunk := make([]byte, 65536)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		var n uint32
		if err := windows.ReadFile(pipe, chunk, &n, nil); err != nil {
			if err != windows.ERROR_BROKEN_PIPE {
				log.Printf("⚠️  Clipboard helper pipe read failed: %v", err)
			}
			return
		}
		if n == 0 {
			continue
		}
		carry = append(carry, chunk[:n]...)
		// Drain as many complete messages as we have.
		for len(carry) >= 4 {
			size := binary.LittleEndian.Uint32(carry[:4])
			if size == 0 || size > 50*1024*1024 {
				log.Printf("⚠️  Clipboard helper sent invalid size: %d", size)
				return
			}
			if uint32(len(carry)) < 4+size {
				break // incomplete, wait for more
			}
			body := make([]byte, size)
			copy(body, carry[4:4+size])
			carry = carry[4+size:]

			var msg struct {
				Type    string `json:"type"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal(body, &msg); err != nil {
				log.Printf("⚠️  Clipboard helper sent bad JSON: %v", err)
				continue
			}
			log.Printf("📋 readLoop got %s message (%d bytes)", msg.Type, len(msg.Content))
			switch msg.Type {
			case "text":
				if h.onTextChange != nil {
					h.onTextChange(msg.Content)
				}
			case "image":
				data, err := b64Decode(msg.Content)
				if err == nil && h.onImageChange != nil {
					h.onImageChange(data)
				}
			}
		}
	}
}

// SetText / SetImage are no-ops on this code path — the pipe is one-way
// (helper → service). The service uses shell.Run with --as-user to write
// the clipboard from the controller, bypassing the helper entirely.
func (h *SessionHelper) SetText(text string) error  { _ = text; return nil }
func (h *SessionHelper) SetImage(pngData []byte) error { _ = pngData; return nil }
func (h *SessionHelper) RememberText(_ string)         {}
func (h *SessionHelper) RememberImage(_ []byte)        {}

func (h *SessionHelper) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	if h.cancelCtx != nil {
		h.cancelCtx()
	}
	if h.pipeListener != 0 {
		windows.CloseHandle(h.pipeListener)
	}
	if h.helperProc != 0 {
		// Best-effort kill if helper didn't quit
		windows.TerminateProcess(h.helperProc, 0)
		windows.CloseHandle(h.helperProc)
	}
	log.Println("📋 Clipboard helper stopped")
}

// ─── helper process ───────────────────────────────────────────────────

// RunHelper is the entry point when the agent binary is invoked with
// --clipboard-helper <pipe>. Mirrors session0 capture-helper conventions.
func RunHelper(pipeName string) error {
	logDir := os.TempDir()
	// O_SYNC so log entries hit disk immediately — debugging helper
	// behaviour requires not waiting for OS buffer flushes.
	logFile, _ := os.OpenFile(logDir+`\rd-clipboard-helper.log`,
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC|os.O_SYNC, 0644)
	if logFile != nil {
		log.SetOutput(logFile)
		log.SetFlags(log.LstdFlags | log.Lmicroseconds)
		defer logFile.Close()
	}
	// Recover panics so we capture the cause in the log instead of a silent exit
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC: %v", r)
		}
		if logFile != nil {
			logFile.Sync()
		}
	}()
	log.Printf("Clipboard helper starting, pipe: %s, pid: %d", pipeName, os.Getpid())

	pipeNameUTF16, _ := windows.UTF16PtrFromString(pipeName)
	var pipeHandle windows.Handle
	var err error
	for i := 0; i < 30; i++ {
		// Write-only matches the service's PIPE_ACCESS_INBOUND.
		pipeHandle, err = windows.CreateFile(pipeNameUTF16,
			windows.GENERIC_WRITE,
			0, nil, windows.OPEN_EXISTING, 0, 0)
		if err == nil {
			log.Printf("Pipe opened on attempt %d", i+1)
			break
		}
		log.Printf("Pipe open attempt %d: %v", i+1, err)
		time.Sleep(time.Second)
	}
	if err != nil {
		return fmt.Errorf("connect pipe: %w", err)
	}
	defer windows.CloseHandle(pipeHandle)
	log.Println("Connected to pipe")

	w := &pipeWriter{handle: pipeHandle}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Polling-based change detection using GetClipboardSequenceNumber
	// (no OpenClipboard required) plus raw OpenClipboard with bounded
	// retries for the actual read. Bypasses golang.design/x/clipboard
	// whose internal OpenClipboard retry loop hangs in this context.
	var lastTextHash string
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		log.Println("clipboard poller goroutine alive (raw Win32, OS thread locked)")
		var lastSeq uint32
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		// Send the initial state once so the dashboard sees what's there
		// at connect time.
		log.Printf("Initial sequence number: %d", rawSequence())
		log.Println("Calling rawReadText...")
		t, ok := rawReadText()
		log.Printf("Initial rawReadText: ok=%v len=%d", ok, len(t))
		if ok && t != "" {
			lastTextHash = hashString(t)
			body, _ := json.Marshal(map[string]interface{}{"type": "text", "content": t})
			log.Printf("Calling pipe Write (initial)...")
			err := w.Write(body)
			log.Printf("Pipe Write returned: %v", err)
			if err == nil {
				log.Printf("📋 sent initial text (%d bytes)", len(t))
			}
		}
		log.Println("Entering poll loop")
		tickCount := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			tickCount++
			seq := rawSequence()
			if tickCount%10 == 0 {
				log.Printf("[poll #%d] seq=%d lastSeq=%d", tickCount, seq, lastSeq)
			}
			if seq == lastSeq {
				continue
			}
			log.Printf("📋 seq changed: %d → %d", lastSeq, seq)
			lastSeq = seq

			text, ok := rawReadText()
			if !ok || text == "" {
				log.Printf("rawReadText after seq change: ok=%v len=%d", ok, len(text))
				continue
			}
			h := hashString(text)
			if h == lastTextHash {
				continue
			}
			lastTextHash = h

			body, _ := json.Marshal(map[string]interface{}{
				"type":    "text",
				"content": text,
			})
			if err := w.Write(body); err != nil {
				log.Printf("write text event failed: %v", err)
				cancel()
				return
			}
			log.Printf("📋 sent text event (%d bytes, seq=%d)", len(text), seq)
		}
	}()

	// Pipe is write-only from helper's side. Service writes to clipboard
	// via shell.Run (separate code path), not via this pipe. Block on
	// context cancel; the writer goroutines call cancel() if pipe breaks.
	<-ctx.Done()
	log.Println("Helper exiting (context cancelled)")
	return nil
}

// ─── helper process spawn (CreateProcessAsUser) ───────────────────────

var (
	procWTSGetActiveConsoleSessionId = windows.NewLazySystemDLL("kernel32.dll").NewProc("WTSGetActiveConsoleSessionId")
	procWTSQueryUserToken            = windows.NewLazySystemDLL("wtsapi32.dll").NewProc("WTSQueryUserToken")
)

func (h *SessionHelper) spawnHelper() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("os.Executable: %w", err)
	}

	sessionID, _, _ := procWTSGetActiveConsoleSessionId.Call()
	if sessionID == 0xFFFFFFFF {
		return fmt.Errorf("no active console session")
	}

	var userToken windows.Token
	ret, _, errCall := procWTSQueryUserToken.Call(sessionID, uintptr(unsafe.Pointer(&userToken)))
	if ret == 0 {
		return fmt.Errorf("WTSQueryUserToken: %v", errCall)
	}
	defer userToken.Close()

	var dupToken windows.Token
	if err := windows.DuplicateTokenEx(userToken, 0x02000000, nil,
		windows.SecurityImpersonation, windows.TokenPrimary, &dupToken); err != nil {
		return fmt.Errorf("DuplicateTokenEx: %w", err)
	}
	defer dupToken.Close()

	cmdLine := fmt.Sprintf(`"%s" --clipboard-helper "%s"`, exePath, h.pipeName)
	cmdLineUTF16, _ := windows.UTF16PtrFromString(cmdLine)
	desktop, _ := windows.UTF16PtrFromString("winsta0\\default")
	si := windows.StartupInfo{
		Cb:      uint32(unsafe.Sizeof(windows.StartupInfo{})),
		Desktop: desktop,
	}
	var pi windows.ProcessInformation
	if err := windows.CreateProcessAsUser(dupToken, nil, cmdLineUTF16, nil, nil, false,
		windows.CREATE_NO_WINDOW, nil, nil, &si, &pi); err != nil {
		return fmt.Errorf("CreateProcessAsUser: %w", err)
	}
	windows.CloseHandle(pi.Thread)
	h.helperProc = pi.Process
	log.Printf("📋 Clipboard helper spawned (PID: %d, session: %d)", pi.ProcessId, sessionID)
	return nil
}
