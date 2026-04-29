//go:build windows

package clipboard

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"
	"unsafe"

	"golang.design/x/clipboard"
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
	// Frame: 4-byte length + payload
	header := make([]byte, 4)
	binary.LittleEndian.PutUint32(header, uint32(len(data)))
	var n uint32
	if err := windows.WriteFile(p.handle, header, &n, nil); err != nil {
		return err
	}
	if err := windows.WriteFile(p.handle, data, &n, nil); err != nil {
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
	pipe, err := windows.CreateNamedPipe(pipeNameUTF16,
		windows.PIPE_ACCESS_DUPLEX,
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
	headerBuf := make([]byte, 4)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		// Read length header
		var n uint32
		if err := windows.ReadFile(pipe, headerBuf, &n, nil); err != nil || n != 4 {
			if err != nil && err != windows.ERROR_BROKEN_PIPE {
				log.Printf("⚠️  Clipboard helper pipe read failed: %v", err)
			}
			return
		}
		size := binary.LittleEndian.Uint32(headerBuf)
		if size == 0 || size > 50*1024*1024 {
			log.Printf("⚠️  Clipboard helper sent invalid size: %d", size)
			return
		}
		body := make([]byte, size)
		var got uint32
		for got < size {
			var read uint32
			if err := windows.ReadFile(pipe, body[got:], &read, nil); err != nil {
				log.Printf("⚠️  Clipboard helper pipe body read failed: %v", err)
				return
			}
			got += read
		}

		var msg struct {
			Type    string `json:"type"`    // "text" or "image"
			Content string `json:"content"` // text payload, or base64-encoded PNG
		}
		if err := json.Unmarshal(body, &msg); err != nil {
			log.Printf("⚠️  Clipboard helper sent bad JSON: %v", err)
			continue
		}
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

// SetText pushes text to the helper so it lands in the user-session clipboard.
func (h *SessionHelper) SetText(text string) error {
	return h.send(map[string]interface{}{"cmd": "set_text", "content": text})
}

// SetImage pushes a PNG to the helper.
func (h *SessionHelper) SetImage(pngData []byte) error {
	return h.send(map[string]interface{}{"cmd": "set_image", "content": b64Encode(pngData)})
}

// RememberText / RememberImage notify the helper that a clipboard write came
// from us so the helper's monitor doesn't bounce the same content back.
func (h *SessionHelper) RememberText(text string)             { _ = h.send(map[string]interface{}{"cmd": "remember_text", "content": text}) }
func (h *SessionHelper) RememberImage(pngData []byte)         { _ = h.send(map[string]interface{}{"cmd": "remember_image", "content": b64Encode(pngData)}) }

func (h *SessionHelper) send(msg map[string]interface{}) error {
	if h.pipeWriter == nil {
		return fmt.Errorf("pipe not ready")
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return h.pipeWriter.Write(body)
}

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
	if h.pipeWriter != nil {
		// Send quit so the helper exits cleanly
		_ = h.pipeWriter.Write([]byte(`{"cmd":"quit"}`))
		h.pipeWriter.Close()
	}
	if h.pipeListener != 0 && h.pipeWriter == nil {
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
		pipeHandle, err = windows.CreateFile(pipeNameUTF16,
			windows.GENERIC_READ|windows.GENERIC_WRITE,
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

	if err := clipboard.Init(); err != nil {
		log.Printf("clipboard.Init failed: %v", err)
		return fmt.Errorf("clipboard.Init: %w", err)
	}
	log.Println("clipboard.Init OK")

	w := &pipeWriter{handle: pipeHandle}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Watch text + image changes, forward as length-prefixed JSON.
	textCh := clipboard.Watch(ctx, clipboard.FmtText)
	imgCh := clipboard.Watch(ctx, clipboard.FmtImage)
	log.Printf("Watch channels created: text=%v image=%v", textCh != nil, imgCh != nil)

	if data := clipboard.Read(clipboard.FmtText); len(data) > 0 {
		log.Printf("Initial text in clipboard: %d bytes", len(data))
	}

	// Diagnostic heartbeat goroutine — separate from poller, just to
	// confirm the runtime scheduler is alive.
	go func() {
		for i := 0; ; i++ {
			time.Sleep(2 * time.Second)
			log.Printf("[heartbeat #%d]", i)
		}
	}()

	// Backup polling — log every read to debug
	go func() {
		log.Println("manual poller goroutine alive")
		var lastHash string
		readCount := 0
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			time.Sleep(500 * time.Millisecond)
			{
				data := clipboard.Read(clipboard.FmtText)
				readCount++
				if readCount%4 == 0 { // log every 2s
					preview := string(data)
					if len(preview) > 40 {
						preview = preview[:40] + "..."
					}
					log.Printf("[poll #%d] clipboard.Read returned %d bytes: %q", readCount, len(data), preview)
				}
				if len(data) == 0 || len(data) > 10*1024*1024 {
					continue
				}
				h := hashString(string(data))
				if h == lastHash {
					continue
				}
				lastHash = h
				body, _ := json.Marshal(map[string]interface{}{
					"type":    "text",
					"content": string(data),
				})
				if err := w.Write(body); err == nil {
					log.Printf("📋 [poll] sent text event (%d bytes)", len(data))
				} else {
					log.Printf("[poll] write failed: %v", err)
				}
			}
		}
	}()

	var lastTextHash, lastImageHash string

	go func() {
		log.Println("text watcher goroutine alive")
		for data := range textCh {
			if len(data) == 0 || len(data) > 10*1024*1024 {
				continue
			}
			text := string(data)
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
			log.Printf("📋 sent text event (%d bytes)", len(text))
		}
	}()
	go func() {
		log.Println("image watcher goroutine alive")
		for data := range imgCh {
			if len(data) == 0 || len(data) > 50*1024*1024 {
				continue
			}
			h := hashBytes(data)
			if h == lastImageHash {
				continue
			}
			lastImageHash = h
			body, _ := json.Marshal(map[string]interface{}{
				"type":    "image",
				"content": b64Encode(data),
			})
			if err := w.Write(body); err != nil {
				log.Printf("write image event failed: %v", err)
				cancel()
				return
			}
			log.Printf("📋 sent image event (%d bytes)", len(data))
		}
	}()

	// Read commands from service. Each command is length-prefixed JSON.
	r := bufio.NewReader(&pipeReader{handle: pipeHandle})
	headerBuf := make([]byte, 4)
	for {
		if _, err := io.ReadFull(r, headerBuf); err != nil {
			log.Printf("Pipe closed: %v", err)
			cancel()
			return nil
		}
		size := binary.LittleEndian.Uint32(headerBuf)
		if size == 0 || size > 50*1024*1024 {
			continue
		}
		body := make([]byte, size)
		if _, err := io.ReadFull(r, body); err != nil {
			log.Printf("Read body failed: %v", err)
			cancel()
			return nil
		}
		var cmd struct {
			Cmd     string `json:"cmd"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(body, &cmd); err != nil {
			continue
		}
		switch cmd.Cmd {
		case "set_text":
			clipboard.Write(clipboard.FmtText, []byte(cmd.Content))
			lastTextHash = hashString(cmd.Content)
		case "set_image":
			if data, err := b64Decode(cmd.Content); err == nil {
				clipboard.Write(clipboard.FmtImage, data)
				lastImageHash = hashBytes(data)
			}
		case "remember_text":
			lastTextHash = hashString(cmd.Content)
		case "remember_image":
			if data, err := b64Decode(cmd.Content); err == nil {
				lastImageHash = hashBytes(data)
			}
		case "quit":
			log.Println("Quit command received")
			return nil
		}
	}
}

type pipeReader struct{ handle windows.Handle }

func (p *pipeReader) Read(b []byte) (int, error) {
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
