package webrtc

import (
	"encoding/json"
	"log"
	"strconv"

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
		var message map[string]interface{}
		if err := json.Unmarshal(msg.Data, &message); err != nil {
			log.Printf("⚠️ Invalid process message: %v", err)
			return
		}

		op, _ := message["op"].(string)
		switch op {
		case "ps":
			m.handleProcessList(dc)
		case "kill":
			pidVal, _ := message["pid"].(float64) // JSON numbers are float64
			m.handleProcessKill(dc, int(pidVal))
		case "sysinfo":
			m.handleSysinfo(dc)
		default:
			sendProcessError(dc, "unknown op: "+op)
		}
	})
}

func (m *Manager) handleProcessList(dc *pionwebrtc.DataChannel) {
	procs, err := process.List()
	if err != nil {
		sendProcessError(dc, err.Error())
		return
	}

	resp := map[string]interface{}{
		"op":        "ps_result",
		"processes": procs,
		"count":     len(procs),
	}

	data, err := json.Marshal(resp)
	if err != nil {
		sendProcessError(dc, err.Error())
		return
	}
	dc.Send(data)
}

func (m *Manager) handleProcessKill(dc *pionwebrtc.DataChannel, pid int) {
	if pid <= 0 {
		sendProcessError(dc, "invalid PID")
		return
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
}

func (m *Manager) handleSysinfo(dc *pionwebrtc.DataChannel) {
	info, err := sysinfo.Collect()
	if err != nil {
		sendProcessError(dc, err.Error())
		return
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
		return
	}
	dc.Send(data)
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
