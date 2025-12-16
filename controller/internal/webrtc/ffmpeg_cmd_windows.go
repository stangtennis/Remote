//go:build windows

package webrtc

import (
	"os/exec"
	"syscall"
)

func configureFFmpegCmd(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
}

