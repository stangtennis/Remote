package webrtc

import (
	"fmt"
	"log"
	"time"
)

// enableSupportAuthorization installs the server-returned scope set before a
// support peer connection is created. WebRTC payloads never get to choose it.
func (m *Manager) enableSupportAuthorization(sessionID, grant string, scopes []string, expiresAt time.Time) {
	m.supportAuthMu.Lock()
	defer m.supportAuthMu.Unlock()
	m.supportMode = true
	m.supportSessionID = sessionID
	m.supportGrant = grant
	m.supportExpiresAt = expiresAt
	m.supportScopes = make(map[string]bool, len(scopes))
	for _, scope := range scopes {
		m.supportScopes[scope] = true
	}
	log.Printf("Support authorization enabled for %s with scopes=%v", sessionID, scopes)
}

func (m *Manager) supportAllows(scope string) bool {
	m.supportAuthMu.RLock()
	defer m.supportAuthMu.RUnlock()
	if !m.supportMode || m.supportGrant == "" {
		return false
	}
	if !m.supportExpiresAt.IsZero() && time.Now().After(m.supportExpiresAt) {
		return false
	}
	return m.supportScopes[scope]
}

func (m *Manager) supportIsActive() bool {
	m.supportAuthMu.RLock()
	defer m.supportAuthMu.RUnlock()
	return m.supportMode && m.supportGrant != ""
}

func (m *Manager) supportSession() (string, string) {
	m.supportAuthMu.RLock()
	defer m.supportAuthMu.RUnlock()
	return m.supportSessionID, m.supportGrant
}

func (m *Manager) setSupportAIOfferID(offerID string) {
	m.supportOfferMu.Lock()
	m.supportAIOfferID = offerID
	m.supportOfferMu.Unlock()
}

func (m *Manager) supportAIOfferIDValue() string {
	m.supportOfferMu.RLock()
	defer m.supportOfferMu.RUnlock()
	return m.supportAIOfferID
}

func (m *Manager) supportChannelAllowed(label string) bool {
	switch label {
	case "file":
		return m.supportAllows("files")
	case "terminal", "shell":
		return m.supportAllows("terminal")
	case "process":
		return m.supportAllows("process")
	case "control":
		return m.supportAllows("screen")
	default:
		return m.supportAllows("screen")
	}
}

func (m *Manager) recordSupportAction(actionType, status, summary, target string, details map[string]interface{}) error {
	if !m.supportIsActive() {
		return nil
	}
	_, err := m.supportRequest("record-action", map[string]interface{}{
		"action_type":    actionType,
		"action_status":  status,
		"action_summary": summary,
		"action_target":  target,
		"action_details": details,
	})
	if err != nil {
		return fmt.Errorf("support audit failed: %w", err)
	}
	return nil
}
