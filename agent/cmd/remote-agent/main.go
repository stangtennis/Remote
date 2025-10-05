// +build windows

package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/stangtennis/remote-agent/internal/config"
	"github.com/stangtennis/remote-agent/internal/desktop"
	"github.com/stangtennis/remote-agent/internal/device"
	"github.com/stangtennis/remote-agent/internal/webrtc"
	"golang.org/x/sys/windows/svc"
)

var (
	cfg *config.Config
	dev *device.Device
	rtc *webrtc.Manager
)

func main() {
	// Check if running as Windows Service
	isWindowsService, err := svc.IsWindowsService()
	if err != nil {
		log.Fatalf("Failed to determine if running as service: %v", err)
	}

	if isWindowsService {
		// Run as Windows Service
		runService()
		return
	}

	// Run interactively
	runInteractive()
}

func runInteractive() {
	fmt.Println("🖥️  Remote Desktop Agent Starting...")
	fmt.Println("=====================================")
	
	// Check current desktop
	desktopName, _ := desktop.GetInputDesktop()
	fmt.Printf("🖥️  Current desktop: %s\n", desktopName)
	if desktop.IsOnLoginScreen() {
		fmt.Println("⚠️  Running on login screen - limited functionality")
	}

	if err := startAgent(); err != nil {
		log.Fatalf("Failed to start agent: %v", err)
	}

	// Start desktop monitoring
	go desktop.MonitorDesktopSwitch(func(dt desktop.DesktopType) {
		switch dt {
		case desktop.DesktopWinlogon:
			fmt.Println("🔒 Switched to login screen")
		case desktop.DesktopDefault:
			fmt.Println("🔓 Switched to user desktop")
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
