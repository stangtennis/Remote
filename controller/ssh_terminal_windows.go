//go:build windows

package main

import (
	"fmt"
	"io"
	"os/exec"
	"sync"

	"github.com/stangtennis/Remote/controller/internal/sshconfig"
)

type sshWindowsTerminal struct {
	stdin io.WriteCloser
	cmd   *exec.Cmd
	mu    sync.Mutex
}

func startSSHTerminal(cfg *sshconfig.Config, onOutput func([]byte), onExit func()) (localTerminalSession, error) {
	sshPath, err := exec.LookPath("ssh.exe")
	if err != nil {
		return nil, fmt.Errorf("OpenSSH (ssh.exe) was not found on PATH: %w", err)
	}
	args, err := buildSSHArgs(cfg)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(sshPath, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = stderr.Close()
		return nil, fmt.Errorf("could not start SSH terminal: %w", err)
	}
	session := &sshWindowsTerminal{stdin: stdin, cmd: cmd}
	go func() {
		done := make(chan struct{}, 2)
		go func() { _, _ = io.Copy(outputWriter{onOutput}, stdout); done <- struct{}{} }()
		go func() { _, _ = io.Copy(outputWriter{onOutput}, stderr); done <- struct{}{} }()
		<-done
		<-done
		_ = cmd.Wait()
		onExit()
	}()
	return session, nil
}

func (s *sshWindowsTerminal) Write(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stdin == nil {
		return fmt.Errorf("SSH terminal is closed")
	}
	_, err := s.stdin.Write(data)
	return err
}

func (s *sshWindowsTerminal) Close() error {
	s.mu.Lock()
	stdin := s.stdin
	s.stdin = nil
	s.mu.Unlock()
	if stdin != nil {
		_ = stdin.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		return s.cmd.Process.Kill()
	}
	return nil
}
