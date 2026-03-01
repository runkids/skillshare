package version

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"

	ghclient "skillshare/internal/github"
)

// RateLimitError is an alias for the canonical rate limit error type.
type RateLimitError = ghclient.RateLimitError

// Release holds GitHub release information
type Release struct {
	TagName string  `json:"tag_name"`
	Version string  // Parsed from TagName (without 'v' prefix)
	Assets  []Asset `json:"assets"`
}

// Asset holds GitHub release asset information
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// GetDownloadURL returns the download URL for the current platform
func (r *Release) GetDownloadURL() (string, error) {
	return BuildDownloadURL(r.Version)
}

// BuildDownloadURL constructs download URL from version string
func BuildDownloadURL(version string) (string, error) {
	osName := runtime.GOOS
	archName := runtime.GOARCH

	// Windows uses .zip, others use .tar.gz
	ext := "tar.gz"
	if osName == "windows" {
		ext = "zip"
	}

	// Construct URL directly (avoids needing asset list from API)
	filename := fmt.Sprintf("skillshare_%s_%s_%s.%s", version, osName, archName, ext)
	url := fmt.Sprintf("https://github.com/%s/releases/download/v%s/%s", githubRepo, version, filename)
	return url, nil
}

// BuildUIDistURL constructs the direct download URL for the UI dist tarball
func BuildUIDistURL(ver string) string {
	return fmt.Sprintf("https://github.com/%s/releases/download/v%s/skillshare-ui-dist.tar.gz", githubRepo, ver)
}

// BuildChecksumsURL constructs the direct download URL for the checksums file
func BuildChecksumsURL(ver string) string {
	return fmt.Sprintf("https://github.com/%s/releases/download/v%s/checksums.txt", githubRepo, ver)
}

// FetchLatestRelease fetches the latest release from GitHub with full asset info.
// Uses the centralized github client which supports GITHUB_TOKEN, GH_TOKEN,
// and `gh auth token` fallback.
func FetchLatestRelease() (*Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", githubRepo)

	req, err := ghclient.NewRequest(url)
	if err != nil {
		return nil, err
	}

	client := ghclient.NewClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	if err := ghclient.CheckRateLimit(resp); err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	release.Version = strings.TrimPrefix(release.TagName, "v")
	return &release, nil
}

// FetchLatestVersionOnly fetches just the version string (for background checks)
func FetchLatestVersionOnly() (string, error) {
	release, err := FetchLatestRelease()
	if err != nil {
		return "", err
	}
	return release.Version, nil
}
