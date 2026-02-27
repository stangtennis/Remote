package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/stangtennis/Remote/controller/internal/config"
)

const daemonIdleTimeout = 10 * time.Minute

// daemonRequest is the JSON protocol for CLI → daemon communication
type daemonRequest struct {
	Cmd  string                 `json:"cmd"`
	Args map[string]interface{} `json:"args,omitempty"`
}

// daemonResponse is the JSON protocol for daemon → CLI communication
type daemonResponse struct {
	OK    bool                   `json:"ok"`
	Data  map[string]interface{} `json:"data,omitempty"`
	Error string                 `json:"error,omitempty"`
}

func getDaemonDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".remote-desktop")
}

func getSocketPath() string {
	return filepath.Join(getDaemonDir(), "daemon.sock")
}

func getPIDPath() string {
	return filepath.Join(getDaemonDir(), "daemon.pid")
}

// startDaemon launches the daemon process in the background and waits for it to be ready
func startDaemon(cfg *config.Config, auth *authInfo, deviceID, deviceName string) (int, error) {
	// Kill any existing daemon
	stopExistingDaemon()

	daemonDir := getDaemonDir()
	os.MkdirAll(daemonDir, 0700)

	// Remove stale socket
	os.Remove(getSocketPath())

	// Start daemon as a subprocess
	exe, _ := os.Executable()
	cmd := exec.Command(exe, "__daemon__", deviceID, deviceName)
	cmd.Env = append(os.Environ(),
		"RD_EMAIL="+auth.email,
		"RD_PASSWORD="+auth.password,
	)

	// Redirect daemon logs to file
	logFile, err := os.OpenFile(filepath.Join(daemonDir, "daemon.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return 0, fmt.Errorf("failed to create log file: %w", err)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Detach from parent process
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return 0, fmt.Errorf("failed to start daemon: %w", err)
	}

	pid := cmd.Process.Pid

	// Write PID file
	os.WriteFile(getPIDPath(), []byte(strconv.Itoa(pid)), 0600)

	// Don't wait for the child (it's detached)
	cmd.Process.Release()
	logFile.Close()

	// Wait for daemon to become ready (socket exists and responds)
	for i := 0; i < 600; i++ { // Up to 60 seconds
		time.Sleep(100 * time.Millisecond)
		if resp, err := sendDaemonRequest(daemonRequest{Cmd: "status"}); err == nil {
			if resp.OK {
				return pid, nil
			}
			if !resp.OK && resp.Error != "" {
				return 0, fmt.Errorf("daemon failed: %s", resp.Error)
			}
		}
	}

	return 0, fmt.Errorf("daemon did not become ready within 60 seconds (check %s/daemon.log)", daemonDir)
}

// stopExistingDaemon kills any existing daemon process
func stopExistingDaemon() {
	pidData, err := os.ReadFile(getPIDPath())
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err != nil {
		return
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	proc.Signal(syscall.SIGTERM)
	// Wait briefly for cleanup
	time.Sleep(200 * time.Millisecond)
	proc.Signal(syscall.SIGKILL)
	os.Remove(getSocketPath())
	os.Remove(getPIDPath())
}

// runDaemon is the main daemon process entry point (called when argv[1] == "__daemon__")
func runDaemon(deviceID, deviceName string) {
	daemonDir := getDaemonDir()
	os.MkdirAll(daemonDir, 0700)

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("[daemon] Starting for device %s (%s)", deviceName, deviceID)

	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[daemon] Failed to load config: %v", err)
	}

	// Authenticate
	email := os.Getenv("RD_EMAIL")
	password := os.Getenv("RD_PASSWORD")
	if email == "" || password == "" {
		log.Fatal("[daemon] RD_EMAIL and RD_PASSWORD required")
	}

	auth, err := supabaseSignIn(cfg.SupabaseURL, cfg.SupabaseAnonKey, email, password)
	if err != nil {
		log.Fatalf("[daemon] Auth failed: %v", err)
	}
	log.Printf("[daemon] Authenticated as %s", auth.userID)

	// Create connection manager and connect
	connMgr := NewConnectionManager(cfg, auth)
	connMgr.StartIdleChecker()

	if err := connMgr.Connect(deviceID, deviceName); err != nil {
		log.Fatalf("[daemon] Failed to connect: %v", err)
	}
	log.Printf("[daemon] Connected to %s", deviceName)

	// Wait for first frame
	time.Sleep(500 * time.Millisecond)

	// Start Unix socket listener
	socketPath := getSocketPath()
	os.Remove(socketPath) // Remove stale socket

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatalf("[daemon] Failed to listen on socket: %v", err)
	}
	defer listener.Close()
	os.Chmod(socketPath, 0600)

	log.Printf("[daemon] Listening on %s", socketPath)

	startTime := time.Now()
	lastActivity := time.Now()
	var activityMu sync.Mutex

	// Idle timeout checker
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			activityMu.Lock()
			idle := time.Since(lastActivity)
			activityMu.Unlock()
			if idle > daemonIdleTimeout {
				log.Printf("[daemon] Idle timeout (%s), shutting down", idle.Round(time.Second))
				connMgr.Disconnect(deviceID)
				listener.Close()
				os.Remove(socketPath)
				os.Remove(getPIDPath())
				os.Exit(0)
			}
		}
	}()

	// Accept connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("[daemon] Accept error: %v", err)
			break
		}

		activityMu.Lock()
		lastActivity = time.Now()
		activityMu.Unlock()

		go handleDaemonConnection(conn, connMgr, deviceID, deviceName, startTime)
	}

	// Cleanup
	connMgr.Disconnect(deviceID)
	os.Remove(socketPath)
	os.Remove(getPIDPath())
}

func handleDaemonConnection(conn net.Conn, connMgr *ConnectionManager, deviceID, deviceName string, startTime time.Time) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	var req daemonRequest
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		sendResponse(conn, daemonResponse{OK: false, Error: "invalid request"})
		return
	}

	resp := handleCommand(req, connMgr, deviceID, deviceName, startTime)
	sendResponse(conn, resp)

	// If disconnect was requested, shutdown daemon
	if req.Cmd == "disconnect" && resp.OK {
		go func() {
			time.Sleep(100 * time.Millisecond)
			os.Remove(getSocketPath())
			os.Remove(getPIDPath())
			os.Exit(0)
		}()
	}
}

func sendResponse(conn net.Conn, resp daemonResponse) {
	json.NewEncoder(conn).Encode(resp)
}

func handleCommand(req daemonRequest, connMgr *ConnectionManager, deviceID, deviceName string, startTime time.Time) daemonResponse {
	switch req.Cmd {
	case "status":
		return handleStatus(connMgr, deviceID, deviceName, startTime)
	case "screenshot":
		return handleScreenshot(req, connMgr, deviceID)
	case "click":
		return handleClick(req, connMgr, deviceID)
	case "type":
		return handleType(req, connMgr, deviceID)
	case "key":
		return handleKey(req, connMgr, deviceID)
	case "scroll":
		return handleScroll(req, connMgr, deviceID)
	case "disconnect":
		return handleDisconnect(connMgr, deviceID)
	default:
		return daemonResponse{OK: false, Error: fmt.Sprintf("unknown command: %s", req.Cmd)}
	}
}

func init() {
	// Check if we're being invoked as the daemon subprocess
	if len(os.Args) >= 4 && os.Args[1] == "__daemon__" {
		runDaemon(os.Args[2], os.Args[3])
		os.Exit(0)
	}
}
