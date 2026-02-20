package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Credentials holds user login credentials
type Credentials struct {
	Email        string `json:"email"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	UserID       string `json:"user_id"`
	ExpiresAt    int64  `json:"expires_at"`
}

// AuthConfig holds Supabase auth configuration
type AuthConfig struct {
	SupabaseURL string
	AnonKey     string
}

// AuthResult represents the result of authentication
type AuthResult struct {
	Success     bool
	UserID      string
	Email       string
	AccessToken string
	Message     string
	Approved    bool
}

// Login authenticates user with Supabase
func Login(config AuthConfig, email, password string) (*AuthResult, error) {
	url := fmt.Sprintf("%s/auth/v1/token?grant_type=password", config.SupabaseURL)

	payload := map[string]string{
		"email":    email,
		"password": password,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", config.AnonKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		if json.Unmarshal(body, &errResp) == nil {
			if msg, ok := errResp["error_description"].(string); ok {
				return &AuthResult{Success: false, Message: msg}, nil
			}
			if msg, ok := errResp["msg"].(string); ok {
				return &AuthResult{Success: false, Message: msg}, nil
			}
		}
		return &AuthResult{Success: false, Message: "Login failed"}, nil
	}

	var authResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		User         struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"user"`
	}

	if err := json.Unmarshal(body, &authResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check if user is approved
	approved, err := CheckUserApproval(config, authResp.AccessToken, authResp.User.ID)
	if err != nil {
		return &AuthResult{
			Success: false,
			Message: "Could not verify account approval",
		}, nil
	}

	if !approved {
		return &AuthResult{
			Success:  false,
			Message:  "Your account is pending approval.\nPlease wait for an administrator to approve your account.",
			Approved: false,
		}, nil
	}

	// Save credentials
	creds := &Credentials{
		Email:        authResp.User.Email,
		AccessToken:  authResp.AccessToken,
		RefreshToken: authResp.RefreshToken,
		UserID:       authResp.User.ID,
		ExpiresAt:    time.Now().Add(time.Duration(authResp.ExpiresIn) * time.Second).Unix(),
	}
	
	credPath := GetCredentialsPath()
	fmt.Printf("📁 Saving credentials to: %s\n", credPath)
	
	if err := SaveCredentials(creds); err != nil {
		fmt.Printf("❌ Failed to save credentials: %v\n", err)
		return &AuthResult{
			Success: false,
			Message: fmt.Sprintf("Login succeeded but failed to save credentials: %v", err),
		}, nil
	}
	fmt.Printf("✅ Credentials saved successfully\n")

	return &AuthResult{
		Success:     true,
		UserID:      authResp.User.ID,
		Email:       authResp.User.Email,
		AccessToken: authResp.AccessToken,
		Message:     "Login successful",
		Approved:    true,
	}, nil
}

// CheckUserApproval checks if user is approved in user_approvals table
func CheckUserApproval(config AuthConfig, accessToken, userID string) (bool, error) {
	url := fmt.Sprintf("%s/rest/v1/user_approvals?user_id=eq.%s&select=approved", config.SupabaseURL, userID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, err
	}

	req.Header.Set("apikey", config.AnonKey)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var approvals []struct {
		Approved bool `json:"approved"`
	}

	if err := json.Unmarshal(body, &approvals); err != nil {
		return false, err
	}

	if len(approvals) == 0 {
		return false, nil
	}

	return approvals[0].Approved, nil
}

// GetCredentialsPath returns the path to the credentials file
// Uses AppData (Windows), ~/Library/Application Support (macOS), or exe directory
func GetCredentialsPath() string {
	// macOS: ~/Library/Application Support/RemoteDesktopAgent/
	if home, err := os.UserHomeDir(); err == nil {
		macDir := filepath.Join(home, "Library", "Application Support", "RemoteDesktopAgent")
		if _, err := os.Stat(filepath.Join(home, "Library")); err == nil {
			if err := os.MkdirAll(macDir, 0755); err == nil {
				return filepath.Join(macDir, ".credentials")
			}
		}
	}

	// Windows: AppData (works without admin)
	appData := os.Getenv("APPDATA")
	if appData != "" {
		credDir := filepath.Join(appData, "RemoteDesktopAgent")
		if err := os.MkdirAll(credDir, 0755); err == nil {
			credPath := filepath.Join(credDir, ".credentials")
			if _, err := os.Stat(credDir); err == nil {
				return credPath
			}
		}
	}

	// Windows: ProgramData (for services running as SYSTEM)
	programData := os.Getenv("ProgramData")
	if programData != "" {
		credDir := filepath.Join(programData, "RemoteDesktopAgent")
		if err := os.MkdirAll(credDir, 0755); err == nil {
			return filepath.Join(credDir, ".credentials")
		}
	}

	// Fallback to exe directory
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	return filepath.Join(exeDir, ".credentials")
}

// SaveCredentials saves credentials to file
// Saves to BOTH AppData (for user) and ProgramData (for service)
func SaveCredentials(creds *Credentials) error {
	data, err := json.Marshal(creds)
	if err != nil {
		log.Printf("❌ Failed to marshal credentials: %v", err)
		return err
	}

	var lastErr error
	saved := false

	// Save to macOS Application Support
	if home, err := os.UserHomeDir(); err == nil {
		macDir := filepath.Join(home, "Library", "Application Support", "RemoteDesktopAgent")
		if _, err := os.Stat(filepath.Join(home, "Library")); err == nil {
			if err := os.MkdirAll(macDir, 0755); err == nil {
				credPath := filepath.Join(macDir, ".credentials")
				if err := os.WriteFile(credPath, data, 0600); err == nil {
					log.Printf("Saved to Application Support: %s", credPath)
					saved = true
				}
			}
		}
	}

	// Save to AppData (user accessible, Windows)
	appData := os.Getenv("APPDATA")
	if appData != "" {
		credDir := filepath.Join(appData, "RemoteDesktopAgent")
		log.Printf("📁 Creating dir: %s", credDir)
		if err := os.MkdirAll(credDir, 0755); err == nil {
			credPath := filepath.Join(credDir, ".credentials")
			log.Printf("📁 Writing to: %s", credPath)
			if err := os.WriteFile(credPath, data, 0600); err == nil {
				log.Printf("✅ Saved to AppData: %s", credPath)
				saved = true
			} else {
				log.Printf("❌ Failed to write to AppData: %v", err)
				lastErr = err
			}
		} else {
			log.Printf("❌ Failed to create AppData dir: %v", err)
		}
	}

	// Also try to save to ProgramData (service accessible)
	programData := os.Getenv("ProgramData")
	log.Printf("📁 ProgramData=%s", programData)
	if programData != "" {
		credDir := filepath.Join(programData, "RemoteDesktopAgent")
		if err := os.MkdirAll(credDir, 0755); err == nil {
			credPath := filepath.Join(credDir, ".credentials")
			if err := os.WriteFile(credPath, data, 0600); err == nil {
				log.Printf("✅ Saved to ProgramData: %s", credPath)
				saved = true
			} else {
				log.Printf("⚠️ Failed to write to ProgramData: %v", err)
				// Only set error if we haven't saved anywhere yet
				if !saved {
					lastErr = err
				}
			}
		}
	}

	if saved {
		log.Printf("✅ Credentials saved successfully")
		return nil
	}
	log.Printf("❌ Failed to save credentials anywhere: %v", lastErr)
	return lastErr
}

// LoadCredentials loads credentials from file
// Checks multiple locations: ProgramData (service), AppData (user), exe dir
func LoadCredentials() (*Credentials, error) {
	// Build list of paths to check
	var paths []string

	// 0. macOS Application Support
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, "Library", "Application Support", "RemoteDesktopAgent", ".credentials"))
	}

	// 1. ProgramData (for services running as SYSTEM, Windows)
	programData := os.Getenv("ProgramData")
	if programData != "" {
		paths = append(paths, filepath.Join(programData, "RemoteDesktopAgent", ".credentials"))
	}
	
	// 2. User's AppData
	appData := os.Getenv("APPDATA")
	if appData != "" {
		paths = append(paths, filepath.Join(appData, "RemoteDesktopAgent", ".credentials"))
	}
	
	// 3. Exe directory
	exePath, _ := os.Executable()
	if exePath != "" {
		paths = append(paths, filepath.Join(filepath.Dir(exePath), ".credentials"))
	}
	
	// Try each path
	var lastErr error
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			lastErr = err
			continue
		}
		
		var creds Credentials
		if err := json.Unmarshal(data, &creds); err != nil {
			lastErr = err
			continue
		}
		
		log.Printf("📁 Loaded credentials from: %s", path)
		return &creds, nil
	}
	
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("no credentials found in any location")
}

// ClearCredentials removes saved credentials
func ClearCredentials() error {
	return os.Remove(GetCredentialsPath())
}

// IsLoggedIn checks if there are valid saved credentials
func IsLoggedIn() bool {
	creds, err := LoadCredentials()
	if err != nil {
		return false
	}

	// Check if token is expired
	if time.Now().Unix() > creds.ExpiresAt {
		return false
	}

	return true
}

// GetCurrentUser returns the current logged in user info
func GetCurrentUser() (*Credentials, error) {
	return LoadCredentials()
}

// RefreshToken refreshes the access token using refresh token
func RefreshToken(config AuthConfig, refreshToken string) (*AuthResult, error) {
	url := fmt.Sprintf("%s/auth/v1/token?grant_type=refresh_token", config.SupabaseURL)

	payload := map[string]string{
		"refresh_token": refreshToken,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("apikey", config.AnonKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &AuthResult{Success: false, Message: "Token refresh failed"}, nil
	}

	body, _ := io.ReadAll(resp.Body)

	var authResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		User         struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"user"`
	}

	if err := json.Unmarshal(body, &authResp); err != nil {
		return nil, err
	}

	// Save new credentials
	creds := &Credentials{
		Email:        authResp.User.Email,
		AccessToken:  authResp.AccessToken,
		RefreshToken: authResp.RefreshToken,
		UserID:       authResp.User.ID,
		ExpiresAt:    time.Now().Add(time.Duration(authResp.ExpiresIn) * time.Second).Unix(),
	}
	SaveCredentials(creds)

	return &AuthResult{
		Success:     true,
		UserID:      authResp.User.ID,
		Email:       authResp.User.Email,
		AccessToken: authResp.AccessToken,
	}, nil
}
