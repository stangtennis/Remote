//go:build windows
// +build windows

package main

import (
	"fmt"
	"image/color"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
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
	"github.com/stangtennis/remote-agent/internal/state"
	"github.com/stangtennis/remote-agent/internal/tray"
	"github.com/stangtennis/remote-agent/internal/updater"
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
		fmt.Println("Logging setup successful")
	}
	log.Println("Starting GUI mode...")
	flushLog()

	a := app.New()
	a.Settings().SetTheme(&darkTheme{})

	gui := &AgentGUI{
		app: a,
	}

	log.Println("Creating main window...")
	gui.window = a.NewWindow("Remote Desktop Agent")
	gui.window.Resize(fyne.NewSize(480, 560))
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
	// Header with accent bar
	accentBar := canvas.NewRectangle(color.NRGBA{R: 99, G: 102, B: 241, A: 255})
	accentBar.SetMinSize(fyne.NewSize(4, 0))

	title := canvas.NewText("Remote Desktop Agent", color.White)
	title.TextSize = 22
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignLeading

	subtitle := canvas.NewText("WebRTC-baseret fjernadgang · v"+tray.Version, color.NRGBA{R: 148, G: 163, B: 184, A: 255})
	subtitle.TextSize = 11
	subtitle.Alignment = fyne.TextAlignLeading

	header := container.NewBorder(
		nil, nil,
		container.NewHBox(accentBar),
		nil,
		container.NewPadded(container.NewVBox(title, subtitle)),
	)

	// Status card
	g.userLabel = widget.NewLabel("")
	g.serviceLabel = widget.NewLabel("")
	g.statusLabel = widget.NewLabel("")
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
		container.NewHBox(
			widget.NewIcon(theme.InfoIcon()),
			g.statusLabel,
		),
		widget.NewSeparator(),
	)

	// Action buttons
	g.actionButtons = container.NewVBox()
	g.updateActionButtons()

	// Footer with links
	githubLink := widget.NewHyperlink("GitHub releases", parseURL("https://github.com/stangtennis/Remote/releases"))
	dashboardLink := widget.NewHyperlink("Dashboard", parseURL("https://dashboard.hawkeye123.dk"))
	helpLink := widget.NewHyperlink("Hjælp", parseURL("https://github.com/stangtennis/Remote#readme"))

	footerSep := canvas.NewText("·", color.NRGBA{R: 100, G: 116, B: 139, A: 255})
	footerSep2 := canvas.NewText("·", color.NRGBA{R: 100, G: 116, B: 139, A: 255})
	footer := container.NewHBox(
		layout.NewSpacer(),
		dashboardLink,
		widget.NewLabel("  "), footerSep, widget.NewLabel("  "),
		githubLink,
		widget.NewLabel("  "), footerSep2, widget.NewLabel("  "),
		helpLink,
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

	padded := container.NewPadded(content)
	g.window.SetContent(padded)
}

func (g *AgentGUI) updateStatusLabels() {
	if g.isLoggedIn {
		g.userLabel.SetText("Logget ind: " + g.userEmail)
	} else {
		g.userLabel.SetText("Ikke logget ind")
	}

	if g.serviceRunning {
		g.serviceLabel.SetText("Service: Kører")
	} else if g.serviceInstalled {
		g.serviceLabel.SetText("Service: Stoppet")
	} else if isProgramInstalled() {
		g.serviceLabel.SetText("Program: Installeret (autostart)")
	} else {
		g.serviceLabel.SetText("Tilstand: Ikke installeret")
	}

	// Status dot + text
	if g.serviceRunning {
		g.statusLabel.SetText("Agent kører og er klar til forbindelser")
	} else if !g.isLoggedIn {
		g.statusLabel.SetText("Log ind for at komme i gang")
	} else if g.serviceInstalled {
		g.statusLabel.SetText("Service er installeret men stoppet")
	} else {
		g.statusLabel.SetText("Vælg installationsmetode nedenfor")
	}
}

func (g *AgentGUI) updateActionButtons() {
	g.actionButtons.Objects = nil

	if !g.isLoggedIn {
		loginBtn := widget.NewButtonWithIcon("Log ind", theme.LoginIcon(), g.showLoginDialog)
		loginBtn.Importance = widget.HighImportance
		g.actionButtons.Add(loginBtn)
	} else {
		programInstalled := isProgramInstalled()

		if !g.serviceInstalled {
			installServiceBtn := widget.NewButtonWithIcon("Installer som service (anbefalet)", theme.DownloadIcon(), g.doInstall)
			installServiceBtn.Importance = widget.HighImportance
			g.actionButtons.Add(installServiceBtn)

			if !programInstalled {
				installProgramBtn := widget.NewButtonWithIcon("Installer som program (autostart)", theme.ComputerIcon(), g.doInstallProgram)
				g.actionButtons.Add(installProgramBtn)
			} else {
				uninstallProgramBtn := widget.NewButtonWithIcon("Afinstaller program", theme.DeleteIcon(), g.doUninstallProgram)
				g.actionButtons.Add(uninstallProgramBtn)
			}

			runOnceBtn := widget.NewButtonWithIcon("Kør én gang (denne session)", theme.MediaPlayIcon(), g.doRunOnce)
			g.actionButtons.Add(runOnceBtn)
		} else if !g.serviceRunning {
			startBtn := widget.NewButtonWithIcon("Start service", theme.MediaPlayIcon(), g.doStart)
			startBtn.Importance = widget.HighImportance
			g.actionButtons.Add(startBtn)

			uninstallBtn := widget.NewButtonWithIcon("Afinstaller service", theme.DeleteIcon(), g.doUninstall)
			g.actionButtons.Add(uninstallBtn)
		} else {
			stopBtn := widget.NewButtonWithIcon("Stop service", theme.MediaStopIcon(), g.doStop)
			g.actionButtons.Add(stopBtn)

			uninstallBtn := widget.NewButtonWithIcon("Afinstaller service", theme.DeleteIcon(), g.doUninstall)
			g.actionButtons.Add(uninstallBtn)
		}

		g.actionButtons.Add(widget.NewSeparator())

		updateBtn := widget.NewButtonWithIcon("Tjek opdateringer", theme.ViewRefreshIcon(), g.doCheckUpdates)
		g.actionButtons.Add(updateBtn)

		logBtn := widget.NewButtonWithIcon("Vis log", theme.DocumentIcon(), g.doShowLog)
		g.actionButtons.Add(logBtn)

		logoutBtn := widget.NewButtonWithIcon("Log ud / skift konto", theme.LogoutIcon(), g.doLogout)
		g.actionButtons.Add(logoutBtn)
	}

	g.actionButtons.Add(widget.NewSeparator())

	exitBtn := widget.NewButtonWithIcon("Afslut", theme.CancelIcon(), func() {
		g.app.Quit()
	})
	g.actionButtons.Add(exitBtn)

	g.actionButtons.Refresh()
}

func (g *AgentGUI) showLoginDialog() {
	emailEntry := widget.NewEntry()
	emailEntry.SetPlaceHolder("your@email.com")

	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetPlaceHolder("Adgangskode")

	form := widget.NewForm(
		widget.NewFormItem("Email", emailEntry),
		widget.NewFormItem("Adgangskode", passwordEntry),
	)

	d := dialog.NewCustomConfirm("Log ind", "Log ind", "Annuller", form, func(ok bool) {
		if !ok {
			return
		}

		email := emailEntry.Text
		password := passwordEntry.Text

		if email == "" || password == "" {
			dialog.ShowError(fmt.Errorf("Indtast email og adgangskode"), g.window)
			return
		}

		// Show progress
		progress := dialog.NewCustomWithoutButtons("Logger ind...", widget.NewProgressBarInfinite(), g.window)
		progress.Show()

		go func() {
			log.Printf("🔐 Login attempt for: %s", email)
			
			cfg, err := config.Load()
			if err != nil {
				log.Printf("❌ Failed to load config: %v", err)
				fyne.Do(func() {
					progress.Hide()
					dialog.ShowError(fmt.Errorf("Kunne ikke indlæse konfiguration: %v", err), g.window)
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
					dialog.ShowError(fmt.Errorf("Login fejlede: %v", err), g.window)
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
			
			// Update UI first
			fyne.Do(func() {
				g.refreshStatus()
				log.Printf("📊 After refresh - isLoggedIn: %v, email: %s", g.isLoggedIn, g.userEmail)
				flushLog()
				g.updateStatusLabels()
				g.updateActionButtons()
			})
			
			// Small delay to let UI update, then show success message
			time.Sleep(100 * time.Millisecond)
			
			fyne.Do(func() {
				// Show success message - simple approach
				dialog.ShowInformation("Login lykkedes", 
					"✅ Velkommen, "+result.Email+"!\n\n"+
					"Du kan nu bruge Kør én gang eller installere som Windows service.", g.window)
			})
		}()
	}, g.window)

	d.Resize(fyne.NewSize(350, 200))
	d.Show()
}

func (g *AgentGUI) doInstall() {
	if !isAdmin() {
		dialog.ShowConfirm("Administrator kræves",
			"Installation som service kræver Administrator rettigheder.\n\nGenstart som Administrator?",
			func(ok bool) {
				if ok {
					runAsAdmin()
					g.app.Quit()
				}
			}, g.window)
		return
	}

	progress := dialog.NewCustomWithoutButtons("Installerer service...", widget.NewProgressBarInfinite(), g.window)
	progress.Show()

	go func() {
		err := installService()
		if err != nil {
			fyne.Do(func() {
				progress.Hide()
				dialog.ShowError(fmt.Errorf("Kunne ikke installere: %v", err), g.window)
			})
			return
		}

		err = startService()
		fyne.Do(func() {
			progress.Hide()

			if err != nil {
				dialog.ShowError(fmt.Errorf("Installeret men kunne ikke starte: %v", err), g.window)
			} else {
				dialog.ShowInformation("Installation færdig",
					"✅ Service installeret og startet!\n\n"+
						"Remote Desktop Agent kører nu.\n\n"+
						"• Starter automatisk med Windows\n"+
						"• Virker på login skærm\n"+
						"• Kør denne app for at administrere servicen", g.window)
			}

			g.refreshStatus()
			g.updateStatusLabels()
			g.updateActionButtons()
		})
	}()
}

func (g *AgentGUI) doUninstall() {
	dialog.ShowConfirm("Afinstaller Service", "Er du sikker på at du vil afinstallere servicen?", func(ok bool) {
		if !ok {
			return
		}

		if !isAdmin() {
			dialog.ShowConfirm("Administrator kræves",
				"Afinstallation kræver Administrator rettigheder.\n\nGenstart som Administrator?",
				func(ok bool) {
					if ok {
						runAsAdmin()
						g.app.Quit()
					}
				}, g.window)
			return
		}

		progress := dialog.NewCustomWithoutButtons("Afinstallerer...", widget.NewProgressBarInfinite(), g.window)
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
					dialog.ShowError(fmt.Errorf("Kunne ikke afinstallere: %v", err), g.window)
				} else {
					dialog.ShowInformation("Afinstallation færdig", 
						"✅ Service afinstalleret.\n\n"+
						"Remote Desktop Agent kører ikke længere som service.", g.window)
				}

				g.refreshStatus()
				g.updateStatusLabels()
				g.updateActionButtons()
			})
		}()
	}, g.window)
}

func (g *AgentGUI) doStart() {
	progress := dialog.NewCustomWithoutButtons("Starter service...", widget.NewProgressBarInfinite(), g.window)
	progress.Show()

	go func() {
		err := startService()
		fyne.Do(func() {
			progress.Hide()

			if err != nil {
				dialog.ShowError(fmt.Errorf("Kunne ikke starte: %v", err), g.window)
			} else {
				dialog.ShowInformation("Service startet", 
					"✅ Service startet!\n\n"+
					"Remote Desktop Agent kører nu og er klar til forbindelser.", g.window)
			}

			g.refreshStatus()
			g.updateStatusLabels()
			g.updateActionButtons()
		})
	}()
}

func (g *AgentGUI) doStop() {
	progress := dialog.NewCustomWithoutButtons("Stopper service...", widget.NewProgressBarInfinite(), g.window)
	progress.Show()

	go func() {
		err := stopService()
		fyne.Do(func() {
			progress.Hide()

			if err != nil {
				dialog.ShowError(fmt.Errorf("Kunne ikke stoppe: %v", err), g.window)
			} else {
				dialog.ShowInformation("Service stoppet", 
					"✅ Service stoppet.\n\n"+
					"Fjernforbindelser er ikke mulige før servicen startes igen.", g.window)
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
		dialog.ShowError(fmt.Errorf("Log ind først før du kører agenten"), g.window)
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
		dialog.ShowError(fmt.Errorf("Kunne ikke starte agent: %v", err), g.window)
		return
	}

	log.Println("✅ Agent started successfully")

	// Update GUI to show running status
	g.statusLabel.SetText("Agent kører")
	g.serviceLabel.SetText("Tilstand: Interaktiv (Kør én gang)")

	// Reset privacy state each Run Once
	state.SetPrivacyMode(false)

	// Disable Run Once button, enable a Stop button
	g.actionButtons.RemoveAll()

	stopBtn := widget.NewButtonWithIcon("Stop agent", theme.MediaStopIcon(), func() {
		log.Println("Stopping agent...")
		stopAgent()
		state.SetPrivacyMode(false)
		g.statusLabel.SetText("Agent stoppet")
		g.serviceLabel.SetText("Tilstand: Stoppet")
		g.updateActionButtons()
		dialog.ShowInformation("Agent stoppet", "Agenten er stoppet.", g.window)
	})
	stopBtn.Importance = widget.DangerImportance
	g.actionButtons.Add(stopBtn)

	// Privacy toggle — only works in Run Once (same process as capture)
	var privacyBtn *widget.Button
	privacyBtn = widget.NewButtonWithIcon("Skjul skærm (privacy)", theme.VisibilityOffIcon(), func() {
		if state.IsPrivacyModeEnabled() {
			state.SetPrivacyMode(false)
			privacyBtn.SetText("Skjul skærm (privacy)")
			privacyBtn.SetIcon(theme.VisibilityOffIcon())
			log.Println("🔓 Privacy mode OFF — normal capture genoptaget")
		} else {
			state.SetPrivacyMode(true)
			privacyBtn.SetText("Vis skærm igen")
			privacyBtn.SetIcon(theme.VisibilityIcon())
			log.Println("🔒 Privacy mode ON — controller får sort skærm")
		}
	})
	g.actionButtons.Add(privacyBtn)

	exitBtn := widget.NewButtonWithIcon("Afslut", theme.CancelIcon(), func() {
		stopAgent()
		state.SetPrivacyMode(false)
		g.app.Quit()
	})
	g.actionButtons.Add(exitBtn)

	g.actionButtons.Refresh()

	dialog.ShowInformation("Agent startet",
		"✅ Agent kører nu!\n\n"+
		"Du kan minimere dette vindue.\n"+
		"Klik 'Skjul skærm' for at sende sort skærm til controlleren.\n"+
		"Klik 'Stop agent' for at afbryde.", g.window)
}

func (g *AgentGUI) doInstallProgram() {
	if !isAdmin() {
		dialog.ShowConfirm("Administrator kræves",
			"Installation som program kræver Administrator rettigheder.\n\nGenstart som Administrator?",
			func(ok bool) {
				if ok {
					runAsAdmin()
					g.app.Quit()
				}
			}, g.window)
		return
	}

	dialog.ShowConfirm("Installer som Program",
		"Dette vil:\n\n"+
			"• Kopiere agenten til Program Files\n"+
			"• Sætte autostart ved Windows login\n"+
			"• Agenten kører som din bruger (ikke service)\n\n"+
			"Bemærk: Service-mode anbefales for login-skærm support.\n\n"+
			"Fortsæt?",
		func(ok bool) {
			if !ok {
				return
			}

			progress := dialog.NewCustomWithoutButtons("Installerer program...", widget.NewProgressBarInfinite(), g.window)
			progress.Show()

			go func() {
				err := installAsProgram()
				fyne.Do(func() {
					progress.Hide()

					if err != nil {
						dialog.ShowError(fmt.Errorf("Kunne ikke installere: %v", err), g.window)
						g.refreshStatus()
						g.updateStatusLabels()
						g.updateActionButtons()
					} else {
						// Show success and close GUI - agent is now running from Program Files
						d := dialog.NewInformation("Installation færdig",
							"✅ Program installeret og startet!\n\n"+
								"• Agenten kører nu i baggrunden\n"+
								"• Se system tray ikonet (ved uret)\n"+
								"• Starter automatisk ved Windows login\n\n"+
								"Dette vindue lukkes nu.", g.window)
						d.SetOnClosed(func() {
							g.app.Quit()
						})
						d.Show()
					}
				})
			}()
		}, g.window)
}

func (g *AgentGUI) doUninstallProgram() {
	dialog.ShowConfirm("Afinstaller Program", "Er du sikker på at du vil afinstallere programmet?\n\nDette fjerner autostart og sletter installationen.", func(ok bool) {
		if !ok {
			return
		}

		if !isAdmin() {
			dialog.ShowConfirm("Administrator kræves",
				"Afinstallation kræver Administrator rettigheder.\n\nGenstart som Administrator?",
				func(ok bool) {
					if ok {
						runAsAdmin()
						g.app.Quit()
					}
				}, g.window)
			return
		}

		progress := dialog.NewCustomWithoutButtons("Afinstallerer...", widget.NewProgressBarInfinite(), g.window)
		progress.Show()

		go func() {
			err := uninstallProgram()
			fyne.Do(func() {
				progress.Hide()

				if err != nil {
					dialog.ShowError(fmt.Errorf("Kunne ikke afinstallere: %v", err), g.window)
				} else {
					dialog.ShowInformation("Afinstallation færdig",
						"✅ Program afinstalleret.\n\nAutostart er fjernet.", g.window)
				}

				g.refreshStatus()
				g.updateStatusLabels()
				g.updateActionButtons()
			})
		}()
	}, g.window)
}

func (g *AgentGUI) doCheckUpdates() {
	// Create updater
	u, err := updater.NewUpdater(tray.Version)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Kunne ikke initialisere opdatering: %v", err), g.window)
		return
	}

	// Status label
	statusLabel := widget.NewLabel("Klar til at tjekke for opdateringer")
	statusLabel.Wrapping = fyne.TextWrapWord

	// Progress bar (hidden initially)
	progressBar := widget.NewProgressBar()
	progressBar.Hide()

	// Version info
	currentVersionLabel := widget.NewLabel(fmt.Sprintf("Nuværende version: %s", tray.Version))

	// Buttons
	var checkBtn, downloadBtn, installBtn *widget.Button

	checkBtn = widget.NewButtonWithIcon("Tjek for opdateringer", theme.SearchIcon(), func() {
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
	downloadBtn = widget.NewButtonWithIcon("Download opdatering", theme.DownloadIcon(), func() {
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
	installBtn = widget.NewButtonWithIcon("Installer og genstart", theme.UploadIcon(), func() {
		dialog.ShowConfirm("Installer opdatering",
			"Agenten vil lukke og genstarte med den nye version.\n\nFortsæt?",
			func(confirmed bool) {
				if !confirmed {
					return
				}

				statusLabel.SetText("Installerer...")
				installBtn.Disable()

				go func() {
					// Stop agent if running
					stopAgent()

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
						g.app.Quit()
					})
				}()
			}, g.window)
	})
	installBtn.Importance = widget.DangerImportance
	installBtn.Hide()

	// Layout
	content := container.NewVBox(
		widget.NewLabelWithStyle("Opdateringer", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		currentVersionLabel,
		widget.NewSeparator(),
		statusLabel,
		progressBar,
		widget.NewSeparator(),
		checkBtn,
		downloadBtn,
		installBtn,
	)

	scrollContent := container.NewScroll(content)
	scrollContent.SetMinSize(fyne.NewSize(400, 300))

	dialog.ShowCustom("Opdateringer", "Luk", scrollContent, g.window)
}

// doShowLog viser service-log'en. Først forsøger den at åbne filen i
// Notepad (åben i baggrunden), og hvis det fejler eller log'en mangler,
// viser den de sidste linjer i et dialog-vindue så brugeren kan kopiere
// teksten direkte til support-chatten.
func (g *AgentGUI) doShowLog() {
	logPath := filepath.Join(os.Getenv("ProgramData"), "RemoteDesktopAgent", "service.log")

	// 1) Åben i Notepad (foretrukket — let at scrolle og kopiere)
	if _, err := os.Stat(logPath); err == nil {
		cmd := exec.Command("notepad.exe", logPath)
		if err := cmd.Start(); err == nil {
			// Process starter — vinduet detacher fra agenten
			return
		}
	}

	// 2) Fallback — vis de sidste 50 linjer i et tekst-dialog
	data, err := os.ReadFile(logPath)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Kunne ikke åbne log: %v\nForventet sti: %s", err, logPath), g.window)
		return
	}

	// Tail sidste ~50 linjer
	lines := splitLines(string(data))
	tailFrom := 0
	if len(lines) > 50 {
		tailFrom = len(lines) - 50
	}
	tail := joinLines(lines[tailFrom:])

	logEntry := widget.NewMultiLineEntry()
	logEntry.SetText(tail)
	logEntry.Wrapping = fyne.TextWrapWord
	logEntry.Disable()
	scroll := container.NewScroll(logEntry)
	scroll.SetMinSize(fyne.NewSize(700, 400))

	pathLabel := widget.NewLabel(logPath)
	pathLabel.TextStyle = fyne.TextStyle{Italic: true}
	content := container.NewBorder(pathLabel, nil, nil, nil, scroll)

	d := dialog.NewCustom("Service log (sidste 50 linjer)", "Luk", content, g.window)
	d.Resize(fyne.NewSize(750, 500))
	d.Show()
}

func splitLines(s string) []string {
	out := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func joinLines(lines []string) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n"
		}
		out += l
	}
	return out
}

func (g *AgentGUI) doLogout() {
	dialog.ShowConfirm("Log ud", "Er du sikker på at du vil logge ud?", func(ok bool) {
		if !ok {
			return
		}

		if err := auth.ClearCredentials(); err != nil {
			dialog.ShowError(fmt.Errorf("Kunne ikke logge ud: %v", err), g.window)
		} else {
			dialog.ShowInformation("Succes", "✅ Logget ud!", g.window)
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
