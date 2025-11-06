package settings

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/stangtennis/Remote/controller/internal/logger"
)

// Settings stores application configuration
type Settings struct {
	// Performance settings
	HighQualityMode bool   `json:"high_quality_mode"`
	MaxResolution   string `json:"max_resolution"`    // "1080p", "1440p", "4K"
	TargetFPS       int    `json:"target_fps"`        // 30, 60, 120
	VideoQuality    int    `json:"video_quality"`     // 1-100
	Codec           string `json:"codec"`             // "H.264", "H.265", "VP9"
	
	// Network settings
	MaxBitrate      int    `json:"max_bitrate"`       // Mbps
	AdaptiveBitrate bool   `json:"adaptive_bitrate"`
	
	// UI settings
	Theme           string `json:"theme"`             // "dark", "light"
	WindowWidth     int    `json:"window_width"`
	WindowHeight    int    `json:"window_height"`
	
	// Feature toggles
	EnableFileTransfer  bool `json:"enable_file_transfer"`
	EnableClipboardSync bool `json:"enable_clipboard_sync"`
	EnableAudio         bool `json:"enable_audio"`
	
	// Advanced
	HardwareAcceleration bool `json:"hardware_acceleration"`
	LowLatencyMode       bool `json:"low_latency_mode"`
}

const settingsFile = "settings.json"

// Default returns default settings
func Default() *Settings {
	return &Settings{
		// Performance - High Quality defaults
		HighQualityMode: true,
		MaxResolution:   "4K",
		TargetFPS:       60,
		VideoQuality:    80,
		Codec:           "H.264",
		
		// Network
		MaxBitrate:      50,
		AdaptiveBitrate: true,
		
		// UI
		Theme:        "dark",
		WindowWidth:  1200,
		WindowHeight: 800,
		
		// Features
		EnableFileTransfer:  true,
		EnableClipboardSync: true,
		EnableAudio:         true,
		
		// Advanced
		HardwareAcceleration: true,
		LowLatencyMode:       true,
	}
}

// LowQuality returns settings optimized for lower-end systems
func LowQuality() *Settings {
	s := Default()
	s.HighQualityMode = false
	s.MaxResolution = "1080p"
	s.TargetFPS = 30
	s.VideoQuality = 50
	s.MaxBitrate = 10
	s.HardwareAcceleration = false
	return s
}

// MediumQuality returns balanced settings
func MediumQuality() *Settings {
	s := Default()
	s.HighQualityMode = false
	s.MaxResolution = "1440p"
	s.TargetFPS = 60
	s.VideoQuality = 70
	s.MaxBitrate = 25
	return s
}

// getSettingsPath returns the path to the settings file
func getSettingsPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	
	appDir := filepath.Join(configDir, "RemoteDesktopController")
	if err := os.MkdirAll(appDir, 0700); err != nil {
		return "", err
	}
	
	return filepath.Join(appDir, settingsFile), nil
}

// Save saves settings to disk
func Save(s *Settings) error {
	path, err := getSettingsPath()
	if err != nil {
		logger.Error("Failed to get settings path: %v", err)
		return err
	}
	
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		logger.Error("Failed to marshal settings: %v", err)
		return err
	}
	
	if err := os.WriteFile(path, data, 0600); err != nil {
		logger.Error("Failed to write settings: %v", err)
		return err
	}
	
	logger.Info("Settings saved successfully")
	return nil
}

// Load loads settings from disk
func Load() (*Settings, error) {
	path, err := getSettingsPath()
	if err != nil {
		return Default(), err
	}
	
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// No saved settings, return defaults
			logger.Info("No saved settings found, using defaults")
			return Default(), nil
		}
		logger.Error("Failed to read settings: %v", err)
		return Default(), err
	}
	
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		logger.Error("Failed to unmarshal settings: %v", err)
		return Default(), err
	}
	
	logger.Info("Settings loaded successfully")
	return &s, nil
}

// GetResolutionPixels returns the pixel dimensions for a resolution string
func (s *Settings) GetResolutionPixels() (width, height int) {
	switch s.MaxResolution {
	case "720p":
		return 1280, 720
	case "1080p":
		return 1920, 1080
	case "1440p":
		return 2560, 1440
	case "4K":
		return 3840, 2160
	default:
		return 1920, 1080
	}
}

// GetQualityDescription returns a human-readable quality description
func (s *Settings) GetQualityDescription() string {
	if s.HighQualityMode {
		return "Ultra (High-Performance Mode)"
	} else if s.VideoQuality >= 70 {
		return "High"
	} else if s.VideoQuality >= 50 {
		return "Medium"
	} else {
		return "Low"
	}
}

// ApplyPreset applies a quality preset
func (s *Settings) ApplyPreset(preset string) {
	var newSettings *Settings
	switch preset {
	case "ultra":
		newSettings = Default()
	case "high":
		newSettings = MediumQuality()
	case "low":
		newSettings = LowQuality()
	default:
		return
	}
	
	// Copy all settings
	*s = *newSettings
}
