package webrtc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	pionwebrtc "github.com/pion/webrtc/v3"
	"github.com/stangtennis/remote-agent/internal/config"
	"github.com/stangtennis/remote-agent/internal/device"
)

// SupportConsentFunc is called after the admin code is accepted and before
// any native control channel is enabled.
type SupportConsentFunc func(scopes []string) bool

// SupportStatusFunc reports portable support progress to the visible client UI.
type SupportStatusFunc func(message string)

type supportSessionResponse struct {
	SessionID       string   `json:"session_id"`
	Token           string   `json:"token"`
	ClientGrant     string   `json:"client_grant_token"`
	SupportMode     string   `json:"support_mode"`
	RequestedScopes []string `json:"requested_scopes"`
	RequiresConsent bool     `json:"requires_consent"`
	ExpiresAt       string   `json:"expires_at"`
}

// RunPortableSupport runs a temporary, non-installed support agent. It never
// registers a device, creates a service, or opens a listener.
func RunPortableSupport(cfg *config.Config, dev *device.Device, pin string, consent SupportConsentFunc) error {
	return RunPortableSupportWithCallbacks(cfg, dev, pin, consent, nil, nil)
}

// RunPortableSupportWithCallbacks runs portable support with user-visible
// status updates and an optional stop channel controlled by the support window.
func RunPortableSupportWithCallbacks(cfg *config.Config, dev *device.Device, pin string, consent SupportConsentFunc, status SupportStatusFunc, stop <-chan struct{}) error {
	setStatus := func(message string) {
		if status != nil {
			status(message)
		}
	}
	if strings.TrimSpace(pin) == "" {
		return fmt.Errorf("support PIN is required")
	}

	setStatus("Validerer PIN...")
	validated, err := supportRequest(cfg, map[string]interface{}{
		"action": "validate",
		"pin":    strings.TrimSpace(pin),
	})
	if err != nil {
		return err
	}
	if supportStopped(stop) {
		return fmt.Errorf("support session ended by user")
	}

	var session supportSessionResponse
	if err := json.Unmarshal(validated, &session); err != nil {
		return fmt.Errorf("invalid support validation response: %w", err)
	}
	grant := session.ClientGrant
	if grant == "" {
		grant = session.Token
	}
	if session.SessionID == "" || grant == "" {
		return fmt.Errorf("support validation returned no usable session grant")
	}
	effectiveScopes := session.RequestedScopes
	if session.SupportMode == "ai" {
		setStatus("PIN accepteret. Venter på samtykke...")
		if consent == nil || !consent(session.RequestedScopes) {
			_, _ = supportRequest(cfg, map[string]interface{}{
				"action":             "end",
				"client_grant_token": grant,
			})
			return fmt.Errorf("support consent was declined")
		}
		consentData, err := supportRequest(cfg, map[string]interface{}{
			"action":             "consent",
			"client_grant_token": grant,
			"approved":           true,
			"scopes":             session.RequestedScopes,
			"client_label":       "Portable Remote Support",
		})
		if err != nil {
			return err
		}
		var consentResult struct {
			Scopes []string `json:"scopes"`
		}
		if err := json.Unmarshal(consentData, &consentResult); err != nil || len(consentResult.Scopes) == 0 {
			return fmt.Errorf("consent response did not contain effective scopes")
		}
		effectiveScopes = consentResult.Scopes
	}
	if supportStopped(stop) {
		return fmt.Errorf("support session ended by user")
	}

	setStatus("Samtykke godkendt. Henter TURN-forbindelse...")
	m, err := New(cfg, dev, nil)
	if err != nil {
		return err
	}
	expiresAt, err := time.Parse(time.RFC3339, session.ExpiresAt)
	if err != nil {
		return fmt.Errorf("invalid support expiry: %w", err)
	}
	m.enableSupportAuthorization(session.SessionID, grant, effectiveScopes, expiresAt)
	m.sessionID = session.SessionID
	defer func() {
		if m.peerConnection != nil || m.supportViewerPresent() {
			m.cleanupConnection("portable support ended")
		} else {
			_, _ = m.supportRequest("end", nil)
		}
	}()

	turnData, err := m.supportRequest("turn", nil)
	if err != nil {
		return err
	}
	setStatus("TURN-forbindelse klar. Venter på AI-forbindelse...")
	iceServers, err := decodeSupportICEServers(turnData)
	if err != nil {
		return err
	}
	if err := m.CreatePeerConnection(iceServers); err != nil {
		return err
	}
	m.supportAIOfferID = ""
	if _, err := m.supportRequest("ready", nil); err != nil {
		m.cleanupConnection("support ready failed")
		return err
	}

	log.Printf("Portable support waiting for admin offer: session=%s", session.SessionID)
	setStatus("Support aktiv. Venter på forbindelse fra administrator...")
	return m.pollPortableSupport(stop, setStatus, iceServers)
}

func supportStopped(stop <-chan struct{}) bool {
	if stop == nil {
		return false
	}
	select {
	case <-stop:
		return true
	default:
		return false
	}
}

func supportRequest(cfg *config.Config, body map[string]interface{}) ([]byte, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, cfg.SupabaseURL+"/functions/v1/support-signal", bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", cfg.SupabaseAnonKey)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("support request failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return data, nil
}

func (m *Manager) supportRequest(action string, extra map[string]interface{}) ([]byte, error) {
	_, grant := m.supportSession()
	body := map[string]interface{}{
		"action":             action,
		"client_grant_token": grant,
	}
	for key, value := range extra {
		body[key] = value
	}
	return supportRequest(m.cfg, body)
}

func (m *Manager) supportWriteSignal(signalType string, payload interface{}) error {
	_, grant := m.supportSession()
	_, err := m.supportRequest("signal-write", map[string]interface{}{
		"client_grant_token": grant,
		"signal_type":        signalType,
		"signal_payload":     payload,
	})
	return err
}

// Legacy portable AI signaling did not identify its peer. Treat only missing
// peer_id as AI; viewer messages must opt into the explicit viewer route.
func supportSignalRouting(payload json.RawMessage) (peerID, offerID string) {
	var route struct {
		PeerID  string `json:"peer_id"`
		OfferID string `json:"offer_id"`
	}
	if json.Unmarshal(payload, &route) != nil || route.PeerID == "" {
		return "ai", route.OfferID
	}
	return route.PeerID, route.OfferID
}

func decodeSupportICEServers(data []byte) ([]pionwebrtc.ICEServer, error) {
	var raw struct {
		ICEServers []struct {
			URLs       interface{} `json:"urls"`
			Username   string      `json:"username"`
			Credential string      `json:"credential"`
		} `json:"iceServers"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid TURN response: %w", err)
	}
	servers := make([]pionwebrtc.ICEServer, 0, len(raw.ICEServers))
	for _, item := range raw.ICEServers {
		urls := make([]string, 0, 2)
		switch value := item.URLs.(type) {
		case string:
			urls = append(urls, value)
		case []interface{}:
			for _, url := range value {
				if text, ok := url.(string); ok {
					urls = append(urls, text)
				}
			}
		}
		if len(urls) == 0 {
			continue
		}
		servers = append(servers, pionwebrtc.ICEServer{
			URLs:       urls,
			Username:   item.Username,
			Credential: item.Credential,
		})
	}
	if len(servers) == 0 {
		return nil, fmt.Errorf("support TURN response contained no ICE servers")
	}
	return servers, nil
}

func (m *Manager) pollPortableSupport(stop <-chan struct{}, status SupportStatusFunc, iceServers []pionwebrtc.ICEServer) error {
	processed := make(map[int]bool)
	var pendingICE []pionwebrtc.ICECandidateInit
	remoteDescriptionSet := false
	offerHandled := false
	aiOfferID := ""
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return fmt.Errorf("support session ended by user")
		case <-ticker.C:
			if m.peerConnection == nil {
				return nil
			}
			state := m.peerConnection.ConnectionState()
			if state == pionwebrtc.PeerConnectionStateFailed || state == pionwebrtc.PeerConnectionStateClosed {
				return nil
			}
			data, err := m.supportRequest("signal-read", nil)
			if err != nil {
				if strings.Contains(err.Error(), "(400)") || strings.Contains(err.Error(), "(401)") || strings.Contains(err.Error(), "(403)") || strings.Contains(err.Error(), "(404)") {
					return fmt.Errorf("support session closed or revoked: %w", err)
				}
				log.Printf("Support signaling poll failed: %v", err)
				continue
			}
			var result struct {
				Signals []SignalMessage `json:"signals"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				continue
			}
			var latestOffer *SignalMessage
			for _, signal := range result.Signals {
				if processed[signal.ID] {
					continue
				}
				processed[signal.ID] = true
				peerID, offerID := supportSignalRouting(signal.Payload)
				switch signal.MsgType {
				case "bye":
					if peerID == "ai" {
						return nil
					}
				case "offer":
					if peerID == supportViewerPeerID {
						if err := m.handleSupportViewerOffer(signal, iceServers); err != nil {
							log.Printf("Support viewer offer failed: %v", err)
						}
						continue
					}
					if peerID != "ai" {
						continue
					}
					candidate := signal
					latestOffer = &candidate
				case "ice":
					if peerID == supportViewerPeerID {
						m.handleSupportViewerICE(signal)
						continue
					}
					if peerID != "ai" || (aiOfferID != "" && offerID != "" && offerID != aiOfferID) {
						continue
					}
					var candidate ICEPayload
					if json.Unmarshal(signal.Payload, &candidate) != nil || candidate.Candidate == "" {
						continue
					}
					init := pionwebrtc.ICECandidateInit{Candidate: candidate.Candidate, SDPMid: &candidate.SDPMid}
					if candidate.SDPMLineIndex != nil {
						index := uint16(*candidate.SDPMLineIndex)
						init.SDPMLineIndex = &index
					}
					if !remoteDescriptionSet {
						pendingICE = append(pendingICE, init)
						continue
					}
					_ = m.peerConnection.AddICECandidate(init)
				}
			}
			if latestOffer != nil && !offerHandled {
				if status != nil {
					status("Forbinder til administrator...")
				}
				if err := m.handlePortableSupportOffer(*latestOffer, &remoteDescriptionSet, &pendingICE); err != nil {
					return err
				}
				var offerPayload struct {
					OfferID string `json:"offer_id"`
				}
				_ = json.Unmarshal(latestOffer.Payload, &offerPayload)
				aiOfferID = offerPayload.OfferID
				offerHandled = true
				if status != nil {
					status("AI-support er forbundet.")
				}
			}
		}
	}
}

func (m *Manager) handlePortableSupportOffer(signal SignalMessage, remoteSet *bool, pending *[]pionwebrtc.ICECandidateInit) error {
	var offerPayload struct {
		Type    string `json:"type"`
		SDP     string `json:"sdp"`
		PeerID  string `json:"peer_id"`
		OfferID string `json:"offer_id"`
	}
	if err := json.Unmarshal(signal.Payload, &offerPayload); err != nil {
		return err
	}
	if offerPayload.PeerID != "" && offerPayload.PeerID != "ai" {
		return fmt.Errorf("support offer is for another peer")
	}
	if strings.TrimSpace(offerPayload.OfferID) == "" {
		return fmt.Errorf("support offer is missing offer_id")
	}
	m.supportAIOfferID = offerPayload.OfferID
	if err := m.peerConnection.SetRemoteDescription(pionwebrtc.SessionDescription{
		Type: pionwebrtc.SDPTypeOffer,
		SDP:  offerPayload.SDP,
	}); err != nil {
		return err
	}
	*remoteSet = true
	for _, candidate := range *pending {
		_ = m.peerConnection.AddICECandidate(candidate)
	}
	*pending = nil
	answer, err := m.peerConnection.CreateAnswer(nil)
	if err != nil {
		return err
	}
	if err := m.peerConnection.SetLocalDescription(answer); err != nil {
		return err
	}
	if err := m.supportWriteSignal("answer", map[string]interface{}{
		"type":     "answer",
		"sdp":      answer.SDP,
		"peer_id":  "ai",
		"offer_id": offerPayload.OfferID,
	}); err != nil {
		return err
	}
	m.answerSent = true
	for _, candidate := range m.pendingCandidates {
		m.sendICECandidate(candidate)
	}
	m.pendingCandidates = nil
	return nil
}
