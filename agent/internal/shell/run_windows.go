//go:build windows

package shell

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	_MAXIMUM_ALLOWED         = 0x02000000
	_SE_PRIVILEGE_ENABLED    = 0x00000002
	_TOKEN_ADJUST_PRIVILEGES = 0x0020
	_TOKEN_QUERY             = 0x0008
)

var (
	modKernel32  = windows.NewLazySystemDLL("kernel32.dll")
	modWtsapi32  = windows.NewLazySystemDLL("wtsapi32.dll")
	modAdvapi32  = windows.NewLazySystemDLL("advapi32.dll")

	procWTSGetActiveConsoleSessionID = modKernel32.NewProc("WTSGetActiveConsoleSessionId")
	procWTSQueryUserToken            = modWtsapi32.NewProc("WTSQueryUserToken")
	procLookupPrivilegeValueW        = modAdvapi32.NewProc("LookupPrivilegeValueW")
	procAdjustTokenPrivileges        = modAdvapi32.NewProc("AdjustTokenPrivileges")
)

// runPlatform executes a PowerShell command. When opts.AsUser is true and a
// console user is logged in, the command runs as that user; otherwise it runs
// as the agent identity (SYSTEM when installed as a service).
func runPlatform(ctx context.Context, opts ExecOptions, onStarted StartedFunc, onStdout, onStderr OutputFunc) Result {
	start := time.Now()

	// Encode command as base64 UTF-16LE to avoid quoting issues with
	// PowerShell -Command parsing of complex command strings.
	encoded := encodePowerShellCommand(opts.Cmd)

	// PowerShell args (-NoProfile speeds up startup, -NonInteractive prevents prompts)
	args := []string{
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy", "Bypass",
		"-EncodedCommand", encoded,
	}

	cmd := exec.Command("powershell.exe", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	// Optional impersonation: spawn the process under the active console user.
	var userTokenToClose windows.Token
	if opts.AsUser {
		token, err := acquireUserToken()
		if err != nil {
			log.Printf("⚠️ shell: --as-user requested but token acquisition failed: %v — falling back to SYSTEM", err)
		} else {
			userTokenToClose = token
			cmd.SysProcAttr.Token = syscall.Token(token)
		}
	}
	if userTokenToClose != 0 {
		defer userTokenToClose.Close()
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return Result{Err: fmt.Errorf("stdout pipe: %w", err), DurationMs: msSince(start)}
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return Result{Err: fmt.Errorf("stderr pipe: %w", err), DurationMs: msSince(start)}
	}

	if err := cmd.Start(); err != nil {
		return Result{Err: fmt.Errorf("start: %w", err), DurationMs: msSince(start)}
	}

	pid := cmd.Process.Pid
	if onStarted != nil {
		onStarted(pid)
	}

	// Optional timeout
	var timer *time.Timer
	if opts.TimeoutSec > 0 {
		timer = time.AfterFunc(time.Duration(opts.TimeoutSec)*time.Second, func() {
			_ = cmd.Process.Kill()
		})
	}

	// Watch ctx for cancel
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
	go streamPipe(&wg, stdoutPipe, onStdout)
	go streamPipe(&wg, stderrPipe, onStderr)
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
			return Result{PID: pid, ExitCode: -1, Err: exitErr, DurationMs: msSince(start)}
		}
	}
	return Result{PID: pid, ExitCode: exitCode, DurationMs: msSince(start)}
}

func streamPipe(wg *sync.WaitGroup, r io.ReadCloser, cb OutputFunc) {
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

func msSince(t time.Time) int64 {
	return time.Since(t).Milliseconds()
}

// encodePowerShellCommand returns a base64(UTF-16LE) encoding suitable for
// `powershell.exe -EncodedCommand`.
func encodePowerShellCommand(cmd string) string {
	utf16le := utf16.Encode([]rune(cmd))
	buf := make([]byte, len(utf16le)*2)
	for i, c := range utf16le {
		binary.LittleEndian.PutUint16(buf[i*2:], c)
	}
	return base64.StdEncoding.EncodeToString(buf)
}

// acquireUserToken duplicates the token of the active console user as a
// primary token suitable for passing to syscall.SysProcAttr.Token.
func acquireUserToken() (windows.Token, error) {
	// Make sure we hold the privileges required to query/use the token.
	for _, priv := range []string{"SeTcbPrivilege", "SeAssignPrimaryTokenPrivilege", "SeIncreaseQuotaPrivilege"} {
		_ = enablePrivilege(priv) // best effort
	}

	sessionID, _, _ := procWTSGetActiveConsoleSessionID.Call()
	if sessionID == 0xFFFFFFFF {
		return 0, fmt.Errorf("no active console session")
	}

	var userToken windows.Token
	ret, _, errCall := procWTSQueryUserToken.Call(sessionID, uintptr(unsafe.Pointer(&userToken)))
	if ret == 0 {
		return 0, fmt.Errorf("WTSQueryUserToken(session=%d): %v", sessionID, errCall)
	}
	defer userToken.Close()

	var dup windows.Token
	if err := windows.DuplicateTokenEx(
		userToken,
		_MAXIMUM_ALLOWED,
		nil,
		windows.SecurityImpersonation,
		windows.TokenPrimary,
		&dup,
	); err != nil {
		return 0, fmt.Errorf("DuplicateTokenEx: %w", err)
	}
	return dup, nil
}

// enablePrivilege flips the named privilege on the current process token.
func enablePrivilege(name string) error {
	var token windows.Token
	process, _ := windows.GetCurrentProcess()
	if err := windows.OpenProcessToken(process, _TOKEN_ADJUST_PRIVILEGES|_TOKEN_QUERY, &token); err != nil {
		return err
	}
	defer token.Close()

	nameUTF16, _ := windows.UTF16PtrFromString(name)
	var luid [2]uint32
	ret, _, errCall := procLookupPrivilegeValueW.Call(0,
		uintptr(unsafe.Pointer(nameUTF16)),
		uintptr(unsafe.Pointer(&luid[0])),
	)
	if ret == 0 {
		return fmt.Errorf("LookupPrivilegeValue(%s): %v", name, errCall)
	}
	var tp [16]byte
	binary.LittleEndian.PutUint32(tp[0:4], 1)
	binary.LittleEndian.PutUint32(tp[4:8], luid[0])
	binary.LittleEndian.PutUint32(tp[8:12], luid[1])
	binary.LittleEndian.PutUint32(tp[12:16], _SE_PRIVILEGE_ENABLED)

	ret, _, errCall = procAdjustTokenPrivileges.Call(uintptr(token), 0,
		uintptr(unsafe.Pointer(&tp[0])), 0, 0, 0)
	if ret == 0 {
		return fmt.Errorf("AdjustTokenPrivileges(%s): %v", name, errCall)
	}
	return nil
}
