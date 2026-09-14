//go:build linux

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"syscall"
)

func watchUID() int { return os.Getuid() }

// findSupportWatchPIDs scans a /proc tree (root is injectable for tests)
// without spawning any process and without a shell. A PID matches when its
// owner uid matches uid and its argv contains `remote-desktop-cli
// support-watch` as consecutive arguments.
func findSupportWatchPIDs(root string, uid int) ([]int, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", root, err)
	}

	self := os.Getpid()
	var pids []int
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid == self {
			continue
		}

		procDir := filepath.Join(root, entry.Name())
		if info, statErr := os.Stat(procDir); statErr == nil {
			if sys, ok := info.Sys().(*syscall.Stat_t); ok && int(sys.Uid) != uid {
				continue // not our user; ignore
			}
		}

		data, err := os.ReadFile(filepath.Join(procDir, "cmdline"))
		if err != nil || len(data) == 0 {
			continue
		}
		argv := bytes.Split(bytes.TrimRight(data, "\x00"), []byte{0})
		if argvMatchesSupportWatch(argv) {
			pids = append(pids, pid)
		}
	}
	sort.Ints(pids)
	return pids, nil
}

func argvMatchesSupportWatch(argv [][]byte) bool {
	for i := 0; i+1 < len(argv); i++ {
		if filepath.Base(string(argv[i])) == "remote-desktop-cli" && string(argv[i+1]) == "support-watch" {
			return true
		}
	}
	return false
}
