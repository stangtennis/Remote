package updater

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	if err := json.Unmarshal(data, &u.state); err != nil {
		log.Printf("⚠️ Korrupt update state fil, bruger defaults: %v", err)
		os.Remove(u.stateFilePath)
	}
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

	// Gem LastCheck kun ved succesfuld check (ikke ved netværksfejl)
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

	ext := ".exe"
	if runtime.GOOS == "darwin" {
		ext = ""
	}
	exePath := filepath.Join(versionDir, fmt.Sprintf("remote-agent-%s%s", info.TagName, ext))
	log.Printf("📥 Downloading %s to %s", info.ExeURL, exePath)

	if err := u.downloader.DownloadFile(info.ExeURL, exePath, info.ExeSize); err != nil {
		u.lastError = err
		u.setStatus(StatusError)
		return err
	}

	// Verificer SHA256 — inline hash fra version.json har forrang
	expectedHash := info.SHA256Hash
	if expectedHash == "" && info.SHA256URL != "" {
		log.Printf("🔐 Henter SHA256 fra URL...")
		var err error
		expectedHash, err = u.github.DownloadSHA256(info.SHA256URL)
		if err != nil {
			u.lastError = fmt.Errorf("failed to get SHA256: %w", err)
			u.setStatus(StatusError)
			os.Remove(exePath)
			return u.lastError
		}
	}

	if expectedHash != "" {
		log.Printf("🔐 Verificerer SHA256...")
		if err := VerifySHA256(exePath, expectedHash); err != nil {
			u.lastError = err
			u.setStatus(StatusError)
			os.Remove(exePath)
			return err
		}
	} else {
		log.Printf("⚠️ Ingen SHA256 hash tilgængelig — springer verifikation over")
	}

	u.state.DownloadedVersion = info.TagName
	u.state.DownloadPath = exePath
	u.saveState()

	log.Printf("✅ Download complete and verified")
	u.setStatus(StatusReadyToInstall)
	return nil
}

// InstallUpdate installs the downloaded update
// On Windows: launches new exe with --update-from flag
// On macOS: replaces current binary in-place and restarts
func (u *Updater) InstallUpdate() error {
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

	if runtime.GOOS == "darwin" {
		return u.installMacOS(currentExe)
	}

	// Windows: launch the NEW exe with --update-from flag
	log.Printf("🚀 Starting new version with update mode: %s --update-from %s", u.state.DownloadPath, currentExe)
	cmd := exec.Command(u.state.DownloadPath, "--update-from", currentExe)

	if err := cmd.Start(); err != nil {
		u.lastError = err
		u.setStatus(StatusError)
		return err
	}

	u.state.DownloadedVersion = ""
	u.state.DownloadPath = ""
	u.saveState()

	log.Println("✅ New version started, exiting for update...")
	return nil
}

// installMacOS replaces the current binary and restarts
func (u *Updater) installMacOS(currentExe string) error {
	downloadPath := u.state.DownloadPath

	// Make downloaded binary executable
	if err := os.Chmod(downloadPath, 0755); err != nil {
		u.lastError = fmt.Errorf("chmod failed: %w", err)
		u.setStatus(StatusError)
		return u.lastError
	}

	// Backup current binary
	backupPath := currentExe + ".bak"
	if err := os.Rename(currentExe, backupPath); err != nil {
		u.lastError = fmt.Errorf("backup failed: %w", err)
		u.setStatus(StatusError)
		return u.lastError
	}

	// Move new binary to current location
	if err := os.Rename(downloadPath, currentExe); err != nil {
		// Restore backup on failure
		os.Rename(backupPath, currentExe)
		u.lastError = fmt.Errorf("replace failed: %w", err)
		u.setStatus(StatusError)
		return u.lastError
	}

	// Clean up backup
	os.Remove(backupPath)

	u.state.DownloadedVersion = ""
	u.state.DownloadPath = ""
	u.saveState()

	log.Println("✅ Binary replaced, restarting...")

	// Restart self
	cmd := exec.Command(currentExe)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		u.lastError = fmt.Errorf("restart failed: %w", err)
		u.setStatus(StatusError)
		return u.lastError
	}

	log.Println("✅ New version started, exiting...")
	os.Exit(0)
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

// GetDownloadPath returns the path to the downloaded update binary
func (u *Updater) GetDownloadPath() string {
	return u.state.DownloadPath
}

// FetchVersionInfo fetches version info from the update server
func (u *Updater) FetchVersionInfo() (*VersionInfo, error) {
	return u.github.FetchVersionInfo()
}

// CleanOldDownloads sletter gamle version-directories i updates-mappen.
// Beholder kun den aktuelle version og update_state.json.
func (u *Updater) CleanOldDownloads(currentVersion string) {
	updateDir, err := GetUpdateDirectory()
	if err != nil {
		return
	}

	entries, err := os.ReadDir(updateDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue // Behold filer som update_state.json
		}
		if entry.Name() == currentVersion {
			continue // Behold aktuel version
		}
		dirPath := filepath.Join(updateDir, entry.Name())
		if err := os.RemoveAll(dirPath); err != nil {
			log.Printf("⚠️ Kunne ikke slette gammel download %s: %v", entry.Name(), err)
		} else {
			log.Printf("🧹 Slettet gammel download: %s", entry.Name())
		}
	}
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
