package credentials

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
)

const secretServiceName = "RemoteDesktopController"

var errSecretNotFound = errors.New("secret not found")

func secretFilename(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:]) + ".secret"
}

func secretFilePath(key string) (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	secretDir := filepath.Join(configDir, secretServiceName, "secrets")
	if err := os.MkdirAll(secretDir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(secretDir, secretFilename(key)), nil
}
