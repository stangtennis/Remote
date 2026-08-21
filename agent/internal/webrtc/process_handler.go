package webrtc

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	pionwebrtc "github.com/pion/webrtc/v3"
	"github.com/stangtennis/remote-agent/internal/process"
	"github.com/stangtennis/remote-agent/internal/sysinfo"
)

// setupProcessChannelHandlers sets up the process management data channel
func (m *Manager) setupProcessChannelHandlers(dc *pionwebrtc.DataChannel) {
	dc.OnOpen(func() {
		log.Println("⚙️ Process channel open")
	})

	dc.OnMessage(func(msg pionwebrtc.DataChannelMessage) {
		if m.supportIsActive() && !m.supportAllows("process") {
			sendProcessError(dc, "process scope denied")
			return
		}
		var message map[string]interface{}
		if err := json.Unmarshal(msg.Data, &message); err != nil {
			log.Printf("⚠️ Invalid process message: %v", err)
			return
		}

		op, _ := message["op"].(string)
		actionType := "PROCESS_" + strings.ToUpper(op)
		var opErr error
		if m.supportIsActive() {
			if err := m.recordSupportAction(actionType, "started", "Started process operation", op, map[string]interface{}{}); err != nil {
				sendProcessError(dc, err.Error())
				return
			}
		}
		switch op {
		case "ps":
			opErr = m.handleProcessList(dc)
		case "kill":
			if m.supportIsActive() && !m.supportAllows("admin") {
				sendProcessError(dc, "admin scope required to kill processes")
				return
			}
			pidVal, _ := message["pid"].(float64) // JSON numbers are float64
			opErr = m.handleProcessKill(dc, int(pidVal))
		case "sysinfo":
			opErr = m.handleSysinfo(dc)
		default:
			opErr = fmt.Errorf("unknown op: %s", op)
			sendProcessError(dc, opErr.Error())
		}
		if m.supportIsActive() {
			status := "succeeded"
			if opErr != nil {
				status = "failed"
			}
			_ = m.recordSupportAction(actionType, status, "Finished process operation", op, map[string]interface{}{})
		}
	})
}

func (m *Manager) handleProcessList(dc *pionwebrtc.DataChannel) error {
	procs, err := process.List()
	if err != nil {
		sendProcessError(dc, err.Error())
		return err
	}

	resp := map[string]interface{}{
		"op":        "ps_result",
		"processes": procs,
		"count":     len(procs),
	}

	data, err := json.Marshal(resp)
	if err != nil {
		sendProcessError(dc, err.Error())
		return err
	}
	dc.Send(data)
	return nil
}

func (m *Manager) handleProcessKill(dc *pionwebrtc.DataChannel, pid int) error {
	if pid <= 0 {
		sendProcessError(dc, "invalid PID")
		return fmt.Errorf("invalid PID")
	}

	// Refuse to kill the agent itself to prevent a connected client from
	// remotely disabling the agent.
	if pid == os.Getpid() {
		sendProcessError(dc, "refusing to kill agent process")
		return fmt.Errorf("refusing to kill agent process")
	}

	log.Printf("⚙️ Killing process PID %d", pid)
	err := process.Kill(pid)

	resp := map[string]interface{}{
		"op":  "kill_result",
		"pid": pid,
		"ok":  err == nil,
	}
	if err != nil {
		resp["error"] = err.Error()
	}

	data, _ := json.Marshal(resp)
	dc.Send(data)
	return err
}

func (m *Manager) handleSysinfo(dc *pionwebrtc.DataChannel) error {
	info, err := sysinfo.Collect()
	if err != nil {
		sendProcessError(dc, err.Error())
		return err
	}

	// Inline assemble so we can include the literal op tag.
	resp := map[string]interface{}{
		"op":             "sysinfo_result",
		"os":             info.OS,
		"hostname":       info.Hostname,
		"cpu":            info.CPU,
		"cpu_cores":      info.CPUCores,
		"ram_total_gb":   info.RAMTotalGB,
		"ram_free_gb":    info.RAMFreeGB,
		"disks":          info.Disks,
		"uptime_sec":     info.UptimeSec,
		"installed_apps": info.InstalledApps,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		sendProcessError(dc, err.Error())
		return err
	}
	dc.Send(data)
	return nil
}

func sendProcessError(dc *pionwebrtc.DataChannel, msg string) {
	resp := map[string]interface{}{
		"op":    "error",
		"error": msg,
	}
	data, _ := json.Marshal(resp)
	dc.Send(data)
	log.Printf("⚠️ Process error: %s", msg)
}

// Helper to convert PID to string for logging
func pidStr(pid int) string {
	return strconv.Itoa(pid)
}
