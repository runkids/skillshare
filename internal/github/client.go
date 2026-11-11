package github

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// RateLimitError indicates GitHub API rate limit was exceeded.
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
	msg += "; set GITHUB_TOKEN/GH_TOKEN or run 'gh auth login'"
	return msg
}

// NewClient keeps provider authorization on the initial forge API or hub origin.
func NewClient() *http.Client {
	return &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			if req.URL.Scheme != via[0].URL.Scheme || !strings.EqualFold(req.URL.Host, via[0].URL.Host) {
				req.Header.Del("Authorization")
				req.Header.Del("PRIVATE-TOKEN")
			}
			return nil
		},
	}
}

// NewRequest creates a GET request with default GitHub headers. GitHub
// credentials are attached only to its public API and raw-content HTTPS origins.
func NewRequest(url string) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")

	if req.URL.Scheme == "https" && (req.URL.Port() == "" || req.URL.Port() == "443") {
		switch strings.ToLower(req.URL.Hostname()) {
		case "api.github.com", "raw.githubusercontent.com":
			if token := GetToken(); token != "" {
				req.Header.Set("Authorization", "token "+token)
			}
		}
	}

	return req, nil
}

var (
	tokenOnce   sync.Once
	cachedToken string
)

// GetToken returns an auth token from environment variables first, then from
// `gh auth token` (cached with sync.Once).
func GetToken() string {
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token
	}
	if token := os.Getenv("GH_TOKEN"); token != "" {
		return token
	}

	tokenOnce.Do(func() {
		cmd := exec.Command("gh", "auth", "token")
		out, err := cmd.Output()
		if err != nil {
			return
		}
		cachedToken = strings.TrimSpace(string(out))
	})

	return cachedToken
}

// CheckRateLimit checks response status and returns a rate-limit typed error
// when the API has throttled the caller.
func CheckRateLimit(resp *http.Response) error {
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		return &RateLimitError{
			Limit:     resp.Header.Get("X-RateLimit-Limit"),
			Remaining: resp.Header.Get("X-RateLimit-Remaining"),
			Reset:     resp.Header.Get("X-RateLimit-Reset"),
		}
	}
	return nil
}
