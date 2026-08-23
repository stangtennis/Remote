package main

import (
	"fmt"
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
	return []string{
		"-tt",
		"-i", filepath.Clean(keyPath),
		"-p", fmt.Sprintf("%d", cfg.Port),
		"-o", "ConnectTimeout=15",
		"-o", "ServerAliveInterval=30",
		"-o", "ServerAliveCountMax=3",
		fmt.Sprintf("%s@%s", cfg.User, cfg.Host),
		remoteShellCommand(cfg.Workdir),
	}, nil
}

func remoteShellCommand(workdir string) string {
	return "cd -- " + shellQuote(workdir) +
		" && if [ -f ./ai-controller.env ]; then set -a; . ./ai-controller.env; set +a; fi" +
		" && exec \"${SHELL:-/bin/bash}\" -il"
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
