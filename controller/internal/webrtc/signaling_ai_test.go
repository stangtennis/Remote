package webrtc

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateAISessionRequiresKey(t *testing.T) {
	client := NewSignalingClient("https://example.test", "anon", "token")
	if _, err := client.CreateAISession("https://example.test", "device-1", "user-1", ""); err == nil {
		t.Fatal("expected missing AI key error")
	}
}

func TestCreateAISessionUsesTrustedEdgeFunction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/functions/v1/ai-connect" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("x-ai-controller-key"); got != "secret" {
			t.Fatalf("AI key = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Fatalf("authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"session_id":"session-1","device_id":"device-1"}`))
	}))
	defer server.Close()

	client := NewSignalingClient(server.URL, "anon", "token")
	session, err := client.CreateAISession(server.URL, "device-1", "user-1", "secret")
	if err != nil {
		t.Fatalf("CreateAISession() error = %v", err)
	}
	if session.SessionID != "session-1" || session.DeviceID != "device-1" {
		t.Fatalf("unexpected session: %#v", session)
	}
}
