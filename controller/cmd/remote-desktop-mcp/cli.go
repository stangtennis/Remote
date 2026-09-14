package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// Output limits. CLI output is expected to be tiny (a handful of lines);
// the limits only exist as a hard safety bound so a misbehaving CLI can
// never flood the MCP transport or memory.
const (
	cliStreamLimit = 64 << 10 // per-stream capture cap while the CLI runs
	toolTextLimit  = 8 << 10  // final tool-result cap
	cliTimeout     = 30 * time.Second
)

// cliSubcommands is the fixed allow-list of CLI subcommands this adapter may
// invoke. Anything else (click/type/exec/upload/connect/...) is refused, so
// MCP tool input can never smuggle extra arguments into the subprocess.
var cliSubcommands = map[string]bool{
	"list":         true,
	"status":       true,
	"support-list": true,
}

// cliRunner invokes the existing remote-desktop-cli binary read-only.
//
// The subprocess is started with exec.Command directly (no shell), with a
// nil Env so it inherits the adapter's environment verbatim. That preserves
// RD_CREDENTIALS_FILE / RD_EMAIL / RD_PASSWORD exactly as the CLI's own
// credential loader (loadRemoteCredentials in the CLI) expects, without this
// adapter ever reading or printing secret values.
type cliRunner struct {
	path string
}

func newCLIRunner(path string) *cliRunner {
	return &cliRunner{path: path}
}

// runCLI executes one allow-listed subcommand and returns bounded, redacted
// stdout/stderr. It never passes user-controlled text to the subprocess.
func (r *cliRunner) runCLI(subcommand string) (stdout, stderr string, err error) {
	if !cliSubcommands[subcommand] {
		return "", "", fmt.Errorf("subcommand %q is not allowed in this adapter", subcommand)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cliTimeout)
	defer cancel()

	// No shell: argv is fixed to [cliPath, subcommand].
	cmd := exec.CommandContext(ctx, r.path, subcommand)
	var out, errOut limitedWriter
	out.reset(cliStreamLimit)
	errOut.reset(cliStreamLimit)
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	runErr := cmd.Run()
	stdout = redactSecrets(boundText(out.String(), toolTextLimit))
	stderr = redactSecrets(boundText(errOut.String(), toolTextLimit))
	if runErr != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return stdout, stderr, fmt.Errorf("remote-desktop-cli %q timed out after %s", subcommand, cliTimeout)
		}
		return stdout, stderr, fmt.Errorf("remote-desktop-cli %q failed: %w", subcommand, runErr)
	}
	return stdout, stderr, nil
}

// limitedWriter is an io.Writer that keeps at most limit bytes and reports
// truncation. Extra bytes are silently dropped so the CLI never fails with a
// short write and unbounded output never accumulates in memory.
type limitedWriter struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (w *limitedWriter) reset(limit int) {
	w.buf.Reset()
	w.limit = limit
	w.truncated = false
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if room := w.limit - w.buf.Len(); room > 0 {
		if len(p) <= room {
			w.buf.Write(p)
		} else {
			w.buf.Write(p[:room])
			w.truncated = true
		}
	} else if len(p) > 0 {
		w.truncated = true
	}
	return len(p), nil
}

func (w *limitedWriter) String() string {
	if w.truncated {
		return w.buf.String() + "\n...[output truncated]"
	}
	return w.buf.String()
}

// boundText caps s at limit bytes, preferring to cut on the last newline so
// partial lines are not returned.
func boundText(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	cut := s[:limit]
	if i := strings.LastIndexByte(cut, '\n'); i > limit/2 {
		cut = cut[:i]
	}
	return cut + "\n...[output truncated]"
}

var (
	secretAssignmentRe = regexp.MustCompile(`(?i)\b(rd_password|password|passwd|token|secret|apikey|api_key|authorization)\b(\s*[=:]\s*)(?:bearer\s+)?(\S+)`)
	bearerRe           = regexp.MustCompile(`(?i)\bbearer\s+\S+`)
)

// redactSecrets masks values that look like credentials. The CLI is not
// expected to print any, but the adapter must never leak one if it does.
func redactSecrets(s string) string {
	s = secretAssignmentRe.ReplaceAllString(s, "$1$2***")
	return bearerRe.ReplaceAllString(s, "Bearer ***")
}
