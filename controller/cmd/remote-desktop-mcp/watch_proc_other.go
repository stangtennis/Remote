//go:build !linux

package main

import (
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

func watchUID() int { return 0 }

// findSupportWatchPIDs falls back to pgrep with a fixed argv (no shell) on
// platforms without /proc. The pattern is a constant; nothing user-supplied
// is ever interpolated into it.
func findSupportWatchPIDs(root string, uid int) ([]int, error) {
	out, err := exec.Command("pgrep", "-u", strconv.Itoa(uid), "-f", "remote-desktop-cli support-watch").Output()
	if err != nil {
		// pgrep exits non-zero when there is simply no match.
		if _, ok := err.(*exec.ExitError); ok {
			return nil, nil
		}
		return nil, err
	}
	var pids []int
	for _, field := range strings.Fields(string(out)) {
		if pid, convErr := strconv.Atoi(field); convErr == nil {
			pids = append(pids, pid)
		}
	}
	sort.Ints(pids)
	return pids, nil
}
