package sshconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateDisabledDoesNotRequireKey(t *testing.T) {
	if err := (&Config{}).Validate(); err != nil {
		t.Fatalf("disabled config should validate: %v", err)
	}
}

func TestDefaultUsesCloudflareAIEndpoint(t *testing.T) {
	for _, name := range []string{"RD_AI_SSH_HOST", "RD_AI_SSH_USER", "RD_AI_SSH_WORKDIR", "RD_AI_SSH_KEY", "RD_AI_SSH_CLOUDFLARE"} {
		t.Setenv(name, "")
	}
	c := Default()
	if c.Host != "ssh.hawkeye123.dk" || c.User != "dennis" || c.Workdir != "/home/dennis/projekter/aisupport" || !c.CloudflareAccess {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}

func TestValidateEnabledConfig(t *testing.T) {
	dir := t.TempDir()
	key := filepath.Join(dir, "id_ed25519")
	if err := os.WriteFile(key, []byte("key"), 0600); err != nil {
		t.Fatal(err)
	}
	c := &Config{Enabled: true, Host: "ubuntu.example", Port: 22, User: "dennis", KeyPath: key, Workdir: "/home/dennis/projekter/aisupport"}
	if err := c.Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
}

func TestValidateRejectsShellOptionHost(t *testing.T) {
	dir := t.TempDir()
	key := filepath.Join(dir, "id_ed25519")
	if err := os.WriteFile(key, []byte("key"), 0600); err != nil {
		t.Fatal(err)
	}
	c := &Config{Enabled: true, Host: "-oProxyCommand=bad", User: "dennis", KeyPath: key, Workdir: "/tmp"}
	if err := c.Validate(); err == nil {
		t.Fatal("expected option-like host to be rejected")
	}
}
