package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/stangtennis/Remote/controller/internal/config"
)

const edgeMCPTokenLifetime = 50 * time.Minute

type edgeMCPClient struct {
	endpoint    string
	supabaseURL string
	anonKey     string
	email       string
	password    string
	httpClient  *http.Client

	mu          sync.Mutex
	accessToken string
	obtainedAt  time.Time
}

type edgeMCPResponse struct {
	Result *struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"result"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func newEdgeMCPClient() (*edgeMCPClient, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	email, password, err := loadMCPLoginCredentials()
	if err != nil {
		return nil, err
	}
	return &edgeMCPClient{
		endpoint:    strings.TrimRight(cfg.SupabaseURL, "/") + "/functions/v1/readonly-mcp",
		supabaseURL: strings.TrimRight(cfg.SupabaseURL, "/"),
		anonKey:     cfg.SupabaseAnonKey,
		email:       email,
		password:    password,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func loadMCPLoginCredentials() (string, string, error) {
	if email, password := os.Getenv("RD_EMAIL"), os.Getenv("RD_PASSWORD"); email != "" && password != "" {
		return email, password, nil
	}
	path := os.Getenv("RD_CREDENTIALS_FILE")
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", "", fmt.Errorf("cannot determine credentials directory: %w", err)
		}
		path = filepath.Join(home, ".config", "remote-desktop", "credentials.env")
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", "", fmt.Errorf("credentials file unavailable: %w", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return "", "", fmt.Errorf("credentials file %s must not be group/world accessible", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("cannot read credentials file: %w", err)
	}
	var email, password string
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), "=", 2)
		if len(parts) != 2 {
			continue
		}
		value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
		switch parts[0] {
		case "RD_EMAIL":
			email = value
		case "RD_PASSWORD":
			password = value
		}
	}
	if email == "" || password == "" {
		return "", "", fmt.Errorf("credentials file does not define both RD_EMAIL and RD_PASSWORD")
	}
	return email, password, nil
}

func (c *edgeMCPClient) loginLocked(ctx context.Context) error {
	body, err := json.Marshal(map[string]string{"email": c.email, "password": c.password})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.supabaseURL+"/auth/v1/token?grant_type=password", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("apikey", c.anonKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("MCP authentication failed (HTTP %d)", resp.StatusCode)
	}
	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}
	if result.AccessToken == "" {
		return fmt.Errorf("MCP authentication returned no access token")
	}
	c.accessToken = result.AccessToken
	c.obtainedAt = time.Now()
	return nil
}

func (c *edgeMCPClient) callTool(ctx context.Context, name string, args map[string]any) (string, error) {
	c.mu.Lock()
	if c.accessToken == "" || time.Since(c.obtainedAt) > edgeMCPTokenLifetime {
		if err := c.loginLocked(ctx); err != nil {
			c.mu.Unlock()
			return "", err
		}
	}
	token := c.accessToken
	c.mu.Unlock()

	result, status, err := c.callWithToken(ctx, token, name, args)
	if err != nil || status != http.StatusUnauthorized {
		return result, err
	}

	c.mu.Lock()
	if err := c.loginLocked(ctx); err != nil {
		c.mu.Unlock()
		return "", err
	}
	token = c.accessToken
	c.mu.Unlock()
	result, _, err = c.callWithToken(ctx, token, name, args)
	return result, err
}

func (c *edgeMCPClient) callWithToken(ctx context.Context, token, name string, args map[string]any) (string, int, error) {
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": name, "arguments": args},
	})
	if err != nil {
		return "", 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("apikey", c.anonKey)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 128<<10))
	if err != nil {
		return "", resp.StatusCode, err
	}
	var envelope edgeMCPResponse
	if err := json.Unmarshal(data, &envelope); err != nil {
		return "", resp.StatusCode, fmt.Errorf("invalid MCP response")
	}
	if envelope.Error != nil {
		return "", resp.StatusCode, fmt.Errorf("MCP tool %s failed: %s", name, envelope.Error.Message)
	}
	if envelope.Result == nil {
		return "", resp.StatusCode, fmt.Errorf("MCP tool %s returned no result", name)
	}
	for _, item := range envelope.Result.Content {
		if item.Type == "text" {
			return item.Text, resp.StatusCode, nil
		}
	}
	return "", resp.StatusCode, fmt.Errorf("MCP tool %s returned no text", name)
}
