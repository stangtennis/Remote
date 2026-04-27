package main

import (
	"encoding/json"
	"fmt"
	"time"
)

// requestProcessList asks the agent for the running process list. The agent
// responds on the same process channel with op="ps_result".
func requestProcessList(conn *DeviceConnection, timeout time.Duration) ([]map[string]interface{}, error) {
	if !conn.ProcessReady() {
		return nil, fmt.Errorf("process channel not open (agent likely older than v3.0.2)")
	}

	// ps/kill/sysinfo are id-less — register a generic "" subscriber.
	sub := conn.processRouter.Subscribe("")
	defer conn.processRouter.Unsubscribe("")

	req, _ := json.Marshal(map[string]interface{}{"op": "ps"})
	if err := conn.SendProcess(req); err != nil {
		return nil, fmt.Errorf("send ps: %w", err)
	}

	deadline := time.After(timeout)
	for {
		select {
		case msg, ok := <-sub:
			if !ok {
				return nil, fmt.Errorf("process channel closed unexpectedly")
			}
			var parsed map[string]interface{}
			if err := json.Unmarshal(msg, &parsed); err != nil {
				continue
			}
			switch parsed["op"] {
			case "ps_result":
				rawList, _ := parsed["processes"].([]interface{})
				out := make([]map[string]interface{}, 0, len(rawList))
				for _, raw := range rawList {
					if m, ok := raw.(map[string]interface{}); ok {
						out = append(out, m)
					}
				}
				return out, nil
			case "error":
				errStr, _ := parsed["error"].(string)
				return nil, fmt.Errorf("agent error: %s", errStr)
			}
		case <-deadline:
			return nil, fmt.Errorf("ps timeout after %s", timeout)
		}
	}
}

// requestProcessKill asks the agent to terminate the given PID.
func requestProcessKill(conn *DeviceConnection, pid int, timeout time.Duration) error {
	if !conn.ProcessReady() {
		return fmt.Errorf("process channel not open")
	}
	sub := conn.processRouter.Subscribe("")
	defer conn.processRouter.Unsubscribe("")

	req, _ := json.Marshal(map[string]interface{}{"op": "kill", "pid": pid})
	if err := conn.SendProcess(req); err != nil {
		return fmt.Errorf("send kill: %w", err)
	}

	deadline := time.After(timeout)
	for {
		select {
		case msg, ok := <-sub:
			if !ok {
				return fmt.Errorf("process channel closed unexpectedly")
			}
			var parsed map[string]interface{}
			if err := json.Unmarshal(msg, &parsed); err != nil {
				continue
			}
			if parsed["op"] == "kill_result" {
				if okV, _ := parsed["ok"].(bool); okV {
					return nil
				}
				errStr, _ := parsed["error"].(string)
				return fmt.Errorf("kill failed: %s", errStr)
			}
			if parsed["op"] == "error" {
				errStr, _ := parsed["error"].(string)
				return fmt.Errorf("agent error: %s", errStr)
			}
		case <-deadline:
			return fmt.Errorf("kill timeout after %s", timeout)
		}
	}
}

// requestSysinfo asks the agent for a system diagnostic snapshot.
func requestSysinfo(conn *DeviceConnection, timeout time.Duration) (map[string]interface{}, error) {
	if !conn.ProcessReady() {
		return nil, fmt.Errorf("process channel not open (agent likely older than v3.0.2)")
	}
	sub := conn.processRouter.Subscribe("")
	defer conn.processRouter.Unsubscribe("")

	req, _ := json.Marshal(map[string]interface{}{"op": "sysinfo"})
	if err := conn.SendProcess(req); err != nil {
		return nil, fmt.Errorf("send sysinfo: %w", err)
	}

	deadline := time.After(timeout)
	for {
		select {
		case msg, ok := <-sub:
			if !ok {
				return nil, fmt.Errorf("process channel closed unexpectedly")
			}
			var parsed map[string]interface{}
			if err := json.Unmarshal(msg, &parsed); err != nil {
				continue
			}
			if parsed["op"] == "sysinfo_result" {
				return parsed, nil
			}
			if parsed["op"] == "error" {
				errStr, _ := parsed["error"].(string)
				return nil, fmt.Errorf("agent error: %s", errStr)
			}
		case <-deadline:
			return nil, fmt.Errorf("sysinfo timeout after %s", timeout)
		}
	}
}
