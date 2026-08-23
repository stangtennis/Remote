package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/stangtennis/Remote/controller/internal/config"
)

type activeSupportSession struct {
	ID                  string    `json:"id"`
	PIN                 string    `json:"pin"`
	Status              string    `json:"status"`
	Mode                string    `json:"support_mode"`
	CreatedAt           time.Time `json:"created_at"`
	ExpiresAt           time.Time `json:"expires_at"`
	ControllerRequested bool      `json:"controller_requested"`
	ControllerClaimedBy string    `json:"controller_claimed_by"`
}

func supportControllerID() string {
	if configured := os.Getenv("RD_CONTROLLER_ID"); configured != "" {
		return configured
	}
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "ubuntu"
	}
	user := os.Getenv("USER")
	if user == "" {
		user = os.Getenv("USERNAME")
	}
	if user == "" {
		user = "controller"
	}
	return fmt.Sprintf("%s-%s", host, user)
}

func updateSupportControllerClaim(cfg *config.Config, auth *authInfo, sessionID, controllerID, action string) (bool, error) {
	body := map[string]string{
		"action":        action,
		"session_id":    sessionID,
		"controller_id": controllerID,
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return false, err
	}
	req, err := http.NewRequest(http.MethodPost, cfg.SupabaseURL+"/functions/v1/support-signal", bytes.NewReader(encoded))
	if err != nil {
		return false, err
	}
	req.Header.Set("apikey", cfg.SupabaseAnonKey)
	req.Header.Set("Authorization", "Bearer "+auth.GetToken())
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("controller claim failed (HTTP %d): %s", resp.StatusCode, string(body))
	}
	var result struct {
		Claimed bool `json:"claimed"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}
	return result.Claimed || action == "release-controller", nil
}

func fetchOwnedAISupportSessions(cfg *config.Config, auth *authInfo, statuses string) ([]activeSupportSession, error) {
	query := url.Values{}
	query.Set("select", "id,pin,status,support_mode,created_at,expires_at,controller_requested,controller_claimed_by")
	query.Set("created_by", "eq."+auth.userID)
	query.Set("support_mode", "eq.ai")
	query.Set("status", statuses)
	query.Set("order", "created_at.desc")
	query.Set("limit", "20")

	req, err := http.NewRequest(http.MethodGet, cfg.SupabaseURL+"/rest/v1/support_sessions?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", cfg.SupabaseAnonKey)
	req.Header.Set("Authorization", "Bearer "+auth.GetToken())
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("support session lookup failed (HTTP %d)", resp.StatusCode)
	}

	var sessions []activeSupportSession
	if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
		return nil, fmt.Errorf("invalid support session response: %w", err)
	}
	return sessions, nil
}

// fetchActiveSupportSessions reads only sessions owned by the authenticated
// admin. RLS remains the authority; the watcher never uses the service key.
func fetchActiveSupportSessions(cfg *config.Config, auth *authInfo) ([]activeSupportSession, error) {
	sessions, err := fetchOwnedAISupportSessions(cfg, auth, "eq.active")
	if err != nil {
		return nil, err
	}
	requested := sessions[:0]
	for _, session := range sessions {
		if session.ControllerRequested {
			requested = append(requested, session)
		}
	}
	return requested, nil
}

func resolveSupportSessionKey(cfg *config.Config, auth *authInfo, key string) (string, error) {
	query := url.Values{}
	query.Set("select", "id,pin,status,support_mode,expires_at")
	query.Set("created_by", "eq."+auth.userID)
	query.Set("support_mode", "eq.ai")
	query.Set("status", "in.(pending,active)")
	if len(key) == 6 {
		query.Set("pin", "eq."+key)
	} else {
		query.Set("id", "eq."+key)
	}
	query.Set("limit", "2")

	req, err := http.NewRequest(http.MethodGet, cfg.SupabaseURL+"/rest/v1/support_sessions?"+query.Encode(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("apikey", cfg.SupabaseAnonKey)
	req.Header.Set("Authorization", "Bearer "+auth.GetToken())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return "", fmt.Errorf("support key lookup failed (HTTP %d)", resp.StatusCode)
	}
	var sessions []activeSupportSession
	if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
		return "", err
	}
	if len(sessions) == 0 {
		return "", fmt.Errorf("no active AI support session matches key %q", key)
	}
	if len(sessions) > 1 {
		return "", fmt.Errorf("client key %q is ambiguous; use the full session ID", key)
	}
	return sessions[0].ID, nil
}
