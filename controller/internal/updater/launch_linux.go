//go:build linux

package updater

import (
	"fmt"
	"os/exec"
)

// launchElevated launches an executable with elevated privileges using pkexec
func launchElevated(exe string, args string) error {
	cmd := exec.Command("pkexec", exe)
	if args != "" {
		cmd = exec.Command("pkexec", exe, args)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("pkexec failed: %w", err)
	}
	return nil
}
