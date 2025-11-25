//go:build windows
// +build windows

package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/stangtennis/remote-agent/internal/config"
	"github.com/stangtennis/remote-agent/internal/desktop"
	"github.com/stangtennis/remote-agent/internal/device"
	"github.com/stangtennis/remote-agent/internal/tray"
	"github.com/stangtennis/remote-agent/internal/webrtc"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
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

	// Open log file (truncate mode - clears previous logs)
	logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
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
	log.Printf("📦 Version: %s", tray.VersionString)
	log.Printf("📝 Log file: %s", logPath)
	log.Printf("========================================")

	return nil
}

const serviceName = "RemoteDesktopAgent"
const serviceDisplayName = "Remote Desktop Agent"
const serviceDescription = "Provides remote desktop access with login screen support"

func main() {
	// Parse command-line flags
	installFlag := flag.Bool("install", false, "Install as Windows Service")
	uninstallFlag := flag.Bool("uninstall", false, "Uninstall Windows Service")
	startFlag := flag.Bool("start", false, "Start the Windows Service")
	stopFlag := flag.Bool("stop", false, "Stop the Windows Service")
	statusFlag := flag.Bool("status", false, "Show service status")
	helpFlag := flag.Bool("help", false, "Show help")
	flag.Parse()

	// Handle service management commands (no logging needed)
	if *helpFlag {
		printUsage()
		return
	}
	if *installFlag {
		if err := installService(); err != nil {
			fmt.Printf("❌ Failed to install service: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if *uninstallFlag {
		if err := uninstallService(); err != nil {
			fmt.Printf("❌ Failed to uninstall service: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if *startFlag {
		if err := startService(); err != nil {
			fmt.Printf("❌ Failed to start service: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if *stopFlag {
		if err := stopService(); err != nil {
			fmt.Printf("❌ Failed to stop service: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if *statusFlag {
		showServiceStatus()
		return
	}

	// Setup logging for normal operation
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

func printUsage() {
	fmt.Println("Remote Desktop Agent - v" + tray.VersionString)
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  remote-agent.exe              Run interactively (with system tray)")
	fmt.Println("  remote-agent.exe -install     Install as Windows Service (requires Admin)")
	fmt.Println("  remote-agent.exe -uninstall   Uninstall Windows Service (requires Admin)")
	fmt.Println("  remote-agent.exe -start       Start the Windows Service")
	fmt.Println("  remote-agent.exe -stop        Stop the Windows Service")
	fmt.Println("  remote-agent.exe -status      Show service status")
	fmt.Println("  remote-agent.exe -help        Show this help")
	fmt.Println()
	fmt.Println("Service Mode:")
	fmt.Println("  When installed as a service, the agent runs at system startup")
	fmt.Println("  and can capture the login screen (Session 0 support).")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Install and start service (run as Administrator):")
	fmt.Println("  remote-agent.exe -install")
	fmt.Println("  remote-agent.exe -start")
	fmt.Println()
	fmt.Println("  # Check status:")
	fmt.Println("  remote-agent.exe -status")
}

// installService installs the agent as a Windows Service
func installService() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to service manager (are you running as Administrator?): %w", err)
	}
	defer m.Disconnect()

	// Check if service already exists
	s, err := m.OpenService(serviceName)
	if err == nil {
		s.Close()
		return fmt.Errorf("service %s already exists - use -uninstall first", serviceName)
	}

	// Create the service
	s, err = m.CreateService(serviceName, exePath, mgr.Config{
		DisplayName: serviceDisplayName,
		Description: serviceDescription,
		StartType:   mgr.StartAutomatic,
	}, "")
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}
	defer s.Close()

	// Configure recovery actions (restart on failure)
	err = s.SetRecoveryActions([]mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 10 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 30 * time.Second},
	}, 86400) // Reset failure count after 24 hours
	if err != nil {
		fmt.Printf("⚠️  Warning: Could not set recovery actions: %v\n", err)
	}

	fmt.Println("✅ Service installed successfully!")
	fmt.Println()
	fmt.Printf("   Service Name: %s\n", serviceName)
	fmt.Printf("   Display Name: %s\n", serviceDisplayName)
	fmt.Printf("   Executable:   %s\n", exePath)
	fmt.Printf("   Start Type:   Automatic\n")
	fmt.Println()
	fmt.Println("To start the service, run:")
	fmt.Println("   remote-agent.exe -start")
	fmt.Println()
	fmt.Println("Or use Windows Services (services.msc)")
	return nil
}

// uninstallService removes the Windows Service
func uninstallService() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to service manager (are you running as Administrator?): %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("service %s not found: %w", serviceName, err)
	}
	defer s.Close()

	// Stop the service first if running
	status, err := s.Query()
	if err == nil && status.State != svc.Stopped {
		fmt.Println("Stopping service...")
		_, err = s.Control(svc.Stop)
		if err != nil {
			fmt.Printf("⚠️  Warning: Could not stop service: %v\n", err)
		}
		// Wait for service to stop
		time.Sleep(2 * time.Second)
	}

	// Delete the service
	err = s.Delete()
	if err != nil {
		return fmt.Errorf("failed to delete service: %w", err)
	}

	fmt.Println("✅ Service uninstalled successfully!")
	return nil
}

// startService starts the Windows Service
func startService() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("service %s not found (install it first with -install): %w", serviceName, err)
	}
	defer s.Close()

	err = s.Start()
	if err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	fmt.Println("✅ Service started successfully!")
	fmt.Println()
	fmt.Println("Check status with: remote-agent.exe -status")
	fmt.Println("View logs in: agent.log")
	return nil
}

// stopService stops the Windows Service
func stopService() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("service %s not found: %w", serviceName, err)
	}
	defer s.Close()

	status, err := s.Query()
	if err != nil {
		return fmt.Errorf("failed to query service: %w", err)
	}

	if status.State == svc.Stopped {
		fmt.Println("Service is already stopped.")
		return nil
	}

	_, err = s.Control(svc.Stop)
	if err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}

	fmt.Println("✅ Service stopped successfully!")
	return nil
}

// showServiceStatus displays the current service status
func showServiceStatus() {
	m, err := mgr.Connect()
	if err != nil {
		// Try using sc query as fallback (works without admin for status)
		fmt.Printf("Service: %s\n", serviceName)
		fmt.Println("(Run as Administrator for detailed status)")
		return
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		fmt.Printf("Service: %s\n", serviceName)
		fmt.Println("Status:  NOT INSTALLED")
		fmt.Println()
		fmt.Println("Install with: remote-agent.exe -install")
		return
	}
	defer s.Close()

	status, err := s.Query()
	if err != nil {
		fmt.Printf("❌ Cannot query service: %v\n", err)
		return
	}

	statusStr := "UNKNOWN"
	switch status.State {
	case svc.Stopped:
		statusStr = "STOPPED"
	case svc.StartPending:
		statusStr = "STARTING..."
	case svc.StopPending:
		statusStr = "STOPPING..."
	case svc.Running:
		statusStr = "RUNNING ✅"
	case svc.ContinuePending:
		statusStr = "CONTINUING..."
	case svc.PausePending:
		statusStr = "PAUSING..."
	case svc.Paused:
		statusStr = "PAUSED"
	}

	fmt.Printf("Service: %s\n", serviceName)
	fmt.Printf("Status:  %s\n", statusStr)
	fmt.Printf("PID:     %d\n", status.ProcessId)
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

	// Desktop monitoring is now handled by WebRTC manager
	// It will automatically reinitialize screen capture on desktop switch

	log.Println("🔔 Starting system tray...")

	// Run system tray (blocks until user exits from tray menu)
	trayApp := tray.New(dev, func() {
		log.Println("🛑 Shutting down from system tray...")
		stopAgent()
		log.Println("👋 Goodbye!")
	})

	trayApp.Run()
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

	// Desktop monitoring is now handled by WebRTC manager
	// It will automatically detect Session 0 and reinitialize screen capture on desktop switch
	if _, err := desktop.GetInputDesktop(); err == nil {
		log.Println("✅ Desktop access available")
	} else {
		log.Println("⚠️  No desktop access (Session 0 / pre-login)")
		log.Println("   Service will run - WebRTC manager handles desktop detection")
	}

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	log.Println("Service running")

loop:
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
				log.Println("Service interrogated - responding")
			case svc.Stop:
				log.Println("Service received STOP command")
				break loop
			case svc.Shutdown:
				log.Println("Service received SHUTDOWN command")
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
