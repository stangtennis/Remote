//go:build darwin

package shell

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

// runPlatform executes a bash command on macOS. The agent runs as the user
// that started it (typically the logged-in user on macOS), so AsUser is a
// no-op here — there is no SYSTEM-equivalent privilege to drop.
func runPlatform(ctx context.Context, opts ExecOptions, onStarted StartedFunc, onStdout, onStderr OutputFunc) Result {
	start := time.Now()

	shell := "/bin/bash"
	if _, err := exec.LookPath("bash"); err != nil {
		shell = "/bin/sh"
	}

	cmd := exec.Command(shell, "-c", opts.Cmd)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return Result{Err: fmt.Errorf("stdout pipe: %w", err), DurationMs: time.Since(start).Milliseconds()}
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return Result{Err: fmt.Errorf("stderr pipe: %w", err), DurationMs: time.Since(start).Milliseconds()}
	}

	if err := cmd.Start(); err != nil {
		return Result{Err: fmt.Errorf("start: %w", err), DurationMs: time.Since(start).Milliseconds()}
	}

	pid := cmd.Process.Pid
	if onStarted != nil {
		onStarted(pid)
	}

	var timer *time.Timer
	if opts.TimeoutSec > 0 {
		timer = time.AfterFunc(time.Duration(opts.TimeoutSec)*time.Second, func() {
			_ = cmd.Process.Kill()
		})
	}

	ctxDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = cmd.Process.Kill()
		case <-ctxDone:
		}
	}()

	var wg sync.WaitGroup
	wg.Add(2)
	go drainPipe(&wg, stdoutPipe, onStdout)
	go drainPipe(&wg, stderrPipe, onStderr)
	wg.Wait()

	exitErr := cmd.Wait()
	close(ctxDone)
	if timer != nil {
		timer.Stop()
	}

	exitCode := 0
	if exitErr != nil {
		if ee, ok := exitErr.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			return Result{PID: pid, ExitCode: -1, Err: exitErr, DurationMs: time.Since(start).Milliseconds()}
		}
	}
	return Result{PID: pid, ExitCode: exitCode, DurationMs: time.Since(start).Milliseconds()}
}

func drainPipe(wg *sync.WaitGroup, r io.ReadCloser, cb OutputFunc) {
	defer wg.Done()
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 && cb != nil {
			out := make([]byte, n)
			copy(out, buf[:n])
			cb(out)
		}
		if err != nil {
			return
		}
	}
}
