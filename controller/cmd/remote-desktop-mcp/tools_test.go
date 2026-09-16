package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// forbiddenTools must never be registered by this support adapter.
var forbiddenTools = []string{
	"click", "type", "press_key", "key", "scroll", "exec",
	"upload", "download", "ssh", "connect", "disconnect",
	"screenshot", "ps", "kill", "sysinfo", "support_connect",
}

func TestServerRegistersExpectedTools(t *testing.T) {
	s := newServer(newCLIRunner(writeFakeCLI(t, "true")))

	listed := s.ListTools()
	if len(listed) != len(toolAllowList) {
		t.Fatalf("registered %d tools, want %d: %v", len(listed), len(toolAllowList), listed)
	}
	for _, name := range toolAllowList {
		tool, ok := listed[name]
		if !ok {
			t.Fatalf("tool %q not registered", name)
		}
		if tool.Tool.Description == "" {
			t.Errorf("tool %q should carry a description", name)
		}
		writeTool := name == toolKnowledgeDraft || name == toolKnowledgePublish
		if tool.Tool.Annotations.ReadOnlyHint == nil || *tool.Tool.Annotations.ReadOnlyHint == writeTool {
			t.Errorf("tool %q has incorrect read-only annotation", name)
		}
		if tool.Tool.Annotations.DestructiveHint == nil || *tool.Tool.Annotations.DestructiveHint {
			t.Errorf("tool %q must not be marked destructive", name)
		}
		if tool.Tool.Annotations.IdempotentHint == nil || *tool.Tool.Annotations.IdempotentHint == writeTool {
			t.Errorf("tool %q has incorrect idempotent annotation", name)
		}
	}
	for _, name := range forbiddenTools {
		if _, ok := listed[name]; ok {
			t.Errorf("support adapter must not register %q", name)
		}
	}
}

func TestCLISubcommandAllowListCoversRegisteredTools(t *testing.T) {
	// Every CLI-facing tool must map to an allow-listed subcommand; the
	// runner must reject everything else.
	for _, sub := range []string{"list", "status", "support-list"} {
		if !cliSubcommands[sub] {
			t.Errorf("subcommand %q used by tools but missing from allow-list", sub)
		}
	}
	if cliSubcommands["support-watch"] {
		t.Error("adapter must never start the watcher via CLI")
	}
}

func callTool(t *testing.T, name string, run *cliRunner) *mcp.CallToolResult {
	t.Helper()
	listed := newServer(run).ListTools()
	tool, ok := listed[name]
	if !ok {
		t.Fatalf("tool %q not registered", name)
	}
	res, err := tool.Handler(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handler returned transport error (must be a tool result): %v", err)
	}
	return res
}

func resultText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if len(res.Content) != 1 {
		t.Fatalf("expected exactly one content block, got %d", len(res.Content))
	}
	tc, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}
	return tc.Text
}

func TestStatusToolReportsDaemonAbsentAsText(t *testing.T) {
	cli := writeFakeCLI(t, `echo "Not connected (daemon not running)"`)
	res := callTool(t, toolStatus, newCLIRunner(cli))
	if res.IsError {
		t.Fatalf("daemon-absent status must be text, not tool error: %+v", res)
	}
	if !strings.Contains(resultText(t, res), "Not connected") {
		t.Fatal("status text missing daemon-absent message")
	}
}

func TestStatusToolSurfacesCLIErrorsWithoutPanicking(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-cli")
	res := callTool(t, toolStatus, newCLIRunner(missing))
	if !res.IsError {
		t.Fatalf("missing CLI must produce a tool error, got text %q", resultText(t, res))
	}
	if !strings.Contains(resultText(t, res), missing) {
		t.Fatalf("tool error should name the CLI path: %q", resultText(t, res))
	}
}

func TestListDevicesToolReturnsStructuredJSON(t *testing.T) {
	cli := writeFakeCLI(t, `echo "SERVER (windows) online  ID: dev-abc123"`)
	res := callTool(t, toolListDevices, newCLIRunner(cli))
	if res.IsError {
		t.Fatalf("unexpected tool error: %q", resultText(t, res))
	}
	var entries []deviceEntry
	if err := json.Unmarshal([]byte(resultText(t, res)), &entries); err != nil {
		t.Fatalf("expected JSON output, got %q: %v", resultText(t, res), err)
	}
	if len(entries) != 1 || entries[0].DeviceID != "dev-abc123" || entries[0].Status != "online" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}

func TestListDevicesToolFallsBackToBoundedText(t *testing.T) {
	cli := writeFakeCLI(t, `echo "No devices registered."`)
	res := callTool(t, toolListDevices, newCLIRunner(cli))
	if res.IsError {
		t.Fatalf("unexpected tool error: %q", resultText(t, res))
	}
	if strings.TrimSpace(resultText(t, res)) != "No devices registered." {
		t.Fatalf("fallback text mismatch: %q", resultText(t, res))
	}
}

func TestSupportListToolReturnsStructuredJSON(t *testing.T) {
	cli := writeFakeCLI(t, `echo "Key 123456  active expires 14:22  session sess-1"`)
	res := callTool(t, toolSupportList, newCLIRunner(cli))
	if res.IsError {
		t.Fatalf("unexpected tool error: %q", resultText(t, res))
	}
	var entries []supportSessionEntry
	if err := json.Unmarshal([]byte(resultText(t, res)), &entries); err != nil {
		t.Fatalf("expected JSON output, got %q: %v", resultText(t, res), err)
	}
	if len(entries) != 1 || entries[0].SessionID != "sess-1" || entries[0].Key != "123456" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}

func TestSupportWatchStatusToolNeverInvokesCLI(t *testing.T) {
	record := filepath.Join(t.TempDir(), "must-not-exist")
	t.Setenv("FAKE_CLI_RECORD", record)
	cli := writeFakeCLI(t, `printf '%s\n' "$@" > "$FAKE_CLI_RECORD"`)
	t.Setenv("RD_WATCH_LOG", filepath.Join(t.TempDir(), "absent.log"))

	res := callTool(t, toolSupportWatchStatus, newCLIRunner(cli))
	if res.IsError {
		t.Fatalf("unexpected tool error: %q", resultText(t, res))
	}
	text := resultText(t, res)
	if !strings.Contains(text, "Support watcher:") {
		t.Fatalf("missing watcher state: %q", text)
	}
	if _, err := os.Stat(record); err == nil {
		t.Fatal("support_watch_status must not execute the CLI")
	}
}
