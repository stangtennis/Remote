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
	"fyne.io/fyne/v2/dialog"
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
	statusLabel    *widget.Label
	fpsLabel       *widget.Label
	latencyLabel   *widget.Label
	bandwidthLabel *widget.Label
	modeLabel      *widget.Label // Streaming mode (JPEG/H.264)
	qualitySlider  *widget.Slider

	// Bandwidth tracking
	bytesReceived     int64
	lastBandwidthTime time.Time
	currentBandwidth  float64 // Mbit/s

	// Streaming mode tracking
	currentStreamingMode string // "jpeg" or "h264"

	// Buttons
	fullscreenBtn *widget.Button

	// WebRTC and Input
	webrtcClient      interface{} // Will be *webrtc.Client
	inputHandler      *InputHandler
	fileTransferMgr   interface{} // Will be *filetransfer.Manager
	reconnectionMgr   interface{} // Will be *reconnection.Manager
	clipboardReceiver interface{} // Will be *clipboard.Receiver
	clipboardMonitor  interface{} // Will be *clipboard.Monitor
	fileBrowser       interface{} // Will be *filebrowser.FileBrowser

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

	// Create window - start at reasonable size, user can resize
	v.window = app.NewWindow(fmt.Sprintf("Remote Desktop - %s", deviceName))
	v.window.Resize(fyne.NewSize(1280, 720)) // Start smaller, resizable
	v.window.CenterOnScreen()
	v.window.SetFixedSize(false) // Ensure window is resizable

	v.buildUI()

	return v
}

// buildUI constructs the viewer interface
func (v *Viewer) buildUI() {
	// Create video canvas (black background initially)
	// Use smaller initial size - will be updated when frames arrive
	v.videoCanvas = canvas.NewImageFromImage(createBlackImage(1280, 720))
	v.videoCanvas.FillMode = canvas.ImageFillContain // Maintain aspect ratio
	v.videoCanvas.ScaleMode = canvas.ImageScaleSmooth
	v.videoCanvas.SetMinSize(fyne.NewSize(320, 240)) // Allow shrinking

	// Wrap in interactive canvas for input capture
	v.interactiveCanvas = NewInteractiveCanvas(v.videoCanvas)

	// Build toolbar
	v.toolbar = v.createToolbar()

	// Build status bar
	v.statusBar = v.createStatusBar()

	// Video container - no padding to maximize space
	videoBorder := canvas.NewRectangle(color.RGBA{R: 40, G: 40, B: 40, A: 255})
	v.videoContainer = container.NewStack(
		videoBorder,
		v.interactiveCanvas, // No padding - use full space
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

	// File transfer button - opens Total Commander style browser
	fileTransferBtn := widget.NewButton("📁 File Browser", func() {
		v.OpenFileBrowser()
	})

	// Clipboard sync button
	clipboardBtn := widget.NewButton("📋 Sync Clipboard", func() {
		v.handleClipboardSync()
	})

	// H.264 mode toggle button
	h264Btn := widget.NewButton("🎬 H.264", func() {
		v.toggleH264Mode()
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
		h264Btn,
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

	// Bandwidth indicator
	v.bandwidthLabel = widget.NewLabel("0.0 Mbit/s")

	// Streaming mode indicator
	v.modeLabel = widget.NewLabel("📺 JPEG")
	v.modeLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Resolution label
	resolutionLabel := widget.NewLabel("Resolution: 1920x1080")

	// Keyboard/Mouse status
	inputLabel := widget.NewLabel("🖱️ Mouse & ⌨️ Keyboard Active")

	// Fullscreen toggle button for status bar
	fullscreenToggleBtn := widget.NewButton("⛶ Fullscreen", func() {
		v.toggleFullscreen()
	})

	// Keyboard shortcuts hint
	shortcutsLabel := widget.NewLabel("💡 F11 | ESC")
	shortcutsLabel.TextStyle = fyne.TextStyle{Italic: true}

	// Initialize bandwidth tracking
	v.lastBandwidthTime = time.Now()

	return container.NewHBox(
		v.modeLabel,
		widget.NewSeparator(),
		v.fpsLabel,
		widget.NewSeparator(),
		v.bandwidthLabel,
		widget.NewSeparator(),
		v.latencyLabel,
		widget.NewSeparator(),
		resolutionLabel,
		widget.NewSeparator(),
		inputLabel,
		layout.NewSpacer(),
		fullscreenToggleBtn,
		widget.NewSeparator(),
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
	v.latencyLabel.SetText(fmt.Sprintf("RTT: %dms", latency))
}

// UpdateRTT updates the RTT display from ping/pong measurement
func (v *Viewer) UpdateRTT(rtt time.Duration) {
	fyne.Do(func() {
		v.latencyLabel.SetText(fmt.Sprintf("RTT: %dms", rtt.Milliseconds()))
	})
}

// TrackBytesReceived adds bytes to the bandwidth counter
func (v *Viewer) TrackBytesReceived(bytes int) {
	v.bytesReceived += int64(bytes)
}

// UpdateBandwidth calculates and updates bandwidth display (call every second)
func (v *Viewer) UpdateBandwidth() {
	now := time.Now()
	elapsed := now.Sub(v.lastBandwidthTime).Seconds()

	if elapsed > 0 && v.bytesReceived > 0 {
		bitsPerSecond := float64(v.bytesReceived*8) / elapsed
		v.currentBandwidth = bitsPerSecond / 1000000 // Convert to Mbit/s
		v.bandwidthLabel.SetText(fmt.Sprintf("%.1f Mbit/s", v.currentBandwidth))
	}

	// Reset counters
	v.bytesReceived = 0
	v.lastBandwidthTime = now
}

// UpdateStreamingMode updates the streaming mode indicator
func (v *Viewer) UpdateStreamingMode(mode string) {
	// Update internal state
	v.currentStreamingMode = mode

	fyne.Do(func() {
		switch mode {
		case "h264":
			v.modeLabel.SetText("🎬 H.264")
		case "hybrid":
			v.modeLabel.SetText("🔀 Hybrid")
		default:
			v.modeLabel.SetText("📺 JPEG")
		}
	})
}

// GetCurrentBandwidth returns current bandwidth in Mbit/s
func (v *Viewer) GetCurrentBandwidth() float64 {
	return v.currentBandwidth
}

// toggleH264Mode toggles between JPEG and H.264 streaming modes
func (v *Viewer) toggleH264Mode() {
	if !v.connected {
		log.Println("⚠️ Not connected - cannot toggle H.264 mode")
		return
	}

	// Toggle based on internal state
	// Note: agent expects "tiles" not "jpeg" for JPEG mode
	var newMode string
	if v.currentStreamingMode == "h264" {
		newMode = "tiles"
		log.Println("📺 Switching to JPEG tiles mode...")
	} else {
		newMode = "h264"
		log.Println("🎬 Switching to H.264 mode...")
	}

	// Send mode change to agent
	if client, ok := v.webrtcClient.(interface {
		SetStreamingMode(mode string, bitrate int) error
	}); ok {
		if err := client.SetStreamingMode(newMode, 0); err != nil {
			log.Printf("❌ Failed to set streaming mode: %v", err)
		} else {
			// Update internal state immediately for responsive UI
			v.currentStreamingMode = newMode
		}
	}
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

	// FPS display
	fpsLabel := widget.NewLabel("Target FPS: 30")
	fpsSlider := widget.NewSlider(10, 60)
	fpsSlider.Value = 30
	fpsSlider.Step = 5
	fpsSlider.OnChanged = func(value float64) {
		fpsLabel.SetText(fmt.Sprintf("Target FPS: %.0f", value))
		// TODO: Send FPS change to agent
	}

	// Quality display
	qualityLabel := widget.NewLabel("JPEG Quality: 70%")
	qualitySlider := widget.NewSlider(30, 95)
	qualitySlider.Value = 70
	qualitySlider.Step = 5
	qualitySlider.OnChanged = func(value float64) {
		qualityLabel.SetText(fmt.Sprintf("JPEG Quality: %.0f%%", value))
		// TODO: Send quality change to agent
	}

	// Connection info
	connInfo := widget.NewLabel("Connection: WebRTC DataChannel")
	if v.webrtcClient != nil {
		connInfo.SetText("Connection: WebRTC DataChannel (Active)")
	}

	// Keyboard shortcuts info
	shortcutsInfo := widget.NewLabel(
		"Keyboard Shortcuts:\n" +
			"  F11 - Toggle Fullscreen\n" +
			"  Home - Toggle Toolbar\n" +
			"  Escape - Exit Fullscreen",
	)

	content := container.NewVBox(
		widget.NewLabelWithStyle("⚙️ Session Settings", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		fpsLabel,
		fpsSlider,
		widget.NewSeparator(),
		qualityLabel,
		qualitySlider,
		widget.NewSeparator(),
		connInfo,
		widget.NewSeparator(),
		shortcutsInfo,
	)

	// Wrap in scroll container for smaller screens
	scrollContent := container.NewScroll(content)
	scrollContent.SetMinSize(fyne.NewSize(350, 400))

	dialog.ShowCustom("Settings", "Close", scrollContent, v.window)
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
