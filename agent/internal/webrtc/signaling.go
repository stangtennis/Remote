package webrtc

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/pion/webrtc/v3"
)

type Session struct {
	ID        string                 `json:"session_id"`
	Token     string                 `json:"token"`
	PIN       string                 `json:"pin"`
	ExpiresAt string                 `json:"expires_at"`
	TurnConfig map[string]interface{} `json:"turn_config"`
}

type SignalMessage struct {
	SessionID string `json:"session_id"`
	FromSide  string `json:"from_side"`
	MsgType   string `json:"msg_type"`
	Payload   struct {
		SDP       string                  `json:"sdp"`
		Candidate *webrtc.ICECandidateInit `json:"candidate"`
	} `json:"payload"`
}

func (m *Manager) ListenForSessions() {
	// Poll for new sessions from dashboard
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		sessions, err := m.fetchPendingSessions()
		if err != nil {
			// Silently continue on error
			continue
		}

		for _, session := range sessions {
			log.Printf("📞 Incoming session: %s (PIN: %s)", session.ID, session.PIN)
			m.sessionID = session.ID
			
			// Handle this session
			go m.handleSession(session)
			
			// Only handle one session at a time
			return
		}
	}
}

func (m *Manager) fetchPendingSessions() ([]Session, error) {
	// Query remote_sessions for pending sessions for this device
	url := m.cfg.SupabaseURL + "/rest/v1/remote_sessions"
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("apikey", m.cfg.SupabaseAnonKey)
	req.Header.Set("Authorization", "Bearer "+m.cfg.SupabaseAnonKey)
	
	q := req.URL.Query()
	q.Add("device_id", "eq."+m.device.ID)
	q.Add("status", "eq.pending")
	q.Add("select", "*")
	req.URL.RawQuery = q.Encode()

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var sessions []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
		return nil, err
	}

	var result []Session
	for _, s := range sessions {
		session := Session{
			ID:  s["id"].(string),
			PIN: s["pin"].(string),
		}
		if token, ok := s["token"].(string); ok {
			session.Token = token
		}
		if turnConfig, ok := s["turn_config"].(map[string]interface{}); ok {
			session.TurnConfig = turnConfig
		}
		result = append(result, session)
	}

	return result, nil
}

func (m *Manager) handleSession(session Session) {
	log.Println("🔧 Setting up WebRTC connection...")

	// Get ICE servers from session TURN config or use default STUN
	iceServers := []webrtc.ICEServer{
		{URLs: []string{"stun:stun.l.google.com:19302"}},
		{URLs: []string{"stun:stun1.l.google.com:19302"}},
	}

	// Parse TURN config from session
	if session.TurnConfig != nil {
		if iceServersRaw, ok := session.TurnConfig["iceServers"].([]interface{}); ok {
			iceServers = []webrtc.ICEServer{}
			for _, serverRaw := range iceServersRaw {
				if serverMap, ok := serverRaw.(map[string]interface{}); ok {
					server := webrtc.ICEServer{}
					
					// Parse URLs
					if urlsRaw, ok := serverMap["urls"]; ok {
						switch urls := urlsRaw.(type) {
						case string:
							server.URLs = []string{urls}
						case []interface{}:
							for _, url := range urls {
								if urlStr, ok := url.(string); ok {
									server.URLs = append(server.URLs, urlStr)
								}
							}
						}
					}
					
					// Parse credentials
					if username, ok := serverMap["username"].(string); ok {
						server.Username = username
					}
					if credential, ok := serverMap["credential"].(string); ok {
						server.Credential = credential
					}
					
					if len(server.URLs) > 0 {
						iceServers = append(iceServers, server)
					}
				}
			}
			log.Printf("🔐 Using TURN config with %d ICE servers", len(iceServers))
		}
	}

	if err := m.CreatePeerConnection(iceServers); err != nil {
		log.Printf("Failed to create peer connection: %v", err)
		return
	}

	// Wait for offer from dashboard
	log.Println("⏳ Waiting for offer from dashboard...")
	m.waitForOffer(session.ID)
}

func (m *Manager) waitForOffer(sessionID string) {
	// Poll for signaling messages
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(30 * time.Second)

	for {
		select {
		case <-timeout:
			log.Println("⏱️  Timeout waiting for offer")
			return
		case <-ticker.C:
			signals, err := m.fetchSignalingMessages(sessionID, "dashboard")
			if err != nil {
				continue
			}

			for _, sig := range signals {
				if sig.MsgType == "offer" {
					log.Println("📨 Received offer from dashboard")
					m.handleOffer(sessionID, sig)
					return
				} else if sig.MsgType == "ice" && sig.Payload.Candidate != nil {
					m.handleICECandidate(sig.Payload.Candidate)
				}
			}
		}
	}
}

func (m *Manager) handleOffer(sessionID string, sig SignalMessage) {
	// Set remote description
	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  sig.Payload.SDP,
	}

	if err := m.peerConnection.SetRemoteDescription(offer); err != nil {
		log.Printf("Failed to set remote description: %v", err)
		return
	}

	// Create answer
	answer, err := m.peerConnection.CreateAnswer(nil)
	if err != nil {
		log.Printf("Failed to create answer: %v", err)
		return
	}

	if err := m.peerConnection.SetLocalDescription(answer); err != nil {
		log.Printf("Failed to set local description: %v", err)
		return
	}

	// Send answer
	m.sendAnswer(sessionID, answer.SDP)

	// Continue listening for ICE candidates
	go m.listenForICE(sessionID)
}

func (m *Manager) listenForICE(sessionID string) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for m.isStreaming || m.peerConnection.ConnectionState() == webrtc.PeerConnectionStateConnecting {
		<-ticker.C

		signals, err := m.fetchSignalingMessages(sessionID, "dashboard")
		if err != nil {
			continue
		}

		for _, sig := range signals {
			if sig.MsgType == "ice" && sig.Payload.Candidate != nil {
				m.handleICECandidate(sig.Payload.Candidate)
			}
		}
	}
}

func (m *Manager) handleICECandidate(candidate *webrtc.ICECandidateInit) {
	if err := m.peerConnection.AddICECandidate(*candidate); err != nil {
		log.Printf("Failed to add ICE candidate: %v", err)
	} else {
		log.Println("➕ Added ICE candidate")
	}
}

func (m *Manager) fetchSignalingMessages(sessionID, fromSide string) ([]SignalMessage, error) {
	url := m.cfg.SupabaseURL + "/rest/v1/session_signaling"
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("apikey", m.cfg.SupabaseAnonKey)
	req.Header.Set("Authorization", "Bearer "+m.cfg.SupabaseAnonKey)
	
	q := req.URL.Query()
	q.Add("session_id", "eq."+sessionID)
	q.Add("from_side", "eq."+fromSide)
	q.Add("select", "*")
	q.Add("order", "created_at.asc")  // Oldest first - get offer before ICE candidates
	q.Add("limit", "50")  // Increased to handle all ICE candidates
	req.URL.RawQuery = q.Encode()

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read response body for debugging
	body, _ := io.ReadAll(resp.Body)
	
	var signals []SignalMessage
	if err := json.Unmarshal(body, &signals); err != nil {
		log.Printf("⚠️  Failed to parse signals: %v (body: %s)", err, string(body))
		return nil, err
	}

	if len(signals) > 0 {
		log.Printf("🔍 Fetched %d signals for session %s from %s", len(signals), sessionID, fromSide)
	}

	return signals, nil
}

func (m *Manager) sendAnswer(sessionID, sdp string) {
	url := m.cfg.SupabaseURL + "/rest/v1/session_signaling"

	payload := map[string]interface{}{
		"session_id": sessionID,
		"from_side":  "agent",
		"msg_type":   "answer",
		"payload": map[string]interface{}{
			"sdp": sdp,
		},
	}

	jsonData, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", m.cfg.SupabaseAnonKey)
	req.Header.Set("Authorization", "Bearer "+m.cfg.SupabaseAnonKey)
	req.Header.Set("Prefer", "return=minimal")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to send answer: %v", err)
		return
	}
	defer resp.Body.Close()

	log.Println("📤 Sent answer to dashboard")
}

func (m *Manager) sendICECandidate(candidate *webrtc.ICECandidate) {
	if m.sessionID == "" {
		return
	}

	url := m.cfg.SupabaseURL + "/rest/v1/session_signaling"

	candidateInit := candidate.ToJSON()
	payload := map[string]interface{}{
		"session_id": m.sessionID,
		"from_side":  "agent",
		"msg_type":   "ice",
		"payload": map[string]interface{}{
			"candidate": candidateInit,
		},
	}

	jsonData, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", m.cfg.SupabaseAnonKey)
	req.Header.Set("Authorization", "Bearer "+m.cfg.SupabaseAnonKey)
	req.Header.Set("Prefer", "return=minimal")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to send ICE candidate: %v", err)
		return
	}
	defer resp.Body.Close()

	// Read and discard body
	io.Copy(io.Discard, resp.Body)
}
