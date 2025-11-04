package main

import (
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func main() {
	// Create application
	myApp := app.New()
	myWindow := myApp.NewWindow("Remote Desktop Controller")
	myWindow.Resize(fyne.NewSize(800, 600))

	// Create UI
	content := createMainUI()
	myWindow.SetContent(content)

	// Show and run
	myWindow.ShowAndRun()
}

func createMainUI() *fyne.Container {
	// Title
	title := widget.NewLabel("🎮 Remote Desktop Controller")
	title.TextStyle = fyne.TextStyle{Bold: true}

	// Login section
	emailEntry := widget.NewEntry()
	emailEntry.SetPlaceHolder("Email")
	
	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetPlaceHolder("Password")

	statusLabel := widget.NewLabel("Not connected")

	loginButton := widget.NewButton("Login", func() {
		email := emailEntry.Text
		password := passwordEntry.Text
		
		if email == "" || password == "" {
			statusLabel.SetText("❌ Please enter email and password")
			return
		}

		statusLabel.SetText("🔄 Connecting to Supabase...")
		
		// TODO: Implement Supabase authentication
		// For now, just simulate success
		go func() {
			// Simulate network delay
			// time.Sleep(1 * time.Second)
			statusLabel.SetText("✅ Connected as: " + email)
		}()
	})

	loginForm := container.NewVBox(
		widget.NewLabel("Login to Remote Desktop"),
		emailEntry,
		passwordEntry,
		loginButton,
		statusLabel,
	)

	// Device list section (placeholder)
	deviceList := widget.NewList(
		func() int {
			return 5 // Number of devices
		},
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel("Device Name"),
				widget.NewButton("Connect", func() {}),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			// Update list item
			deviceNames := []string{
				"🟢 John's PC (Windows)",
				"🟢 Office Laptop (Windows)",
				"🟢 Web-Chrome (Browser)",
				"🔴 Server-01 (Offline)",
				"🟡 Mobile-Android (Away)",
			}
			
			box := obj.(*fyne.Container)
			label := box.Objects[0].(*widget.Label)
			button := box.Objects[1].(*widget.Button)
			
			label.SetText(deviceNames[id])
			
			// Disable button for offline devices
			if id == 3 {
				button.Disable()
			} else {
				button.Enable()
				button.OnTapped = func() {
					log.Printf("Connecting to device %d", id)
					// TODO: Implement connection
				}
			}
		},
	)

	deviceSection := container.NewBorder(
		widget.NewLabel("📱 Available Devices"),
		nil,
		nil,
		nil,
		deviceList,
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
