package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stangtennis/Remote/controller/internal/config"
)

func registerTools(s *server.MCPServer, cfg *config.Config, auth *authInfo, connMgr *ConnectionManager) {
	// list_devices
	s.AddTool(
		mcp.NewTool("list_devices",
			mcp.WithDescription("List all registered remote desktop agents with online/offline status"),
		),
		listDevicesHandler(cfg, auth),
	)

	// connect_device
	s.AddTool(
		mcp.NewTool("connect_device",
			mcp.WithDescription("Connect to a remote desktop agent via WebRTC. Required before screenshot/input tools."),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID to connect to (from list_devices)"),
			),
		),
		connectDeviceHandler(cfg, auth, connMgr),
	)

	// disconnect_device
	s.AddTool(
		mcp.NewTool("disconnect_device",
			mcp.WithDescription("Disconnect from a remote desktop agent"),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID to disconnect from"),
			),
		),
		disconnectDeviceHandler(connMgr),
	)

	// screenshot
	s.AddTool(
		mcp.NewTool("screenshot",
			mcp.WithDescription("Take a screenshot of the remote desktop. Returns the image and its dimensions for coordinate mapping."),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID to screenshot"),
			),
			mcp.WithNumber("max_width",
				mcp.Description("Maximum image width in pixels (default: 1280)"),
			),
			mcp.WithNumber("quality",
				mcp.Description("JPEG quality 1-100 (default: 60)"),
			),
		),
		screenshotHandler(connMgr),
	)

	// click
	s.AddTool(
		mcp.NewTool("click",
			mcp.WithDescription("Click at x,y coordinates on the remote desktop"),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID"),
			),
			mcp.WithNumber("x",
				mcp.Required(),
				mcp.Description("X coordinate (in screenshot pixel space)"),
			),
			mcp.WithNumber("y",
				mcp.Required(),
				mcp.Description("Y coordinate (in screenshot pixel space)"),
			),
			mcp.WithString("button",
				mcp.Description("Mouse button: left, right, middle (default: left)"),
			),
			mcp.WithBoolean("double_click",
				mcp.Description("Double-click instead of single click (default: false)"),
			),
		),
		clickHandler(connMgr),
	)

	// type_text
	s.AddTool(
		mcp.NewTool("type_text",
			mcp.WithDescription("Type text on the remote desktop by sending individual key events"),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID"),
			),
			mcp.WithString("text",
				mcp.Required(),
				mcp.Description("Text to type"),
			),
		),
		typeTextHandler(connMgr),
	)

	// press_key
	s.AddTool(
		mcp.NewTool("press_key",
			mcp.WithDescription("Press a key with optional modifiers on the remote desktop"),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID"),
			),
			mcp.WithString("key",
				mcp.Required(),
				mcp.Description("Key name: Enter, Tab, Escape, Backspace, Delete, Space, ArrowUp/Down/Left/Right, Home, End, PageUp, PageDown, F1-F12, or a single character"),
			),
			mcp.WithBoolean("ctrl",
				mcp.Description("Hold Ctrl (default: false)"),
			),
			mcp.WithBoolean("shift",
				mcp.Description("Hold Shift (default: false)"),
			),
			mcp.WithBoolean("alt",
				mcp.Description("Hold Alt (default: false)"),
			),
		),
		pressKeyHandler(connMgr),
	)

	// scroll
	s.AddTool(
		mcp.NewTool("scroll",
			mcp.WithDescription("Scroll the mouse wheel on the remote desktop"),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID"),
			),
			mcp.WithNumber("delta",
				mcp.Required(),
				mcp.Description("Scroll amount: positive = scroll down, negative = scroll up (e.g., 3 or -3)"),
			),
			mcp.WithNumber("x",
				mcp.Description("X coordinate to scroll at (optional, moves mouse first)"),
			),
			mcp.WithNumber("y",
				mcp.Description("Y coordinate to scroll at (optional, moves mouse first)"),
			),
		),
		scrollHandler(connMgr),
	)
}

// --- Tool handlers ---

func listDevicesHandler(cfg *config.Config, auth *authInfo) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		devices, err := fetchDevices(cfg.SupabaseURL, cfg.SupabaseAnonKey, auth)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch devices: %v", err)), nil
		}

		if len(devices) == 0 {
			return mcp.NewToolResultText("No devices registered."), nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Found %d device(s):\n\n", len(devices)))
		for _, d := range devices {
			status := d.statusString()
			lastSeen := "never"
			if !d.LastSeen.IsZero() {
				lastSeen = time.Since(d.LastSeen).Round(time.Second).String() + " ago"
			}
			sb.WriteString(fmt.Sprintf("- **%s** (%s)\n  ID: `%s`\n  Status: %s | Last seen: %s\n\n",
				d.DeviceName, d.Platform, d.DeviceID, status, lastSeen))
		}

		return mcp.NewToolResultText(sb.String()), nil
	}
}

func connectDeviceHandler(cfg *config.Config, auth *authInfo, connMgr *ConnectionManager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		deviceID, err := request.RequireString("device_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Look up device name
		devices, err := fetchDevices(cfg.SupabaseURL, cfg.SupabaseAnonKey, auth)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch devices: %v", err)), nil
		}

		deviceName := deviceID
		for _, d := range devices {
			if d.DeviceID == deviceID {
				deviceName = d.DeviceName
				if !d.isOnline() {
					return mcp.NewToolResultError(fmt.Sprintf("Device '%s' is offline (last seen: %s)", d.DeviceName, d.LastSeen.Format(time.RFC3339))), nil
				}
				break
			}
		}

		if err := connMgr.Connect(deviceID, deviceName); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to connect: %v", err)), nil
		}

		// Wait a moment for first frame
		time.Sleep(500 * time.Millisecond)

		return mcp.NewToolResultText(fmt.Sprintf("Connected to '%s' (%s). You can now use screenshot, click, type_text, press_key, and scroll tools.", deviceName, deviceID)), nil
	}
}

func disconnectDeviceHandler(connMgr *ConnectionManager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		deviceID, err := request.RequireString("device_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if err := connMgr.Disconnect(deviceID); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to disconnect: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Disconnected from %s", deviceID)), nil
	}
}

func screenshotHandler(connMgr *ConnectionManager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		deviceID, err := request.RequireString("device_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		maxWidth := request.GetInt("max_width", 1280)
		quality := request.GetInt("quality", 60)

		conn, err := connMgr.GetConnection(deviceID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Get the cached frame
		frame, frameTime := conn.GetLastFrame()
		if frame == nil {
			// Wait for a frame (agent may take up to ~6s to start streaming after connect)
			log.Printf("[MCP] No frame cached, waiting up to 10s...")
			for i := 0; i < 100; i++ {
				time.Sleep(100 * time.Millisecond)
				frame, frameTime = conn.GetLastFrame()
				if frame != nil {
					break
				}
			}
			if frame == nil {
				return mcp.NewToolResultError("No video frame received from device (waited 10s). The agent may not be streaming."), nil
			}
		}

		// Downscale
		scaled, w, h, err := downscaleJPEG(frame, maxWidth, quality)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to process screenshot: %v", err)), nil
		}

		// Encode to base64
		b64 := base64.StdEncoding.EncodeToString(scaled)

		frameAge := time.Since(frameTime).Round(time.Millisecond)
		log.Printf("[MCP] Screenshot: %dx%d, %d bytes, frame age: %s", w, h, len(scaled), frameAge)

		// Return image + metadata text
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Screenshot: %dx%d pixels (frame age: %s)\nUse these coordinates for click/scroll tools. Coordinates are in this image's pixel space.", w, h, frameAge),
				},
				mcp.ImageContent{
					Type:     "image",
					Data:     b64,
					MIMEType: "image/jpeg",
				},
			},
		}, nil
	}
}

func clickHandler(connMgr *ConnectionManager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		deviceID, err := request.RequireString("device_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		x := request.GetInt("x", 0)
		y := request.GetInt("y", 0)
		button := request.GetString("button", "left")
		doubleClick := request.GetBool("double_click", false)

		conn, err := connMgr.GetConnection(deviceID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Move mouse first
		if err := conn.SendInput(buildMouseMove(x, y)); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to move mouse: %v", err)), nil
		}
		time.Sleep(10 * time.Millisecond)

		// Click
		var events []string
		if doubleClick {
			events = buildMouseDoubleClick(x, y, button)
		} else {
			events = buildMouseClick(x, y, button)
		}

		for _, evt := range events {
			if err := conn.SendInput(evt); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to send click: %v", err)), nil
			}
			time.Sleep(5 * time.Millisecond)
		}

		action := "Clicked"
		if doubleClick {
			action = "Double-clicked"
		}
		return mcp.NewToolResultText(fmt.Sprintf("%s at (%d, %d) with %s button", action, x, y, button)), nil
	}
}

func typeTextHandler(connMgr *ConnectionManager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		deviceID, err := request.RequireString("device_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		text, err := request.RequireString("text")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		conn, err := connMgr.GetConnection(deviceID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		events := buildTypeText(text)
		for _, evt := range events {
			if err := conn.SendInput(evt); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to send key: %v", err)), nil
			}
			time.Sleep(5 * time.Millisecond)
		}

		return mcp.NewToolResultText(fmt.Sprintf("Typed %d characters", len(text))), nil
	}
}

func pressKeyHandler(connMgr *ConnectionManager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		deviceID, err := request.RequireString("device_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		key, err := request.RequireString("key")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		ctrl := request.GetBool("ctrl", false)
		shift := request.GetBool("shift", false)
		alt := request.GetBool("alt", false)

		conn, err := connMgr.GetConnection(deviceID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		keyCode := parseKeyName(key)
		events := buildKeyPress(keyCode, ctrl, shift, alt)

		for _, evt := range events {
			if err := conn.SendInput(evt); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to send key: %v", err)), nil
			}
			time.Sleep(5 * time.Millisecond)
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
		return mcp.NewToolResultText(fmt.Sprintf("Pressed %s%s", modifiers, key)), nil
	}
}

func scrollHandler(connMgr *ConnectionManager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		deviceID, err := request.RequireString("device_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		delta := request.GetInt("delta", 0)
		x := request.GetInt("x", -1)
		y := request.GetInt("y", -1)

		conn, err := connMgr.GetConnection(deviceID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Move mouse to position first if coordinates provided
		if x >= 0 && y >= 0 {
			if err := conn.SendInput(buildMouseMove(x, y)); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to move mouse: %v", err)), nil
			}
			time.Sleep(10 * time.Millisecond)
		}

		if err := conn.SendInput(buildScroll(delta)); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to scroll: %v", err)), nil
		}

		direction := "down"
		if delta < 0 {
			direction = "up"
		}
		return mcp.NewToolResultText(fmt.Sprintf("Scrolled %s by %d", direction, abs(delta))), nil
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
