package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// streamingDial opens the daemon socket without setting an aggressive deadline,
// so streaming commands can keep the connection open for the full operation.
func streamingDial() (net.Conn, error) {
	socketPath := getSocketPath()
	conn, err := net.DialTimeout("unix", socketPath, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("daemon not running (use 'connect' first): %v", err)
	}
	// 15 minute deadline matches the daemon-side deadline for exec/upload/download.
	conn.SetDeadline(time.Now().Add(15 * time.Minute))
	return conn, nil
}

func cmdExec() {
	asUser := false
	timeoutSec := 300
	var cmdParts []string
	for i := 2; i < len(os.Args); i++ {
		a := os.Args[i]
		switch {
		case a == "--as-user":
			asUser = true
		case strings.HasPrefix(a, "--timeout="):
			if v, err := strconv.Atoi(strings.TrimPrefix(a, "--timeout=")); err == nil {
				timeoutSec = v
			}
		case a == "--timeout":
			if i+1 < len(os.Args) {
				if v, err := strconv.Atoi(os.Args[i+1]); err == nil {
					timeoutSec = v
				}
				i++
			}
		default:
			cmdParts = append(cmdParts, a)
		}
	}
	if len(cmdParts) == 0 {
		fmt.Fprintln(os.Stderr, `Usage: remote-desktop-cli exec [--as-user] [--timeout=N] "command"`)
		os.Exit(2)
	}
	cmd := strings.Join(cmdParts, " ")

	conn, err := streamingDial()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	if err := json.NewEncoder(conn).Encode(daemonRequest{
		Cmd: "exec",
		Args: map[string]interface{}{
			"cmd":         cmd,
			"as_user":     asUser,
			"timeout_sec": timeoutSec,
		},
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Error: send request: %v\n", err)
		os.Exit(1)
	}

	dec := json.NewDecoder(conn)
	exitCode := 0
	for {
		var m streamMsg
		if err := dec.Decode(&m); err != nil {
			fmt.Fprintf(os.Stderr, "\nError reading from daemon: %v\n", err)
			os.Exit(1)
		}
		switch m.Type {
		case "started":
			// silent — exit code is what the user actually wants
		case "stdout":
			fmt.Print(m.Data)
		case "stderr":
			fmt.Fprint(os.Stderr, m.Data)
		case "exit":
			exitCode = m.Code
			if m.Error != "" {
				fmt.Fprintf(os.Stderr, "\n%s\n", m.Error)
			}
			os.Exit(exitCode)
		case "error":
			fmt.Fprintf(os.Stderr, "Error: %s\n", m.Error)
			os.Exit(1)
		}
	}
}

func cmdUpload() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "Usage: remote-desktop-cli upload <local> <remote>")
		os.Exit(2)
	}
	local := os.Args[2]
	remote := os.Args[3]
	abs, err := filepath.Abs(local)
	if err == nil {
		local = abs
	}
	streamFileTransfer("upload", local, remote)
}

func cmdDownload() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "Usage: remote-desktop-cli download <remote> <local>")
		os.Exit(2)
	}
	remote := os.Args[2]
	local := os.Args[3]
	abs, err := filepath.Abs(local)
	if err == nil {
		local = abs
	}
	streamFileTransfer("download", local, remote)
}

func streamFileTransfer(kind, local, remote string) {
	conn, err := streamingDial()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	if err := json.NewEncoder(conn).Encode(daemonRequest{
		Cmd: kind,
		Args: map[string]interface{}{
			"local":  local,
			"remote": remote,
		},
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	dec := json.NewDecoder(conn)
	for {
		var m streamMsg
		if err := dec.Decode(&m); err != nil {
			fmt.Fprintf(os.Stderr, "Error reading from daemon: %v\n", err)
			os.Exit(1)
		}
		if m.Type == "end" {
			if m.Error != "" {
				fmt.Fprintf(os.Stderr, "Failed: %s (%d bytes transferred)\n", m.Error, m.Bytes)
				os.Exit(1)
			}
			fmt.Printf("OK: %d bytes %s\n", m.Bytes, map[string]string{
				"upload":   "uploaded → " + remote,
				"download": "downloaded → " + local,
			}[kind])
			return
		}
		if m.Type == "error" {
			fmt.Fprintf(os.Stderr, "Error: %s\n", m.Error)
			os.Exit(1)
		}
	}
}

func cmdPs() {
	resp, err := sendDaemonRequest(daemonRequest{Cmd: "ps"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if !resp.OK {
		fmt.Fprintf(os.Stderr, "Error: %s\n", resp.Error)
		os.Exit(1)
	}
	rawList, _ := resp.Data["processes"].([]interface{})
	if len(rawList) == 0 {
		fmt.Println("(no processes)")
		return
	}
	fmt.Printf("%-7s  %-25s  %8s  %5s  %s\n", "PID", "NAME", "MEM(MB)", "CPU%", "USER")
	for _, raw := range rawList {
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		pid, _ := m["pid"].(float64)
		name, _ := m["name"].(string)
		mem, _ := m["memory_mb"].(float64)
		cpu, _ := m["cpu"].(float64)
		user, _ := m["user"].(string)
		if len(name) > 25 {
			name = name[:24] + "…"
		}
		fmt.Printf("%-7d  %-25s  %8.1f  %5.1f  %s\n", int(pid), name, mem, cpu, user)
	}
}

func cmdKill() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: remote-desktop-cli kill <pid>")
		os.Exit(2)
	}
	pid, err := strconv.Atoi(os.Args[2])
	if err != nil || pid <= 0 {
		fmt.Fprintf(os.Stderr, "Invalid pid: %s\n", os.Args[2])
		os.Exit(2)
	}
	resp, err := sendDaemonRequest(daemonRequest{Cmd: "kill", Args: map[string]interface{}{"pid": pid}})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if !resp.OK {
		fmt.Fprintf(os.Stderr, "Error: %s\n", resp.Error)
		os.Exit(1)
	}
	fmt.Printf("Killed PID %d\n", pid)
}

func cmdSysinfo() {
	resp, err := sendDaemonRequest(daemonRequest{Cmd: "sysinfo"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if !resp.OK {
		fmt.Fprintf(os.Stderr, "Error: %s\n", resp.Error)
		os.Exit(1)
	}
	d := resp.Data
	fmt.Printf("Host:       %s\n", d["hostname"])
	fmt.Printf("OS:         %s\n", d["os"])
	fmt.Printf("CPU:        %s (%v cores)\n", d["cpu"], d["cpu_cores"])
	fmt.Printf("RAM:        %.1f GB free of %.1f GB\n", numFloat(d["ram_free_gb"]), numFloat(d["ram_total_gb"]))
	if up, ok := d["uptime_sec"].(float64); ok {
		fmt.Printf("Uptime:     %s\n", time.Duration(int64(up))*time.Second)
	}
	fmt.Println("Disks:")
	if disks, ok := d["disks"].([]interface{}); ok {
		for _, raw := range disks {
			m, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			fmt.Printf("  %-10s %.1f GB free / %.1f GB total\n",
				m["mount"], numFloat(m["free_gb"]), numFloat(m["total_gb"]))
		}
	}
	if apps, ok := d["installed_apps"].([]interface{}); ok && len(apps) > 0 {
		fmt.Printf("Installed:  %d apps\n", len(apps))
		// Print first 20 to keep output compact; full list is in JSON via the daemon if needed
		shown := 20
		if len(apps) < shown {
			shown = len(apps)
		}
		for i := 0; i < shown; i++ {
			m, ok := apps[i].(map[string]interface{})
			if !ok {
				continue
			}
			ver := ""
			if v, ok := m["version"].(string); ok && v != "" {
				ver = " " + v
			}
			fmt.Printf("  %s%s\n", m["name"], ver)
		}
		if len(apps) > shown {
			fmt.Printf("  …and %d more\n", len(apps)-shown)
		}
	}
}

func numFloat(v interface{}) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	return 0
}
