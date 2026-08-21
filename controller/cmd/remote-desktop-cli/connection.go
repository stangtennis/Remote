package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/stangtennis/Remote/controller/internal/config"
	rtc "github.com/stangtennis/Remote/controller/internal/webrtc"
)

const idleTimeout = 5 * time.Minute

// DeviceConnection holds an active WebRTC connection to a device
type DeviceConnection struct {
	client      *rtc.Client
	signaling   *rtc.SignalingClient
	sessionID   string
	deviceID    string
	deviceName  string
	lastFrame   []byte
	lastFrameAt time.Time
	lastUsedAt  time.Time
	connected   bool
	mu          sync.RWMutex

	// Routers for op-based JSON channels (shell + process). The exec/ps/sysinfo
	// commands generate a UUID and register a subscriber here; arriving messages
	// are delivered to the subscriber whose id matches.
	shellRouter   *channelRouter
	processRouter *channelRouter
	fileRouter    *fileTransferRouter
}

// channelRouter dispatches incoming JSON-with-"id" messages to per-id subscribers.
// One subscriber per id; concurrent execs use distinct ids.
type channelRouter struct {
	mu   sync.Mutex
	subs map[string]chan []byte
}

func newChannelRouter() *channelRouter {
	return &channelRouter{subs: make(map[string]chan []byte)}
}

// Subscribe registers a buffered channel for the given id. The buffer must be
// large enough to absorb burst output without dropping messages.
func (r *channelRouter) Subscribe(id string) chan []byte {
	ch := make(chan []byte, 256)
	r.mu.Lock()
	r.subs[id] = ch
	r.mu.Unlock()
	return ch
}

// Unsubscribe removes the subscriber and closes its channel.
func (r *channelRouter) Unsubscribe(id string) {
	r.mu.Lock()
	if ch, ok := r.subs[id]; ok {
		close(ch)
		delete(r.subs, id)
	}
	r.mu.Unlock()
}

// Dispatch delivers a raw message to the subscriber whose id field matches,
// falling back to the generic "" subscriber if no id field is present.
func (r *channelRouter) Dispatch(data []byte) {
	var probe struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(data, &probe)

	r.mu.Lock()
	ch, ok := r.subs[probe.ID]
	if !ok {
		ch, ok = r.subs[""] // generic fallback (e.g., ps/sysinfo without id)
	}
	r.mu.Unlock()
	if !ok {
		return
	}
	select {
	case ch <- data:
	default:
		// subscriber backed up — drop to avoid blocking the data channel
	}
}

// ConnectionManager manages a pool of WebRTC connections
type ConnectionManager struct {
	connections map[string]*DeviceConnection // device_id -> connection
	cfg         *config.Config
	auth        *authInfo
	mu          sync.RWMutex
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager(cfg *config.Config, auth *authInfo) *ConnectionManager {
	return &ConnectionManager{
		connections: make(map[string]*DeviceConnection),
		cfg:         cfg,
		auth:        auth,
	}
}

// Connect establishes a WebRTC connection to a device
func (cm *ConnectionManager) Connect(deviceID, deviceName string) error {
	cm.mu.Lock()
	if conn, exists := cm.connections[deviceID]; exists && conn.connected {
		cm.mu.Unlock()
		conn.mu.Lock()
		conn.lastUsedAt = time.Now()
		conn.mu.Unlock()
		return nil
	}
	cm.mu.Unlock()

	log.Printf("[cli] Connecting to device %s (%s)...", deviceName, deviceID)

	client, err := rtc.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create WebRTC client: %w", err)
	}

	conn := &DeviceConnection{
		client:        client,
		deviceID:      deviceID,
		deviceName:    deviceName,
		lastUsedAt:    time.Now(),
		shellRouter:   newChannelRouter(),
		processRouter: newChannelRouter(),
		fileRouter:    newFileTransferRouter(),
	}

	client.SetOnFrame(func(frameData []byte) {
		conn.mu.Lock()
		conn.lastFrame = make([]byte, len(frameData))
		copy(conn.lastFrame, frameData)
		conn.lastFrameAt = time.Now()
		conn.mu.Unlock()
	})

	client.SetOnShellMessage(func(data []byte) {
		conn.shellRouter.Dispatch(data)
	})
	client.SetOnProcessMessage(func(data []byte) {
		conn.processRouter.Dispatch(data)
	})
	client.SetOnFileMessage(func(data []byte) {
		conn.fileRouter.Dispatch(data)
	})

	connectedCh := make(chan bool, 1)
	client.SetOnConnected(func() {
		log.Printf("[cli] WebRTC connected to %s", deviceName)
		conn.mu.Lock()
		conn.connected = true
		conn.mu.Unlock()
		select {
		case connectedCh <- true:
		default:
		}
	})

	client.SetOnDisconnected(func() {
		log.Printf("[cli] WebRTC disconnected from %s", deviceName)
		conn.mu.Lock()
		conn.connected = false
		conn.mu.Unlock()
	})

	token := cm.auth.GetToken()
	iceServers := fetchICEServers(cm.cfg.SupabaseURL, cm.cfg.SupabaseAnonKey, token)

	if forceRelayEnabled() {
		log.Println("[cli] RD_FORCE_RELAY enabled — using TURN relay-only ICE policy")
		if err := client.CreatePeerConnectionWithPolicy(iceServers, webrtc.ICETransportPolicyRelay); err != nil {
			return fmt.Errorf("failed to create peer connection: %w", err)
		}
	} else if err := client.CreatePeerConnection(iceServers); err != nil {
		return fmt.Errorf("failed to create peer connection: %w", err)
	}

	signalingClient := rtc.NewSignalingClient(cm.cfg.SupabaseURL, cm.cfg.SupabaseAnonKey, token)
	conn.signaling = signalingClient

	session, err := signalingClient.CreateSession(deviceID, cm.auth.userID)
	if err != nil {
		client.Close()
		return fmt.Errorf("failed to create session: %w", err)
	}
	conn.sessionID = session.SessionID

	offerJSON, err := client.CreateOffer()
	if err != nil {
		client.Close()
		signalingClient.DeleteSession(session.SessionID)
		return fmt.Errorf("failed to create offer: %w", err)
	}

	if err := signalingClient.SendOffer(session.SessionID, offerJSON); err != nil {
		client.Close()
		signalingClient.DeleteSession(session.SessionID)
		return fmt.Errorf("failed to send offer: %w", err)
	}

	answerJSON, err := signalingClient.WaitForAnswer(session.SessionID, 30*time.Second)
	if err != nil {
		client.Close()
		signalingClient.DeleteSession(session.SessionID)
		return fmt.Errorf("failed to get answer (timeout): %w", err)
	}

	if err := client.SetAnswer(answerJSON); err != nil {
		client.Close()
		signalingClient.DeleteSession(session.SessionID)
		return fmt.Errorf("failed to set answer: %w", err)
	}

	select {
	case <-connectedCh:
		log.Printf("[cli] Connected to %s", deviceName)
	case <-time.After(30 * time.Second):
		client.Close()
		signalingClient.DeleteSession(session.SessionID)
		return fmt.Errorf("timeout waiting for WebRTC connection")
	}

	cm.mu.Lock()
	cm.connections[deviceID] = conn
	cm.mu.Unlock()

	return nil
}

// ConnectSupport connects the authenticated admin CLI to a temporary support
// session. The session is identified by its server-side UUID, not by a device
// registration, and uses only outbound Supabase/Cloudflare traffic.
func (cm *ConnectionManager) ConnectSupport(supportSessionID string) error {
	deviceKey := "support:" + supportSessionID
	cm.mu.RLock()
	if conn, exists := cm.connections[deviceKey]; exists && conn.connected {
		cm.mu.RUnlock()
		return nil
	}
	cm.mu.RUnlock()

	client, err := rtc.NewClient()
	if err != nil {
		return err
	}
	conn := &DeviceConnection{
		client:        client,
		deviceID:      deviceKey,
		deviceName:    "AI Support",
		lastUsedAt:    time.Now(),
		shellRouter:   newChannelRouter(),
		processRouter: newChannelRouter(),
		fileRouter:    newFileTransferRouter(),
	}
	client.SetOnFrame(func(frameData []byte) {
		conn.mu.Lock()
		conn.lastFrame = append(conn.lastFrame[:0], frameData...)
		conn.lastFrameAt = time.Now()
		conn.mu.Unlock()
	})
	client.SetOnShellMessage(func(data []byte) { conn.shellRouter.Dispatch(data) })
	client.SetOnProcessMessage(func(data []byte) { conn.processRouter.Dispatch(data) })
	client.SetOnFileMessage(func(data []byte) { conn.fileRouter.Dispatch(data) })
	connectedCh := make(chan bool, 1)
	client.SetOnConnected(func() {
		conn.mu.Lock()
		conn.connected = true
		conn.mu.Unlock()
		select {
		case connectedCh <- true:
		default:
		}
	})
	client.SetOnDisconnected(func() {
		conn.mu.Lock()
		conn.connected = false
		conn.mu.Unlock()
	})

	token := cm.auth.GetToken()
	iceServers, err := fetchSupportICEServers(cm.cfg.SupabaseURL, cm.cfg.SupabaseAnonKey, token)
	if err != nil {
		client.Close()
		return err
	}
	if err := client.CreatePeerConnectionWithPolicy(iceServers, webrtc.ICETransportPolicyRelay); err != nil {
		client.Close()
		return err
	}
	signaling := rtc.NewSignalingClient(cm.cfg.SupabaseURL, cm.cfg.SupabaseAnonKey, token)
	conn.signaling = signaling
	offerJSON, err := client.CreateOffer()
	if err != nil {
		client.Close()
		return err
	}
	var offer map[string]interface{}
	if err := json.Unmarshal([]byte(offerJSON), &offer); err != nil {
		client.Close()
		return err
	}
	offerID := fmt.Sprintf("support-offer-%d", time.Now().UnixNano())
	offer["offer_id"] = offerID
	if err := signaling.SendSupportSignal(supportSessionID, "offer", offer); err != nil {
		client.Close()
		return err
	}
	conn.sessionID = supportSessionID
	deadline := time.Now().Add(60 * time.Second)
	answerReceived := false
	processedSignals := make(map[int]bool)
	for time.Now().Before(deadline) {
		signals, pollErr := signaling.GetSupportSignals(supportSessionID)
		if pollErr == nil {
			for _, signal := range signals {
				if processedSignals[signal.ID] {
					continue
				}
				processedSignals[signal.ID] = true
				switch signal.MsgType {
				case "answer":
					if answerReceived || signal.Payload["type"] != "answer" {
						continue
					}
					answerID, ok := signal.Payload["offer_id"].(string)
					if !ok || answerID != offerID {
						continue
					}
					answerJSON, _ := json.Marshal(signal.Payload)
					if err := client.SetAnswer(string(answerJSON)); err != nil {
						client.Close()
						return err
					}
					answerReceived = true
				case "ice":
					candidate, ok := signal.Payload["candidate"].(string)
					if !ok || candidate == "" || !answerReceived {
						continue
					}
					init := webrtc.ICECandidateInit{Candidate: candidate}
					if mid, ok := signal.Payload["sdpMid"].(string); ok {
						init.SDPMid = &mid
					}
					if index, ok := signal.Payload["sdpMLineIndex"].(float64); ok {
						value := uint16(index)
						init.SDPMLineIndex = &value
					}
					_ = client.AddRemoteICECandidate(init)
				}
			}
		}
		if answerReceived {
			select {
			case <-connectedCh:
				goto supportConnected
			default:
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	client.Close()
	if !answerReceived {
		return fmt.Errorf("timeout waiting for support answer")
	}
	return fmt.Errorf("timeout waiting for support connection")

supportConnected:
	cm.mu.Lock()
	cm.connections[deviceKey] = conn
	cm.mu.Unlock()
	return nil
}

func (cm *ConnectionManager) AuditSupportAction(deviceID, actionType, status, summary, target string, details map[string]interface{}) error {
	if !strings.HasPrefix(deviceID, "support:") {
		return nil
	}
	body, err := json.Marshal(map[string]interface{}{
		"action":         "record-admin-action",
		"session_id":     strings.TrimPrefix(deviceID, "support:"),
		"action_type":    actionType,
		"action_status":  status,
		"action_summary": summary,
		"action_target":  target,
		"action_details": details,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", cm.cfg.SupabaseURL+"/functions/v1/support-signal", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cm.auth.GetToken())
	req.Header.Set("apikey", cm.cfg.SupabaseAnonKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("support audit failed (%d): %s", resp.StatusCode, string(data))
	}
	return nil
}

func forceRelayEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("RD_FORCE_RELAY"))) {
	case "1", "true", "yes", "y", "on", "relay":
		return true
	default:
		return false
	}
}

// Disconnect closes a WebRTC connection
func (cm *ConnectionManager) Disconnect(deviceID string) error {
	cm.mu.Lock()
	conn, exists := cm.connections[deviceID]
	if !exists {
		cm.mu.Unlock()
		return fmt.Errorf("no connection to device %s", deviceID)
	}
	delete(cm.connections, deviceID)
	cm.mu.Unlock()

	if conn.signaling != nil && conn.sessionID != "" {
		conn.signaling.DeleteSession(conn.sessionID)
	}
	return conn.client.Close()
}

// GetConnection returns an active connection, updating its last-used time
func (cm *ConnectionManager) GetConnection(deviceID string) (*DeviceConnection, error) {
	cm.mu.RLock()
	conn, exists := cm.connections[deviceID]
	cm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("not connected to device %s", deviceID)
	}

	conn.mu.Lock()
	if !conn.connected {
		conn.mu.Unlock()
		return nil, fmt.Errorf("connection to %s is disconnected", deviceID)
	}
	conn.lastUsedAt = time.Now()
	conn.mu.Unlock()

	return conn, nil
}

// GetLastFrame returns the cached last frame
func (dc *DeviceConnection) GetLastFrame() ([]byte, time.Time) {
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	return dc.lastFrame, dc.lastFrameAt
}

// SendInput sends an input event over the data channel
func (dc *DeviceConnection) SendInput(inputJSON string) error {
	return dc.client.SendInput(inputJSON)
}

// SendShell writes a JSON op message to the shell data channel.
func (dc *DeviceConnection) SendShell(data []byte) error {
	return dc.client.SendShellData(data)
}

// SendProcess writes a JSON op message to the process data channel.
func (dc *DeviceConnection) SendProcess(data []byte) error {
	return dc.client.SendProcessData(data)
}

// SendFile writes a JSON op message to the file data channel.
func (dc *DeviceConnection) SendFile(data []byte) error {
	return dc.client.SendFileData(data)
}

// ShellReady reports whether the shell channel is open.
func (dc *DeviceConnection) ShellReady() bool { return dc.client.ShellChannelReady() }

// ProcessReady reports whether the process channel is open.
func (dc *DeviceConnection) ProcessReady() bool { return dc.client.ProcessChannelReady() }

// StartIdleChecker starts a goroutine that disconnects idle connections
func (cm *ConnectionManager) StartIdleChecker() {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			cm.mu.RLock()
			var toDisconnect []string
			for id, conn := range cm.connections {
				conn.mu.RLock()
				if time.Since(conn.lastUsedAt) > idleTimeout {
					toDisconnect = append(toDisconnect, id)
				}
				conn.mu.RUnlock()
			}
			cm.mu.RUnlock()

			for _, id := range toDisconnect {
				log.Printf("[cli] Idle timeout: disconnecting %s", id)
				cm.Disconnect(id)
			}
		}
	}()
}

// fetchICEServers gets TURN/STUN servers
func fetchICEServers(supabaseURL, anonKey, authToken string) []webrtc.ICEServer {
	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest("POST", supabaseURL+"/functions/v1/turn-credentials", nil)
	req.Header.Set("Authorization", "Bearer "+authToken)
	req.Header.Set("apikey", anonKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err == nil && resp.StatusCode == 200 {
		defer resp.Body.Close()
		var result struct {
			ICEServers []struct {
				URLs       interface{} `json:"urls"`
				Username   string      `json:"username,omitempty"`
				Credential string      `json:"credential,omitempty"`
			} `json:"iceServers"`
		}
		if json.NewDecoder(resp.Body).Decode(&result) == nil {
			var servers []webrtc.ICEServer
			for _, s := range result.ICEServers {
				var urls []string
				switch v := s.URLs.(type) {
				case string:
					urls = []string{v}
				case []interface{}:
					for _, u := range v {
						if str, ok := u.(string); ok {
							urls = append(urls, str)
						}
					}
				}
				server := webrtc.ICEServer{URLs: urls}
				if s.Username != "" {
					server.Username = s.Username
					server.Credential = s.Credential
				}
				servers = append(servers, server)
			}
			if len(servers) > 0 {
				log.Println("[cli] TURN credentials fetched")
				return servers
			}
		}
	}

	servers := []webrtc.ICEServer{
		{URLs: []string{"stun:stun.l.google.com:19302"}},
	}
	if ts := os.Getenv("TURN_SERVER"); ts != "" {
		servers = append(servers, webrtc.ICEServer{
			URLs:       []string{"turn:" + ts, "turn:" + ts + "?transport=tcp"},
			Username:   os.Getenv("TURN_USERNAME"),
			Credential: os.Getenv("TURN_PASSWORD"),
		})
	}
	return servers
}

func fetchSupportICEServers(supabaseURL, anonKey, authToken string) ([]webrtc.ICEServer, error) {
	req, err := http.NewRequest("POST", supabaseURL+"/functions/v1/turn-credentials", strings.NewReader(`{"require_relay":true}`))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+authToken)
	req.Header.Set("apikey", anonKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Cloudflare TURN unavailable (%d): %s", resp.StatusCode, string(body))
	}
	var result struct {
		ICEServers []struct {
			URLs       interface{} `json:"urls"`
			Username   string      `json:"username"`
			Credential string      `json:"credential"`
		} `json:"iceServers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	servers := make([]webrtc.ICEServer, 0, len(result.ICEServers))
	for _, item := range result.ICEServers {
		var urls []string
		switch value := item.URLs.(type) {
		case string:
			urls = []string{value}
		case []interface{}:
			for _, raw := range value {
				if text, ok := raw.(string); ok {
					urls = append(urls, text)
				}
			}
		}
		if len(urls) > 0 && item.Username != "" {
			servers = append(servers, webrtc.ICEServer{URLs: urls, Username: item.Username, Credential: item.Credential})
		}
	}
	if len(servers) == 0 {
		return nil, fmt.Errorf("Cloudflare TURN response contained no relay server")
	}
	return servers, nil
}
