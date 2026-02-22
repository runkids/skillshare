package search

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"skillshare/internal/install"
)

type indexDocument struct {
	SourcePath string       `json:"sourcePath"`
	Skills     []indexSkill `json:"skills"`
}

type indexSkill struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Source      string   `json:"source"`
	Skill       string   `json:"skill"`
	Tags        []string `json:"tags"`
	RiskScore   *int     `json:"riskScore,omitempty"`
	RiskLabel   string   `json:"riskLabel,omitempty"`
}

// SearchFromIndexURL searches skills from a private index.json URL or local path.
// A limit of 0 means no limit (return all results).
func SearchFromIndexURL(query string, limit int, indexURL string) ([]SearchResult, error) {
	doc, err := loadIndex(indexURL)
	if err != nil {
		return nil, err
	}
	return searchIndex(query, limit, doc)
}

// SearchFromIndexJSON searches skills from raw index JSON data.
// Used by the server to search an in-memory index without file I/O.
// A limit of 0 means no limit (return all results).
func SearchFromIndexJSON(query string, limit int, data []byte) ([]SearchResult, error) {
	var doc indexDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse hub: %w", err)
	}
	return searchIndex(query, limit, &doc)
}

func searchIndex(query string, limit int, doc *indexDocument) ([]SearchResult, error) {
	sourcePath := strings.TrimSpace(doc.SourcePath)

	q := strings.ToLower(strings.TrimSpace(query))
	results := make([]SearchResult, 0, len(doc.Skills))
	for _, it := range doc.Skills {
		name := strings.TrimSpace(it.Name)
		source := strings.TrimSpace(it.Source)
		if name == "" {
			continue
		}
		if source == "" {
			source = name
		}

		// Resolve relative source paths using sourcePath from the index.
		// A relative source (e.g. "team/skill") would otherwise be misinterpreted
		// as a GitHub shorthand. Joining with sourcePath produces an absolute
		// local path that ParseSource handles correctly.
		if sourcePath != "" && isRelativeSource(source) {
			source = filepath.Join(sourcePath, source)
		}

		if q != "" {
			hay := strings.ToLower(name + "\n" + it.Description + "\n" + source + "\n" + strings.Join(it.Tags, " "))
			if !strings.Contains(hay, q) {
				continue
			}
		}
		owner, repo := parseOwnerRepo(source)
		results = append(results, SearchResult{
			Name:        name,
			Description: strings.TrimSpace(it.Description),
			Source:      source,
			Skill:       strings.TrimSpace(it.Skill),
			Tags:        it.Tags,
			Owner:       owner,
			Repo:        repo,
			RiskScore:   it.RiskScore,
			RiskLabel:   it.RiskLabel,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// isRelativeSource returns true if the source looks like a relative path
// rather than a remote URL or absolute path.
func isRelativeSource(source string) bool {
	if strings.HasPrefix(source, "/") ||
		strings.HasPrefix(source, "~") ||
		strings.HasPrefix(source, "git@") ||
		strings.HasPrefix(source, "http://") ||
		strings.HasPrefix(source, "https://") ||
		strings.HasPrefix(source, "file://") {
		return false
	}
	// Windows absolute paths: C:\ or C:/
	if len(source) >= 3 && source[1] == ':' &&
		((source[0] >= 'A' && source[0] <= 'Z') || (source[0] >= 'a' && source[0] <= 'z')) &&
		(source[2] == '/' || source[2] == '\\') {
		return false
	}
	// Windows UNC paths: \\server\share
	if strings.HasPrefix(source, `\\`) {
		return false
	}
	// If the first path segment contains a dot, it's a domain (e.g. gitlab.com/...)
	firstSlash := strings.Index(source, "/")
	if firstSlash > 0 && strings.Contains(source[:firstSlash], ".") {
		return false
	}
	return true
}

func loadIndex(indexURL string) (*indexDocument, error) {
	s := strings.TrimSpace(indexURL)
	if s == "" {
		return nil, fmt.Errorf("hub URL is required")
	}

	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		client := &http.Client{Timeout: 15 * time.Second}
		req, err := buildHubRequest(s)
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetch hub: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, hubHTTPError(req.URL.String(), resp.StatusCode)
		}
		var doc indexDocument
		if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
			return nil, fmt.Errorf("parse hub: %w", err)
		}
		return &doc, nil
	}

	rawPath := strings.TrimPrefix(s, "file://")
	data, err := os.ReadFile(rawPath)
	if err != nil {
		return nil, err
	}
	var doc indexDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse hub: %w", err)
	}
	return &doc, nil
}

// normalizeHubURL rewrites common web file-view URLs to raw-content URLs.
func normalizeHubURL(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return rawURL
	}

	host := strings.ToLower(u.Hostname())
	if strings.Contains(host, "gitlab") {
		// GitLab web URLs for private repos commonly redirect to sign-in. Convert
		// to API raw endpoint so PAT auth headers work reliably.
		if apiURL, ok := gitLabWebToAPIRawURL(u); ok {
			return apiURL
		}
		if strings.Contains(u.Path, "/-/blob/") {
			u.Path = strings.Replace(u.Path, "/-/blob/", "/-/raw/", 1)
			return u.String()
		}
	}

	// Bitbucket web view URL:
	// https://bitbucket.org/<workspace>/<repo>/src/<ref>/<path>
	// -> https://bitbucket.org/<workspace>/<repo>/raw/<ref>/<path>
	if strings.Contains(host, "bitbucket") {
		parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
		if len(parts) >= 5 && parts[2] == "src" {
			parts[2] = "raw"
			u.Path = "/" + strings.Join(parts, "/")
			return u.String()
		}
	}

	// GitHub blob URL:
	// https://github.com/<owner>/<repo>/blob/<ref>/<path>
	// -> https://raw.githubusercontent.com/<owner>/<repo>/<ref>/<path>
	if host == "github.com" {
		parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
		if len(parts) >= 5 && parts[2] == "blob" {
			return (&url.URL{
				Scheme: "https",
				Host:   "raw.githubusercontent.com",
				Path:   "/" + parts[0] + "/" + parts[1] + "/" + strings.Join(parts[3:], "/"),
			}).String()
		}
	}

	return rawURL
}

// gitLabWebToAPIRawURL converts:
//
//	https://gitlab.com/<project>/-/(blob|raw)/<ref>/<file>
//
// to:
//
//	https://gitlab.com/api/v4/projects/<project-escaped>/repository/files/<file-escaped>/raw?ref=<ref>
func gitLabWebToAPIRawURL(u *url.URL) (string, bool) {
	path := u.Path
	marker := "/-/blob/"
	if !strings.Contains(path, marker) {
		marker = "/-/raw/"
		if !strings.Contains(path, marker) {
			return "", false
		}
	}

	parts := strings.SplitN(path, marker, 2)
	if len(parts) != 2 {
		return "", false
	}
	projectPath := strings.Trim(parts[0], "/")
	rest := strings.TrimPrefix(parts[1], "/")
	slash := strings.Index(rest, "/")
	if projectPath == "" || slash <= 0 || slash >= len(rest)-1 {
		return "", false
	}
	ref := rest[:slash]
	filePath := rest[slash+1:]

	return fmt.Sprintf(
		"%s://%s/api/v4/projects/%s/repository/files/%s/raw?ref=%s",
		u.Scheme,
		u.Host,
		url.PathEscape(projectPath),
		url.PathEscape(filePath),
		url.QueryEscape(ref),
	), true
}

func buildHubRequest(indexURL string) (*http.Request, error) {
	req, err := http.NewRequest("GET", normalizeHubURL(indexURL), nil)
	if err != nil {
		return nil, err
	}
	applyHubAuthHeaders(req)
	return req, nil
}

func applyHubAuthHeaders(req *http.Request) {
	token, username := install.ResolveTokenForURL(req.URL.String())
	if token == "" {
		return
	}

	switch install.DetectPlatformForURL(req.URL.String()) {
	case install.PlatformGitLab:
		req.Header.Set("PRIVATE-TOKEN", token)
		req.Header.Set("Authorization", "Bearer "+token)
	case install.PlatformGitHub:
		req.Header.Set("Authorization", "Bearer "+token)
	case install.PlatformBitbucket:
		if username == "" {
			username = "x-token-auth"
		}
		req.SetBasicAuth(username, token)
	default:
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

func hubHTTPError(indexURL string, status int) error {
	if status != http.StatusUnauthorized && status != http.StatusForbidden {
		return fmt.Errorf("fetch hub: HTTP %d", status)
	}

	switch install.DetectPlatformForURL(indexURL) {
	case install.PlatformGitLab:
		return fmt.Errorf("fetch hub: HTTP %d (authentication required; set GITLAB_TOKEN or SKILLSHARE_GIT_TOKEN)", status)
	case install.PlatformGitHub:
		return fmt.Errorf("fetch hub: HTTP %d (authentication required; set GITHUB_TOKEN or SKILLSHARE_GIT_TOKEN)", status)
	case install.PlatformBitbucket:
		return fmt.Errorf("fetch hub: HTTP %d (authentication required; set BITBUCKET_TOKEN or SKILLSHARE_GIT_TOKEN)", status)
	default:
		return fmt.Errorf("fetch hub: HTTP %d (authentication required)", status)
	}
}

func parseOwnerRepo(source string) (owner, repo string) {
	s := strings.TrimPrefix(source, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "github.com/")
	parts := strings.Split(s, "/")
	if len(parts) >= 2 {
		return parts[0], parts[1]
	}
	return "", ""
}
