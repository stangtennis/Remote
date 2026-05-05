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
// On Windows when running as a service in Session 0, the OS clipboard is
// per-session — a clipboard.Watch in Session 0 never sees the user's
// copies in Session 1+. We bridge it with a small helper process spawned
// in the active console user's session via CreateProcessAsUser; the
// helper polls GetClipboardSequenceNumber + raw OpenClipboard and
// forwards events back over a named pipe.
func (m *Manager) startClipboardMonitoring() {
	m.clipboardMu.Lock()
	if m.clipboardStarting || m.clipboardMonitor != nil || m.clipboardSessionHelper != nil {
		m.clipboardMu.Unlock()
		return
	}
	m.clipboardStarting = true
	m.clipboardMu.Unlock()

	defer func() {
		m.clipboardMu.Lock()
		m.clipboardStarting = false
		m.clipboardMu.Unlock()
	}()

	send := func(msg map[string]interface{}) {
		if m.dataChannel == nil || m.dataChannel.ReadyState() != pionwebrtc.DataChannelStateOpen {
			return
		}
		data, err := json.Marshal(msg)
		if err != nil {
			log.Printf("❌ Failed to marshal clipboard msg: %v", err)
			return
		}
		if err := m.dataChannel.Send(data); err != nil {
			log.Printf("❌ Failed to send clipboard msg: %v", err)
		}
	}

	if m.startedInSession0 {
		log.Println("📋 Spawning clipboard helper in user session (Session 0 service can't see user's clipboard directly)")
		helper := clipboard.NewSessionHelper()
		helper.SetOnTextChange(func(text string) {
			send(map[string]interface{}{"type": "clipboard_text", "content": text})
		})
		helper.SetOnImageChange(func(imageData []byte) {
			send(map[string]interface{}{"type": "clipboard_image", "content": base64.StdEncoding.EncodeToString(imageData)})
		})
		if err := helper.Start(); err != nil {
			log.Printf("⚠️  Clipboard session helper failed to start: %v — falling back to in-process monitor", err)
		} else {
			m.clipboardMu.Lock()
			m.clipboardSessionHelper = helper
			m.clipboardMu.Unlock()
			return
		}
	}

	// In-process monitor (used when agent runs in user session: console
	// mode, --as-user, macOS).
	monitor := clipboard.NewMonitor()

	// Set up text clipboard callback
	monitor.SetOnTextChange(func(text string) {
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
	monitor.SetOnImageChange(func(imageData []byte) {
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
	if err := monitor.Start(); err != nil {
		log.Printf("❌ Failed to start clipboard monitor: %v", err)
		return
	}

	m.clipboardMu.Lock()
	m.clipboardMonitor = monitor
	m.clipboardMu.Unlock()
}

func (m *Manager) stopClipboardMonitoring() {
	m.clipboardMu.Lock()
	monitor := m.clipboardMonitor
	helper := m.clipboardSessionHelper
	m.clipboardMonitor = nil
	m.clipboardSessionHelper = nil
	m.clipboardStarting = false
	m.clipboardMu.Unlock()

	if monitor != nil {
		monitor.Stop()
	}
	if helper != nil {
		helper.Stop()
	}
}
