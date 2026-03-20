package terminal

import (
	"io"
	"log"
	"os/exec"
	"runtime"
	"sync"
)

// Terminal manages a remote shell session
type Terminal struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	mu     sync.Mutex
	closed bool
}

// New creates and starts a new terminal session
func New() (*Terminal, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd.exe")
	} else {
		shell := "/bin/bash"
		// Try bash first, fall back to sh
		if _, err := exec.LookPath("bash"); err != nil {
			shell = "/bin/sh"
		}
		cmd = exec.Command(shell)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	log.Printf("Terminal started (PID: %d, shell: %s)", cmd.Process.Pid, cmd.Path)

	return &Terminal{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	}, nil
}

// Write sends input to the terminal
func (t *Terminal) Write(data []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return io.ErrClosedPipe
	}
	_, err := t.stdin.Write(data)
	return err
}

// ReadOutput reads from stdout in a loop and calls the callback
func (t *Terminal) ReadOutput(callback func(data []byte)) {
	buf := make([]byte, 4096)
	for {
		n, err := t.stdout.Read(buf)
		if n > 0 {
			out := make([]byte, n)
			copy(out, buf[:n])
			callback(out)
		}
		if err != nil {
			if err != io.EOF {
				log.Printf("Terminal stdout error: %v", err)
			}
			return
		}
	}
}

// ReadStderr reads from stderr in a loop and calls the callback
func (t *Terminal) ReadStderr(callback func(data []byte)) {
	buf := make([]byte, 4096)
	for {
		n, err := t.stderr.Read(buf)
		if n > 0 {
			out := make([]byte, n)
			copy(out, buf[:n])
			callback(out)
		}
		if err != nil {
			return
		}
	}
}

// Close terminates the terminal session
func (t *Terminal) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return
	}
	t.closed = true
	t.stdin.Close()
	if t.cmd.Process != nil {
		t.cmd.Process.Kill()
	}
	t.cmd.Wait()
	log.Println("Terminal closed")
}
