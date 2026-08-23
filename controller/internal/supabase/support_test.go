package supabase

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testSupportClient(serverURL string) *Client {
	client := NewClient(serverURL, "anon-key")
	client.AuthToken = "access-token"
	client.tokenExpiry = time.Now().Add(time.Hour)
	return client
}

func TestCreateAISupportSessionSendsModeAndScopes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/functions/v1/create-support-session" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer access-token" {
			t.Fatalf("unexpected authorization header: %s", got)
		}
		var payload struct {
			SupportMode     string   `json:"support_mode"`
			RequestedScopes []string `json:"requested_scopes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload.SupportMode != "ai" {
			t.Fatalf("support_mode = %q, want ai", payload.SupportMode)
		}
		if len(payload.RequestedScopes) != 3 || payload.RequestedScopes[0] != "screen" || payload.RequestedScopes[1] != "input" || payload.RequestedScopes[2] != "terminal" {
			t.Fatalf("requested_scopes = %#v", payload.RequestedScopes)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"session_id":"session-1","pin":"123456","share_url":"https://example.test/support.html?token=secret","expires_at":"2026-08-22T12:00:00Z","support_mode":"ai","requested_scopes":["screen","input","terminal"],"requires_client_code":true}`))
	}))
	defer server.Close()

	session, err := testSupportClient(server.URL).CreateAISupportSession([]string{"screen", "input", "terminal"})
	if err != nil {
		t.Fatalf("CreateAISupportSession() error = %v", err)
	}
	if session.SupportMode != "ai" || !session.RequiresClientCode {
		t.Fatalf("unexpected session response: %#v", session)
	}
}

func TestRequestAIControllerSendsAuthenticatedRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/functions/v1/support-signal" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload["action"] != "request-controller" || payload["session_id"] != "session-1" {
			t.Fatalf("unexpected payload: %#v", payload)
		}
		if r.Header.Get("Authorization") != "Bearer access-token" {
			t.Fatalf("missing authenticated request")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"requested":true}`))
	}))
	defer server.Close()

	if err := testSupportClient(server.URL).RequestAIController("session-1"); err != nil {
		t.Fatalf("RequestAIController() error = %v", err)
	}
}

func TestGetAndRevokeAISupportSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			if r.URL.Path != "/rest/v1/support_sessions" || r.URL.Query().Get("id") != "eq.session-1" {
				t.Fatalf("unexpected state request: %s?%s", r.URL.Path, r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"status":"active","controller_requested":true,"controller_claimed_by":"ubuntu-1"}]`))
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/functions/v1/support-signal" {
			var payload map[string]string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode revoke request: %v", err)
			}
			if payload["action"] != "revoke" || payload["session_id"] != "session-1" {
				t.Fatalf("unexpected revoke payload: %#v", payload)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	client := testSupportClient(server.URL)
	state, err := client.GetAISupportSessionState("session-1")
	if err != nil {
		t.Fatalf("GetAISupportSessionState() error = %v", err)
	}
	if state.Status != "active" || state.ControllerClaimedBy != "ubuntu-1" || !state.ControllerRequested {
		t.Fatalf("unexpected state: %#v", state)
	}
	if err := client.RevokeSupportSession("session-1", "test"); err != nil {
		t.Fatalf("RevokeSupportSession() error = %v", err)
	}
}
