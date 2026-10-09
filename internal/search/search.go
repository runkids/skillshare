package search

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"unicode"

	"gopkg.in/yaml.v3"

	ghclient "skillshare/internal/github"
	"skillshare/internal/utils"
	"skillshare/internal/validate"
)

// SearchResult represents a skill found via search
type SearchResult struct {
	Name        string   // Skill name (from SKILL.md frontmatter or directory name)
	Description string   // From SKILL.md frontmatter
	Source      string   // Installable source (owner/repo/path)
	Skill       string   // Specific skill name within a multi-skill repo (install -s)
	Stars       int      // Repository star count
	Owner       string   // Repository owner
	Repo        string   // Repository name
	Path        string   // Path within repository
	Tags        []string // Classification tags from hub index
	RiskScore   *int     `json:"riskScore,omitempty"` // Audit risk score (0-100), nil if not audited
	RiskLabel   string   `json:"riskLabel,omitempty"` // Audit risk label (clean/low/medium/high/critical)
	Score       float64  `json:"-"`                   // Internal relevance score, hidden from JSON output
}

// RateLimitError indicates GitHub API rate limit was exceeded.
type RateLimitError = ghclient.RateLimitError

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

type gitHubRepositorySearchResponse struct {
	Items []gitHubRepositorySearchItem `json:"items"`
}

type gitHubRepositorySearchItem struct {
	FullName        string `json:"full_name"`
	StargazersCount int    `json:"stargazers_count"`
	Description     string `json:"description"`
	Fork            bool   `json:"fork"`
	DefaultBranch   string `json:"default_branch"`
}

type gitHubTreeResponse struct {
	Tree []gitHubTreeItem `json:"tree"`
}

type gitHubTreeItem struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

type repoID struct {
	owner string
	repo  string
}

// gitHubContentResponse represents the GitHub contents API response
type gitHubContentResponse struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

type skillMetadata struct {
	Name        string
	Description string
	Valid       bool
}

var preferredSkillRepos = []string{
	"anthropics/skills",
	"vercel-labs/skills",
}

// Search searches GitHub for skills matching the query.
// A single HTTP client is shared across all API calls for connection reuse.
func Search(query string, limit int) ([]SearchResult, error) {
	limit = normalizeLimit(limit)
	client := ghclient.NewClient()
	_, _, _, repoScopedQuery := parseRepoQuery(query)

	searchResp, err := fetchCodeSearchResults(client, query)
	if err != nil {
		return nil, err
	}
	if !repoScopedQuery {
		appendPreferredSkillRepoResults(client, searchResp, query)
		appendRepositorySearchSkillResults(client, searchResp, query)
	}

	results := processSearchItems(searchResp.Items)
	enrichWithStars(client, results)
	sortByStars(results)

	// Enrich top candidates with descriptions before scoring
	metadataLimit := 30
	if !repoScopedQuery {
		metadataLimit = len(results)
	}
	results = enrichWithDescriptions(client, results, metadataLimit, !repoScopedQuery)
	if !repoScopedQuery {
		results = dedupeEquivalentSkills(results)
		results = filterLowQuality(results)
	}

	// For repo-scoped queries, score by subdir keyword (or stars-only if no subdir)
	scoringQuery := query
	if _, _, subdir, ok := parseRepoQuery(query); ok {
		scoringQuery = lastPathSegment(subdir)
	}
	scoreAndSort(results, scoringQuery)

	if len(results) > limit {
		results = results[:limit]
	}

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

// parseRepoQuery detects owner/repo[/subdir] patterns in the query.
// Returns the components if the query looks like a GitHub repo reference.
func parseRepoQuery(query string) (owner, repo, subdir string, ok bool) {
	if query == "" || strings.Contains(query, " ") {
		return "", "", "", false
	}

	// Strip common URL prefixes
	q := query
	q = strings.TrimPrefix(q, "https://github.com/")
	q = strings.TrimPrefix(q, "http://github.com/")
	q = strings.TrimPrefix(q, "github.com/")

	parts := strings.SplitN(q, "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", false
	}

	owner = parts[0]
	repo = parts[1]
	if len(parts) == 3 {
		subdir = strings.TrimSuffix(parts[2], "/")
	}

	if !isValidGitHubName(owner) || !isValidGitHubName(repo) {
		return "", "", "", false
	}

	return owner, repo, subdir, true
}

// isValidGitHubName checks if a string looks like a valid GitHub username or repo name.
func isValidGitHubName(s string) bool {
	if s == "" || s[0] == '-' {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '.' || c == '_') {
			return false
		}
	}
	return true
}

func buildGitHubCodeSearchQuery(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return `filename:SKILL.md "name:" "description:"`
	}

	if owner, repo, subdir, ok := parseRepoQuery(query); ok {
		// Repo-scoped search: find SKILL.md files within a specific repo.
		searchQuery := fmt.Sprintf("filename:SKILL.md repo:%s/%s", owner, repo)
		if subdir != "" {
			searchQuery += fmt.Sprintf(" path:%s", subdir)
		}
		return searchQuery
	}

	return fmt.Sprintf(`filename:SKILL.md "name:" "description:" %s`, query)
}

func buildTrustedSkillRepoSearchQuery(query, repo string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return fmt.Sprintf(`filename:SKILL.md repo:%s "name:" "description:"`, repo)
	}
	return fmt.Sprintf(`filename:SKILL.md repo:%s "name:" "description:" %s`, repo, query)
}

// fetchCodeSearchResults fetches results from GitHub code search API
func fetchCodeSearchResults(client *http.Client, query string) (*gitHubSearchResponse, error) {
	searchQuery := buildGitHubCodeSearchQuery(query)
	return fetchCodeSearchResultsByQuery(client, searchQuery)
}

func fetchCodeSearchResultsByQuery(client *http.Client, searchQuery string) (*gitHubSearchResponse, error) {
	apiURL := fmt.Sprintf(
		"https://api.github.com/search/code?q=%s&per_page=%d",
		url.QueryEscape(searchQuery),
		100, // GitHub API max per page
	)

	req, err := ghclient.NewRequest(apiURL)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	if err := ghclient.CheckRateLimit(resp); err != nil {
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

func appendPreferredSkillRepoResults(client *http.Client, primary *gitHubSearchResponse, query string) {
	if primary == nil {
		return
	}

	for _, repo := range preferredSkillRepos {
		searchQuery := buildTrustedSkillRepoSearchQuery(query, repo)
		resp, err := fetchCodeSearchResultsByQuery(client, searchQuery)
		if err != nil {
			continue
		}
		primary.Items = append(primary.Items, resp.Items...)
		primary.TotalCount += resp.TotalCount
	}
}

func appendRepositorySearchSkillResults(client *http.Client, primary *gitHubSearchResponse, query string) {
	if primary == nil || strings.TrimSpace(query) == "" {
		return
	}

	repos, err := fetchRepositorySearchResults(client, query)
	if err != nil {
		return
	}

	for _, repo := range repos {
		items, err := fetchRepositorySkillTreeItems(client, repo)
		if err != nil {
			continue
		}
		primary.Items = append(primary.Items, items...)
		primary.TotalCount += len(items)
	}
}

func fetchRepositorySearchResults(client *http.Client, query string) ([]gitHubRepositorySearchItem, error) {
	searchQuery := buildGitHubRepositorySearchQuery(query)
	if searchQuery == "" {
		return nil, nil
	}

	apiURL := fmt.Sprintf(
		"https://api.github.com/search/repositories?q=%s&sort=stars&order=desc&per_page=%d",
		url.QueryEscape(searchQuery),
		8,
	)

	req, err := ghclient.NewRequest(apiURL)
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
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub repository search returned %d", resp.StatusCode)
	}

	var searchResp gitHubRepositorySearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, err
	}

	return searchResp.Items, nil
}

func buildGitHubRepositorySearchQuery(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}
	return query + " in:name,description,readme"
}

func fetchRepositorySkillTreeItems(client *http.Client, repo gitHubRepositorySearchItem) ([]gitHubCodeItem, error) {
	if repo.Fork || repo.FullName == "" {
		return nil, nil
	}

	ref := repo.DefaultBranch
	if ref == "" {
		ref = "HEAD"
	}
	apiURL := fmt.Sprintf(
		"https://api.github.com/repos/%s/git/trees/%s?recursive=1",
		repo.FullName,
		url.PathEscape(ref),
	)

	req, err := ghclient.NewRequest(apiURL)
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
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub tree API returned %d", resp.StatusCode)
	}

	var tree gitHubTreeResponse
	if err := json.NewDecoder(resp.Body).Decode(&tree); err != nil {
		return nil, err
	}

	return repositorySkillTreeItems(repo, tree.Tree, 40), nil
}

func repositorySkillTreeItems(repo gitHubRepositorySearchItem, tree []gitHubTreeItem, max int) []gitHubCodeItem {
	if repo.Fork || repo.FullName == "" || max <= 0 {
		return nil
	}

	items := make([]gitHubCodeItem, 0)
	for _, entry := range tree {
		if entry.Type != "blob" || entry.Path == "" {
			continue
		}
		if entry.Path != "SKILL.md" && !strings.HasSuffix(entry.Path, "/SKILL.md") {
			continue
		}
		items = append(items, gitHubCodeItem{
			Name: "SKILL.md",
			Path: entry.Path,
			Repository: gitHubRepository{
				FullName:        repo.FullName,
				StargazersCount: repo.StargazersCount,
				Description:     repo.Description,
				Fork:            repo.Fork,
			},
		})
		if len(items) >= max {
			break
		}
	}
	return items
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

// enrichWithStars fetches and updates star counts for results in parallel.
func enrichWithStars(client *http.Client, results []SearchResult) {
	const maxRepoFetch = 30
	const concurrency = 10

	repos := reposForStarFetch(results, maxRepoFetch)

	// Fetch stars concurrently
	type starResult struct {
		id    repoID
		stars int
	}
	ch := make(chan starResult, len(repos))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for _, id := range repos {
		wg.Add(1)
		go func(id repoID) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if stars, err := fetchRepoStars(client, id.owner, id.repo); err == nil {
				ch <- starResult{id, stars}
			}
		}(id)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	repoStars := make(map[repoID]int)
	for sr := range ch {
		repoStars[sr.id] = sr.stars
	}

	for i := range results {
		id := repoID{results[i].Owner, results[i].Repo}
		if stars, exists := repoStars[id]; exists {
			results[i].Stars = stars
		}
	}
}

func reposForStarFetch(results []SearchResult, max int) []repoID {
	seen := make(map[repoID]bool)
	repos := make([]repoID, 0, max)

	add := func(r SearchResult) {
		id := repoID{r.Owner, r.Repo}
		if id.owner == "" || id.repo == "" || seen[id] {
			return
		}
		seen[id] = true
		repos = append(repos, id)
	}

	for _, r := range results {
		if isPreferredSkillRepo(r.Owner, r.Repo) {
			add(r)
		}
	}
	for _, r := range results {
		if len(repos) >= max {
			break
		}
		add(r)
	}

	return repos
}

// sortByStars sorts results by star count descending
func sortByStars(results []SearchResult) {
	sort.Slice(results, func(i, j int) bool {
		return results[i].Stars > results[j].Stars
	})
}

// enrichWithDescriptions fetches descriptions and names for top results in parallel.
// In broad GitHub search mode, it also drops candidates whose SKILL.md does not
// look like a skill frontmatter document. Candidates are kept on fetch errors so
// transient GitHub failures do not turn a search into an empty result set.
func enrichWithDescriptions(client *http.Client, results []SearchResult, limit int, filterInvalid bool) []SearchResult {
	const concurrency = 10

	n := len(results)
	if n > limit {
		n = limit
	}
	if n <= 0 {
		return results
	}

	type metaResult struct {
		idx  int
		meta skillMetadata
	}
	ch := make(chan metaResult, n)
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			meta, err := fetchSkillMetadata(client, results[idx].Owner, results[idx].Repo, results[idx].Path)
			if err == nil {
				ch <- metaResult{idx, meta}
			}
		}(i)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	fetched := make(map[int]skillMetadata, n)
	for mr := range ch {
		fetched[mr.idx] = mr.meta
		if mr.meta.Description != "" {
			results[mr.idx].Description = mr.meta.Description
		}
		if mr.meta.Name != "" {
			results[mr.idx].Name = mr.meta.Name
		}
	}

	if !filterInvalid {
		return results
	}

	filtered := results[:0]
	for i := range results {
		if meta, ok := fetched[i]; ok && !meta.Valid {
			continue
		}
		filtered = append(filtered, results[i])
	}
	return filtered
}

func dedupeEquivalentSkills(results []SearchResult) []SearchResult {
	deduped := make([]SearchResult, 0, len(results))

	for _, result := range results {
		idx := findEquivalentSkill(deduped, result)
		if idx < 0 {
			deduped = append(deduped, result)
			continue
		}

		if preferSearchResult(result, deduped[idx]) {
			deduped[idx] = result
		}
	}

	return deduped
}

func findEquivalentSkill(results []SearchResult, candidate SearchResult) int {
	for i, result := range results {
		if equivalentSkillIdentity(result, candidate) {
			return i
		}
	}
	return -1
}

func equivalentSkillIdentity(a, b SearchResult) bool {
	nameA := normalizeSkillIdentityText(a.Name)
	nameB := normalizeSkillIdentityText(b.Name)
	if nameA == "" || nameA != nameB {
		return false
	}

	descA := normalizeSkillIdentityText(a.Description)
	descB := normalizeSkillIdentityText(b.Description)
	if descA == "" || descB == "" {
		return false
	}
	if descA == descB {
		return true
	}

	shorter, longer := descA, descB
	if len(shorter) > len(longer) {
		shorter, longer = longer, shorter
	}
	return len(shorter) >= 80 && strings.Contains(longer, shorter)
}

func normalizeSkillIdentityText(s string) string {
	normalized := strings.Map(func(r rune) rune {
		switch {
		case unicode.IsLetter(r):
			return unicode.ToLower(r)
		case unicode.IsDigit(r):
			return r
		default:
			return ' '
		}
	}, strings.TrimSpace(s))
	return strings.Join(strings.Fields(normalized), " ")
}

// filterLowQuality removes search results that are almost certainly not useful skills.
// Preferred repos (anthropics/skills, vercel-labs/skills) are never filtered.
// Results that failed metadata fetch are kept (transient GitHub errors shouldn't empty results).
func filterLowQuality(results []SearchResult) []SearchResult {
	filtered := make([]SearchResult, 0, len(results))
	for _, r := range results {
		if isPreferredSkillRepo(r.Owner, r.Repo) {
			filtered = append(filtered, r)
			continue
		}
		if isLowQualityResult(r) {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered
}

func isLowQualityResult(r SearchResult) bool {
	// Zero-star repos are almost always personal experiments or spam
	if r.Stars == 0 {
		return true
	}
	// Very short descriptions indicate stub or auto-generated skills
	if len(strings.TrimSpace(r.Description)) > 0 && len(strings.TrimSpace(r.Description)) < 10 {
		return true
	}
	// Known spam organizations
	if isSpamOrg(r.Owner) {
		return true
	}
	return false
}

var spamOrgs = []string{
	"inference-sh",
}

func isSpamOrg(owner string) bool {
	ol := strings.ToLower(strings.TrimSpace(owner))
	for _, org := range spamOrgs {
		if ol == org {
			return true
		}
	}
	return false
}

func preferSearchResult(candidate, current SearchResult) bool {
	candidateQuality := sourceQualityScore(candidate)
	currentQuality := sourceQualityScore(current)
	if candidateQuality != currentQuality {
		return candidateQuality > currentQuality
	}
	if candidate.Stars != current.Stars {
		return candidate.Stars > current.Stars
	}
	if len(candidate.Path) != len(current.Path) {
		return len(candidate.Path) < len(current.Path)
	}
	return candidate.Source < current.Source
}

// scoreAndSort computes relevance scores and sorts results descending.
func scoreAndSort(results []SearchResult, query string) {
	for i := range results {
		results[i].Score = scoreResult(results[i], query)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
}

// fetchRepoStars fetches the star count for a repository
func fetchRepoStars(client *http.Client, owner, repo string) (int, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)

	req, err := ghclient.NewRequest(apiURL)
	if err != nil {
		return 0, err
	}

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

// fetchSkillMetadata fetches SKILL.md and extracts skill metadata from frontmatter.
func fetchSkillMetadata(client *http.Client, owner, repo, path string) (skillMetadata, error) {
	skillPath := "SKILL.md"
	if path != "" && path != "." {
		skillPath = path + "/SKILL.md"
	}

	apiURL := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/contents/%s",
		owner, repo, url.PathEscape(skillPath),
	)

	req, err := ghclient.NewRequest(apiURL)
	if err != nil {
		return skillMetadata{}, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return skillMetadata{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return skillMetadata{}, fmt.Errorf("failed to fetch SKILL.md")
	}

	var content gitHubContentResponse
	if err := json.NewDecoder(resp.Body).Decode(&content); err != nil {
		return skillMetadata{}, err
	}

	if content.Encoding != "base64" {
		return skillMetadata{}, fmt.Errorf("unexpected encoding: %s", content.Encoding)
	}

	decoded, err := base64.StdEncoding.DecodeString(content.Content)
	if err != nil {
		return skillMetadata{}, err
	}

	return parseSkillMetadata(string(decoded)), nil
}

func parseSkillMetadata(content string) skillMetadata {
	raw, ok := extractSkillFrontmatter(content)
	if !ok {
		return skillMetadata{}
	}

	fm, ok := parseSkillFrontmatter(raw)
	if !ok {
		return skillMetadata{}
	}

	name := yamlScalarString(fm["name"])
	desc := yamlScalarString(fm["description"])
	if !isDiscoverableSkillFrontmatter(fm, name, desc) {
		return skillMetadata{}
	}
	return skillMetadata{
		Name:        name,
		Description: desc,
		Valid:       true,
	}
}

func extractSkillFrontmatter(content string) (string, bool) {
	lines := strings.Split(utils.TrimBOM(content), "\n")
	if len(lines) == 0 {
		return "", false
	}

	first := strings.TrimSpace(lines[0])
	if strings.HasPrefix(first, "--- ") {
		rest := strings.TrimSpace(strings.TrimPrefix(first, "---"))
		if end := strings.Index(rest, " ---"); end >= 0 {
			return strings.TrimSpace(rest[:end]), true
		}
	}

	if len(lines) < 3 || first != "---" {
		return "", false
	}

	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], "\n"), true
		}
	}
	return "", false
}

func parseSkillFrontmatter(raw string) (map[string]any, bool) {
	var fm map[string]any
	if err := yaml.Unmarshal([]byte(raw), &fm); err == nil && len(fm) > 0 {
		return fm, true
	}
	return parseInlineSkillFrontmatter(raw)
}

func parseInlineSkillFrontmatter(raw string) (map[string]any, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.Contains(raw, "\n") {
		return nil, false
	}

	positions := inlineFrontmatterFieldPositions(raw)
	if len(positions) == 0 {
		return nil, false
	}

	fm := make(map[string]any, len(positions))
	for i, pos := range positions {
		end := len(raw)
		if i+1 < len(positions) {
			end = positions[i+1].start
		}
		value := strings.TrimSpace(raw[pos.valueStart:end])
		value = strings.Trim(value, `"'`)
		if value != "" {
			fm[pos.key] = value
		}
	}
	return fm, len(fm) > 0
}

type inlineFrontmatterField struct {
	key        string
	start      int
	valueStart int
}

func inlineFrontmatterFieldPositions(raw string) []inlineFrontmatterField {
	keys := []string{"name", "description", "metadata", "targets", "allowed-tools", "tags", "pattern", "category"}
	var positions []inlineFrontmatterField

	for _, key := range keys {
		prefix := key + ":"
		if strings.HasPrefix(raw, prefix) {
			positions = append(positions, inlineFrontmatterField{
				key:        key,
				start:      0,
				valueStart: len(prefix),
			})
		}

		marker := " " + prefix
		offset := 0
		for {
			idx := strings.Index(raw[offset:], marker)
			if idx < 0 {
				break
			}
			start := offset + idx + 1
			positions = append(positions, inlineFrontmatterField{
				key:        key,
				start:      start,
				valueStart: start + len(prefix),
			})
			offset = start + len(prefix)
		}
	}

	sort.Slice(positions, func(i, j int) bool {
		return positions[i].start < positions[j].start
	})
	return positions
}

func isDiscoverableSkillFrontmatter(fm map[string]any, name, desc string) bool {
	// Same rule install applies, so search never hides an installable skill.
	if validate.SkillName(name) != nil {
		return false
	}
	if desc != "" {
		return true
	}

	for _, field := range []string{"metadata", "targets", "allowed-tools", "tags", "pattern", "category"} {
		if hasYAMLValue(fm[field]) {
			return true
		}
	}

	return false
}

func yamlScalarString(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case int:
		return fmt.Sprintf("%d", x)
	case int64:
		return fmt.Sprintf("%d", x)
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", x), "0"), ".")
	case bool:
		return fmt.Sprintf("%t", x)
	default:
		return ""
	}
}

func hasYAMLValue(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(x) != ""
	case []any:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	default:
		return true
	}
}

// parseFrontmatterField extracts a field value from YAML frontmatter.
func parseFrontmatterField(content, field string) string {
	content = utils.TrimBOM(content)
	if raw, ok := extractSkillFrontmatter(content); ok {
		if fm, ok := parseSkillFrontmatter(raw); ok {
			if val := yamlScalarString(fm[field]); val != "" {
				return val
			}
		}
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	inFrontmatter := false
	prefix := field + ":"

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "---" {
			if inFrontmatter {
				break
			}
			inFrontmatter = true
			continue
		}

		if inFrontmatter && strings.HasPrefix(line, prefix) {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				val := strings.TrimSpace(parts[1])
				val = strings.Trim(val, `"'`)
				return val
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

// FormatStars formats star count for display (e.g., 2400 -> 2.4k)
func FormatStars(stars int) string {
	if stars >= 1000 {
		return fmt.Sprintf("%.1fk", float64(stars)/1000)
	}
	return fmt.Sprintf("%d", stars)
}
