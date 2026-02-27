package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
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
		client:     client,
		deviceID:   deviceID,
		deviceName: deviceName,
		lastUsedAt: time.Now(),
	}

	client.SetOnFrame(func(frameData []byte) {
		conn.mu.Lock()
		conn.lastFrame = make([]byte, len(frameData))
		copy(conn.lastFrame, frameData)
		conn.lastFrameAt = time.Now()
		conn.mu.Unlock()
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

	if err := client.CreatePeerConnection(iceServers); err != nil {
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
