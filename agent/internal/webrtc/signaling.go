package webrtc

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	Offer     string                 `json:"offer"`
	TurnConfig map[string]interface{} `json:"turn_config"`
}

type SignalMessage struct {
	ID        int    `json:"id"`
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

	handledSessions := make(map[string]bool)
	errorCount := 0
	
	log.Println("🔄 Session polling started (checking every 2 seconds)")
	
	for range ticker.C {
		sessions, err := m.fetchPendingSessions()
		if err != nil {
			errorCount++
			// Log every 30th error to avoid spam (once per minute)
			if errorCount%30 == 1 {
				log.Printf("⚠️  Error fetching sessions (count: %d): %v", errorCount, err)
			}
			continue
		}
		
		// Reset error count on success
		if errorCount > 0 {
			log.Printf("✅ Session polling recovered after %d errors", errorCount)
			errorCount = 0
		}

		for _, session := range sessions {
			// Skip if already handling this session
			if handledSessions[session.ID] {
				continue
			}
			
			// Skip if currently connected
			if m.peerConnection != nil && m.peerConnection.ConnectionState() == webrtc.PeerConnectionStateConnected {
				continue
			}
			
			// Log only when we're actually starting a NEW session
			log.Printf("📞 Incoming session: %s (PIN: %s)", session.ID, session.PIN)
			log.Printf("   Device ID: %s", m.device.ID)
			handledSessions[session.ID] = true
			m.sessionID = session.ID
			
			// Handle this session in background
			go m.handleSession(session)
		}
	}
}

func (m *Manager) fetchPendingSessions() ([]Session, error) {
	// Query webrtc_sessions for sessions with offers for this device
	url := m.cfg.SupabaseURL + "/rest/v1/webrtc_sessions"
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("apikey", m.cfg.SupabaseAnonKey)
	req.Header.Set("Authorization", "Bearer "+m.cfg.SupabaseAnonKey)
	
	q := req.URL.Query()
	q.Add("device_id", "eq."+m.device.ID)
	q.Add("offer", "not.is.null") // Has an offer waiting
	q.Add("answer", "is.null")     // No answer yet
	q.Add("select", "*")
	q.Add("order", "created_at.desc")
	q.Add("limit", "1")
	req.URL.RawQuery = q.Encode()

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Log HTTP status
	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var sessions []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
		return nil, fmt.Errorf("JSON decode failed: %w", err)
	}

	// Reduced logging - only log when no sessions (once per minute)
	if len(sessions) == 0 {
		// Log only every 30 polls (once per minute) to avoid spam
		if time.Now().Unix()%60 < 2 {
			log.Printf("🔍 No pending sessions found for device: %s", m.device.ID)
		}
	}

	var result []Session
	for _, s := range sessions {
		// Get session_id from webrtc_sessions table
		sessionID, ok := s["session_id"].(string)
		if !ok {
			log.Printf("⚠️  Skipping session with invalid session_id: %+v", s)
			continue
		}
		
		// Get offer (SDP) from the session
		offer, _ := s["offer"].(string)
		
		session := Session{
			ID:    sessionID,
			Offer: offer, // Store offer in the Offer field
		}
		
		result = append(result, session)
	}

	return result, nil
}

func (m *Manager) handleSession(session Session) {
	log.Println("🔧 Setting up WebRTC connection...")
	
	// Ensure previous connection is fully cleaned up
	if m.peerConnection != nil {
		log.Println("⚠️  Previous connection still exists, cleaning up first...")
		m.cleanupConnection("Preparing for new session")
		// Small delay to ensure cleanup completes
		time.Sleep(100 * time.Millisecond)
	}

	// Use default STUN servers (TURN can be added later)
	iceServers := []webrtc.ICEServer{
		{URLs: []string{"stun:stun.l.google.com:19302"}},
		{URLs: []string{"stun:stun1.l.google.com:19302"}},
	}

	if err := m.CreatePeerConnection(iceServers); err != nil {
		log.Printf("Failed to create peer connection: %v", err)
		return
	}

	// Extract offer from session
	log.Printf("🔍 DEBUG: session.Offer length: %d", len(session.Offer))
	if len(session.Offer) > 0 {
		log.Printf("🔍 DEBUG: First 100 chars of offer: %.100s", session.Offer)
	}
	if session.Offer == "" {
		log.Println("❌ No offer found in session")
		return
	}

	// Parse the JSON-encoded SessionDescription
	var sessionDesc webrtc.SessionDescription
	if err := json.Unmarshal([]byte(session.Offer), &sessionDesc); err != nil {
		log.Printf("❌ Failed to parse offer JSON: %v", err)
		return
	}

	// Process the offer immediately
	log.Println("📨 Processing offer from controller...")
	m.handleOfferDirect(session.ID, sessionDesc.SDP)
}

func (m *Manager) waitForOffer(sessionID string) {
	// Poll for signaling messages - stop once connected
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	processedIDs := make(map[int]bool)

	// Stop polling once peer connection is established
	for m.peerConnection.ConnectionState() != webrtc.PeerConnectionStateConnected {
		select {
		case <-ticker.C:
			signals, err := m.fetchSignalingMessages(sessionID, "dashboard")
			if err != nil {
				continue
			}

			for _, sig := range signals {
				// Skip already processed signals
				if processedIDs[sig.ID] {
					continue
				}
				processedIDs[sig.ID] = true

				if sig.MsgType == "offer" {
					log.Println("📨 Received offer from dashboard")
					m.handleOffer(sessionID, sig)
					// Continue listening for ICE candidates
					// (don't return - keep processing signals)
				} else if sig.MsgType == "ice" && sig.Payload.Candidate != nil {
					m.handleICECandidate(sig.Payload.Candidate)
				}
			}
		}
	}
	
	log.Println("🛑 Stopped signal polling - connection established")
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
	processedIDs := make(map[int]bool)

	log.Println("🔍 Starting ICE candidate listener...")

	// Only poll while connecting, stop once connected or failed
	for {
		// Check state BEFORE waiting for tick
		if m.peerConnection == nil {
			log.Println("🛑 Stopped ICE candidate polling - connection closed")
			return
		}
		
		state := m.peerConnection.ConnectionState()
		
		// Stop if connected (ICE negotiation complete) or failed/closed
		if state == webrtc.PeerConnectionStateConnected || 
		   state == webrtc.PeerConnectionStateFailed || 
		   state == webrtc.PeerConnectionStateClosed {
			log.Printf("🛑 Stopped ICE candidate polling - connection state: %s", state)
			return
		}

		// Now wait for tick or proceed immediately on first iteration
		select {
		case <-ticker.C:
			// Continue to fetch signals
		}

		signals, err := m.fetchSignalingMessages(sessionID, "dashboard")
		if err != nil {
			continue
		}

		for _, sig := range signals {
			// Skip already processed signals
			if processedIDs[sig.ID] {
				continue
			}
			processedIDs[sig.ID] = true

			if sig.MsgType == "ice" && sig.Payload.Candidate != nil {
				m.handleICECandidate(sig.Payload.Candidate)
			}
		}
	}
}

func (m *Manager) handleICECandidate(candidate *webrtc.ICECandidateInit) {
	// Check if peer connection exists and is in valid state
	if m.peerConnection == nil {
		log.Println("⚠️  Cannot add ICE candidate - no peer connection")
		return
	}
	
	state := m.peerConnection.ConnectionState()
	
	// Only add ICE candidates during connection setup
	// Don't try to add them if already connected, failed, or closed
	if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateClosed {
		// Silently ignore - connection is already done
		return
	}
	
	if err := m.peerConnection.AddICECandidate(*candidate); err != nil {
		// Only log as warning, not error - this can happen during normal operation
		log.Printf("⚠️  Could not add ICE candidate (state: %s): %v", state, err)
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

// handleOfferDirect processes an offer directly from the session
func (m *Manager) handleOfferDirect(sessionID, offerSDP string) {
	// Set remote description
	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  offerSDP,
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

	// Send answer back to webrtc_sessions table
	m.sendAnswer(sessionID, answer.SDP)
}

func (m *Manager) sendAnswer(sessionID, sdp string) {
	url := m.cfg.SupabaseURL + "/rest/v1/webrtc_sessions"

	payload := map[string]interface{}{
		"answer": sdp,
		"status": "answered",
	}

	jsonData, _ := json.Marshal(payload)
	
	// Create PATCH request with session_id filter
	req, _ := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", m.cfg.SupabaseAnonKey)
	req.Header.Set("Authorization", "Bearer "+m.cfg.SupabaseAnonKey)
	req.Header.Set("Prefer", "return=minimal")
	
	// Add query parameter to target specific session
	q := req.URL.Query()
	q.Add("session_id", "eq."+sessionID)
	req.URL.RawQuery = q.Encode()

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("❌ Failed to send answer: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("❌ Failed to send answer: HTTP %d - %s", resp.StatusCode, string(body))
		return
	}

	log.Println("📤 Sent answer to controller")
}

func (m *Manager) updateSessionStatus(status string) {
	if m.sessionID == "" {
		return
	}

	url := m.cfg.SupabaseURL + "/rest/v1/remote_sessions"
	
	updateData := map[string]interface{}{
		"status": status,
	}
	
	jsonData, err := json.Marshal(updateData)
	if err != nil {
		log.Printf("Failed to marshal session update: %v", err)
		return
	}

	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Failed to create session update request: %v", err)
		return
	}

	req.Header.Set("apikey", m.cfg.SupabaseAnonKey)
	req.Header.Set("Authorization", "Bearer "+m.cfg.SupabaseAnonKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=minimal")
	
	q := req.URL.Query()
	q.Add("id", "eq."+m.sessionID)
	req.URL.RawQuery = q.Encode()

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to update session status: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Failed to update session (status %d): %s", resp.StatusCode, string(body))
		return
	}

	log.Printf("✅ Session %s marked as %s", m.sessionID, status)
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
		log.Printf("❌ Failed to send ICE candidate: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("❌ Failed to send ICE candidate: HTTP %d - %s", resp.StatusCode, string(body))
		return
	}

	// Read and discard body
	io.Copy(io.Discard, resp.Body)
}
