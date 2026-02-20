//go:build darwin

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/stangtennis/remote-agent/internal/auth"
	"github.com/stangtennis/remote-agent/internal/config"
	"github.com/stangtennis/remote-agent/internal/device"
	"github.com/stangtennis/remote-agent/internal/tray"
	"github.com/stangtennis/remote-agent/internal/webrtc"
	"github.com/stangtennis/remote-agent/pkg/logging"
)

var (
	cfg         *config.Config
	dev         *device.Device
	rtc         *webrtc.Manager
	currentUser *auth.Credentials
)

func setupLogging() error {
	loggingCfg := logging.DefaultConfig()
	loggingCfg.Console = true
	loggingCfg.Level = "info"

	if err := logging.Init(loggingCfg); err != nil {
		return fmt.Errorf("failed to initialize logging: %w", err)
	}

	logging.Logger.Info().
		Str("version", tray.VersionString).
		Str("platform", "darwin").
		Str("log_file", logging.GetLogFilePath()).
		Msg("Remote Desktop Agent starting")

	logging.Sync()

	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			logging.Sync()
		}
	}()

	return nil
}

func main() {
	consoleFlag := flag.Bool("console", false, "Run in console mode (full logging)")
	logoutFlag := flag.Bool("logout", false, "Log out and clear saved credentials")
	helpFlag := flag.Bool("help", false, "Show help")
	flag.Parse()

	if *helpFlag {
		printUsage()
		return
	}

	if *logoutFlag {
		if err := auth.ClearCredentials(); err != nil {
			fmt.Printf("Could not clear credentials: %v\n", err)
		} else {
			fmt.Println("Logged out successfully.")
		}
		return
	}

	if err := setupLogging(); err != nil {
		fmt.Printf("Could not setup logging: %v\n", err)
		os.Exit(1)
	}
	defer logging.Sync()

	if *consoleFlag {
		runConsoleMode()
		return
	}

	// Default: interactive mode
	runInteractive()
}

func printUsage() {
	fmt.Println("Remote Desktop Agent (macOS) - v" + tray.VersionString)
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  remote-agent              Run interactively (with system tray)")
	fmt.Println("  remote-agent --console    Run in console mode (full logging)")
	fmt.Println("  remote-agent --logout     Clear saved credentials")
	fmt.Println("  remote-agent --help       Show this help")
}

func runConsoleMode() {
	log.Println("========================================")
	log.Println("CONSOLE MODE - Full Logging")
	log.Println("========================================")
	log.Println("Press Ctrl+C to stop")
	log.Println("")

	if !auth.IsLoggedIn() {
		log.Println("Not logged in. Starting login...")
		doLogin()
		if !auth.IsLoggedIn() {
			log.Println("Login failed or cancelled")
			return
		}
	}

	creds, err := auth.GetCurrentUser()
	if err != nil {
		log.Printf("Could not load credentials: %v", err)
		return
	}
	log.Printf("Logged in as: %s", creds.Email)
	currentUser = creds

	if err := startAgent(); err != nil {
		log.Fatalf("Could not start agent: %v", err)
	}

	log.Println("")
	log.Println("========================================")
	log.Println("Agent running! Waiting for connections...")
	log.Println("Press Ctrl+C to stop")
	log.Println("========================================")

	// Wait for signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	stopAgent()
}

func runInteractive() {
	if !auth.IsLoggedIn() {
		log.Println("Login required...")
		doLogin()
		if !auth.IsLoggedIn() {
			log.Println("Login failed or cancelled")
			return
		}
	}

	creds, err := auth.GetCurrentUser()
	if err != nil {
		log.Println("Could not load credentials, please log in again")
		auth.ClearCredentials()
		runInteractive()
		return
	}
	log.Printf("Logged in as: %s", creds.Email)
	currentUser = creds

	if err := startAgent(); err != nil {
		log.Fatalf("Could not start agent: %v", err)
	}

	log.Println("Starting system tray...")

	trayApp := tray.New(dev, func() {
		log.Println("Shutting down from system tray...")
		stopAgent()
		log.Println("Goodbye!")
	})

	trayApp.Run()
}

func doLogin() {
	tempCfg, err := config.Load()
	if err != nil {
		log.Fatalf("Could not load config: %v", err)
	}

	authConfig := auth.AuthConfig{
		SupabaseURL: tempCfg.SupabaseURL,
		AnonKey:     tempCfg.SupabaseAnonKey,
	}

	result := auth.ShowLoginDialog(authConfig)
	if result == nil || !result.Success {
		log.Println("Login cancelled or failed")
		return
	}

	log.Printf("Logged in as: %s", result.Email)
}

func startAgent() error {
	var err error

	cfg, err = config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	log.Printf("Credentials path: %s", auth.GetCredentialsPath())
	deviceID, _ := device.GetOrCreateDeviceID()
	log.Printf("Device ID: %s", deviceID)

	authConfig := auth.AuthConfig{
		SupabaseURL: cfg.SupabaseURL,
		AnonKey:     cfg.SupabaseAnonKey,
	}

	if !auth.IsLoggedIn() {
		log.Println("No valid credentials found")
		creds, err := auth.LoadCredentials()
		if err == nil && creds.RefreshToken != "" {
			log.Println("Attempting to refresh token...")
			result, err := auth.RefreshToken(authConfig, creds.RefreshToken)
			if err == nil && result.Success {
				log.Printf("Token refreshed for: %s", result.Email)
				currentUser, _ = auth.GetCurrentUser()
			} else {
				return fmt.Errorf("token refresh failed — run agent interactively to login")
			}
		} else {
			return fmt.Errorf("no credentials found — run agent interactively to login first")
		}
	} else {
		currentUser, _ = auth.GetCurrentUser()
		log.Printf("Using saved credentials for: %s", currentUser.Email)
	}

	creds, err := auth.LoadCredentials()
	if err != nil {
		return fmt.Errorf("failed to load credentials: %w", err)
	}
	tokenProvider := auth.NewTokenProvider(authConfig, creds)
	log.Println("TokenProvider created")

	dev, err = device.New(cfg, tokenProvider)
	if err != nil {
		return fmt.Errorf("failed to initialize device: %w", err)
	}

	log.Println("Registering device...")
	if err := dev.Register(); err != nil {
		return fmt.Errorf("failed to register device: %w", err)
	}

	log.Printf("Device registered: %s", dev.ID)
	log.Printf("   Name: %s", dev.Name)
	log.Printf("   Platform: %s", dev.Platform)
	log.Printf("   Arch: %s", dev.Arch)

	go dev.StartPresence()

	rtc, err = webrtc.New(cfg, dev, tokenProvider)
	if err != nil {
		return fmt.Errorf("failed to initialize WebRTC: %w", err)
	}

	log.Println("Listening for incoming connections...")
	go rtc.ListenForSessions()

	return nil
}

func stopAgent() {
	if dev != nil {
		dev.SetOffline()
	}
	time.Sleep(500 * time.Millisecond)
}
