package main

import (
	"fmt"
	"log"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/stangtennis/Remote/controller/internal/config"
	"github.com/stangtennis/Remote/controller/internal/logger"
	"github.com/stangtennis/Remote/controller/internal/supabase"
)

var (
	supabaseClient *supabase.Client
	currentUser    *supabase.User
)

func main() {
	// Initialize logger first
	if err := logger.Init(); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
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

	// Create application
	logger.Info("Creating Fyne application...")
	myApp := app.New()
	myWindow := myApp.NewWindow("Remote Desktop Controller")
	myWindow.Resize(fyne.NewSize(800, 600))
	logger.Info("Application window created")

	// Create UI
	logger.Info("Building user interface...")
	content := createMainUI(myWindow)
	myWindow.SetContent(content)
	logger.Info("UI initialized successfully")

	// Show and run
	logger.Info("Launching application window")
	myWindow.ShowAndRun()
	logger.Info("Application shutdown")
}

func createMainUI(window fyne.Window) *fyne.Container {
	// Title
	title := widget.NewLabel("🎮 Remote Desktop Controller")
	title.TextStyle = fyne.TextStyle{Bold: true}

	// Login section
	emailEntry := widget.NewEntry()
	emailEntry.SetPlaceHolder("Email")
	
	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetPlaceHolder("Password")

	statusLabel := widget.NewLabel("Not connected")
	
	// Device list (will be populated after login)
	var deviceListWidget *widget.List
	var devicesData []supabase.Device
	var loginButton *widget.Button

	loginButton = widget.NewButton("Login", func() {
		email := emailEntry.Text
		password := passwordEntry.Text
		
		if email == "" || password == "" {
			statusLabel.SetText("❌ Please enter email and password")
			return
		}

		statusLabel.SetText("🔄 Connecting to Supabase...")
		loginButton.Disable()
		
		// Authenticate with Supabase
		go func() {
			logger.Info("Attempting login for user: %s", email)
			authResp, err := supabaseClient.SignIn(email, password)
			if err != nil {
				logger.Error("Login failed for %s: %v", email, err)
				// Update UI on main thread
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
				// Update UI on main thread
				window.Canvas().Content().Refresh()
				statusLabel.SetText("❌ Failed to check approval")
				loginButton.Enable()
				return
			}

			logger.Info("Approval status: %v", approved)
			if !approved {
				logger.Info("User %s is not approved yet", currentUser.Email)
				// Update UI on main thread
				window.Canvas().Content().Refresh()
				statusLabel.SetText("⏸️ Account pending approval")
				loginButton.Enable()
				return
			}

			// Update UI on main thread
			window.Canvas().Content().Refresh()
			statusLabel.SetText("✅ Connected as: " + currentUser.Email)
			
			// Fetch devices assigned to this user
			logger.Info("Fetching devices for user: %s", currentUser.ID)
			devices, err := supabaseClient.GetDevices(currentUser.ID)
			if err != nil {
				logger.Error("Failed to fetch devices for user %s: %v", currentUser.ID, err)
				logger.Debug("Device fetch error details: %+v", err)
				// Update UI on main thread
				window.Canvas().Content().Refresh()
				statusLabel.SetText("⚠️ Connected but failed to load devices")
			} else {
				logger.Info("✅ Successfully loaded %d assigned devices", len(devices))
				for i, device := range devices {
					logger.Debug("Device %d: Name=%s, ID=%s, Platform=%s, Status=%s", 
						i+1, device.DeviceName, device.DeviceID, device.Platform, device.Status)
				}
				
				// Update device list and UI on main thread
				devicesData = devices
				if deviceListWidget != nil {
					window.Canvas().Content().Refresh()
					deviceListWidget.Refresh()
					logger.Debug("Device list widget refreshed with %d devices", len(devicesData))
				} else {
					logger.Error("Device list widget is nil")
				}
				
				// Update status with device count
				statusLabel.SetText(fmt.Sprintf("✅ Connected: %s (%d devices)", currentUser.Email, len(devices)))
			}
			
			window.Canvas().Content().Refresh()
			loginButton.Enable()
		}()
	})

	loginForm := container.NewVBox(
		widget.NewLabel("Login to Remote Desktop"),
		emailEntry,
		passwordEntry,
		loginButton,
		statusLabel,
	)

	// Device list section (real data from Supabase)
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
			
			// Disable button for offline devices
			if device.Status != "online" {
				button.Disable()
			} else {
				button.Enable()
				button.OnTapped = func() {
					log.Printf("Connecting to device: %s (%s)", device.DeviceName, device.DeviceID)
					// TODO: Implement WebRTC connection
					connectToDevice(device)
				}
			}
		},
	)

	deviceSection := container.NewBorder(
		widget.NewLabel("📱 Available Devices"),
		nil,
		nil,
		nil,
		deviceListWidget,
	)

	// Main layout with tabs
	tabs := container.NewAppTabs(
		container.NewTabItem("Login", loginForm),
		container.NewTabItem("Devices", deviceSection),
		container.NewTabItem("Settings", widget.NewLabel("Settings coming soon...")),
	)

	return container.NewBorder(
		title,
		widget.NewLabel("Status: Ready"),
		nil,
		nil,
		tabs,
	)
}

// connectToDevice initiates a connection to a remote device
func connectToDevice(device supabase.Device) {
	logger.Info("🔗 Initiating connection to device: %s (ID: %s)", device.DeviceName, device.DeviceID)
	logger.Debug("Device details - Platform: %s, Status: %s", device.Platform, device.Status)
	
	// TODO: Implement WebRTC viewer window
	// For now, show a dialog
	dialog.ShowInformation(
		"Connecting...",
		fmt.Sprintf("Connecting to:\n\n"+
			"Device: %s\n"+
			"Platform: %s\n"+
			"ID: %s\n\n"+
			"WebRTC viewer coming soon!",
			device.DeviceName,
			device.Platform,
			device.DeviceID),
		nil,
	)
}
