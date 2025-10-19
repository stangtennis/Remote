package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/go-vgo/robotgo"
)

// Message represents a message from/to the extension
type Message struct {
	Type    string                 `json:"type"`
	Command map[string]interface{} `json:"command,omitempty"`
	Status  string                 `json:"status,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

func main() {
	// Send initial connected message
	sendMessage(Message{
		Type:   "connected",
		Status: "ready",
	})

	// Read messages from stdin (Chrome extension)
	for {
		msg, err := readMessage()
		if err != nil {
			if err == io.EOF {
				break
			}
			logError("Failed to read message: %v", err)
			continue
		}

		handleMessage(msg)
	}
}

// Read a native messaging message (length-prefixed JSON)
func readMessage() (Message, error) {
	var msg Message

	// Read message length (4 bytes, little-endian)
	var length uint32
	if err := binary.Read(os.Stdin, binary.LittleEndian, &length); err != nil {
		return msg, err
	}

	// Read message data
	data := make([]byte, length)
	if _, err := io.ReadFull(os.Stdin, data); err != nil {
		return msg, err
	}

	// Parse JSON
	if err := json.Unmarshal(data, &msg); err != nil {
		return msg, err
	}

	return msg, nil
}

// Send a native messaging message
func sendMessage(msg Message) error {
	// Marshal to JSON
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// Write length (4 bytes, little-endian)
	length := uint32(len(data))
	if err := binary.Write(os.Stdout, binary.LittleEndian, length); err != nil {
		return err
	}

	// Write data
	if _, err := os.Stdout.Write(data); err != nil {
		return err
	}

	return nil
}

// Handle incoming message
func handleMessage(msg Message) {
	switch msg.Type {
	case "ping":
		sendMessage(Message{Type: "pong"})

	case "input":
		handleInput(msg.Command)

	default:
		logError("Unknown message type: %s", msg.Type)
	}
}

// Handle input command
func handleInput(cmd map[string]interface{}) {
	inputType, ok := cmd["type"].(string)
	if !ok {
		sendError("Invalid input type")
		return
	}

	switch inputType {
	case "mouse_move":
		handleMouseMove(cmd)
	case "mouse_click":
		handleMouseClick(cmd)
	case "mouse_down":
		handleMouseButton(cmd, true)
	case "mouse_up":
		handleMouseButton(cmd, false)
	case "mouse_scroll":
		handleMouseScroll(cmd)
	case "keyboard_press":
		handleKeyboardPress(cmd)
	case "keyboard_type":
		handleKeyboardType(cmd)
	default:
		sendError(fmt.Sprintf("Unknown input type: %s", inputType))
	}
}

// Mouse move
func handleMouseMove(cmd map[string]interface{}) {
	x := int(cmd["x"].(float64))
	y := int(cmd["y"].(float64))

	robotgo.Move(x, y)
	sendMessage(Message{Type: "input_success"})
}

// Mouse click
func handleMouseClick(cmd map[string]interface{}) {
	button := "left"
	if b, ok := cmd["button"].(string); ok {
		button = b
	}

	double := false
	if d, ok := cmd["double"].(bool); ok {
		double = d
	}

	if double {
		robotgo.Click(button, true)
	} else {
		robotgo.Click(button, false)
	}

	sendMessage(Message{Type: "input_success"})
}

// Mouse button down/up
func handleMouseButton(cmd map[string]interface{}, down bool) {
	button := "left"
	if b, ok := cmd["button"].(string); ok {
		button = b
	}

	if down {
		robotgo.Toggle(button, "down")
	} else {
		robotgo.Toggle(button, "up")
	}

	sendMessage(Message{Type: "input_success"})
}

// Mouse scroll
func handleMouseScroll(cmd map[string]interface{}) {
	deltaY := 0
	if d, ok := cmd["deltaY"].(float64); ok {
		deltaY = int(d)
	}

	// Normalize scroll amount (positive = down, negative = up)
	scrollAmount := deltaY / 10
	if scrollAmount == 0 && deltaY != 0 {
		if deltaY > 0 {
			scrollAmount = 1
		} else {
			scrollAmount = -1
		}
	}

	// robotgo.Scroll(x, y) - negative y scrolls up, positive scrolls down
	robotgo.Scroll(0, scrollAmount)
	sendMessage(Message{Type: "input_success"})
}

// Keyboard press
func handleKeyboardPress(cmd map[string]interface{}) {
	key, ok := cmd["key"].(string)
	if !ok {
		sendError("Missing key")
		return
	}

	// Get modifiers
	var mods []string
	if modifiers, ok := cmd["modifiers"].(map[string]interface{}); ok {
		if ctrl, ok := modifiers["ctrl"].(bool); ok && ctrl {
			mods = append(mods, "ctrl")
		}
		if alt, ok := modifiers["alt"].(bool); ok && alt {
			mods = append(mods, "alt")
		}
		if shift, ok := modifiers["shift"].(bool); ok && shift {
			mods = append(mods, "shift")
		}
		if meta, ok := modifiers["meta"].(bool); ok && meta {
			mods = append(mods, "cmd")
		}
	}

	// Map key to robotgo format
	robotgoKey := mapKeyToRobotgo(key)

	// Press key with modifiers
	if len(mods) > 0 {
		robotgo.KeyTap(robotgoKey, mods)
	} else {
		robotgo.KeyTap(robotgoKey)
	}

	sendMessage(Message{Type: "input_success"})
}

// Keyboard type
func handleKeyboardType(cmd map[string]interface{}) {
	text, ok := cmd["text"].(string)
	if !ok {
		sendError("Missing text")
		return
	}

	robotgo.TypeStr(text)
	sendMessage(Message{Type: "input_success"})
}

// Map JavaScript key names to robotgo key names
func mapKeyToRobotgo(key string) string {
	keyMap := map[string]string{
		"Enter":     "enter",
		"Backspace": "backspace",
		"Delete":    "delete",
		"Tab":       "tab",
		"Escape":    "esc",
		"Space":     "space",
		"ArrowUp":   "up",
		"ArrowDown": "down",
		"ArrowLeft": "left",
		"ArrowRight": "right",
		"Home":      "home",
		"End":       "end",
		"PageUp":    "pageup",
		"PageDown":  "pagedown",
		"Insert":    "insert",
	}

	if mapped, ok := keyMap[key]; ok {
		return mapped
	}

	// For single character keys, use lowercase
	if len(key) == 1 {
		return key
	}

	// For function keys (F1-F12)
	if len(key) > 1 && key[0] == 'F' {
		return key
	}

	return key
}

// Send error message
func sendError(errMsg string) {
	sendMessage(Message{
		Type:  "input_error",
		Error: errMsg,
	})
}

// Log error to stderr
func logError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "[Native Host Error] "+format+"\n", args...)
}
