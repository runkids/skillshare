package install

import "strings"

func isGitHubLikeHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return strings.Contains(host, "github") || strings.HasSuffix(host, ".ghe.com")
}

func gitHubAPIBaseForHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "github.com" {
		return "https://api.github.com"
	}
	if strings.HasSuffix(host, ".ghe.com") {
		return "https://api." + host
	}
	return "https://" + host + "/api/v3"
}
