package credentials

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/stangtennis/Remote/controller/internal/logger"
)

// Credentials stores user login information
type Credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"` // Note: In production, use OS keyring instead
	Remember bool   `json:"remember"`
}

// DeviceLogin stores Windows login details for a single remote device.
// Stored locally only; never sent to Supabase. The file is restricted to the
// current OS user, matching the existing controller-login credential storage.
type DeviceLogin struct {
	DeviceID     string `json:"device_id"`
	DeviceName   string `json:"device_name,omitempty"`
	Username     string `json:"username,omitempty"`
	Domain       string `json:"domain,omitempty"`
	Password     string `json:"password"`
	SendUsername bool   `json:"send_username"`
	UpdatedAt    string `json:"updated_at"`
}

const (
	credentialsFile       = "credentials.json"
	deviceCredentialsFile = "device_logins.json"
)

// getCredentialsPath returns the path to the credentials file
func getCredentialsPath() (string, error) {
	return getAppConfigPath(credentialsFile)
}

func getDeviceCredentialsPath() (string, error) {
	return getAppConfigPath(deviceCredentialsFile)
}

func getAppConfigPath(filename string) (string, error) {
	// Store in user's config directory
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	appDir := filepath.Join(configDir, "RemoteDesktopController")
	if err := os.MkdirAll(appDir, 0700); err != nil {
		return "", err
	}

	return filepath.Join(appDir, filename), nil
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

// LoadDeviceLogin loads the saved Windows login for a remote device.
func LoadDeviceLogin(deviceID string) (*DeviceLogin, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, nil
	}

	logins, err := loadDeviceLogins()
	if err != nil {
		return nil, err
	}
	login, ok := logins[deviceID]
	if !ok {
		return nil, nil
	}
	return &login, nil
}

// SaveDeviceLogin saves or replaces the Windows login for a remote device.
func SaveDeviceLogin(login *DeviceLogin) error {
	if login == nil || strings.TrimSpace(login.DeviceID) == "" {
		return nil
	}
	login.DeviceID = strings.TrimSpace(login.DeviceID)
	login.DeviceName = strings.TrimSpace(login.DeviceName)
	login.Username = strings.TrimSpace(login.Username)
	login.Domain = strings.TrimSpace(login.Domain)
	login.UpdatedAt = time.Now().Format(time.RFC3339)

	logins, err := loadDeviceLogins()
	if err != nil {
		return err
	}
	logins[login.DeviceID] = *login
	return saveDeviceLogins(logins)
}

// DeleteDeviceLogin removes the saved Windows login for a remote device.
func DeleteDeviceLogin(deviceID string) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil
	}
	logins, err := loadDeviceLogins()
	if err != nil {
		return err
	}
	delete(logins, deviceID)
	return saveDeviceLogins(logins)
}

func loadDeviceLogins() (map[string]DeviceLogin, error) {
	path, err := getDeviceCredentialsPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]DeviceLogin{}, nil
		}
		logger.Error("Failed to read device logins: %v", err)
		return nil, err
	}

	var logins map[string]DeviceLogin
	if err := json.Unmarshal(data, &logins); err != nil {
		logger.Error("Failed to unmarshal device logins: %v", err)
		return nil, err
	}
	if logins == nil {
		logins = map[string]DeviceLogin{}
	}
	return logins, nil
}

func saveDeviceLogins(logins map[string]DeviceLogin) error {
	path, err := getDeviceCredentialsPath()
	if err != nil {
		logger.Error("Failed to get device logins path: %v", err)
		return err
	}

	data, err := json.MarshalIndent(logins, "", "  ")
	if err != nil {
		logger.Error("Failed to marshal device logins: %v", err)
		return err
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		logger.Error("Failed to write device logins: %v", err)
		return err
	}
	logger.Info("Device login credentials saved")
	return nil
}
