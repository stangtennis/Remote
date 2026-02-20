//go:build darwin

package device

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetPlatform(t *testing.T) {
	got := GetPlatform()
	if got != "macOS" {
		t.Errorf("GetPlatform() = %q, want \"macOS\"", got)
	}
}

func TestGetDeviceName(t *testing.T) {
	name := GetDeviceName()
	if name == "" {
		t.Error("GetDeviceName() returned empty string")
	}
	// Should not be "Unknown Mac" on a real macOS system
	t.Logf("Device name: %s", name)
}

func TestGenerateDeviceID(t *testing.T) {
	id, err := generateDeviceID()
	if err != nil {
		t.Fatalf("generateDeviceID() error = %v", err)
	}
	if !strings.HasPrefix(id, "device_") {
		t.Errorf("generateDeviceID() = %q, want prefix \"device_\"", id)
	}
	if len(id) < 20 {
		t.Errorf("generateDeviceID() = %q, too short (len=%d)", id, len(id))
	}
	t.Logf("Generated device ID: %s", id)
}

func TestGetOrCreateDeviceID(t *testing.T) {
	// Use a temp dir to avoid polluting the real Application Support
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")

	// Temporarily override HOME so loadFromAppSupport/saveToAppSupport use temp dir
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create the Application Support dir structure
	appSupportDir := filepath.Join(tmpDir, "Library", "Application Support", configDir)
	os.MkdirAll(appSupportDir, 0755)

	// First call — should generate new ID
	id1, err := GetOrCreateDeviceID()
	if err != nil {
		t.Fatalf("GetOrCreateDeviceID() first call error = %v", err)
	}
	if id1 == "" {
		t.Fatal("GetOrCreateDeviceID() returned empty string")
	}

	// Second call — should load from file
	id2, err := GetOrCreateDeviceID()
	if err != nil {
		t.Fatalf("GetOrCreateDeviceID() second call error = %v", err)
	}
	if id2 != id1 {
		t.Errorf("GetOrCreateDeviceID() second call = %q, want %q (should be persistent)", id2, id1)
	}
}

func TestGetMacHardwareUUID(t *testing.T) {
	uuid, err := getMacHardwareUUID()
	if err != nil {
		// On CI this might fail if ioreg is restricted, so just log it
		t.Logf("getMacHardwareUUID() error (may be expected on CI): %v", err)
		return
	}
	if uuid == "" {
		t.Error("getMacHardwareUUID() returned empty string")
	}
	t.Logf("Hardware UUID: %s", uuid)
}
