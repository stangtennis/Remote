package main

import (
	"fmt"
	"log"
	"os"
	"time"
)

func handleStatus(connMgr *ConnectionManager, deviceID, deviceName string, startTime time.Time) daemonResponse {
	conn, err := connMgr.GetConnection(deviceID)
	if err != nil {
		return daemonResponse{OK: true, Data: map[string]interface{}{
			"connected": false,
		}}
	}

	data := map[string]interface{}{
		"connected":   true,
		"device_id":   deviceID,
		"device_name": deviceName,
		"pid":         float64(os.Getpid()),
		"uptime":      time.Since(startTime).Round(time.Second).String(),
	}

	frame, frameTime := conn.GetLastFrame()
	if frame != nil {
		data["frame_age"] = time.Since(frameTime).Round(time.Millisecond).String()
	}

	return daemonResponse{OK: true, Data: data}
}

func handleScreenshot(req daemonRequest, connMgr *ConnectionManager, deviceID string) daemonResponse {
	conn, err := connMgr.GetConnection(deviceID)
	if err != nil {
		return daemonResponse{OK: false, Error: err.Error()}
	}

	maxWidth := getIntArg(req.Args, "max_width", 1280)
	quality := getIntArg(req.Args, "quality", 60)
	file := getStringArg(req.Args, "file", "/tmp/rd-screenshot.jpg")

	// Get frame (wait if none yet)
	frame, _ := conn.GetLastFrame()
	if frame == nil {
		log.Printf("[daemon] No frame cached, waiting up to 10s...")
		for i := 0; i < 100; i++ {
			time.Sleep(100 * time.Millisecond)
			frame, _ = conn.GetLastFrame()
			if frame != nil {
				break
			}
		}
		if frame == nil {
			return daemonResponse{OK: false, Error: "no video frame received (waited 10s)"}
		}
	}

	// Downscale
	scaled, w, h, err := downscaleJPEG(frame, maxWidth, quality)
	if err != nil {
		return daemonResponse{OK: false, Error: fmt.Sprintf("failed to process screenshot: %v", err)}
	}

	// Write to file
	if err := os.WriteFile(file, scaled, 0644); err != nil {
		return daemonResponse{OK: false, Error: fmt.Sprintf("failed to write file: %v", err)}
	}

	return daemonResponse{OK: true, Data: map[string]interface{}{
		"file":   file,
		"width":  float64(w),
		"height": float64(h),
		"bytes":  float64(len(scaled)),
	}}
}

func handleClick(req daemonRequest, connMgr *ConnectionManager, deviceID string) daemonResponse {
	conn, err := connMgr.GetConnection(deviceID)
	if err != nil {
		return daemonResponse{OK: false, Error: err.Error()}
	}

	x := getIntArg(req.Args, "x", 0)
	y := getIntArg(req.Args, "y", 0)
	button := getStringArg(req.Args, "button", "left")
	doubleClick := getBoolArg(req.Args, "double_click", false)

	// Move mouse first
	if err := conn.SendInput(buildMouseMove(x, y)); err != nil {
		return daemonResponse{OK: false, Error: fmt.Sprintf("failed to move mouse: %v", err)}
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
			return daemonResponse{OK: false, Error: fmt.Sprintf("failed to send click: %v", err)}
		}
		time.Sleep(5 * time.Millisecond)
	}

	return daemonResponse{OK: true}
}

func handleType(req daemonRequest, connMgr *ConnectionManager, deviceID string) daemonResponse {
	conn, err := connMgr.GetConnection(deviceID)
	if err != nil {
		return daemonResponse{OK: false, Error: err.Error()}
	}

	text := getStringArg(req.Args, "text", "")
	if text == "" {
		return daemonResponse{OK: false, Error: "no text provided"}
	}

	events := buildTypeText(text)
	for _, evt := range events {
		if err := conn.SendInput(evt); err != nil {
			return daemonResponse{OK: false, Error: fmt.Sprintf("failed to send key: %v", err)}
		}
		time.Sleep(5 * time.Millisecond)
	}

	return daemonResponse{OK: true, Data: map[string]interface{}{
		"chars": float64(len(text)),
	}}
}

func handleKey(req daemonRequest, connMgr *ConnectionManager, deviceID string) daemonResponse {
	conn, err := connMgr.GetConnection(deviceID)
	if err != nil {
		return daemonResponse{OK: false, Error: err.Error()}
	}

	key := getStringArg(req.Args, "key", "")
	if key == "" {
		return daemonResponse{OK: false, Error: "no key provided"}
	}
	ctrl := getBoolArg(req.Args, "ctrl", false)
	shift := getBoolArg(req.Args, "shift", false)
	alt := getBoolArg(req.Args, "alt", false)

	keyCode := parseKeyName(key)
	events := buildKeyPress(keyCode, ctrl, shift, alt)

	for _, evt := range events {
		if err := conn.SendInput(evt); err != nil {
			return daemonResponse{OK: false, Error: fmt.Sprintf("failed to send key: %v", err)}
		}
		time.Sleep(5 * time.Millisecond)
	}

	return daemonResponse{OK: true}
}

func handleScroll(req daemonRequest, connMgr *ConnectionManager, deviceID string) daemonResponse {
	conn, err := connMgr.GetConnection(deviceID)
	if err != nil {
		return daemonResponse{OK: false, Error: err.Error()}
	}

	delta := getIntArg(req.Args, "delta", 0)
	x := getIntArg(req.Args, "x", -1)
	y := getIntArg(req.Args, "y", -1)

	// Move mouse first if coordinates provided
	if x >= 0 && y >= 0 {
		if err := conn.SendInput(buildMouseMove(x, y)); err != nil {
			return daemonResponse{OK: false, Error: fmt.Sprintf("failed to move mouse: %v", err)}
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err := conn.SendInput(buildScroll(delta)); err != nil {
		return daemonResponse{OK: false, Error: fmt.Sprintf("failed to scroll: %v", err)}
	}

	return daemonResponse{OK: true}
}

func handleDisconnect(connMgr *ConnectionManager, deviceID string) daemonResponse {
	if err := connMgr.Disconnect(deviceID); err != nil {
		return daemonResponse{OK: false, Error: err.Error()}
	}
	return daemonResponse{OK: true}
}

// --- Arg helpers ---

func getIntArg(args map[string]interface{}, key string, def int) int {
	if args == nil {
		return def
	}
	v, ok := args[key]
	if !ok {
		return def
	}
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	case string:
		if n, err := fmt.Sscanf(val, "%d"); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func getStringArg(args map[string]interface{}, key string, def string) string {
	if args == nil {
		return def
	}
	v, ok := args[key]
	if !ok {
		return def
	}
	if s, ok := v.(string); ok {
		return s
	}
	return def
}

func getBoolArg(args map[string]interface{}, key string, def bool) bool {
	if args == nil {
		return def
	}
	v, ok := args[key]
	if !ok {
		return def
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return def
}
