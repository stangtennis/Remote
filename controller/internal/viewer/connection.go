package viewer

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"math"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"github.com/pion/webrtc/v3"
	"github.com/stangtennis/Remote/controller/internal/clipboard"
	"github.com/stangtennis/Remote/controller/internal/filebrowser"
	"github.com/stangtennis/Remote/controller/internal/filetransfer"
	"github.com/stangtennis/Remote/controller/internal/reconnection"
	rtc "github.com/stangtennis/Remote/controller/internal/webrtc"
	xclipboard "golang.design/x/clipboard"
)

// ConnectWebRTC initiates a WebRTC connection to the remote device
func (v *Viewer) ConnectWebRTC(supabaseURL, anonKey, authToken, userID string) error {
	log.Printf("🔗 Initiating WebRTC connection to device: %s", v.deviceID)

	// Store connection parameters for reconnection
	v.supabaseURL = supabaseURL
	v.anonKey = anonKey
	v.authToken = authToken
	v.userID = userID

	// Create WebRTC client
	client, err := rtc.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create WebRTC client: %w", err)
	}

	// Store client reference for input forwarding
	v.webrtcClient = client

	// Create signaling client
	signalingClient := rtc.NewSignalingClient(supabaseURL, anonKey, authToken)

	// Set up reconnection manager
	v.setupReconnection()

	// Set up callbacks
	client.SetOnFrame(func(frameData []byte) {
		// Track bandwidth
		v.TrackBytesReceived(len(frameData))
		v.handleVideoFrame(frameData)
	})

	client.SetOnConnected(func() {
		log.Println("✅ WebRTC connected!")
		v.connected = true
		fyne.Do(func() {
			v.statusLabel.SetText("🟢 Connected")
		})

		// Enable input forwarding
		v.setupInputForwarding()

		// Initialize file transfer
		v.InitializeFileTransfer()

		// Initialize clipboard receiver
		v.InitializeClipboard()

		// Start bandwidth update ticker
		go v.startBandwidthUpdater()

		// Start RTT measurement
		client.SetOnRTTUpdate(func(rtt time.Duration) {
			v.UpdateRTT(rtt)
		})
		client.StartPingLoop()
	})

	client.SetOnDisconnected(func() {
		log.Println("❌ WebRTC disconnected")
		v.connected = false
		fyne.Do(func() {
			v.statusLabel.SetText("🔴 Disconnected")
		})

		// Stop clipboard monitor
		if mon, ok := v.clipboardMonitor.(*clipboard.Monitor); ok {
			mon.Stop()
			v.clipboardMonitor = nil
		}

		// Start auto-reconnection
		if reconnMgr, ok := v.reconnectionMgr.(*reconnection.Manager); ok {
			if !reconnMgr.IsReconnecting() {
				log.Println("🔄 Starting auto-reconnection...")
				reconnMgr.StartReconnection()
			}
		}
	})

	// Create peer connection with STUN and TURN servers
	iceServers := []webrtc.ICEServer{
		{URLs: []string{"stun:stun.l.google.com:19302"}},
		{URLs: []string{"stun:stun1.l.google.com:19302"}},
		// TURN server for relay when direct connection fails (NAT traversal)
		{
			URLs:       []string{"turn:188.228.14.94:3478", "turn:188.228.14.94:3478?transport=tcp"},
			Username:   "remotedesktop",
			Credential: "Hawkeye2025Turn!",
		},
	}

	if err := client.CreatePeerConnection(iceServers); err != nil {
		return fmt.Errorf("failed to create peer connection: %w", err)
	}

	// Create session in Supabase
	log.Println("📝 Creating WebRTC session...")
	session, err := signalingClient.CreateSession(v.deviceID, userID)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	log.Printf("✅ Session created: %s", session.SessionID)

	// Create offer
	log.Println("📤 Creating WebRTC offer...")
	offerJSON, err := client.CreateOffer()
	if err != nil {
		return fmt.Errorf("failed to create offer: %w", err)
	}

	// Send offer to Supabase
	log.Println("📤 Sending offer to agent...")
	if err := signalingClient.SendOffer(session.SessionID, offerJSON); err != nil {
		return fmt.Errorf("failed to send offer: %w", err)
	}

	// Wait for answer from agent
	log.Println("⏳ Waiting for answer from agent...")
	fyne.Do(func() {
		v.statusLabel.SetText("⏳ Waiting for agent...")
	})

	go func() {
		answerJSON, err := signalingClient.WaitForAnswer(session.SessionID, 30*time.Second)
		if err != nil {
			log.Printf("❌ Failed to get answer: %v", err)
			v.window.Canvas().Refresh(v.statusLabel)
			return
		}

		log.Println("📨 Received answer from agent")

		// Set answer
		if err := client.SetAnswer(answerJSON); err != nil {
			log.Printf("❌ Failed to set answer: %v", err)
			v.window.Canvas().Refresh(v.statusLabel)
			return
		}

		log.Println("🎉 WebRTC handshake complete, waiting for connection...")
	}()

	return nil
}

// handleVideoFrame processes incoming video frames
func (v *Viewer) handleVideoFrame(frameData []byte) {
	// Decode JPEG frame
	img, err := jpeg.Decode(bytes.NewReader(frameData))
	if err != nil {
		log.Printf("⚠️  Failed to decode frame (%d bytes): %v", len(frameData), err)
		return
	}

	// Update canvas on UI thread
	v.updateCanvas(img)

	// Update FPS counter
	v.updateFPS()
}

var lastFrameTime time.Time
var frameCount int
var currentFPS int

// updateFPS calculates and updates the FPS counter
func (v *Viewer) updateFPS() {
	frameCount++

	now := time.Now()
	if lastFrameTime.IsZero() {
		lastFrameTime = now
		return
	}

	elapsed := now.Sub(lastFrameTime)
	if elapsed >= time.Second {
		currentFPS = frameCount
		frameCount = 0
		lastFrameTime = now

		// Update FPS label on UI thread
		fps := currentFPS
		fyne.Do(func() {
			v.fpsLabel.SetText(fmt.Sprintf("FPS: %d", fps))
		})
	}
}

// updateCanvas updates the video canvas with a new frame
func (v *Viewer) updateCanvas(img image.Image) {
	// Cache the frame for restore after minimize
	v.lastFrame = img

	// Update canvas on UI thread
	fyne.Do(func() {
		v.videoCanvas.Image = img
		v.videoCanvas.Refresh()
	})
}

// setupInputForwarding configures input event forwarding to the agent
func (v *Viewer) setupInputForwarding() {
	log.Println("🎮 Setting up input forwarding...")

	if v.interactiveCanvas == nil {
		log.Println("⚠️  Interactive canvas not initialized")
		return
	}

	// Hook up mouse move
	v.interactiveCanvas.SetOnMouseMove(func(x, y float32) {
		v.SendMouseMove(x, y)
		// Check if mouse is at top of screen for fullscreen overlay
		v.CheckMousePosition(y)
	})

	// Hook up mouse buttons
	v.interactiveCanvas.SetOnMouseButton(func(button desktop.MouseButton, pressed bool, x, y float32) {
		buttonInt := int(button)
		log.Printf("🖱️  Button event: button=%d (%v) pressed=%v", buttonInt, button, pressed)
		v.SendMouseButton(buttonInt, pressed, x, y)
	})

	// Hook up mouse scroll
	v.interactiveCanvas.SetOnMouseScroll(func(deltaX, deltaY float32) {
		v.SendMouseScroll(deltaX, deltaY)
	})

	// Hook up keyboard
	v.interactiveCanvas.SetOnKeyPress(func(key *fyne.KeyEvent) {
		// Intercept ESC for local fullscreen exit
		if key.Name == fyne.KeyEscape && v.fullscreen {
			log.Println("🔑 ESC pressed - exiting fullscreen locally")
			fyne.Do(func() {
				v.toggleFullscreen()
			})
			return // Don't send to remote
		}

		// Intercept F11 for local fullscreen toggle
		if key.Name == fyne.KeyF11 {
			log.Println("🔑 F11 pressed - toggling fullscreen locally")
			fyne.Do(func() {
				v.toggleFullscreen()
			})
			return // Don't send to remote
		}

		// Map Fyne key names to JavaScript KeyboardEvent.code format
		jsCode := mapFyneKeyToJSCode(key.Name)
		if jsCode != "" {
			// Send key down and immediately key up (tap)
			v.SendKeyPress(jsCode, true)
			v.SendKeyPress(jsCode, false)
		}
	})

	// Request focus so keyboard events are captured
	v.window.Canvas().Focus(v.interactiveCanvas)

	log.Println("✅ Input forwarding enabled")
}

// SendMouseMove sends a mouse move event to the agent
func (v *Viewer) SendMouseMove(x, y float32) {
	if v.webrtcClient == nil || v.interactiveCanvas == nil || v.videoCanvas == nil {
		return
	}

	// Get actual remote screen resolution from the video image
	img := v.videoCanvas.Image
	if img == nil {
		return
	}

	bounds := img.Bounds()
	imgWidth := float64(bounds.Dx())
	imgHeight := float64(bounds.Dy())

	// Get canvas display size
	canvasSize := v.interactiveCanvas.Size()
	displayWidth := float64(canvasSize.Width)
	displayHeight := float64(canvasSize.Height)

	// Avoid division by zero
	if displayWidth <= 0 || displayHeight <= 0 || imgWidth <= 0 || imgHeight <= 0 {
		return
	}

	// ImageFillContain: image is scaled to fit while maintaining aspect ratio
	// Calculate scale factor (use smaller of the two to fit entirely)
	scaleX := displayWidth / imgWidth
	scaleY := displayHeight / imgHeight
	scale := math.Min(scaleX, scaleY)

	// Calculate rendered image size
	renderWidth := imgWidth * scale
	renderHeight := imgHeight * scale

	// Calculate offset (image is centered)
	offsetX := (displayWidth - renderWidth) / 2
	offsetY := (displayHeight - renderHeight) / 2

	// Convert canvas coordinates to image coordinates
	// First subtract offset, then divide by scale
	imageX := (float64(x) - offsetX) / scale
	imageY := (float64(y) - offsetY) / scale

	// Round to nearest pixel
	remoteX := math.Round(imageX)
	remoteY := math.Round(imageY)

	// Clamp to remote screen bounds
	if remoteX < 0 {
		remoteX = 0
	}
	if remoteY < 0 {
		remoteY = 0
	}
	if remoteX >= imgWidth {
		remoteX = imgWidth - 1
	}
	if remoteY >= imgHeight {
		remoteY = imgHeight - 1
	}

	event := map[string]interface{}{
		"t": "mouse_move",
		"x": remoteX,
		"y": remoteY,
	}

	eventJSON, _ := json.Marshal(event)

	// Send via WebRTC data channel
	if client, ok := v.webrtcClient.(*rtc.Client); ok {
		client.SendInput(string(eventJSON))
	}
}

// SendMouseButton sends a mouse button event to the agent
func (v *Viewer) SendMouseButton(button int, pressed bool, x, y float32) {
	if v.webrtcClient == nil || v.interactiveCanvas == nil || v.videoCanvas == nil {
		return
	}

	// Get actual remote screen resolution from the video image
	img := v.videoCanvas.Image
	if img == nil {
		return
	}

	bounds := img.Bounds()
	imgWidth := float64(bounds.Dx())
	imgHeight := float64(bounds.Dy())

	// Get canvas display size
	canvasSize := v.interactiveCanvas.Size()
	displayWidth := float64(canvasSize.Width)
	displayHeight := float64(canvasSize.Height)

	// Avoid division by zero
	if displayWidth <= 0 || displayHeight <= 0 || imgWidth <= 0 || imgHeight <= 0 {
		return
	}

	// ImageFillContain: image is scaled to fit while maintaining aspect ratio
	// Calculate scale factor (use smaller of the two to fit entirely)
	scaleX := displayWidth / imgWidth
	scaleY := displayHeight / imgHeight
	scale := math.Min(scaleX, scaleY)

	// Calculate rendered image size
	renderWidth := imgWidth * scale
	renderHeight := imgHeight * scale

	// Calculate offset (image is centered)
	offsetX := (displayWidth - renderWidth) / 2
	offsetY := (displayHeight - renderHeight) / 2

	// Convert canvas coordinates to image coordinates
	imageX := (float64(x) - offsetX) / scale
	imageY := (float64(y) - offsetY) / scale

	// Round to nearest pixel
	remoteX := math.Round(imageX)
	remoteY := math.Round(imageY)

	// Clamp to remote screen bounds
	if remoteX < 0 {
		remoteX = 0
	}
	if remoteY < 0 {
		remoteY = 0
	}
	if remoteX >= imgWidth {
		remoteX = imgWidth - 1
	}
	if remoteY >= imgHeight {
		remoteY = imgHeight - 1
	}

	// Map Fyne button to string
	buttonStr := "left"
	if button == 1 {
		buttonStr = "left"
	} else if button == 2 {
		buttonStr = "right"
	} else if button == 3 {
		buttonStr = "middle"
	}

	event := map[string]interface{}{
		"t":      "mouse_click",
		"button": buttonStr,
		"down":   pressed,
		"x":      remoteX,
		"y":      remoteY,
	}

	eventJSON, _ := json.Marshal(event)

	if client, ok := v.webrtcClient.(*rtc.Client); ok {
		client.SendInput(string(eventJSON))
	}
}

// SendMouseScroll sends a mouse scroll event to the agent
func (v *Viewer) SendMouseScroll(deltaX, deltaY float32) {
	if v.webrtcClient == nil {
		return
	}

	event := map[string]interface{}{
		"t":     "mouse_scroll",
		"delta": deltaY, // Use Y delta for vertical scrolling
	}

	eventJSON, _ := json.Marshal(event)

	if client, ok := v.webrtcClient.(*rtc.Client); ok {
		client.SendInput(string(eventJSON))
	}
}

// SendKeyPress sends a keyboard event to the agent
func (v *Viewer) SendKeyPress(key string, pressed bool) {
	if v.webrtcClient == nil {
		return
	}

	event := map[string]interface{}{
		"t":    "key",
		"code": key,
		"down": pressed,
	}

	eventJSON, _ := json.Marshal(event)

	if client, ok := v.webrtcClient.(*rtc.Client); ok {
		client.SendInput(string(eventJSON))
	}
}

// mapFyneKeyToJSCode maps Fyne key names to JavaScript KeyboardEvent.code format
func mapFyneKeyToJSCode(keyName fyne.KeyName) string {
	keyMap := map[fyne.KeyName]string{
		// Special keys
		fyne.KeyTab:       "Tab",
		fyne.KeyReturn:    "Enter",
		fyne.KeyEnter:     "Enter",
		fyne.KeySpace:     "Space",
		fyne.KeyBackspace: "Backspace",
		fyne.KeyDelete:    "Delete",
		fyne.KeyInsert:    "Insert",
		fyne.KeyHome:      "Home",
		fyne.KeyEnd:       "End",
		fyne.KeyPageUp:    "PageUp",
		fyne.KeyPageDown:  "PageDown",
		fyne.KeyEscape:    "Escape",

		// Arrow keys
		fyne.KeyUp:    "ArrowUp",
		fyne.KeyDown:  "ArrowDown",
		fyne.KeyLeft:  "ArrowLeft",
		fyne.KeyRight: "ArrowRight",

		// Function keys
		fyne.KeyF1:  "F1",
		fyne.KeyF2:  "F2",
		fyne.KeyF3:  "F3",
		fyne.KeyF4:  "F4",
		fyne.KeyF5:  "F5",
		fyne.KeyF6:  "F6",
		fyne.KeyF7:  "F7",
		fyne.KeyF8:  "F8",
		fyne.KeyF9:  "F9",
		fyne.KeyF10: "F10",
		fyne.KeyF11: "F11",
		fyne.KeyF12: "F12",
	}

	if code, ok := keyMap[keyName]; ok {
		return code
	}

	// For letters and numbers, convert to JavaScript format
	name := string(keyName)
	if len(name) == 1 {
		char := name[0]
		// Letters A-Z
		if char >= 'A' && char <= 'Z' {
			return "Key" + name
		}
		if char >= 'a' && char <= 'z' {
			return "Key" + string(char-32) // Convert to uppercase
		}
		// Numbers 0-9
		if char >= '0' && char <= '9' {
			return "Digit" + name
		}
	}

	// Return empty for unmapped keys
	return ""
}

// InitializeFileTransfer sets up the file transfer manager
func (v *Viewer) InitializeFileTransfer() {
	ftManager := filetransfer.NewManager()

	// Set callback to send data via WebRTC data channel
	ftManager.SetSendDataCallback(func(data []byte) error {
		if client, ok := v.webrtcClient.(*rtc.Client); ok {
			return client.SendData(data)
		}
		return fmt.Errorf("WebRTC client not available")
	})

	// Set callback for new transfers
	ftManager.SetOnTransferCallback(func(transfer *filetransfer.Transfer) {
		log.Printf("📁 New transfer: %s (%d bytes)", transfer.Filename, transfer.Size)
		// TODO: Show transfer progress in UI
	})

	v.fileTransferMgr = ftManager
	log.Println("✅ File transfer initialized")
}

// SendFile opens a file picker and sends the selected file
func (v *Viewer) SendFile() {
	// Check if connected
	if !v.connected {
		dialog.ShowInformation("Not Connected", "Please connect to a device first", v.window)
		return
	}

	// Check if file transfer manager is initialized
	if v.fileTransferMgr == nil {
		dialog.ShowInformation("Not Ready", "File transfer is not ready yet. Please wait.", v.window)
		return
	}

	fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, v.window)
			return
		}
		if reader == nil {
			return // User cancelled
		}
		defer reader.Close()

		filePath := reader.URI().Path()
		log.Printf("📤 Sending file: %s", filePath)

		if ftMgr, ok := v.fileTransferMgr.(*filetransfer.Manager); ok {
			transfer, err := ftMgr.SendFile(filePath)
			if err != nil {
				dialog.ShowError(err, v.window)
				return
			}

			// Show progress dialog
			v.showFileTransferProgress(transfer)
		} else {
			dialog.ShowError(fmt.Errorf("file transfer not available"), v.window)
		}
	}, v.window)

	fileDialog.Show()
}

// showFileTransferProgress shows a progress dialog for file transfer
func (v *Viewer) showFileTransferProgress(transfer *filetransfer.Transfer) {
	progressDialog := dialog.NewCustom(
		"File Transfer",
		"Cancel",
		nil, // TODO: Add progress bar widget
		v.window,
	)

	// Set up progress callback
	transfer.SetOnProgress(func(progress, total int64) {
		// TODO: Update progress bar
		log.Printf("📊 Transfer progress: %d%%", (progress*100)/total)
	})

	// Set up completion callback
	transfer.SetOnComplete(func(success bool, err error) {
		progressDialog.Hide()
		if success {
			dialog.ShowInformation("Success", "File transferred successfully!", v.window)
		} else {
			dialog.ShowError(err, v.window)
		}
	})

	progressDialog.Show()
}

// HandleFileTransferData processes incoming file transfer data
func (v *Viewer) HandleFileTransferData(data []byte) error {
	if ftMgr, ok := v.fileTransferMgr.(*filetransfer.Manager); ok {
		return ftMgr.HandleIncomingData(data)
	}
	return fmt.Errorf("file transfer manager not initialized")
}

// OpenFileBrowser opens the Total Commander-style file browser
func (v *Viewer) OpenFileBrowser() {
	// Recover from any panics to prevent app crash
	defer func() {
		if r := recover(); r != nil {
			log.Printf("❌ OpenFileBrowser panic recovered: %v", r)
			fyne.Do(func() {
				dialog.ShowError(fmt.Errorf("File browser error: %v", r), v.window)
			})
		}
	}()

	if !v.connected {
		dialog.ShowInformation("Not Connected", "Please connect to a device first", v.window)
		return
	}

	log.Println("📁 Opening TotalCMD-style file browser...")

	// Get WebRTC client
	client, ok := v.webrtcClient.(*rtc.Client)
	if !ok {
		dialog.ShowError(fmt.Errorf("WebRTC client not available"), v.window)
		return
	}

	// Create file transfer manager
	ftManager := filetransfer.NewManager()
	
	// Set send function to use file channel
	ftManager.SetSendFunc(func(data []byte) error {
		return client.SendFileData(data)
	})
	
	// Set up file channel message handler
	client.SetOnFileMessage(func(data []byte) {
		ftManager.HandleMessage(data)
	})

	// Get app from window
	app := fyne.CurrentApp()
	
	// Create new TotalCMD-style file browser
	fb := filetransfer.NewFileBrowser(app, ftManager, func() {
		log.Println("📁 File browser closed")
	})
	
	if fb == nil {
		log.Println("❌ Failed to create file browser")
		dialog.ShowError(fmt.Errorf("Failed to create file browser"), v.window)
		return
	}

	fb.Show()
}

// requestRemoteDirListing requests directory listing from remote agent
func (v *Viewer) requestRemoteDirListing(path string) {
	if client, ok := v.webrtcClient.(*rtc.Client); ok {
		request := map[string]string{
			"type": "dir_list",
			"path": path,
		}
		data, _ := json.Marshal(request)
		if err := client.SendData(data); err != nil {
			log.Printf("❌ Failed to request dir listing: %v", err)
		} else {
			log.Printf("📂 Requested dir listing: %s", path)
		}
	}
}

// requestRemoteDrives requests drive list from remote agent
func (v *Viewer) requestRemoteDrives() {
	if client, ok := v.webrtcClient.(*rtc.Client); ok {
		request := map[string]string{
			"type": "drives_list",
		}
		data, _ := json.Marshal(request)
		if err := client.SendData(data); err != nil {
			log.Printf("❌ Failed to request drives: %v", err)
		} else {
			log.Println("💾 Requested remote drives")
		}
	}
}

// sendFileToRemote sends a file to the remote agent
func (v *Viewer) sendFileToRemote(localPath, remotePath string) error {
	log.Printf("📤 Sending file: %s -> %s", localPath, remotePath)

	if ftMgr, ok := v.fileTransferMgr.(*filetransfer.Manager); ok {
		_, err := ftMgr.SendFile(localPath)
		return err
	}
	return fmt.Errorf("file transfer not available")
}

// receiveFileFromRemote requests a file from the remote agent
func (v *Viewer) receiveFileFromRemote(remotePath, localPath string) error {
	log.Printf("📥 Requesting file: %s -> %s", remotePath, localPath)

	if client, ok := v.webrtcClient.(*rtc.Client); ok {
		request := map[string]interface{}{
			"type":       "file_request",
			"remotePath": remotePath,
			"localPath":  localPath,
		}
		data, _ := json.Marshal(request)
		return client.SendData(data)
	}
	return fmt.Errorf("not connected")
}

// handleFileBrowserMessage handles file browser related messages from agent
func (v *Viewer) handleFileBrowserMessage(data map[string]interface{}) {
	msgType, _ := data["type"].(string)

	switch msgType {
	case "dir_list_response":
		// Parse files from response
		filesData, _ := json.Marshal(data["files"])
		var files []filebrowser.FileInfo
		json.Unmarshal(filesData, &files)

		if fb, ok := v.fileBrowser.(*filebrowser.FileBrowser); ok {
			fb.SetRemoteFiles(files)
		}

	case "drives_list_response":
		// Parse drives from response
		drivesData, _ := data["drives"].([]interface{})
		drives := make([]string, 0, len(drivesData))
		for _, d := range drivesData {
			if drive, ok := d.(string); ok {
				drives = append(drives, drive)
			}
		}

		if fb, ok := v.fileBrowser.(*filebrowser.FileBrowser); ok {
			fb.SetRemoteDrives(drives)
		}
	}
}

// InitializeClipboard initializes the clipboard receiver and monitor for bidirectional sync
func (v *Viewer) InitializeClipboard() {
	log.Println("📋 Initializing clipboard sync...")

	receiver := clipboard.NewReceiver()
	if err := receiver.Initialize(); err != nil {
		log.Printf("❌ Failed to initialize clipboard receiver: %v", err)
		return
	}

	v.clipboardReceiver = receiver
	log.Println("✅ Clipboard receiver initialized")

	// Start monitoring local clipboard to push updates to agent
	mon := clipboard.NewMonitor()
	mon.SetOnTextChange(func(text string) {
		log.Printf("📋 Local clipboard changed, sending to agent (%d bytes)", len(text))
		v.sendClipboardText(text)
	})
	mon.SetOnImageChange(func(img []byte) {
		log.Printf("📋 Local clipboard image changed, sending to agent (%d bytes)", len(img))
		v.sendClipboardImage(img)
	})
	if err := mon.Start(); err != nil {
		log.Printf("❌ Failed to start clipboard monitor: %v", err)
	} else {
		v.clipboardMonitor = mon
		log.Println("✅ Clipboard monitor started - auto-sync enabled!")
	}

	// Set up message handler for clipboard data
	if client, ok := v.webrtcClient.(*rtc.Client); ok {
		client.SetOnDataChannelMessage(func(msg []byte) {
			v.handleDataChannelMessage(msg)
		})
		log.Println("✅ Clipboard message handler registered")
	}
}

// handleDataChannelMessage processes incoming data channel messages
func (v *Viewer) handleDataChannelMessage(msg []byte) {
	var data map[string]interface{}
	if err := json.Unmarshal(msg, &data); err != nil {
		return
	}

	msgType, ok := data["type"].(string)
	if !ok {
		return
	}

	switch msgType {
	case "clipboard_text":
		if content, ok := data["content"].(string); ok {
			log.Printf("📋 Received clipboard text from agent (%d bytes)", len(content))
			if receiver, ok := v.clipboardReceiver.(*clipboard.Receiver); ok {
				if err := receiver.SetText(content); err != nil {
					log.Printf("❌ Failed to set clipboard text: %v", err)
				} else {
					log.Println("✅ Clipboard text set locally")
					if mon, ok := v.clipboardMonitor.(*clipboard.Monitor); ok {
						mon.RememberText(content)
					}
				}
			}
		}

	case "clipboard_image":
		if contentB64, ok := data["content"].(string); ok {
			imageData, err := base64.StdEncoding.DecodeString(contentB64)
			if err != nil {
				log.Printf("? Failed to decode clipboard image: %v", err)
				return
			}

			if receiver, ok := v.clipboardReceiver.(*clipboard.Receiver); ok {
				if err := receiver.SetImageRaw(imageData); err != nil {
					log.Printf("? Failed to set clipboard image: %v", err)
				} else if mon, ok := v.clipboardMonitor.(*clipboard.Monitor); ok {
					mon.RememberImage(imageData)
				}
			}
		}

	case "dir_list_response", "drives_list_response":
		// Handle file browser responses
		v.handleFileBrowserMessage(data)

	case "stats":
		// Handle streaming stats from agent
		if mode, ok := data["mode"].(string); ok {
			v.UpdateStreamingMode(mode)
		}
		if fps, ok := data["fps"].(float64); ok {
			if rtt, ok := data["rtt"].(float64); ok {
				v.UpdateStats(int(fps), int(rtt))
			}
		}
		// Extended stats: quality, scale, cpu
		if quality, ok := data["quality"].(float64); ok {
			v.UpdateQuality(int(quality))
		}
		if scale, ok := data["scale"].(float64); ok {
			v.UpdateScale(scale)
		}
		if cpu, ok := data["cpu"].(float64); ok {
			v.UpdateCPU(cpu)
		}

	case "file_transfer_start", "file_transfer_chunk", "file_transfer_complete", "file_transfer_error":
		// Route file transfer messages to handler
		v.HandleFileTransferData(msg)
	}
}

// sendClipboardText pushes local text clipboard to the agent.
func (v *Viewer) sendClipboardText(text string) {
	if !v.connected {
		return
	}

	msg := map[string]interface{}{
		"type":    "clipboard_text",
		"content": text,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("? Failed to marshal clipboard text: %v", err)
		return
	}

	if client, ok := v.webrtcClient.(*rtc.Client); ok {
		if err := client.SendInput(string(data)); err != nil {
			log.Printf("? Failed to send clipboard text: %v", err)
		}
	}
}

// sendClipboardImage pushes local image clipboard to the agent.
func (v *Viewer) sendClipboardImage(img []byte) {
	if !v.connected {
		return
	}

	imgB64 := base64.StdEncoding.EncodeToString(img)
	msg := map[string]interface{}{
		"type":    "clipboard_image",
		"content": imgB64,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("? Failed to marshal clipboard image: %v", err)
		return
	}

	if client, ok := v.webrtcClient.(*rtc.Client); ok {
		if err := client.SendInput(string(data)); err != nil {
			log.Printf("? Failed to send clipboard image: %v", err)
		}
	}
}

// sendClipboardNow performs a one-shot sync of current local clipboard.
func (v *Viewer) sendClipboardNow() {
	if err := xclipboard.Init(); err != nil {
		log.Printf("? Clipboard init failed: %v", err)
		return
	}

	text := xclipboard.Read(xclipboard.FmtText)
	if len(text) > 0 {
		v.sendClipboardText(string(text))
		if mon, ok := v.clipboardMonitor.(*clipboard.Monitor); ok {
			mon.RememberText(string(text))
		}
		return
	}

	img := xclipboard.Read(xclipboard.FmtImage)
	if len(img) > 0 {
		v.sendClipboardImage(img)
		if mon, ok := v.clipboardMonitor.(*clipboard.Monitor); ok {
			mon.RememberImage(img)
		}
	}
}

// setupReconnection initializes the reconnection manager
func (v *Viewer) setupReconnection() {
	reconnMgr := reconnection.NewManager()

	// Set reconnection function
	reconnMgr.SetReconnectFunc(func() error {
		log.Println("🔄 Attempting to reconnect...")
		return v.ConnectWebRTC(v.supabaseURL, v.anonKey, v.authToken, v.userID)
	})

	// Set callbacks
	reconnMgr.SetOnReconnecting(func(attempt int, maxAttempts int, nextDelay time.Duration) {
		statusText := fmt.Sprintf("🔄 Reconnecting... (%d/%d)", attempt, maxAttempts)
		fyne.Do(func() {
			v.statusLabel.SetText(statusText)
		})
		log.Printf("🔄 Reconnection attempt %d/%d, next attempt in %v", attempt, maxAttempts, nextDelay)
	})

	reconnMgr.SetOnReconnected(func() {
		log.Println("✅ Reconnection successful!")
		fyne.Do(func() {
			v.statusLabel.SetText("🟢 Connected")
			dialog.ShowInformation("Reconnected", "Connection restored successfully!", v.window)
		})
	})

	reconnMgr.SetOnReconnectFailed(func() {
		log.Println("❌ Reconnection failed after all attempts")
		fyne.Do(func() {
			v.statusLabel.SetText("❌ Connection Failed")
		})
		dialog.ShowError(
			fmt.Errorf("failed to reconnect after %d attempts", reconnMgr.GetMaxRetries()),
			v.window,
		)
	})

	v.reconnectionMgr = reconnMgr
	log.Println("✅ Reconnection manager initialized")
}

// CancelReconnection stops any ongoing reconnection attempts
func (v *Viewer) CancelReconnection() {
	if reconnMgr, ok := v.reconnectionMgr.(*reconnection.Manager); ok {
		reconnMgr.Cancel()
	}
}

// startBandwidthUpdater updates bandwidth display every second
func (v *Viewer) startBandwidthUpdater() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if !v.connected {
			return // Stop when disconnected
		}
		v.UpdateBandwidth()
	}
}
