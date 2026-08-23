//go:build !windows

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/creack/pty/v2"
	"github.com/stangtennis/Remote/controller/internal/sshconfig"
)

type sshUnixTerminal struct {
	ptmx *os.File
	cmd  *exec.Cmd
}

func startSSHTerminal(cfg *sshconfig.Config, onOutput func([]byte), onExit func()) (localTerminalSession, error) {
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return nil, fmt.Errorf("OpenSSH was not found on PATH: %w", err)
	}
	args, err := buildSSHArgs(cfg)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(sshPath, args...)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=true")
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("could not start SSH terminal: %w", err)
	}
	session := &sshUnixTerminal{ptmx: ptmx, cmd: cmd}
	go func() {
		_, _ = io.Copy(outputWriter{onOutput}, ptmx)
		_ = cmd.Wait()
		_ = ptmx.Close()
		onExit()
	}()
	return session, nil
}

func (s *sshUnixTerminal) Write(data []byte) error {
	if s.ptmx == nil {
		return fmt.Errorf("SSH terminal is closed")
	}
	_, err := s.ptmx.Write(data)
	return err
}

func (s *sshUnixTerminal) Close() error {
	if s.ptmx != nil {
		_ = s.ptmx.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		return s.cmd.Process.Kill()
	}
	return nil
}
