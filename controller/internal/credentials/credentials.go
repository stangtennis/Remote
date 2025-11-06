package credentials

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/stangtennis/Remote/controller/internal/logger"
)

// Credentials stores user login information
type Credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"` // Note: In production, use OS keyring instead
	Remember bool   `json:"remember"`
}

const credentialsFile = "credentials.json"

// getCredentialsPath returns the path to the credentials file
func getCredentialsPath() (string, error) {
	// Store in user's config directory
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	
	appDir := filepath.Join(configDir, "RemoteDesktopController")
	if err := os.MkdirAll(appDir, 0700); err != nil {
		return "", err
	}
	
	return filepath.Join(appDir, credentialsFile), nil
}

// Save saves credentials to disk
func Save(creds *Credentials) error {
	if !creds.Remember {
		// If remember is false, delete any saved credentials
		return Delete()
	}
	
	path, err := getCredentialsPath()
	if err != nil {
		logger.Error("Failed to get credentials path: %v", err)
		return err
	}
	
	data, err := json.Marshal(creds)
	if err != nil {
		logger.Error("Failed to marshal credentials: %v", err)
		return err
	}
	
	// Write with restricted permissions (owner only)
	if err := os.WriteFile(path, data, 0600); err != nil {
		logger.Error("Failed to write credentials: %v", err)
		return err
	}
	
	logger.Info("Credentials saved successfully")
	return nil
}

// Load loads credentials from disk
func Load() (*Credentials, error) {
	path, err := getCredentialsPath()
	if err != nil {
		return nil, err
	}
	
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// No saved credentials
			return nil, nil
		}
		logger.Error("Failed to read credentials: %v", err)
		return nil, err
	}
	
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		logger.Error("Failed to unmarshal credentials: %v", err)
		return nil, err
	}
	
	logger.Info("Credentials loaded successfully")
	return &creds, nil
}

// Delete removes saved credentials
func Delete() error {
	path, err := getCredentialsPath()
	if err != nil {
		return err
	}
	
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		logger.Error("Failed to delete credentials: %v", err)
		return err
	}
	
	logger.Info("Credentials deleted")
	return nil
}
