package main

import (
	"context"
	"fmt"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/stangtennis/Remote/controller/internal/config"
	"github.com/stangtennis/Remote/controller/internal/credentials"
	"github.com/stangtennis/Remote/controller/internal/logger"
	"github.com/stangtennis/Remote/controller/internal/settings"
	"github.com/stangtennis/Remote/controller/internal/supabase"
	"github.com/stangtennis/Remote/controller/internal/updater"
)

// App struct — methods are exposed to frontend via Wails bindings
type App struct {
	ctx            context.Context
	cfg            *config.Config
	supabase       *supabase.Client
	currentUser    *supabase.User
	appSettings    *settings.Settings
	deviceTicker   *time.Ticker
	deviceTickStop chan bool
}

// NewApp creates a new App instance
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Initialize logger
	if err := logger.Init(); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
	}

	logger.Info("=== Remote Desktop Controller Starting (Wails) ===")
	logger.Info("Version: %s", VersionInfo)

	// Load settings
	var err error
	a.appSettings, err = settings.Load()
	if err != nil {
		logger.Error("Failed to load settings, using defaults: %v", err)
		a.appSettings = settings.Default()
	}

	// Load config
	a.cfg, err = config.Load()
	if err != nil {
		logger.Error("Failed to load config: %v", err)
		return
	}

	// Initialize Supabase client
	a.supabase = supabase.NewClient(a.cfg.SupabaseURL, a.cfg.SupabaseAnonKey)
	logger.Info("Supabase client initialized")

	// Auto-update check
	go a.autoUpdateCheck()
}

// shutdown is called when the app closes
func (a *App) shutdown(ctx context.Context) {
	a.stopDeviceRefresh()
	if a.appSettings != nil {
		settings.Save(a.appSettings)
	}
	logger.Info("Application shutdown")
	logger.Close()
}

// ==================== AUTH ====================

// AuthResult is returned from Login
type AuthResult struct {
	Email    string `json:"email"`
	UserID   string `json:"user_id"`
	Approved bool   `json:"approved"`
}

// Login authenticates with email and password
func (a *App) Login(email, password string) (*AuthResult, error) {
	logger.Info("Login attempt for: %s", email)

	authResp, err := a.supabase.SignIn(email, password)
	if err != nil {
		logger.Error("Login failed: %v", err)
		return nil, fmt.Errorf("Login mislykkedes: %v", err)
	}

	a.currentUser = &authResp.User
	logger.Info("Logged in as: %s (ID: %s)", a.currentUser.Email, a.currentUser.ID)

	// Check approval
	approved, err := a.supabase.CheckApproval(a.currentUser.ID)
	if err != nil {
		logger.Error("Approval check failed: %v", err)
		return &AuthResult{Email: a.currentUser.Email, UserID: a.currentUser.ID, Approved: false}, nil
	}

	if approved {
		// Start device refresh ticker
		a.startDeviceRefresh()
	}

	return &AuthResult{
		Email:    a.currentUser.Email,
		UserID:   a.currentUser.ID,
		Approved: approved,
	}, nil
}

// Logout signs out the current user
func (a *App) Logout() {
	a.stopDeviceRefresh()
	a.currentUser = nil
	logger.Info("User logged out")
}

// CredentialsInfo holds saved credentials info
type CredentialsInfo struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Remember bool   `json:"remember"`
}

// LoadCredentials loads saved credentials
func (a *App) LoadCredentials() *CredentialsInfo {
	creds, err := credentials.Load()
	if err != nil || creds == nil {
		return nil
	}
	return &CredentialsInfo{
		Email:    creds.Email,
		Password: creds.Password,
		Remember: creds.Remember,
	}
}

// SaveCredentials saves user credentials
func (a *App) SaveCredentials(email, password string, remember bool) error {
	if !remember {
		return credentials.Delete()
	}
	return credentials.Save(&credentials.Credentials{
		Email:    email,
		Password: password,
		Remember: true,
	})
}

// ==================== DEVICES ====================

// DeviceInfo is a frontend-friendly device representation
type DeviceInfo struct {
	DeviceID     string `json:"device_id"`
	DeviceName   string `json:"device_name"`
	Platform     string `json:"platform"`
	OwnerID      string `json:"owner_id"`
	Status       string `json:"status"`
	AgentVersion string `json:"agent_version"`
	LastSeen     string `json:"last_seen"`
	TimeSince    string `json:"time_since"`
	IsOnline     bool   `json:"is_online"`
	IsAway       bool   `json:"is_away"`
}

func deviceToInfo(d supabase.Device) DeviceInfo {
	info := DeviceInfo{
		DeviceID:     d.DeviceID,
		DeviceName:   d.DeviceName,
		Platform:     d.Platform,
		OwnerID:      d.OwnerID,
		AgentVersion: d.AgentVersion,
	}

	if d.LastSeen.IsZero() {
		info.Status = "offline"
		info.TimeSince = "never"
		info.LastSeen = ""
	} else {
		info.LastSeen = d.LastSeen.Format(time.RFC3339)
		since := time.Since(d.LastSeen)
		if since < 2*time.Minute {
			info.Status = "online"
			info.IsOnline = true
			info.TimeSince = "now"
		} else if since < 5*time.Minute {
			info.Status = "away"
			info.IsAway = true
			info.TimeSince = fmt.Sprintf("%dm ago", int(since.Minutes()))
		} else {
			info.Status = "offline"
			if since < time.Hour {
				info.TimeSince = fmt.Sprintf("%dm ago", int(since.Minutes()))
			} else if since < 24*time.Hour {
				info.TimeSince = fmt.Sprintf("%dh ago", int(since.Hours()))
			} else {
				info.TimeSince = fmt.Sprintf("%dd ago", int(since.Hours()/24))
			}
		}
	}

	return info
}

// GetDevices returns devices assigned to the current user
func (a *App) GetDevices() ([]DeviceInfo, error) {
	if a.currentUser == nil {
		return nil, fmt.Errorf("not logged in")
	}
	devices, err := a.supabase.GetDevices(a.currentUser.ID)
	if err != nil {
		return nil, err
	}
	result := make([]DeviceInfo, len(devices))
	for i, d := range devices {
		result[i] = deviceToInfo(d)
	}
	return result, nil
}

// GetPendingDevices returns devices without an owner
func (a *App) GetPendingDevices() ([]DeviceInfo, error) {
	if a.currentUser == nil {
		return nil, fmt.Errorf("not logged in")
	}
	allDevices, err := a.supabase.GetAllDevices()
	if err != nil {
		return nil, err
	}
	var pending []DeviceInfo
	for _, d := range allDevices {
		if d.OwnerID == "" {
			pending = append(pending, deviceToInfo(d))
		}
	}
	return pending, nil
}

// ApproveDevice assigns a device to the current user
func (a *App) ApproveDevice(deviceID string) error {
	if a.currentUser == nil {
		return fmt.Errorf("not logged in")
	}
	return a.supabase.AssignDevice(deviceID, a.currentUser.ID)
}

// RenameDevice updates a device's name
func (a *App) RenameDevice(deviceID, newName string) error {
	return a.supabase.RenameDevice(deviceID, newName)
}

// RemoveDevice unassigns a device from the current user
func (a *App) RemoveDevice(deviceID string) error {
	if a.currentUser == nil {
		return fmt.Errorf("not logged in")
	}
	return a.supabase.UnassignDevice(deviceID, a.currentUser.ID)
}

// DeleteDevice permanently deletes a device
func (a *App) DeleteDevice(deviceID string) error {
	return a.supabase.DeleteDevice(deviceID)
}

// ==================== CONNECTION CONFIG ====================

// ConnectionConfig provides everything the frontend needs for WebRTC
type ConnectionConfig struct {
	SupabaseURL  string `json:"supabase_url"`
	AnonKey      string `json:"anon_key"`
	AuthToken    string `json:"auth_token"`
	UserID       string `json:"user_id"`
	RefreshToken string `json:"refresh_token"`
}

// GetConnectionConfig returns config for frontend WebRTC connections
func (a *App) GetConnectionConfig() (*ConnectionConfig, error) {
	if a.currentUser == nil || a.supabase == nil {
		return nil, fmt.Errorf("not logged in")
	}
	return &ConnectionConfig{
		SupabaseURL:  a.supabase.URL,
		AnonKey:      a.supabase.AnonKey,
		AuthToken:    a.supabase.AuthToken,
		UserID:       a.currentUser.ID,
		RefreshToken: a.supabase.RefreshTok,
	}, nil
}

// ==================== SETTINGS ====================

// GetSettings returns current app settings
func (a *App) GetSettings() *settings.Settings {
	if a.appSettings == nil {
		a.appSettings = settings.Default()
	}
	return a.appSettings
}

// SaveSettings saves app settings
func (a *App) SaveSettings(s *settings.Settings) error {
	a.appSettings = s
	return settings.Save(s)
}

// ApplyPreset applies a quality preset
func (a *App) ApplyPreset(preset string) *settings.Settings {
	a.appSettings.ApplyPreset(preset)
	settings.Save(a.appSettings)
	return a.appSettings
}

// ==================== SYSTEM ====================

// ToggleFullscreen toggles between fullscreen and windowed mode
func (a *App) ToggleFullscreen() {
	if a.ctx == nil {
		return
	}
	if runtime.WindowIsFullscreen(a.ctx) {
		runtime.WindowUnfullscreen(a.ctx)
	} else {
		runtime.WindowFullscreen(a.ctx)
	}
}

// GetVersion returns the current version
func (a *App) GetVersion() string {
	return Version
}

// UpdateInfo holds update information
type UpdateInfo struct {
	Available          bool   `json:"available"`
	ControllerVersion  string `json:"controller_version"`
	AgentVersion       string `json:"agent_version"`
	CurrentVersion     string `json:"current_version"`
}

// CheckForUpdate checks for available updates
func (a *App) CheckForUpdate() (*UpdateInfo, error) {
	u, err := updater.NewUpdater(Version, "controller")
	if err != nil {
		return nil, err
	}

	versionInfo, err := u.FetchVersionInfo()
	if err != nil {
		return nil, err
	}

	currentCtrl, _ := updater.ParseVersion(Version)
	remoteCtrl, _ := updater.ParseVersion(versionInfo.ControllerVersion)

	return &UpdateInfo{
		Available:         remoteCtrl.IsNewerThan(currentCtrl),
		ControllerVersion: versionInfo.ControllerVersion,
		AgentVersion:      versionInfo.AgentVersion,
		CurrentVersion:    Version,
	}, nil
}

// DownloadAndInstallUpdate downloads and installs the update
func (a *App) DownloadAndInstallUpdate() error {
	u, err := updater.NewUpdater(Version, "controller")
	if err != nil {
		return err
	}

	if err := u.CheckForUpdate(); err != nil {
		return err
	}

	// Emit progress events
	u.SetProgressCallback(func(p updater.DownloadProgress) {
		runtime.EventsEmit(a.ctx, "update-progress", p.Percent)
	})

	if err := u.DownloadUpdate(); err != nil {
		return err
	}

	return u.InstallUpdate()
}

// InstallController installs the controller to Program Files / Applications
func (a *App) InstallController() error {
	if !isAdmin() {
		return fmt.Errorf("administrator rettigheder kræves")
	}
	err := installControllerAsProgram()
	if err != nil {
		return err
	}
	// Create shortcuts
	targetExe := getInstalledExePath()
	createStartMenuShortcut(targetExe)
	createDesktopShortcut(targetExe)
	return nil
}

// UninstallController removes the controller installation
func (a *App) UninstallController() error {
	if !isAdmin() {
		return fmt.Errorf("administrator rettigheder kræves")
	}
	return uninstallControllerProgram()
}

// IsInstalled checks if the controller is installed
func (a *App) IsInstalled() bool {
	return isInstalledAsProgram()
}

// IsAdmin checks if running with admin/root privileges
func (a *App) IsAdmin() bool {
	return isAdmin()
}

// RunAsAdmin restarts with elevated privileges
func (a *App) RunAsAdmin() {
	runAsAdmin()
}

// RestartApp restarts the application
func (a *App) RestartApp() {
	restartApplication()
}

// SupportInfo holds support session info
type SupportInfo struct {
	SessionID string `json:"session_id"`
	PIN       string `json:"pin"`
	ShareURL  string `json:"share_url"`
	ExpiresAt string `json:"expires_at"`
}

// CreateSupportSession creates a Quick Support session
func (a *App) CreateSupportSession() (*SupportInfo, error) {
	if a.currentUser == nil || a.supabase == nil {
		return nil, fmt.Errorf("not logged in")
	}
	session, err := a.supabase.CreateSupportSession()
	if err != nil {
		return nil, err
	}
	return &SupportInfo{
		SessionID: session.SessionID,
		PIN:       session.PIN,
		ShareURL:  session.ShareURL,
		ExpiresAt: session.ExpiresAt,
	}, nil
}

// GetLogContent returns recent log entries
func (a *App) GetLogContent(lines int) (string, error) {
	if lines <= 0 {
		lines = 200
	}
	return logger.ReadLog(lines)
}

// ==================== DEVICE REFRESH ====================

func (a *App) startDeviceRefresh() {
	a.stopDeviceRefresh()
	a.deviceTicker = time.NewTicker(5 * time.Second)
	a.deviceTickStop = make(chan bool)

	go func() {
		for {
			select {
			case <-a.deviceTicker.C:
				if a.currentUser == nil {
					continue
				}
				devices, err := a.supabase.GetDevices(a.currentUser.ID)
				if err != nil {
					logger.Debug("Device refresh failed: %v", err)
					continue
				}
				result := make([]DeviceInfo, len(devices))
				for i, d := range devices {
					result[i] = deviceToInfo(d)
				}
				runtime.EventsEmit(a.ctx, "devices-updated", result)
			case <-a.deviceTickStop:
				return
			}
		}
	}()
}

func (a *App) stopDeviceRefresh() {
	if a.deviceTicker != nil {
		a.deviceTicker.Stop()
		a.deviceTicker = nil
	}
	if a.deviceTickStop != nil {
		select {
		case a.deviceTickStop <- true:
		default:
		}
		a.deviceTickStop = nil
	}
}

// ==================== AUTO-UPDATE ====================

func (a *App) autoUpdateCheck() {
	time.Sleep(3 * time.Second)
	logger.Info("Auto-update check starting...")

	u, err := updater.NewUpdater(Version, "controller")
	if err != nil {
		logger.Error("Failed to create updater: %v", err)
		return
	}

	if !u.ShouldAutoCheck(5 * time.Minute) {
		logger.Info("Auto-update: skipped (checked recently)")
		return
	}

	if err := u.CheckForUpdate(); err != nil {
		logger.Error("Auto-update check failed: %v", err)
		return
	}

	info := u.GetAvailableUpdate()
	if info == nil {
		logger.Info("Controller is up to date")
		return
	}

	logger.Info("Update available: %s", info.TagName)
	runtime.EventsEmit(a.ctx, "update-available", info.TagName)
}
