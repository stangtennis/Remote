package sshconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"
)

const (
	defaultHost    = "ssh.hawkeye123.dk"
	defaultWinHost = "192.168.1.92"
	defaultUser    = "dennis"
	defaultWorkdir = "/home/dennis/projekter/aisupport"
)

// Config describes the Ubuntu host used by the controller's AI terminal.
// Only the private-key path is persisted; the key contents never enter the UI
// or controller settings file.
type Config struct {
	Enabled          bool   `json:"enabled"`
	Host             string `json:"host"`
	Port             int    `json:"port"`
	User             string `json:"user"`
	KeyPath          string `json:"key_path"`
	Workdir          string `json:"workdir"`
	CloudflareAccess bool   `json:"cloudflare_access"`
}

func Default() *Config {
	home, _ := os.UserHomeDir()
	keyPath := ""
	if home != "" {
		for _, name := range []string{"id_ed25519", "id_rsa"} {
			candidate := filepath.Join(home, ".ssh", name)
			if _, err := os.Stat(candidate); err == nil {
				keyPath = candidate
				break
			}
			if keyPath == "" {
				keyPath = candidate
			}
		}
	}
	if configuredKey := strings.TrimSpace(os.Getenv("RD_AI_SSH_KEY")); configuredKey != "" {
		keyPath = configuredKey
	}
	host := strings.TrimSpace(os.Getenv("RD_AI_SSH_HOST"))
	cloudflare := runtime.GOOS != "windows" && runtime.GOOS != "linux"
	if host == "" {
		switch runtime.GOOS {
		case "windows":
			host = defaultWinHost
		case "linux":
			host = "localhost"
		default:
			host = defaultHost
		}
	}
	user := strings.TrimSpace(os.Getenv("RD_AI_SSH_USER"))
	if user == "" {
		user = defaultUser
	}
	workdir := strings.TrimSpace(os.Getenv("RD_AI_SSH_WORKDIR"))
	if workdir == "" {
		workdir = defaultWorkdir
	}
	if configured := strings.TrimSpace(os.Getenv("RD_AI_SSH_CLOUDFLARE")); configured != "" {
		cloudflare = configured != "0" && strings.ToLower(configured) != "false" && strings.ToLower(configured) != "no"
	}
	enabled := keyPath != ""
	if _, err := os.Stat(keyPath); err != nil {
		enabled = false
	}
	return &Config{
		Enabled: enabled, Host: host, Port: 22, User: user, KeyPath: keyPath,
		Workdir: workdir, CloudflareAccess: cloudflare,
	}
}

func (c *Config) normalize() {
	if c.Port == 0 {
		c.Port = 22
	}
	if c.Workdir == "" {
		c.Workdir = defaultWorkdir
	}
	if c.KeyPath == "" {
		if defaults := Default(); defaults.KeyPath != "" {
			c.KeyPath = defaults.KeyPath
		}
	}
}

func (c *Config) Validate() error {
	if c == nil || !c.Enabled {
		return nil
	}
	c.normalize()
	if err := validateToken("SSH host", c.Host); err != nil {
		return err
	}
	if err := validateToken("SSH user", c.User); err != nil {
		return err
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("SSH port must be between 1 and 65535")
	}
	if c.Workdir == "" || !strings.HasPrefix(c.Workdir, "/") || strings.IndexFunc(c.Workdir, unicode.IsControl) >= 0 || strings.ContainsRune(c.Workdir, 0) {
		return fmt.Errorf("remote AI workdir is invalid")
	}
	if strings.TrimSpace(c.KeyPath) == "" {
		return fmt.Errorf("SSH private key path is required")
	}
	keyPath, err := ResolveKeyPath(c.KeyPath)
	if err != nil {
		return err
	}
	info, err := os.Stat(keyPath)
	if err != nil {
		return fmt.Errorf("SSH private key is unavailable: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("SSH private key path is not a regular file")
	}
	f, err := os.Open(keyPath)
	if err != nil {
		return fmt.Errorf("SSH private key is not readable: %w", err)
	}
	_ = f.Close()
	return nil
}

func validateToken(label, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s is required", label)
	}
	if strings.HasPrefix(value, "-") || strings.IndexFunc(value, unicode.IsSpace) >= 0 || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return fmt.Errorf("%s contains invalid characters", label)
	}
	return nil
}

func ResolveKeyPath(path string) (string, error) {
	path = strings.TrimSpace(os.ExpandEnv(path))
	if path == "" {
		return "", fmt.Errorf("SSH private key path is required")
	}
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot resolve SSH key home directory: %w", err)
		}
		path = filepath.Join(home, path[2:])
	}
	return filepath.Clean(path), nil
}

func configPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appDir := filepath.Join(configDir, "RemoteDesktopController")
	if err := os.MkdirAll(appDir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(appDir, "ssh.json"), nil
}

func Load() (*Config, error) {
	c := Default()
	path, err := configPath()
	if err != nil {
		return c, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			c.normalize()
			return c, nil
		}
		return c, err
	}
	if err := json.Unmarshal(data, c); err != nil {
		return Default(), err
	}
	c.normalize()
	return c, nil
}

func Save(c *Config) error {
	if c == nil {
		return fmt.Errorf("SSH configuration is nil")
	}
	c.normalize()
	if err := c.Validate(); err != nil {
		return err
	}
	path, err := configPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".ssh-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil && runtime.GOOS == "windows" {
		if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
			return err
		}
		return os.Rename(tmpPath, path)
	} else {
		return err
	}
}
