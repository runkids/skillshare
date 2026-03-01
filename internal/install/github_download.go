package install

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	ghclient "skillshare/internal/github"
)

type gitHubContentItem struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	DownloadURL string `json:"download_url"`
}

type gitHubCommit struct {
	SHA string `json:"sha"`
}

func isGitHubAPISource(source *Source) bool {
	if source == nil {
		return false
	}
	return source.GitHubOwner() != "" && source.GitHubRepo() != ""
}

func downloadGitHubDir(owner, repo, path, destDir string, source *Source, onProgress ProgressCallback) (string, error) {
	apiBase, err := gitHubAPIBase(source)
	if err != nil {
		return "", err
	}
	return downloadGitHubDirWithAPIBase(owner, repo, path, destDir, apiBase, source, onProgress)
}

func downloadGitHubDirWithAPIBase(owner, repo, path, destDir, apiBase string, source *Source, onProgress ProgressCallback) (string, error) {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", err
	}

	if onProgress != nil {
		onProgress("Downloading via GitHub API...")
	}

	client := ghclient.NewClient()
	if err := downloadDirRecursive(client, owner, repo, apiBase, strings.Trim(path, "/"), destDir, onProgress); err != nil {
		return "", err
	}

	commitHash, err := fetchLatestCommitHash(owner, repo, apiBase, source)
	if err != nil {
		return "", nil
	}
	return commitHash, nil
}

func downloadDirRecursive(client *http.Client, owner, repo, apiBase, path, destDir string, onProgress ProgressCallback) error {
	apiURL := buildGitHubContentsURL(apiBase, owner, repo, path)

	req, err := ghclient.NewRequest(apiURL)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("GitHub API request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := ghclient.CheckRateLimit(resp); err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub contents API returned %d", resp.StatusCode)
	}

	var raw json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return fmt.Errorf("failed to parse GitHub contents response: %w", err)
	}

	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return fmt.Errorf("empty GitHub contents response for %q", path)
	}

	// File path
	if trimmed[0] == '{' {
		var item gitHubContentItem
		if err := json.Unmarshal(trimmed, &item); err != nil {
			return err
		}
		if item.Type != "file" {
			return fmt.Errorf("unsupported GitHub content type %q", item.Type)
		}
		fileName, err := sanitizeGitHubName(item.Name)
		if err != nil {
			return err
		}
		target := filepath.Join(destDir, fileName)
		if onProgress != nil {
			onProgress(fmt.Sprintf("Downloading %s", item.Path))
		}
		return downloadFile(client, item.DownloadURL, target)
	}

	// Directory path
	if trimmed[0] == '[' {
		var items []gitHubContentItem
		if err := json.Unmarshal(trimmed, &items); err != nil {
			return err
		}
		for _, item := range items {
			name, err := sanitizeGitHubName(item.Name)
			if err != nil {
				return err
			}
			switch item.Type {
			case "dir":
				childDir := filepath.Join(destDir, name)
				if err := os.MkdirAll(childDir, 0755); err != nil {
					return err
				}
				if err := downloadDirRecursive(client, owner, repo, apiBase, item.Path, childDir, onProgress); err != nil {
					return err
				}
			case "file":
				target := filepath.Join(destDir, name)
				if onProgress != nil {
					onProgress(fmt.Sprintf("Downloading %s", item.Path))
				}
				if err := downloadFile(client, item.DownloadURL, target); err != nil {
					return err
				}
			}
		}
		return nil
	}

	return fmt.Errorf("unexpected GitHub contents payload for %q", path)
}

func downloadFile(client *http.Client, fileURL, destPath string) error {
	req, err := ghclient.NewRequest(fileURL)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := ghclient.CheckRateLimit(resp); err != nil {
		return err
	}
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

func fetchLatestCommitHash(owner, repo, apiBase string, source *Source) (string, error) {
	commitsURL := fmt.Sprintf("%s/repos/%s/%s/commits?per_page=1", strings.TrimRight(apiBase, "/"), owner, repo)
	req, err := ghclient.NewRequest(commitsURL)
	if err != nil {
		return "", err
	}

	client := ghclient.NewClient()
	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		if rateErr := ghclient.CheckRateLimit(resp); rateErr != nil {
			return "", rateErr
		}
		if resp.StatusCode == http.StatusOK {
			var commits []gitHubCommit
			if decodeErr := json.NewDecoder(resp.Body).Decode(&commits); decodeErr == nil && len(commits) > 0 {
				return shortCommitHash(commits[0].SHA), nil
			}
		}
	}

	// Fallback for edge cases (e.g. GHE API path differences): use git ls-remote.
	if source != nil && source.CloneURL != "" {
		return getRemoteHeadCommit(source.CloneURL)
	}

	return "", fmt.Errorf("failed to fetch latest commit hash")
}

func getRemoteHeadCommit(cloneURL string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitCommandTimeout)
	defer cancel()

	cmd := gitCommand(ctx, "ls-remote", cloneURL, "HEAD")
	cmd.Env = append(cmd.Env, authEnv(cloneURL)...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) == 0 {
		return "", fmt.Errorf("no HEAD ref found")
	}
	return shortCommitHash(parts[0]), nil
}

func shortCommitHash(hash string) string {
	hash = strings.TrimSpace(hash)
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}

func gitHubAPIBase(source *Source) (string, error) {
	if source == nil {
		return "", fmt.Errorf("nil source")
	}

	host := strings.ToLower(extractHost(source.CloneURL))
	if host == "" {
		return "", fmt.Errorf("unable to determine source host")
	}
	if !strings.Contains(host, "github") {
		return "", fmt.Errorf("source host %q is not GitHub-compatible", host)
	}
	if host == "github.com" {
		return "https://api.github.com", nil
	}
	return fmt.Sprintf("https://%s/api/v3", host), nil
}

func buildGitHubContentsURL(apiBase, owner, repo, path string) string {
	base := fmt.Sprintf("%s/repos/%s/%s/contents", strings.TrimRight(apiBase, "/"), owner, repo)
	path = strings.Trim(strings.TrimSpace(path), "/")
	if path == "" {
		return base
	}
	return base + "/" + escapeGitHubPath(path)
}

func escapeGitHubPath(path string) string {
	parts := strings.Split(path, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

func sanitizeGitHubName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return "", fmt.Errorf("invalid GitHub item name %q", name)
	}
	return name, nil
}
