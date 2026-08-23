//go:build windows

package main

import (
	"io"
	"os"
	"os/exec"
)

type windowsLocalTerminal struct {
	stdin  io.WriteCloser
	cmd    *exec.Cmd
	stdout io.ReadCloser
}

func startLocalTerminal(cwd string, onOutput func([]byte), onExit func()) (localTerminalSession, error) {
	cmd := exec.Command("cmd.exe", "/Q")
	cmd.Dir = cwd
	cmd.Env = os.Environ()
	if key := localAIControllerKey(cwd); key != "" {
		cmd.Env = append(cmd.Env, "RD_AI_CONTROLLER_KEY="+key)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	session := &windowsLocalTerminal{stdin: stdin, cmd: cmd, stdout: stdout}
	go func() {
		_, _ = io.Copy(outputWriter{onOutput}, stdout)
		_, _ = io.Copy(outputWriter{onOutput}, stderr)
		_ = cmd.Wait()
		onExit()
	}()
	return session, nil
}

func (s *windowsLocalTerminal) Write(data []byte) error {
	_, err := s.stdin.Write(data)
	return err
}

func (s *windowsLocalTerminal) Close() error {
	if s.stdin != nil {
		_ = s.stdin.Close()
	}
	if s.stdout != nil {
		_ = s.stdout.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		return s.cmd.Process.Kill()
	}
	return nil
}
