package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withNoCredentialEnv clears credential env vars so tests never depend on
// (or leak) real credentials on the machine running the tests.
func withNoCredentialEnv(t *testing.T) {
	t.Helper()
	t.Setenv("RD_EMAIL", "")
	t.Setenv("RD_PASSWORD", "")
}

func writeCredsFile(t *testing.T, content string, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "credentials.env")
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil { // honor exact mode despite umask
		t.Fatal(err)
	}
	return path
}

func TestCheckCredentialPresenceFromEnv(t *testing.T) {
	withNoCredentialEnv(t)
	t.Setenv("RD_CREDENTIALS_FILE", filepath.Join(t.TempDir(), "absent.env"))

	t.Setenv("RD_EMAIL", "user@example.com")
	t.Setenv("RD_PASSWORD", "irrelevant-but-set")
	if err := checkCredentialPresence(); err != nil {
		t.Fatalf("env credentials should be accepted: %v", err)
	}
}

func TestCheckCredentialPresenceFromFile(t *testing.T) {
	withNoCredentialEnv(t)
	path := writeCredsFile(t, "# comment\nRD_EMAIL=user@example.com\nRD_PASSWORD=some-value\n", 0o600)
	t.Setenv("RD_CREDENTIALS_FILE", path)

	if err := checkCredentialPresence(); err != nil {
		t.Fatalf("valid credentials file should be accepted: %v", err)
	}
}

func TestCheckCredentialPresenceNothingConfigured(t *testing.T) {
	withNoCredentialEnv(t)
	t.Setenv("RD_CREDENTIALS_FILE", filepath.Join(t.TempDir(), "absent.env"))

	err := checkCredentialPresence()
	if err == nil {
		t.Fatal("expected error when no credentials are available")
	}
	if !strings.Contains(err.Error(), "absent.env") {
		t.Fatalf("error should name the credentials path: %v", err)
	}
	if strings.Contains(err.Error(), "some-value") || strings.Contains(err.Error(), "example.com") {
		t.Fatalf("error must not echo credential material: %v", err)
	}
}

func TestCheckCredentialPresenceRejectsLoosePermissions(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("permission checks are unreliable as root")
	}
	withNoCredentialEnv(t)
	path := writeCredsFile(t, "RD_EMAIL=user@example.com\nRD_PASSWORD=some-value\n", 0o644)
	t.Setenv("RD_CREDENTIALS_FILE", path)

	err := checkCredentialPresence()
	if err == nil {
		t.Fatal("group/world-readable credentials file must be rejected")
	}
	if !strings.Contains(err.Error(), "0600") {
		t.Fatalf("error should suggest mode 0600: %v", err)
	}
}

func TestCheckCredentialPresenceIncompleteFile(t *testing.T) {
	withNoCredentialEnv(t)

	for name, content := range map[string]string{
		"missing password": "RD_EMAIL=user@example.com\n",
		"missing email":    "RD_PASSWORD=some-value\n",
		"empty password":   "RD_EMAIL=user@example.com\nRD_PASSWORD=\n",
		"comments only":    "# nothing here\n",
	} {
		path := writeCredsFile(t, content, 0o600)
		t.Setenv("RD_CREDENTIALS_FILE", path)
		if err := checkCredentialPresence(); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestResolveCLIPath(t *testing.T) {
	t.Setenv("REMOTE_DESKTOP_CLI", "/definitely/not/here/remote-desktop-cli")
	if _, err := resolveCLIPath(); err == nil {
		t.Fatal("expected error for missing CLI")
	}

	exec := writeFakeCLI(t, "true")
	t.Setenv("REMOTE_DESKTOP_CLI", exec)
	if got, err := resolveCLIPath(); err != nil || got != exec {
		t.Fatalf("resolveCLIPath() = %q, %v; want %q, nil", got, err, exec)
	}

	notExec := filepath.Join(t.TempDir(), "cli-not-executable")
	if err := os.WriteFile(notExec, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("REMOTE_DESKTOP_CLI", notExec)
	if _, err := resolveCLIPath(); err == nil {
		t.Fatal("expected error for non-executable CLI")
	}
}
