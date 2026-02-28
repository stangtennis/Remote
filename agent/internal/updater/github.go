package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// Use Caddy updates server (no auth required for auto-update)
	UpdatesBaseURL  = "https://updates.hawkeye123.dk"
	VersionCheckURL = "https://updates.hawkeye123.dk/version.json"

	// GitHub API (used for beta/prerelease channel)
	GitHubAPIBase = "https://api.github.com"
	RepoOwner     = "stangtennis"
	RepoName      = "Remote"
)

// ReleaseAsset represents a GitHub release asset
type ReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// Release represents a GitHub release
type Release struct {
	TagName     string         `json:"tag_name"`
	Name        string         `json:"name"`
	Prerelease  bool           `json:"prerelease"`
	Draft       bool           `json:"draft"`
	Assets      []ReleaseAsset `json:"assets"`
	PublishedAt string         `json:"published_at"`
}

// UpdateInfo contains information about an available update
type UpdateInfo struct {
	Version      Version
	TagName      string
	ExeURL       string
	ExeSize      int64
	SHA256URL    string // Deprecated: brug SHA256Hash i stedet
	SHA256Hash   string // Inline SHA256 hash fra version.json
	IsPrerelease bool
}

// GitHubClient handles GitHub API requests
type GitHubClient struct {
	httpClient *http.Client
	userAgent  string
}

// NewGitHubClient creates a new GitHub API client
func NewGitHubClient() *GitHubClient {
	return &GitHubClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		userAgent: "RemoteDesktop-Agent-Updater/1.0",
	}
}

// GetLatestRelease fetches the latest stable release
func (c *GitHubClient) GetLatestRelease() (*Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", GitHubAPIBase, RepoOwner, RepoName)
	return c.fetchRelease(url)
}

// GetLatestBetaRelease fetches the latest prerelease
func (c *GitHubClient) GetLatestBetaRelease() (*Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=20", GitHubAPIBase, RepoOwner, RepoName)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("failed to parse releases: %w", err)
	}

	for _, r := range releases {
		if r.Prerelease && !r.Draft {
			return &r, nil
		}
	}

	return nil, fmt.Errorf("no beta release found")
}

func (c *GitHubClient) fetchRelease(url string) (*Release, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no release found")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release: %w", err)
	}

	return &release, nil
}

// VersionInfo represents version information from Caddy server
type VersionInfo struct {
	AgentVersion      string `json:"agent_version"`
	ControllerVersion string `json:"controller_version"`
	AgentURL          string `json:"agent_url"`
	ControllerURL     string `json:"controller_url"`
	AgentSHA256       string `json:"agent_sha256,omitempty"`
}

// CheckForUpdate checks if an update is available for the agent
func (c *GitHubClient) CheckForUpdate(currentVersion string, channel string) (*UpdateInfo, error) {
	current, err := ParseVersion(currentVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to parse current version: %w", err)
	}

	// Fetch version info from Caddy downloads server
	req, err := http.NewRequest("GET", VersionCheckURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch version info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("version check returned status %d", resp.StatusCode)
	}

	var versionInfo VersionInfo
	if err := json.NewDecoder(resp.Body).Decode(&versionInfo); err != nil {
		return nil, fmt.Errorf("failed to parse version info: %w", err)
	}

	remoteVersion, err := ParseVersion(versionInfo.AgentVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to parse remote version: %w", err)
	}

	if !remoteVersion.IsNewerThan(current) {
		return nil, nil
	}

	info := &UpdateInfo{
		Version:      remoteVersion,
		TagName:      versionInfo.AgentVersion,
		ExeURL:       versionInfo.AgentURL,
		ExeSize:      0, // Size will be determined during download
		SHA256Hash:   versionInfo.AgentSHA256,
		IsPrerelease: false,
	}

	return info, nil
}

// FetchVersionInfo fetches version info from the update server
// Returns the full VersionInfo so callers can display both agent and controller versions
func (c *GitHubClient) FetchVersionInfo() (*VersionInfo, error) {
	req, err := http.NewRequest("GET", VersionCheckURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch version info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("version check returned status %d", resp.StatusCode)
	}

	var versionInfo VersionInfo
	if err := json.NewDecoder(resp.Body).Decode(&versionInfo); err != nil {
		return nil, fmt.Errorf("failed to parse version info: %w", err)
	}

	return &versionInfo, nil
}

// DownloadSHA256 downloads and parses a SHA256 checksum file
func (c *GitHubClient) DownloadSHA256(url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download SHA256: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download SHA256: status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	line := strings.TrimSpace(string(data))
	parts := strings.Fields(line)
	if len(parts) < 1 {
		return "", fmt.Errorf("invalid SHA256 file format")
	}

	hash := strings.ToLower(parts[0])
	if len(hash) != 64 {
		return "", fmt.Errorf("invalid SHA256 hash length: %d", len(hash))
	}

	return hash, nil
}
