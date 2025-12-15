package updater

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// UpdateState tracks the state of updates
type UpdateState struct {
	LastCheck        time.Time `json:"last_check"`
	IgnoredVersion   string    `json:"ignored_version,omitempty"`
	DownloadedVersion string   `json:"downloaded_version,omitempty"`
	DownloadPath     string    `json:"download_path,omitempty"`
	Channel          string    `json:"channel"` // "stable" or "beta"
	AutoCheck        bool      `json:"auto_check"`
}

// StateManager manages update state persistence
type StateManager struct {
	mu       sync.RWMutex
	state    UpdateState
	filePath string
}

// NewStateManager creates a new state manager
func NewStateManager() (*StateManager, error) {
	updateDir, err := GetUpdateDirectory()
	if err != nil {
		return nil, err
	}

	sm := &StateManager{
		filePath: filepath.Join(updateDir, "update_state.json"),
		state: UpdateState{
			Channel:   "stable",
			AutoCheck: true,
		},
	}

	// Load existing state
	sm.load()

	return sm, nil
}

// load reads state from disk
func (sm *StateManager) load() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	data, err := os.ReadFile(sm.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No state file yet
		}
		return err
	}

	return json.Unmarshal(data, &sm.state)
}

// save writes state to disk
func (sm *StateManager) save() error {
	data, err := json.MarshalIndent(sm.state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(sm.filePath, data, 0644)
}

// GetState returns a copy of the current state
func (sm *StateManager) GetState() UpdateState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state
}

// SetLastCheck updates the last check time
func (sm *StateManager) SetLastCheck(t time.Time) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.state.LastCheck = t
	return sm.save()
}

// SetIgnoredVersion sets a version to ignore
func (sm *StateManager) SetIgnoredVersion(version string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.state.IgnoredVersion = version
	return sm.save()
}

// SetDownloadedVersion records a downloaded update
func (sm *StateManager) SetDownloadedVersion(version string, path string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.state.DownloadedVersion = version
	sm.state.DownloadPath = path
	return sm.save()
}

// ClearDownload clears the downloaded update info
func (sm *StateManager) ClearDownload() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.state.DownloadedVersion = ""
	sm.state.DownloadPath = ""
	return sm.save()
}

// SetChannel sets the update channel
func (sm *StateManager) SetChannel(channel string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.state.Channel = channel
	return sm.save()
}

// SetAutoCheck enables/disables auto-check
func (sm *StateManager) SetAutoCheck(enabled bool) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.state.AutoCheck = enabled
	return sm.save()
}

// ShouldCheck returns true if enough time has passed since last check
func (sm *StateManager) ShouldCheck(interval time.Duration) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	if !sm.state.AutoCheck {
		return false
	}
	
	return time.Since(sm.state.LastCheck) >= interval
}

// IsVersionIgnored returns true if the version should be ignored
func (sm *StateManager) IsVersionIgnored(version string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state.IgnoredVersion == version
}

// HasPendingUpdate returns true if there's a downloaded update ready
func (sm *StateManager) HasPendingUpdate() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	if sm.state.DownloadedVersion == "" || sm.state.DownloadPath == "" {
		return false
	}
	
	// Check if file still exists
	if _, err := os.Stat(sm.state.DownloadPath); os.IsNotExist(err) {
		return false
	}
	
	return true
}
