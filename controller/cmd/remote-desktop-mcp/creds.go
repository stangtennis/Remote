package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// credentialFileLimit bounds how much of the credentials file is inspected.
// Only key presence is checked; values are never stored or printed.
const credentialFileLimit = 8 << 10

// checkCredentialPresence verifies — without reading secret values into
// anything this adapter reports — that credentials the CLI will need are
// available. It mirrors the CLI's own loader: either RD_EMAIL and RD_PASSWORD
// are both set in the environment, or the credentials file referenced by
// RD_CREDENTIALS_FILE (default ~/.config/remote-desktop/credentials.env)
// exists with tight permissions and defines both variables.
//
// This is a presence check only. Actual authentication is performed by the
// CLI subprocess itself.
func checkCredentialPresence() error {
	if os.Getenv("RD_EMAIL") != "" && os.Getenv("RD_PASSWORD") != "" {
		return nil
	}

	path := os.Getenv("RD_CREDENTIALS_FILE")
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("cannot determine home directory for default credentials file: %w", err)
		}
		path = filepath.Join(home, ".config", "remote-desktop", "credentials.env")
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("no credentials: set RD_EMAIL/RD_PASSWORD or create %s (mode 0600)", path)
	}
	if info.IsDir() {
		return fmt.Errorf("no credentials: %s is a directory, expected a credentials file", path)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("credentials file %s must not be group/world accessible (want 0600)", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read credentials file %s: %w", path, err)
	}
	if len(data) > credentialFileLimit {
		data = data[:credentialFileLimit]
	}

	hasEmail, hasPassword := false, false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
		switch parts[0] {
		case "RD_EMAIL":
			hasEmail = value != ""
		case "RD_PASSWORD":
			hasPassword = value != ""
		}
	}
	if !hasEmail || !hasPassword {
		return fmt.Errorf("credentials file %s does not define both RD_EMAIL and RD_PASSWORD", path)
	}
	return nil
}
