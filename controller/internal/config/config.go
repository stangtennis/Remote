package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Config holds application configuration
type Config struct {
	SupabaseURL     string
	SupabaseAnonKey string
}

// Load loads configuration from .env file
func Load() (*Config, error) {
	config := &Config{
		// Default values (local Supabase instance for development/testing)
		SupabaseURL:     "http://192.168.1.92:8888",
		SupabaseAnonKey: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyAgCiAgICAicm9sZSI6ICJhbm9uIiwKICAgICJpc3MiOiAic3VwYWJhc2UtZGVtbyIsCiAgICAiaWF0IjogMTY0MTc2OTIwMCwKICAgICJleHAiOiAxNzk5NTM1NjAwCn0.dc_X5iR_VP_qT0zsiyj_I_OZ2T9FtRU2BBNWN8Bu4GE",
	}

	// Try to load from .env file if it exists
	file, err := os.Open(".env")
	if err != nil {
		// .env file doesn't exist, use defaults
		return config, nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "SUPABASE_URL":
			config.SupabaseURL = value
		case "SUPABASE_ANON_KEY":
			config.SupabaseAnonKey = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading .env file: %w", err)
	}

	return config, nil
}
