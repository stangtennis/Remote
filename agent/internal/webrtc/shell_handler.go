package webrtc

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	pionwebrtc "github.com/pion/webrtc/v3"
	"github.com/stangtennis/remote-agent/internal/device"
	"github.com/stangtennis/remote-agent/internal/shell"
)

// inflightExec tracks one running command so a "kill" op can cancel it.
type inflightExec struct {
	cancel context.CancelFunc
}

// shellState lives on the Manager so multiple concurrent execs can be tracked
// per shell channel. We allocate it lazily.
type shellState struct {
	mu       sync.Mutex
	inflight map[string]*inflightExec
}

func (m *Manager) ensureShellState() *shellState {
	m.shellOnce.Do(func() {
		m.shellSt = &shellState{inflight: make(map[string]*inflightExec)}
	})
	return m.shellSt
}

// setupShellChannelHandlers wires the "shell" data channel to the shell
// package. Protocol (op-based JSON):
//
//	→ {"op":"exec","id":"<uuid>","cmd":"...","as_user":bool,"timeout_sec":int}
//	← {"op":"started","id":"...","pid":1234}
//	← {"op":"stdout","id":"...","data":"<base64>"}
//	← {"op":"stderr","id":"...","data":"<base64>"}
//	← {"op":"exit","id":"...","code":0,"duration_ms":1234}
//	→ {"op":"kill","id":"..."}
func (m *Manager) setupShellChannelHandlers(dc *pionwebrtc.DataChannel) {
	state := m.ensureShellState()

	dc.OnOpen(func() {
		log.Println("🐚 Shell channel open")
	})

	dc.OnClose(func() {
		log.Println("🐚 Shell channel closed — killing in-flight commands")
		state.mu.Lock()
		for _, ex := range state.inflight {
			ex.cancel()
		}
		state.inflight = make(map[string]*inflightExec)
		state.mu.Unlock()
	})

	dc.OnMessage(func(msg pionwebrtc.DataChannelMessage) {
		var req map[string]interface{}
		if err := json.Unmarshal(msg.Data, &req); err != nil {
			log.Printf("⚠️ shell: invalid message: %v", err)
			return
		}
		op, _ := req["op"].(string)
		id, _ := req["id"].(string)

		switch op {
		case "exec":
			cmdStr, _ := req["cmd"].(string)
			asUser, _ := req["as_user"].(bool)
			timeoutF, _ := req["timeout_sec"].(float64)
			go m.handleShellExec(dc, id, cmdStr, asUser, int(timeoutF))
		case "kill":
			state.mu.Lock()
			ex, ok := state.inflight[id]
			state.mu.Unlock()
			if ok {
				ex.cancel()
				log.Printf("🐚 shell: killed exec id=%s", id)
			}
		default:
			sendShellMsg(dc, map[string]interface{}{
				"op":    "error",
				"id":    id,
				"error": "unknown op: " + op,
			})
		}
	})
}

func (m *Manager) handleShellExec(dc *pionwebrtc.DataChannel, id, cmdStr string, asUser bool, timeoutSec int) {
	if id == "" || cmdStr == "" {
		sendShellMsg(dc, map[string]interface{}{
			"op": "error", "id": id, "error": "missing id or cmd",
		})
		return
	}

	state := m.ensureShellState()
	ctx, cancel := context.WithCancel(context.Background())
	state.mu.Lock()
	state.inflight[id] = &inflightExec{cancel: cancel}
	state.mu.Unlock()
	defer func() {
		state.mu.Lock()
		delete(state.inflight, id)
		state.mu.Unlock()
		cancel()
	}()

	log.Printf("🐚 shell: exec id=%s as_user=%v timeout=%ds cmd=%q", id, asUser, timeoutSec, truncate(cmdStr, 200))

	onStarted := func(pid int) {
		sendShellMsg(dc, map[string]interface{}{
			"op": "started", "id": id, "pid": pid,
		})
	}
	onStdout := func(data []byte) {
		sendShellMsg(dc, map[string]interface{}{
			"op": "stdout", "id": id, "data": string(data),
		})
	}
	onStderr := func(data []byte) {
		sendShellMsg(dc, map[string]interface{}{
			"op": "stderr", "id": id, "data": string(data),
		})
	}

	res := shell.Run(ctx, shell.ExecOptions{
		Cmd:        cmdStr,
		AsUser:     asUser,
		TimeoutSec: timeoutSec,
	}, onStarted, onStdout, onStderr)

	exitMsg := map[string]interface{}{
		"op":          "exit",
		"id":          id,
		"code":        res.ExitCode,
		"pid":         res.PID,
		"duration_ms": res.DurationMs,
	}
	if res.Err != nil {
		exitMsg["error"] = res.Err.Error()
	}
	sendShellMsg(dc, exitMsg)

	// Best-effort audit log (async, fire-and-forget)
	if m.device != nil {
		go m.auditShellCommand(cmdStr, asUser, res)
	}
}

func sendShellMsg(dc *pionwebrtc.DataChannel, msg map[string]interface{}) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("⚠️ shell: marshal error: %v", err)
		return
	}
	if err := dc.Send(data); err != nil {
		log.Printf("⚠️ shell: send error: %v", err)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// auditShellCommand records an executed shell command. Posts to Supabase
// audit_logs (fire-and-forget); also logged locally so the operator can
// inspect it on the host without database access.
func (m *Manager) auditShellCommand(cmd string, asUser bool, res shell.Result) {
	log.Printf("🛡️ AUDIT shell exit=%d duration=%dms as_user=%v cmd=%q",
		res.ExitCode, res.DurationMs, asUser, truncate(cmd, 200))

	severity := "info"
	if res.ExitCode != 0 || res.Err != nil {
		severity = "warning"
	}
	details := map[string]interface{}{
		"cmd":         truncate(cmd, 4096),
		"as_user":     asUser,
		"exit_code":   res.ExitCode,
		"duration_ms": res.DurationMs,
		"pid":         res.PID,
	}
	if res.Err != nil {
		details["error"] = res.Err.Error()
	}
	m.device.WriteAudit(device.AuditEvent{
		Event:    "SHELL_EXEC",
		Severity: severity,
		Details:  details,
	})
}
