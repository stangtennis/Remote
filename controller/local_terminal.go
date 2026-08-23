package main

import (
	"fmt"
	"os"
	"path/filepath"
	stdruntime "runtime"
	"strings"

	"github.com/stangtennis/Remote/controller/internal/sshconfig"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type localTerminalSession interface {
	Write([]byte) error
	Close() error
}

type outputWriter struct{ onOutput func([]byte) }

func (w outputWriter) Write(data []byte) (int, error) {
	copyData := append([]byte(nil), data...)
	w.onOutput(copyData)
	return len(data), nil
}

func (a *App) StartLocalAITerminal() error {
	if !a.isApprovedAdmin() {
		return fmt.Errorf("approved admin access required")
	}
	if a.ctx == nil {
		return fmt.Errorf("controller is not ready")
	}
	a.localTerminalMu.Lock()
	defer a.localTerminalMu.Unlock()
	if a.localTerminal != nil {
		return nil
	}
	cwd, err := localAITerminalDirectory()
	a.sshConfigMu.RLock()
	configuredSSH := cloneSSHConfig(a.sshConfig)
	a.sshConfigMu.RUnlock()
	onOutput := func(data []byte) { runtime.EventsEmit(a.ctx, "local-ai-terminal-output", string(data)) }
	var session localTerminalSession
	onExit := func() {
		a.localTerminalMu.Lock()
		if a.localTerminal == session {
			a.localTerminal = nil
		}
		a.localTerminalMu.Unlock()
		runtime.EventsEmit(a.ctx, "local-ai-terminal-exit")
	}
	if configuredSSH != nil && configuredSSH.Enabled {
		if err := configuredSSH.Validate(); err != nil {
			return err
		}
		cwd = configuredSSH.Workdir
		session, err = startSSHTerminal(configuredSSH, onOutput, onExit)
	} else {
		if err != nil {
			return err
		}
		session, err = startLocalTerminal(cwd, onOutput, onExit)
	}
	if err != nil {
		return err
	}
	a.localTerminal = session
	runtime.EventsEmit(a.ctx, "local-ai-terminal-started", cwd)
	return nil
}

func (a *App) WriteLocalAITerminal(data string) error {
	if !a.isApprovedAdmin() {
		return fmt.Errorf("approved admin access required")
	}
	a.localTerminalMu.Lock()
	session := a.localTerminal
	a.localTerminalMu.Unlock()
	if session == nil {
		return fmt.Errorf("AI terminal is not running")
	}
	return session.Write([]byte(data))
}

func (a *App) StopLocalAITerminal() {
	a.localTerminalMu.Lock()
	session := a.localTerminal
	a.localTerminal = nil
	a.localTerminalMu.Unlock()
	if session != nil {
		_ = session.Close()
	}
}

func (a *App) GetLocalAITerminalDirectory() (string, error) {
	if !a.isApprovedAdmin() {
		return "", fmt.Errorf("approved admin access required")
	}
	a.sshConfigMu.RLock()
	configuredSSH := cloneSSHConfig(a.sshConfig)
	a.sshConfigMu.RUnlock()
	if configuredSSH != nil && configuredSSH.Enabled {
		return fmt.Sprintf("ssh://%s@%s:%d%s", configuredSSH.User, configuredSSH.Host, configuredSSH.Port, configuredSSH.Workdir), nil
	}
	return localAITerminalDirectory()
}

func (a *App) GetSSHConfig() (*sshconfig.Config, error) {
	if !a.isApprovedAdmin() {
		return nil, fmt.Errorf("approved admin access required")
	}
	a.sshConfigMu.RLock()
	defer a.sshConfigMu.RUnlock()
	return cloneSSHConfig(a.sshConfig), nil
}

func (a *App) SaveSSHConfig(c *sshconfig.Config) error {
	if !a.isApprovedAdmin() {
		return fmt.Errorf("approved admin access required")
	}
	if c == nil {
		return fmt.Errorf("SSH configuration is missing")
	}
	copyConfig := *c
	if err := sshconfig.Save(&copyConfig); err != nil {
		return err
	}
	a.sshConfigMu.Lock()
	a.sshConfig = &copyConfig
	a.sshConfigMu.Unlock()
	return nil
}

func cloneSSHConfig(c *sshconfig.Config) *sshconfig.Config {
	if c == nil {
		return sshconfig.Default()
	}
	copyConfig := *c
	return &copyConfig
}

func (a *App) isApprovedAdmin() bool {
	return a.currentUser != nil && a.supabase != nil && a.supabase.IsAdmin(a.currentUser.ID)
}

func localAITerminalDirectory() (string, error) {
	path := os.Getenv("RD_AI_WORKDIR")
	if path == "" && stdruntime.GOOS == "linux" {
		path = "/home/dennis/projekter/aisupport"
	}
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine home directory: %w", err)
		}
		path = filepath.Join(home, "aisupport")
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid AI terminal directory: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("AI terminal directory unavailable: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("AI terminal path is not a directory: %s", path)
	}
	return path, nil
}

func localAIControllerKey(cwd string) string {
	data, err := os.ReadFile(filepath.Join(cwd, "ai-controller.env"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), "=", 2)
		if len(parts) == 2 && parts[0] == "RD_AI_CONTROLLER_KEY" {
			return strings.TrimSpace(parts[1])
		}
	}
	return ""
}

func (a *App) closeLocalAITerminal() {
	a.StopLocalAITerminal()
}
