//go:build !windows && !darwin

package credentials

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
)

// secretStorePath returns a stable, permission-restricted path for a secret.
// Linux has no standard OS keychain available to all desktops, so this is a
// file-based fallback bound to the OS user via 0600 permissions. It mirrors
// the approach used by the agent on non-Windows platforms.
func secretStorePath(key string) (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appDir := filepath.Join(configDir, "RemoteDesktopController", "secrets")
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(appDir, hex.EncodeToString(sum[:])+".secret"), nil
}

func saveSecret(key, value string) error {
	path, err := secretStorePath(key)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".secret-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.WriteString(value); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func loadSecret(key string) (string, error) {
	path, err := secretStorePath(key)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func deleteSecret(key string) error {
	path, err := secretStorePath(key)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
