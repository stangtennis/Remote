//go:build linux

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// fakeProc creates a /proc-like tree. procs maps pid -> argv (the process is
// considered matching when argv contains remote-desktop-cli support-watch).
func fakeProc(t *testing.T, procs map[int][]string) string {
	t.Helper()
	root := t.TempDir()
	for pid, argv := range procs {
		dir := filepath.Join(root, strconv.Itoa(pid))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		cmdline := strings.Join(argv, "\x00") + "\x00"
		if err := os.WriteFile(filepath.Join(dir, "cmdline"), []byte(cmdline), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestFindSupportWatchPIDs(t *testing.T) {
	selfPid := os.Getpid()
	root := fakeProc(t, map[int][]string{
		101:     {"/home/user/.local/bin/remote-desktop-cli", "support-watch"},
		102:     {"/opt/remote-desktop-cli", "support-watch"},
		103:     {"bash", "-c", "echo remote-desktop-cli support-watch"}, // text match, not argv match
		104:     {"remote-desktop-cli", "list"},
		selfPid: {"/usr/bin/remote-desktop-cli", "support-watch"}, // own pid must be skipped
	})

	pids, err := findSupportWatchPIDs(root, os.Getuid())
	if err != nil {
		t.Fatal(err)
	}
	want := []int{101, 102}
	if fmt.Sprint(pids) != fmt.Sprint(want) {
		t.Fatalf("pids = %v, want %v", pids, want)
	}
}

func TestFindSupportWatchPIDsNoneRunning(t *testing.T) {
	root := fakeProc(t, map[int][]string{
		201: {"remote-desktop-cli", "status"},
		202: {"some-other-daemon", "--support-watch"},
	})
	pids, err := findSupportWatchPIDs(root, os.Getuid())
	if err != nil {
		t.Fatal(err)
	}
	if len(pids) != 0 {
		t.Fatalf("expected no matches, got %v", pids)
	}
}

func TestFindSupportWatchPIDsBadRoot(t *testing.T) {
	if _, err := findSupportWatchPIDs(filepath.Join(t.TempDir(), "missing"), os.Getuid()); err == nil {
		t.Fatal("expected error for missing /proc root")
	}
}

func TestTailLog(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "watch.log")

	var b strings.Builder
	for i := 1; i <= 100; i++ {
		fmt.Fprintf(&b, "line %03d\n", i)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}

	tail, err := tailLog(path, 10, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(tail, "line 100") || strings.Contains(tail, "line 089") {
		t.Fatalf("tail should contain the last 10 lines only: %q", tail)
	}

	// Byte cap smaller than the content.
	tail, err = tailLog(path, 1000, 60)
	if err != nil {
		t.Fatal(err)
	}
	if len(tail) > 60+len("\n...[output truncated]") {
		t.Fatalf("tail not byte-bounded: %d bytes", len(tail))
	}
}

func TestTailLogMissingFile(t *testing.T) {
	if _, err := tailLog(filepath.Join(t.TempDir(), "absent.log"), 10, 1024); err == nil {
		t.Fatal("expected error for missing log file")
	}
}

func TestSupportWatchStatusTextIncludesLogTail(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "support-watch.log")
	if err := os.WriteFile(logPath, []byte("Watching dashboard for active AI support sessions...\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RD_WATCH_LOG", logPath)

	text := supportWatchStatusText()
	if !strings.Contains(text, "Support watcher:") {
		t.Fatalf("status text should report watcher state: %q", text)
	}
	if !strings.Contains(text, logPath) || !strings.Contains(text, "Watching dashboard") {
		t.Fatalf("status text should include bounded log tail: %q", text)
	}
}

func TestSupportWatchStatusRedactsSecretsFromLog(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "support-watch.log")
	if err := os.WriteFile(logPath, []byte("RD_PASSWORD=not-for-mcp token=hidden\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RD_WATCH_LOG", logPath)

	text := supportWatchStatusText()
	for _, secret := range []string{"not-for-mcp", "hidden"} {
		if strings.Contains(text, secret) {
			t.Fatalf("watch log leaked secret %q: %q", secret, text)
		}
	}
}

func TestSupportWatchStatusTextMissingLog(t *testing.T) {
	t.Setenv("RD_WATCH_LOG", filepath.Join(t.TempDir(), "absent.log"))
	text := supportWatchStatusText()
	if !strings.Contains(text, "Support watcher:") {
		t.Fatalf("status text should report watcher state: %q", text)
	}
	if !strings.Contains(text, "absent.log") {
		t.Fatalf("status text should mention unreadable log path: %q", text)
	}
}
