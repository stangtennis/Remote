package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	maxRetries    = 100
	retryInterval = 100 * time.Millisecond
)

var (
	targetPath  string
	sourcePath  string
	backupPath  string
	restart     bool
	serviceName string
	logFile     string
)

func init() {
	flag.StringVar(&targetPath, "target", "", "Path to the current remote-agent.exe to replace")
	flag.StringVar(&sourcePath, "source", "", "Path to the new remote-agent.exe")
	flag.StringVar(&backupPath, "backup", "", "Path for backup of old exe (default: target.old)")
	flag.BoolVar(&restart, "restart", false, "Restart the agent after update")
	flag.StringVar(&serviceName, "service-name", "", "Windows service name (if running as service)")
	flag.StringVar(&logFile, "log", "", "Log file path")
}

func main() {
	flag.Parse()

	setupLogging()

	log.Println("========================================")
	log.Println("Agent Updater Started")
	log.Println("========================================")

	if targetPath == "" || sourcePath == "" {
		log.Fatal("❌ Error: --target and --source are required")
	}

	if backupPath == "" {
		backupPath = targetPath + ".old"
	}

	log.Printf("Target: %s", targetPath)
	log.Printf("Source: %s", sourcePath)
	log.Printf("Backup: %s", backupPath)
	log.Printf("Restart: %v", restart)
	log.Printf("Service: %s", serviceName)

	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		log.Fatalf("❌ Source file not found: %s", sourcePath)
	}

	// If running as service, stop it first
	if serviceName != "" {
		log.Printf("🛑 Stopping service: %s", serviceName)
		if err := stopService(serviceName); err != nil {
			log.Printf("⚠️ Warning: Failed to stop service: %v", err)
		} else {
			log.Println("✅ Service stopped")
		}
	}

	// Wait for target to be unlocked
	log.Println("⏳ Waiting for agent to exit...")
	if err := waitForFileUnlock(targetPath); err != nil {
		log.Fatalf("❌ Failed to wait for file unlock: %v", err)
	}

	// Remove old backup if exists
	if _, err := os.Stat(backupPath); err == nil {
		log.Printf("🗑️ Removing old backup: %s", backupPath)
		if err := os.Remove(backupPath); err != nil {
			log.Printf("⚠️ Warning: Failed to remove old backup: %v", err)
		}
	}

	// Rename current exe to backup
	log.Printf("📦 Creating backup: %s -> %s", targetPath, backupPath)
	if err := os.Rename(targetPath, backupPath); err != nil {
		log.Fatalf("❌ Failed to create backup: %v", err)
	}

	// Copy new exe to target
	log.Printf("📥 Installing new version: %s -> %s", sourcePath, targetPath)
	if err := copyFile(sourcePath, targetPath); err != nil {
		log.Printf("❌ Failed to install new version: %v", err)
		log.Println("🔄 Rolling back...")
		if rbErr := os.Rename(backupPath, targetPath); rbErr != nil {
			log.Fatalf("❌ CRITICAL: Rollback failed: %v", rbErr)
		}
		log.Println("✅ Rollback successful")
		
		// Restart service if it was running
		if serviceName != "" && restart {
			startService(serviceName)
		}
		os.Exit(1)
	}

	log.Println("✅ Update installed successfully!")

	// Clean up source file
	os.Remove(sourcePath)

	// Restart if requested
	if restart {
		if serviceName != "" {
			log.Printf("🚀 Starting service: %s", serviceName)
			if err := startService(serviceName); err != nil {
				log.Printf("⚠️ Warning: Failed to start service: %v", err)
			} else {
				log.Println("✅ Service started")
			}
		} else {
			log.Printf("🚀 Starting agent: %s", targetPath)
			cmd := exec.Command(targetPath)
			if err := cmd.Start(); err != nil {
				log.Printf("⚠️ Warning: Failed to start agent: %v", err)
			} else {
				log.Println("✅ Agent started")
			}
		}
	}

	log.Println("========================================")
	log.Println("Update complete!")
	log.Println("========================================")
}

func setupLogging() {
	if logFile == "" {
		programData := os.Getenv("PROGRAMDATA")
		if programData == "" {
			exePath, _ := os.Executable()
			programData = filepath.Dir(exePath)
		}
		logDir := filepath.Join(programData, "RemoteDesktopAgent", "updates")
		os.MkdirAll(logDir, 0755)
		logFile = filepath.Join(logDir, "updater.log")
	}

	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.SetOutput(os.Stderr)
		return
	}

	log.SetOutput(io.MultiWriter(os.Stderr, f))
	log.SetFlags(log.Ldate | log.Ltime)
}

func waitForFileUnlock(path string) error {
	for i := 0; i < maxRetries; i++ {
		f, err := os.OpenFile(path, os.O_RDWR, 0)
		if err == nil {
			f.Close()
			log.Println("✅ File is unlocked")
			return nil
		}

		if os.IsNotExist(err) {
			log.Println("✅ Target file doesn't exist (new install)")
			return nil
		}

		if i%10 == 0 {
			log.Printf("⏳ Waiting for file to be unlocked... (attempt %d/%d)", i+1, maxRetries)
		}
		time.Sleep(retryInterval)
	}

	return fmt.Errorf("timeout waiting for file to be unlocked after %d attempts", maxRetries)
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source: %w", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy: %w", err)
	}

	if err := destFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync: %w", err)
	}

	sourceInfo, err := os.Stat(src)
	if err == nil {
		os.Chmod(dst, sourceInfo.Mode())
	}

	return nil
}

func stopService(name string) error {
	cmd := exec.Command("sc", "stop", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sc stop failed: %v - %s", err, string(output))
	}

	// Wait for service to stop (max 30 seconds)
	for i := 0; i < 30; i++ {
		if !isServiceRunning(name) {
			return nil
		}
		time.Sleep(time.Second)
	}

	return fmt.Errorf("timeout waiting for service to stop")
}

func startService(name string) error {
	cmd := exec.Command("sc", "start", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sc start failed: %v - %s", err, string(output))
	}
	return nil
}

func isServiceRunning(name string) bool {
	cmd := exec.Command("sc", "query", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	// Check if output contains "RUNNING"
	return containsString(string(output), "RUNNING")
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
