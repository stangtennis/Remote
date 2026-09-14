package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFakeCLI creates an executable stub that stands in for
// remote-desktop-cli. body is appended after #!/bin/sh.
func writeFakeCLI(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "remote-desktop-cli")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunCLIRunsOnlyAllowListedSubcommands(t *testing.T) {
	record := filepath.Join(t.TempDir(), "argv")
	t.Setenv("FAKE_CLI_RECORD", record)
	cli := writeFakeCLI(t, `printf '%s\n' "$@" > "$FAKE_CLI_RECORD"`)

	for _, sub := range []string{"list", "status", "support-list"} {
		if _, _, err := (&cliRunner{path: cli}).runCLI(sub); err != nil {
			t.Fatalf("runCLI(%q) unexpected error: %v", sub, err)
		}
		got, err := os.ReadFile(record)
		if err != nil {
			t.Fatalf("record missing for %q: %v", sub, err)
		}
		if strings.TrimSpace(string(got)) != sub {
			t.Fatalf("runCLI(%q) argv = %q, want exactly the subcommand", sub, got)
		}
		os.Remove(record)
	}
}

func TestRunCLIRejectsUnsafeSubcommands(t *testing.T) {
	record := filepath.Join(t.TempDir(), "argv")
	t.Setenv("FAKE_CLI_RECORD", record)
	cli := writeFakeCLI(t, `printf '%s\n' "$@" > "$FAKE_CLI_RECORD"`)

	dangerous := []string{
		"click", "type", "key", "scroll", "exec", "upload", "download",
		"connect", "disconnect", "screenshot", "ps", "kill", "sysinfo",
		"status; rm -rf /", "status --extra-flag", "", "support-watch",
	}
	for _, sub := range dangerous {
		_, _, err := (&cliRunner{path: cli}).runCLI(sub)
		if err == nil {
			t.Errorf("runCLI(%q) unexpectedly allowed", sub)
		}
	}
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Fatalf("rejected subcommand must not execute the CLI (record file exists)")
	}
}

func TestRunCLIMissingBinary(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	_, _, err := (&cliRunner{path: missing}).runCLI("status")
	if err == nil {
		t.Fatal("expected error for missing CLI binary")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Fatalf("error should mention the missing path, got: %v", err)
	}
}

func TestRunCLIReturnsBoundedRedactedOutput(t *testing.T) {
	// ~200KB of output plus a leaked-looking secret; must come back bounded
	// and redacted.
	var big strings.Builder
	big.WriteString("RD_PASSWORD=super-secret token=abc123.456 apikey: XYZ789 Bearer jwtvalue\n")
	line := strings.Repeat("x", 200)
	for i := 0; i < 1000; i++ {
		big.WriteString(line + "\n")
	}
	outFile := filepath.Join(t.TempDir(), "big.txt")
	if err := os.WriteFile(outFile, []byte(big.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	cli := writeFakeCLI(t, `cat "$FAKE_CLI_OUT_FILE"`)

	t.Setenv("FAKE_CLI_OUT_FILE", outFile)
	stdout, _, err := (&cliRunner{path: cli}).runCLI("status")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stdout) > toolTextLimit+64 { // cap + truncation marker allowance
		t.Fatalf("output not bounded: %d bytes", len(stdout))
	}
	for _, leak := range []string{"super-secret", "abc123.456", "XYZ789", "jwtvalue"} {
		if strings.Contains(stdout, leak) {
			t.Fatalf("secret value %q leaked into tool output", leak)
		}
	}
	if !strings.Contains(stdout, "truncated") {
		t.Fatalf("expected truncation marker, got tail: %q", stdout[len(stdout)-80:])
	}
}

func TestRunCLIReportsExitFailure(t *testing.T) {
	cli := writeFakeCLI(t, `echo "boom" >&2; exit 3`)
	stdout, stderr, err := (&cliRunner{path: cli}).runCLI("status")
	if err == nil {
		t.Fatal("expected error for non-zero exit")
	}
	if !strings.Contains(err.Error(), "status") {
		t.Fatalf("error should name the subcommand: %v", err)
	}
	if !strings.Contains(stderr, "boom") {
		t.Fatalf("stderr should be surfaced: %q", stderr)
	}
	if stdout != "" {
		t.Fatalf("unexpected stdout: %q", stdout)
	}
}

func TestLimitedWriterBoundsMemory(t *testing.T) {
	var w limitedWriter
	w.reset(10)
	chunk := strings.Repeat("a", 8)
	for i := 0; i < 100; i++ {
		if n, err := w.Write([]byte(chunk)); n != len(chunk) || err != nil {
			t.Fatalf("Write returned %d, %v", n, err)
		}
	}
	if got := w.buf.Len(); got != 10 {
		t.Fatalf("buffer holds %d bytes, want 10", got)
	}
	if !w.truncated {
		t.Fatal("truncation should be flagged")
	}
}

func TestBoundTextPrefersLineBoundary(t *testing.T) {
	in := strings.Repeat("line\n", 1000)
	out := boundText(in, 1000)
	if len(out) > 1000+len("\n...[output truncated]") {
		t.Fatalf("boundText returned %d bytes", len(out))
	}
	if !strings.HasSuffix(out, "\n...[output truncated]") {
		t.Fatalf("missing truncation marker: %q", out[len(out)-40:])
	}
	if strings.HasSuffix(strings.TrimSuffix(out, "\n...[output truncated]"), "li") {
		t.Fatal("should cut at a line boundary, not mid-line")
	}
}

func TestRedactSecrets(t *testing.T) {
	cases := map[string]string{
		"RD_PASSWORD=hunter2":                     "RD_PASSWORD=***",
		"password: hunter2":                       "password: ***",
		"api_key=abcd1234":                        "api_key=***",
		"Authorization: Bearer eyJhb.abc.def":     "Authorization: ***",
		"connected to device abc (windows)":       "connected to device abc (windows)",
		"Key 123456 active expires 14:22 session": "Key 123456 active expires 14:22 session",
	}
	for in, want := range cases {
		if got := redactSecrets(in); got != want {
			t.Errorf("redactSecrets(%q) = %q, want %q", in, got, want)
		}
	}
}
