package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/stangtennis/Remote/controller/internal/config"
)

// authInfo holds authenticated user credentials with auto-refresh
type authInfo struct {
	accessToken string
	userID      string
	obtainedAt  time.Time
	email       string
	password    string
	supabaseURL string
	anonKey     string
	mu          sync.Mutex
}

const tokenLifetime = 50 * time.Minute // Refresh before the 60-min Supabase expiry

// GetToken returns a valid access token, refreshing if needed
func (a *authInfo) GetToken() string {
	a.mu.Lock()
	defer a.mu.Unlock()

	if time.Since(a.obtainedAt) > tokenLifetime {
		log.Println("[cli] Token expired, refreshing...")
		if err := a.refreshLocked(); err != nil {
			log.Printf("[cli] Token refresh failed: %v", err)
		}
	}
	return a.accessToken
}

// refreshLocked re-authenticates. Caller must hold a.mu.
func (a *authInfo) refreshLocked() error {
	payload := map[string]string{"email": a.email, "password": a.password}
	jsonData, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", a.supabaseURL+"/auth/v1/token?grant_type=password", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("apikey", a.anonKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("auth failed (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken string `json:"access_token"`
		User        struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	a.accessToken = result.AccessToken
	a.userID = result.User.ID
	a.obtainedAt = time.Now()
	log.Println("[cli] Token refreshed successfully")
	return nil
}

// supabaseSignIn authenticates with Supabase and returns auth info
func supabaseSignIn(supabaseURL, anonKey, email, password string) (*authInfo, error) {
	payload := map[string]string{"email": email, "password": password}
	jsonData, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", supabaseURL+"/auth/v1/token?grant_type=password", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", anonKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("auth failed (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken string `json:"access_token"`
		User        struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &authInfo{
		accessToken: result.AccessToken,
		userID:      result.User.ID,
		obtainedAt:  time.Now(),
		email:       email,
		password:    password,
		supabaseURL: supabaseURL,
		anonKey:     anonKey,
	}, nil
}

// getAuthAndConfig loads config and authenticates from env vars
func getAuthAndConfig() (*authInfo, *config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load config: %w", err)
	}

	email := os.Getenv("RD_EMAIL")
	password := os.Getenv("RD_PASSWORD")
	if email == "" || password == "" {
		return nil, nil, fmt.Errorf("RD_EMAIL and RD_PASSWORD environment variables required")
	}

	auth, err := supabaseSignIn(cfg.SupabaseURL, cfg.SupabaseAnonKey, email, password)
	if err != nil {
		return nil, nil, fmt.Errorf("authentication failed: %w", err)
	}

	return auth, cfg, nil
}
