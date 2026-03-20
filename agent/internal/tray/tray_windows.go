//go:build windows

package tray

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/getlantern/systray"
	"github.com/stangtennis/remote-agent/internal/device"
	"github.com/stangtennis/remote-agent/internal/updater"
	"github.com/stangtennis/remote-agent/internal/version"
)

//go:embed agent.ico
var agentIconICO []byte

// Version aliases for backwards compatibility (ldflags still inject here)
var (
	Version       = "dev"
	BuildDate     = "unknown"
	VersionString = ""
)

func init() {
	// Sync: if ldflags injected here, propagate to version package
	if Version != "dev" {
		version.Version = Version
		version.BuildDate = BuildDate
	} else if version.Version != "dev" {
		// version package was set directly
		Version = version.Version
		BuildDate = version.BuildDate
	}
	if VersionString == "" {
		VersionString = Version + " (built " + BuildDate + ")"
	}
}

type TrayApp struct {
	device  *device.Device
	onExit  func()
	mStatus *systray.MenuItem
}

func New(dev *device.Device, onExit func()) *TrayApp {
	return &TrayApp{
		device: dev,
		onExit: onExit,
	}
}

func (t *TrayApp) Run() {
	systray.Run(t.onReady, t.onExit)
}

func (t *TrayApp) onReady() {
	// Set up the system tray icon and menu
	systray.SetIcon(getIcon())
	systray.SetTitle("Remote")
	systray.SetTooltip(fmt.Sprintf("Remote Desktop Agent %s\nEnhed: %s", Version, t.device.Name))

	// Add menu items
	mDeviceName := systray.AddMenuItem(fmt.Sprintf("Enhed: %s", t.device.Name), "")
	mDeviceName.Disable()

	t.mStatus = systray.AddMenuItem("Status: Online", "Aktuel forbindelsesstatus")
	t.mStatus.Disable()

	systray.AddSeparator()

	mConsole := systray.AddMenuItem("Vis konsol vindue", "Åbn live konsol output")
	mLogs := systray.AddMenuItem("Vis log fil", "Åbn log fil i editor")
	mVersion := systray.AddMenuItem(fmt.Sprintf("Version %s", Version), "Agent version")
	mVersion.Disable()

	systray.AddSeparator()

	mUpdate := systray.AddMenuItem("🔄 Tjek opdatering", "Tjek for nye versioner")
	mLogout := systray.AddMenuItem("Skift konto / Log ud", "Log ud og skift til anden konto")
	mQuit := systray.AddMenuItem("Afslut", "Luk agenten")

	// Handle menu clicks
	go func() {
		for {
			select {
			case <-mConsole.ClickedCh:
				openConsole()
			case <-mLogs.ClickedCh:
				openLogFile()
			case <-mUpdate.ClickedCh:
				log.Println("🔄 Update check requested from system tray")
				checkForUpdates()
			case <-mLogout.ClickedCh:
				log.Println("🔄 Logout requested from system tray")
				clearCredentialsAndRestart()
			case <-mQuit.ClickedCh:
				log.Println("🛑 Exit requested from system tray")
				systray.Quit()
				return
			}
		}
	}()
}

func openConsole() {
	// Get log path from APPDATA (where logging package stores logs)
	appData := os.Getenv("APPDATA")
	if appData == "" {
		log.Printf("APPDATA environment variable not set")
		return
	}
	logPath := filepath.Join(appData, "RemoteAgent", "logs", "agent.log")

	// Check if log file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		log.Printf("Log fil findes ikke: %s", logPath)
		return
	}

	// Open a PowerShell window that tails the log file
	log.Println("🪟 Åbner konsol vindue med live logs...")

	// Use cmd to start PowerShell with proper escaping
	// This ensures the window opens correctly
	psCmd := fmt.Sprintf(`Get-Content -Path "%s" -Wait -Tail 50`, logPath)
	cmd := exec.Command("cmd", "/c", "start", "powershell", "-NoExit", "-Command", psCmd)

	if err := cmd.Start(); err != nil {
		log.Printf("Kunne ikke åbne konsol: %v", err)
	} else {
		log.Printf("✅ Konsol vindue åbnet for: %s", logPath)
	}
}

func openLogFile() {
	// Get log path from APPDATA (where logging package stores logs)
	appData := os.Getenv("APPDATA")
	if appData == "" {
		log.Printf("APPDATA environment variable not set")
		return
	}
	logPath := filepath.Join(appData, "RemoteAgent", "logs", "agent.log")

	// Check if log file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		log.Printf("Log fil findes ikke: %s", logPath)
		return
	}

	// Open log file with default text editor
	log.Printf("📄 Opening log file: %s", logPath)

	// Use notepad as it's always available on Windows
	cmd := exec.Command("notepad", logPath)
	if err := cmd.Start(); err != nil {
		log.Printf("Kunne ikke åbne log fil: %v", err)
	}
}

// clearCredentialsAndRestart clears saved credentials and restarts the agent
func clearCredentialsAndRestart() {
	// Get credentials path
	programData := os.Getenv("ProgramData")
	var credPath string
	if programData != "" {
		credPath = filepath.Join(programData, "RemoteDesktopAgent", ".credentials")
	} else {
		exePath, _ := os.Executable()
		credPath = filepath.Join(filepath.Dir(exePath), ".credentials")
	}

	// Remove credentials file
	if err := os.Remove(credPath); err != nil && !os.IsNotExist(err) {
		log.Printf("⚠️ Kunne ikke fjerne login oplysninger: %v", err)
	} else {
		log.Println("✅ Login oplysninger ryddet")
	}

	// Restart the agent
	exePath, err := os.Executable()
	if err != nil {
		log.Printf("❌ Kunne ikke finde exe sti: %v", err)
		return
	}

	log.Println("🔄 Genstarter agent for nyt login...")

	// Start new instance
	cmd := exec.Command(exePath)
	cmd.Start()

	// Exit current instance
	systray.Quit()
}

// UpdateStatus updates the tray tooltip and status menu item with connection info
func (t *TrayApp) UpdateStatus(status string) {
	if t.mStatus != nil {
		t.mStatus.SetTitle(status)
	}
	systray.SetTooltip(fmt.Sprintf("Remote Desktop Agent %s\nEnhed: %s\n%s", Version, t.device.Name, status))
}

// getIcon returns the embedded ICO file for the system tray
func getIcon() []byte {
	return agentIconICO
}

// checkForUpdates tjekker for tilgængelige opdateringer og viser resultat
func checkForUpdates() {
	log.Println("🔍 Tjekker for opdateringer...")

	u, err := updater.NewUpdater(Version)
	if err != nil {
		log.Printf("❌ Kunne ikke initialisere opdatering: %v", err)
		showMessageBox("Fejl", fmt.Sprintf("Kunne ikke tjekke for opdateringer:\n%v", err), mbIconError)
		return
	}

	// Fetch full version info to show both agent and controller versions
	versionInfo, err := u.FetchVersionInfo()
	if err != nil {
		log.Printf("❌ Kunne ikke hente versions-info: %v", err)
		showMessageBox("Fejl", fmt.Sprintf("Kunne ikke hente versions-info:\n%v", err), mbIconError)
		return
	}

	// Compare versions
	currentAgent, _ := updater.ParseVersion(Version)
	remoteAgent, _ := updater.ParseVersion(versionInfo.AgentVersion)

	agentStatus := "✅ Opdateret"
	if remoteAgent.IsNewerThan(currentAgent) {
		agentStatus = "🆕 NY VERSION"
	}

	// Build version display message
	msg := fmt.Sprintf(
		"═══ Versioner ═══\n\n"+
			"Agent (denne):\n"+
			"  Installeret:  %s\n"+
			"  Tilgængelig:  %s  %s\n\n"+
			"Controller:\n"+
			"  Tilgængelig:  %s\n\n"+
			"═════════════════",
		Version,
		versionInfo.AgentVersion, agentStatus,
		versionInfo.ControllerVersion,
	)

	if remoteAgent.IsNewerThan(currentAgent) {
		// Update available - ask to download
		msg += "\n\nVil du downloade og installere den nye version?"
		result := showYesNoBox("Opdatering tilgængelig", msg)
		if result {
			log.Printf("📥 Bruger valgte at opdatere til %s", versionInfo.AgentVersion)
			doDownloadAndInstall(u)
		}
	} else {
		showMessageBox("Opdateringer", msg, mbIconInfo)
	}
}

// doDownloadAndInstall downloads and installs the update
func doDownloadAndInstall(u *updater.Updater) {
	log.Println("🔍 Tjekker for opdatering...")
	if err := u.CheckForUpdate(); err != nil {
		log.Printf("❌ Tjek fejlede: %v", err)
		showMessageBox("Fejl", fmt.Sprintf("Opdateringstjek fejlede:\n%v", err), mbIconError)
		return
	}

	info := u.GetAvailableUpdate()
	if info == nil {
		showMessageBox("Opdateringer", "Ingen opdatering tilgængelig.", mbIconInfo)
		return
	}

	log.Printf("📥 Downloader %s...", info.TagName)
	if err := u.DownloadUpdate(); err != nil {
		log.Printf("❌ Download fejlede: %v", err)
		showMessageBox("Fejl", fmt.Sprintf("Download fejlede:\n%v", err), mbIconError)
		return
	}

	log.Println("✅ Download færdig, installerer...")
	if err := u.InstallUpdate(); err != nil {
		log.Printf("❌ Installation fejlede: %v", err)
		showMessageBox("Fejl", fmt.Sprintf("Installation fejlede:\n%v", err), mbIconError)
		return
	}

	log.Println("🚀 Opdatering installeret, genstarter...")
	systray.Quit()
}

// Windows MessageBox constants and helpers for tray (no Fyne available)
const (
	mbOK        = 0x00000000
	mbYesNo     = 0x00000004
	mbIconInfo  = 0x00000040
	mbIconError = 0x00000010
	mbIDYes     = 6
)

var (
	trayUser32      = syscall.NewLazyDLL("user32.dll")
	trayMessageBoxW = trayUser32.NewProc("MessageBoxW")
)

func showMessageBox(title, text string, flags uintptr) {
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	textPtr, _ := syscall.UTF16PtrFromString(text)
	trayMessageBoxW.Call(0, uintptr(unsafe.Pointer(textPtr)), uintptr(unsafe.Pointer(titlePtr)), flags)
}

func showYesNoBox(title, text string) bool {
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	textPtr, _ := syscall.UTF16PtrFromString(text)
	ret, _, _ := trayMessageBoxW.Call(0, uintptr(unsafe.Pointer(textPtr)), uintptr(unsafe.Pointer(titlePtr)), uintptr(mbYesNo|mbIconInfo))
	return ret == mbIDYes
}
