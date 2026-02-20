//go:build darwin

package auth

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ShowLoginDialog displays a login dialog.
// On macOS, uses osascript (AppleScript) for GUI dialog or falls back to terminal.
func ShowLoginDialog(config AuthConfig) *AuthResult {
	// Try osascript GUI dialog first
	email, err := osascriptInput("Remote Desktop Agent - Login", "Enter your email:")
	if err != nil {
		// Fallback to terminal input
		return terminalLogin(config)
	}

	password, err := osascriptPassword("Remote Desktop Agent - Login", "Enter your password:")
	if err != nil {
		return terminalLogin(config)
	}

	if email == "" || password == "" {
		return nil
	}

	// Perform login
	result, err := Login(config, email, password)
	if err != nil {
		osascriptAlert("Login Error", "Could not connect to server: "+err.Error())
		return nil
	}

	if !result.Success {
		osascriptAlert("Login Failed", result.Message)
		return result
	}

	return result
}

func osascriptInput(title, prompt string) (string, error) {
	script := fmt.Sprintf(`display dialog "%s" default answer "" with title "%s" buttons {"Cancel", "OK"} default button "OK"`, prompt, title)
	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	// Output format: "button returned:OK, text returned:user@example.com"
	text := string(output)
	if idx := strings.Index(text, "text returned:"); idx >= 0 {
		result := strings.TrimSpace(text[idx+len("text returned:"):])
		return result, nil
	}
	return "", fmt.Errorf("unexpected osascript output")
}

func osascriptPassword(title, prompt string) (string, error) {
	script := fmt.Sprintf(`display dialog "%s" default answer "" with title "%s" buttons {"Cancel", "OK"} default button "OK" with hidden answer`, prompt, title)
	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	text := string(output)
	if idx := strings.Index(text, "text returned:"); idx >= 0 {
		result := strings.TrimSpace(text[idx+len("text returned:"):])
		return result, nil
	}
	return "", fmt.Errorf("unexpected osascript output")
}

func osascriptAlert(title, message string) {
	script := fmt.Sprintf(`display alert "%s" message "%s" as warning`, title, message)
	exec.Command("osascript", "-e", script).Run()
}

func terminalLogin(config AuthConfig) *AuthResult {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Email: ")
	email, _ := reader.ReadString('\n')
	email = strings.TrimSpace(email)

	fmt.Print("Password: ")
	password, _ := reader.ReadString('\n')
	password = strings.TrimSpace(password)

	if email == "" || password == "" {
		fmt.Println("Email and password are required")
		return nil
	}

	result, err := Login(config, email, password)
	if err != nil {
		fmt.Printf("Login error: %v\n", err)
		return nil
	}

	if !result.Success {
		fmt.Printf("Login failed: %s\n", result.Message)
	}

	return result
}
