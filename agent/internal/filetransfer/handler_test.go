package filetransfer

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSanitizePath_RejectsEmpty(t *testing.T) {
	if _, err := sanitizePath(""); err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestSanitizePath_RejectsTraversal(t *testing.T) {
	cases := []string{
		"../secret",
		"foo/../../etc/passwd",
		filepath.Join("..", "..", "escape"),
	}
	for _, p := range cases {
		if _, err := sanitizePath(p); err == nil {
			t.Errorf("expected error for traversal path: %s", p)
		}
	}
}

func TestSanitizePath_RejectsUNCAndDeviceNamespace(t *testing.T) {
	cases := []string{
		`\\server\share\file`,
		`\\?\C:\Windows\System32`,
		`\\.\C:\raw`,
		`//server/share/file`,
	}
	for _, p := range cases {
		if _, err := sanitizePath(p); err == nil {
			t.Errorf("expected error for UNC/device path: %s", p)
		}
	}
}

func TestSanitizePath_AcceptsClean(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "nested", "file.txt")
	if _, err := sanitizePath(p); err != nil {
		t.Errorf("unexpected error for clean path %s: %v", p, err)
	}
}

func TestSanitizePath_ResolvesSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink test skipped on Windows (needs privileges)")
	}
	target := t.TempDir()
	base := t.TempDir()
	link := filepath.Join(base, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}
	resolved, err := sanitizePath(link)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved != target {
		t.Errorf("expected symlink to resolve to %s, got %s", target, resolved)
	}
}
