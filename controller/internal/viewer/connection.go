package viewer

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"time"

	rtc "github.com/stangtennis/Remote/controller/internal/webrtc"
	"github.com/stangtennis/Remote/controller/internal/clipboard"
	"github.com/stangtennis/Remote/controller/internal/filetransfer"
	"github.com/stangtennis/Remote/controller/internal/reconnection"
	"github.com/pion/webrtc/v3"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
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
		v.handleVideoFrame(frameData)
	})

	client.SetOnConnected(func() {
		log.Println("✅ WebRTC connected!")
		v.connected = true
		v.statusLabel.SetText("🟢 Connected")
		
		// Enable input forwarding
		v.setupInputForwarding()
		
		// Initialize file transfer
		v.InitializeFileTransfer()
		
		// Initialize clipboard receiver
		v.InitializeClipboard()
	})

	client.SetOnDisconnected(func() {
		log.Println("❌ WebRTC disconnected")
		v.connected = false
		v.statusLabel.SetText("🔴 Disconnected")
		
		// Start auto-reconnection
		if reconnMgr, ok := v.reconnectionMgr.(*reconnection.Manager); ok {
			if !reconnMgr.IsReconnecting() {
				log.Println("🔄 Starting auto-reconnection...")
				reconnMgr.StartReconnection()
			}
		}
	})

	// Create peer connection with STUN servers
	iceServers := []webrtc.ICEServer{
		{URLs: []string{"stun:stun.l.google.com:19302"}},
		{URLs: []string{"stun:stun1.l.google.com:19302"}},
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
	v.statusLabel.SetText("⏳ Waiting for agent...")
	
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
		log.Printf("⚠️  Failed to decode frame: %v", err)
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
		
		// Send other keys to remote
		v.SendKeyPress(string(key.Name), true)
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
	imgWidth := float32(bounds.Dx())
	imgHeight := float32(bounds.Dy())
	
	// Get canvas display size
	canvasSize := v.interactiveCanvas.Size()
	
	// Calculate how the image is scaled within the canvas (ImageFillContain)
	canvasAspect := canvasSize.Width / canvasSize.Height
	imgAspect := imgWidth / imgHeight
	
	var scaledWidth, scaledHeight, offsetX, offsetY float32
	
	if canvasAspect > imgAspect {
		// Canvas is wider - pillarboxing
		scaledHeight = canvasSize.Height
		scaledWidth = imgWidth * (canvasSize.Height / imgHeight)
		offsetX = (canvasSize.Width - scaledWidth) / 2
		offsetY = 0
	} else {
		// Canvas is taller - letterboxing
		scaledWidth = canvasSize.Width
		scaledHeight = imgHeight * (canvasSize.Width / imgWidth)
		offsetX = 0
		offsetY = (canvasSize.Height - scaledHeight) / 2
	}
	
	// Adjust coordinates
	adjustedX := x - offsetX
	adjustedY := y - offsetY
	
	// Convert to remote coordinates
	remoteX := (adjustedX / scaledWidth) * imgWidth
	remoteY := (adjustedY / scaledHeight) * imgHeight
	
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
	imgWidth := float32(bounds.Dx())
	imgHeight := float32(bounds.Dy())
	
	// Get canvas display size
	canvasSize := v.interactiveCanvas.Size()
	
	// Calculate how the image is scaled within the canvas (ImageFillContain)
	// The image maintains aspect ratio and is centered
	canvasAspect := canvasSize.Width / canvasSize.Height
	imgAspect := imgWidth / imgHeight
	
	var scaledWidth, scaledHeight, offsetX, offsetY float32
	
	if canvasAspect > imgAspect {
		// Canvas is wider - image is limited by height (pillarboxing)
		scaledHeight = canvasSize.Height
		scaledWidth = imgWidth * (canvasSize.Height / imgHeight)
		offsetX = (canvasSize.Width - scaledWidth) / 2
		offsetY = 0
	} else {
		// Canvas is taller - image is limited by width (letterboxing)
		scaledWidth = canvasSize.Width
		scaledHeight = imgHeight * (canvasSize.Width / imgWidth)
		offsetX = 0
		offsetY = (canvasSize.Height - scaledHeight) / 2
	}
	
	// Adjust click coordinates to account for scaling and offset
	adjustedX := x - offsetX
	adjustedY := y - offsetY
	
	// Convert to remote coordinates
	remoteX := (adjustedX / scaledWidth) * imgWidth
	remoteY := (adjustedY / scaledHeight) * imgHeight
	
	// Debug logging
	log.Printf("🖱️  Click: canvas=(%.0f,%.0f) canvasSize=(%.0fx%.0f) scaled=(%.0fx%.0f) offset=(%.0f,%.0f) adjusted=(%.0f,%.0f) remote=(%.0f,%.0f) imgSize=(%.0fx%.0f)",
		x, y, canvasSize.Width, canvasSize.Height, scaledWidth, scaledHeight, offsetX, offsetY, adjustedX, adjustedY, remoteX, remoteY, imgWidth, imgHeight)
	
	// Map Fyne button to string
	// Fyne uses: MouseButtonPrimary=1 (left), MouseButtonSecondary=2 (right), MouseButtonTertiary=3 (middle)
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

// InitializeFileTransfer sets up the file transfer manager
func (v *Viewer) InitializeFileTransfer() {
	ftManager := filetransfer.NewManager()
	
	// Set callback to send data via WebRTC
	ftManager.SetSendDataCallback(func(data []byte) error {
		if client, ok := v.webrtcClient.(*rtc.Client); ok {
			return client.SendInput(string(data))
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

// InitializeClipboard initializes the clipboard receiver
func (v *Viewer) InitializeClipboard() {
	receiver := clipboard.NewReceiver()
	if err := receiver.Initialize(); err != nil {
		log.Printf("❌ Failed to initialize clipboard receiver: %v", err)
		return
	}
	
	v.clipboardReceiver = receiver
	log.Println("✅ Clipboard receiver initialized")
	
	// Set up message handler for clipboard data
	if client, ok := v.webrtcClient.(*rtc.Client); ok {
		client.SetOnDataChannelMessage(func(msg []byte) {
			v.handleDataChannelMessage(msg)
		})
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
			if receiver, ok := v.clipboardReceiver.(*clipboard.Receiver); ok {
				if err := receiver.SetText(content); err != nil {
					log.Printf("❌ Failed to set clipboard text: %v", err)
				}
			}
		}
		
	case "clipboard_image":
		if contentB64, ok := data["content"].(string); ok {
			imageData, err := base64.StdEncoding.DecodeString(contentB64)
			if err != nil {
				log.Printf("❌ Failed to decode clipboard image: %v", err)
				return
			}
			
			if receiver, ok := v.clipboardReceiver.(*clipboard.Receiver); ok {
				if err := receiver.SetImageRaw(imageData); err != nil {
					log.Printf("❌ Failed to set clipboard image: %v", err)
				}
			}
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
		v.statusLabel.SetText(statusText)
		log.Printf("🔄 Reconnection attempt %d/%d, next attempt in %v", attempt, maxAttempts, nextDelay)
	})
	
	reconnMgr.SetOnReconnected(func() {
		log.Println("✅ Reconnection successful!")
		v.statusLabel.SetText("🟢 Connected")
		dialog.ShowInformation("Reconnected", "Connection restored successfully!", v.window)
	})
	
	reconnMgr.SetOnReconnectFailed(func() {
		log.Println("❌ Reconnection failed after all attempts")
		v.statusLabel.SetText("❌ Connection Failed")
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
