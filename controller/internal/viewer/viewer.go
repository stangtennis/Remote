package viewer

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// Viewer represents the remote desktop viewer window
type Viewer struct {
	window            fyne.Window
	videoCanvas       *canvas.Image
	interactiveCanvas *InteractiveCanvas
	deviceID          string
	deviceName        string
	connected         bool
	fullscreen        bool
	toolbarVisible    bool
	lastFrame         image.Image // Cache last frame for restore

	// UI Components
	toolbar        *fyne.Container
	statusBar      *fyne.Container
	videoContainer *fyne.Container
	mainContent    *fyne.Container

	// Status indicators
	statusLabel   *widget.Label
	fpsLabel      *widget.Label
	latencyLabel  *widget.Label
	qualitySlider *widget.Slider

	// Buttons
	fullscreenBtn *widget.Button

	// WebRTC and Input
	webrtcClient      interface{} // Will be *webrtc.Client
	inputHandler      *InputHandler
	fileTransferMgr   interface{} // Will be *filetransfer.Manager
	reconnectionMgr   interface{} // Will be *reconnection.Manager
	clipboardReceiver interface{} // Will be *clipboard.Receiver
	clipboardMonitor  interface{} // Will be *clipboard.Monitor

	// Connection state
	supabaseURL string
	anonKey     string
	authToken   string
	userID      string

	// Callbacks
	onDisconnect   func()
	onFileTransfer func()
}

// NewViewer creates a new remote desktop viewer
func NewViewer(app fyne.App, deviceID, deviceName string) *Viewer {
	v := &Viewer{
		deviceID:       deviceID,
		deviceName:     deviceName,
		connected:      false,
		fullscreen:     false,
		toolbarVisible: true,
	}

	// Create window optimized for Full HD
	v.window = app.NewWindow(fmt.Sprintf("Remote Desktop - %s", deviceName))
	v.window.Resize(fyne.NewSize(1920, 1080))
	v.window.CenterOnScreen()

	v.buildUI()

	return v
}

// buildUI constructs the viewer interface
func (v *Viewer) buildUI() {
	// Create video canvas (black background initially)
	v.videoCanvas = canvas.NewImageFromImage(createBlackImage(1920, 1080))
	v.videoCanvas.FillMode = canvas.ImageFillStretch // Stretch to fill - no black bars
	v.videoCanvas.ScaleMode = canvas.ImageScaleSmooth

	// Wrap in interactive canvas for input capture
	v.interactiveCanvas = NewInteractiveCanvas(v.videoCanvas)

	// Build toolbar
	v.toolbar = v.createToolbar()

	// Build status bar
	v.statusBar = v.createStatusBar()

	// Video container with border
	videoBorder := canvas.NewRectangle(color.RGBA{R: 40, G: 40, B: 40, A: 255})
	v.videoContainer = container.NewStack(
		videoBorder,
		container.NewPadded(v.interactiveCanvas),
	)

	// Main layout
	v.mainContent = container.NewBorder(
		v.toolbar,        // Top
		v.statusBar,      // Bottom
		nil,              // Left
		nil,              // Right
		v.videoContainer, // Center
	)

	v.window.SetContent(v.mainContent)

	// Setup keyboard shortcuts
	v.window.Canvas().SetOnTypedKey(func(key *fyne.KeyEvent) {
		// Home to toggle toolbar visibility
		if key.Name == fyne.KeyHome {
			v.toggleToolbarVisibility()
		}
		// F11 for fullscreen toggle
		if key.Name == fyne.KeyF11 {
			v.toggleFullscreen()
		}
		// Escape to exit fullscreen
		if key.Name == fyne.KeyEscape && v.fullscreen {
			v.toggleFullscreen()
		}
	})

	// Start periodic canvas refresh to handle minimize/restore
	go v.startCanvasRefreshLoop()

	// Handle window close
	v.window.SetOnClosed(func() {
		log.Println("Viewer window closing, cleaning up...")

		// Close WebRTC connection
		if v.webrtcClient != nil {
			if client, ok := v.webrtcClient.(interface{ Close() error }); ok {
				if err := client.Close(); err != nil {
					log.Printf("Error closing WebRTC client: %v", err)
				}
			}
		}

		// Stop reconnection manager
		if v.reconnectionMgr != nil {
			if reconnMgr, ok := v.reconnectionMgr.(interface{ Stop() }); ok {
				reconnMgr.Stop()
			}
		}

		// Call disconnect callback if set
		if v.onDisconnect != nil {
			v.onDisconnect()
		}
	})
}

// createToolbar builds the top toolbar
func (v *Viewer) createToolbar() *fyne.Container {
	// Connection status
	v.statusLabel = widget.NewLabel("🔴 Disconnected")
	v.statusLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Connect button
	connectBtn := widget.NewButton("Connect", func() {
		v.handleConnect()
	})
	connectBtn.Importance = widget.HighImportance

	// Disconnect button
	disconnectBtn := widget.NewButton("Disconnect", func() {
		v.handleDisconnect()
	})
	disconnectBtn.Importance = widget.DangerImportance

	// Fullscreen toggle
	v.fullscreenBtn = widget.NewButton("⛶ Fullscreen", func() {
		v.toggleFullscreen()
	})

	// File transfer button
	fileTransferBtn := widget.NewButton("📁 Send File", func() {
		v.SendFile()
	})

	// Clipboard sync button
	clipboardBtn := widget.NewButton("📋 Sync Clipboard", func() {
		v.handleClipboardSync()
	})

	// Quality control
	qualityLabel := widget.NewLabel("Quality:")
	v.qualitySlider = widget.NewSlider(1, 100)
	v.qualitySlider.Value = 80
	v.qualitySlider.Step = 10
	v.qualitySlider.OnChanged = func(value float64) {
		v.handleQualityChange(value)
	}

	// Settings button
	settingsBtn := widget.NewButton("⚙️ Settings", func() {
		v.showSettings()
	})

	// Layout toolbar
	leftSection := container.NewHBox(
		v.statusLabel,
		layout.NewSpacer(),
		connectBtn,
		disconnectBtn,
	)

	middleSection := container.NewHBox(
		v.fullscreenBtn,
		fileTransferBtn,
		clipboardBtn,
	)

	rightSection := container.NewHBox(
		qualityLabel,
		container.NewGridWithColumns(1, v.qualitySlider),
		settingsBtn,
	)

	return container.NewBorder(
		nil, nil,
		leftSection,
		rightSection,
		middleSection,
	)
}

// createStatusBar builds the bottom status bar
func (v *Viewer) createStatusBar() *fyne.Container {
	// FPS indicator
	v.fpsLabel = widget.NewLabel("FPS: 0")

	// Latency indicator
	v.latencyLabel = widget.NewLabel("Latency: 0ms")

	// Resolution label
	resolutionLabel := widget.NewLabel("Resolution: 1920x1080")

	// Keyboard/Mouse status
	inputLabel := widget.NewLabel("🖱️ Mouse & ⌨️ Keyboard Active")

	// Keyboard shortcuts hint
	shortcutsLabel := widget.NewLabel("💡 F11: Fullscreen | ESC: Exit Fullscreen | Home: Toggle Toolbar")
	shortcutsLabel.TextStyle = fyne.TextStyle{Italic: true}

	return container.NewHBox(
		v.fpsLabel,
		widget.NewSeparator(),
		v.latencyLabel,
		widget.NewSeparator(),
		resolutionLabel,
		widget.NewSeparator(),
		inputLabel,
		layout.NewSpacer(),
		shortcutsLabel,
		layout.NewSpacer(),
		widget.NewLabel(fmt.Sprintf("Device: %s", v.deviceName)),
	)
}

// Show displays the viewer window
func (v *Viewer) Show() {
	v.window.Show()
}

// UpdateFrame updates the video frame
func (v *Viewer) UpdateFrame(img image.Image) {
	if img == nil {
		return
	}
	v.videoCanvas.Image = img
	v.videoCanvas.Refresh()
}

// UpdateStatus updates connection status
func (v *Viewer) UpdateStatus(connected bool) {
	v.connected = connected
	if connected {
		v.statusLabel.SetText("🟢 Connected")
	} else {
		v.statusLabel.SetText("🔴 Disconnected")
	}
}

// UpdateStats updates performance statistics
func (v *Viewer) UpdateStats(fps int, latency int) {
	v.fpsLabel.SetText(fmt.Sprintf("FPS: %d", fps))
	v.latencyLabel.SetText(fmt.Sprintf("Latency: %dms", latency))
}

// Event handlers
func (v *Viewer) handleConnect() {
	log.Println("Connecting to remote device...")
	// TODO: Implement WebRTC connection
}

func (v *Viewer) handleDisconnect() {
	log.Println("Disconnecting from remote device...")

	// Close WebRTC connection
	if v.webrtcClient != nil {
		if client, ok := v.webrtcClient.(interface{ Close() error }); ok {
			if err := client.Close(); err != nil {
				log.Printf("Error closing WebRTC client: %v", err)
			}
		}
		v.webrtcClient = nil
	}

	// Stop reconnection manager
	if v.reconnectionMgr != nil {
		if reconnMgr, ok := v.reconnectionMgr.(interface{ Stop() }); ok {
			reconnMgr.Stop()
		}
	}

	// Update status
	v.connected = false
	v.statusLabel.SetText("🔴 Disconnected")

	// Call disconnect callback if set
	if v.onDisconnect != nil {
		v.onDisconnect()
	}

	// Close the viewer window
	v.window.Close()
}

func (v *Viewer) toggleFullscreen() {
	v.fullscreen = !v.fullscreen
	v.window.SetFullScreen(v.fullscreen)

	if v.fullscreen {
		// In fullscreen, hide toolbar and status bar initially
		v.fullscreenBtn.SetText("⛶ Exit Fullscreen")
		v.hideToolbars()
	} else {
		// Windowed mode, show toolbars
		v.fullscreenBtn.SetText("⛶ Fullscreen")
		v.showToolbars()
	}
}

func (v *Viewer) hideToolbars() {
	v.toolbarVisible = false
	// Replace content with just video (no toolbar/statusbar)
	v.window.SetContent(v.videoContainer)
}

func (v *Viewer) showToolbars() {
	v.toolbarVisible = true
	// Restore full layout with toolbar and statusbar
	v.window.SetContent(v.mainContent)
}

func (v *Viewer) toggleToolbarVisibility() {
	if v.toolbarVisible {
		v.hideToolbars()
	} else {
		v.showToolbars()
	}
}

func (v *Viewer) handleFileTransfer() {
	log.Println("Opening file transfer dialog...")
	if v.onFileTransfer != nil {
		v.onFileTransfer()
	}
	// TODO: Implement file transfer
}

func (v *Viewer) handleClipboardSync() {
	log.Println("Syncing clipboard...")
	v.sendClipboardNow()
}

func (v *Viewer) handleQualityChange(value float64) {
	log.Printf("Quality changed to: %.0f%%", value)
	// TODO: Adjust video quality
}

func (v *Viewer) showSettings() {
	log.Println("Opening settings...")
	// TODO: Show settings dialog
}

// SetOnDisconnect sets the disconnect callback
func (v *Viewer) SetOnDisconnect(callback func()) {
	v.onDisconnect = callback
}

// SetOnFileTransfer sets the file transfer callback
func (v *Viewer) SetOnFileTransfer(callback func()) {
	v.onFileTransfer = callback
}

// startCanvasRefreshLoop periodically checks if canvas needs refresh (for minimize/restore)
func (v *Viewer) startCanvasRefreshLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var lastRefreshTime time.Time

	for range ticker.C {
		// Only do periodic refresh if connected and we have a cached frame
		// and it's been more than 1 second since last normal frame update
		if v.connected && v.lastFrame != nil {
			// Check if canvas image is nil or stale (window was minimized)
			if v.videoCanvas.Image == nil {
				fyne.Do(func() {
					v.videoCanvas.Image = v.lastFrame
					v.videoCanvas.Refresh()
				})
				lastRefreshTime = time.Now()
			} else if time.Since(lastRefreshTime) > 2*time.Second {
				// Periodic refresh to ensure canvas stays updated
				fyne.Do(func() {
					v.videoCanvas.Refresh()
				})
				lastRefreshTime = time.Now()
			}
		}
	}
}

// Helper functions
func createBlackImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	black := color.RGBA{R: 0, G: 0, B: 0, A: 255}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, black)
		}
	}
	return img
}
