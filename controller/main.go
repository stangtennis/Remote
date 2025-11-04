package main

import (
	"fmt"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/stangtennis/Remote/controller/internal/config"
	"github.com/stangtennis/Remote/controller/internal/supabase"
)

var (
	supabaseClient *supabase.Client
	currentUser    *supabase.User
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize Supabase client
	supabaseClient = supabase.NewClient(cfg.SupabaseURL, cfg.SupabaseAnonKey)
	log.Println("✅ Supabase client initialized")

	// Create application
	myApp := app.New()
	myWindow := myApp.NewWindow("Remote Desktop Controller")
	myWindow.Resize(fyne.NewSize(800, 600))

	// Create UI
	content := createMainUI(myWindow)
	myWindow.SetContent(content)

	// Show and run
	myWindow.ShowAndRun()
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
			authResp, err := supabaseClient.SignIn(email, password)
			if err != nil {
				log.Printf("Login failed: %v", err)
				statusLabel.SetText("❌ Login failed: " + err.Error())
				loginButton.Enable()
				return
			}

			currentUser = &authResp.User
			log.Printf("✅ Logged in as: %s", currentUser.Email)

			// Check if user is approved
			approved, err := supabaseClient.CheckApproval(currentUser.ID)
			if err != nil {
				log.Printf("Failed to check approval: %v", err)
				statusLabel.SetText("❌ Failed to check approval")
				loginButton.Enable()
				return
			}

			if !approved {
				statusLabel.SetText("⏸️ Account pending approval")
				loginButton.Enable()
				return
			}

			statusLabel.SetText("✅ Connected as: " + currentUser.Email)
			
			// Fetch devices assigned to this user
			devices, err := supabaseClient.GetDevices(currentUser.ID)
			if err != nil {
				log.Printf("Failed to fetch devices: %v", err)
				statusLabel.SetText("⚠️ Connected but failed to load devices")
			} else {
				devicesData = devices
				log.Printf("✅ Loaded %d assigned devices", len(devices))
				if deviceListWidget != nil {
					deviceListWidget.Refresh()
				}
			}
			
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
			
			// Format device name with status indicator
			statusIcon := "🔴" // offline
			if device.Status == "online" {
				statusIcon = "🟢"
			} else if device.Status == "away" {
				statusIcon = "🟡"
			}
			
			displayName := fmt.Sprintf("%s %s (%s)", statusIcon, device.DeviceName, device.Platform)
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
	log.Printf("🔗 Initiating connection to: %s", device.DeviceName)
	
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
