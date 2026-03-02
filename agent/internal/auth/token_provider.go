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

// StartBackgroundRefresh starts a goroutine that pre-refreshes the token
// 5 minutes before expiry. This prevents synchronous refresh blocking polling.
func (tp *TokenProvider) StartBackgroundRefresh() {
	go func() {
		for {
			tp.mu.Lock()
			if tp.creds == nil {
				tp.mu.Unlock()
				time.Sleep(30 * time.Second)
				continue
			}
			expiresAt := tp.creds.ExpiresAt
			tp.mu.Unlock()

			// Refresh 5 minutes before expiry
			refreshAt := time.Unix(expiresAt, 0).Add(-5 * time.Minute)
			sleepDur := time.Until(refreshAt)
			if sleepDur <= 0 {
				sleepDur = 10 * time.Second // Already expired — retry soon
			}
			time.Sleep(sleepDur)

			tp.mu.Lock()
			result, err := RefreshToken(tp.config, tp.creds.RefreshToken)
			if err != nil || !result.Success {
				log.Printf("⚠️  Background token refresh failed: %v", err)
				tp.mu.Unlock()
				continue
			}
			newCreds, err := LoadCredentials()
			if err == nil {
				tp.creds = newCreds
				log.Printf("✅ Token pre-refreshed (expires: %s)", time.Unix(tp.creds.ExpiresAt, 0).Format("15:04:05"))
			}
			tp.mu.Unlock()
		}
	}()
}

// GetAnonKey returns the Supabase anon key (needed for apikey header).
func (tp *TokenProvider) GetAnonKey() string {
	return tp.config.AnonKey
}
