package webrtc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pion/webrtc/v3"
)

// contains is a helper function for string contains check
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// fetchTurnCredentials fetches TURN credentials from the Supabase Edge Function
func fetchTurnCredentials(supabaseURL, anonKey, authToken string) []webrtc.ICEServer {
	if supabaseURL == "" || authToken == "" {
		return nil
	}

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("POST", supabaseURL+"/functions/v1/turn-credentials", nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+authToken)
	req.Header.Set("apikey", anonKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("⚠️ TURN fetch failed: %v", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil
	}

	var result struct {
		ICEServers []struct {
			URLs       interface{} `json:"urls"`
			Username   string      `json:"username,omitempty"`
			Credential string      `json:"credential,omitempty"`
		} `json:"iceServers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil
	}

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

	if len(servers) > 2 {
		log.Printf("🔒 TURN credentials hentet automatisk")
	}
	return servers
}

// getICEServers returns the ICE server configuration with STUN and optional TURN
func (m *Manager) getICEServers() []webrtc.ICEServer {
	// Try fetching from Edge Function first
	authToken, _ := m.tokenProvider.GetToken()
	if servers := fetchTurnCredentials(m.cfg.SupabaseURL, m.cfg.SupabaseAnonKey, authToken); len(servers) > 0 {
		return servers
	}

	// Fallback to env vars
	iceServers := []webrtc.ICEServer{
		{URLs: []string{"stun:stun.l.google.com:19302"}},
		{URLs: []string{"stun:stun1.l.google.com:19302"}},
	}

	turnServer := os.Getenv("TURN_SERVER")
	turnUser := os.Getenv("TURN_USERNAME")
	turnPass := os.Getenv("TURN_PASSWORD")
	if turnServer != "" && turnUser != "" && turnPass != "" {
		iceServers = append(iceServers, webrtc.ICEServer{
			URLs:       []string{"turn:" + turnServer, "turn:" + turnServer + "?transport=tcp"},
			Username:   turnUser,
			Credential: turnPass,
		})
		log.Printf("🔒 TURN server configured: %s", turnServer)
	}

	return iceServers
}

type Session struct {
	ID         string                 `json:"session_id"`
	Token      string                 `json:"token"`
	PIN        string                 `json:"pin"`
	ExpiresAt  string                 `json:"expires_at"`
	Offer      string                 `json:"offer"`
	TurnConfig map[string]interface{} `json:"turn_config"`
}

type SignalMessage struct {
	ID        int             `json:"id"`
	SessionID string          `json:"session_id"`
	FromSide  string          `json:"from_side"`
	MsgType   string          `json:"msg_type"`
	Payload   json.RawMessage `json:"payload"`
}

// OfferPayload for offer/answer messages
type OfferPayload struct {
	Type string `json:"type"`
	SDP  string `json:"sdp"`
}

// ICEPayload for ICE candidate messages - dashboard sends candidate directly in payload
type ICEPayload struct {
	Candidate        string `json:"candidate"`
	SDPMid           string `json:"sdpMid"`
	SDPMLineIndex    *int   `json:"sdpMLineIndex"`
	UsernameFragment string `json:"usernameFragment"`
}

func (m *Manager) ListenForSessions() {
	// Poll for new sessions from BOTH controller (webrtc_sessions) AND dashboard (session_signaling)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	handledSessions := make(map[string]bool)
	errorCount := 0

	log.Println("🔄 Session polling started (checking every 2 seconds)")
	log.Println("   Listening on: webrtc_sessions (controller) + session_signaling (dashboard)")
	log.Printf("   Device ID: %s", m.device.ID)
	log.Printf("   Supabase URL: %s", m.cfg.SupabaseURL)

	// Start kick signal listener in background
	go m.listenForKickSignals()

	for range ticker.C {
		isConnected := m.peerConnection != nil && m.peerConnection.ConnectionState() == webrtc.PeerConnectionStateConnected

		// Check for kick signals on current session
		if m.sessionID != "" && m.peerConnection != nil {
			if kicked := m.checkIfKicked(); kicked {
				log.Println("🔴 Session was kicked - cleaning up for new connection")
				m.cleanupConnection("Kicked by new controller")
				// Clear handledSessions so we pick up new offers immediately
				handledSessions = make(map[string]bool)
				isConnected = false
			}
		}

		// 1. Check webrtc_sessions (Go controller)
		sessions, err := m.fetchPendingSessions()
		if err != nil {
			errorCount++
			if errorCount%30 == 1 {
				log.Printf("⚠️  Error fetching webrtc_sessions (count: %d): %v", errorCount, err)
			}
		} else if errorCount > 0 {
			log.Printf("✅ Session polling recovered after %d errors", errorCount)
			errorCount = 0
		}

		// Only handle the NEWEST unhandled controller session
		var newestCtrlSession *Session
		for i := range sessions {
			if handledSessions[sessions[i].ID] {
				continue
			}
			handledSessions[sessions[i].ID] = true
			if newestCtrlSession == nil {
				newestCtrlSession = &sessions[i]
			}
		}
		if newestCtrlSession != nil {
			if isConnected {
				// New controller session while connected — takeover
				log.Printf("📞 New session (controller) while connected — takeover: %s", newestCtrlSession.ID)
				m.cleanupConnection("New controller connection")
			}
			log.Printf("📞 Incoming session (controller): %s", newestCtrlSession.ID)
			log.Printf("   Device ID: %s", m.device.ID)
			m.sessionID = newestCtrlSession.ID
			go m.handleSession(*newestCtrlSession)
		}

		// 2. Check session_signaling (Web dashboard)
		webSessions, err := m.fetchWebDashboardSessions()
		if err != nil {
			// Only log occasionally
			if errorCount%30 == 1 {
				log.Printf("⚠️  Error fetching session_signaling: %v", err)
			}
		}

		// Only handle the NEWEST unhandled session (avoid concurrent session storms)
		var newestSession *Session
		for i := range webSessions {
			if handledSessions[webSessions[i].ID] {
				continue
			}
			handledSessions[webSessions[i].ID] = true
			if newestSession == nil {
				newestSession = &webSessions[i]
			}
		}
		if newestSession != nil {
			if isConnected {
				// New dashboard session while connected — takeover
				log.Printf("📞 New session (dashboard) while connected — takeover: %s", newestSession.ID)
				m.cleanupConnection("New dashboard connection")
			}
			log.Printf("📞 Incoming session (web dashboard): %s", newestSession.ID)
			log.Printf("   Device ID: %s", m.device.ID)
			go m.handleWebSession(*newestSession)
		}
	}
}

// fetchWebDashboardSessions checks session_signaling for offers from web dashboard
func (m *Manager) fetchWebDashboardSessions() ([]Session, error) {
	url := m.cfg.SupabaseURL + "/rest/v1/session_signaling"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if err := m.setAuthHeaders(req); err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("msg_type", "eq.offer")
	q.Add("from_side", "eq.dashboard")
	q.Add("select", "*")
	q.Add("order", "created_at.desc")
	q.Add("limit", "10")
	req.URL.RawQuery = q.Encode()

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var signals []SignalMessage
	if err := json.NewDecoder(resp.Body).Decode(&signals); err != nil {
		return nil, fmt.Errorf("JSON decode failed: %w", err)
	}

	// Convert signals to sessions, filtering by device ID
	var result []Session
	for _, sig := range signals {
		// We need to check if this session is for our device
		// The session_signaling table doesn't have device_id directly,
		// so we need to look up the session in remote_sessions
		isForDevice, pin := m.checkSessionDevice(sig.SessionID)
		if !isForDevice {
			continue
		}

		// Parse offer payload
		var offerPayload OfferPayload
		if err := json.Unmarshal(sig.Payload, &offerPayload); err != nil {
			log.Printf("⚠️  Failed to parse offer payload: %v", err)
			continue
		}

		session := Session{
			ID:    sig.SessionID,
			Offer: offerPayload.SDP,
			PIN:   pin,
		}
		result = append(result, session)
	}

	return result, nil
}

// checkSessionDevice checks if a session belongs to this device
func (m *Manager) checkSessionDevice(sessionID string) (bool, string) {
	url := m.cfg.SupabaseURL + "/rest/v1/remote_sessions"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, ""
	}

	if err := m.setAuthHeaders(req); err != nil {
		log.Printf("⚠️ checkSessionDevice auth error: %v", err)
		return false, ""
	}

	q := req.URL.Query()
	q.Add("id", "eq."+sessionID) // Use 'id' not 'session_id'
	q.Add("device_id", "eq."+m.device.ID)
	q.Add("select", "id,pin,status")
	q.Add("limit", "1")
	req.URL.RawQuery = q.Encode()

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, ""
	}
	defer resp.Body.Close()

	var sessions []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
		return false, ""
	}

	if len(sessions) == 0 {
		return false, ""
	}

	// Check status - only handle pending/connecting sessions
	status, _ := sessions[0]["status"].(string)
	if status != "pending" && status != "connecting" {
		return false, ""
	}

	pin, _ := sessions[0]["pin"].(string)
	return true, pin
}

// handleWebSession handles a session from the web dashboard
func (m *Manager) handleWebSession(session Session) {
	// Skip if we're already handling this exact session (check sessionID first, peerConnection may not be set yet)
	if m.sessionID == session.ID {
		log.Printf("⏭️  Already handling session %s, skipping duplicate", session.ID)
		return
	}

	// Mark this session as being handled immediately to prevent race conditions
	m.sessionID = session.ID

	log.Println("🔧 Setting up WebRTC connection (web dashboard)...")

	// Cleanup previous connection if it's a different session
	if m.peerConnection != nil || m.isStreaming.Load() {
		log.Println("🔄 New connection requested - disconnecting previous session...")
		m.isStreaming.Store(false)
		if m.connCancel != nil {
			m.connCancel()
		}
		m.cleanupConnection("New connection from dashboard")
		time.Sleep(200 * time.Millisecond) // Give time for cleanup
		log.Println("✅ Previous session disconnected, ready for new connection")
	}

	// Reset ICE candidate buffer for new session
	m.pendingCandidates = nil
	m.answerSent = false

	if err := m.CreatePeerConnection(m.getICEServers()); err != nil {
		log.Printf("Failed to create peer connection: %v", err)
		return
	}

	if session.Offer == "" {
		log.Println("❌ No offer found in web session")
		return
	}

	// Set remote description (offer from dashboard)
	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  session.Offer,
	}

	// Log SDP diagnostics: check for data channel (m=application) in offer
	if strings.Contains(session.Offer, "m=application") {
		log.Println("✅ Offer SDP contains m=application (data channel/SCTP)")
	} else {
		log.Println("❌ Offer SDP is MISSING m=application - data channel will NOT work!")
		log.Printf("   Offer SDP (first 500 chars): %.500s", session.Offer)
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

	// Log answer SDP diagnostics
	if strings.Contains(answer.SDP, "m=application") {
		log.Println("✅ Answer SDP contains m=application (data channel/SCTP)")
	} else {
		log.Println("❌ Answer SDP is MISSING m=application!")
	}

	if err := m.peerConnection.SetLocalDescription(answer); err != nil {
		log.Printf("Failed to set local description: %v", err)
		return
	}

	// Send answer to session_signaling table (for web dashboard)
	m.sendAnswerToSignaling(session.ID, answer.SDP)

	// Continue listening for ICE candidates from dashboard
	go m.listenForICE(session.ID)
}

// sendAnswerToSignaling sends answer to session_signaling table for web dashboard
func (m *Manager) sendAnswerToSignaling(sessionID, sdp string) {
	url := m.cfg.SupabaseURL + "/rest/v1/session_signaling"

	payload := map[string]interface{}{
		"session_id": sessionID,
		"from_side":  "agent",
		"msg_type":   "answer",
		"payload": map[string]interface{}{
			"type": "answer",
			"sdp":  sdp,
		},
	}

	jsonData, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("❌ Failed to create answer request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if err := m.setAuthHeaders(req); err != nil {
		log.Printf("❌ Failed to set auth headers for answer: %v", err)
		return
	}
	req.Header.Set("Prefer", "return=minimal")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("❌ Failed to send answer to signaling: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("❌ Failed to send answer to signaling: HTTP %d - %s", resp.StatusCode, string(body))
		return
	}

	log.Println("📤 Sent answer to web dashboard")

	// Mark answer as sent and flush buffered ICE candidates
	m.answerSent = true
	if len(m.pendingCandidates) > 0 {
		log.Printf("📤 Sending %d buffered ICE candidates...", len(m.pendingCandidates))
		for _, candidate := range m.pendingCandidates {
			m.sendICECandidate(candidate)
		}
		m.pendingCandidates = nil
	}
}

func (m *Manager) fetchPendingSessions() ([]Session, error) {
	// Query webrtc_sessions for sessions with offers for this device
	url := m.cfg.SupabaseURL + "/rest/v1/webrtc_sessions"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if err := m.setAuthHeaders(req); err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("device_id", "eq."+m.device.ID)
	q.Add("offer", "not.is.null") // Has an offer waiting
	q.Add("answer", "is.null")    // No answer yet
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

	if err := m.CreatePeerConnection(m.getICEServers()); err != nil {
		log.Printf("Failed to create peer connection: %v", err)
		return
	}

	// Extract offer from session
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
				} else if sig.MsgType == "ice" {
					// Parse ICE payload - dashboard sends candidate directly in payload
					var icePayload ICEPayload
					if err := json.Unmarshal(sig.Payload, &icePayload); err != nil {
						log.Printf("⚠️  Failed to parse ICE payload: %v", err)
						continue
					}
					if icePayload.Candidate != "" {
						candidate := &webrtc.ICECandidateInit{
							Candidate:     icePayload.Candidate,
							SDPMid:        &icePayload.SDPMid,
							SDPMLineIndex: func() *uint16 { if icePayload.SDPMLineIndex != nil { v := uint16(*icePayload.SDPMLineIndex); return &v }; return nil }(),
						}
						m.handleICECandidate(candidate)
					}
				}
			}
		}
	}

	log.Println("🛑 Stopped signal polling - connection established")
}

func (m *Manager) handleOffer(sessionID string, sig SignalMessage) {
	// Parse offer payload
	var offerPayload OfferPayload
	if err := json.Unmarshal(sig.Payload, &offerPayload); err != nil {
		log.Printf("Failed to parse offer payload: %v", err)
		return
	}

	// Set remote description
	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  offerPayload.SDP,
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

	// Send answer - marshal the full SessionDescription to JSON
	answerJSON, err := json.Marshal(answer)
	if err != nil {
		log.Printf("Failed to marshal answer: %v", err)
		return
	}
	m.sendAnswer(sessionID, string(answerJSON))

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

			if sig.MsgType == "ice" {
				// Parse ICE payload
				var icePayload ICEPayload
				if err := json.Unmarshal(sig.Payload, &icePayload); err != nil {
					log.Printf("⚠️  Failed to parse ICE payload: %v", err)
					continue
				}
				if icePayload.Candidate != "" {
					candidate := &webrtc.ICECandidateInit{
						Candidate:     icePayload.Candidate,
						SDPMid:        &icePayload.SDPMid,
						SDPMLineIndex: func() *uint16 { if icePayload.SDPMLineIndex != nil { v := uint16(*icePayload.SDPMLineIndex); return &v }; return nil }(),
					}
					m.handleICECandidate(candidate)
				}
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

	// Log the candidate type for debugging
	candidateStr := candidate.Candidate
	candidateType := "unknown"
	if len(candidateStr) > 10 {
		// Parse candidate type from the candidate string
		if idx := len(candidateStr); idx > 0 {
			if contains(candidateStr, "typ relay") {
				candidateType = "relay (TURN)"
			} else if contains(candidateStr, "typ srflx") {
				candidateType = "srflx (STUN)"
			} else if contains(candidateStr, "typ host") {
				candidateType = "host (local)"
			} else if contains(candidateStr, "typ prflx") {
				candidateType = "prflx (peer reflexive)"
			}
		}
	}
	log.Printf("📥 Received ICE candidate from dashboard: %s", candidateType)

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

	if err := m.setAuthHeaders(req); err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("session_id", "eq."+sessionID)
	q.Add("from_side", "eq."+fromSide)
	q.Add("select", "*")
	q.Add("order", "created_at.asc") // Oldest first - get offer before ICE candidates
	q.Add("limit", "50")             // Increased to handle all ICE candidates
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

	// Send answer back to webrtc_sessions table - marshal to JSON
	answerJSON, err := json.Marshal(answer)
	if err != nil {
		log.Printf("Failed to marshal answer: %v", err)
		return
	}
	m.sendAnswer(sessionID, string(answerJSON))
}

func (m *Manager) sendAnswer(sessionID, sdp string) {
	url := m.cfg.SupabaseURL + "/rest/v1/webrtc_sessions"

	payload := map[string]interface{}{
		"answer": sdp,
		"status": "answered",
	}

	jsonData, _ := json.Marshal(payload)

	// Create PATCH request with session_id filter
	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("❌ Failed to create answer request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if err := m.setAuthHeaders(req); err != nil {
		log.Printf("❌ Failed to set auth headers for answer: %v", err)
		return
	}
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

	if err := m.setAuthHeaders(req); err != nil {
		log.Printf("Failed to set auth headers for session update: %v", err)
		return
	}
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

	// Build candidate object with required fields for browser compatibility
	// Browser requires non-empty sdpMid and valid sdpMLineIndex
	sdpMid := "0"
	sdpMLineIndex := uint16(0)

	// Use values from candidateInit if available and non-empty
	if candidateInit.SDPMid != nil && *candidateInit.SDPMid != "" {
		sdpMid = *candidateInit.SDPMid
	}
	if candidateInit.SDPMLineIndex != nil {
		sdpMLineIndex = *candidateInit.SDPMLineIndex
	}

	// Send ICE candidate directly in payload (same format as dashboard sends)
	// Dashboard expects: payload = { candidate: "...", sdpMid: "0", sdpMLineIndex: 0 }
	payload := map[string]interface{}{
		"session_id": m.sessionID,
		"from_side":  "agent",
		"msg_type":   "ice",
		"payload": map[string]interface{}{
			"candidate":     candidateInit.Candidate,
			"sdpMid":        sdpMid,
			"sdpMLineIndex": sdpMLineIndex,
		},
	}

	jsonData, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("❌ Failed to create ICE candidate request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if err := m.setAuthHeaders(req); err != nil {
		log.Printf("❌ Failed to set auth headers for ICE candidate: %v", err)
		return
	}
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

// listenForKickSignals listens for kick signals in the background
func (m *Manager) listenForKickSignals() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	log.Println("👂 Started kick signal listener")

	for range ticker.C {
		if m.sessionID == "" {
			continue
		}

		// Check for kick signals on current session
		signals, err := m.fetchKickSignals(m.sessionID)
		if err != nil {
			continue
		}

		for _, sig := range signals {
			if sig.MsgType == "kick" {
				log.Println("🔴 KICK SIGNAL RECEIVED - another controller took over")
				log.Printf("   Kick payload: %s", string(sig.Payload))
				m.cleanupConnection("Kicked by new controller")
				// Continue listening — don't exit, so we can handle future kicks
				break
			}
		}
	}
}

// fetchKickSignals fetches kick signals for a session
func (m *Manager) fetchKickSignals(sessionID string) ([]SignalMessage, error) {
	url := m.cfg.SupabaseURL + "/rest/v1/session_signaling"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if err := m.setAuthHeaders(req); err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("session_id", "eq."+sessionID)
	q.Add("msg_type", "eq.kick")
	q.Add("select", "*")
	q.Add("limit", "1")
	req.URL.RawQuery = q.Encode()

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var signals []SignalMessage
	if err := json.NewDecoder(resp.Body).Decode(&signals); err != nil {
		return nil, err
	}

	return signals, nil
}

// checkIfKicked checks if the current session has been kicked
func (m *Manager) checkIfKicked() bool {
	if m.sessionID == "" {
		return false
	}

	// Method 1: Check via RPC function (webrtc_sessions table)
	url := m.cfg.SupabaseURL + "/rest/v1/rpc/check_session_kicked"

	payload := map[string]string{
		"p_session_id": m.sessionID,
	}

	jsonData, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err == nil {
		req.Header.Set("Content-Type", "application/json")
		if err := m.setAuthHeaders(req); err == nil {
			client := &http.Client{Timeout: 3 * time.Second}
			resp, err := client.Do(req)
			if err == nil {
				defer resp.Body.Close()
				var result map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
					if kicked, _ := result["kicked"].(bool); kicked {
						return true
					}
				}
			}
		}
	}

	// Method 2: Check remote_sessions table for ended/kicked status
	rsURL := m.cfg.SupabaseURL + "/rest/v1/remote_sessions"
	rsReq, err := http.NewRequest("GET", rsURL, nil)
	if err != nil {
		return false
	}
	if err := m.setAuthHeaders(rsReq); err != nil {
		return false
	}
	q := rsReq.URL.Query()
	q.Add("id", "eq."+m.sessionID)
	q.Add("select", "status")
	q.Add("limit", "1")
	rsReq.URL.RawQuery = q.Encode()

	client := &http.Client{Timeout: 3 * time.Second}
	rsResp, err := client.Do(rsReq)
	if err != nil {
		return false
	}
	defer rsResp.Body.Close()

	var sessions []map[string]interface{}
	if err := json.NewDecoder(rsResp.Body).Decode(&sessions); err != nil {
		return false
	}

	if len(sessions) > 0 {
		status, _ := sessions[0]["status"].(string)
		if status == "ended" || status == "kicked" {
			return true
		}
	}

	return false
}
