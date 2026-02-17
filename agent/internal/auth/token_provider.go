package auth

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// TokenProvider manages JWT token lifecycle with auto-refresh.
// Thread-safe - can be shared across goroutines.
type TokenProvider struct {
	mu     sync.Mutex
	config AuthConfig
	creds  *Credentials
}

// NewTokenProvider creates a TokenProvider from existing credentials.
func NewTokenProvider(config AuthConfig, creds *Credentials) *TokenProvider {
	return &TokenProvider{
		config: config,
		creds:  creds,
	}
}

// GetToken returns a valid access token, refreshing if needed.
// Uses a 60-second margin before expiry to avoid mid-request expiration.
func (tp *TokenProvider) GetToken() (string, error) {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	if tp.creds == nil {
		return "", fmt.Errorf("no credentials available")
	}

	// Check if token expires within 60 seconds
	if time.Now().Unix() < tp.creds.ExpiresAt-60 {
		return tp.creds.AccessToken, nil
	}

	// Token expired or about to expire - refresh
	log.Println("🔄 Token expired or expiring soon, refreshing...")

	result, err := RefreshToken(tp.config, tp.creds.RefreshToken)
	if err != nil {
		return "", fmt.Errorf("token refresh failed: %w", err)
	}
	if !result.Success {
		return "", fmt.Errorf("token refresh failed: %s", result.Message)
	}

	// Reload saved credentials (RefreshToken already saves them)
	newCreds, err := LoadCredentials()
	if err != nil {
		return "", fmt.Errorf("failed to load refreshed credentials: %w", err)
	}

	tp.creds = newCreds
	log.Printf("✅ Token refreshed successfully (expires: %s)",
		time.Unix(tp.creds.ExpiresAt, 0).Format("15:04:05"))

	return tp.creds.AccessToken, nil
}

// GetAnonKey returns the Supabase anon key (needed for apikey header).
func (tp *TokenProvider) GetAnonKey() string {
	return tp.config.AnonKey
}
