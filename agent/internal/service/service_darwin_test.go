//go:build darwin

package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
)

func TestGetPlistPath(t *testing.T) {
	path := getPlistPath()
	if !strings.Contains(path, "LaunchAgents") {
		t.Errorf("getPlistPath() = %q, want path containing \"LaunchAgents\"", path)
	}
	if !strings.HasSuffix(path, serviceName+".plist") {
		t.Errorf("getPlistPath() = %q, want suffix %q", path, serviceName+".plist")
	}
	t.Logf("Plist path: %s", path)
}

func TestGetLogDir(t *testing.T) {
	// Override HOME to use temp dir
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	logDir := getLogDir()
	if !strings.Contains(logDir, "RemoteDesktopAgent") {
		t.Errorf("getLogDir() = %q, want path containing \"RemoteDesktopAgent\"", logDir)
	}

	// Should have created the directory
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		t.Errorf("getLogDir() should create directory %q", logDir)
	}
}

func TestPlistTemplate(t *testing.T) {
	tmpl, err := template.New("plist").Parse(launchdPlistTemplate)
	if err != nil {
		t.Fatalf("Failed to parse plist template: %v", err)
	}

	data := struct {
		Label   string
		ExePath string
		LogDir  string
		WorkDir string
	}{
		Label:   serviceName,
		ExePath: "/usr/local/bin/remote-agent",
		LogDir:  "/tmp/logs",
		WorkDir: "/usr/local/bin",
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("Failed to execute template: %v", err)
	}

	result := buf.String()

	// Verify key elements
	if !strings.Contains(result, serviceName) {
		t.Error("plist should contain service name")
	}
	if !strings.Contains(result, "/usr/local/bin/remote-agent") {
		t.Error("plist should contain executable path")
	}
	if !strings.Contains(result, "--console") {
		t.Error("plist should contain --console argument")
	}
	if !strings.Contains(result, "<true/>") {
		t.Error("plist should contain RunAtLoad/KeepAlive true")
	}
}

func TestInstallService(t *testing.T) {
	// Use temp dir to avoid modifying real system
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	exePath := filepath.Join(tmpDir, "remote-agent")
	// Create fake executable
	os.WriteFile(exePath, []byte("#!/bin/sh\n"), 0755)

	err := InstallService(exePath)
	if err != nil {
		t.Fatalf("InstallService() error = %v", err)
	}

	// Check plist was created
	plistPath := filepath.Join(tmpDir, "Library", "LaunchAgents", serviceName+".plist")
	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		t.Errorf("InstallService() should create plist at %q", plistPath)
	}

	// Read and verify content
	data, err := os.ReadFile(plistPath)
	if err != nil {
		t.Fatalf("Could not read plist: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, exePath) {
		t.Error("plist should contain executable path")
	}
}

func TestRunService(t *testing.T) {
	// On macOS, RunService is a no-op
	err := RunService()
	if err != nil {
		t.Errorf("RunService() error = %v, want nil", err)
	}
}

func TestServiceName(t *testing.T) {
	if serviceName != "dk.hawkeye.remote-agent" {
		t.Errorf("serviceName = %q, want \"dk.hawkeye.remote-agent\"", serviceName)
	}
}
