package search

import (
	"math"
	"sort"
	"testing"
)

func TestNameMatchScore(t *testing.T) {
	tests := []struct {
		desc     string
		skillNam string
		query    string
		want     float64
	}{
		{"exact match", "pdf-tools", "pdf-tools", 1.0},
		{"exact case insensitive", "PDF-Tools", "pdf-tools", 1.0},
		{"contains", "my-pdf-tools", "pdf", 0.7},
		{"contains query", "react-hooks", "hooks", 0.7},
		{"contains query underscore", "my_skill", "skill", 0.7},
		{"word boundary only", "go-fmt-lint", "fmt", 0.7},
		{"word boundary exact segment", "x-pdf-y", "pdf", 0.7},
		{"no match", "frontend", "zzz", 0.0},
		{"shared chars no match", "skills-scout", "vercel", 0.0},
		{"single char overlap no match", "react", "xyz", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := nameMatchScore(tt.skillNam, tt.query)
			if math.Abs(got-tt.want) > 0.01 {
				t.Errorf("nameMatchScore(%q, %q) = %f, want %f", tt.skillNam, tt.query, got, tt.want)
			}
		})
	}
}

func TestDescriptionMatchScore(t *testing.T) {
	tests := []struct {
		desc  string
		text  string
		query string
		want  float64
	}{
		{"all words match", "A tool for generating PDF files", "pdf files", 1.0},
		{"partial match", "React component library", "react hooks", 0.5},
		{"no match", "Go testing framework", "python async", 0.0},
		{"empty description", "", "react", 0.0},
		{"empty query", "some description", "", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := descriptionMatchScore(tt.text, tt.query)
			if math.Abs(got-tt.want) > 0.01 {
				t.Errorf("descriptionMatchScore(%q, %q) = %f, want %f", tt.text, tt.query, got, tt.want)
			}
		})
	}
}

func TestNormalizeStars(t *testing.T) {
	tests := []struct {
		desc  string
		stars int
		want  float64
	}{
		{"zero", 0, 0.0},
		{"one", 1, 0.0},
		{"ten", 10, 0.2},
		{"hundred", 100, 0.4},
		{"thousand", 1000, 0.6},
		{"ten thousand", 10000, 0.8},
		{"hundred thousand", 100000, 1.0},
		{"million capped", 1000000, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := normalizeStars(tt.stars)
			if math.Abs(got-tt.want) > 0.01 {
				t.Errorf("normalizeStars(%d) = %f, want %f", tt.stars, got, tt.want)
			}
		})
	}
}

func TestScoreResult(t *testing.T) {
	t.Run("name match beats high stars", func(t *testing.T) {
		exactMatch := SearchResult{Name: "pdf-tools", Stars: 10}
		popularRepo := SearchResult{Name: "awesome-list", Stars: 50000}

		scoreExact := scoreResult(exactMatch, "pdf-tools")
		scorePopular := scoreResult(popularRepo, "pdf-tools")

		if scoreExact <= scorePopular {
			t.Errorf("exact name match (%.3f) should beat popular repo (%.3f)", scoreExact, scorePopular)
		}
	})

	t.Run("empty query uses stars only", func(t *testing.T) {
		low := SearchResult{Name: "skill-a", Stars: 10}
		high := SearchResult{Name: "skill-b", Stars: 10000}

		scoreLow := scoreResult(low, "")
		scoreHigh := scoreResult(high, "")

		if scoreLow >= scoreHigh {
			t.Errorf("high stars (%.3f) should beat low stars (%.3f) in browse mode", scoreHigh, scoreLow)
		}
	})
}

func TestScoreResult_Ordering(t *testing.T) {
	results := []SearchResult{
		{Name: "awesome-list", Description: "curated list", Stars: 50000},
		{Name: "react-hooks", Description: "Custom React hooks collection", Stars: 200},
		{Name: "react", Description: "React library for building UIs", Stars: 5000},
		{Name: "my-react-app", Description: "Sample app", Stars: 30},
	}

	query := "react"

	type scored struct {
		name  string
		score float64
	}
	var ranked []scored
	for _, r := range results {
		ranked = append(ranked, scored{r.Name, scoreResult(r, query)})
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})

	// "react" (exact match + decent stars) should be #1
	if ranked[0].name != "react" {
		t.Errorf("expected 'react' first, got %q (scores: %v)", ranked[0].name, ranked)
	}

	// "react-hooks" (word boundary + description match) should beat "awesome-list" (no name match, high stars)
	reactHooksIdx := -1
	awesomeIdx := -1
	for i, r := range ranked {
		if r.name == "react-hooks" {
			reactHooksIdx = i
		}
		if r.name == "awesome-list" {
			awesomeIdx = i
		}
	}
	if reactHooksIdx > awesomeIdx {
		t.Errorf("react-hooks should rank above awesome-list (scores: %v)", ranked)
	}
}

func TestParseRepoQuery(t *testing.T) {
	tests := []struct {
		desc   string
		query  string
		owner  string
		repo   string
		subdir string
		ok     bool
	}{
		{"owner/repo", "vercel-labs/skills", "vercel-labs", "skills", "", true},
		{"owner/repo/subdir", "owner/repo/tools/pdf", "owner", "repo", "tools/pdf", true},
		{"github URL", "https://github.com/vercel-labs/skills", "vercel-labs", "skills", "", true},
		{"github URL with subdir", "github.com/owner/repo/sub", "owner", "repo", "sub", true},
		{"single word", "react", "", "", "", false},
		{"space separated", "react hooks", "", "", "", false},
		{"empty", "", "", "", "", false},
		{"dots in repo", "owner/my.repo", "owner", "my.repo", "", true},
		{"starts with hyphen", "-bad/repo", "", "", "", false},
		{"trailing slash subdir", "owner/repo/sub/", "owner", "repo", "sub", true},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			owner, repo, subdir, ok := parseRepoQuery(tt.query)
			if ok != tt.ok {
				t.Fatalf("parseRepoQuery(%q) ok = %v, want %v", tt.query, ok, tt.ok)
			}
			if !ok {
				return
			}
			if owner != tt.owner {
				t.Errorf("owner = %q, want %q", owner, tt.owner)
			}
			if repo != tt.repo {
				t.Errorf("repo = %q, want %q", repo, tt.repo)
			}
			if subdir != tt.subdir {
				t.Errorf("subdir = %q, want %q", subdir, tt.subdir)
			}
		})
	}
}
