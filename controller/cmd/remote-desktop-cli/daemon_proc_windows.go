//go:build windows

package main

import (
	"os"
	"os/exec"
)

func configureDaemonChild(cmd *exec.Cmd) {
	// Keep default SysProcAttr on Windows for broad compatibility.
	cmd.SysProcAttr = nil
}

func terminateDaemonProcess(proc *os.Process) {
	_ = proc.Kill()
}
