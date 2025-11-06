package main

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/stangtennis/Remote/controller/internal/config"
	"github.com/stangtennis/Remote/controller/internal/credentials"
	"github.com/stangtennis/Remote/controller/internal/logger"
	"github.com/stangtennis/Remote/controller/internal/supabase"
	"github.com/stangtennis/Remote/controller/internal/viewer"
)

var (
	supabaseClient *supabase.Client
	currentUser    *supabase.User
	myApp          fyne.App
	myWindow       fyne.Window
)

func main() {
	// Initialize logger first
	if err := logger.Init(); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Close()

	logger.Info("=== Remote Desktop Controller Starting ===")
	logger.Info("Application startup initiated")

	// Load configuration
	logger.Info("Loading configuration...")
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Failed to load config: %v", err)
	}
	logger.Info("Configuration loaded successfully")
	logger.Debug("Supabase URL: %s", cfg.SupabaseURL)
	logger.Debug("Supabase Key length: %d characters", len(cfg.SupabaseAnonKey))

	// Initialize Supabase client
	logger.Info("Initializing Supabase client...")
	supabaseClient = supabase.NewClient(cfg.SupabaseURL, cfg.SupabaseAnonKey)
	if supabaseClient == nil {
		logger.Fatal("Failed to create Supabase client: client is nil")
	}
	logger.Info("✅ Supabase client initialized successfully")

	// Create application with modern theme
	logger.Info("Creating Fyne application...")
	myApp = app.New()
	myApp.Settings().SetTheme(theme.DarkTheme())
	
	myWindow = myApp.NewWindow("Remote Desktop Controller - Modern Edition")
	myWindow.Resize(fyne.NewSize(1200, 800))
	myWindow.CenterOnScreen()
	logger.Info("Application window created")

	// Create UI
	logger.Info("Building user interface...")
	content := createModernUI(myWindow)
	myWindow.SetContent(content)
	logger.Info("UI initialized successfully")

	// Show and run
	logger.Info("Launching application window")
	myWindow.ShowAndRun()
	logger.Info("Application shutdown")
}

func createModernUI(window fyne.Window) *fyne.Container {
	// Title with modern styling
	title := widget.NewLabelWithStyle(
		"🎮 Remote Desktop Controller",
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)
	title.TextStyle.Bold = true

	// Subtitle
	subtitle := widget.NewLabel("High-Performance Remote Control for Powerful Computers")
	subtitle.Alignment = fyne.TextAlignCenter

	// Login section
	emailEntry := widget.NewEntry()
	emailEntry.SetPlaceHolder("Email Address")
	
	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetPlaceHolder("Password")

	// Remember me checkbox
	rememberCheck := widget.NewCheck("Remember Me", nil)
	
	statusLabel := widget.NewLabel("Ready to connect")
	statusLabel.Alignment = fyne.TextAlignCenter
	
	// Device list (will be populated after login)
	var deviceListWidget *widget.List
	var devicesData []supabase.Device
	var loginButton *widget.Button

	// Try to load saved credentials
	savedCreds, err := credentials.Load()
	if err == nil && savedCreds != nil && savedCreds.Remember {
		emailEntry.SetText(savedCreds.Email)
		passwordEntry.SetText(savedCreds.Password)
		rememberCheck.Checked = true
		logger.Info("Loaded saved credentials for: %s", savedCreds.Email)
	}

	// Login button
	loginButton = widget.NewButton("Login", func() {
		email := emailEntry.Text
		password := passwordEntry.Text
		
		if email == "" || password == "" {
			statusLabel.SetText("❌ Please enter email and password")
			return
		}

		statusLabel.SetText("🔄 Connecting to Supabase...")
		loginButton.Disable()
		
		// Save credentials if remember me is checked
		if rememberCheck.Checked {
			creds := &credentials.Credentials{
				Email:    email,
				Password: password,
				Remember: true,
			}
			if err := credentials.Save(creds); err != nil {
				logger.Error("Failed to save credentials: %v", err)
			}
		} else {
			credentials.Delete()
		}
		
		// Authenticate with Supabase
		go func() {
			logger.Info("Attempting login for user: %s", email)
			authResp, err := supabaseClient.SignIn(email, password)
			if err != nil {
				logger.Error("Login failed for %s: %v", email, err)
				window.Canvas().Content().Refresh()
				statusLabel.SetText("❌ Login failed: " + err.Error())
				loginButton.Enable()
				return
			}

			currentUser = &authResp.User
			logger.Info("✅ Logged in successfully as: %s (ID: %s)", currentUser.Email, currentUser.ID)

			// Check if user is approved
			logger.Info("Checking approval status for user: %s", currentUser.ID)
			approved, err := supabaseClient.CheckApproval(currentUser.ID)
			if err != nil {
				logger.Error("Failed to check approval for user %s: %v", currentUser.ID, err)
				window.Canvas().Content().Refresh()
				statusLabel.SetText("❌ Failed to check approval")
				loginButton.Enable()
				return
			}

			logger.Info("Approval status: %v", approved)
			if !approved {
				logger.Info("User %s is not approved yet", currentUser.Email)
				window.Canvas().Content().Refresh()
				statusLabel.SetText("⏸️ Account pending approval")
				loginButton.Enable()
				return
			}

			window.Canvas().Content().Refresh()
			statusLabel.SetText("✅ Connected as: " + currentUser.Email)
			
			// Fetch devices assigned to this user
			logger.Info("Fetching devices for user: %s", currentUser.ID)
			devices, err := supabaseClient.GetDevices(currentUser.ID)
			if err != nil {
				logger.Error("Failed to fetch devices for user %s: %v", currentUser.ID, err)
				logger.Debug("Device fetch error details: %+v", err)
				window.Canvas().Content().Refresh()
				statusLabel.SetText("⚠️ Connected but failed to load devices")
			} else {
				logger.Info("✅ Successfully loaded %d assigned devices", len(devices))
				for i, device := range devices {
					logger.Debug("Device %d: Name=%s, ID=%s, Platform=%s, Status=%s", 
						i+1, device.DeviceName, device.DeviceID, device.Platform, device.Status)
				}
				
				devicesData = devices
				if deviceListWidget != nil {
					window.Canvas().Content().Refresh()
					deviceListWidget.Refresh()
					logger.Debug("Device list widget refreshed with %d devices", len(devicesData))
				} else {
					logger.Error("Device list widget is nil")
				}
				
				statusLabel.SetText(fmt.Sprintf("✅ Connected: %s (%d devices)", currentUser.Email, len(devices)))
			}
			
			window.Canvas().Content().Refresh()
			loginButton.Enable()
		}()
	})
	loginButton.Importance = widget.HighImportance

	// Logout button
	logoutButton := widget.NewButton("Logout", func() {
		currentUser = nil
		devicesData = nil
		if deviceListWidget != nil {
			deviceListWidget.Refresh()
		}
		statusLabel.SetText("Logged out")
		logger.Info("User logged out")
	})

	// Restart button
	restartButton := widget.NewButton("🔄 Restart App", func() {
		dialog.ShowConfirm("Restart Application", 
			"Are you sure you want to restart the application?",
			func(confirmed bool) {
				if confirmed {
					logger.Info("Restarting application...")
					restartApplication()
				}
			}, window)
	})
	restartButton.Importance = widget.MediumImportance

	loginForm := container.NewVBox(
		widget.NewSeparator(),
		widget.NewLabel("Login to Remote Desktop"),
		emailEntry,
		passwordEntry,
		rememberCheck,
		container.NewGridWithColumns(2, loginButton, logoutButton),
		statusLabel,
		widget.NewSeparator(),
	)

	// Device list section with modern styling
	deviceListWidget = widget.NewList(
		func() int {
			return len(devicesData)
		},
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel("Device Name"),
				widget.NewButton("Connect", func() {}),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id >= len(devicesData) {
				return
			}
			
			device := devicesData[id]
			
			box := obj.(*fyne.Container)
			label := box.Objects[0].(*widget.Label)
			button := box.Objects[1].(*widget.Button)
			
			// Format device name with status indicator and time info
			var statusIcon string
			var statusText string
			
			if device.Status == "online" {
				statusIcon = "🟢"
				statusText = "Online"
			} else if device.Status == "away" {
				statusIcon = "🟡"
				statusText = "Away"
			} else {
				statusIcon = "🔴"
				// Calculate time since last seen
				if !device.LastSeen.IsZero() {
					timeSince := time.Since(device.LastSeen)
					if timeSince < time.Minute {
						statusText = "Offline (just now)"
					} else if timeSince < time.Hour {
						mins := int(timeSince.Minutes())
						statusText = fmt.Sprintf("Offline (%dm ago)", mins)
					} else if timeSince < 24*time.Hour {
						hours := int(timeSince.Hours())
						statusText = fmt.Sprintf("Offline (%dh ago)", hours)
					} else {
						days := int(timeSince.Hours() / 24)
						statusText = fmt.Sprintf("Offline (%dd ago)", days)
					}
				} else {
					statusText = "Offline"
				}
			}
			
			displayName := fmt.Sprintf("%s %s (%s) - %s", statusIcon, device.DeviceName, device.Platform, statusText)
			label.SetText(displayName)
			
			// Configure connect button
			if device.Status != "online" {
				button.Disable()
				button.SetText("Offline")
			} else {
				button.Enable()
				button.SetText("Connect")
				button.Importance = widget.HighImportance
				button.OnTapped = func() {
					logger.Info("🔗 Initiating connection to device: %s (ID: %s)", device.DeviceName, device.DeviceID)
					logger.Debug("Device details - Platform: %s, Status: %s", device.Platform, device.Status)
					connectToDevice(device)
				}
			}
		},
	)

	deviceSection := container.NewBorder(
		widget.NewLabel("📱 Available Devices (High-Performance Mode)"),
		nil,
		nil,
		nil,
		deviceListWidget,
	)

	// Main layout with tabs
	tabs := container.NewAppTabs(
		container.NewTabItem("Login", loginForm),
		container.NewTabItem("Devices", deviceSection),
		container.NewTabItem("Settings", createSettingsTab()),
	)

	// Top bar with restart button
	topBar := container.NewBorder(
		nil, nil,
		title,
		restartButton,
		subtitle,
	)

	return container.NewBorder(
		topBar,
		widget.NewLabel("Status: Ready | High-Performance Mode Enabled"),
		nil,
		nil,
		tabs,
	)
}

// createSettingsTab creates the settings tab
func createSettingsTab() *fyne.Container {
	qualityLabel := widget.NewLabel("Video Quality: Ultra (Best for powerful computers)")
	fpsLabel := widget.NewLabel("Target FPS: 60")
	resolutionLabel := widget.NewLabel("Max Resolution: 4K (3840x2160)")
	codecLabel := widget.NewLabel("Codec: H.264 High Profile")
	
	return container.NewVBox(
		widget.NewLabel("⚙️ High-Performance Settings"),
		widget.NewSeparator(),
		qualityLabel,
		fpsLabel,
		resolutionLabel,
		codecLabel,
		widget.NewSeparator(),
		widget.NewLabel("These settings are optimized for powerful computers"),
		widget.NewLabel("to provide the best possible remote desktop experience."),
	)
}

// connectToDevice initiates a connection to a remote device with high-quality settings
func connectToDevice(device supabase.Device) {
	logger.Info("🔗 Opening high-performance viewer for: %s", device.DeviceName)
	
	// Create and show the modern viewer
	v := viewer.NewViewer(myApp, device.DeviceID, device.DeviceName)
	v.Show()
	
	logger.Info("Viewer window opened for device: %s", device.DeviceID)
}

// restartApplication restarts the application
func restartApplication() {
	logger.Info("Restarting application...")
	
	// Get the current executable path
	executable, err := os.Executable()
	if err != nil {
		logger.Error("Failed to get executable path: %v", err)
		dialog.ShowError(fmt.Errorf("Failed to restart: %v", err), myWindow)
		return
	}
	
	// Start a new instance
	cmd := exec.Command(executable)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Start(); err != nil {
		logger.Error("Failed to start new instance: %v", err)
		dialog.ShowError(fmt.Errorf("Failed to restart: %v", err), myWindow)
		return
	}
	
	logger.Info("New instance started, shutting down current instance")
	
	// Exit current instance
	myApp.Quit()
}
