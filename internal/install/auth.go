package install

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// Platform represents a git hosting platform.
type Platform int

const (
	PlatformUnknown     Platform = iota
	PlatformGitHub               // github.com and GitHub Enterprise
	PlatformGitLab               // gitlab.com and self-hosted GitLab
	PlatformBitbucket            // bitbucket.org
	PlatformAzureDevOps          // dev.azure.com and visualstudio.com
)

// extractHost returns the hostname from a clone URL.
// Supports HTTPS (https://host/...) and SSH (git@host:...) formats.
func extractHost(cloneURL string) string {
	s := strings.TrimSpace(cloneURL)
	if s == "" {
		return ""
	}

	// SSH: git@host:owner/repo.git
	if strings.Contains(s, "@") && strings.Contains(s, ":") && !strings.Contains(s, "://") {
		at := strings.Index(s, "@")
		colon := strings.Index(s[at:], ":")
		if colon > 0 {
			return s[at+1 : at+colon]
		}
	}

	// HTTPS or file://
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Hostname()
}

// detectPlatform identifies the git hosting platform from a clone URL.
func detectPlatform(cloneURL string) Platform {
	host := strings.ToLower(extractHost(cloneURL))
	if host == "" {
		return PlatformUnknown
	}
	if strings.Contains(host, "github") {
		return PlatformGitHub
	}
	if strings.Contains(host, "gitlab") {
		return PlatformGitLab
	}
	if strings.Contains(host, "bitbucket") {
		return PlatformBitbucket
	}
	if host == "dev.azure.com" || host == "ssh.dev.azure.com" || strings.HasSuffix(host, ".visualstudio.com") {
		return PlatformAzureDevOps
	}
	return PlatformUnknown
}

// resolveToken looks up a token from environment variables based on the
// detected platform. Platform-specific vars take priority over the generic
// SKILLSHARE_GIT_TOKEN. Returns empty strings if no token is available or
// the URL is not HTTPS.
func resolveToken(cloneURL string) (token, username string) {
	if !isHTTPS(cloneURL) {
		return "", ""
	}

	platform := detectPlatform(cloneURL)
	switch platform {
	case PlatformGitHub:
		if t := os.Getenv("GITHUB_TOKEN"); t != "" {
			return t, "x-access-token"
		}
		if t := os.Getenv("GH_TOKEN"); t != "" {
			return t, "x-access-token"
		}
	case PlatformGitLab:
		if t := os.Getenv("GITLAB_TOKEN"); t != "" {
			return t, "oauth2"
		}
	case PlatformBitbucket:
		if t := os.Getenv("BITBUCKET_TOKEN"); t != "" {
			if u := os.Getenv("BITBUCKET_USERNAME"); u != "" {
				return t, u
			}
			return t, "x-token-auth"
		}
	case PlatformAzureDevOps:
		if t := os.Getenv("AZURE_DEVOPS_TOKEN"); t != "" {
			return t, "x-access-token"
		}
	}

	// Generic fallback — use platform-appropriate username, or preserve
	// existing URL username (e.g. https://myuser@host/... → myuser:token@...).
	if t := os.Getenv("SKILLSHARE_GIT_TOKEN"); t != "" {
		if u := urlUsername(cloneURL); u != "" {
			return t, u
		}
		switch platform {
		case PlatformGitLab:
			return t, "oauth2"
		case PlatformBitbucket:
			return t, "x-token-auth"
		default:
			return t, "x-access-token"
		}
	}
	return "", ""
}

// authEnv returns environment variables that inject token authentication
// via GIT_CONFIG_COUNT/KEY/VALUE (Git 2.31+). This avoids the git -c
// key=value format which breaks when tokens contain '=' characters.
// If GIT_CONFIG_COUNT is already set in the environment, the new entry
// is appended at the next available index to avoid overriding existing
// git config entries (e.g. from CI pipelines).
// Returns nil for SSH/file URLs or when no token is available.
func authEnv(cloneURL string) []string {
	token, username := resolveToken(cloneURL)
	if token == "" {
		return nil
	}

	host := extractHost(cloneURL)
	if host == "" {
		return nil
	}

	base := existingConfigCount()
	authed := fmt.Sprintf("https://%s:%s@%s/", username, token, host)
	original := originalPrefix(cloneURL, host)
	return []string{
		fmt.Sprintf("GIT_CONFIG_COUNT=%d", base+1),
		fmt.Sprintf("GIT_CONFIG_KEY_%d=url.%s.insteadOf", base, authed),
		fmt.Sprintf("GIT_CONFIG_VALUE_%d=%s", base, original),
	}
}

// AuthEnvForURL returns git auth injection env for HTTPS clone/pull URLs.
// Returns nil when no token is available or URL is not HTTPS.
func AuthEnvForURL(cloneURL string) []string {
	return authEnv(cloneURL)
}

// ResolveTokenForURL returns token credentials resolved from env vars for the
// provided HTTPS URL. Returns empty strings when no token is available.
func ResolveTokenForURL(cloneURL string) (token, username string) {
	return resolveToken(cloneURL)
}

// DetectPlatformForURL identifies the git hosting platform for a URL.
func DetectPlatformForURL(cloneURL string) Platform {
	return detectPlatform(cloneURL)
}

// existingConfigCount returns the current GIT_CONFIG_COUNT from the
// environment, or 0 if unset/invalid.
func existingConfigCount() int {
	s := os.Getenv("GIT_CONFIG_COUNT")
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// originalPrefix returns the URL prefix that git's insteadOf should match.
// If the URL already contains a username (e.g. https://user@host/...),
// the prefix includes it so the rewrite rule matches the actual URL.
func originalPrefix(cloneURL, host string) string {
	u, err := url.Parse(strings.TrimSpace(cloneURL))
	if err == nil && u.User != nil && u.User.Username() != "" {
		return fmt.Sprintf("https://%s@%s/", u.User.Username(), host)
	}
	return fmt.Sprintf("https://%s/", host)
}

// sanitizeTokens replaces any known credential values found in text with
// [REDACTED]. This prevents tokens and associated usernames from leaking
// in error messages.
func sanitizeTokens(text string) string {
	vars := []string{
		"GITHUB_TOKEN", "GH_TOKEN", "GITLAB_TOKEN", "BITBUCKET_TOKEN",
		"AZURE_DEVOPS_TOKEN", "SKILLSHARE_GIT_TOKEN", "BITBUCKET_USERNAME",
	}
	for _, v := range vars {
		if t := os.Getenv(v); t != "" {
			text = strings.ReplaceAll(text, t, "[REDACTED]")
		}
	}
	return text
}

// urlUsername extracts the username from an HTTPS URL, if present.
// Returns "" for URLs without userinfo (e.g. https://github.com/...).
func urlUsername(cloneURL string) string {
	u, err := url.Parse(strings.TrimSpace(cloneURL))
	if err != nil || u.User == nil {
		return ""
	}
	return u.User.Username()
}

// isHTTPS returns true if the URL uses the HTTPS scheme.
func isHTTPS(cloneURL string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(cloneURL)), "https://")
}

// IsAuthError returns true when git stderr indicates an authentication failure.
func IsAuthError(output string) bool {
	return strings.Contains(output, "Authentication failed") ||
		strings.Contains(output, "Access denied") ||
		strings.Contains(output, "could not read Username") ||
		strings.Contains(output, "terminal prompts disabled")
}
