package main

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

// streamMsg is the wire format used by streaming daemon → CLI commands. The
// CLI reads JSON messages in a loop and stops once it sees Type=="end".
type streamMsg struct {
	Type    string  `json:"type"`              // "started" | "stdout" | "stderr" | "exit" | "progress" | "end" | "error"
	PID     int     `json:"pid,omitempty"`
	Code    int     `json:"code"`              // populated on "exit"
	Data    string  `json:"data,omitempty"`
	Bytes   int64   `json:"bytes,omitempty"`
	Total   int64   `json:"total,omitempty"`
	Error   string  `json:"error,omitempty"`
	Elapsed float64 `json:"elapsed_ms,omitempty"`
}

// streamWriter serializes writes to a net.Conn from multiple goroutines.
type streamWriter struct {
	mu  sync.Mutex
	enc *json.Encoder
}

func newStreamWriter(conn net.Conn) *streamWriter {
	return &streamWriter{enc: json.NewEncoder(conn)}
}

func (sw *streamWriter) Send(msg streamMsg) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.enc.Encode(msg)
}

// handleExecStream streams a remote PowerShell/bash command back to the CLI.
func handleExecStream(conn net.Conn, req daemonRequest, connMgr *ConnectionManager, deviceID string) {
	sw := newStreamWriter(conn)
	deviceConn, err := connMgr.GetConnection(deviceID)
	if err != nil {
		sw.Send(streamMsg{Type: "error", Error: err.Error()})
		return
	}

	cmd, _ := req.Args["cmd"].(string)
	asUser, _ := req.Args["as_user"].(bool)
	timeoutF, _ := req.Args["timeout_sec"].(float64)
	timeoutSec := int(timeoutF)
	if cmd == "" {
		sw.Send(streamMsg{Type: "error", Error: "empty cmd"})
		return
	}

	exitCode, durationMs, runErr := execRemoteCommand(deviceConn, cmd, asUser, timeoutSec,
		func(pid int) { sw.Send(streamMsg{Type: "started", PID: pid}) },
		func(out string) { sw.Send(streamMsg{Type: "stdout", Data: out}) },
		func(out string) { sw.Send(streamMsg{Type: "stderr", Data: out}) },
	)
	exitMsg := streamMsg{Type: "exit", Code: exitCode, Elapsed: float64(durationMs)}
	if runErr != nil {
		exitMsg.Error = runErr.Error()
	}
	sw.Send(exitMsg)
}

// handleFileStream handles upload/download with periodic progress messages.
func handleFileStream(conn net.Conn, req daemonRequest, connMgr *ConnectionManager, deviceID string) {
	sw := newStreamWriter(conn)
	deviceConn, err := connMgr.GetConnection(deviceID)
	if err != nil {
		sw.Send(streamMsg{Type: "error", Error: err.Error()})
		return
	}

	local, _ := req.Args["local"].(string)
	remote, _ := req.Args["remote"].(string)
	if local == "" || remote == "" {
		sw.Send(streamMsg{Type: "error", Error: "missing local or remote path"})
		return
	}

	// 10 minute timeout for large files; chunk-level acks already provide
	// flow control so this only fires if the agent stops responding.
	const opTimeout = 10 * time.Minute

	switch req.Cmd {
	case "upload":
		bytes, err := uploadLocalFile(deviceConn, local, remote, opTimeout)
		msg := streamMsg{Type: "end", Bytes: bytes}
		if err != nil {
			msg.Error = err.Error()
		}
		sw.Send(msg)
	case "download":
		bytes, err := downloadRemoteFile(deviceConn, remote, local, opTimeout)
		msg := streamMsg{Type: "end", Bytes: bytes}
		if err != nil {
			msg.Error = err.Error()
		}
		sw.Send(msg)
	default:
		sw.Send(streamMsg{Type: "error", Error: fmt.Sprintf("unknown file cmd: %s", req.Cmd)})
	}
}

// handlePs returns the running process list.
func handlePs(req daemonRequest, connMgr *ConnectionManager, deviceID string) daemonResponse {
	deviceConn, err := connMgr.GetConnection(deviceID)
	if err != nil {
		return daemonResponse{OK: false, Error: err.Error()}
	}
	procs, err := requestProcessList(deviceConn, 15*time.Second)
	if err != nil {
		return daemonResponse{OK: false, Error: err.Error()}
	}
	return daemonResponse{OK: true, Data: map[string]interface{}{"processes": procs}}
}

// handleKill terminates a remote process by PID.
func handleKill(req daemonRequest, connMgr *ConnectionManager, deviceID string) daemonResponse {
	deviceConn, err := connMgr.GetConnection(deviceID)
	if err != nil {
		return daemonResponse{OK: false, Error: err.Error()}
	}
	pidF, _ := req.Args["pid"].(float64)
	if pidF <= 0 {
		return daemonResponse{OK: false, Error: "invalid pid"}
	}
	if err := requestProcessKill(deviceConn, int(pidF), 10*time.Second); err != nil {
		return daemonResponse{OK: false, Error: err.Error()}
	}
	return daemonResponse{OK: true, Data: map[string]interface{}{"pid": int(pidF)}}
}

// handleSysinfo returns the diagnostic snapshot.
func handleSysinfo(req daemonRequest, connMgr *ConnectionManager, deviceID string) daemonResponse {
	deviceConn, err := connMgr.GetConnection(deviceID)
	if err != nil {
		return daemonResponse{OK: false, Error: err.Error()}
	}
	info, err := requestSysinfo(deviceConn, 30*time.Second)
	if err != nil {
		return daemonResponse{OK: false, Error: err.Error()}
	}
	return daemonResponse{OK: true, Data: info}
}
