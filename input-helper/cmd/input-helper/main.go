package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	Version = "v1.0.0"
	Port    = 9877
)

// Allowed origins for WebSocket connections
var allowedOrigins = map[string]bool{
	"https://stangtennis.github.io": true,
	"http://localhost":              true,
	"http://127.0.0.1":              true,
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return false
		}
		// Check exact match or match with port variants
		if allowedOrigins[origin] {
			return true
		}
		// Allow localhost with any port
		for allowed := range allowedOrigins {
			if len(origin) > len(allowed) && origin[:len(allowed)] == allowed && origin[len(allowed)] == ':' {
				return true
			}
		}
		log.Printf("⚠️ Rejected WebSocket from origin: %s", origin)
		return false
	},
}

// InputEvent represents an input event from the browser
type InputEvent struct {
	Type      string  `json:"type"`
	T         string  `json:"t,omitempty"` // alias from dashboard shorthand
	Seq       int64   `json:"seq,omitempty"`
	Timestamp int64   `json:"ts,omitempty"`

	// Auth
	Token     string `json:"token,omitempty"`
	DeviceID  string `json:"device_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`

	// Mouse move (relative)
	DX int `json:"dx,omitempty"`
	DY int `json:"dy,omitempty"`

	// Mouse move (absolute, normalized 0-1)
	X float64 `json:"x,omitempty"`
	Y float64 `json:"y,omitempty"`

	// Mouse button
	Button string `json:"button,omitempty"`
	Down   bool   `json:"down,omitempty"`

	// Key
	Code  string `json:"code,omitempty"`
	Key   string `json:"key,omitempty"`
	Ctrl  bool   `json:"ctrl,omitempty"`
	Shift bool   `json:"shift,omitempty"`
	Alt   bool   `json:"alt,omitempty"`

	// Clipboard
	Direction string `json:"direction,omitempty"`
	Content   string `json:"content,omitempty"`

	// Control
	Action string `json:"action,omitempty"`
	Scope  string `json:"scope,omitempty"`
}

// Response sent back to browser
type Response struct {
	Type    string `json:"type"`
	Seq     int64  `json:"seq,omitempty"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	Content string `json:"content,omitempty"`
}

// Session state
type Session struct {
	authenticated bool
	deviceID      string
	sessionID     string
	mouseEnabled  bool
	keyEnabled    bool
	clipEnabled   bool
	lastActivity  time.Time
	mu            sync.Mutex
}

// InputHelper manages WebSocket connections and input injection
type InputHelper struct {
	sessions map[*websocket.Conn]*Session
	mu       sync.RWMutex
	injector *InputInjector
}

func NewInputHelper() *InputHelper {
	return &InputHelper{
		sessions: make(map[*websocket.Conn]*Session),
		injector: NewInputInjector(),
	}
}

func (h *InputHelper) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	session := &Session{
		mouseEnabled: true,  // Mouse enabled by default
		keyEnabled:   false, // Keyboard requires explicit enable
		clipEnabled:  false, // Clipboard requires explicit enable
		lastActivity: time.Now(),
	}

	h.mu.Lock()
	h.sessions[conn] = session
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.sessions, conn)
		h.mu.Unlock()
	}()

	log.Printf("🔌 New WebSocket connection from %s", r.RemoteAddr)

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		var event InputEvent
		if err := json.Unmarshal(message, &event); err != nil {
			log.Printf("Invalid JSON: %v", err)
			continue
		}

		response := h.handleEvent(conn, session, &event)
		if response != nil {
			responseJSON, _ := json.Marshal(response)
			conn.WriteMessage(websocket.TextMessage, responseJSON)
		}
	}

	log.Printf("🔌 WebSocket connection closed")
}

func (h *InputHelper) handleEvent(conn *websocket.Conn, session *Session, event *InputEvent) *Response {
	session.mu.Lock()
	session.lastActivity = time.Now()
	session.mu.Unlock()

	// Accept "t" as alias for "type" (dashboard shorthand)
	if event.Type == "" && event.T != "" {
		event.Type = event.T
	}

	switch event.Type {
	case "auth":
		return h.handleAuth(session, event)

	case "mouse_move":
		if !session.authenticated {
			return &Response{Type: "ack", Seq: event.Seq, OK: false, Error: "not authenticated"}
		}
		if !session.mouseEnabled {
			return nil // Silently ignore
		}
		h.injector.MouseMoveRelative(event.DX, event.DY)
		return nil // Don't ack every mouse move (too noisy)

	case "mouse_abs":
		if !session.authenticated {
			return &Response{Type: "ack", Seq: event.Seq, OK: false, Error: "not authenticated"}
		}
		if !session.mouseEnabled {
			return nil
		}
		h.injector.MouseMoveAbsolute(event.X, event.Y)
		return nil

	case "mouse_button":
		if !session.authenticated {
			return &Response{Type: "ack", Seq: event.Seq, OK: false, Error: "not authenticated"}
		}
		if !session.mouseEnabled {
			return nil
		}
		h.injector.MouseButton(event.Button, event.Down)
		return &Response{Type: "ack", Seq: event.Seq, OK: true}

	case "wheel":
		if !session.authenticated {
			return &Response{Type: "ack", Seq: event.Seq, OK: false, Error: "not authenticated"}
		}
		if !session.mouseEnabled {
			return nil
		}
		h.injector.MouseWheel(event.DX, event.DY)
		return nil

	case "key":
		if !session.authenticated {
			return &Response{Type: "ack", Seq: event.Seq, OK: false, Error: "not authenticated"}
		}
		if !session.keyEnabled {
			return &Response{Type: "ack", Seq: event.Seq, OK: false, Error: "keyboard not enabled"}
		}
		h.injector.KeyEvent(event.Code, event.Down, event.Ctrl, event.Shift, event.Alt)
		return &Response{Type: "ack", Seq: event.Seq, OK: true}

	case "clipboard":
		if !session.authenticated {
			return &Response{Type: "ack", Seq: event.Seq, OK: false, Error: "not authenticated"}
		}
		if !session.clipEnabled {
			return &Response{Type: "ack", Seq: event.Seq, OK: false, Error: "clipboard not enabled"}
		}
		if event.Direction == "to_system" {
			h.injector.SetClipboard(event.Content)
			return &Response{Type: "ack", Seq: event.Seq, OK: true}
		} else if event.Direction == "from_system" {
			content := h.injector.GetClipboard()
			return &Response{Type: "clipboard_content", Seq: event.Seq, OK: true, Content: content}
		}

	case "control":
		if !session.authenticated {
			return &Response{Type: "ack", Seq: event.Seq, OK: false, Error: "not authenticated"}
		}
		return h.handleControl(session, event)

	default:
		log.Printf("Unknown event type: %s", event.Type)
	}

	return nil
}

func (h *InputHelper) handleAuth(session *Session, event *InputEvent) *Response {
	// Validate required fields and minimum token length
	if event.Token == "" || event.DeviceID == "" {
		return &Response{Type: "ack", OK: false, Error: "missing token or device_id"}
	}
	if len(event.Token) < 8 {
		return &Response{Type: "ack", OK: false, Error: "invalid token"}
	}
	if event.SessionID == "" {
		return &Response{Type: "ack", OK: false, Error: "missing session_id"}
	}

	session.mu.Lock()
	session.authenticated = true
	session.deviceID = event.DeviceID
	session.sessionID = event.SessionID
	session.mu.Unlock()

	log.Printf("✅ Authenticated: device=%s session=%s", event.DeviceID, event.SessionID)

	return &Response{
		Type: "status",
		OK:   true,
	}
}

func (h *InputHelper) handleControl(session *Session, event *InputEvent) *Response {
	session.mu.Lock()
	defer session.mu.Unlock()

	switch event.Action {
	case "enable":
		switch event.Scope {
		case "mouse":
			session.mouseEnabled = true
		case "keyboard":
			session.keyEnabled = true
		case "clipboard":
			session.clipEnabled = true
		case "all":
			session.mouseEnabled = true
			session.keyEnabled = true
			session.clipEnabled = true
		}
		log.Printf("🎮 Enabled: %s", event.Scope)

	case "disable":
		switch event.Scope {
		case "mouse":
			session.mouseEnabled = false
		case "keyboard":
			session.keyEnabled = false
		case "clipboard":
			session.clipEnabled = false
		case "all":
			session.mouseEnabled = false
			session.keyEnabled = false
			session.clipEnabled = false
		}
		log.Printf("🎮 Disabled: %s", event.Scope)

	case "pause":
		session.mouseEnabled = false
		session.keyEnabled = false
		session.clipEnabled = false
		log.Printf("⏸️ Paused all input")

	case "resume":
		session.mouseEnabled = true
		// Don't auto-enable keyboard/clipboard on resume
		log.Printf("▶️ Resumed mouse input")
	}

	return &Response{Type: "ack", Seq: event.Seq, OK: true}
}

func (h *InputHelper) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	h.mu.RLock()
	sessionCount := len(h.sessions)
	h.mu.RUnlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "ok",
		"version":  Version,
		"sessions": sessionCount,
	})
}

func (h *InputHelper) handleStop(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Pause all sessions
	h.mu.RLock()
	for _, session := range h.sessions {
		session.mu.Lock()
		session.mouseEnabled = false
		session.keyEnabled = false
		session.clipEnabled = false
		session.mu.Unlock()
	}
	h.mu.RUnlock()

	log.Printf("🛑 Emergency stop triggered")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "stopped",
	})
}

func main() {
	port := flag.Int("port", Port, "WebSocket server port")
	flag.Parse()

	helper := NewInputHelper()

	http.HandleFunc("/input", helper.handleWebSocket)
	http.HandleFunc("/status", helper.handleStatus)
	http.HandleFunc("/stop", helper.handleStop)

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	log.Printf("🚀 Input Helper %s starting on %s", Version, addr)
	log.Printf("📡 WebSocket: ws://%s/input", addr)
	log.Printf("📊 Status: http://%s/status", addr)
	log.Printf("🛑 Stop: POST http://%s/stop", addr)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
