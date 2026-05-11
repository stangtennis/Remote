//go:build !windows

package main

import (
	"os"
	"os/exec"
	"syscall"
	"time"
)

func configureDaemonChild(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

func terminateDaemonProcess(proc *os.Process) {
	_ = proc.Signal(syscall.SIGTERM)
	time.Sleep(200 * time.Millisecond)
	_ = proc.Signal(syscall.SIGKILL)
}
