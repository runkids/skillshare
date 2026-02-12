package search

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type indexDocument struct {
	SourcePath string       `json:"sourcePath"`
	Skills     []indexSkill `json:"skills"`
}

type indexSkill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`
}

// SearchFromIndexURL searches skills from a private index.json URL or local path.
func SearchFromIndexURL(query string, limit int, indexURL string) ([]SearchResult, error) {
	limit = normalizeLimit(limit)
	doc, err := loadIndex(indexURL)
	if err != nil {
		return nil, err
	}

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
			hay := strings.ToLower(name + "\n" + it.Description + "\n" + source)
			if !strings.Contains(hay, q) {
				continue
			}
		}
		owner, repo := parseOwnerRepo(source)
		results = append(results, SearchResult{
			Name:        name,
			Description: strings.TrimSpace(it.Description),
			Source:      source,
			Owner:       owner,
			Repo:        repo,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})
	if len(results) > limit {
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
		return nil, fmt.Errorf("index URL is required")
	}

	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		client := &http.Client{Timeout: 15 * time.Second}
		req, err := http.NewRequest("GET", s, nil)
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetch index: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("fetch index: HTTP %d", resp.StatusCode)
		}
		var doc indexDocument
		if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
			return nil, fmt.Errorf("parse index: %w", err)
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
		return nil, fmt.Errorf("parse index: %w", err)
	}
	return &doc, nil
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
