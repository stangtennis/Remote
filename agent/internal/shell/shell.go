// Package shell provides remote shell command execution with streaming
// stdout/stderr output. On Windows commands run as SYSTEM by default; with
// AsUser set true the command runs as the active console user (via
// CreateProcessAsUser with a duplicated user token).
package shell

import "context"

// ExecOptions describes a single command invocation.
type ExecOptions struct {
	Cmd        string
	AsUser     bool
	TimeoutSec int
}

// Result is returned when the command finishes.
type Result struct {
	PID        int
	ExitCode   int
	DurationMs int64
	Err        error
}

// OutputFunc receives a chunk of stdout or stderr.
type OutputFunc func(data []byte)

// StartedFunc is called once the process has spawned.
type StartedFunc func(pid int)

// Run executes the command and streams output via callbacks. It blocks until
// the command exits, the timeout fires, or ctx is cancelled. The platform
// specific implementation lives in run_windows.go / run_darwin.go.
func Run(ctx context.Context, opts ExecOptions, onStarted StartedFunc, onStdout, onStderr OutputFunc) Result {
	return runPlatform(ctx, opts, onStarted, onStdout, onStderr)
}
