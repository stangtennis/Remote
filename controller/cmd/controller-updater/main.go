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
	targetPath string
	sourcePath string
	backupPath string
	startAfter bool
	logFile    string
)

func init() {
	flag.StringVar(&targetPath, "target", "", "Path to the current controller.exe to replace")
	flag.StringVar(&sourcePath, "source", "", "Path to the new controller.exe")
	flag.StringVar(&backupPath, "backup", "", "Path for backup of old exe (default: target.old)")
	flag.BoolVar(&startAfter, "start", false, "Start the controller after update")
	flag.StringVar(&logFile, "log", "", "Log file path (default: updates/updater.log)")
}

func main() {
	flag.Parse()

	// Set up logging
	setupLogging()

	log.Println("========================================")
	log.Println("Controller Updater Started")
	log.Println("========================================")

	// Validate arguments
	if targetPath == "" || sourcePath == "" {
		log.Fatal("❌ Error: --target and --source are required")
	}

	if backupPath == "" {
		backupPath = targetPath + ".old"
	}

	log.Printf("Target: %s", targetPath)
	log.Printf("Source: %s", sourcePath)
	log.Printf("Backup: %s", backupPath)
	log.Printf("Start after: %v", startAfter)

	// Verify source exists
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		log.Fatalf("❌ Source file not found: %s", sourcePath)
	}

	// Wait for target to be unlocked (controller has exited)
	log.Println("⏳ Waiting for controller to exit...")
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

	// Copy new exe to target (use copy instead of rename for cross-volume support)
	log.Printf("📥 Installing new version: %s -> %s", sourcePath, targetPath)
	if err := copyFile(sourcePath, targetPath); err != nil {
		// Rollback: restore backup
		log.Printf("❌ Failed to install new version: %v", err)
		log.Println("🔄 Rolling back...")
		if rbErr := os.Rename(backupPath, targetPath); rbErr != nil {
			log.Fatalf("❌ CRITICAL: Rollback failed: %v", rbErr)
		}
		log.Println("✅ Rollback successful")
		os.Exit(1)
	}

	log.Println("✅ Update installed successfully!")

	// Clean up source file
	os.Remove(sourcePath)

	// Start controller if requested
	if startAfter {
		log.Printf("🚀 Starting controller: %s", targetPath)
		cmd := exec.Command(targetPath)
		if err := cmd.Start(); err != nil {
			log.Printf("⚠️ Warning: Failed to start controller: %v", err)
		} else {
			log.Println("✅ Controller started")
		}
	}

	log.Println("========================================")
	log.Println("Update complete!")
	log.Println("========================================")
}

func setupLogging() {
	// Determine log file path
	if logFile == "" {
		// Try to use the updates directory
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			home, _ := os.UserHomeDir()
			localAppData = filepath.Join(home, "AppData", "Local")
		}
		logDir := filepath.Join(localAppData, "RemoteDesktopController", "updates")
		os.MkdirAll(logDir, 0755)
		logFile = filepath.Join(logDir, "updater.log")
	}

	// Open log file
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		// Fall back to stderr only
		log.SetOutput(os.Stderr)
		return
	}

	// Log to both file and stderr
	log.SetOutput(io.MultiWriter(os.Stderr, f))
	log.SetFlags(log.Ldate | log.Ltime)
}

func waitForFileUnlock(path string) error {
	for i := 0; i < maxRetries; i++ {
		// Try to open file for writing (exclusive)
		f, err := os.OpenFile(path, os.O_RDWR, 0)
		if err == nil {
			f.Close()
			log.Println("✅ File is unlocked")
			return nil
		}

		// File might not exist yet (first install)
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

	// Sync to ensure write is complete
	if err := destFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync: %w", err)
	}

	// Copy file permissions
	sourceInfo, err := os.Stat(src)
	if err == nil {
		os.Chmod(dst, sourceInfo.Mode())
	}

	return nil
}
