package version

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

// RateLimitError indicates GitHub API rate limit was exceeded
type RateLimitError struct {
	Limit     string
	Remaining string
	Reset     string
}

func (e *RateLimitError) Error() string {
	msg := "GitHub API rate limit exceeded"
	if e.Remaining == "0" {
		msg += fmt.Sprintf(" (0/%s remaining)", e.Limit)
	}
	msg += "\n\nTo fix this, either:\n"
	msg += "  1. Wait for rate limit to reset\n"
	msg += "  2. Set GITHUB_TOKEN environment variable for higher limits (5000/hr)\n"
	msg += "     export GITHUB_TOKEN=ghp_your_token_here"
	return msg
}

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
	osName := runtime.GOOS
	archName := runtime.GOARCH

	// Windows uses .zip, others use .tar.gz
	ext := "tar.gz"
	if osName == "windows" {
		ext = "zip"
	}
	expectedName := fmt.Sprintf("skillshare_%s_%s_%s.%s", r.Version, osName, archName, ext)

	for _, asset := range r.Assets {
		if asset.Name == expectedName {
			return asset.BrowserDownloadURL, nil
		}
	}

	return "", fmt.Errorf("no release found for %s/%s", osName, archName)
}

// newGitHubClient creates an HTTP client with optional auth
func newGitHubClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

// newGitHubRequest creates a request with auth header if token is available
func newGitHubRequest(url string) (*http.Request, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")

	// Check for GitHub token in environment
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "token "+token)
	} else if token := os.Getenv("GH_TOKEN"); token != "" {
		// gh CLI uses GH_TOKEN
		req.Header.Set("Authorization", "token "+token)
	}

	return req, nil
}

// checkRateLimit checks response for rate limit errors
func checkRateLimit(resp *http.Response) error {
	if resp.StatusCode == 403 || resp.StatusCode == 429 {
		return &RateLimitError{
			Limit:     resp.Header.Get("X-RateLimit-Limit"),
			Remaining: resp.Header.Get("X-RateLimit-Remaining"),
			Reset:     resp.Header.Get("X-RateLimit-Reset"),
		}
	}
	return nil
}

// FetchLatestRelease fetches the latest release from GitHub with full asset info
func FetchLatestRelease() (*Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", githubRepo)

	req, err := newGitHubRequest(url)
	if err != nil {
		return nil, err
	}

	client := newGitHubClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	// Check for rate limiting
	if err := checkRateLimit(resp); err != nil {
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
