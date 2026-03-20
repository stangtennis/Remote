package updater

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
		return "Tjekker for opdateringer..."
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

// Updater manages the update process
type Updater struct {
	currentVersion string
	appType        string // "controller" or "remote-agent"
	github         *GitHubClient
	downloader     *Downloader
	state          *StateManager
	
	status         UpdateStatus
	availableUpdate *UpdateInfo
	lastError      error
	
	onStatusChange func(UpdateStatus)
	onProgress     func(DownloadProgress)
}

// NewUpdater creates a new updater for the specified app
func NewUpdater(currentVersion string, appType string) (*Updater, error) {
	state, err := NewStateManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create state manager: %w", err)
	}

	u := &Updater{
		currentVersion: currentVersion,
		appType:        appType,
		github:         NewGitHubClient(),
		downloader:     NewDownloader(),
		state:          state,
		status:         StatusUpToDate,
	}

	// Set up progress callback
	u.downloader.SetProgressCallback(func(p DownloadProgress) {
		if u.onProgress != nil {
			u.onProgress(p)
		}
	})

	return u, nil
}

// SetStatusCallback sets the callback for status changes
func (u *Updater) SetStatusCallback(callback func(UpdateStatus)) {
	u.onStatusChange = callback
}

// SetProgressCallback sets the callback for download progress
func (u *Updater) SetProgressCallback(callback func(DownloadProgress)) {
	u.onProgress = callback
}

// setStatus updates the status and notifies callback
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
	return u.state.GetState().Channel
}

// SetChannel sets the update channel
func (u *Updater) SetChannel(channel string) error {
	return u.state.SetChannel(channel)
}

// GetAutoCheck returns whether auto-check is enabled
func (u *Updater) GetAutoCheck() bool {
	return u.state.GetState().AutoCheck
}

// SetAutoCheck enables/disables auto-check
func (u *Updater) SetAutoCheck(enabled bool) error {
	return u.state.SetAutoCheck(enabled)
}

// CheckForUpdate checks for available updates
func (u *Updater) CheckForUpdate() error {
	u.setStatus(StatusCheckingForUpdate)
	u.lastError = nil
	u.availableUpdate = nil

	channel := u.state.GetState().Channel
	log.Printf("🔍 Checking for updates (channel: %s, current: %s)", channel, u.currentVersion)

	info, err := u.github.CheckForUpdate(u.currentVersion, u.appType, channel)
	if err != nil {
		u.lastError = err
		u.setStatus(StatusError)
		return err
	}

	// Update last check time
	u.state.SetLastCheck(time.Now())

	if info == nil {
		log.Println("✅ Already up to date")
		u.setStatus(StatusUpToDate)
		return nil
	}

	// Check if this version is ignored
	if u.state.IsVersionIgnored(info.TagName) {
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

	// Get download directory
	updateDir, err := GetUpdateDirectory()
	if err != nil {
		u.lastError = err
		u.setStatus(StatusError)
		return err
	}

	// Create version-specific directory
	versionDir := filepath.Join(updateDir, info.TagName)
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		u.lastError = err
		u.setStatus(StatusError)
		return err
	}

	// Download file (platform-specific naming)
	var exePath string
	if runtime.GOOS == "darwin" {
		exePath = filepath.Join(versionDir, fmt.Sprintf("%s-%s.tar.gz", u.appType, info.TagName))
	} else {
		exePath = filepath.Join(versionDir, fmt.Sprintf("%s-%s.exe", u.appType, info.TagName))
	}
	log.Printf("📥 Downloading %s to %s", info.ExeURL, exePath)

	if err := u.downloader.DownloadFile(info.ExeURL, exePath, info.ExeSize); err != nil {
		u.lastError = err
		u.setStatus(StatusError)
		return err
	}

	// Verify SHA256 if available
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

	// Save state
	u.state.SetDownloadedVersion(info.TagName, exePath)

	log.Printf("✅ Download complete and verified")
	u.setStatus(StatusReadyToInstall)
	return nil
}

// InstallUpdate installs the downloaded update
// The new exe is started with --update-from flag, which handles the replacement
func (u *Updater) InstallUpdate() error {
	state := u.state.GetState()
	if state.DownloadPath == "" {
		return fmt.Errorf("no update downloaded")
	}

	// Verify file exists
	if _, err := os.Stat(state.DownloadPath); os.IsNotExist(err) {
		return fmt.Errorf("downloaded file not found: %s", state.DownloadPath)
	}

	u.setStatus(StatusInstalling)

	// Get current executable path
	currentExe, err := os.Executable()
	if err != nil {
		u.lastError = err
		u.setStatus(StatusError)
		return err
	}

	launchPath := state.DownloadPath

	// macOS: extract .tar.gz first, then find the binary inside
	if runtime.GOOS == "darwin" && strings.HasSuffix(state.DownloadPath, ".tar.gz") {
		extractDir := filepath.Dir(state.DownloadPath)
		log.Printf("📦 Extracting macOS update: %s", state.DownloadPath)
		if err := extractTarGz(state.DownloadPath, extractDir); err != nil {
			u.lastError = fmt.Errorf("failed to extract update: %w", err)
			u.setStatus(StatusError)
			return u.lastError
		}
		// Find the .app bundle or binary
		appPath := filepath.Join(extractDir, "Remote Desktop Controller.app", "Contents", "MacOS", "RemoteDesktopController")
		if _, err := os.Stat(appPath); err == nil {
			launchPath = appPath
		} else {
			// Try finding any executable in extract dir
			entries, _ := os.ReadDir(extractDir)
			for _, e := range entries {
				p := filepath.Join(extractDir, e.Name())
				info, _ := os.Stat(p)
				if info != nil && !info.IsDir() && info.Mode()&0111 != 0 && !strings.HasSuffix(p, ".tar.gz") {
					launchPath = p
					break
				}
			}
		}
		log.Printf("📦 Extracted, launching: %s", launchPath)
	}

	args := fmt.Sprintf("--update-from \"%s\"", currentExe)
	log.Printf("🚀 Starting new version with update mode: %s %s", launchPath, args)

	// Launch elevated (runas) — needed when installed in Program Files
	if err := launchElevated(launchPath, args); err != nil {
		u.lastError = err
		u.setStatus(StatusError)
		return err
	}

	// Clear download state
	u.state.ClearDownload()

	log.Println("✅ New version started, exiting for update...")
	return nil
}

// FetchVersionInfo fetches version info from the update server
func (u *Updater) FetchVersionInfo() (*VersionInfo, error) {
	return u.github.FetchVersionInfo()
}

// IgnoreUpdate ignores the current available update
func (u *Updater) IgnoreUpdate() error {
	if u.availableUpdate == nil {
		return nil
	}
	
	if err := u.state.SetIgnoredVersion(u.availableUpdate.TagName); err != nil {
		return err
	}
	
	u.availableUpdate = nil
	u.setStatus(StatusUpToDate)
	return nil
}

// ShouldAutoCheck returns true if auto-check should run
func (u *Updater) ShouldAutoCheck(interval time.Duration) bool {
	return u.state.ShouldCheck(interval)
}

// HasPendingUpdate returns true if there's a downloaded update ready
func (u *Updater) HasPendingUpdate() bool {
	return u.state.HasPendingUpdate()
}

// extractTarGz extracts a .tar.gz archive to the given directory
func extractTarGz(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(dst, hdr.Name)
		// Prevent path traversal
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dst)) {
			continue
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0755)
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			io.Copy(out, tr)
			out.Close()
		}
	}
	return nil
}
