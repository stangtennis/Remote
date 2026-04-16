//go:build darwin

package process

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// List returns all running processes on macOS using ps.
func List() ([]ProcessInfo, error) {
	cmd := exec.Command("ps", "axo", "pid,pcpu,rss,user,comm")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ps failed: %w", err)
	}

	lines := strings.Split(string(out), "\n")
	var procs []ProcessInfo
	for i, line := range lines {
		if i == 0 { // skip header
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		pid, _ := strconv.Atoi(fields[0])
		if pid == 0 {
			continue
		}
		cpu, _ := strconv.ParseFloat(fields[1], 64)
		rssKB, _ := strconv.ParseFloat(fields[2], 64)

		procs = append(procs, ProcessInfo{
			PID:      pid,
			Name:     fields[4],
			CPU:      cpu,
			MemoryMB: rssKB / 1024,
			User:     fields[3],
		})
	}
	return procs, nil
}

// Kill terminates a process by PID on macOS.
func Kill(pid int) error {
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		return fmt.Errorf("kill %d failed: %w", pid, err)
	}
	return nil
}
