package viewer

import (
	"log"

	"fyne.io/fyne/v2"
)

// Manager handles clipboard synchronization
type Manager struct {
	app          fyne.App
	localClip    string
	remoteClip   string
	autoSync     bool
	
	// Callbacks
	onClipboardChange func(text string)
}

// NewManager creates a new clipboard manager
func NewManager(app fyne.App) *Manager {
	return &Manager{
		app:      app,
		autoSync: false,
	}
}

// EnableAutoSync enables automatic clipboard synchronization
func (m *Manager) EnableAutoSync() {
	m.autoSync = true
	log.Println("Clipboard auto-sync enabled")
}

// DisableAutoSync disables automatic clipboard synchronization
func (m *Manager) DisableAutoSync() {
	m.autoSync = false
	log.Println("Clipboard auto-sync disabled")
}

// SyncToRemote sends local clipboard to remote
func (m *Manager) SyncToRemote() error {
	clip := m.app.Clipboard().Content()
	
	if clip == m.localClip {
		return nil // No change
	}
	
	m.localClip = clip
	log.Printf("Syncing clipboard to remote: %d bytes", len(clip))
	
	if m.onClipboardChange != nil {
		m.onClipboardChange(clip)
	}
	
	return nil
}

// SyncFromRemote receives clipboard from remote
func (m *Manager) SyncFromRemote(text string) error {
	if text == m.remoteClip {
		return nil // No change
	}
	
	m.remoteClip = text
	m.app.Clipboard().SetContent(text)
	log.Printf("Received clipboard from remote: %d bytes", len(text))
	
	return nil
}

// GetLocalClipboard returns local clipboard content
func (m *Manager) GetLocalClipboard() string {
	return m.app.Clipboard().Content()
}

// SetOnClipboardChange sets callback for clipboard changes
func (m *Manager) SetOnClipboardChange(callback func(text string)) {
	m.onClipboardChange = callback
}

// StartMonitoring starts monitoring local clipboard changes
func (m *Manager) StartMonitoring() {
	// TODO: Implement periodic clipboard monitoring
	// Fyne doesn't have clipboard change events, so we need to poll
	log.Println("Clipboard monitoring started")
}

// StopMonitoring stops monitoring clipboard changes
func (m *Manager) StopMonitoring() {
	log.Println("Clipboard monitoring stopped")
}
