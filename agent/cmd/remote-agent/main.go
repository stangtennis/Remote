package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/stangtennis/remote-agent/internal/config"
	"github.com/stangtennis/remote-agent/internal/device"
	"github.com/stangtennis/remote-agent/internal/webrtc"
)

func main() {
	fmt.Println("🖥️  Remote Desktop Agent Starting...")
	fmt.Println("=====================================")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize device
	dev, err := device.New(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize device: %v", err)
	}

	// Register device with Supabase
	fmt.Println("📱 Registering device...")
	if err := dev.Register(); err != nil {
		log.Fatalf("Failed to register device: %v", err)
	}

	fmt.Printf("✅ Device registered: %s\n", dev.ID)
	fmt.Printf("   Name: %s\n", dev.Name)
	fmt.Printf("   Platform: %s\n", dev.Platform)
	fmt.Printf("   Arch: %s\n", dev.Arch)

	// Start presence heartbeat
	go dev.StartPresence()

	// Initialize WebRTC manager
	rtc, err := webrtc.New(cfg, dev)
	if err != nil {
		log.Fatalf("Failed to initialize WebRTC: %v", err)
	}

	// Start listening for sessions
	fmt.Println("👂 Listening for incoming connections...")
	go rtc.ListenForSessions()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	fmt.Println("\n🛑 Shutting down...")

	// Cleanup
	dev.SetOffline()
	time.Sleep(500 * time.Millisecond)

	fmt.Println("👋 Goodbye!")
}
