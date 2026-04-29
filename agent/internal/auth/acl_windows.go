//go:build windows

package auth

import (
	"log"
	"os/exec"
	"syscall"
)

// lockCredentialsACL restricts the DACL on the credentials file to only
// SYSTEM + Administrators. On NTFS, os.WriteFile's mode bits are largely
// ignored — without this the file inherits %ProgramData% / %APPDATA%
// ACLs which usually grant Users:(RX). The credentials JSON contains a
// refresh_token (~30 days of full account access) and the permanent
// per-device api_key, so it must not be readable by other local users.
func lockCredentialsACL(path string) {
	// Use icacls because reaching for windows.SetNamedSecurityInfo would
	// require encoding SDDL/ACE structures by hand. icacls ships with
	// every supported Windows version.
	cmd := exec.Command("icacls", path,
		"/inheritance:r",
		"/grant:r", "*S-1-5-18:F",  // NT AUTHORITY\SYSTEM
		"/grant:r", "*S-1-5-32-544:F", // BUILTIN\Administrators
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("⚠️  Could not tighten ACL on %s: %v (%s)", path, err, out)
	}
}
