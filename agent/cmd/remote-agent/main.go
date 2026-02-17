//go:build windows
// +build windows

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/stangtennis/remote-agent/internal/auth"
	"github.com/stangtennis/remote-agent/internal/config"
	"github.com/stangtennis/remote-agent/internal/desktop"
	"github.com/stangtennis/remote-agent/internal/device"
	"github.com/stangtennis/remote-agent/internal/tray"
	"github.com/stangtennis/remote-agent/internal/webrtc"
	"github.com/stangtennis/remote-agent/pkg/logging"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

var (
	user32        = syscall.NewLazyDLL("user32.dll")
	messageBoxW   = user32.NewProc("MessageBoxW")
	shell32       = syscall.NewLazyDLL("shell32.dll")
	shellExecuteW = shell32.NewProc("ShellExecuteW")
)

const (
	MB_OK              = 0x00000000
	MB_OKCANCEL        = 0x00000001
	MB_YESNOCANCEL     = 0x00000003
	MB_YESNO           = 0x00000004
	MB_ICONQUESTION    = 0x00000020
	MB_ICONINFORMATION = 0x00000040
	MB_ICONWARNING     = 0x00000030
	MB_ICONERROR       = 0x00000010
	IDOK               = 1
	IDCANCEL           = 2
	IDYES              = 6
	IDNO               = 7

	// Menu options
	MENU_LOGIN           = 1
	MENU_INSTALL_SERVICE = 2
	MENU_UNINSTALL       = 3
	MENU_RUN_ONCE        = 4
	MENU_UPDATE          = 5
	MENU_LOGOUT          = 6
	MENU_EXIT            = 7
)

var (
	cfg         *config.Config
	dev         *device.Device
	rtc         *webrtc.Manager
	logFile     *os.File
	currentUser *auth.Credentials
)

func setupLogging() error {
	// Initialize structured logging with rotation
	isService, _ := svc.IsWindowsService()
	
	cfg := logging.DefaultConfig()
	cfg.Console = !isService // Only log to console if not running as service
	cfg.Level = "info"        // Can be changed to "debug" for troubleshooting
	
	if err := logging.Init(cfg); err != nil {
		return fmt.Errorf("failed to initialize logging: %w", err)
	}

	logging.Logger.Info().
		Str("version", tray.VersionString).
		Bool("is_service", isService).
		Msg("Remote Desktop Agent starting")

	// Sync log file immediately
	logFile.Sync()

	// Start background goroutine to periodically sync log file
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if logFile != nil {
				logFile.Sync()
			}
		}
	}()

	return nil
}

// flushLog ensures all log data is written to disk
func flushLog() {
	if logFile != nil {
		logFile.Sync()
	}
}

const serviceName = "RemoteDesktopAgent"
const serviceDisplayName = "Remote Desktop Agent"
const serviceDescription = "Provides remote desktop access with login screen support"

// messageBox shows a Windows message box dialog
func messageBox(title, message string, flags uint32) int {
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	messagePtr, _ := syscall.UTF16PtrFromString(message)
	ret, _, _ := messageBoxW.Call(0, uintptr(unsafe.Pointer(messagePtr)), uintptr(unsafe.Pointer(titlePtr)), uintptr(flags))
	return int(ret)
}

// isAdmin checks if the current process has administrator privileges
func isAdmin() bool {
	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	return err == nil
}

// relaunchAsAdmin restarts the current process with admin privileges via UAC
func relaunchAsAdmin() {
	exe, err := os.Executable()
	if err != nil {
		log.Printf("❌ Kunne ikke finde exe sti: %v", err)
		return
	}

	// Get command line arguments (skip the exe name)
	args := strings.Join(os.Args[1:], " ")

	// Use ShellExecute with "runas" verb to trigger UAC
	verbPtr, _ := syscall.UTF16PtrFromString("runas")
	exePtr, _ := syscall.UTF16PtrFromString(exe)
	argsPtr, _ := syscall.UTF16PtrFromString(args)
	dirPtr, _ := syscall.UTF16PtrFromString("")

	ret, _, _ := shellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verbPtr)),
		uintptr(unsafe.Pointer(exePtr)),
		uintptr(unsafe.Pointer(argsPtr)),
		uintptr(unsafe.Pointer(dirPtr)),
		1, // SW_SHOWNORMAL
	)

	if ret <= 32 {
		log.Printf("❌ Kunne ikke genstarte som admin (fejlkode: %d)", ret)
	}
}

// askYesNo prompts the user for a yes/no answer
func askYesNo(question string) bool {
	result := messageBox("Remote Desktop Agent", question, MB_YESNO|MB_ICONQUESTION)
	return result == IDYES
}

// isFirewallRuleExists checks if the firewall rule already exists for THIS executable
func isFirewallRuleExists() bool {
	exePath, err := os.Executable()
	if err != nil {
		return false
	}

	cmd := exec.Command("netsh", "advfirewall", "firewall", "show", "rule", "name=Remote Desktop Agent", "verbose")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	outputStr := string(output)
	// Check if rule exists AND matches current executable path
	if strings.Contains(outputStr, "No rules match") {
		return false
	}

	// Check if the program path in the rule matches our current exe
	// This ensures we update the rule if the exe moved/was updated
	return strings.Contains(strings.ToLower(outputStr), strings.ToLower(exePath))
}

// setupFirewallRules adds Windows Firewall rules to allow the agent
func setupFirewallRules() {
	exePath, err := os.Executable()
	if err != nil {
		log.Printf("⚠️ Kunne ikke finde exe sti til firewall: %v", err)
		return
	}

	log.Printf("🔥 Tjekker firewall regler for: %s", exePath)

	// Check if rule already exists for THIS exe
	if isFirewallRuleExists() {
		log.Println("✅ Firewall regel findes allerede for denne exe")
		return
	}

	log.Println("🔥 Firewall regel mangler eller forældet - tilføjer nye regler...")

	// If not admin, we can't add rules (but we should be admin due to self-elevation)
	if !isAdmin() {
		log.Println("⚠️ Kører ikke som admin - kan ikke tilføje firewall regler")
		
		// Run netsh as admin using PowerShell
		// This will show a UAC prompt
		psScript := fmt.Sprintf(`
			$exePath = '%s'
			netsh advfirewall firewall delete rule name="Remote Desktop Agent" 2>$null
			netsh advfirewall firewall add rule name="Remote Desktop Agent" dir=in action=allow program="$exePath" enable=yes profile=any
			netsh advfirewall firewall add rule name="Remote Desktop Agent" dir=out action=allow program="$exePath" enable=yes profile=any
		`, exePath)
		
		cmd := exec.Command("powershell", "-Command", 
			"Start-Process", "powershell", 
			"-ArgumentList", fmt.Sprintf(`'-Command', '%s'`, strings.ReplaceAll(psScript, "'", "''")),
			"-Verb", "RunAs", "-Wait")
		
		if err := cmd.Run(); err != nil {
			log.Printf("⚠️ Kunne ikke opsætte firewall (UAC afvist?): %v", err)
		} else {
			log.Println("✅ Firewall regler tilføjet via UAC")
		}
		return
	}

	log.Println("🔥 Opsætter Windows Firewall regler...")

	// Delete existing rules first (ignore errors)
	deleteCmd := exec.Command("netsh", "advfirewall", "firewall", "delete", "rule", "name=Remote Desktop Agent")
	deleteCmd.Run()

	// Add inbound rule
	inCmd := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		"name=Remote Desktop Agent",
		"dir=in",
		"action=allow",
		"program="+exePath,
		"enable=yes",
		"profile=any")
	if err := inCmd.Run(); err != nil {
		log.Printf("⚠️ Kunne ikke tilføje indgående firewall regel: %v", err)
	} else {
		log.Println("✅ Indgående firewall regel tilføjet")
	}

	// Add outbound rule
	outCmd := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		"name=Remote Desktop Agent",
		"dir=out",
		"action=allow",
		"program="+exePath,
		"enable=yes",
		"profile=any")
	if err := outCmd.Run(); err != nil {
		log.Printf("⚠️ Kunne ikke tilføje udgående firewall regel: %v", err)
	} else {
		log.Println("✅ Udgående firewall regel tilføjet")
	}
}

// runAsAdmin restarts the current process with administrator privileges
func runAsAdmin() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	verb, _ := syscall.UTF16PtrFromString("runas")
	exe, _ := syscall.UTF16PtrFromString(exePath)
	args, _ := syscall.UTF16PtrFromString("")
	dir, _ := syscall.UTF16PtrFromString(filepath.Dir(exePath))

	ret, _, _ := shellExecuteW.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(exe)),
		uintptr(unsafe.Pointer(args)), uintptr(unsafe.Pointer(dir)), 1)

	if ret <= 32 {
		return fmt.Errorf("ShellExecute failed with code %d", ret)
	}
	return nil
}

func main() {
	// Check for update mode FIRST (before admin check or anything else)
	if len(os.Args) >= 3 && os.Args[1] == "--update-from" {
		runUpdateMode(os.Args[2])
		return
	}

	// Check if running from Program Files install directory (autostart mode)
	isService, _ := svc.IsWindowsService()
	runningFromProgramFiles := isRunningFromProgramFiles()

	// If running from Program Files, don't require admin (autostart runs as normal user)
	// Otherwise, request admin elevation
	if !isService && !runningFromProgramFiles && !isAdmin() {
		fmt.Println("🔒 Administrator rettigheder kræves. Anmoder om elevation...")
		relaunchAsAdmin()
		return // Exit this instance, the elevated one will take over
	}

	// Parse command-line flags (keep for advanced users)
	installFlag := flag.Bool("install", false, "Install as Windows Service")
	uninstallFlag := flag.Bool("uninstall", false, "Uninstall Windows Service")
	startFlag := flag.Bool("start", false, "Start the Windows Service")
	stopFlag := flag.Bool("stop", false, "Stop the Windows Service")
	statusFlag := flag.Bool("status", false, "Show service status")
	logoutFlag := flag.Bool("logout", false, "Log out and clear saved credentials")
	helpFlag := flag.Bool("help", false, "Show help")
	silentFlag := flag.Bool("silent", false, "Run without GUI prompts")
	consoleFlag := flag.Bool("console", false, "Run in console mode without system tray (full logging)")
	flag.Parse()

	// Handle command-line flags (for advanced users / scripting)
	if *helpFlag {
		printUsage()
		return
	}
	if *installFlag {
		if err := installService(); err != nil {
			fmt.Printf("❌ Kunne ikke installere service: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if *uninstallFlag {
		if err := uninstallService(); err != nil {
			fmt.Printf("❌ Kunne ikke afinstallere service: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if *startFlag {
		if err := startService(); err != nil {
			fmt.Printf("❌ Kunne ikke starte service: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if *stopFlag {
		if err := stopService(); err != nil {
			fmt.Printf("❌ Kunne ikke stoppe service: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if *statusFlag {
		showServiceStatus()
		return
	}
	if *logoutFlag {
		if err := auth.ClearCredentials(); err != nil {
			fmt.Printf("❌ Kunne ikke rydde login oplysninger: %v\n", err)
		} else {
			fmt.Println("✅ Logget ud!")
			fmt.Println("   Login oplysninger er ryddet.")
			fmt.Println("   Kør agent igen for at logge ind med en anden konto.")
		}
		return
	}

	// Check if running as Windows Service
	isWindowsService, err := svc.IsWindowsService()
	if err != nil {
		fmt.Printf("Kunne ikke afgøre om kører som service: %v\n", err)
		os.Exit(1)
	}

	if isWindowsService {
		// Setup logging for service mode
		if err := setupLogging(); err != nil {
			os.Exit(1)
		}
		defer func() {
			if logFile != nil {
				logFile.Close()
			}
		}()
		log.Println("🔧 Kører som Windows Service")
		runService()
		return
	}

	// If running from Program Files and logged in, auto-start as background agent with tray
	if runningFromProgramFiles && auth.IsLoggedIn() && !*silentFlag && !*consoleFlag {
		if err := setupLogging(); err != nil {
			fmt.Printf("Kunne ikke opsætte logging: %v\n", err)
			os.Exit(1)
		}
		defer func() {
			if logFile != nil {
				logFile.Close()
			}
		}()
		log.Println("🔧 Kører fra Program Files - auto-start med system tray")
		runInteractive()
		return
	}

	// Interactive mode - show GUI unless -silent flag
	if !*silentFlag && !*consoleFlag {
		// Note: GUI mode sets up its own logging in NewAgentGUI()
		showGUI()
		return
	}

	// Silent mode - just run interactively
	if err := setupLogging(); err != nil {
		fmt.Printf("Kunne ikke opsætte logging: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if logFile != nil {
			logFile.Close()
		}
	}()
	
	// Console mode - run without system tray, keep CMD open
	if *consoleFlag {
		log.Println("🔧 Kører i KONSOL tilstand (ingen system tray)")
		runConsoleMode()
		return
	}
	
	log.Println("🔧 Kører i interaktiv tilstand")
	runInteractive()
}

// showStartupDialog shows the main startup dialog with options
func showStartupDialog() {
	// Check login status
	isLoggedIn := auth.IsLoggedIn()
	var userEmail string
	if isLoggedIn {
		if creds, err := auth.GetCurrentUser(); err == nil {
			userEmail = creds.Email
		}
	}

	// Check service status
	serviceInstalled := isServiceInstalled()
	serviceRunning := false
	if serviceInstalled {
		serviceRunning = isServiceRunning()
	}

	// Build status message
	statusLine := ""
	if isLoggedIn {
		statusLine = "👤 Logget ind som: " + userEmail + "\n"
	} else {
		statusLine = "⚠️ Ikke logget ind\n"
	}

	if serviceRunning {
		statusLine += "✅ Service: KØRER\n"
	} else if serviceInstalled {
		statusLine += "⏸️ Service: STOPPET\n"
	} else {
		statusLine += "❌ Service: Ikke installeret\n"
	}

	// Show main menu
	for {
		menuOptions := buildMenuOptions(isLoggedIn, serviceInstalled, serviceRunning)

		message := "Remote Desktop Agent v" + tray.VersionString + "\n\n" +
			statusLine + "\n" +
			"━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n" +
			menuOptions + "\n\n" +
			"Indtast nummer (eller Annuller for at afslutte):"

		// Use input dialog
		choice := showInputDialog("Remote Desktop Agent", message)

		if choice == "" || choice == "0" {
			return // Exit
		}

		action := handleMenuChoice(choice, isLoggedIn, serviceInstalled, serviceRunning)

		if action == "refresh" {
			// Refresh status
			isLoggedIn = auth.IsLoggedIn()
			if isLoggedIn {
				if creds, err := auth.GetCurrentUser(); err == nil {
					userEmail = creds.Email
				}
			} else {
				userEmail = ""
			}
			serviceInstalled = isServiceInstalled()
			if serviceInstalled {
				serviceRunning = isServiceRunning()
			} else {
				serviceRunning = false
			}

			// Update status line
			if isLoggedIn {
				statusLine = "👤 Logget ind som: " + userEmail + "\n"
			} else {
				statusLine = "⚠️ Ikke logget ind\n"
			}
			if serviceRunning {
				statusLine += "✅ Service: KØRER\n"
			} else if serviceInstalled {
				statusLine += "⏸️ Service: STOPPET\n"
			} else {
				statusLine += "❌ Service: Ikke installeret\n"
			}
		} else if action == "run_interactive" {
			// Run interactively
			if err := setupLogging(); err != nil {
				messageBox("Error", "Failed to setup logging: "+err.Error(), MB_OK|MB_ICONERROR)
				return
			}
			defer func() {
				if logFile != nil {
					logFile.Close()
				}
			}()
			log.Println("🔧 Kører i interaktiv tilstand")
			runInteractive()
			return
		} else if action == "exit" {
			return
		}
	}
}

// buildMenuOptions builds the menu based on current state
func buildMenuOptions(isLoggedIn, serviceInstalled, serviceRunning bool) string {
	options := ""
	num := 1

	if !isLoggedIn {
		options += fmt.Sprintf("[%d] 🔑 Log ind\n", num)
		num++
	}

	if isLoggedIn {
		if !serviceInstalled {
			options += fmt.Sprintf("[%d] 📦 Installer som Service (anbefalet)\n", num)
			num++
			options += fmt.Sprintf("[%d] ▶️ Kør én gang (kun denne session)\n", num)
			num++
		} else if !serviceRunning {
			options += fmt.Sprintf("[%d] ▶️ Start Service\n", num)
			num++
			options += fmt.Sprintf("[%d] 🗑️ Afinstaller Service\n", num)
			num++
		} else {
			options += fmt.Sprintf("[%d] ⏹️ Stop Service\n", num)
			num++
			options += fmt.Sprintf("[%d] 🗑️ Afinstaller Service\n", num)
			num++
		}

		options += fmt.Sprintf("[%d] 🔄 Tjek for opdateringer\n", num)
		num++
		options += fmt.Sprintf("[%d] 🚪 Log ud / Skift konto\n", num)
		num++
	}

	options += fmt.Sprintf("[0] ❌ Afslut")
	return options
}

// handleMenuChoice processes the user's menu selection
func handleMenuChoice(choice string, isLoggedIn, serviceInstalled, serviceRunning bool) string {
	// Map choice to action based on current state
	num := 1

	if !isLoggedIn {
		if choice == fmt.Sprintf("%d", num) {
			// Login
			doLogin()
			return "refresh"
		}
		num++
	}

	if isLoggedIn {
		if !serviceInstalled {
			if choice == fmt.Sprintf("%d", num) {
				// Install service
				doInstallService()
				return "refresh"
			}
			num++
			if choice == fmt.Sprintf("%d", num) {
				// Run once
				return "run_interactive"
			}
			num++
		} else if !serviceRunning {
			if choice == fmt.Sprintf("%d", num) {
				// Start service
				if err := startService(); err != nil {
					messageBox("Fejl", "Kunne ikke starte service: "+err.Error(), MB_OK|MB_ICONERROR)
				} else {
					messageBox("Succes", "✅ Service startet!", MB_OK|MB_ICONINFORMATION)
				}
				return "refresh"
			}
			num++
			if choice == fmt.Sprintf("%d", num) {
				// Uninstall
				doUninstallService()
				return "refresh"
			}
			num++
		} else {
			if choice == fmt.Sprintf("%d", num) {
				// Stop service
				if err := stopService(); err != nil {
					messageBox("Fejl", "Kunne ikke stoppe service: "+err.Error(), MB_OK|MB_ICONERROR)
				} else {
					messageBox("Succes", "✅ Service stoppet!", MB_OK|MB_ICONINFORMATION)
				}
				return "refresh"
			}
			num++
			if choice == fmt.Sprintf("%d", num) {
				// Uninstall
				doUninstallService()
				return "refresh"
			}
			num++
		}

		if choice == fmt.Sprintf("%d", num) {
			// Check for updates
			doCheckUpdates()
			return "refresh"
		}
		num++
		if choice == fmt.Sprintf("%d", num) {
			// Logout
			if err := auth.ClearCredentials(); err != nil {
				messageBox("Fejl", "Kunne ikke logge ud: "+err.Error(), MB_OK|MB_ICONERROR)
			} else {
				messageBox("Succes", "✅ Logget ud!", MB_OK|MB_ICONINFORMATION)
			}
			return "refresh"
		}
		num++
	}

	return "refresh"
}

// showInputDialog shows an input dialog and returns the user's input
func showInputDialog(title, message string) string {
	// Windows doesn't have a native input dialog, so we use a series of Yes/No dialogs
	// For simplicity, show numbered options and use Yes/No to navigate

	// Show the menu and ask for first digit
	result := messageBox(title, message+"\n\nKlik JA for mulighed 1, NEJ for andre muligheder", MB_YESNO|MB_ICONQUESTION)

	if result == IDYES {
		return "1"
	}

	// Ask for next option
	result = messageBox(title, "Mulighed 2?\n\nJA = Vælg mulighed 2\nNEJ = Flere muligheder", MB_YESNO|MB_ICONQUESTION)
	if result == IDYES {
		return "2"
	}

	result = messageBox(title, "Mulighed 3?\n\nJA = Vælg mulighed 3\nNEJ = Flere muligheder", MB_YESNO|MB_ICONQUESTION)
	if result == IDYES {
		return "3"
	}

	result = messageBox(title, "Mulighed 4?\n\nJA = Vælg mulighed 4\nNEJ = Flere muligheder", MB_YESNO|MB_ICONQUESTION)
	if result == IDYES {
		return "4"
	}

	result = messageBox(title, "Mulighed 5?\n\nJA = Vælg mulighed 5\nNEJ = Afslut", MB_YESNO|MB_ICONQUESTION)
	if result == IDYES {
		return "5"
	}

	return "0" // Exit
}

// doLogin handles the login process
func doLogin() {
	// Load config for Supabase credentials
	cfg, err := config.Load()
	if err != nil {
		messageBox("Fejl", "Kunne ikke indlæse konfiguration: "+err.Error(), MB_OK|MB_ICONERROR)
		return
	}

	// Get email
	email := showTextInputDialog("Log ind - Email", "Indtast din email adresse:")
	if email == "" {
		return
	}

	// Get password
	password := showTextInputDialog("Log ind - Adgangskode", "Indtast din adgangskode:")
	if password == "" {
		return
	}

	// Attempt login
	authConfig := auth.AuthConfig{
		SupabaseURL: cfg.SupabaseURL,
		AnonKey:     cfg.SupabaseAnonKey,
	}

	result, err := auth.Login(authConfig, email, password)
	if err != nil {
		messageBox("Login fejlede", "Fejl: "+err.Error(), MB_OK|MB_ICONERROR)
		return
	}

	if !result.Success {
		messageBox("Login fejlede", result.Message, MB_OK|MB_ICONWARNING)
		return
	}

	messageBox("Login lykkedes", "✅ Velkommen, "+result.Email+"!\n\nDu kan nu installere servicen.", MB_OK|MB_ICONINFORMATION)
}

// showTextInputDialog shows a simple text input dialog using PowerShell
func showTextInputDialog(title, prompt string) string {
	// Since Windows MessageBox doesn't support text input,
	// we'll use PowerShell's InputBox

	// Escape single quotes in prompt and title
	escapedPrompt := escapeForPowerShell(prompt)
	escapedTitle := escapeForPowerShell(title)

	psScript := fmt.Sprintf(`
Add-Type -AssemblyName Microsoft.VisualBasic
[Microsoft.VisualBasic.Interaction]::InputBox('%s', '%s', '')
`, escapedPrompt, escapedTitle)

	cmd := exec.Command("powershell", "-Command", psScript)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	// Trim whitespace and newlines
	result := string(output)
	// Remove trailing \r\n if present
	for len(result) > 0 && (result[len(result)-1] == '\n' || result[len(result)-1] == '\r') {
		result = result[:len(result)-1]
	}
	return result
}

// escapeForPowerShell escapes single quotes for PowerShell strings
func escapeForPowerShell(s string) string {
	result := ""
	for _, c := range s {
		if c == '\'' {
			result += "''"
		} else {
			result += string(c)
		}
	}
	return result
}

// doInstallService handles service installation
func doInstallService() {
	if !isAdmin() {
		messageBox("Administrator kræves",
			"Installation som service kræver Administrator rettigheder.\n\n"+
				"Klik OK for at genstarte som Administrator.", MB_OK|MB_ICONWARNING)
		runAsAdmin()
		return
	}

	if err := installService(); err != nil {
		messageBox("Fejl", "Kunne ikke installere service:\n\n"+err.Error(), MB_OK|MB_ICONERROR)
		return
	}

	if err := startService(); err != nil {
		messageBox("Advarsel", "Service installeret men kunne ikke startes:\n\n"+err.Error(), MB_OK|MB_ICONWARNING)
		return
	}

	messageBox("Succes",
		"✅ Service installeret og startet!\n\n"+
			"Remote Desktop Agent kører nu som Windows Service.\n\n"+
			"• Den starter automatisk når Windows starter\n"+
			"• Den kan optage login skærmen\n"+
			"• Kør denne exe igen for at administrere servicen", MB_OK|MB_ICONINFORMATION)
}

// doUninstallService handles service uninstallation
func doUninstallService() {
	if !isAdmin() {
		messageBox("Administrator kræves",
			"Klik OK for at genstarte som Administrator.", MB_OK|MB_ICONWARNING)
		runAsAdmin()
		return
	}

	// Stop first if running
	if isServiceRunning() {
		if err := stopService(); err != nil {
			messageBox("Fejl", "Kunne ikke stoppe service: "+err.Error(), MB_OK|MB_ICONERROR)
			return
		}
	}

	if err := uninstallService(); err != nil {
		messageBox("Fejl", "Kunne ikke afinstallere service: "+err.Error(), MB_OK|MB_ICONERROR)
		return
	}

	messageBox("Succes", "✅ Service afinstalleret.", MB_OK|MB_ICONINFORMATION)
}

// doCheckUpdates checks for updates from GitHub
func doCheckUpdates() {
	messageBox("Tjek for opdateringer",
		"For at opdatere agent:\n\n"+
			"1. Download seneste version fra:\n"+
			"   https://github.com/stangtennis/Remote/releases\n\n"+
			"2. Stop den nuværende service (hvis den kører)\n"+
			"3. Erstat denne exe med den nye\n"+
			"4. Start servicen igen\n\n"+
			"Nuværende version: v"+tray.VersionString, MB_OK|MB_ICONINFORMATION)
}

// isServiceInstalled checks if the service is installed
func isServiceInstalled() bool {
	m, err := mgr.Connect()
	if err != nil {
		return false
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return false
	}
	s.Close()
	return true
}

// isServiceRunning checks if the service is currently running
func isServiceRunning() bool {
	m, err := mgr.Connect()
	if err != nil {
		return false
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return false
	}
	defer s.Close()

	status, err := s.Query()
	if err != nil {
		return false
	}
	return status.State == svc.Running
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
		// Service exists - check if it's a different executable
		cfg, err := s.Config()
		s.Close()

		if err == nil && cfg.BinaryPathName != exePath {
			fmt.Println("⚠️  Service already exists with different executable:")
			fmt.Printf("   Current: %s\n", cfg.BinaryPathName)
			fmt.Printf("   New:     %s\n", exePath)
			fmt.Println()

			// Ask user if they want to upgrade
			if askYesNo("Do you want to upgrade to the new version?") {
				fmt.Println("🔄 Upgrading service...")
				if err := uninstallService(); err != nil {
					return fmt.Errorf("failed to uninstall old service: %w", err)
				}
				// Reconnect after uninstall
				m, err = mgr.Connect()
				if err != nil {
					return fmt.Errorf("failed to reconnect to service manager: %w", err)
				}
				defer m.Disconnect()
			} else {
				return fmt.Errorf("service upgrade cancelled")
			}
		} else {
			return fmt.Errorf("service %s already exists - use -uninstall first", serviceName)
		}
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

func runConsoleMode() {
	// Console mode - runs without system tray, keeps CMD window open with full logging
	log.Println("========================================")
	log.Println("🖥️  KONSOL TILSTAND - Fuld Logging Aktiveret")
	log.Println("========================================")
	log.Println("Tryk Ctrl+C for at stoppe agenten")
	log.Println("")

	// Setup firewall rules
	setupFirewallRules()

	// Check if already logged in
	if !auth.IsLoggedIn() {
		log.Println("❌ Ikke logget ind! Kør uden --console først for at logge ind via GUI")
		log.Println("   Eller kør: remote-agent.exe --silent for at logge ind via native dialog")
		return
	}

	// Load existing credentials
	creds, err := auth.GetCurrentUser()
	if err != nil {
		log.Printf("❌ Kunne ikke indlæse gemte credentials: %v", err)
		return
	}
	log.Printf("✅ Logget ind som: %s", creds.Email)
	currentUser = creds

	// Check current desktop
	desktopName, err := desktop.GetInputDesktop()
	if err != nil {
		log.Printf("⚠️  Kan ikke detektere skrivebord: %v", err)
	} else {
		log.Printf("🖥️  Nuværende skrivebord: %s", desktopName)
	}

	// Start agent
	if err := startAgent(); err != nil {
		log.Fatalf("❌ Kunne ikke starte agent: %v", err)
	}

	log.Println("")
	log.Println("========================================")
	log.Println("✅ Agent kører! Venter på forbindelser...")
	log.Println("   Tjek dashboard for at forbinde")
	log.Println("   Tryk Ctrl+C for at stoppe")
	log.Println("========================================")

	// Block forever (until Ctrl+C)
	select {}
}

func runInteractive() {
	// Setup firewall rules (requires admin, will skip if not admin)
	setupFirewallRules()

	// Check if already logged in
	if !auth.IsLoggedIn() {
		// Show login dialog
		log.Println("🔐 Login påkrævet...")

		// Load config for auth
		tempCfg, err := config.Load()
		if err != nil {
			log.Fatalf("Kunne ikke indlæse config: %v", err)
		}

		authConfig := auth.AuthConfig{
			SupabaseURL: tempCfg.SupabaseURL,
			AnonKey:     tempCfg.SupabaseAnonKey,
		}

		result := auth.ShowLoginDialog(authConfig)
		if result == nil || !result.Success {
			log.Println("❌ Login annulleret eller fejlet")
			return
		}

		log.Printf("✅ Logget ind som: %s", result.Email)
	} else {
		// Load existing credentials
		creds, err := auth.GetCurrentUser()
		if err != nil {
			log.Println("⚠️  Kunne ikke indlæse gemte credentials, log venligst ind igen")
			auth.ClearCredentials()
			runInteractive() // Retry with login
			return
		}
		log.Printf("✅ Allerede logget ind som: %s", creds.Email)
	}

	// Load current user credentials
	currentUser, _ = auth.GetCurrentUser()

	// Check current desktop (non-fatal if fails)
	desktopName, err := desktop.GetInputDesktop()
	if err != nil {
		log.Printf("⚠️  Kan ikke detektere skrivebord: %v", err)
		log.Println("   (Dette er normalt når der køres som service)")
	} else {
		log.Printf("🖥️  Nuværende skrivebord: %s", desktopName)
		if desktop.IsOnLoginScreen() {
			log.Println("⚠️  Kører på login skærm - begrænset funktionalitet")
		}
	}

	if err := startAgent(); err != nil {
		log.Fatalf("Kunne ikke starte agent: %v", err)
	}

	// Desktop monitoring is now handled by WebRTC manager
	// It will automatically reinitialize screen capture on desktop switch

	log.Println("🔔 Starter system tray...")

	// Run system tray (blocks until user exits from tray menu)
	trayApp := tray.New(dev, func() {
		log.Println("🛑 Lukker ned fra system tray...")
		stopAgent()
		log.Println("👋 Farvel!")
	})

	trayApp.Run()
}

func runService() {
	// Windows Service mode
	log.Println("Starter som Windows Service...")

	// Setup firewall rules (service runs as SYSTEM, has admin rights)
	setupFirewallRules()

	serviceName := "RemoteDesktopAgent"

	err := svc.Run(serviceName, &windowsService{})
	if err != nil {
		log.Fatalf("Service fejlede: %v", err)
	}
}

type windowsService struct{}

func (s *windowsService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}

	if err := startAgent(); err != nil {
		log.Printf("Service start fejlede: %v", err)
		return true, 1
	}

	// Desktop monitoring is now handled by WebRTC manager
	// It will automatically detect Session 0 and reinitialize screen capture on desktop switch
	if _, err := desktop.GetInputDesktop(); err == nil {
		log.Println("✅ Skrivebords adgang tilgængelig")
	} else {
		log.Println("⚠️  Ingen skrivebords adgang (Session 0 / før-login)")
		log.Println("   Service kører - WebRTC manager håndterer skrivebords detektion")
	}

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	log.Println("Service kører")

loop:
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
				log.Println("Service forespørgt - svarer")
			case svc.Stop:
				log.Println("Service modtog STOP kommando")
				break loop
			case svc.Shutdown:
				log.Println("Service modtog SHUTDOWN kommando")
				break loop
			default:
				log.Printf("Uventet kontrol forespørgsel #%d", c)
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

	// Log important paths for debugging
	log.Printf("📁 Credentials path: %s", auth.GetCredentialsPath())
	deviceID, _ := device.GetOrCreateDeviceID()
	log.Printf("🔑 Device ID: %s", deviceID)

	authConfig := auth.AuthConfig{
		SupabaseURL: cfg.SupabaseURL,
		AnonKey:     cfg.SupabaseAnonKey,
	}

	// Check and refresh credentials if needed (important for service mode)
	if !auth.IsLoggedIn() {
		log.Println("⚠️  No valid credentials found")
		// Try to refresh token
		creds, err := auth.LoadCredentials()
		if err == nil && creds.RefreshToken != "" {
			log.Println("🔄 Attempting to refresh token...")
			result, err := auth.RefreshToken(authConfig, creds.RefreshToken)
			if err == nil && result.Success {
				log.Printf("✅ Token refreshed for: %s", result.Email)
				currentUser, _ = auth.GetCurrentUser()
			} else {
				log.Println("❌ Token refresh failed - please run agent interactively to login")
				return fmt.Errorf("authentication required - run agent interactively first")
			}
		} else {
			return fmt.Errorf("no credentials found - run agent interactively to login first")
		}
	} else {
		currentUser, _ = auth.GetCurrentUser()
		log.Printf("✅ Using saved credentials for: %s", currentUser.Email)
	}

	// Create TokenProvider for authenticated API calls
	creds, err := auth.LoadCredentials()
	if err != nil {
		return fmt.Errorf("failed to load credentials for token provider: %w", err)
	}
	tokenProvider := auth.NewTokenProvider(authConfig, creds)
	log.Println("🔐 TokenProvider oprettet (authenticated API kald)")

	// Initialize device
	dev, err = device.New(cfg, tokenProvider)
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
	rtc, err = webrtc.New(cfg, dev, tokenProvider)
	if err != nil {
		return fmt.Errorf("failed to initialize WebRTC: %w", err)
	}

	// Start listening for sessions
	log.Println("👂 Listening for incoming connections...")
	go rtc.ListenForSessions()

	return nil
}

// Program installation paths
const (
	programInstallDir  = `C:\Program Files\RemoteDesktopAgent`
	programExeName     = "remote-agent.exe"
	registryRunKey     = `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`
	registryValueName  = "RemoteDesktopAgent"
)

// isProgramInstalled checks if the agent is installed as a program
func isProgramInstalled() bool {
	exePath := filepath.Join(programInstallDir, programExeName)
	_, err := os.Stat(exePath)
	return err == nil
}

// isProgramAutostart checks if autostart registry entry exists
func isProgramAutostart() bool {
	key, err := openRegistryKey(registryRunKey)
	if err != nil {
		return false
	}
	defer key.Close()

	_, _, err = key.GetStringValue(registryValueName)
	return err == nil
}

// stopRunningAgent kills any running remote-agent.exe processes (except this one)
func stopRunningAgent() {
	myPID := os.Getpid()
	log.Printf("🛑 Stopper kørende agent-processer (vores PID: %d)...", myPID)

	// Use tasklist to find all remote-agent.exe PIDs, then kill each except ours
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("IMAGENAME eq %s", programExeName), "/FO", "CSV", "/NH")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("   tasklist fejl: %v", err)
		return
	}

	killed := 0
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "No tasks") || strings.Contains(line, "INFO:") {
			continue
		}
		// CSV format: "remote-agent.exe","1234","Console","1","12,345 K"
		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}
		pidStr := strings.Trim(parts[1], "\" ")
		pid := 0
		fmt.Sscanf(pidStr, "%d", &pid)
		if pid == 0 || pid == myPID {
			continue
		}
		log.Printf("   Dræber PID %d...", pid)
		killCmd := exec.Command("taskkill", "/F", "/PID", fmt.Sprintf("%d", pid))
		killCmd.Run()
		killed++
	}

	if killed > 0 {
		log.Printf("   ✅ %d agent-proces(ser) stoppet", killed)
		// Give processes time to fully exit and release file handles
		time.Sleep(500 * time.Millisecond)
	} else {
		log.Printf("   Ingen andre agent-processer fundet")
	}
}

// installAsProgram copies the exe to Program Files and sets up autostart
func installAsProgram() error {
	if !isAdmin() {
		return fmt.Errorf("administrator rettigheder kræves")
	}

	// Stop any running agent first (so we can overwrite the exe)
	stopRunningAgent()

	// Create install directory
	if err := os.MkdirAll(programInstallDir, 0755); err != nil {
		return fmt.Errorf("kunne ikke oprette mappe: %w", err)
	}

	// Get current exe path
	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("kunne ikke finde exe: %w", err)
	}

	targetExe := filepath.Join(programInstallDir, programExeName)

	// Copy exe to Program Files
	srcFile, err := os.Open(currentExe)
	if err != nil {
		return fmt.Errorf("kunne ikke åbne kilde: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(targetExe)
	if err != nil {
		return fmt.Errorf("kunne ikke oprette destination: %w", err)
	}

	if _, err := dstFile.ReadFrom(srcFile); err != nil {
		dstFile.Close()
		return fmt.Errorf("kunne ikke kopiere: %w", err)
	}
	dstFile.Close()

	// Copy .env file if it exists next to current exe
	envSrc := filepath.Join(filepath.Dir(currentExe), ".env")
	if _, err := os.Stat(envSrc); err == nil {
		envDst := filepath.Join(programInstallDir, ".env")
		copyFile(envSrc, envDst)
	}

	// Add autostart registry entry (runs in background with tray icon)
	if err := setAutostartRegistry(targetExe); err != nil {
		return fmt.Errorf("kunne ikke sætte autostart: %w", err)
	}

	// Create Start Menu shortcut
	if smErr := createAgentStartMenuShortcut(targetExe); smErr != nil {
		log.Printf("⚠️ Start Menu genvej fejlede: %v", smErr)
	}

	// Start the agent now from Program Files
	log.Printf("🚀 Starter agent fra: %s", targetExe)
	cmd := exec.Command(targetExe)
	cmd.Dir = programInstallDir
	if err := cmd.Start(); err != nil {
		log.Printf("⚠️ Kunne ikke starte agent: %v", err)
		// Not fatal - it will start on next login
	}

	log.Printf("✅ Program installeret: %s", targetExe)
	return nil
}

// uninstallProgram removes the program installation and autostart
func uninstallProgram() error {
	if !isAdmin() {
		return fmt.Errorf("administrator rettigheder kræves")
	}

	// Stop any running agent first (so we can delete the exe)
	stopRunningAgent()

	// Remove autostart registry entry
	removeAutostartRegistry()

	// Remove shortcuts
	removeAgentShortcuts()

	// Remove install directory
	if err := os.RemoveAll(programInstallDir); err != nil {
		return fmt.Errorf("kunne ikke fjerne mappe: %w", err)
	}

	log.Println("✅ Program afinstalleret")
	return nil
}

// createAgentShortcut creates a Windows .lnk shortcut file using PowerShell
func createAgentShortcut(shortcutPath, targetExe, description string) error {
	psScript := fmt.Sprintf(`
$ws = New-Object -ComObject WScript.Shell
$s = $ws.CreateShortcut('%s')
$s.TargetPath = '%s'
$s.WorkingDirectory = '%s'
$s.Description = '%s'
$s.Save()
`, shortcutPath, targetExe, filepath.Dir(targetExe), description)

	cmd := exec.Command("powershell", "-NoProfile", "-Command", psScript)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("powershell shortcut failed: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// createAgentStartMenuShortcut creates a Start Menu shortcut for the agent
func createAgentStartMenuShortcut(targetExe string) error {
	startMenuDir := filepath.Join(os.Getenv("ProgramData"), "Microsoft", "Windows", "Start Menu", "Programs")
	if _, err := os.Stat(startMenuDir); os.IsNotExist(err) {
		startMenuDir = filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs")
	}
	shortcutPath := filepath.Join(startMenuDir, "Remote Desktop Agent.lnk")
	log.Printf("📌 Opretter Start Menu genvej: %s", shortcutPath)
	return createAgentShortcut(shortcutPath, targetExe, "Remote Desktop Agent")
}

// removeAgentShortcuts removes Start Menu shortcuts for the agent
func removeAgentShortcuts() {
	startMenuAll := filepath.Join(os.Getenv("ProgramData"), "Microsoft", "Windows", "Start Menu", "Programs", "Remote Desktop Agent.lnk")
	if err := os.Remove(startMenuAll); err == nil {
		log.Printf("🗑️ Start Menu genvej fjernet (alle brugere)")
	}
	startMenuUser := filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs", "Remote Desktop Agent.lnk")
	if err := os.Remove(startMenuUser); err == nil {
		log.Printf("🗑️ Start Menu genvej fjernet (bruger)")
	}
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = dstFile.ReadFrom(srcFile)
	return err
}

// setAutostartRegistry adds the agent to Windows autostart via registry
func setAutostartRegistry(exePath string) error {
	// Use HKCU for user autostart (HKLM requires SYSTEM and only works for services)
	cmd := exec.Command("reg", "add",
		`HKCU\`+registryRunKey,
		"/v", registryValueName,
		"/t", "REG_SZ",
		"/d", `"`+exePath+`"`,
		"/f")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("reg add failed: %w", err)
	}
	return nil
}

// removeAutostartRegistry removes the agent from Windows autostart
func removeAutostartRegistry() {
	// Remove from HKCU (user autostart)
	cmd := exec.Command("reg", "delete",
		`HKCU\`+registryRunKey,
		"/v", registryValueName,
		"/f")
	cmd.Run() // Ignore errors
}

// openRegistryKey opens a registry key for reading
func openRegistryKey(keyPath string) (*registryKey, error) {
	// Check HKCU for user autostart (not HKLM which is for system/services)
	cmd := exec.Command("reg", "query", `HKCU\`+keyPath, "/v", registryValueName)
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return &registryKey{}, nil
}

type registryKey struct{}

func (k *registryKey) Close() error { return nil }
func (k *registryKey) GetStringValue(name string) (string, uint32, error) {
	return "", 0, nil
}

// isRunningFromProgramFiles checks if the current exe is in the Program Files install dir
func isRunningFromProgramFiles() bool {
	exePath, err := os.Executable()
	if err != nil {
		return false
	}
	exeDir := filepath.Dir(exePath)
	return strings.EqualFold(exeDir, programInstallDir)
}

func stopAgent() {
	if dev != nil {
		dev.SetOffline()
	}
	time.Sleep(500 * time.Millisecond)
}

// runUpdateMode runs when started with --update-from flag
// This replaces the old exe and restarts normally
func runUpdateMode(oldExePath string) {
	// Simple logging to file
	logFile := oldExePath + ".update.log"
	f, _ := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if f != nil {
		defer f.Close()
	}

	logMsg := func(msg string) {
		line := fmt.Sprintf("[%s] %s\n", time.Now().Format("15:04:05"), msg)
		if f != nil {
			f.WriteString(line)
		}
		fmt.Print(line)
	}

	logMsg("Update mode started")
	logMsg(fmt.Sprintf("Old exe: %s", oldExePath))
	
	// Check if old exe is running from Program Files install directory
	// If yes, also update the installed exe so updates persist across reboots
	installedExe := filepath.Join(programInstallDir, programExeName)
	updateInstalledCopy := false
	if isProgramInstalled() && !strings.EqualFold(oldExePath, installedExe) {
		logMsg(fmt.Sprintf("Detected Program Files installation at: %s", installedExe))
		logMsg("Will also update installed copy to persist across reboots")
		updateInstalledCopy = true
	}

	currentExe, err := os.Executable()
	if err != nil {
		logMsg(fmt.Sprintf("ERROR: Failed to get current exe path: %v", err))
		return
	}
	logMsg(fmt.Sprintf("New exe: %s", currentExe))

	// Wait for old exe to exit (max 10 seconds)
	logMsg("Waiting for old exe to exit...")
	for i := 0; i < 100; i++ {
		file, err := os.OpenFile(oldExePath, os.O_RDWR, 0)
		if err == nil {
			file.Close()
			logMsg("Old exe is unlocked")
			break
		}
		if os.IsNotExist(err) {
			logMsg("Old exe already deleted")
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Delete old exe
	logMsg("Deleting old exe...")
	if err := os.Remove(oldExePath); err != nil {
		if !os.IsNotExist(err) {
			logMsg(fmt.Sprintf("WARNING: Failed to delete old exe: %v", err))
		}
	} else {
		logMsg("Old exe deleted")
	}

	// Copy current exe to old exe location
	logMsg(fmt.Sprintf("Copying %s to %s", currentExe, oldExePath))

	srcFile, err := os.Open(currentExe)
	if err != nil {
		logMsg(fmt.Sprintf("ERROR: Failed to open source: %v", err))
		return
	}

	dstFile, err := os.Create(oldExePath)
	if err != nil {
		srcFile.Close()
		logMsg(fmt.Sprintf("ERROR: Failed to create destination: %v", err))
		return
	}

	_, err = dstFile.ReadFrom(srcFile)
	srcFile.Close()
	dstFile.Close()

	if err != nil {
		logMsg(fmt.Sprintf("ERROR: Failed to copy: %v", err))
		return
	}
	logMsg("Copy complete")

	// If agent is installed in Program Files, also update that copy
	if updateInstalledCopy {
		logMsg("Updating installed copy in Program Files...")
		
		// Stop any running instance from Program Files first
		stopRunningAgent()
		time.Sleep(500 * time.Millisecond)
		
		// Copy new exe to Program Files
		srcFile2, err := os.Open(currentExe)
		if err != nil {
			logMsg(fmt.Sprintf("WARNING: Failed to open source for installed copy: %v", err))
		} else {
			dstFile2, err := os.Create(installedExe)
			if err != nil {
				srcFile2.Close()
				logMsg(fmt.Sprintf("WARNING: Failed to create installed copy: %v", err))
			} else {
				_, err = dstFile2.ReadFrom(srcFile2)
				srcFile2.Close()
				dstFile2.Close()
				
				if err != nil {
					logMsg(fmt.Sprintf("WARNING: Failed to copy to installed location: %v", err))
				} else {
					logMsg("✅ Installed copy updated successfully")
				}
			}
		}
	}

	// Start the copied exe (now at original location)
	logMsg("Starting agent from original location...")
	cmd := exec.Command(oldExePath)
	if err := cmd.Start(); err != nil {
		logMsg(fmt.Sprintf("ERROR: Failed to start: %v", err))
		return
	}

	logMsg("Update complete!")
}
