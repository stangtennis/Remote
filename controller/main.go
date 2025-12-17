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
	"github.com/stangtennis/Remote/controller/internal/settings"
	"github.com/stangtennis/Remote/controller/internal/supabase"
	"github.com/stangtennis/Remote/controller/internal/updater"
	"github.com/stangtennis/Remote/controller/internal/viewer"
)

const (
	Version     = "v2.62.16"
	BuildDate   = "2025-12-16"
	VersionInfo = Version + " (" + BuildDate + ")"
)

var (
	supabaseClient        *supabase.Client
	currentUser           *supabase.User
	myApp                 fyne.App
	myWindow              fyne.Window
	appSettings           *settings.Settings
	devicesData           []supabase.Device
	deviceListWidget      *widget.List
	refreshPendingDevices func()
)

func main() {
	// Check for update mode first (before any GUI initialization)
	if len(os.Args) >= 3 && os.Args[1] == "--update-from" {
		runUpdateMode(os.Args[2])
		return
	}

	// Initialize logger first
	if err := logger.Init(); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Close()

	logger.Info("=== Remote Desktop Controller Starting ===")
	logger.Info("Version: %s", VersionInfo)
	logger.Info("Application startup initiated")

	// Load settings
	logger.Info("Loading settings...")
	var err error
	appSettings, err = settings.Load()
	if err != nil {
		logger.Error("Failed to load settings, using defaults: %v", err)
		appSettings = settings.Default()
	}
	logger.Info("Settings loaded: Quality=%s, Resolution=%s, FPS=%d",
		appSettings.GetQualityDescription(), appSettings.MaxResolution, appSettings.TargetFPS)

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

	// Create application with theme from settings
	logger.Info("Creating Fyne application...")
	myApp = app.New()
	if appSettings.Theme == "dark" {
		myApp.Settings().SetTheme(theme.DarkTheme())
	} else {
		myApp.Settings().SetTheme(theme.LightTheme())
	}

	windowTitle := "Remote Desktop Controller " + Version
	if appSettings.HighQualityMode {
		windowTitle += " - High-Performance Mode"
	}
	myWindow = myApp.NewWindow(windowTitle)
	myWindow.Resize(fyne.NewSize(float32(appSettings.WindowWidth), float32(appSettings.WindowHeight)))
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
		"🎮 Remote Desktop Controller "+Version,
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
	var loginButton *widget.Button
	var loginForm *fyne.Container
	var loggedInContainer *fyne.Container
	var pendingDevicesWidget *widget.List
	var pendingDevicesData []supabase.Device

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
		logger.Info("Login button clicked")
		email := emailEntry.Text
		password := passwordEntry.Text

		logger.Info("Email: %s, Password length: %d", email, len(password))

		if email == "" || password == "" {
			statusLabel.SetText("❌ Please enter email and password")
			logger.Info("Empty email or password")
			return
		}

		statusLabel.SetText("🔄 Connecting to Supabase...")
		loginButton.Disable()
		logger.Info("Starting login process...")

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

		// Authenticate with Supabase in background
		go func() {
			logger.Info("Attempting login for user: %s", email)
			authResp, err := supabaseClient.SignIn(email, password)
			if err != nil {
				logger.Error("Login failed for %s: %v", email, err)
				fyne.Do(func() {
					statusLabel.SetText("❌ Login failed: " + err.Error())
					dialog.ShowError(fmt.Errorf("Login failed: %v", err), window)
					loginButton.Enable()
				})
				return
			}

			currentUser = &authResp.User
			logger.Info("✅ Logged in successfully as: %s (ID: %s)", currentUser.Email, currentUser.ID)

			// Check if user is approved
			logger.Info("Checking approval status for user: %s", currentUser.ID)
			approved, err := supabaseClient.CheckApproval(currentUser.ID)
			if err != nil {
				logger.Error("Failed to check approval for user %s: %v", currentUser.ID, err)
				fyne.Do(func() {
					statusLabel.SetText("❌ Failed to check approval")
					dialog.ShowError(fmt.Errorf("Failed to check approval: %v", err), window)
					loginButton.Enable()
				})
				return
			}

			logger.Info("Approval status: %v", approved)
			if !approved {
				logger.Info("User %s is not approved yet", currentUser.Email)
				fyne.Do(func() {
					statusLabel.SetText("⏸️ Account pending approval")
					loginButton.Enable()
				})
				return
			}

			fyne.Do(func() {
				statusLabel.SetText("✅ Connected as: " + currentUser.Email)
			})

			// Fetch devices assigned to this user
			logger.Info("Fetching devices for user: %s", currentUser.ID)
			devices, err := supabaseClient.GetDevices(currentUser.ID)
			if err != nil {
				logger.Error("Failed to fetch devices for user %s: %v", currentUser.ID, err)
				logger.Debug("Device fetch error details: %+v", err)
				fyne.Do(func() {
					statusLabel.SetText("⚠️ Connected but failed to load devices")
				})
			} else {
				logger.Info("✅ Successfully loaded %d assigned devices", len(devices))
				for i, device := range devices {
					logger.Debug("Device %d: Name=%s, ID=%s, Platform=%s, Status=%s",
						i+1, device.DeviceName, device.DeviceID, device.Platform, device.Status)
				}

				devicesData = devices
				fyne.Do(func() {
					if deviceListWidget != nil {
						deviceListWidget.Refresh()
						logger.Debug("Device list widget refreshed with %d devices", len(devicesData))
					} else {
						logger.Error("Device list widget is nil")
					}
					statusLabel.SetText(fmt.Sprintf("✅ Connected: %s (%d devices)", currentUser.Email, len(devices)))
					loginButton.Enable()
					// Hide login form, show logged in view
					loginForm.Hide()
					loggedInContainer.Show()
					// Also refresh pending devices
					refreshPendingDevices()
					// Start periodic device refresh (every 10 seconds)
					startDeviceRefreshTicker()
				})
			}
		}()
	})
	loginButton.Importance = widget.HighImportance

	// Logout button
	logoutButton := widget.NewButton("Logout", func() {
		// Stop device refresh ticker
		stopDeviceRefreshTicker()
		currentUser = nil
		devicesData = nil
		if deviceListWidget != nil {
			deviceListWidget.Refresh()
		}
		statusLabel.SetText("Logged out")
		logger.Info("User logged out")
		// Show login form, hide logged in view
		loginForm.Show()
		loggedInContainer.Hide()
	})

	// Restart button
	restartButton := widget.NewButton("🔄 Genstart", func() {
		dialog.ShowConfirm("Genstart applikation",
			"Er du sikker på at du vil genstarte?",
			func(confirmed bool) {
				if confirmed {
					logger.Info("Restarting application...")
					restartApplication()
				}
			}, window)
	})
	restartButton.Importance = widget.MediumImportance

	// Update button
	updateButton := widget.NewButton("🔄 Tjek opdatering", func() {
		showUpdateDialog(window)
	})
	updateButton.Importance = widget.LowImportance

	loginForm = container.NewVBox(
		widget.NewSeparator(),
		widget.NewLabel("Login to Remote Desktop"),
		emailEntry,
		passwordEntry,
		rememberCheck,
		loginButton,
		statusLabel,
		widget.NewSeparator(),
	)

	// Logged in view (shown after successful login)
	loggedInContainer = container.NewVBox(
		widget.NewSeparator(),
		statusLabel,
		container.NewGridWithColumns(3, logoutButton, restartButton, updateButton),
		widget.NewSeparator(),
	)
	loggedInContainer.Hide() // Hidden by default

	// Device list section with modern styling
	deviceListWidget = widget.NewList(
		func() int {
			return len(devicesData)
		},
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel("Device Name"),
				widget.NewButton("Connect", func() {}),
				widget.NewButton("🗑️ Remove", func() {}),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id >= len(devicesData) {
				return
			}

			device := devicesData[id]

			box := obj.(*fyne.Container)
			label := box.Objects[0].(*widget.Label)
			connectBtn := box.Objects[1].(*widget.Button)
			removeBtn := box.Objects[2].(*widget.Button)

			// Calculate REAL status based on last_seen (not trusting is_online flag)
			// Device is considered online if last_seen is within 2 minutes
			var statusIcon string
			var statusText string
			var isReallyOnline bool

			if !device.LastSeen.IsZero() {
				timeSince := time.Since(device.LastSeen)
				if timeSince < 2*time.Minute {
					// Recently seen = online
					statusIcon = "🟢"
					statusText = "Online"
					isReallyOnline = true
				} else if timeSince < 5*time.Minute {
					// Seen within 5 min = away
					statusIcon = "🟡"
					statusText = fmt.Sprintf("Away (%dm ago)", int(timeSince.Minutes()))
					isReallyOnline = false
				} else {
					// Not seen for 5+ min = offline
					statusIcon = "🔴"
					isReallyOnline = false
					if timeSince < time.Hour {
						mins := int(timeSince.Minutes())
						statusText = fmt.Sprintf("Offline (%dm ago)", mins)
					} else if timeSince < 24*time.Hour {
						hours := int(timeSince.Hours())
						statusText = fmt.Sprintf("Offline (%dh ago)", hours)
					} else {
						days := int(timeSince.Hours() / 24)
						statusText = fmt.Sprintf("Offline (%dd ago)", days)
					}
				}
			} else {
				statusIcon = "🔴"
				statusText = "Offline (never seen)"
				isReallyOnline = false
			}

			displayName := fmt.Sprintf("%s %s (%s) - %s", statusIcon, device.DeviceName, device.Platform, statusText)
			label.SetText(displayName)

			// Configure connect button based on REAL online status
			if !isReallyOnline {
				connectBtn.Disable()
				connectBtn.SetText("Offline")
			} else {
				connectBtn.Enable()
				connectBtn.SetText("Connect")
				connectBtn.Importance = widget.HighImportance
				connectBtn.OnTapped = func() {
					logger.Info("🔗 Initiating connection to device: %s (ID: %s)", device.DeviceName, device.DeviceID)
					logger.Debug("Device details - Platform: %s, Status: %s", device.Platform, device.Status)
					connectToDevice(device)
				}
			}

			// Configure remove button
			removeBtn.SetText("🗑️ Remove")
			removeBtn.Importance = widget.DangerImportance
			removeBtn.OnTapped = func() {
				dialog.ShowConfirm("Remove Device",
					fmt.Sprintf("Remove device '%s' from your account?\n\nThis will unassign the device but not delete it.", device.DeviceName),
					func(confirmed bool) {
						if confirmed && currentUser != nil {
							go func() {
								err := supabaseClient.UnassignDevice(device.DeviceID, currentUser.ID)
								if err != nil {
									logger.Error("Failed to remove device: %v", err)
									fyne.Do(func() {
										dialog.ShowError(fmt.Errorf("Failed to remove device: %v", err), window)
									})
								} else {
									logger.Info("✅ Device removed: %s", device.DeviceName)
									fyne.Do(func() {
										dialog.ShowInformation("Success", "Device removed from your account!", window)
										// Refresh device list
										go func() {
											devices, _ := supabaseClient.GetDevices(currentUser.ID)
											devicesData = devices
											fyne.Do(func() {
												deviceListWidget.Refresh()
											})
										}()
									})
								}
							}()
						}
					}, window)
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

	// Login tab with both login form and logged-in view
	loginTab := container.NewStack(loginForm, loggedInContainer)

	// Pending devices tab for approval
	refreshPendingDevices = func() {
		if currentUser == nil {
			return
		}
		go func() {
			allDevices, err := supabaseClient.GetAllDevices()
			if err != nil {
				logger.Error("Failed to fetch all devices: %v", err)
				return
			}

			// Filter for unassigned devices (owner_id is empty)
			var pending []supabase.Device
			for _, dev := range allDevices {
				if dev.OwnerID == "" {
					pending = append(pending, dev)
				}
			}

			pendingDevicesData = pending
			fyne.Do(func() {
				if pendingDevicesWidget != nil {
					pendingDevicesWidget.Refresh()
					logger.Info("Refreshed pending devices: %d found", len(pending))
				}
			})
		}()
	}

	pendingDevicesWidget = widget.NewList(
		func() int {
			return len(pendingDevicesData)
		},
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel("Device Name"),
				widget.NewButton("✅ Approve", func() {}),
				widget.NewButton("🗑️ Delete", func() {}),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id >= len(pendingDevicesData) {
				return
			}

			device := pendingDevicesData[id]
			box := obj.(*fyne.Container)
			label := box.Objects[0].(*widget.Label)
			approveBtn := box.Objects[1].(*widget.Button)
			deleteBtn := box.Objects[2].(*widget.Button)

			label.SetText(fmt.Sprintf("📱 %s (%s) - ID: %s", device.DeviceName, device.Platform, device.DeviceID))

			// Configure approve button
			approveBtn.SetText("✅ Approve")
			approveBtn.Importance = widget.SuccessImportance
			approveBtn.OnTapped = func() {
				dialog.ShowConfirm("Approve Device",
					fmt.Sprintf("Approve device '%s' and assign it to your account?", device.DeviceName),
					func(confirmed bool) {
						if confirmed && currentUser != nil {
							go func() {
								err := supabaseClient.AssignDevice(device.DeviceID, currentUser.ID)
								if err != nil {
									logger.Error("Failed to approve device: %v", err)
									fyne.Do(func() {
										dialog.ShowError(fmt.Errorf("Failed to approve device: %v", err), window)
									})
								} else {
									logger.Info("✅ Device approved: %s", device.DeviceName)
									fyne.Do(func() {
										dialog.ShowInformation("Success", "Device approved successfully!", window)
										refreshPendingDevices()
										// Also refresh assigned devices
										if deviceListWidget != nil {
											go func() {
												devices, _ := supabaseClient.GetDevices(currentUser.ID)
												devicesData = devices
												fyne.Do(func() {
													deviceListWidget.Refresh()
												})
											}()
										}
									})
								}
							}()
						}
					}, window)
			}

			// Configure delete button
			deleteBtn.SetText("🗑️ Delete")
			deleteBtn.Importance = widget.DangerImportance
			deleteBtn.OnTapped = func() {
				dialog.ShowConfirm("Delete Device",
					fmt.Sprintf("Permanently delete device '%s'?\n\nThis cannot be undone!", device.DeviceName),
					func(confirmed bool) {
						if confirmed {
							go func() {
								err := supabaseClient.DeleteDevice(device.DeviceID)
								if err != nil {
									logger.Error("Failed to delete device: %v", err)
									fyne.Do(func() {
										dialog.ShowError(fmt.Errorf("Failed to delete device: %v", err), window)
									})
								} else {
									logger.Info("✅ Device deleted: %s", device.DeviceName)
									fyne.Do(func() {
										dialog.ShowInformation("Success", "Device permanently deleted!", window)
										refreshPendingDevices()
									})
								}
							}()
						}
					}, window)
			}
		},
	)

	pendingDevicesSection := container.NewBorder(
		container.NewVBox(
			widget.NewLabel("⏳ Pending Devices (Waiting for Approval)"),
			widget.NewButton("🔄 Refresh", func() {
				refreshPendingDevices()
			}),
		),
		nil, nil, nil,
		pendingDevicesWidget,
	)

	// Main layout with tabs
	tabs := container.NewAppTabs(
		container.NewTabItem("Login", loginTab),
		container.NewTabItem("My Devices", deviceSection),
		container.NewTabItem("Approve Devices", pendingDevicesSection),
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

// createSettingsTab creates the comprehensive settings tab
func createSettingsTab() *fyne.Container {
	// Performance Mode Toggle
	highQualityCheck := widget.NewCheck("Enable High-Performance Mode", func(checked bool) {
		appSettings.HighQualityMode = checked
		if checked {
			appSettings.ApplyPreset("ultra")
		}
		settings.Save(appSettings)
		logger.Info("High-performance mode: %v", checked)
	})
	highQualityCheck.Checked = appSettings.HighQualityMode

	// Quality Preset Buttons
	presetUltra := widget.NewButton("Ultra (4K, 60 FPS)", func() {
		appSettings.ApplyPreset("ultra")
		settings.Save(appSettings)
		dialog.ShowInformation("Preset Applied", "Ultra quality preset applied. Restart for full effect.", myWindow)
		logger.Info("Applied Ultra preset")
	})
	presetUltra.Importance = widget.HighImportance

	presetHigh := widget.NewButton("High (1440p, 60 FPS)", func() {
		appSettings.ApplyPreset("high")
		settings.Save(appSettings)
		dialog.ShowInformation("Preset Applied", "High quality preset applied. Restart for full effect.", myWindow)
		logger.Info("Applied High preset")
	})

	presetLow := widget.NewButton("Low (1080p, 30 FPS)", func() {
		appSettings.ApplyPreset("low")
		settings.Save(appSettings)
		dialog.ShowInformation("Preset Applied", "Low quality preset applied. Restart for full effect.", myWindow)
		logger.Info("Applied Low preset")
	})

	// Resolution Selection
	resolutionSelect := widget.NewSelect([]string{"720p", "1080p", "1440p", "4K"}, func(value string) {
		appSettings.MaxResolution = value
		settings.Save(appSettings)
		logger.Info("Resolution changed to: %s", value)
	})
	resolutionSelect.SetSelected(appSettings.MaxResolution)

	// FPS Selection
	fpsSelect := widget.NewSelect([]string{"30", "60", "120"}, func(value string) {
		if value == "30" {
			appSettings.TargetFPS = 30
		} else if value == "60" {
			appSettings.TargetFPS = 60
		} else {
			appSettings.TargetFPS = 120
		}
		settings.Save(appSettings)
		logger.Info("Target FPS changed to: %d", appSettings.TargetFPS)
	})
	fpsSelect.SetSelected(fmt.Sprintf("%d", appSettings.TargetFPS))

	// Video Quality Slider
	qualitySlider := widget.NewSlider(1, 100)
	qualitySlider.Value = float64(appSettings.VideoQuality)
	qualitySlider.Step = 10
	qualityLabel := widget.NewLabel(fmt.Sprintf("Video Quality: %d%%", appSettings.VideoQuality))
	qualitySlider.OnChanged = func(value float64) {
		appSettings.VideoQuality = int(value)
		qualityLabel.SetText(fmt.Sprintf("Video Quality: %d%%", int(value)))
		settings.Save(appSettings)
	}

	// Codec Selection
	codecSelect := widget.NewSelect([]string{"H.264", "H.265", "VP9"}, func(value string) {
		appSettings.Codec = value
		settings.Save(appSettings)
		logger.Info("Codec changed to: %s", value)
	})
	codecSelect.SetSelected(appSettings.Codec)

	// Bitrate Slider
	bitrateSlider := widget.NewSlider(5, 100)
	bitrateSlider.Value = float64(appSettings.MaxBitrate)
	bitrateSlider.Step = 5
	bitrateLabel := widget.NewLabel(fmt.Sprintf("Max Bitrate: %d Mbps", appSettings.MaxBitrate))
	bitrateSlider.OnChanged = func(value float64) {
		appSettings.MaxBitrate = int(value)
		bitrateLabel.SetText(fmt.Sprintf("Max Bitrate: %d Mbps", int(value)))
		settings.Save(appSettings)
	}

	// Feature Toggles
	adaptiveBitrateCheck := widget.NewCheck("Adaptive Bitrate", func(checked bool) {
		appSettings.AdaptiveBitrate = checked
		settings.Save(appSettings)
	})
	adaptiveBitrateCheck.Checked = appSettings.AdaptiveBitrate

	hardwareAccelCheck := widget.NewCheck("Hardware Acceleration", func(checked bool) {
		appSettings.HardwareAcceleration = checked
		settings.Save(appSettings)
	})
	hardwareAccelCheck.Checked = appSettings.HardwareAcceleration

	lowLatencyCheck := widget.NewCheck("Low Latency Mode", func(checked bool) {
		appSettings.LowLatencyMode = checked
		settings.Save(appSettings)
	})
	lowLatencyCheck.Checked = appSettings.LowLatencyMode

	fileTransferCheck := widget.NewCheck("Enable File Transfer", func(checked bool) {
		appSettings.EnableFileTransfer = checked
		settings.Save(appSettings)
	})
	fileTransferCheck.Checked = appSettings.EnableFileTransfer

	clipboardCheck := widget.NewCheck("Enable Clipboard Sync", func(checked bool) {
		appSettings.EnableClipboardSync = checked
		settings.Save(appSettings)
	})
	clipboardCheck.Checked = appSettings.EnableClipboardSync

	audioCheck := widget.NewCheck("Enable Audio Streaming", func(checked bool) {
		appSettings.EnableAudio = checked
		settings.Save(appSettings)
	})
	audioCheck.Checked = appSettings.EnableAudio

	// Theme Selection
	themeSelect := widget.NewSelect([]string{"dark", "light"}, nil)
	themeSelect.SetSelected(appSettings.Theme)

	// Now attach the callback after setting initial value
	themeSelect.OnChanged = func(value string) {
		if value == appSettings.Theme {
			return // No change, don't show dialog
		}
		appSettings.Theme = value
		settings.Save(appSettings)
		dialog.ShowInformation("Theme Changed", "Please restart the application to apply the new theme.", myWindow)
		logger.Info("Theme changed to: %s", value)
	}

	// Reset to Defaults Button
	resetButton := widget.NewButton("Reset to Defaults", func() {
		dialog.ShowConfirm("Reset Settings",
			"Are you sure you want to reset all settings to defaults?",
			func(confirmed bool) {
				if confirmed {
					appSettings = settings.Default()
					settings.Save(appSettings)
					dialog.ShowInformation("Settings Reset", "All settings have been reset. Please restart the application.", myWindow)
					logger.Info("Settings reset to defaults")
				}
			}, myWindow)
	})
	resetButton.Importance = widget.DangerImportance

	// Current Settings Display
	currentSettings := widget.NewLabel(fmt.Sprintf(
		"Current: %s | %s @ %d FPS | Quality: %d%% | Bitrate: %d Mbps",
		appSettings.GetQualityDescription(),
		appSettings.MaxResolution,
		appSettings.TargetFPS,
		appSettings.VideoQuality,
		appSettings.MaxBitrate,
	))
	currentSettings.Wrapping = fyne.TextWrapWord

	// Layout
	return container.NewVBox(
		widget.NewLabelWithStyle("⚙️ Performance Settings", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),

		highQualityCheck,
		widget.NewLabel("Quick Presets:"),
		container.NewGridWithColumns(3, presetUltra, presetHigh, presetLow),

		widget.NewSeparator(),
		widget.NewLabel("Video Settings:"),
		container.NewGridWithColumns(2,
			widget.NewLabel("Resolution:"), resolutionSelect,
			widget.NewLabel("Target FPS:"), fpsSelect,
			widget.NewLabel("Codec:"), codecSelect,
		),
		qualityLabel,
		qualitySlider,

		widget.NewSeparator(),
		widget.NewLabel("Network Settings:"),
		bitrateLabel,
		bitrateSlider,
		adaptiveBitrateCheck,

		widget.NewSeparator(),
		widget.NewLabel("Advanced Options:"),
		hardwareAccelCheck,
		lowLatencyCheck,

		widget.NewSeparator(),
		widget.NewLabel("Features:"),
		fileTransferCheck,
		clipboardCheck,
		audioCheck,

		widget.NewSeparator(),
		widget.NewLabel("Appearance:"),
		container.NewGridWithColumns(2,
			widget.NewLabel("Theme:"), themeSelect,
		),

		widget.NewSeparator(),
		widget.NewLabel("Debugging:"),
		widget.NewButton("📋 View Log", func() {
			showLogViewer(myWindow)
		}),

		widget.NewSeparator(),
		currentSettings,
		resetButton,
	)
}

// showLogViewer shows a dialog with the log contents
func showLogViewer(parent fyne.Window) {
	logContent, err := logger.ReadLog(200) // Last 200 lines
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to read log: %v", err), parent)
		return
	}

	// Create scrollable text area
	logText := widget.NewMultiLineEntry()
	logText.SetText(logContent)
	logText.Wrapping = fyne.TextWrapOff
	logText.Disable() // Read-only

	// Create scroll container
	scroll := container.NewScroll(logText)
	scroll.SetMinSize(fyne.NewSize(800, 500))

	// Scroll to bottom
	logText.CursorRow = len(logContent)

	// Refresh button
	refreshBtn := widget.NewButton("🔄 Refresh", func() {
		newContent, err := logger.ReadLog(200)
		if err == nil {
			logText.SetText(newContent)
		}
	})

	// Copy button
	copyBtn := widget.NewButton("📋 Copy All", func() {
		parent.Clipboard().SetContent(logText.Text)
		dialog.ShowInformation("Copied", "Log copied to clipboard", parent)
	})

	// Open file button
	openBtn := widget.NewButton("📂 Open Log File", func() {
		logPath := logger.GetLogPath()
		var cmd *exec.Cmd
		if os.PathSeparator == '\\' { // Windows
			cmd = exec.Command("notepad", logPath)
		} else {
			cmd = exec.Command("xdg-open", logPath)
		}
		cmd.Start()
	})

	buttons := container.NewHBox(refreshBtn, copyBtn, openBtn)

	content := container.NewBorder(
		widget.NewLabel("📋 Controller Log (last 200 lines)"),
		buttons,
		nil, nil,
		scroll,
	)

	logDialog := dialog.NewCustom("Controller Log", "Close", content, parent)
	logDialog.Resize(fyne.NewSize(850, 600))
	logDialog.Show()
}

// connectToDevice initiates a connection to a remote device with high-quality settings
func connectToDevice(device supabase.Device) {
	logger.Info("🔗 Opening high-performance viewer for: %s", device.DeviceName)

	// Create and show the modern viewer
	v := viewer.NewViewer(myApp, device.DeviceID, device.DeviceName)
	v.Show()

	logger.Info("Viewer window opened for device: %s", device.DeviceID)

	// Initiate WebRTC connection
	if currentUser != nil {
		go func() {
			if err := v.ConnectWebRTC(
				supabaseClient.URL,
				supabaseClient.AnonKey,
				supabaseClient.AuthToken,
				currentUser.ID,
			); err != nil {
				logger.Error("Failed to connect WebRTC: %v", err)
			}
		}()
	}
}

// restartApplication restarts the application
func restartApplication() {
	logger.Info("Restarting application...")

	// Show progress dialog
	progressDialog := dialog.NewCustom("Restarting", "Cancel",
		container.NewVBox(
			widget.NewLabel("Restarting application..."),
			widget.NewProgressBarInfinite(),
		), myWindow)
	progressDialog.Show()

	// Get the current executable path
	executable, err := os.Executable()
	if err != nil {
		logger.Error("Failed to get executable path: %v", err)
		progressDialog.Hide()
		dialog.ShowError(fmt.Errorf("Failed to restart: %v", err), myWindow)
		return
	}

	logger.Info("Executable path: %s", executable)

	// Start a new instance in background
	go func() {
		// Small delay to ensure UI updates
		time.Sleep(500 * time.Millisecond)

		// On Windows, use cmd.exe to start the process detached
		var cmd *exec.Cmd
		if os.PathSeparator == '\\' { // Windows
			// Use cmd /c start to launch detached process that survives parent exit
			cmd = exec.Command("cmd", "/c", "start", "", executable)
		} else { // Unix-like
			cmd = exec.Command(executable)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}

		logger.Info("Starting new instance with command: %v", cmd.Args)

		// Run synchronously to ensure process starts before we exit
		if err := cmd.Run(); err != nil {
			logger.Error("Failed to start new instance: %v", err)
			progressDialog.Hide()
			dialog.ShowError(fmt.Errorf("Failed to restart: %v", err), myWindow)
			return
		}

		logger.Info("New instance started successfully, shutting down current instance")

		// Exit current instance immediately
		myApp.Quit()
	}()
}

// Device refresh ticker for automatic status updates
var deviceRefreshTicker *time.Ticker
var deviceRefreshStop chan bool

func startDeviceRefreshTicker() {
	// Stop any existing ticker
	stopDeviceRefreshTicker()
	
	deviceRefreshTicker = time.NewTicker(5 * time.Second)
	deviceRefreshStop = make(chan bool)
	
	go func() {
		logger.Info("📡 Started device refresh ticker (every 5 seconds)")
		for {
			select {
			case <-deviceRefreshTicker.C:
				if currentUser == nil {
					continue
				}
				// Fetch updated device list
				devices, err := supabaseClient.GetDevices(currentUser.ID)
				if err != nil {
					logger.Debug("Device refresh failed: %v", err)
					continue
				}
				
				// Check if any device changed (count, status, or last_seen)
				if len(devices) != len(devicesData) {
					logger.Info("📡 Device count changed: %d -> %d", len(devicesData), len(devices))
				} else {
					for i, d := range devices {
						if i < len(devicesData) {
							old := devicesData[i]
							// Check status change
							if d.Status != old.Status {
								logger.Info("📡 Device %s status changed: %s -> %s", d.DeviceName, old.Status, d.Status)
								break
							}
							// Check last_seen change (device came online/offline)
							if d.LastSeen != old.LastSeen {
								logger.Debug("📡 Device %s last_seen updated", d.DeviceName)
								break
							}
						}
					}
				}
				
				devicesData = devices
				// Always refresh UI to update "X minutes ago" text
				if deviceListWidget != nil {
					fyne.Do(func() {
						deviceListWidget.Refresh()
					})
				}
				
				// Also refresh pending devices
				refreshPendingDevices()
				
			case <-deviceRefreshStop:
				logger.Info("📡 Device refresh ticker stopped")
				return
			}
		}
	}()
}

func stopDeviceRefreshTicker() {
	if deviceRefreshTicker != nil {
		deviceRefreshTicker.Stop()
		deviceRefreshTicker = nil
	}
	if deviceRefreshStop != nil {
		select {
		case deviceRefreshStop <- true:
		default:
		}
		deviceRefreshStop = nil
	}
}

// showUpdateDialog shows the update check dialog
func showUpdateDialog(window fyne.Window) {
	// Create updater
	u, err := updater.NewUpdater(Version, "controller")
	if err != nil {
		dialog.ShowError(fmt.Errorf("Kunne ikke initialisere opdatering: %v", err), window)
		return
	}

	// Status label
	statusLabel := widget.NewLabel("Klar til at tjekke for opdateringer")
	statusLabel.Wrapping = fyne.TextWrapWord

	// Progress bar (hidden initially)
	progressBar := widget.NewProgressBar()
	progressBar.Hide()

	// Version info
	currentVersionLabel := widget.NewLabel(fmt.Sprintf("Nuværende version: %s", Version))

	// Channel selector
	channelSelect := widget.NewSelect([]string{"stable", "beta"}, func(value string) {
		u.SetChannel(value)
	})
	channelSelect.SetSelected(u.GetChannel())

	// Check button
	var checkBtn, downloadBtn, installBtn *widget.Button

	checkBtn = widget.NewButton("🔍 Tjek for opdateringer", func() {
		checkBtn.Disable()
		statusLabel.SetText("Tjekker for opdateringer...")

		go func() {
			err := u.CheckForUpdate()
			fyne.Do(func() {
				checkBtn.Enable()
				if err != nil {
					statusLabel.SetText(fmt.Sprintf("❌ Fejl: %v", err))
					return
				}

				info := u.GetAvailableUpdate()
				if info == nil {
					statusLabel.SetText("✅ Du har den nyeste version!")
				} else {
					statusLabel.SetText(fmt.Sprintf("🆕 Ny version tilgængelig: %s", info.TagName))
					downloadBtn.Show()
				}
			})
		}()
	})
	checkBtn.Importance = widget.HighImportance

	// Download button (hidden initially)
	downloadBtn = widget.NewButton("📥 Download opdatering", func() {
		downloadBtn.Disable()
		progressBar.Show()
		progressBar.SetValue(0)
		statusLabel.SetText("Downloader...")

		u.SetProgressCallback(func(p updater.DownloadProgress) {
			fyne.Do(func() {
				progressBar.SetValue(p.Percent / 100)
				statusLabel.SetText(fmt.Sprintf("Downloader... %.0f%%", p.Percent))
			})
		})

		go func() {
			err := u.DownloadUpdate()
			fyne.Do(func() {
				progressBar.Hide()
				if err != nil {
					statusLabel.SetText(fmt.Sprintf("❌ Download fejlede: %v", err))
					downloadBtn.Enable()
					return
				}

				statusLabel.SetText("✅ Download færdig! Klar til installation.")
				downloadBtn.Hide()
				installBtn.Show()
			})
		}()
	})
	downloadBtn.Importance = widget.HighImportance
	downloadBtn.Hide()

	// Install button (hidden initially)
	installBtn = widget.NewButton("🚀 Installer og genstart", func() {
		dialog.ShowConfirm("Installer opdatering",
			"Applikationen vil lukke og genstarte med den nye version.\n\nFortsæt?",
			func(confirmed bool) {
				if !confirmed {
					return
				}

				statusLabel.SetText("Installerer...")
				installBtn.Disable()

				go func() {
					err := u.InstallUpdate()
					if err != nil {
						fyne.Do(func() {
							statusLabel.SetText(fmt.Sprintf("❌ Installation fejlede: %v", err))
							installBtn.Enable()
						})
						return
					}

					// Exit app - updater will restart it
					fyne.Do(func() {
						myApp.Quit()
					})
				}()
			}, window)
	})
	installBtn.Importance = widget.DangerImportance
	installBtn.Hide()

	// Layout
	content := container.NewVBox(
		widget.NewLabelWithStyle("🔄 Opdateringer", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		currentVersionLabel,
		container.NewHBox(widget.NewLabel("Kanal:"), channelSelect),
		widget.NewSeparator(),
		statusLabel,
		progressBar,
		widget.NewSeparator(),
		checkBtn,
		downloadBtn,
		installBtn,
	)

	scrollContent := container.NewScroll(content)
	scrollContent.SetMinSize(fyne.NewSize(400, 350))

	dialog.ShowCustom("Opdateringer", "Luk", scrollContent, window)
}

// runUpdateMode runs when started with --update-from flag
// This replaces the old exe and restarts normally
func runUpdateMode(oldExePath string) {
	// Simple logging to file since we can't use the normal logger
	logFile := oldExePath + ".update.log"
	f, _ := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if f != nil {
		defer f.Close()
	}

	log := func(msg string) {
		line := fmt.Sprintf("[%s] %s\n", time.Now().Format("15:04:05"), msg)
		if f != nil {
			f.WriteString(line)
		}
	}

	log("Update mode started")
	log(fmt.Sprintf("Old exe: %s", oldExePath))

	currentExe, err := os.Executable()
	if err != nil {
		log(fmt.Sprintf("ERROR: Failed to get current exe path: %v", err))
		return
	}
	log(fmt.Sprintf("New exe: %s", currentExe))

	// Wait for old exe to exit (max 10 seconds)
	log("Waiting for old exe to exit...")
	for i := 0; i < 100; i++ {
		// Try to open file exclusively
		file, err := os.OpenFile(oldExePath, os.O_RDWR, 0)
		if err == nil {
			file.Close()
			log("Old exe is unlocked")
			break
		}
		if os.IsNotExist(err) {
			log("Old exe already deleted")
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Delete old exe
	log("Deleting old exe...")
	if err := os.Remove(oldExePath); err != nil {
		if !os.IsNotExist(err) {
			log(fmt.Sprintf("WARNING: Failed to delete old exe: %v", err))
		}
	} else {
		log("Old exe deleted")
	}

	// Rename current exe to old exe location (so it's in the right place)
	log(fmt.Sprintf("Moving %s to %s", currentExe, oldExePath))
	
	// Copy instead of rename (works across volumes)
	srcFile, err := os.Open(currentExe)
	if err != nil {
		log(fmt.Sprintf("ERROR: Failed to open source: %v", err))
		return
	}
	
	dstFile, err := os.Create(oldExePath)
	if err != nil {
		srcFile.Close()
		log(fmt.Sprintf("ERROR: Failed to create destination: %v", err))
		return
	}
	
	_, err = dstFile.ReadFrom(srcFile)
	srcFile.Close()
	dstFile.Close()
	
	if err != nil {
		log(fmt.Sprintf("ERROR: Failed to copy: %v", err))
		return
	}
	log("Copy complete")

	// Start the copied exe (now at original location)
	log("Starting controller from original location...")
	cmd := exec.Command(oldExePath)
	if err := cmd.Start(); err != nil {
		log(fmt.Sprintf("ERROR: Failed to start: %v", err))
		return
	}

	log("Update complete!")
	
	// Clean up - delete ourselves (the temp downloaded exe)
	// This won't work on Windows while running, but that's OK
}
