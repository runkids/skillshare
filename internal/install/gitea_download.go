package install

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type giteaContentItem struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	DownloadURL string `json:"download_url"`
}

type cnbContentItem struct {
	Type    string           `json:"type"`
	Name    string           `json:"name"`
	Path    string           `json:"path"`
	Content string           `json:"content"`
	Entries []cnbContentItem `json:"entries"`
}

type giteaCommit struct {
	SHA string `json:"sha"`
}

// isGiteaAPISource reports whether the source is a Gitea instance that supports
// the Contents API for direct file downloads.
func isGiteaAPISource(source *Source) bool {
	if source == nil {
		return false
	}
	host := strings.ToLower(extractHost(source.CloneURL))
	return isGiteaHost(host, source.GiteaHosts)
}

// isCNBAPISource reports whether the source is a CNB instance that supports
// the git contents API documented in bin/cnb-swagger.json.
func isCNBAPISource(source *Source) bool {
	if source == nil {
		return false
	}
	host := strings.ToLower(extractHost(source.CloneURL))
	return isCNBHost(host, source.CNBHosts)
}

// downloadGiteaDir downloads a repository subdirectory via the Gitea Contents API.
func downloadGiteaDir(owner, repo, path, destDir string, source *Source, onProgress ProgressCallback) (string, error) {
	if owner == "" || repo == "" {
		return "", fmt.Errorf("gitea download requires owner and repo")
	}

	apiBase := giteaAPIBase(source)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", err
	}

	if onProgress != nil {
		onProgress("Downloading via Gitea API...")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	if err := giteaDownloadDirRecursive(client, apiBase, owner, repo, strings.Trim(path, "/"), destDir, onProgress); err != nil {
		return "", err
	}

	commitHash, err := giteaFetchLatestCommitHash(apiBase, owner, repo, source)
	if err != nil {
		return "", nil
	}
	return shortHash(commitHash), nil
}

// giteaAPIBase returns the base API URL for a Gitea instance.
func giteaAPIBase(source *Source) string {
	host := strings.ToLower(extractHost(source.CloneURL))
	scheme := "https"
	// For standard gitea.com, use api.gitea.com convention
	// For self-hosted, use https://{host}/api/v1
	if host == "gitea.com" {
		return "https://gitea.com/api/v1"
	}
	return fmt.Sprintf("%s://%s/api/v1", scheme, host)
}

// giteaDownloadDirRecursive recursively downloads a directory via the Gitea Contents API.
func giteaDownloadDirRecursive(client *http.Client, apiBase, owner, repo, path, destDir string, onProgress ProgressCallback) error {
	contentsURL := buildGiteaContentsURL(apiBase, owner, repo, path)

	req, err := giteaNewRequest(contentsURL)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Gitea API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Gitea contents API returned %d for %s", resp.StatusCode, path)
	}

	var raw json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return fmt.Errorf("failed to parse Gitea contents response: %w", err)
	}

	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return fmt.Errorf("empty Gitea contents response for %q", path)
	}

	// Single file response: {type, name, path, download_url, ...}
	if trimmed[0] == '{' {
		var item giteaContentItem
		if err := json.Unmarshal(trimmed, &item); err != nil {
			return err
		}
		if item.Type != "file" {
			return fmt.Errorf("unsupported Gitea content type %q", item.Type)
		}
		fileName, err := giteaSanitizeName(item.Name)
		if err != nil {
			return err
		}
		target := filepath.Join(destDir, fileName)
		if onProgress != nil {
			onProgress(fmt.Sprintf("Downloading %s", item.Path))
		}
		return giteaDownloadFile(client, item.DownloadURL, target)
	}

	// Directory listing response: [{type, name, path, download_url, ...}]
	if trimmed[0] == '[' {
		var items []giteaContentItem
		if err := json.Unmarshal(trimmed, &items); err != nil {
			return err
		}
		for _, item := range items {
			name, err := giteaSanitizeName(item.Name)
			if err != nil {
				return err
			}
			switch item.Type {
			case "dir":
				childDir := filepath.Join(destDir, name)
				if err := os.MkdirAll(childDir, 0755); err != nil {
					return err
				}
				if err := giteaDownloadDirRecursive(client, apiBase, owner, repo, item.Path, childDir, onProgress); err != nil {
					return err
				}
			case "file":
				target := filepath.Join(destDir, name)
				if onProgress != nil {
					onProgress(fmt.Sprintf("Downloading %s", item.Path))
				}
				if err := giteaDownloadFile(client, item.DownloadURL, target); err != nil {
					return err
				}
			}
		}
		return nil
	}

	return fmt.Errorf("unexpected Gitea contents payload for %q", path)
}

// giteaDownloadFile downloads a single file from a URL.
func giteaDownloadFile(client *http.Client, fileURL, destPath string) error {
	req, err := giteaNewRequest(fileURL)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(destPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

// giteaNewRequest creates a GET request with Gitea API headers and optional
// token authentication (GITEA_TOKEN or platform-resolved token).
func giteaNewRequest(reqURL string) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")

	// Try GITEA_TOKEN first, then SKILLSHARE_GIT_TOKEN
	token := os.Getenv("GITEA_TOKEN")
	if token == "" {
		token = os.Getenv("SKILLSHARE_GIT_TOKEN")
	}
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	return req, nil
}

// giteaFetchLatestCommitHash retrieves the latest commit SHA from a Gitea repo.
func giteaFetchLatestCommitHash(apiBase, owner, repo string, source *Source) (string, error) {
	commitsURL := fmt.Sprintf("%s/repos/%s/%s/commits?per_page=1",
		strings.TrimRight(apiBase, "/"), url.PathEscape(owner), url.PathEscape(repo))

	req, err := giteaNewRequest(commitsURL)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var commits []giteaCommit
		if decodeErr := json.NewDecoder(resp.Body).Decode(&commits); decodeErr == nil && len(commits) > 0 {
			return commits[0].SHA, nil
		}
	}

	// Fallback: use git ls-remote
	if source != nil && source.CloneURL != "" {
		return getRemoteHeadCommit(source.CloneURL)
	}

	return "", fmt.Errorf("failed to fetch latest commit hash")
}

// giteaSanitizeName validates a Gitea file/directory name.
func giteaSanitizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return "", fmt.Errorf("invalid Gitea item name %q", name)
	}
	return name, nil
}

// giteaOwnerRepo extracts the owner and repo name from a Gitea clone URL.
// Clone URLs are in the format: https://host/owner/repo.git or git@host:owner/repo.git
func giteaOwnerRepo(cloneURL string) (owner, repo string) {
	u := strings.TrimSpace(cloneURL)
	u = strings.TrimSuffix(u, ".git")
	u = strings.TrimSuffix(u, "/")

	// SSH: git@host:owner/repo
	if strings.HasPrefix(u, "git@") {
		colon := strings.LastIndex(u, ":")
		if colon != -1 {
			segments := strings.Split(strings.Trim(u[colon+1:], "/"), "/")
			if len(segments) >= 2 {
				return segments[0], strings.TrimSuffix(segments[1], ".git")
			}
		}
		return "", ""
	}

	// HTTPS: https://host/owner/repo
	parsed, err := url.Parse(u)
	if err != nil {
		return "", ""
	}
	path := strings.Trim(parsed.Path, "/")
	segments := strings.Split(path, "/")
	if len(segments) >= 2 {
		return segments[0], strings.TrimSuffix(segments[1], ".git")
	}
	return "", ""
}

// escapeGiteaPath escapes each path segment individually for the Gitea Contents API.
// This preserves directory separators while encoding special characters in each segment.
func escapeGiteaPath(path string) string {
	parts := strings.Split(path, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

// downloadCNBDir downloads a repository subdirectory via the CNB git contents API.
func downloadCNBDir(repo, path, destDir string, source *Source, onProgress ProgressCallback) (string, error) {
	if repo == "" {
		return "", fmt.Errorf("cnb download requires repo")
	}
	apiBase := cnbAPIBase(source)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", err
	}
	if onProgress != nil {
		onProgress("Downloading via CNB API...")
	}
	client := &http.Client{Timeout: 30 * time.Second}
	if err := cnbDownloadDirRecursive(client, apiBase, repo, strings.Trim(path, "/"), destDir, onProgress); err != nil {
		return "", err
	}
	commitHash, err := cnbFetchLatestCommitHash(apiBase, repo, source)
	if err != nil {
		return "", nil
	}
	return shortCommitHash(commitHash), nil
}

func cnbAPIBase(source *Source) string {
	host := strings.ToLower(extractHost(source.CloneURL))
	return fmt.Sprintf("https://%s", host)
}

func cnbDownloadDirRecursive(client *http.Client, apiBase, repo, path, destDir string, onProgress ProgressCallback) error {
	contentsURL := buildCNBContentsURL(apiBase, repo, path)
	req, err := cnbNewRequest(contentsURL)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("CNB API request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("CNB contents API returned %d for %s", resp.StatusCode, path)
	}
	var item cnbContentItem
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return fmt.Errorf("failed to parse CNB contents response: %w", err)
	}
	return cnbWriteContentItem(client, apiBase, repo, item, destDir, onProgress)
}

func cnbWriteContentItem(client *http.Client, apiBase, repo string, item cnbContentItem, destDir string, onProgress ProgressCallback) error {
	switch item.Type {
	case "tree":
		entries := item.Entries
		for _, entry := range entries {
			name, err := giteaSanitizeName(entry.Name)
			if err != nil {
				return err
			}
			target := filepath.Join(destDir, name)
			switch entry.Type {
			case "tree":
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
				if err := cnbDownloadDirRecursive(client, apiBase, repo, entry.Path, target, onProgress); err != nil {
					return err
				}
			case "blob":
				if onProgress != nil {
					onProgress(fmt.Sprintf("Downloading %s", entry.Path))
				}
				if err := cnbDownloadFile(client, apiBase, repo, entry.Path, target); err != nil {
					return err
				}
			case "empty":
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
			case "link", "submodule":
				// Ignore links and submodules; they are not portable skill content.
			default:
				return fmt.Errorf("unsupported CNB content type %q", entry.Type)
			}
		}
		return nil
	case "blob":
		name, err := giteaSanitizeName(item.Name)
		if err != nil {
			return err
		}
		if onProgress != nil {
			onProgress(fmt.Sprintf("Downloading %s", item.Path))
		}
		return cnbWriteBlob(item, filepath.Join(destDir, name))
	case "empty":
		return nil
	}
	return fmt.Errorf("unsupported CNB content type %q", item.Type)
}

func cnbDownloadFile(client *http.Client, apiBase, repo, filePath, destPath string) error {
	req, err := cnbNewRequest(buildCNBContentsURL(apiBase, repo, filePath))
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("CNB file download returned %d", resp.StatusCode)
	}
	var item cnbContentItem
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return fmt.Errorf("failed to parse CNB file response: %w", err)
	}
	return cnbWriteBlob(item, destPath)
}

func cnbWriteBlob(item cnbContentItem, destPath string) error {
	if item.Type != "blob" {
		return fmt.Errorf("unsupported CNB file type %q", item.Type)
	}
	data := []byte{}
	if item.Content != "" {
		decoded, decodeErr := base64.StdEncoding.DecodeString(item.Content)
		if decodeErr != nil {
			return fmt.Errorf("failed to decode CNB file content: %w", decodeErr)
		}
		data = decoded
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(destPath, data, 0644)
}

func cnbNewRequest(reqURL string) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.cnb.api+json")
	token := os.Getenv("CNB_TOKEN")
	if token == "" {
		token = os.Getenv("SKILLSHARE_GIT_TOKEN")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req, nil
}

func cnbFetchLatestCommitHash(apiBase, repo string, source *Source) (string, error) {
	commitsURL := fmt.Sprintf("%s/%s/-/git/commits?page=1&page_size=1", strings.TrimRight(apiBase, "/"), escapeRepoPath(repo))
	req, err := cnbNewRequest(commitsURL)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			var commits []giteaCommit
			if decodeErr := json.NewDecoder(resp.Body).Decode(&commits); decodeErr == nil && len(commits) > 0 {
				return commits[0].SHA, nil
			}
		}
	}
	if source != nil && source.CloneURL != "" {
		return getRemoteHeadCommit(source.CloneURL)
	}
	return "", fmt.Errorf("failed to fetch latest commit hash")
}

func cnbRepoPath(cloneURL string) string {
	u := strings.TrimSpace(cloneURL)
	u = strings.TrimSuffix(u, ".git")
	u = strings.TrimSuffix(u, "/")
	if strings.Contains(u, "://") {
		parsed, err := url.Parse(u)
		if err != nil {
			return ""
		}
		return strings.Trim(parsed.Path, "/")
	}
	if strings.Contains(u, "@") {
		if colon := strings.LastIndex(u, ":"); colon != -1 {
			return strings.Trim(u[colon+1:], "/")
		}
	}
	return ""
}

func buildGiteaContentsURL(apiBase, owner, repo, path string) string {
	base := fmt.Sprintf("%s/repos/%s/%s/contents", strings.TrimRight(apiBase, "/"), url.PathEscape(owner), url.PathEscape(repo))
	path = strings.Trim(strings.TrimSpace(path), "/")
	if path == "" {
		return base
	}
	return base + "/" + escapeGiteaPath(path)
}

func buildCNBContentsURL(apiBase, repo, path string) string {
	base := fmt.Sprintf("%s/%s/-/git/contents", strings.TrimRight(apiBase, "/"), escapeRepoPath(repo))
	path = strings.Trim(strings.TrimSpace(path), "/")
	if path == "" {
		return base
	}
	return base + "/" + escapeGiteaPath(path)
}

func escapeRepoPath(repo string) string {
	parts := strings.Split(strings.Trim(repo, "/"), "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}
