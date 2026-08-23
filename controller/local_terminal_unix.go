//go:build !windows

package main

import (
	"io"
	"os"
	"os/exec"

	"github.com/creack/pty/v2"
)

type unixLocalTerminal struct {
	ptmx *os.File
	cmd  *exec.Cmd
}

func startLocalTerminal(cwd string, onOutput func([]byte), onExit func()) (localTerminalSession, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	cmd := exec.Command(shell, "-il")
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=true", "PWD="+cwd)
	if key := localAIControllerKey(cwd); key != "" {
		cmd.Env = append(cmd.Env, "RD_AI_CONTROLLER_KEY="+key)
	}
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}
	session := &unixLocalTerminal{ptmx: ptmx, cmd: cmd}
	go func() {
		_, _ = io.Copy(outputWriter{onOutput}, ptmx)
		_ = cmd.Wait()
		onExit()
	}()
	return session, nil
}

func (s *unixLocalTerminal) Write(data []byte) error {
	_, err := s.ptmx.Write(data)
	return err
}

func (s *unixLocalTerminal) Close() error {
	if s.ptmx != nil {
		_ = s.ptmx.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		return s.cmd.Process.Kill()
	}
	return nil
}
