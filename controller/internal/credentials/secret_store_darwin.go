//go:build darwin

package credentials

import (
	"bytes"
	"os/exec"
	"strings"
)

func saveSecret(key, value string) error {
	cmd := exec.Command("/usr/bin/security", "add-generic-password", "-U", "-s", secretServiceName, "-a", key, "-w", value)
	return cmd.Run()
}

func loadSecret(key string) (string, error) {
	cmd := exec.Command("/usr/bin/security", "find-generic-password", "-s", secretServiceName, "-a", key, "-w")
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && bytes.Contains(exitErr.Stderr, []byte("could not be found")) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimRight(string(out), "\r\n"), nil
}

func deleteSecret(key string) error {
	cmd := exec.Command("/usr/bin/security", "delete-generic-password", "-s", secretServiceName, "-a", key)
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && bytes.Contains(exitErr.Stderr, []byte("could not be found")) {
			return nil
		}
		return err
	}
	return nil
}
