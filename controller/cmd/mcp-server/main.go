package main

import (
	"fmt"
	"log"
	"os"

	"github.com/mark3labs/mcp-go/server"
	"github.com/stangtennis/Remote/controller/internal/config"
)

func main() {
	// Send all logs to stderr (stdout is reserved for MCP JSON-RPC)
	log.SetOutput(os.Stderr)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	log.Println("Starting Remote Desktop MCP Server...")

	// Load config (Supabase URL + anon key)
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Get credentials from env
	email := os.Getenv("RD_EMAIL")
	password := os.Getenv("RD_PASSWORD")
	if email == "" || password == "" {
		log.Fatal("RD_EMAIL and RD_PASSWORD environment variables required")
	}

	// Authenticate with Supabase
	log.Printf("Authenticating as %s...", email)
	auth, err := supabaseSignIn(cfg.SupabaseURL, cfg.SupabaseAnonKey, email, password)
	if err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}
	log.Printf("Authenticated: user_id=%s", auth.userID)

	// Create connection manager
	connMgr := NewConnectionManager(cfg, auth)
	connMgr.StartIdleChecker()

	// Create MCP server
	s := server.NewMCPServer(
		"remote-desktop",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	// Register all tools
	registerTools(s, cfg, auth, connMgr)

	log.Println("MCP Server ready, serving on stdio...")

	// Serve over stdio
	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
