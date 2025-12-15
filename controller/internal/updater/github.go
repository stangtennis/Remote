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
	TagName    string         `json:"tag_name"`
	Name       string         `json:"name"`
	Prerelease bool           `json:"prerelease"`
	Draft      bool           `json:"draft"`
	Assets     []ReleaseAsset `json:"assets"`
	PublishedAt string        `json:"published_at"`
}

// UpdateInfo contains information about an available update
type UpdateInfo struct {
	Version     Version
	TagName     string
	ExeURL      string
	ExeSize     int64
	SHA256URL   string
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
		userAgent: "RemoteDesktop-Updater/1.0",
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

	// Find first prerelease
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

// CheckForUpdate checks if an update is available for the given app
// appType: "controller" or "agent"
// channel: "stable" or "beta"
func (c *GitHubClient) CheckForUpdate(currentVersion string, appType string, channel string) (*UpdateInfo, error) {
	current, err := ParseVersion(currentVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to parse current version: %w", err)
	}

	var release *Release
	if channel == "beta" {
		release, err = c.GetLatestBetaRelease()
	} else {
		release, err = c.GetLatestRelease()
	}
	if err != nil {
		return nil, err
	}

	remoteVersion, err := ParseVersion(release.TagName)
	if err != nil {
		return nil, fmt.Errorf("failed to parse remote version: %w", err)
	}

	// No update if current is same or newer
	if !remoteVersion.IsNewerThan(current) {
		return nil, nil
	}

	// Find the correct assets
	var exeAsset, sha256Asset *ReleaseAsset
	exePattern := fmt.Sprintf("%s-%s.exe", appType, release.TagName)
	sha256Pattern := fmt.Sprintf("%s-%s.exe.sha256", appType, release.TagName)

	for i := range release.Assets {
		asset := &release.Assets[i]
		if strings.EqualFold(asset.Name, exePattern) {
			exeAsset = asset
		} else if strings.EqualFold(asset.Name, sha256Pattern) {
			sha256Asset = asset
		}
	}

	if exeAsset == nil {
		return nil, fmt.Errorf("exe asset not found for %s %s", appType, release.TagName)
	}

	info := &UpdateInfo{
		Version:      remoteVersion,
		TagName:      release.TagName,
		ExeURL:       exeAsset.BrowserDownloadURL,
		ExeSize:      exeAsset.Size,
		IsPrerelease: release.Prerelease,
	}

	if sha256Asset != nil {
		info.SHA256URL = sha256Asset.BrowserDownloadURL
	}

	return info, nil
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

	// Parse format: "<hash>  <filename>" or "<hash> <filename>"
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
