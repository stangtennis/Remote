package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/stangtennis/Remote/controller/internal/sshconfig"
)

func buildSSHArgs(cfg *sshconfig.Config) ([]string, error) {
	if cfg == nil {
		return nil, fmt.Errorf("SSH configuration is missing")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	keyPath, err := sshconfig.ResolveKeyPath(cfg.KeyPath)
	if err != nil {
		return nil, err
	}
	args := []string{
		"-tt",
		"-i", filepath.Clean(keyPath),
		"-p", fmt.Sprintf("%d", cfg.Port),
		"-o", "ConnectTimeout=15",
		"-o", "ServerAliveInterval=30",
		"-o", "ServerAliveCountMax=3",
	}
	if cfg.CloudflareAccess {
		args = append(args, "-o", "ProxyCommand=cloudflared access ssh --hostname %h")
	}
	args = append(args, fmt.Sprintf("%s@%s", cfg.User, cfg.Host), remoteShellCommand(cfg.Workdir))
	return args, nil
}

func validateSSHRuntime(cfg *sshconfig.Config) error {
	if cfg.CloudflareAccess {
		if _, err := exec.LookPath("cloudflared"); err != nil {
			return fmt.Errorf("cloudflared is required for the Cloudflare SSH bridge: %w", err)
		}
	}
	return nil
}

func remoteShellCommand(workdir string) string {
	return "cd -- " + shellQuote(workdir) +
		" && if [ -f ./ai-controller.env ]; then set -a; . ./ai-controller.env; set +a; fi" +
		" && exec \"${SHELL:-/bin/bash}\" -il"
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
