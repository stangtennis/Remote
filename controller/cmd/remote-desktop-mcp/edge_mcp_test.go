package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEdgeMCPClientCallToolReturnsCentralResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("authorization header was not forwarded")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"client_type\":\"ai_support\"}"}]}}`))
	}))
	defer server.Close()

	client := &edgeMCPClient{endpoint: server.URL, anonKey: "anon", httpClient: server.Client()}
	text, status, err := client.callWithToken(context.Background(), "test-token", "support_context", map[string]any{"client_id": "ai-test"})
	if err != nil {
		t.Fatalf("callWithToken returned error: %v", err)
	}
	if status != http.StatusOK || text != `{"client_type":"ai_support"}` {
		t.Fatalf("unexpected MCP result: status=%d text=%q", status, text)
	}
}

func TestEdgeMCPClientCallToolRejectsMCPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32003,"message":"Approved user access required"}}`))
	}))
	defer server.Close()

	client := &edgeMCPClient{endpoint: server.URL, anonKey: "anon", httpClient: server.Client()}
	_, _, err := client.callWithToken(context.Background(), "test-token", "knowledge_search", map[string]any{"query": "network"})
	if err == nil {
		t.Fatal("expected MCP error")
	}
}
