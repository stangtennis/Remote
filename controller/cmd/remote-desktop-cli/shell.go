package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// execRemoteCommand sends an exec request over the shell channel and streams
// stdout/stderr back via onOut/onErr callbacks. Blocks until the remote
// process exits, the timeout fires, or the channel closes.
func execRemoteCommand(conn *DeviceConnection, cmd string, asUser bool, timeoutSec int,
	onStarted func(pid int), onOut func(string), onErr func(string)) (exitCode int, durationMs int64, runErr error) {

	if !conn.ShellReady() {
		return -1, 0, fmt.Errorf("shell channel not open (agent likely older than v3.0.2)")
	}

	id := newExecID()
	sub := conn.shellRouter.Subscribe(id)
	defer conn.shellRouter.Unsubscribe(id)

	req := map[string]interface{}{
		"op":          "exec",
		"id":          id,
		"cmd":         cmd,
		"as_user":     asUser,
		"timeout_sec": timeoutSec,
	}
	data, _ := json.Marshal(req)
	if err := conn.SendShell(data); err != nil {
		return -1, 0, fmt.Errorf("send exec: %w", err)
	}

	// Use a generous outer deadline (2x command timeout) so we still bail out
	// if the agent stops responding mid-stream. Default is 5min.
	outerTimeout := time.Duration(timeoutSec)*2*time.Second + 30*time.Second
	if timeoutSec <= 0 {
		outerTimeout = 5 * time.Minute
	}
	deadline := time.After(outerTimeout)

	for {
		select {
		case msg, ok := <-sub:
			if !ok {
				return -1, 0, fmt.Errorf("shell channel closed before exit")
			}
			var parsed map[string]interface{}
			if err := json.Unmarshal(msg, &parsed); err != nil {
				continue
			}
			op, _ := parsed["op"].(string)
			switch op {
			case "started":
				if onStarted != nil {
					if pidF, ok := parsed["pid"].(float64); ok {
						onStarted(int(pidF))
					}
				}
			case "stdout":
				if onOut != nil {
					if s, ok := parsed["data"].(string); ok {
						onOut(s)
					}
				}
			case "stderr":
				if onErr != nil {
					if s, ok := parsed["data"].(string); ok {
						onErr(s)
					}
				}
			case "exit":
				code := 0
				if cF, ok := parsed["code"].(float64); ok {
					code = int(cF)
				}
				durMs := int64(0)
				if dF, ok := parsed["duration_ms"].(float64); ok {
					durMs = int64(dF)
				}
				if errStr, ok := parsed["error"].(string); ok && errStr != "" {
					return code, durMs, fmt.Errorf("remote: %s", errStr)
				}
				return code, durMs, nil
			case "error":
				errStr, _ := parsed["error"].(string)
				return -1, 0, fmt.Errorf("agent error: %s", errStr)
			}
		case <-deadline:
			// Best-effort kill before bailing
			killMsg, _ := json.Marshal(map[string]interface{}{"op": "kill", "id": id})
			_ = conn.SendShell(killMsg)
			return -1, 0, fmt.Errorf("exec timeout after %s", outerTimeout)
		}
	}
}

func newExecID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
