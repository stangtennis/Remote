package updater

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// UpdateStatus represents the current update status
type UpdateStatus int

const (
	StatusUpToDate UpdateStatus = iota
	StatusCheckingForUpdate
	StatusUpdateAvailable
	StatusDownloading
	StatusReadyToInstall
	StatusInstalling
	StatusError
)

func (s UpdateStatus) String() string {
	switch s {
	case StatusUpToDate:
		return "Opdateret"
	case StatusCheckingForUpdate:
		return "Tjekker..."
	case StatusUpdateAvailable:
		return "Opdatering tilgængelig"
	case StatusDownloading:
		return "Downloader..."
	case StatusReadyToInstall:
		return "Klar til installation"
	case StatusInstalling:
		return "Installerer..."
	case StatusError:
		return "Fejl"
	default:
		return "Ukendt"
	}
}

// UpdateState tracks the state of updates
type UpdateState struct {
	LastCheck         time.Time `json:"last_check"`
	IgnoredVersion    string    `json:"ignored_version,omitempty"`
	DownloadedVersion string    `json:"downloaded_version,omitempty"`
	DownloadPath      string    `json:"download_path,omitempty"`
	Channel           string    `json:"channel"`
	AutoUpdate        bool      `json:"auto_update"`
}

// Updater manages the update process for the agent
type Updater struct {
	currentVersion  string
	github          *GitHubClient
	downloader      *Downloader
	state           UpdateState
	stateFilePath   string
	status          UpdateStatus
	availableUpdate *UpdateInfo
	lastError       error
	onStatusChange  func(UpdateStatus)
	onProgress      func(DownloadProgress)
}

// NewUpdater creates a new updater for the agent
func NewUpdater(currentVersion string) (*Updater, error) {
	updateDir, err := GetUpdateDirectory()
	if err != nil {
		return nil, fmt.Errorf("failed to get update directory: %w", err)
	}

	u := &Updater{
		currentVersion: currentVersion,
		github:         NewGitHubClient(),
		downloader:     NewDownloader(),
		stateFilePath:  filepath.Join(updateDir, "update_state.json"),
		state: UpdateState{
			Channel:    "stable",
			AutoUpdate: true,
		},
		status: StatusUpToDate,
	}

	// Load existing state
	u.loadState()

	// Set up progress callback
	u.downloader.SetProgressCallback(func(p DownloadProgress) {
		if u.onProgress != nil {
			u.onProgress(p)
		}
	})

	return u, nil
}

func (u *Updater) loadState() {
	data, err := os.ReadFile(u.stateFilePath)
	if err != nil {
		return
	}
	json.Unmarshal(data, &u.state)
}

func (u *Updater) saveState() error {
	data, err := json.MarshalIndent(u.state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(u.stateFilePath, data, 0644)
}

// SetStatusCallback sets the callback for status changes
func (u *Updater) SetStatusCallback(callback func(UpdateStatus)) {
	u.onStatusChange = callback
}

// SetProgressCallback sets the callback for download progress
func (u *Updater) SetProgressCallback(callback func(DownloadProgress)) {
	u.onProgress = callback
}

func (u *Updater) setStatus(status UpdateStatus) {
	u.status = status
	if u.onStatusChange != nil {
		u.onStatusChange(status)
	}
}

// GetStatus returns the current status
func (u *Updater) GetStatus() UpdateStatus {
	return u.status
}

// GetLastError returns the last error
func (u *Updater) GetLastError() error {
	return u.lastError
}

// GetAvailableUpdate returns info about available update
func (u *Updater) GetAvailableUpdate() *UpdateInfo {
	return u.availableUpdate
}

// GetChannel returns the current update channel
func (u *Updater) GetChannel() string {
	return u.state.Channel
}

// SetChannel sets the update channel
func (u *Updater) SetChannel(channel string) {
	u.state.Channel = channel
	u.saveState()
}

// GetAutoUpdate returns whether auto-update is enabled
func (u *Updater) GetAutoUpdate() bool {
	return u.state.AutoUpdate
}

// SetAutoUpdate enables/disables auto-update
func (u *Updater) SetAutoUpdate(enabled bool) {
	u.state.AutoUpdate = enabled
	u.saveState()
}

// CheckForUpdate checks for available updates
func (u *Updater) CheckForUpdate() error {
	u.setStatus(StatusCheckingForUpdate)
	u.lastError = nil
	u.availableUpdate = nil

	log.Printf("🔍 Checking for updates (channel: %s, current: %s)", u.state.Channel, u.currentVersion)

	info, err := u.github.CheckForUpdate(u.currentVersion, u.state.Channel)
	if err != nil {
		u.lastError = err
		u.setStatus(StatusError)
		return err
	}

	u.state.LastCheck = time.Now()
	u.saveState()

	if info == nil {
		log.Println("✅ Already up to date")
		u.setStatus(StatusUpToDate)
		return nil
	}

	if u.state.IgnoredVersion == info.TagName {
		log.Printf("⏭️ Version %s is ignored", info.TagName)
		u.setStatus(StatusUpToDate)
		return nil
	}

	log.Printf("🆕 Update available: %s", info.TagName)
	u.availableUpdate = info
	u.setStatus(StatusUpdateAvailable)
	return nil
}

// DownloadUpdate downloads the available update
func (u *Updater) DownloadUpdate() error {
	if u.availableUpdate == nil {
		return fmt.Errorf("no update available")
	}

	u.setStatus(StatusDownloading)
	u.lastError = nil

	info := u.availableUpdate

	updateDir, err := GetUpdateDirectory()
	if err != nil {
		u.lastError = err
		u.setStatus(StatusError)
		return err
	}

	versionDir := filepath.Join(updateDir, info.TagName)
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		u.lastError = err
		u.setStatus(StatusError)
		return err
	}

	exePath := filepath.Join(versionDir, fmt.Sprintf("remote-agent-%s.exe", info.TagName))
	log.Printf("📥 Downloading %s to %s", info.ExeURL, exePath)

	if err := u.downloader.DownloadFile(info.ExeURL, exePath, info.ExeSize); err != nil {
		u.lastError = err
		u.setStatus(StatusError)
		return err
	}

	if info.SHA256URL != "" {
		log.Printf("🔐 Verifying SHA256...")
		expectedHash, err := u.github.DownloadSHA256(info.SHA256URL)
		if err != nil {
			u.lastError = fmt.Errorf("failed to get SHA256: %w", err)
			u.setStatus(StatusError)
			os.Remove(exePath)
			return u.lastError
		}

		if err := VerifySHA256(exePath, expectedHash); err != nil {
			u.lastError = err
			u.setStatus(StatusError)
			os.Remove(exePath)
			return err
		}
	}

	u.state.DownloadedVersion = info.TagName
	u.state.DownloadPath = exePath
	u.saveState()

	log.Printf("✅ Download complete and verified")
	u.setStatus(StatusReadyToInstall)
	return nil
}

// InstallUpdate installs the downloaded update
// serviceName: name of the Windows service (empty for run-once mode)
func (u *Updater) InstallUpdate(serviceName string) error {
	if u.state.DownloadPath == "" {
		return fmt.Errorf("no update downloaded")
	}

	if _, err := os.Stat(u.state.DownloadPath); os.IsNotExist(err) {
		return fmt.Errorf("downloaded file not found: %s", u.state.DownloadPath)
	}

	u.setStatus(StatusInstalling)

	currentExe, err := os.Executable()
	if err != nil {
		u.lastError = err
		u.setStatus(StatusError)
		return err
	}

	exeDir := filepath.Dir(currentExe)
	updaterPath := filepath.Join(exeDir, "agent-updater.exe")

	if _, err := os.Stat(updaterPath); os.IsNotExist(err) {
		u.lastError = fmt.Errorf("updater helper not found: %s", updaterPath)
		u.setStatus(StatusError)
		return u.lastError
	}

	backupPath := currentExe + ".old"

	args := []string{
		"--target", currentExe,
		"--source", u.state.DownloadPath,
		"--backup", backupPath,
		"--restart",
	}

	if serviceName != "" {
		args = append(args, "--service-name", serviceName)
	}

	log.Printf("🚀 Launching updater: %s %v", updaterPath, args)
	cmd := exec.Command(updaterPath, args...)

	if err := cmd.Start(); err != nil {
		u.lastError = err
		u.setStatus(StatusError)
		return err
	}

	u.state.DownloadedVersion = ""
	u.state.DownloadPath = ""
	u.saveState()

	log.Println("✅ Updater started, exiting for update...")
	return nil
}

// ShouldAutoCheck returns true if auto-check should run
func (u *Updater) ShouldAutoCheck(interval time.Duration) bool {
	if !u.state.AutoUpdate {
		return false
	}
	return time.Since(u.state.LastCheck) >= interval
}

// HasPendingUpdate returns true if there's a downloaded update ready
func (u *Updater) HasPendingUpdate() bool {
	if u.state.DownloadPath == "" {
		return false
	}
	if _, err := os.Stat(u.state.DownloadPath); os.IsNotExist(err) {
		return false
	}
	return true
}

// IgnoreUpdate ignores the current available update
func (u *Updater) IgnoreUpdate() {
	if u.availableUpdate != nil {
		u.state.IgnoredVersion = u.availableUpdate.TagName
		u.saveState()
		u.availableUpdate = nil
		u.setStatus(StatusUpToDate)
	}
}
