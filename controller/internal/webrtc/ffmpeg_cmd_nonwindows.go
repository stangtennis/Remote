//go:build !windows

package webrtc

import "os/exec"

func configureFFmpegCmd(cmd *exec.Cmd) {
	_ = cmd
}
