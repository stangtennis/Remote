package main

import (
	_ "embed"
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/stangtennis/Remote/controller/internal/config"
	"github.com/stangtennis/Remote/controller/internal/ui"
	"github.com/stangtennis/Remote/controller/internal/credentials"
	"github.com/stangtennis/Remote/controller/internal/logger"
	"github.com/stangtennis/Remote/controller/internal/settings"
	"github.com/stangtennis/Remote/controller/internal/supabase"
	"github.com/stangtennis/Remote/controller/internal/updater"
	"github.com/stangtennis/Remote/controller/internal/viewer"
)

//go:embed Icon.png
var appIconPNG []byte

// Version information - injected at build time via -ldflags -X
var (
	Version     = "dev"
	BuildDate   = "unknown"
	VersionInfo = ""
)

func init() {
	if VersionInfo == "" {
		VersionInfo = Version + " (built " + BuildDate + ")"
	}
}

var (
	supabaseClient        *supabase.Client
	currentUser           *supabase.User
	myApp                 fyne.App
	myWindow              fyne.Window
	appSettings           *settings.Settings
	devicesData           []supabase.Device
	devicesMu             sync.Mutex
	deviceListWidget      *widget.List
	refreshPendingDevices func()
)

func main() {
	// Check for update mode first (before any GUI initialization)
	if len(os.Args) >= 3 && os.Args[1] == "--update-from" {
		runUpdateMode(os.Args[2])
		return
	}

	// Headless mode — no GUI, direct WebRTC test
	if len(os.Args) >= 2 && os.Args[1] == "--headless" {
		runHeadless()
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
	myApp.SetIcon(fyne.NewStaticResource("icon.png", appIconPNG))
	myApp.Settings().SetTheme(&ui.CyberTheme{})

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

	// Auto-update check in background
	go func() {
		time.Sleep(3 * time.Second) // Vent til UI er klar
		logger.Info("🔍 Auto-update check starting...")
		u, err := updater.NewUpdater(Version, "controller")
		if err != nil {
			logger.Error("Failed to create updater: %v", err)
			return
		}
		if !u.ShouldAutoCheck(5 * time.Minute) {
			logger.Info("Auto-update: skipped (checked within 5 min)")
			return
		}
		if err := u.CheckForUpdate(); err != nil {
			logger.Error("Auto-update check failed: %v", err)
			return
		}
		info := u.GetAvailableUpdate()
		if info == nil {
			logger.Info("✅ Controller is up to date")
			return
		}
		logger.Info("🆕 Update available: %s", info.TagName)
		// Download update
		if err := u.DownloadUpdate(); err != nil {
			logger.Error("Auto-update download failed: %v", err)
			return
		}
		// Prompt user to install
		fyne.Do(func() {
			dialog.ShowConfirm(
				"Opdatering tilgængelig",
				fmt.Sprintf("Version %s er klar til installation.\nGenstart nu?", info.TagName),
				func(ok bool) {
					if ok {
						if err := u.InstallUpdate(); err != nil {
							dialog.ShowError(err, myWindow)
							return
						}
						myApp.Quit()
					}
				},
				myWindow,
			)
		})
	}()

	// Save window size on close
	myWindow.SetOnClosed(func() {
		size := myWindow.Canvas().Size()
		appSettings.WindowWidth = int(size.Width)
		appSettings.WindowHeight = int(size.Height)
		settings.Save(appSettings)
		logger.Info("Window size saved: %dx%d", appSettings.WindowWidth, appSettings.WindowHeight)
	})

	// Show and run
	logger.Info("Launching application window")
	myWindow.ShowAndRun()
	logger.Info("Application shutdown")
}

func createModernUI(window fyne.Window) *fyne.Container {
	// Title with modern styling
	title := widget.NewLabelWithStyle(
		"Remote Desktop Controller "+Version,
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	// Subtitle
	subtitle := widget.NewLabel("High-Performance Remote Control")
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
			statusLabel.SetText("Please enter email and password")
			logger.Info("Empty email or password")
			return
		}

		statusLabel.SetText("Connecting...")
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
					statusLabel.SetText("Login failed: " + err.Error())
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
					statusLabel.SetText("Failed to check approval")
					dialog.ShowError(fmt.Errorf("Failed to check approval: %v", err), window)
					loginButton.Enable()
				})
				return
			}

			logger.Info("Approval status: %v", approved)
			if !approved {
				logger.Info("User %s is not approved yet", currentUser.Email)
				fyne.Do(func() {
					statusLabel.SetText("Account pending approval")
					loginButton.Enable()
				})
				return
			}

			fyne.Do(func() {
				statusLabel.SetText("Connected as: " + currentUser.Email)
			})

			// Fetch devices assigned to this user
			logger.Info("Fetching devices for user: %s", currentUser.ID)
			devices, err := supabaseClient.GetDevices(currentUser.ID)
			if err != nil {
				logger.Error("Failed to fetch devices for user %s: %v", currentUser.ID, err)
				logger.Debug("Device fetch error details: %+v", err)
				fyne.Do(func() {
					statusLabel.SetText("Connected but failed to load devices")
				})
			} else {
				logger.Info("✅ Successfully loaded %d assigned devices", len(devices))
				for i, device := range devices {
					logger.Debug("Device %d: Name=%s, ID=%s, Platform=%s, Status=%s",
						i+1, device.DeviceName, device.DeviceID, device.Platform, device.Status)
				}

				devicesMu.Lock()
				devicesData = devices
				devicesMu.Unlock()
				fyne.Do(func() {
					if deviceListWidget != nil {
						deviceListWidget.Refresh()
						logger.Debug("Device list widget refreshed with %d devices", len(devices))
					} else {
						logger.Error("Device list widget is nil")
					}
					statusLabel.SetText(fmt.Sprintf("Connected: %s (%d devices)", currentUser.Email, len(devices)))
					loginButton.Enable()
					// Hide login form, show logged in view
					loginForm.Hide()
					loggedInContainer.Show()
					// Also refresh pending devices
					refreshPendingDevices()
					// Start periodic device refresh (every 10 seconds)
					startDeviceRefreshTicker()

					// Auto-connect to first online device if AUTO_CONNECT env is set
					if os.Getenv("AUTO_CONNECT") != "" {
						for _, d := range devices {
							if d.Status == "online" {
								logger.Info("🔗 Auto-connecting to: %s", d.DeviceName)
								connectToDevice(d)
								break
							}
						}
					}
				})
			}
		}()
	})
	loginButton.Importance = widget.HighImportance

	// Auto-login if saved credentials exist
	if savedCreds != nil && savedCreds.Remember && savedCreds.Email != "" && savedCreds.Password != "" {
		go func() {
			time.Sleep(500 * time.Millisecond) // Vent til UI er klar
			logger.Info("🔑 Auto-login med gemte credentials for: %s", savedCreds.Email)
			fyne.Do(func() {
				loginButton.Tapped(nil)
			})
		}()
	}

	// Logout button
	logoutButton := widget.NewButton("Logout", func() {
		// Stop device refresh ticker
		stopDeviceRefreshTicker()
		currentUser = nil
		devicesMu.Lock()
		devicesData = nil
		devicesMu.Unlock()
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
	restartButton := widget.NewButtonWithIcon("Genstart", theme.ViewRefreshIcon(), func() {
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
	updateButton := widget.NewButtonWithIcon("Tjek opdatering", theme.ViewRefreshIcon(), func() {
		showUpdateDialog(window)
	})
	updateButton.Importance = widget.LowImportance

	// Install button - dynamic text based on install state
	installBtnText := "Installer"
	if isInstalledAsProgram() {
		installBtnText = "Afinstaller"
	}
	installButton := widget.NewButtonWithIcon(installBtnText, theme.DownloadIcon(), func() {
		showInstallDialog(window)
	})
	installButton.Importance = widget.LowImportance

	// Quick Support button
	quickSupportButton := widget.NewButtonWithIcon("Quick Support", theme.HelpIcon(), func() {
		if currentUser == nil || supabaseClient == nil {
			dialog.ShowInformation("Ikke logget ind", "Du skal være logget ind for at bruge Quick Support.", window)
			return
		}
		go func() {
			session, err := supabaseClient.CreateSupportSession()
			if err != nil {
				logger.Error("Failed to create support session: %v", err)
				fyne.Do(func() {
					dialog.ShowError(fmt.Errorf("Kunne ikke oprette support session: %v", err), window)
				})
				return
			}

			fyne.Do(func() {
				// Show dialog with PIN and link
				pinLabel := widget.NewLabel(fmt.Sprintf("PIN: %s", session.PIN))
				pinLabel.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}

				linkEntry := widget.NewEntry()
				linkEntry.SetText(session.ShareURL)
				linkEntry.Disable()

				copyBtn := widget.NewButtonWithIcon("Kopier link", theme.ContentCopyIcon(), func() {
					window.Clipboard().SetContent(session.ShareURL)
					dialog.ShowInformation("Kopieret", "Link kopieret til udklipsholder!", window)
				})

				openDashboardBtn := widget.NewButton("Åbn i dashboard", func() {
					dashURL := fmt.Sprintf("https://stangtennis.github.io/Remote/dashboard.html?support=%s", session.SessionID)
					openBrowser(dashURL)
				})
				openDashboardBtn.Importance = widget.HighImportance

				content := container.NewVBox(
					widget.NewLabelWithStyle("Quick Support Session", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
					widget.NewSeparator(),
					widget.NewLabel("Del denne PIN eller link med personen:"),
					pinLabel,
					widget.NewSeparator(),
					widget.NewLabel("Delelink:"),
					linkEntry,
					container.NewGridWithColumns(2, copyBtn, openDashboardBtn),
					widget.NewSeparator(),
					widget.NewLabel(fmt.Sprintf("Udløber: %s", session.ExpiresAt)),
				)

				scrollContent := container.NewScroll(content)
				scrollContent.SetMinSize(fyne.NewSize(400, 300))
				dialog.ShowCustom("Quick Support", "Luk", scrollContent, window)
			})
		}()
	})
	quickSupportButton.Importance = widget.WarningImportance

	loginFormContent := container.NewVBox(
		emailEntry,
		passwordEntry,
		rememberCheck,
		loginButton,
		statusLabel,
	)
	loginForm = container.NewCenter(
		widget.NewCard("Login", "Sign in to Remote Desktop", loginFormContent),
	)

	// Logged in view (shown after successful login)
	loggedInContainer = container.NewVBox(
		widget.NewSeparator(),
		statusLabel,
		container.NewGridWithColumns(5, logoutButton, restartButton, updateButton, installButton, quickSupportButton),
		widget.NewSeparator(),
	)
	loggedInContainer.Hide() // Hidden by default

	// Device list section with modern styling
	deviceListWidget = widget.NewList(
		func() int {
			devicesMu.Lock()
			defer devicesMu.Unlock()
			return len(devicesData)
		},
		func() fyne.CanvasObject {
			dot := ui.NewStatusDot(ui.ColorOffline, 10)
			return container.NewHBox(
				dot,
				widget.NewLabel("Device Name"),
				widget.NewButton("Connect", func() {}),
				widget.NewButton("Rename", func() {}),
				widget.NewButtonWithIcon("Remove", theme.DeleteIcon(), func() {}),
				widget.NewButtonWithIcon("Delete", theme.DeleteIcon(), func() {}),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			devicesMu.Lock()
			if id >= len(devicesData) {
				devicesMu.Unlock()
				return
			}
			device := devicesData[id]
			devicesMu.Unlock()

			box := obj.(*fyne.Container)
			dot := box.Objects[0].(*canvas.Circle)
			label := box.Objects[1].(*widget.Label)
			connectBtn := box.Objects[2].(*widget.Button)
			renameBtn := box.Objects[3].(*widget.Button)
			removeBtn := box.Objects[4].(*widget.Button)
			deleteBtn := box.Objects[5].(*widget.Button)

			// Calculate REAL status based on last_seen (not trusting is_online flag)
			var statusColor color.Color
			var statusText string
			var isReallyOnline bool

			if !device.LastSeen.IsZero() {
				timeSince := time.Since(device.LastSeen)
				if timeSince < 2*time.Minute {
					statusColor = ui.ColorOnline
					statusText = "Online"
					isReallyOnline = true
				} else if timeSince < 5*time.Minute {
					statusColor = ui.ColorAway
					statusText = fmt.Sprintf("Away (%dm ago)", int(timeSince.Minutes()))
					isReallyOnline = false
				} else {
					statusColor = ui.ColorOffline
					isReallyOnline = false
					if timeSince < time.Hour {
						statusText = fmt.Sprintf("Offline (%dm ago)", int(timeSince.Minutes()))
					} else if timeSince < 24*time.Hour {
						statusText = fmt.Sprintf("Offline (%dh ago)", int(timeSince.Hours()))
					} else {
						statusText = fmt.Sprintf("Offline (%dd ago)", int(timeSince.Hours()/24))
					}
				}
			} else {
				statusColor = ui.ColorOffline
				statusText = "Offline (never seen)"
				isReallyOnline = false
			}

			dot.FillColor = statusColor
			dot.Refresh()
			versionStr := ""
			if device.AgentVersion != "" {
				versionStr = " " + device.AgentVersion
			}
			label.SetText(fmt.Sprintf("%s (%s%s) - %s", device.DeviceName, device.Platform, versionStr, statusText))

			// Configure connect button based on REAL online status
			if !isReallyOnline {
				connectBtn.Disable()
				connectBtn.SetText("Offline")
			} else {
				connectBtn.Enable()
				connectBtn.SetText("Connect")
				connectBtn.Importance = widget.HighImportance
				connectBtn.OnTapped = func() {
					logger.Info("Initiating connection to device: %s (ID: %s)", device.DeviceName, device.DeviceID)
					connectToDevice(device)
				}
			}

			// Configure rename button
			renameBtn.OnTapped = func() {
				entry := widget.NewEntry()
				entry.SetText(device.DeviceName)
				dialog.ShowForm("Rename Device", "Rename", "Cancel",
					[]*widget.FormItem{
						widget.NewFormItem("New name", entry),
					},
					func(confirmed bool) {
						newName := entry.Text
						if !confirmed || newName == "" || newName == device.DeviceName {
							return
						}
						go func() {
							err := supabaseClient.RenameDevice(device.DeviceID, newName)
							if err != nil {
								logger.Error("Failed to rename device: %v", err)
								fyne.Do(func() {
									dialog.ShowError(fmt.Errorf("Failed to rename device: %v", err), window)
								})
							} else {
								logger.Info("✅ Device renamed: %s → %s", device.DeviceName, newName)
								fyne.Do(func() {
									dialog.ShowInformation("Success", fmt.Sprintf("Device renamed to '%s'!", newName), window)
									go func() {
										devices, _ := supabaseClient.GetDevices(currentUser.ID)
										devicesMu.Lock()
										devicesData = devices
										devicesMu.Unlock()
										fyne.Do(func() {
											deviceListWidget.Refresh()
										})
									}()
								})
							}
						}()
					}, window)
			}

			// Configure remove button
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
											devicesMu.Lock()
											devicesData = devices
											devicesMu.Unlock()
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

			// Configure delete button (permanent deletion)
			deleteBtn.Importance = widget.DangerImportance
			deleteBtn.OnTapped = func() {
				dialog.ShowConfirm("Delete Device Permanently",
					fmt.Sprintf("Permanently delete device '%s'?\n\nThis cannot be undone. The device will need to re-register.", device.DeviceName),
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
									logger.Info("✅ Device permanently deleted: %s", device.DeviceName)
									fyne.Do(func() {
										dialog.ShowInformation("Success", "Device permanently deleted!", window)
										go func() {
											devices, _ := supabaseClient.GetDevices(currentUser.ID)
											devicesMu.Lock()
											devicesData = devices
											devicesMu.Unlock()
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

	deviceSection := widget.NewCard("My Devices", "Manage your connected devices", deviceListWidget)

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
				widget.NewButtonWithIcon("Approve", theme.ConfirmIcon(), func() {}),
				widget.NewButtonWithIcon("Delete", theme.DeleteIcon(), func() {}),
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

			label.SetText(fmt.Sprintf("%s (%s) - ID: %s", device.DeviceName, device.Platform, device.DeviceID))

			// Configure approve button
			approveBtn.SetText("Approve")
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
												devicesMu.Lock()
												devicesData = devices
												devicesMu.Unlock()
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
			deleteBtn.SetText("Delete")
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

	refreshBtn := widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), func() {
		refreshPendingDevices()
	})
	pendingContent := container.NewBorder(refreshBtn, nil, nil, nil, pendingDevicesWidget)
	pendingDevicesSection := widget.NewCard("Pending Devices", "Approve or reject new devices", pendingContent)

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

	hardwareAccelCheck := widget.NewCheck("Hardware Acceleration (coming soon)", func(checked bool) {
		appSettings.HardwareAcceleration = checked
		settings.Save(appSettings)
	})
	hardwareAccelCheck.Checked = appSettings.HardwareAcceleration
	hardwareAccelCheck.Disable() // Not yet implemented

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

	audioCheck := widget.NewCheck("Enable Audio Streaming (coming soon)", func(checked bool) {
		appSettings.EnableAudio = checked
		settings.Save(appSettings)
	})
	audioCheck.Checked = appSettings.EnableAudio
	audioCheck.Disable() // Not yet implemented

	// Theme Selection
	themeSelect := widget.NewSelect([]string{"dark", "light"}, nil)
	themeSelect.SetSelected(appSettings.Theme)

	// Now attach the callback after setting initial value
	themeSelect.OnChanged = func(value string) {
		if value == appSettings.Theme {
			return // No change
		}
		appSettings.Theme = value
		settings.Save(appSettings)
		// Apply theme immediately without restart
		if value == "light" {
			myApp.Settings().SetTheme(theme.LightTheme())
		} else {
			myApp.Settings().SetTheme(&ui.CyberTheme{})
		}
		logger.Info("Theme changed to: %s (applied immediately)", value)
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

	// Layout with Cards
	performanceCard := widget.NewCard("Performance", "", container.NewVBox(
		highQualityCheck,
		widget.NewLabel("Quick Presets:"),
		container.NewGridWithColumns(3, presetUltra, presetHigh, presetLow),
	))

	videoCard := widget.NewCard("Video", "", container.NewVBox(
		container.NewGridWithColumns(2,
			widget.NewLabel("Resolution:"), resolutionSelect,
			widget.NewLabel("Target FPS:"), fpsSelect,
			widget.NewLabel("Codec:"), codecSelect,
		),
		qualityLabel,
		qualitySlider,
	))

	networkCard := widget.NewCard("Network", "", container.NewVBox(
		bitrateLabel,
		bitrateSlider,
		adaptiveBitrateCheck,
	))

	featuresCard := widget.NewCard("Features", "", container.NewVBox(
		hardwareAccelCheck,
		lowLatencyCheck,
		fileTransferCheck,
		clipboardCheck,
		audioCheck,
	))

	viewLogBtn := widget.NewButtonWithIcon("View Log", theme.ContentCopyIcon(), func() {
		showLogViewer(myWindow)
	})

	advancedCard := widget.NewCard("Advanced", "", container.NewVBox(
		container.NewGridWithColumns(2,
			widget.NewLabel("Theme:"), themeSelect,
		),
		viewLogBtn,
		currentSettings,
		resetButton,
	))

	return container.NewVBox(
		performanceCard,
		videoCard,
		networkCard,
		featuresCard,
		advancedCard,
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
	logRefreshBtn := widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), func() {
		newContent, err := logger.ReadLog(200)
		if err == nil {
			logText.SetText(newContent)
		}
	})

	// Copy button
	copyBtn := widget.NewButtonWithIcon("Copy All", theme.ContentCopyIcon(), func() {
		parent.Clipboard().SetContent(logText.Text)
		dialog.ShowInformation("Copied", "Log copied to clipboard", parent)
	})

	// Open file button
	openBtn := widget.NewButtonWithIcon("Open Log File", theme.FolderOpenIcon(), func() {
		logPath := logger.GetLogPath()
		var cmd *exec.Cmd
		if os.PathSeparator == '\\' { // Windows
			cmd = exec.Command("notepad", logPath)
		} else {
			cmd = exec.Command("xdg-open", logPath)
		}
		cmd.Start()
	})

	buttons := container.NewHBox(logRefreshBtn, copyBtn, openBtn)

	content := container.NewBorder(
		widget.NewLabel("Controller Log (last 200 lines)"),
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
	v.SetStreamSettings(appSettings.TargetFPS, appSettings.MaxBitrate)
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
		refreshErrors := 0
		for {
			select {
			case <-deviceRefreshTicker.C:
				if currentUser == nil {
					continue
				}
				// Fetch updated device list
				devices, err := supabaseClient.GetDevices(currentUser.ID)
				if err != nil {
					refreshErrors++
					logger.Debug("Device refresh failed: %v", err)
					// Show notification on first error and every 12th (once per minute)
					if refreshErrors == 1 || refreshErrors%12 == 0 {
						fyne.Do(func() {
							dialog.ShowError(fmt.Errorf("Device refresh failed: %v", err), myWindow)
						})
					}
					continue
				}
				refreshErrors = 0

				devicesMu.Lock()
				// Check if any device changed (count, status, or last_seen)
				if len(devices) != len(devicesData) {
					logger.Info("📡 Device count changed: %d -> %d", len(devicesData), len(devices))
				} else {
					for i, d := range devices {
						if i < len(devicesData) {
							old := devicesData[i]
							if d.Status != old.Status {
								logger.Info("📡 Device %s status changed: %s -> %s", d.DeviceName, old.Status, d.Status)
								break
							}
							if d.LastSeen != old.LastSeen {
								logger.Debug("📡 Device %s last_seen updated", d.DeviceName)
								break
							}
						}
					}
				}
				devicesData = devices
				devicesMu.Unlock()
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

// showUpdateDialog shows the update check dialog with version comparison
func showUpdateDialog(window fyne.Window) {
	// Create updater
	u, err := updater.NewUpdater(Version, "controller")
	if err != nil {
		dialog.ShowError(fmt.Errorf("Kunne ikke initialisere opdatering: %v", err), window)
		return
	}

	// Version list labels (will be populated after check)
	controllerInstalledLabel := widget.NewLabel(fmt.Sprintf("  Installeret:  %s", Version))
	controllerAvailableLabel := widget.NewLabel("  Tilgængelig:  Tjekker...")
	controllerStatusLabel := widget.NewLabel("")
	agentAvailableLabel := widget.NewLabel("  Tilgængelig:  Tjekker...")

	// Status label
	statusLabel := widget.NewLabel("Henter versions-info...")
	statusLabel.Wrapping = fyne.TextWrapWord

	// Progress bar (hidden initially)
	progressBar := widget.NewProgressBar()
	progressBar.Hide()

	// Buttons
	var checkBtn, downloadBtn, installBtn *widget.Button

	checkBtn = widget.NewButtonWithIcon("Tjek igen", theme.SearchIcon(), func() {})
	checkBtn.Hide()

	downloadBtn = widget.NewButtonWithIcon("Download opdatering", theme.DownloadIcon(), func() {})
	downloadBtn.Importance = widget.HighImportance
	downloadBtn.Hide()

	installBtn = widget.NewButtonWithIcon("Installer og genstart", theme.DownloadIcon(), func() {})
	installBtn.Importance = widget.DangerImportance
	installBtn.Hide()

	// Fetch version info immediately
	go func() {
		versionInfo, err := u.FetchVersionInfo()
		fyne.Do(func() {
			if err != nil {
				statusLabel.SetText(fmt.Sprintf("Kunne ikke hente versions-info: %v", err))
				controllerAvailableLabel.SetText("  Tilgængelig:  Fejl")
				agentAvailableLabel.SetText("  Tilgængelig:  Fejl")
				checkBtn.Show()
				return
			}

			// Update version labels
			controllerAvailableLabel.SetText(fmt.Sprintf("  Tilgængelig:  %s", versionInfo.ControllerVersion))
			agentAvailableLabel.SetText(fmt.Sprintf("  Tilgængelig:  %s", versionInfo.AgentVersion))

			// Compare controller versions
			currentCtrl, _ := updater.ParseVersion(Version)
			remoteCtrl, _ := updater.ParseVersion(versionInfo.ControllerVersion)

			if remoteCtrl.IsNewerThan(currentCtrl) {
				controllerStatusLabel.SetText("  NY VERSION TILGÆNGELIG")
				statusLabel.SetText(fmt.Sprintf("Ny controller version: %s → %s", Version, versionInfo.ControllerVersion))
				downloadBtn.Show()
			} else {
				controllerStatusLabel.SetText("  Opdateret")
				statusLabel.SetText("Du har den nyeste version!")
			}
			checkBtn.Show()
		})
	}()

	// Wire up check button
	checkBtn.OnTapped = func() {
		checkBtn.Disable()
		statusLabel.SetText("Tjekker for opdateringer...")
		controllerAvailableLabel.SetText("  Tilgængelig:  Tjekker...")
		controllerStatusLabel.SetText("")
		agentAvailableLabel.SetText("  Tilgængelig:  Tjekker...")
		downloadBtn.Hide()

		go func() {
			versionInfo, err := u.FetchVersionInfo()
			if err != nil {
				fyne.Do(func() {
					checkBtn.Enable()
					statusLabel.SetText(fmt.Sprintf("Fejl: %v", err))
				})
				return
			}

			// Also run the actual update check
			_ = u.CheckForUpdate()

			fyne.Do(func() {
				checkBtn.Enable()
				controllerAvailableLabel.SetText(fmt.Sprintf("  Tilgængelig:  %s", versionInfo.ControllerVersion))
				agentAvailableLabel.SetText(fmt.Sprintf("  Tilgængelig:  %s", versionInfo.AgentVersion))

				currentCtrl, _ := updater.ParseVersion(Version)
				remoteCtrl, _ := updater.ParseVersion(versionInfo.ControllerVersion)

				if remoteCtrl.IsNewerThan(currentCtrl) {
					controllerStatusLabel.SetText("  NY VERSION TILGÆNGELIG")
					statusLabel.SetText(fmt.Sprintf("Ny controller version: %s → %s", Version, versionInfo.ControllerVersion))
					downloadBtn.Show()
				} else {
					controllerStatusLabel.SetText("  Opdateret")
					statusLabel.SetText("Du har den nyeste version!")
				}
			})
		}()
	}

	// Wire up download button
	downloadBtn.OnTapped = func() {
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
			// Ensure update check has run
			if u.GetAvailableUpdate() == nil {
				u.CheckForUpdate()
			}

			err := u.DownloadUpdate()
			fyne.Do(func() {
				progressBar.Hide()
				if err != nil {
					statusLabel.SetText(fmt.Sprintf("Download fejlede: %v", err))
					downloadBtn.Enable()
					return
				}

				statusLabel.SetText("Download færdig! Klar til installation.")
				downloadBtn.Hide()
				installBtn.Show()
			})
		}()
	}

	// Wire up install button
	installBtn.OnTapped = func() {
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
							statusLabel.SetText(fmt.Sprintf("Installation fejlede: %v", err))
							installBtn.Enable()
						})
						return
					}

					fyne.Do(func() {
						myApp.Quit()
					})
				}()
			}, window)
	}

	// Layout - version comparison list
	content := container.NewVBox(
		widget.NewLabelWithStyle("Opdateringer", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),

		widget.NewLabelWithStyle("Controller (denne app):", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		controllerInstalledLabel,
		controllerAvailableLabel,
		controllerStatusLabel,

		widget.NewSeparator(),

		widget.NewLabelWithStyle("Agent:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		agentAvailableLabel,

		widget.NewSeparator(),
		statusLabel,
		progressBar,
		widget.NewSeparator(),
		checkBtn,
		downloadBtn,
		installBtn,
	)

	scrollContent := container.NewScroll(content)
	scrollContent.SetMinSize(fyne.NewSize(420, 400))

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

