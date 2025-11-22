package config

import (
	"fmt"
	"os"
)

type Config struct {
	SupabaseURL       string
	SupabaseAnonKey   string
	DeviceName        string
	DeviceID          string
	APIKey            string // Stored after registration
	HeartbeatInterval int
}

func Load() (*Config, error) {
	cfg := &Config{
		SupabaseURL:       getEnv("SUPABASE_URL", "http://192.168.1.92:8888"),
		SupabaseAnonKey:   getEnv("SUPABASE_ANON_KEY", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyAgCiAgICAicm9sZSI6ICJhbm9uIiwKICAgICJpc3MiOiAic3VwYWJhc2UtZGVtbyIsCiAgICAiaWF0IjogMTY0MTc2OTIwMCwKICAgICJleHAiOiAxNzk5NTM1NjAwCn0.dc_X5iR_VP_qT0zsiyj_I_OZ2T9FtRU2BBNWN8Bu4GE"),
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
