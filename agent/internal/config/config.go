package config

import (
	"fmt"
	"os"
)

type Config struct {
	SupabaseURL     string
	SupabaseAnonKey string
	DeviceName      string
	DeviceID        string
	APIKey          string // Stored after registration
	HeartbeatInterval int
}

func Load() (*Config, error) {
	cfg := &Config{
		SupabaseURL:       getEnv("SUPABASE_URL", "https://mnqtdugcvfyenjuqruol.supabase.co"),
		SupabaseAnonKey:   getEnv("SUPABASE_ANON_KEY", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6Im1ucXRkdWdjdmZ5ZW5qdXFydW9sIiwicm9sZSI6ImFub24iLCJpYXQiOjE3NTkzMDEwODMsImV4cCI6MjA3NDg3NzA4M30.QKs8vMS9tQJgX11GHfarHdpWZHOcCpv0B-aiq7qc15E"),
		DeviceName:        getEnv("DEVICE_NAME", ""),
		HeartbeatInterval: 30, // seconds
	}

	if cfg.SupabaseURL == "" {
		return nil, fmt.Errorf("SUPABASE_URL is required")
	}

	if cfg.SupabaseAnonKey == "" {
		return nil, fmt.Errorf("SUPABASE_ANON_KEY is required")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
