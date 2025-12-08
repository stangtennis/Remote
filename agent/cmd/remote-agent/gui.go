//go:build windows
// +build windows

package main

import (
	"fmt"
	"image/color"
	"log"
	"net/url"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/stangtennis/remote-agent/internal/auth"
	"github.com/stangtennis/remote-agent/internal/config"
	"github.com/stangtennis/remote-agent/internal/tray"
)

// Custom dark theme
type darkTheme struct{}

func (d *darkTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.NRGBA{R: 15, G: 23, B: 42, A: 255} // Dark blue-gray
	case theme.ColorNameButton:
		return color.NRGBA{R: 99, G: 102, B: 241, A: 255} // Indigo
	case theme.ColorNameDisabledButton:
		return color.NRGBA{R: 71, G: 85, B: 105, A: 255}
	case theme.ColorNameForeground:
		return color.NRGBA{R: 248, G: 250, B: 252, A: 255} // Light text
	case theme.ColorNameHover:
		return color.NRGBA{R: 79, G: 70, B: 229, A: 255} // Darker indigo
	case theme.ColorNameInputBackground:
		return color.NRGBA{R: 30, G: 41, B: 59, A: 255}
	case theme.ColorNamePlaceHolder:
		return color.NRGBA{R: 148, G: 163, B: 184, A: 255}
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 99, G: 102, B: 241, A: 255}
	case theme.ColorNameScrollBar:
		return color.NRGBA{R: 71, G: 85, B: 105, A: 255}
	case theme.ColorNameShadow:
		return color.NRGBA{R: 0, G: 0, B: 0, A: 100}
	case theme.ColorNameOverlayBackground:
		return color.NRGBA{R: 30, G: 41, B: 59, A: 255} // Dialog background
	case theme.ColorNameMenuBackground:
		return color.NRGBA{R: 30, G: 41, B: 59, A: 255}
	case theme.ColorNameInputBorder:
		return color.NRGBA{R: 71, G: 85, B: 105, A: 255}
	default:
		return theme.DefaultTheme().Color(name, variant)
	}
}

func (d *darkTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (d *darkTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (d *darkTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 8
	case theme.SizeNameInlineIcon:
		return 24
	case theme.SizeNameText:
		return 14
	case theme.SizeNameHeadingText:
		return 24
	default:
		return theme.DefaultTheme().Size(name)
	}
}

// GUI Application
type AgentGUI struct {
	app              fyne.App
	window           fyne.Window
	statusLabel      *widget.Label
	userLabel        *widget.Label
	serviceLabel     *widget.Label
	actionButtons    *fyne.Container
	isLoggedIn       bool
	userEmail        string
	serviceRunning   bool
	serviceInstalled bool
}

func NewAgentGUI() *AgentGUI {
	// Setup logging early so we can debug GUI issues
	if err := setupLogging(); err != nil {
		fmt.Printf("Warning: Failed to setup logging: %v\n", err)
	} else {
		fmt.Println("✅ Logging setup successful")
	}
	log.Println("🖥️ Starting GUI mode...")
	flushLog() // Force flush immediately

	a := app.New()
	a.Settings().SetTheme(&darkTheme{})

	gui := &AgentGUI{
		app: a,
	}

	log.Println("📱 Creating main window...")
	gui.window = a.NewWindow("Remote Desktop Agent")
	gui.window.Resize(fyne.NewSize(450, 500))
	gui.window.SetFixedSize(true)
	gui.window.CenterOnScreen()

	return gui
}

func (g *AgentGUI) Run() {
	g.refreshStatus()
	g.buildUI()
	g.window.ShowAndRun()
}

func (g *AgentGUI) refreshStatus() {
	g.isLoggedIn = auth.IsLoggedIn()
	if g.isLoggedIn {
		if creds, err := auth.GetCurrentUser(); err == nil {
			g.userEmail = creds.Email
		}
	} else {
		g.userEmail = ""
	}

	g.serviceInstalled = isServiceInstalled()
	if g.serviceInstalled {
		g.serviceRunning = isServiceRunning()
	} else {
		g.serviceRunning = false
	}
}

func (g *AgentGUI) buildUI() {
	// Header with logo
	title := canvas.NewText("🖥️ Remote Desktop Agent", color.White)
	title.TextSize = 24
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignCenter

	version := canvas.NewText("Version "+tray.Version, color.NRGBA{R: 148, G: 163, B: 184, A: 255})
	version.TextSize = 12
	version.Alignment = fyne.TextAlignCenter

	header := container.NewVBox(
		layout.NewSpacer(),
		title,
		version,
		layout.NewSpacer(),
	)

	// Status card
	g.userLabel = widget.NewLabel("")
	g.serviceLabel = widget.NewLabel("")
	g.updateStatusLabels()

	statusCard := container.NewVBox(
		widget.NewSeparator(),
		container.NewHBox(
			widget.NewIcon(theme.AccountIcon()),
			g.userLabel,
		),
		container.NewHBox(
			widget.NewIcon(theme.ComputerIcon()),
			g.serviceLabel,
		),
		widget.NewSeparator(),
	)

	// Action buttons
	g.actionButtons = container.NewVBox()
	g.updateActionButtons()

	// Footer with links
	githubLink := widget.NewHyperlink("GitHub Releases", parseURL("https://github.com/stangtennis/Remote/releases"))
	dashboardLink := widget.NewHyperlink("Web Dashboard", parseURL("https://stangtennis.github.io/Remote"))

	footer := container.NewHBox(
		layout.NewSpacer(),
		githubLink,
		widget.NewLabel(" | "),
		dashboardLink,
		layout.NewSpacer(),
	)

	// Main layout
	content := container.NewVBox(
		header,
		widget.NewSeparator(),
		statusCard,
		g.actionButtons,
		layout.NewSpacer(),
		footer,
	)

	// Add padding
	padded := container.NewPadded(content)
	g.window.SetContent(padded)
}

func (g *AgentGUI) updateStatusLabels() {
	if g.isLoggedIn {
		g.userLabel.SetText("Logged in as: " + g.userEmail)
	} else {
		g.userLabel.SetText("Not logged in")
	}

	if g.serviceRunning {
		g.serviceLabel.SetText("Service: ✅ Running")
	} else if g.serviceInstalled {
		g.serviceLabel.SetText("Service: ⏸️ Stopped")
	} else {
		g.serviceLabel.SetText("Service: ❌ Not installed")
	}
}

func (g *AgentGUI) updateActionButtons() {
	g.actionButtons.Objects = nil

	if !g.isLoggedIn {
		// Login button
		loginBtn := widget.NewButton("🔑  Login", g.showLoginDialog)
		loginBtn.Importance = widget.HighImportance
		g.actionButtons.Add(loginBtn)
	} else {
		// Service management buttons
		if !g.serviceInstalled {
			installBtn := widget.NewButton("📦  Install as Service", g.doInstall)
			installBtn.Importance = widget.HighImportance
			g.actionButtons.Add(installBtn)

			runOnceBtn := widget.NewButton("▶️  Run Once (This Session)", g.doRunOnce)
			g.actionButtons.Add(runOnceBtn)
		} else if !g.serviceRunning {
			startBtn := widget.NewButton("▶️  Start Service", g.doStart)
			startBtn.Importance = widget.HighImportance
			g.actionButtons.Add(startBtn)

			uninstallBtn := widget.NewButton("🗑️  Uninstall Service", g.doUninstall)
			g.actionButtons.Add(uninstallBtn)
		} else {
			stopBtn := widget.NewButton("⏹️  Stop Service", g.doStop)
			g.actionButtons.Add(stopBtn)

			uninstallBtn := widget.NewButton("🗑️  Uninstall Service", g.doUninstall)
			g.actionButtons.Add(uninstallBtn)
		}

		g.actionButtons.Add(widget.NewSeparator())

		// Always show these when logged in
		updateBtn := widget.NewButton("🔄  Check for Updates", g.doCheckUpdates)
		g.actionButtons.Add(updateBtn)

		logoutBtn := widget.NewButton("🚪  Logout / Switch Account", g.doLogout)
		g.actionButtons.Add(logoutBtn)
	}

	g.actionButtons.Add(widget.NewSeparator())

	// Exit button
	exitBtn := widget.NewButton("❌  Exit", func() {
		g.app.Quit()
	})
	g.actionButtons.Add(exitBtn)

	g.actionButtons.Refresh()
}

func (g *AgentGUI) showLoginDialog() {
	emailEntry := widget.NewEntry()
	emailEntry.SetPlaceHolder("your@email.com")

	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetPlaceHolder("Password")

	form := widget.NewForm(
		widget.NewFormItem("Email", emailEntry),
		widget.NewFormItem("Password", passwordEntry),
	)

	d := dialog.NewCustomConfirm("Login", "Login", "Cancel", form, func(ok bool) {
		if !ok {
			return
		}

		email := emailEntry.Text
		password := passwordEntry.Text

		if email == "" || password == "" {
			dialog.ShowError(fmt.Errorf("Please enter email and password"), g.window)
			return
		}

		// Show progress
		progress := dialog.NewCustomWithoutButtons("Logging in...", widget.NewProgressBarInfinite(), g.window)
		progress.Show()

		go func() {
			log.Printf("🔐 Login attempt for: %s", email)
			
			cfg, err := config.Load()
			if err != nil {
				log.Printf("❌ Failed to load config: %v", err)
				fyne.Do(func() {
					progress.Hide()
					dialog.ShowError(fmt.Errorf("Failed to load config: %v", err), g.window)
				})
				return
			}

			log.Printf("📡 Connecting to: %s", cfg.SupabaseURL)
			authConfig := auth.AuthConfig{
				SupabaseURL: cfg.SupabaseURL,
				AnonKey:     cfg.SupabaseAnonKey,
			}

			result, err := auth.Login(authConfig, email, password)
			fyne.Do(func() {
				progress.Hide()
			})

			if err != nil {
				log.Printf("❌ Login error: %v", err)
				fyne.Do(func() {
					dialog.ShowError(fmt.Errorf("Login failed: %v", err), g.window)
				})
				return
			}

			if !result.Success {
				log.Printf("❌ Login failed: %s", result.Message)
				fyne.Do(func() {
					dialog.ShowError(fmt.Errorf(result.Message), g.window)
				})
				return
			}

			// Success!
			log.Printf("✅ Login successful: %s", result.Email)
			flushLog() // Ensure login success is written to log file
			fyne.Do(func() {
				dialog.ShowInformation("Login Successful", 
					"✅ Welcome, "+result.Email+"!\n\n"+
					"You can now install the Remote Desktop Agent as a Windows service.\n\n"+
					"The service will:\n"+
					"• Start automatically with Windows\n"+
					"• Allow remote access to this computer\n"+
					"• Work even on the login screen", g.window)

				g.refreshStatus()
				log.Printf("📊 After refresh - isLoggedIn: %v, email: %s", g.isLoggedIn, g.userEmail)
				flushLog()
				g.updateStatusLabels()
				g.updateActionButtons()
			})
		}()
	}, g.window)

	d.Resize(fyne.NewSize(350, 200))
	d.Show()
}

func (g *AgentGUI) doInstall() {
	if !isAdmin() {
		dialog.ShowConfirm("Administrator Required",
			"Installing as a service requires Administrator privileges.\n\nRestart as Administrator?",
			func(ok bool) {
				if ok {
					runAsAdmin()
					g.app.Quit()
				}
			}, g.window)
		return
	}

	progress := dialog.NewCustomWithoutButtons("Installing service...", widget.NewProgressBarInfinite(), g.window)
	progress.Show()

	go func() {
		err := installService()
		if err != nil {
			fyne.Do(func() {
				progress.Hide()
				dialog.ShowError(fmt.Errorf("Failed to install: %v", err), g.window)
			})
			return
		}

		err = startService()
		fyne.Do(func() {
			progress.Hide()

			if err != nil {
				dialog.ShowError(fmt.Errorf("Installed but failed to start: %v", err), g.window)
			} else {
				dialog.ShowInformation("Installation Complete",
					"✅ Service installed and started!\n\n"+
						"The Remote Desktop Agent is now running.\n\n"+
						"• Starts automatically with Windows\n"+
						"• Works on login screen\n"+
						"• Run this app to manage the service", g.window)
			}

			g.refreshStatus()
			g.updateStatusLabels()
			g.updateActionButtons()
		})
	}()
}

func (g *AgentGUI) doUninstall() {
	dialog.ShowConfirm("Uninstall Service", "Are you sure you want to uninstall the service?", func(ok bool) {
		if !ok {
			return
		}

		if !isAdmin() {
			dialog.ShowConfirm("Administrator Required",
				"Uninstalling requires Administrator privileges.\n\nRestart as Administrator?",
				func(ok bool) {
					if ok {
						runAsAdmin()
						g.app.Quit()
					}
				}, g.window)
			return
		}

		progress := dialog.NewCustomWithoutButtons("Uninstalling...", widget.NewProgressBarInfinite(), g.window)
		progress.Show()

		go func() {
			if g.serviceRunning {
				stopService()
				time.Sleep(time.Second)
			}

			err := uninstallService()
			fyne.Do(func() {
				progress.Hide()

				if err != nil {
					dialog.ShowError(fmt.Errorf("Failed to uninstall: %v", err), g.window)
				} else {
					dialog.ShowInformation("Uninstall Complete", 
						"✅ Service uninstalled successfully.\n\n"+
						"The Remote Desktop Agent is no longer running as a service.", g.window)
				}

				g.refreshStatus()
				g.updateStatusLabels()
				g.updateActionButtons()
			})
		}()
	}, g.window)
}

func (g *AgentGUI) doStart() {
	progress := dialog.NewCustomWithoutButtons("Starting service...", widget.NewProgressBarInfinite(), g.window)
	progress.Show()

	go func() {
		err := startService()
		fyne.Do(func() {
			progress.Hide()

			if err != nil {
				dialog.ShowError(fmt.Errorf("Failed to start: %v", err), g.window)
			} else {
				dialog.ShowInformation("Service Started", 
					"✅ Service started successfully!\n\n"+
					"The Remote Desktop Agent is now running and ready for connections.", g.window)
			}

			g.refreshStatus()
			g.updateStatusLabels()
			g.updateActionButtons()
		})
	}()
}

func (g *AgentGUI) doStop() {
	progress := dialog.NewCustomWithoutButtons("Stopping service...", widget.NewProgressBarInfinite(), g.window)
	progress.Show()

	go func() {
		err := stopService()
		fyne.Do(func() {
			progress.Hide()

			if err != nil {
				dialog.ShowError(fmt.Errorf("Failed to stop: %v", err), g.window)
			} else {
				dialog.ShowInformation("Service Stopped", 
					"✅ Service stopped successfully.\n\n"+
					"Remote connections are no longer possible until the service is started again.", g.window)
			}

			g.refreshStatus()
			g.updateStatusLabels()
			g.updateActionButtons()
		})
	}()
}

func (g *AgentGUI) doRunOnce() {
	// Check if logged in first
	if !auth.IsLoggedIn() {
		dialog.ShowError(fmt.Errorf("Please login first before running the agent"), g.window)
		return
	}

	log.Println("🚀 Starting Run Once mode...")

	// Setup firewall rules
	setupFirewallRules()

	// Load current user credentials
	currentUser, _ = auth.GetCurrentUser()
	log.Printf("✅ Running as: %s", currentUser.Email)

	// Start agent
	if err := startAgent(); err != nil {
		log.Printf("❌ Failed to start agent: %v", err)
		dialog.ShowError(fmt.Errorf("Failed to start agent: %v", err), g.window)
		return
	}

	log.Println("✅ Agent started successfully")

	// Update GUI to show running status
	g.statusLabel.SetText("🟢 Agent Running")
	g.serviceLabel.SetText("Mode: Interactive (Run Once)")
	
	// Disable Run Once button, enable a Stop button
	g.actionButtons.RemoveAll()
	
	stopBtn := widget.NewButton("⏹️  Stop Agent", func() {
		log.Println("🛑 Stopping agent...")
		stopAgent()
		g.statusLabel.SetText("🔴 Agent Stopped")
		g.serviceLabel.SetText("Mode: Stopped")
		g.updateActionButtons()
		dialog.ShowInformation("Agent Stopped", "The agent has been stopped.", g.window)
	})
	stopBtn.Importance = widget.DangerImportance
	g.actionButtons.Add(stopBtn)
	
	exitBtn := widget.NewButton("❌  Exit", func() {
		stopAgent()
		g.app.Quit()
	})
	g.actionButtons.Add(exitBtn)
	
	g.actionButtons.Refresh()

	dialog.ShowInformation("Agent Started", 
		"✅ Agent is now running!\n\n"+
		"You can minimize this window.\n"+
		"Click 'Stop Agent' to stop.", g.window)
}

func (g *AgentGUI) doCheckUpdates() {
	dialog.ShowInformation("Check for Updates",
		"Current version: "+tray.Version+"\n\n"+
			"To update:\n"+
			"1. Download latest from GitHub Releases\n"+
			"2. Stop the service (if running)\n"+
			"3. Replace this exe\n"+
			"4. Start the service again", g.window)
}

func (g *AgentGUI) doLogout() {
	dialog.ShowConfirm("Logout", "Are you sure you want to logout?", func(ok bool) {
		if !ok {
			return
		}

		if err := auth.ClearCredentials(); err != nil {
			dialog.ShowError(fmt.Errorf("Failed to logout: %v", err), g.window)
		} else {
			dialog.ShowInformation("Success", "✅ Logged out successfully!", g.window)
		}

		g.refreshStatus()
		g.updateStatusLabels()
		g.updateActionButtons()
	}, g.window)
}

func parseURL(urlStr string) *url.URL {
	u, _ := url.Parse(urlStr)
	return u
}

// showGUI shows the GUI installer
func showGUI() {
	gui := NewAgentGUI()
	gui.Run()
}
