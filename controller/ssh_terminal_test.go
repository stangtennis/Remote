package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stangtennis/Remote/controller/internal/sshconfig"
)

func TestBuildSSHArgsEscapesRemoteWorkdir(t *testing.T) {
	dir := t.TempDir()
	key := filepath.Join(dir, "id_ed25519")
	if err := os.WriteFile(key, []byte("key"), 0600); err != nil {
		t.Fatal(err)
	}
	args, err := buildSSHArgs(&sshconfig.Config{
		Enabled: true, Host: "ubuntu.example", Port: 2222, User: "dennis",
		KeyPath: key, Workdir: "/home/dennis/ai work/'folder",
	})
	if err != nil {
		t.Fatal(err)
	}
	if args[0] != "-tt" || args[1] != "-i" || args[3] != "-p" {
		t.Fatalf("unexpected SSH args: %#v", args)
	}
	remote := args[len(args)-1]
	if !strings.Contains(remote, "cd -- '/home/dennis/ai work/'\\''folder'") {
		t.Fatalf("remote workdir was not shell-escaped: %q", remote)
	}
	if !strings.Contains(remote, "ai-controller.env") {
		t.Fatalf("remote environment bootstrap missing: %q", remote)
	}
}
