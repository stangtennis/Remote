package updater

import (
	"os/exec"
	"strings"
)

// launchElevated on macOS just launches normally (no elevation needed for user-space apps)
func launchElevated(exePath string, args string) error {
	parts := strings.Fields(args)
	cmd := exec.Command(exePath, parts...)
	return cmd.Start()
}
