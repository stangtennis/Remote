package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	watchLogMaxLines = 30
	watchLogMaxBytes = 4 << 10
)

// watchLogPath resolves the support-watch log location. The opencode
// launchers append the watcher's output to
// $XDG_STATE_HOME/remote-desktop/support-watch.log, defaulting to
// ~/.local/state/remote-desktop/support-watch.log. RD_WATCH_LOG can override
// it for non-standard setups.
func watchLogPath() string {
	if p := os.Getenv("RD_WATCH_LOG"); p != "" {
		return p
	}
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, "remote-desktop", "support-watch.log")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "state", "remote-desktop", "support-watch.log")
}

// tailLog returns the last maxLines lines (and at most maxBytes) of path.
// It never starts, stops, or signals anything — it only reads.
func tailLog(path string, maxLines, maxBytes int) (string, error) {
	if path == "" {
		return "", fmt.Errorf("no log path available")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if info.Size() > int64(maxBytes) {
		if _, err := f.Seek(info.Size()-int64(maxBytes), io.SeekStart); err != nil {
			return "", err
		}
	}
	data, err := io.ReadAll(io.LimitReader(f, int64(maxBytes)+1))
	if err != nil {
		return "", err
	}

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	out := boundText(strings.Join(lines, "\n"), maxBytes)
	if out == "" {
		out = "(log is empty)"
	}
	return out, nil
}

// supportWatchStatusText reports watcher state read-only: whether a
// same-user `remote-desktop-cli support-watch` process exists and the last
// bounded log lines. It never starts/stops the watcher and never runs
// commands on its behalf.
func supportWatchStatusText() string {
	var b strings.Builder
	pids, err := findSupportWatchPIDs("/proc", watchUID())
	switch {
	case err != nil:
		fmt.Fprintf(&b, "Watcher process detection unavailable: %v\n", err)
	case len(pids) == 0:
		b.WriteString("Support watcher: not running (no same-user `remote-desktop-cli support-watch` process found)\n")
	default:
		sort.Ints(pids)
		strs := make([]string, len(pids))
		for i, pid := range pids {
			strs[i] = strconv.Itoa(pid)
		}
		fmt.Fprintf(&b, "Support watcher: running (PID %s)\n", strings.Join(strs, ", "))
	}

	logPath := watchLogPath()
	if logPath == "" {
		b.WriteString("Watch log: location unavailable\n")
		return b.String()
	}
	tail, err := tailLog(logPath, watchLogMaxLines, watchLogMaxBytes)
	if err != nil {
		fmt.Fprintf(&b, "Watch log: %s (no log read: %v)\n", logPath, err)
		return b.String()
	}
	tail = redactSecrets(tail)
	fmt.Fprintf(&b, "Watch log (%s), last lines:\n%s\n", logPath, tail)
	return b.String()
}
