package webrtc

import (
	"encoding/base64"
	"encoding/json"
	"log"

	pionwebrtc "github.com/pion/webrtc/v3"
	"github.com/stangtennis/remote-agent/internal/clipboard"
)

// handleClipboardText handles incoming clipboard text from controller.
// On Windows when running as a service we route writes through the
// user-session helper so the user's clipboard is updated (per-session).
func (m *Manager) handleClipboardText(content string) {
	if m.clipboardSessionHelper != nil {
		if err := m.clipboardSessionHelper.SetText(content); err != nil {
			log.Printf("❌ Helper SetText failed: %v", err)
		} else {
			log.Println("✅ Clipboard text set via session helper")
		}
		return
	}
	if m.clipboardReceiver == nil {
		m.clipboardReceiver = clipboard.NewReceiver()
	}
	if err := m.clipboardReceiver.SetText(content); err != nil {
		log.Printf("❌ Failed to set clipboard text on agent: %v", err)
	} else {
		log.Println("✅ Clipboard text set on agent")
		if m.clipboardMonitor != nil {
			m.clipboardMonitor.RememberText(content)
		}
	}
}

// handleClipboardImage handles incoming clipboard image from controller.
func (m *Manager) handleClipboardImage(contentB64 string) {
	imageData, err := base64.StdEncoding.DecodeString(contentB64)
	if err != nil {
		log.Printf("❌ Failed to decode clipboard image: %v", err)
		return
	}
	if m.clipboardSessionHelper != nil {
		if err := m.clipboardSessionHelper.SetImage(imageData); err != nil {
			log.Printf("❌ Helper SetImage failed: %v", err)
		} else {
			log.Println("✅ Clipboard image set via session helper")
		}
		return
	}
	if m.clipboardReceiver == nil {
		m.clipboardReceiver = clipboard.NewReceiver()
	}
	if err := m.clipboardReceiver.SetImage(imageData); err != nil {
		log.Printf("❌ Failed to set clipboard image on agent: %v", err)
	} else {
		log.Println("✅ Clipboard image set on agent")
		if m.clipboardMonitor != nil {
			m.clipboardMonitor.RememberImage(imageData)
		}
	}
}

// startClipboardMonitoring initializes and starts clipboard monitoring.
//
// KNOWN LIMITATION on Windows: when the agent runs as a service in
// Session 0, the per-session Windows clipboard means clipboard.Watch in
// Session 0 doesn't see the user's copies in Session 1+. A user-session
// bridge helper is sketched in clipboard/helper_windows.go but not yet
// reliable (clipboard.Read inside the helper hangs in some cases). For
// now: copy/paste from the dashboard works only when the agent runs in
// the user session (console mode, autostart-from-Program-Files mode).
// Service mode users should use the controller app instead.
func (m *Manager) startClipboardMonitoring() {
	m.clipboardMonitor = clipboard.NewMonitor()

	// Set up text clipboard callback
	m.clipboardMonitor.SetOnTextChange(func(text string) {
		if m.dataChannel == nil || m.dataChannel.ReadyState() != pionwebrtc.DataChannelStateOpen {
			return
		}

		// Send text clipboard to controller
		msg := map[string]interface{}{
			"type":    "clipboard_text",
			"content": text,
		}

		data, err := json.Marshal(msg)
		if err != nil {
			log.Printf("❌ Failed to marshal clipboard text: %v", err)
			return
		}

		if err := m.dataChannel.Send(data); err != nil {
			log.Printf("❌ Failed to send clipboard text: %v", err)
		}
	})

	// Set up image clipboard callback
	m.clipboardMonitor.SetOnImageChange(func(imageData []byte) {
		if m.dataChannel == nil || m.dataChannel.ReadyState() != pionwebrtc.DataChannelStateOpen {
			return
		}

		// Encode image to base64 for JSON transmission
		imageB64 := base64.StdEncoding.EncodeToString(imageData)

		// Send image clipboard to controller
		msg := map[string]interface{}{
			"type":    "clipboard_image",
			"content": imageB64,
		}

		data, err := json.Marshal(msg)
		if err != nil {
			log.Printf("❌ Failed to marshal clipboard image: %v", err)
			return
		}

		if err := m.dataChannel.Send(data); err != nil {
			log.Printf("❌ Failed to send clipboard image: %v", err)
		}
	})

	// Start monitoring
	if err := m.clipboardMonitor.Start(); err != nil {
		log.Printf("❌ Failed to start clipboard monitor: %v", err)
	}
}
