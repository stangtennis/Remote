package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
	SaveCredentials(creds)

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
func GetCredentialsPath() string {
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	return filepath.Join(exeDir, ".credentials")
}

// SaveCredentials saves credentials to file
func SaveCredentials(creds *Credentials) error {
	data, err := json.Marshal(creds)
	if err != nil {
		return err
	}
	return os.WriteFile(GetCredentialsPath(), data, 0600)
}

// LoadCredentials loads credentials from file
func LoadCredentials() (*Credentials, error) {
	data, err := os.ReadFile(GetCredentialsPath())
	if err != nil {
		return nil, err
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}

	return &creds, nil
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
