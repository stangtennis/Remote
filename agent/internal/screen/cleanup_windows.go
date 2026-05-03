//go:build windows

package screen

import (
	"log"
	"os"
	"strings"

	"github.com/shirou/gopsutil/v4/process"
)

// CleanupOrphanedHelpers terminerer zombie capture-helper / clipboard-helper-
// processer fra forrige service-instanser.
//
// Når servicen kill'es (fx ved update, crash, eller manual restart) bliver
// helper-processer detached fordi vi spawn'er dem via CreateProcessAsUser.
// De forbliver i user-session indefinitely indtil reboot. Hvis brugeren
// connecter+disconnecter ofte, akkumulerer de — set 9 zombier på én VM.
//
// Kald denne ved service-startup, før vi spawner nye helpers. Sikker:
// dræber kun processer der har vores --capture-helper / --clipboard-helper
// flag, og ALDRIG vores eget PID (servicen er ny instans).
func CleanupOrphanedHelpers() {
	myPid := int32(os.Getpid())
	procs, err := process.Processes()
	if err != nil {
		log.Printf("⚠️ CleanupOrphanedHelpers: kunne ikke liste processer: %v", err)
		return
	}

	killed := 0
	for _, p := range procs {
		if p.Pid == myPid {
			continue
		}
		name, err := p.Name()
		if err != nil {
			continue
		}
		if !strings.EqualFold(name, "remote-agent-console.exe") &&
			!strings.EqualFold(name, "remote-agent.exe") {
			continue
		}
		// Kun helpers — ikke andre instanser af agent (fx tray)
		cmdline, err := p.Cmdline()
		if err != nil {
			continue
		}
		isHelper := strings.Contains(cmdline, "--capture-helper") ||
			strings.Contains(cmdline, "--clipboard-helper")
		if !isHelper {
			continue
		}
		if err := p.Kill(); err == nil {
			killed++
			log.Printf("🧹 Killed orphan helper PID=%d cmd=%s", p.Pid, cmdline)
		}
	}
	if killed > 0 {
		log.Printf("✅ CleanupOrphanedHelpers: %d zombie helper(s) terminated", killed)
	}
}
