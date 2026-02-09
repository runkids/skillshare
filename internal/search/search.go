package search

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// SearchResult represents a skill found via search
type SearchResult struct {
	Name        string // Skill name (from SKILL.md frontmatter or directory name)
	Description string // From SKILL.md frontmatter
	Source      string // Installable source (owner/repo/path)
	Stars       int    // Repository star count
	Owner       string // Repository owner
	Repo        string // Repository name
	Path        string // Path within repository
}

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
	return msg
}

// AuthRequiredError indicates GitHub API requires authentication
type AuthRequiredError struct{}

func (e *AuthRequiredError) Error() string {
	return "GitHub Code Search API requires authentication"
}

// gitHubSearchResponse represents the GitHub code search API response
type gitHubSearchResponse struct {
	TotalCount int              `json:"total_count"`
	Items      []gitHubCodeItem `json:"items"`
}

// gitHubCodeItem represents an item in GitHub code search results
type gitHubCodeItem struct {
	Name       string           `json:"name"`
	Path       string           `json:"path"`
	Repository gitHubRepository `json:"repository"`
}

// gitHubRepository represents repository info in code search results
type gitHubRepository struct {
	FullName        string `json:"full_name"`
	StargazersCount int    `json:"stargazers_count"`
	Description     string `json:"description"`
	Fork            bool   `json:"fork"`
}

// gitHubContentResponse represents the GitHub contents API response
type gitHubContentResponse struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

// Search searches GitHub for skills matching the query
func Search(query string, limit int) ([]SearchResult, error) {
	limit = normalizeLimit(limit)

	searchResp, err := fetchCodeSearchResults(query)
	if err != nil {
		return nil, err
	}

	results := processSearchItems(searchResp.Items)
	enrichWithStars(results)
	sortByStars(results)

	if len(results) > limit {
		results = results[:limit]
	}

	enrichWithDescriptions(results)

	return results, nil
}

// normalizeLimit ensures limit is within valid range
func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return limit
}

// fetchCodeSearchResults fetches results from GitHub code search API
func fetchCodeSearchResults(query string) (*gitHubSearchResponse, error) {
	var searchQuery string
	if query == "" {
		searchQuery = "filename:SKILL.md"
	} else {
		searchQuery = fmt.Sprintf("filename:SKILL.md %s", query)
	}
	apiURL := fmt.Sprintf(
		"https://api.github.com/search/code?q=%s&per_page=%d",
		url.QueryEscape(searchQuery),
		100, // GitHub API max per page
	)

	req, err := newGitHubRequest(apiURL)
	if err != nil {
		return nil, err
	}

	client := newGitHubClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	if err := checkRateLimit(resp); err != nil {
		return nil, err
	}

	if resp.StatusCode == 401 {
		return nil, &AuthRequiredError{}
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var searchResp gitHubSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &searchResp, nil
}

// processSearchItems converts raw GitHub items to SearchResults with deduplication
func processSearchItems(items []gitHubCodeItem) []SearchResult {
	seen := make(map[string]bool)
	var results []SearchResult

	for _, item := range items {
		result, ok := convertToSearchResult(item, seen)
		if ok {
			results = append(results, result)
		}
	}

	return results
}

// convertToSearchResult converts a single GitHub item to SearchResult
func convertToSearchResult(item gitHubCodeItem, seen map[string]bool) (SearchResult, bool) {
	if item.Name != "SKILL.md" || item.Repository.Fork {
		return SearchResult{}, false
	}

	dirPath := strings.TrimSuffix(item.Path, "/SKILL.md")
	dirPath = strings.TrimSuffix(dirPath, "SKILL.md")

	key := item.Repository.FullName + "/" + dirPath
	if seen[key] {
		return SearchResult{}, false
	}
	seen[key] = true

	parts := strings.SplitN(item.Repository.FullName, "/", 2)
	if len(parts) != 2 {
		return SearchResult{}, false
	}
	owner, repo := parts[0], parts[1]

	name := repo
	if dirPath != "" && dirPath != "." {
		name = lastPathSegment(dirPath)
	}

	source := item.Repository.FullName
	if dirPath != "" && dirPath != "." {
		source = item.Repository.FullName + "/" + dirPath
	}

	return SearchResult{
		Name:   name,
		Source: source,
		Stars:  item.Repository.StargazersCount,
		Owner:  owner,
		Repo:   repo,
		Path:   dirPath,
	}, true
}

// enrichWithStars fetches and updates star counts for results
func enrichWithStars(results []SearchResult) {
	repoStars := make(map[string]int)
	repoCount := 0
	const maxRepoFetch = 30

	for _, r := range results {
		if repoCount >= maxRepoFetch {
			break
		}
		repoKey := r.Owner + "/" + r.Repo
		if _, exists := repoStars[repoKey]; !exists {
			if stars, err := fetchRepoStars(r.Owner, r.Repo); err == nil {
				repoStars[repoKey] = stars
			}
			repoCount++
		}
	}

	for i := range results {
		repoKey := results[i].Owner + "/" + results[i].Repo
		if stars, exists := repoStars[repoKey]; exists {
			results[i].Stars = stars
		}
	}
}

// sortByStars sorts results by star count descending
func sortByStars(results []SearchResult) {
	sort.Slice(results, func(i, j int) bool {
		return results[i].Stars > results[j].Stars
	})
}

// enrichWithDescriptions fetches descriptions for top results
func enrichWithDescriptions(results []SearchResult) {
	descLimit := len(results)
	if descLimit > 10 {
		descLimit = 10
	}

	for i := 0; i < descLimit; i++ {
		desc, err := fetchSkillDescription(results[i].Owner, results[i].Repo, results[i].Path)
		if err == nil && desc != "" {
			results[i].Description = desc
		}
	}
}

// fetchRepoStars fetches the star count for a repository
func fetchRepoStars(owner, repo string) (int, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)

	req, err := newGitHubRequest(apiURL)
	if err != nil {
		return 0, err
	}

	client := newGitHubClient()
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
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

// fetchSkillDescription fetches and parses SKILL.md to extract description
func fetchSkillDescription(owner, repo, path string) (string, error) {
	// Build path to SKILL.md
	skillPath := "SKILL.md"
	if path != "" && path != "." {
		skillPath = path + "/SKILL.md"
	}

	apiURL := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/contents/%s",
		owner, repo, url.PathEscape(skillPath),
	)

	req, err := newGitHubRequest(apiURL)
	if err != nil {
		return "", err
	}

	client := newGitHubClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to fetch SKILL.md")
	}

	var content gitHubContentResponse
	if err := json.NewDecoder(resp.Body).Decode(&content); err != nil {
		return "", err
	}

	// Decode base64 content
	if content.Encoding != "base64" {
		return "", fmt.Errorf("unexpected encoding: %s", content.Encoding)
	}

	decoded, err := base64.StdEncoding.DecodeString(content.Content)
	if err != nil {
		return "", err
	}

	// Parse frontmatter for description
	return parseSkillDescription(string(decoded)), nil
}

// parseSkillDescription extracts description from SKILL.md frontmatter
func parseSkillDescription(content string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	inFrontmatter := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Detect frontmatter delimiters
		if line == "---" {
			if inFrontmatter {
				break // End of frontmatter
			}
			inFrontmatter = true
			continue
		}

		if inFrontmatter {
			if strings.HasPrefix(line, "description:") {
				// Extract value: "description: my description" -> "my description"
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					desc := strings.TrimSpace(parts[1])
					// Remove quotes if present
					desc = strings.Trim(desc, `"'`)
					return desc
				}
			}
		}
	}

	return ""
}

// lastPathSegment returns the last segment of a path
func lastPathSegment(path string) string {
	path = strings.TrimSuffix(path, "/")
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[idx+1:]
	}
	return path
}

// newGitHubClient creates an HTTP client for GitHub API
func newGitHubClient() *http.Client {
	return &http.Client{Timeout: 15 * time.Second}
}

// newGitHubRequest creates a request with auth header if token is available
func newGitHubRequest(url string) (*http.Request, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")

	// Try to get token from various sources
	token := getGitHubToken()
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	return req, nil
}

// cachedGHToken caches the result of gh auth token command
var cachedGHToken string
var ghTokenChecked bool

// getGitHubToken attempts to get a GitHub token from various sources
func getGitHubToken() string {
	// 1. Check GITHUB_TOKEN environment variable
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token
	}

	// 2. Check GH_TOKEN environment variable (used by gh CLI)
	if token := os.Getenv("GH_TOKEN"); token != "" {
		return token
	}

	// 3. Try to get token from gh CLI (cached)
	if ghTokenChecked {
		return cachedGHToken
	}
	ghTokenChecked = true

	token, err := getGHCLIToken()
	if err == nil && token != "" {
		cachedGHToken = token
		return token
	}

	return ""
}

// getGHCLIToken attempts to get token from gh CLI
func getGHCLIToken() (string, error) {
	cmd := exec.Command("gh", "auth", "token")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
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

// FormatStars formats star count for display (e.g., 2400 -> 2.4k)
func FormatStars(stars int) string {
	if stars >= 1000 {
		return fmt.Sprintf("%.1fk", float64(stars)/1000)
	}
	return fmt.Sprintf("%d", stars)
}
