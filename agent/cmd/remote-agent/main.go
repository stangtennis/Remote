// +build windows

package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/stangtennis/remote-agent/internal/config"
	"github.com/stangtennis/remote-agent/internal/desktop"
	"github.com/stangtennis/remote-agent/internal/device"
	"github.com/stangtennis/remote-agent/internal/webrtc"
	"golang.org/x/sys/windows/svc"
)

var (
	cfg     *config.Config
	dev     *device.Device
	rtc     *webrtc.Manager
	logFile *os.File
)

func setupLogging() error {
	// Get executable directory
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	exeDir := filepath.Dir(exePath)
	
	// Create log file in same directory as executable
	logPath := filepath.Join(exeDir, "agent.log")
	
	// Open log file (append mode)
	logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	
	// Write to both file and console (if interactive)
	isService, _ := svc.IsWindowsService()
	if isService {
		// Service: only write to file
		log.SetOutput(logFile)
	} else {
		// Interactive: write to both
		multiWriter := io.MultiWriter(os.Stdout, logFile)
		log.SetOutput(multiWriter)
	}
	
	// Add timestamp to log entries
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	
	log.Printf("========================================")
	log.Printf("🖥️  Remote Desktop Agent Starting...")
	log.Printf("📝 Log file: %s", logPath)
	log.Printf("========================================")
	
	return nil
}

func main() {
	// Setup logging first
	if err := setupLogging(); err != nil {
		fmt.Printf("Failed to setup logging: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if logFile != nil {
			logFile.Close()
		}
	}()

	// Check if running as Windows Service
	isWindowsService, err := svc.IsWindowsService()
	if err != nil {
		log.Fatalf("Failed to determine if running as service: %v", err)
	}

	if isWindowsService {
		log.Println("🔧 Running as Windows Service")
		// Run as Windows Service
		runService()
		return
	}

	log.Println("🔧 Running in interactive mode")
	// Run interactively
	runInteractive()
}

func runInteractive() {
	// Check current desktop (non-fatal if fails)
	desktopName, err := desktop.GetInputDesktop()
	if err != nil {
		log.Printf("⚠️  Cannot detect desktop: %v", err)
		log.Println("   (This is normal when running as a service)")
	} else {
		log.Printf("🖥️  Current desktop: %s", desktopName)
		if desktop.IsOnLoginScreen() {
			log.Println("⚠️  Running on login screen - limited functionality")
		}
	}

	if err := startAgent(); err != nil {
		log.Fatalf("Failed to start agent: %v", err)
	}

	// Start desktop monitoring (will handle errors internally)
	go desktop.MonitorDesktopSwitch(func(dt desktop.DesktopType) {
		switch dt {
		case desktop.DesktopWinlogon:
			log.Println("🔒 Switched to login screen")
		case desktop.DesktopDefault:
			log.Println("🔓 Switched to user desktop")
		}
	})

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	fmt.Println("\n🛑 Shutting down...")

	stopAgent()
	fmt.Println("👋 Goodbye!")
}

func runService() {
	// Windows Service mode
	log.Println("Starting as Windows Service...")
	
	serviceName := "RemoteDesktopAgent"
	
	err := svc.Run(serviceName, &windowsService{})
	if err != nil {
		log.Fatalf("Service failed: %v", err)
	}
}

type windowsService struct{}

func (s *windowsService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}

	if err := startAgent(); err != nil {
		log.Printf("Service start failed: %v", err)
		return true, 1
	}

	// Start desktop monitoring
	go desktop.MonitorDesktopSwitch(func(dt desktop.DesktopType) {
		log.Printf("Desktop switched to type: %d", dt)
	})

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	log.Println("Service running")

loop:
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				log.Println("Service stopping...")
				break loop
			default:
				log.Printf("Unexpected control request #%d", c)
			}
		}
	}

	changes <- svc.Status{State: svc.StopPending}
	stopAgent()
	return
}

func startAgent() error {
	var err error
	
	// Load configuration
	cfg, err = config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize device
	dev, err = device.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize device: %w", err)
	}

	// Register device with Supabase
	log.Println("📱 Registering device...")
	if err := dev.Register(); err != nil {
		return fmt.Errorf("failed to register device: %w", err)
	}

	log.Printf("✅ Device registered: %s", dev.ID)
	log.Printf("   Name: %s", dev.Name)
	log.Printf("   Platform: %s", dev.Platform)
	log.Printf("   Arch: %s", dev.Arch)

	// Start presence heartbeat
	go dev.StartPresence()

	// Initialize WebRTC manager
	rtc, err = webrtc.New(cfg, dev)
	if err != nil {
		return fmt.Errorf("failed to initialize WebRTC: %w", err)
	}

	// Start listening for sessions
	log.Println("👂 Listening for incoming connections...")
	go rtc.ListenForSessions()

	return nil
}

func stopAgent() {
	if dev != nil {
		dev.SetOffline()
	}
	time.Sleep(500 * time.Millisecond)
}
