package webrtc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// SignalingClient handles WebRTC signaling via Supabase
type SignalingClient struct {
	supabaseURL string
	anonKey     string
	authToken   string
	httpClient  *http.Client
}

// Session represents a WebRTC session
type Session struct {
	SessionID string `json:"session_id"`
	DeviceID  string `json:"device_id"`
	UserID    string `json:"user_id"`
	Status    string `json:"status"`
	Offer     string `json:"offer,omitempty"`
	Answer    string `json:"answer,omitempty"`
}

// NewSignalingClient creates a new signaling client
func NewSignalingClient(supabaseURL, anonKey, authToken string) *SignalingClient {
	return &SignalingClient{
		supabaseURL: supabaseURL,
		anonKey:     anonKey,
		authToken:   authToken,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
	}
}

// CreateSession creates a new WebRTC session
func (s *SignalingClient) CreateSession(deviceID, userID string) (*Session, error) {
	sessionID := uuid.New().String()
	
	session := &Session{
		SessionID: sessionID,
		DeviceID:  deviceID,
		UserID:    userID,
		Status:    "pending",
	}

	url := fmt.Sprintf("%s/rest/v1/webrtc_sessions", s.supabaseURL)
	jsonData, err := json.Marshal(session)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal session: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", s.anonKey)
	req.Header.Set("Authorization", "Bearer "+s.authToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=representation")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create session: %s (status: %d)", string(body), resp.StatusCode)
	}

	return session, nil
}

// SendOffer sends the WebRTC offer to the session
func (s *SignalingClient) SendOffer(sessionID, offer string) error {
	url := fmt.Sprintf("%s/rest/v1/webrtc_sessions?session_id=eq.%s", s.supabaseURL, sessionID)
	
	// offer is already JSON-encoded, so use json.RawMessage to avoid double-encoding
	payload := map[string]interface{}{
		"offer":  json.RawMessage(offer),
		"status": "offer_sent",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", s.anonKey)
	req.Header.Set("Authorization", "Bearer "+s.authToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to send offer: %s (status: %d)", string(body), resp.StatusCode)
	}

	return nil
}

// WaitForAnswer polls for the WebRTC answer from the agent
func (s *SignalingClient) WaitForAnswer(sessionID string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		session, err := s.GetSession(sessionID)
		if err != nil {
			return "", err
		}

		if session.Answer != "" {
			return session.Answer, nil
		}

		<-ticker.C
	}

	return "", fmt.Errorf("timeout waiting for answer")
}

// GetSession retrieves a session from Supabase
func (s *SignalingClient) GetSession(sessionID string) (*Session, error) {
	url := fmt.Sprintf("%s/rest/v1/webrtc_sessions?session_id=eq.%s&select=*", s.supabaseURL, sessionID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", s.anonKey)
	req.Header.Set("Authorization", "Bearer "+s.authToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get session: %s (status: %d)", string(body), resp.StatusCode)
	}

	var sessions []Session
	if err := json.Unmarshal(body, &sessions); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(sessions) == 0 {
		return nil, fmt.Errorf("session not found")
	}

	return &sessions[0], nil
}

// DeleteSession deletes a session from Supabase
func (s *SignalingClient) DeleteSession(sessionID string) error {
	url := fmt.Sprintf("%s/rest/v1/webrtc_sessions?session_id=eq.%s", s.supabaseURL, sessionID)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", s.anonKey)
	req.Header.Set("Authorization", "Bearer "+s.authToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete session: %s (status: %d)", string(body), resp.StatusCode)
	}

	return nil
}
