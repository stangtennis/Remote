//go:build windows

package process

import (
	"encoding/csv"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// List returns all running processes on Windows using tasklist CSV output.
func List() ([]ProcessInfo, error) {
	cmd := exec.Command("tasklist", "/FO", "CSV", "/NH", "/V")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("tasklist failed: %w", err)
	}

	reader := csv.NewReader(strings.NewReader(string(out)))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse CSV failed: %w", err)
	}

	// tasklist /V CSV columns: Image Name, PID, Session Name, Session#, Mem Usage, Status, User Name, CPU Time, Window Title
	var procs []ProcessInfo
	for _, r := range records {
		if len(r) < 7 {
			continue
		}
		pid, _ := strconv.Atoi(strings.TrimSpace(r[1]))
		if pid == 0 {
			continue
		}

		// Parse memory: "12,345 K" → MB
		memStr := strings.ReplaceAll(r[4], ",", "")
		memStr = strings.ReplaceAll(memStr, ".", "")
		memStr = strings.TrimSuffix(strings.TrimSpace(memStr), " K")
		memKB, _ := strconv.ParseFloat(memStr, 64)

		procs = append(procs, ProcessInfo{
			PID:      pid,
			Name:     strings.TrimSpace(r[0]),
			CPU:      0, // tasklist doesn't provide CPU%, could use WMI later
			MemoryMB: memKB / 1024,
			User:     strings.TrimSpace(r[6]),
		})
	}
	return procs, nil
}

// Kill terminates a process by PID on Windows.
func Kill(pid int) error {
	cmd := exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("taskkill failed: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	return nil
}
