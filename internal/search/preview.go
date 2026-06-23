package search

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	ghclient "skillshare/internal/github"
	"skillshare/internal/install"
)

// ErrSkillNotFound is returned when SKILL.md cannot be found at the given source.
var ErrSkillNotFound = errors.New("skill not found")

// ErrPreviewUnsupported is returned when the source cannot be parsed or its
// SKILL.md cannot be retrieved for preview (e.g. an unreachable host).
var ErrPreviewUnsupported = errors.New("preview unavailable for this source")

// SkillPreview contains the full SKILL.md content + parsed frontmatter metadata
// for previewing a remote skill before installation.
type SkillPreview struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	License     string   `json:"license,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Content     string   `json:"content"`
	Source      string   `json:"source"`
	Stars       int      `json:"stars"`
	Owner       string   `json:"owner"`
	Repo        string   `json:"repo"`
}

// FetchPreview resolves a hub/search source string to a full SKILL.md preview.
// GitHub-family hosts (github.com and GitHub Enterprise) use the REST Contents
// API at the host's own API base; all other platforms (GitLab, Bitbucket,
// Azure, SSH, …) fall back to a shallow git clone so their SKILL.md renders too.
func FetchPreview(client *http.Client, source, branch string) (*SkillPreview, error) {
	src, err := install.ParseSource(source)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPreviewUnsupported, err)
	}
	return fetchPreviewFromSource(client, src, source, branch)
}

func fetchPreviewFromSource(client *http.Client, src *install.Source, source, branch string) (*SkillPreview, error) {
	if branch == "" {
		branch = src.Branch
	}

	if apiBase := src.GitHubAPIBase(); apiBase != "" {
		owner, repo := src.GitHubOwner(), src.GitHubRepo()
		if owner != "" && repo != "" {
			if src.Type == install.SourceTypeGitSSH && previewToken(src.CloneURL, apiBase) == "" {
				return fetchViaClone(src, source, branch)
			}
			preview, err := fetchViaContentsAPI(client, apiBase, src.CloneURL, owner, repo, src.Subdir, branch)
			if err == nil || src.Type != install.SourceTypeGitSSH {
				return preview, err
			}
			return fetchViaClone(src, source, branch)
		}
	}
	return fetchViaClone(src, source, branch)
}

// fetchViaContentsAPI fetches SKILL.md through the GitHub/GHE Contents API at
// the given API base. cloneURL is used only to resolve a host-appropriate auth
// token. The path parameter is the subdirectory within the repo (empty for root).
func fetchViaContentsAPI(client *http.Client, apiBase, cloneURL, owner, repo, path, branch string) (*SkillPreview, error) {
	token := previewToken(cloneURL, apiBase)

	skillPath := "SKILL.md"
	if path != "" && path != "." {
		skillPath = path + "/SKILL.md"
	}

	apiURL := fmt.Sprintf(
		"%s/repos/%s/%s/contents/%s",
		apiBase, owner, repo, url.PathEscape(skillPath),
	)
	if branch != "" {
		apiURL += "?ref=" + url.QueryEscape(branch)
	}

	req, err := newContentsRequest(apiURL, token)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := ghclient.CheckRateLimit(resp); err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusNotFound && path != "" && path != "." {
		// Direct path failed — try finding SKILL.md via Git Tree API.
		// Hub sources use install shorthands (e.g. "owner/repo/critique")
		// where the actual file may be at "source/skills/critique/SKILL.md".
		if resolved := resolveSkillPath(client, apiBase, token, owner, repo, path, branch); resolved != "" {
			return fetchViaContentsAPI(client, apiBase, cloneURL, owner, repo, resolved, branch)
		}
		return nil, fmt.Errorf("%w: %s/%s/%s", ErrSkillNotFound, owner, repo, path)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%w: %s/%s", ErrSkillNotFound, owner, repo)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var content gitHubContentResponse
	if err := json.NewDecoder(resp.Body).Decode(&content); err != nil {
		return nil, err
	}
	if content.Encoding != "base64" {
		return nil, fmt.Errorf("unexpected encoding: %s", content.Encoding)
	}

	decoded, err := base64.StdEncoding.DecodeString(content.Content)
	if err != nil {
		return nil, err
	}

	source := owner + "/" + repo
	if path != "" && path != "." {
		source = source + "/" + path
	}

	preview := buildPreview(string(decoded), source, owner, repo)

	// Fetch star count (best-effort, don't fail on error)
	if stars, err := fetchRepoStarsBase(client, apiBase, token, owner, repo); err == nil {
		preview.Stars = stars
	}

	return preview, nil
}

// fetchViaClone shallow-clones the source repo and reads SKILL.md from it. This
// path serves non-GitHub platforms (GitLab, Bitbucket, Azure) and SSH sources,
// relying on the user's git/SSH credentials for private and enterprise hosts.
func fetchViaClone(src *install.Source, source, branch string) (*SkillPreview, error) {
	if src.CloneURL == "" {
		return nil, fmt.Errorf("%w: %s", ErrPreviewUnsupported, source)
	}

	dir, err := install.ShallowCloneToTemp(src.CloneURL, branch)
	if err != nil {
		return nil, fmt.Errorf("%w: clone %s: %v", ErrPreviewUnsupported, source, err)
	}
	defer os.RemoveAll(dir)

	rel := findSkillFile(dir, src.Subdir)
	if rel == "" {
		return nil, fmt.Errorf("%w: %s", ErrSkillNotFound, source)
	}

	data, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrSkillNotFound, source)
	}

	return buildPreview(string(data), strings.TrimSpace(source), src.GitHubOwner(), src.GitHubRepo()), nil
}

// findSkillFile locates SKILL.md within a cloned repo rooted at root. It first
// tries subdir/SKILL.md (or SKILL.md at the root when subdir is empty), then
// falls back to searching for {name}/SKILL.md anywhere in the tree, preferring
// matches under "source/" — mirroring resolveSkillPath for hub shorthands.
// Returns the path relative to root, or "" if not found.
func findSkillFile(root, subdir string) string {
	clean := filepath.Clean(strings.TrimSpace(subdir))

	if clean == "." || clean == "" {
		if fileExists(filepath.Join(root, "SKILL.md")) {
			return "SKILL.md"
		}
	} else if !filepath.IsAbs(clean) && clean != ".." &&
		!strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		direct := filepath.Join(clean, "SKILL.md")
		if fileExists(filepath.Join(root, direct)) {
			return direct
		}
	}

	skillName := filepath.Base(clean)
	if skillName == "." || skillName == string(filepath.Separator) || skillName == "" {
		return ""
	}

	suffix := string(filepath.Separator) + skillName + string(filepath.Separator) + "SKILL.md"
	var best string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil || filepath.Base(rel) != "SKILL.md" {
			return nil
		}
		if !strings.HasSuffix(string(filepath.Separator)+rel, suffix) {
			return nil
		}
		dir := strings.TrimSuffix(rel, string(filepath.Separator)+"SKILL.md")
		if rel == "SKILL.md" {
			dir = ""
		}
		if strings.HasPrefix(rel, "source"+string(filepath.Separator)) {
			best = dir
			return fs.SkipAll // prefer source/ paths
		}
		if best == "" {
			best = dir
		}
		return nil
	})
	if best == "" {
		return ""
	}
	return filepath.Join(best, "SKILL.md")
}

// buildPreview parses SKILL.md frontmatter into a SkillPreview.
func buildPreview(body, source, owner, repo string) *SkillPreview {
	preview := &SkillPreview{
		Name:        parseFrontmatterField(body, "name"),
		Description: parseFrontmatterField(body, "description"),
		License:     parseFrontmatterField(body, "license"),
		Content:     body,
		Source:      source,
		Owner:       owner,
		Repo:        repo,
	}

	// Parse tags (comma-separated or YAML list on one line)
	if tagsRaw := parseFrontmatterField(body, "tags"); tagsRaw != "" {
		tagsRaw = strings.Trim(tagsRaw, "[]")
		for t := range strings.SplitSeq(tagsRaw, ",") {
			t = strings.TrimSpace(t)
			t = strings.Trim(t, `"'`)
			if t != "" {
				preview.Tags = append(preview.Tags, t)
			}
		}
	}

	return preview
}

// previewToken resolves an auth token for the Contents API request. It prefers
// a host-specific token (GITHUB_TOKEN, GHE token via SKILLSHARE_GIT_TOKEN, …)
// and falls back to `gh auth token` only for github.com.
func previewToken(cloneURL, apiBase string) string {
	if t, _ := install.ResolveTokenForURL(cloneURL); t != "" {
		return t
	}
	if lookupURL := previewTokenLookupURL(cloneURL); lookupURL != "" {
		if t, _ := install.ResolveTokenForURL(lookupURL); t != "" {
			return t
		}
	}
	if apiBase == "https://api.github.com" {
		return ghclient.GetToken()
	}
	return ""
}

func previewTokenLookupURL(cloneURL string) string {
	if _, host, ok := install.SSHIdentity(cloneURL); ok {
		return "https://" + host + "/"
	}
	u, err := url.Parse(strings.TrimSpace(cloneURL))
	if err != nil || u.Hostname() == "" {
		return ""
	}
	return "https://" + u.Hostname() + "/"
}

// newContentsRequest builds a GET request for the GitHub/GHE REST API with the
// given token (token may be empty for anonymous requests).
func newContentsRequest(apiURL, token string) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}
	return req, nil
}

// fetchRepoStarsBase fetches the star count for a repository at the given API base.
func fetchRepoStarsBase(client *http.Client, apiBase, token, owner, repo string) (int, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/%s", apiBase, owner, repo)
	req, err := newContentsRequest(apiURL, token)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("failed to fetch repo info")
	}

	var repoInfo struct {
		StargazersCount int `json:"stargazers_count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&repoInfo); err != nil {
		return 0, err
	}
	return repoInfo.StargazersCount, nil
}

// resolveSkillPath uses the Git Tree API to find the actual path of a
// SKILL.md that matches the given skill name. Hub sources use install
// shorthands where the path segment is the skill name, not the repo path.
// Returns the resolved directory path (e.g. "source/skills/critique") or "".
func resolveSkillPath(client *http.Client, apiBase, token, owner, repo, skillName, branch string) string {
	ref := "HEAD"
	if branch != "" {
		ref = branch
	}
	apiURL := fmt.Sprintf(
		"%s/repos/%s/%s/git/trees/%s?recursive=1",
		apiBase, owner, repo, ref,
	)

	req, err := newContentsRequest(apiURL, token)
	if err != nil {
		return ""
	}

	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	var tree struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"tree"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tree); err != nil {
		return ""
	}

	// Look for {skillName}/SKILL.md — prefer paths under "source/" (original source)
	suffix := "/" + skillName + "/SKILL.md"
	var best string
	for _, entry := range tree.Tree {
		if entry.Type != "blob" {
			continue
		}
		if !strings.HasSuffix(entry.Path, suffix) {
			continue
		}
		// Strip the trailing /SKILL.md to get the directory path
		dir := strings.TrimSuffix(entry.Path, "/SKILL.md")
		if strings.HasPrefix(entry.Path, "source/") {
			return dir // prefer source/ paths
		}
		if best == "" {
			best = dir
		}
	}
	return best
}

// fileExists reports whether path exists and is a regular file.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
