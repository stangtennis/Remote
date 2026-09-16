// remote-desktop-mcp is a bounded local MCP adapter (stdio) that
// fronts the existing remote-desktop-cli and the central support MCP context.
//
// It deliberately exposes no control transport. Status tools use the CLI;
// context tools proxy the central support MCP endpoint.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/server"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "remote-desktop-mcp: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Fail fast, before the MCP transport starts, on missing CLI or
	// missing auth configuration. Clear stderr only; nothing secret.
	cliPath, err := resolveCLIPath()
	if err != nil {
		return err
	}
	if err := checkCredentialPresence(); err != nil {
		return fmt.Errorf("auth configuration error: %w", err)
	}
	contextClient, err := newEdgeMCPClient()
	if err != nil {
		return fmt.Errorf("central MCP configuration error: %w", err)
	}

	return server.ServeStdio(newServer(newCLIRunner(cliPath), contextClient))
}

// resolveCLIPath returns the configured remote-desktop-cli path
// (REMOTE_DESKTOP_CLI, default ~/.local/bin/remote-desktop-cli) after
// verifying it exists and is executable.
func resolveCLIPath() (string, error) {
	path := os.Getenv("REMOTE_DESKTOP_CLI")
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine home directory: %w", err)
		}
		path = filepath.Join(home, ".local", "bin", "remote-desktop-cli")
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("remote-desktop-cli not found at %s (install it or set REMOTE_DESKTOP_CLI)", path)
	}
	if info.IsDir() || info.Mode()&0o111 == 0 {
		return "", fmt.Errorf("remote-desktop-cli at %s is not an executable file (set REMOTE_DESKTOP_CLI)", path)
	}
	return path, nil
}
