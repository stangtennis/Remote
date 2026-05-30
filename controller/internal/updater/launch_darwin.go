package updater

import (
	"os/exec"
	"strings"
)

// launchElevated on macOS just launches normally (no elevation needed for user-space apps)
func launchElevated(exePath string, args string) error {
	if oldPath, ok := strings.CutPrefix(args, "--update-from "); ok {
		oldPath = strings.Trim(oldPath, `"`)
		cmd := exec.Command(exePath, "--update-from", oldPath)
		return cmd.Start()
	}

	parts := strings.Fields(args)
	cmd := exec.Command(exePath, parts...)
	return cmd.Start()
}
