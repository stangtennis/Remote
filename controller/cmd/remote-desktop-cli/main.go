package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "list":
		cmdList()
	case "connect":
		cmdConnect()
	case "disconnect":
		cmdDisconnect()
	case "screenshot":
		cmdScreenshot()
	case "click":
		cmdClick()
	case "type":
		cmdType()
	case "key":
		cmdKey()
	case "scroll":
		cmdScroll()
	case "status":
		cmdStatus()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage: remote-desktop-cli <command> [args]

Commands:
  list                              List available devices
  connect <device_id>               Connect to a device (starts daemon)
  disconnect                        Disconnect and stop daemon
  screenshot [-o file.jpg]          Take screenshot and save to file
  click <x> <y> [--right|--double]  Click at coordinates
  type "text"                       Type text
  key <key> [--ctrl] [--shift] [--alt]  Press a key
  scroll <delta> [--at x,y]        Scroll (positive=down, negative=up)
  status                            Show connection status

Environment:
  RD_EMAIL      Supabase email (required for list/connect)
  RD_PASSWORD   Supabase password (required for list/connect)`)
}

// sendDaemonRequest sends a JSON request to the daemon and returns the response
func sendDaemonRequest(req daemonRequest) (*daemonResponse, error) {
	socketPath := getSocketPath()
	conn, err := net.DialTimeout("unix", socketPath, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("daemon not running (use 'connect' first): %v", err)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(30 * time.Second))

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(req); err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}

	var resp daemonResponse
	decoder := json.NewDecoder(conn)
	if err := decoder.Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	return &resp, nil
}

func cmdList() {
	auth, cfg, err := getAuthAndConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	devices, err := fetchDevices(cfg.SupabaseURL, cfg.SupabaseAnonKey, auth)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(devices) == 0 {
		fmt.Println("No devices registered.")
		return
	}

	for _, d := range devices {
		status := d.statusString()
		fmt.Printf("%s (%s) %s  ID: %s\n", d.DeviceName, d.Platform, status, d.DeviceID)
	}
}

func cmdConnect() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: remote-desktop-cli connect <device_id>")
		os.Exit(1)
	}
	deviceID := os.Args[2]

	// Check if daemon is already running and connected
	if resp, err := sendDaemonRequest(daemonRequest{Cmd: "status"}); err == nil && resp.OK {
		if connID, ok := resp.Data["device_id"].(string); ok && connID == deviceID {
			fmt.Printf("Already connected to %s\n", deviceID)
			return
		}
		// Connected to different device — disconnect first
		sendDaemonRequest(daemonRequest{Cmd: "disconnect"})
		time.Sleep(500 * time.Millisecond)
	}

	// Start daemon
	auth, cfg, err := getAuthAndConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Look up device name
	devices, err := fetchDevices(cfg.SupabaseURL, cfg.SupabaseAnonKey, auth)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching devices: %v\n", err)
		os.Exit(1)
	}

	deviceName := deviceID
	for _, d := range devices {
		if d.DeviceID == deviceID {
			deviceName = d.DeviceName
			if !d.isOnline() {
				fmt.Fprintf(os.Stderr, "Error: device '%s' is offline (last seen: %s)\n", d.DeviceName, d.LastSeen.Format(time.RFC3339))
				os.Exit(1)
			}
			break
		}
	}

	pid, err := startDaemon(cfg, auth, deviceID, deviceName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Connected to %s. Daemon running (PID %d).\n", deviceName, pid)
}

func cmdDisconnect() {
	resp, err := sendDaemonRequest(daemonRequest{Cmd: "disconnect"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if !resp.OK {
		fmt.Fprintf(os.Stderr, "Error: %s\n", resp.Error)
		os.Exit(1)
	}
	fmt.Println("Disconnected. Daemon stopped.")
}

func cmdScreenshot() {
	output := "/tmp/rd-screenshot.jpg"
	maxWidth := 1280
	quality := 60

	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-o", "--output":
			if i+1 < len(os.Args) {
				output = os.Args[i+1]
				i++
			}
		case "-w", "--width":
			if i+1 < len(os.Args) {
				if v, err := strconv.Atoi(os.Args[i+1]); err == nil {
					maxWidth = v
				}
				i++
			}
		case "-q", "--quality":
			if i+1 < len(os.Args) {
				if v, err := strconv.Atoi(os.Args[i+1]); err == nil {
					quality = v
				}
				i++
			}
		}
	}

	resp, err := sendDaemonRequest(daemonRequest{
		Cmd: "screenshot",
		Args: map[string]interface{}{
			"max_width": maxWidth,
			"quality":   quality,
			"file":      output,
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if !resp.OK {
		fmt.Fprintf(os.Stderr, "Error: %s\n", resp.Error)
		os.Exit(1)
	}

	w := int(resp.Data["width"].(float64))
	h := int(resp.Data["height"].(float64))
	file := resp.Data["file"].(string)
	fmt.Printf("Screenshot saved: %s (%dx%d)\n", file, w, h)
}

func cmdClick() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "Usage: remote-desktop-cli click <x> <y> [--right|--double]")
		os.Exit(1)
	}

	x, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid x coordinate: %s\n", os.Args[2])
		os.Exit(1)
	}
	y, err := strconv.Atoi(os.Args[3])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid y coordinate: %s\n", os.Args[3])
		os.Exit(1)
	}

	button := "left"
	doubleClick := false
	for i := 4; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--right":
			button = "right"
		case "--middle":
			button = "middle"
		case "--double":
			doubleClick = true
		}
	}

	resp, err := sendDaemonRequest(daemonRequest{
		Cmd: "click",
		Args: map[string]interface{}{
			"x":            x,
			"y":            y,
			"button":       button,
			"double_click": doubleClick,
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if !resp.OK {
		fmt.Fprintf(os.Stderr, "Error: %s\n", resp.Error)
		os.Exit(1)
	}

	action := "Clicked"
	if doubleClick {
		action = "Double-clicked"
	}
	fmt.Printf("%s at (%d, %d) with %s button\n", action, x, y, button)
}

func cmdType() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: remote-desktop-cli type \"text\"")
		os.Exit(1)
	}

	text := strings.Join(os.Args[2:], " ")

	resp, err := sendDaemonRequest(daemonRequest{
		Cmd: "type",
		Args: map[string]interface{}{
			"text": text,
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if !resp.OK {
		fmt.Fprintf(os.Stderr, "Error: %s\n", resp.Error)
		os.Exit(1)
	}

	fmt.Printf("Typed %d characters\n", len(text))
}

func cmdKey() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: remote-desktop-cli key <key> [--ctrl] [--shift] [--alt]")
		os.Exit(1)
	}

	key := os.Args[2]
	ctrl := false
	shift := false
	alt := false

	for i := 3; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--ctrl":
			ctrl = true
		case "--shift":
			shift = true
		case "--alt":
			alt = true
		}
	}

	resp, err := sendDaemonRequest(daemonRequest{
		Cmd: "key",
		Args: map[string]interface{}{
			"key":   key,
			"ctrl":  ctrl,
			"shift": shift,
			"alt":   alt,
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if !resp.OK {
		fmt.Fprintf(os.Stderr, "Error: %s\n", resp.Error)
		os.Exit(1)
	}

	modifiers := ""
	if ctrl {
		modifiers += "Ctrl+"
	}
	if shift {
		modifiers += "Shift+"
	}
	if alt {
		modifiers += "Alt+"
	}
	fmt.Printf("Pressed %s%s\n", modifiers, key)
}

func cmdScroll() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: remote-desktop-cli scroll <delta> [--at x,y]")
		os.Exit(1)
	}

	delta, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid delta: %s\n", os.Args[2])
		os.Exit(1)
	}

	x := -1
	y := -1
	for i := 3; i < len(os.Args); i++ {
		if os.Args[i] == "--at" && i+1 < len(os.Args) {
			parts := strings.Split(os.Args[i+1], ",")
			if len(parts) == 2 {
				x, _ = strconv.Atoi(strings.TrimSpace(parts[0]))
				y, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
			}
			i++
		}
	}

	resp, err := sendDaemonRequest(daemonRequest{
		Cmd: "scroll",
		Args: map[string]interface{}{
			"delta": delta,
			"x":     x,
			"y":     y,
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if !resp.OK {
		fmt.Fprintf(os.Stderr, "Error: %s\n", resp.Error)
		os.Exit(1)
	}

	direction := "down"
	if delta < 0 {
		direction = "up"
	}
	if delta < 0 {
		delta = -delta
	}
	fmt.Printf("Scrolled %s by %d\n", direction, delta)
}

func cmdStatus() {
	resp, err := sendDaemonRequest(daemonRequest{Cmd: "status"})
	if err != nil {
		fmt.Println("Not connected (daemon not running)")
		return
	}
	if !resp.OK {
		fmt.Fprintf(os.Stderr, "Error: %s\n", resp.Error)
		os.Exit(1)
	}

	deviceID, _ := resp.Data["device_id"].(string)
	deviceName, _ := resp.Data["device_name"].(string)
	connected, _ := resp.Data["connected"].(bool)
	uptime, _ := resp.Data["uptime"].(string)
	frameAge, _ := resp.Data["frame_age"].(string)
	pid, _ := resp.Data["pid"].(float64)

	if connected {
		fmt.Printf("Connected to: %s (%s)\n", deviceName, deviceID)
		fmt.Printf("Daemon PID:   %.0f\n", pid)
		fmt.Printf("Uptime:       %s\n", uptime)
		if frameAge != "" {
			fmt.Printf("Frame age:    %s\n", frameAge)
		}
	} else {
		fmt.Println("Not connected")
	}
}
