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

func TestRepositoryMatchScore(t *testing.T) {
	tests := []struct {
		desc   string
		result SearchResult
		query  string
		want   float64
	}{
		{"repo exact", SearchResult{Owner: "obra", Repo: "superpowers"}, "superpowers", 1.0},
		{"full name exact", SearchResult{Owner: "obra", Repo: "superpowers"}, "obra/superpowers", 1.0},
		{"repo contains", SearchResult{Owner: "team", Repo: "awesome-superpowers"}, "superpowers", 0.7},
		{"path ignored", SearchResult{Owner: "team", Repo: "toolbox", Path: "skills/superpowers"}, "superpowers", 0.0},
		{"no match", SearchResult{Owner: "team", Repo: "toolbox"}, "superpowers", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := repositoryMatchScore(tt.result, tt.query)
			if math.Abs(got-tt.want) > 0.01 {
				t.Errorf("repositoryMatchScore() = %f, want %f", got, tt.want)
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
		{"five", 5, 0.23},
		{"ten", 10, 0.33},
		{"fifty", 50, 0.57},
		{"hundred", 100, 0.67},
		{"thousand", 1000, 1.0},
		{"ten thousand capped", 10000, 1.0},
		{"hundred thousand capped", 100000, 1.0},
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

	t.Run("repository match boosts popular source repo", func(t *testing.T) {
		officialRepoSkill := SearchResult{
			Name:        "using-superpowers",
			Description: "Use when starting any conversation",
			Owner:       "obra",
			Repo:        "superpowers",
			Path:        "skills/using-superpowers",
			Stars:       100000,
		}
		exactNameCopy := SearchResult{
			Name:        "superpowers",
			Description: "Core engineering workflow that activates on every task",
			Owner:       "someone",
			Repo:        "personal-skills",
			Path:        "skills/superpowers",
			Stars:       50,
		}

		officialScore := scoreResult(officialRepoSkill, "superpowers")
		copyScore := scoreResult(exactNameCopy, "superpowers")

		if officialScore <= copyScore {
			t.Errorf("popular repo match (%.3f) should beat lower-signal exact skill copy (%.3f)", officialScore, copyScore)
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

func TestBuildGitHubCodeSearchQuery(t *testing.T) {
	tests := []struct {
		desc  string
		query string
		want  string
	}{
		{
			desc:  "browse requires skill metadata",
			query: "",
			want:  `filename:SKILL.md "name:" "description:"`,
		},
		{
			desc:  "keyword search requires skill metadata",
			query: "react",
			want:  `filename:SKILL.md "name:" "description:" react`,
		},
		{
			desc:  "repo-scoped search stays repo scoped",
			query: "anthropics/skills/skills/pdf",
			want:  "filename:SKILL.md repo:anthropics/skills path:skills/pdf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := buildGitHubCodeSearchQuery(tt.query)
			if got != tt.want {
				t.Errorf("buildGitHubCodeSearchQuery(%q) = %q, want %q", tt.query, got, tt.want)
			}
		})
	}
}

func TestParseSkillMetadata(t *testing.T) {
	tests := []struct {
		desc      string
		content   string
		wantValid bool
		wantName  string
		wantDesc  string
	}{
		{
			desc: "valid skill with block description",
			content: `---
name: react-best-practices
description: >-
  React and Next.js performance guidance.
---
# React Best Practices
`,
			wantValid: true,
			wantName:  "react-best-practices",
			wantDesc:  "React and Next.js performance guidance.",
		},
		{
			desc: "valid skill with metadata and no description",
			content: `---
name: claude-only
metadata:
  targets: [claude]
---
# Claude Only
`,
			wantValid: true,
			wantName:  "claude-only",
		},
		{
			desc: "valid skill with one-line frontmatter",
			content: `--- name: using-superpowers description: Use when starting any conversation - establishes how to find and use skills ---
# Using Superpowers
`,
			wantValid: true,
			wantName:  "using-superpowers",
			wantDesc:  "Use when starting any conversation - establishes how to find and use skills",
		},
		{
			desc: "markdown only is not discoverable from broad GitHub search",
			content: `# SKILL

This is a generic markdown page that happens to use this filename.
`,
			wantValid: false,
		},
		{
			desc: "mixed-case name that install accepts is discoverable",
			content: `---
name: MySkill
description: Name with capitals is still installable
---
# MySkill
`,
			wantValid: true,
			wantName:  "MySkill",
			wantDesc:  "Name with capitals is still installable",
		},
		{
			desc: "leading underscore name that install rejects is not discoverable",
			content: `---
name: _private
description: Bare names starting with underscore are not installable
---
# Private
`,
			wantValid: false,
		},
		{
			desc: "invalid skill name is rejected",
			content: `---
name: "Sword Fighting"
description: Generic game skill notes
---
# Sword Fighting
`,
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := parseSkillMetadata(tt.content)
			if got.Valid != tt.wantValid {
				t.Fatalf("Valid = %v, want %v", got.Valid, tt.wantValid)
			}
			if got.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", got.Name, tt.wantName)
			}
			if got.Description != tt.wantDesc {
				t.Errorf("Description = %q, want %q", got.Description, tt.wantDesc)
			}
		})
	}
}

func TestParseFrontmatterField_OneLineFrontmatter(t *testing.T) {
	content := `--- name: using-superpowers description: Use when starting any conversation - establishes how to find and use skills ---
# Using Superpowers
`

	if got := parseFrontmatterField(content, "name"); got != "using-superpowers" {
		t.Fatalf("name = %q, want using-superpowers", got)
	}
	if got := parseFrontmatterField(content, "description"); got != "Use when starting any conversation - establishes how to find and use skills" {
		t.Fatalf("description = %q, want one-line description", got)
	}
}

func TestParseFrontmatterField_BOM(t *testing.T) {
	content := "\ufeff---\nname: bom-skill\ndescription: Written by Notepad\n---\n# Body\n"

	if got := parseFrontmatterField(content, "name"); got != "bom-skill" {
		t.Fatalf("name = %q, want bom-skill", got)
	}
	// The fallback line scanner runs when the YAML does not parse.
	broken := "\ufeff---\nname: bom-skill\nbad: [unclosed\n---\n"
	if got := parseFrontmatterField(broken, "name"); got != "bom-skill" {
		t.Fatalf("fallback name = %q, want bom-skill", got)
	}
}

func TestBuildTrustedSkillRepoSearchQuery(t *testing.T) {
	got := buildTrustedSkillRepoSearchQuery("frontend-design", "anthropics/skills")
	want := `filename:SKILL.md repo:anthropics/skills "name:" "description:" frontend-design`
	if got != want {
		t.Fatalf("buildTrustedSkillRepoSearchQuery() = %q, want %q", got, want)
	}
}

func TestRepositorySkillTreeItems(t *testing.T) {
	repo := gitHubRepositorySearchItem{
		FullName:        "owner/popular-tools",
		StargazersCount: 1234,
		Description:     "Popular tools with skills",
	}
	tree := []gitHubTreeItem{
		{Path: "README.md", Type: "blob"},
		{Path: "skills/alpha/SKILL.md", Type: "blob"},
		{Path: "skills/beta/SKILL.md", Type: "blob"},
		{Path: "skills/gamma/SKILL.md", Type: "tree"},
	}

	items := repositorySkillTreeItems(repo, tree, 10)
	results := processSearchItems(items)

	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].Source != "owner/popular-tools/skills/alpha" {
		t.Fatalf("source[0] = %q, want owner/popular-tools/skills/alpha", results[0].Source)
	}
	if results[0].Stars != 1234 {
		t.Fatalf("stars = %d, want 1234", results[0].Stars)
	}
	if results[1].Name != "beta" {
		t.Fatalf("name[1] = %q, want beta", results[1].Name)
	}
}

func TestFilterLowQuality(t *testing.T) {
	tests := []struct {
		desc      string
		results   []SearchResult
		wantCount int
		wantNames []string
	}{
		{
			desc: "filters zero-star non-preferred repos",
			results: []SearchResult{
				{Name: "good-skill", Stars: 10, Owner: "someone", Repo: "skills"},
				{Name: "zero-star", Stars: 0, Owner: "random", Repo: "test"},
				{Name: "also-good", Stars: 5, Owner: "dev", Repo: "tools"},
			},
			wantCount: 2,
			wantNames: []string{"good-skill", "also-good"},
		},
		{
			desc: "keeps zero-star preferred repos",
			results: []SearchResult{
				{Name: "new-skill", Stars: 0, Owner: "anthropics", Repo: "skills"},
				{Name: "zero-star", Stars: 0, Owner: "random", Repo: "test"},
			},
			wantCount: 1,
			wantNames: []string{"new-skill"},
		},
		{
			desc: "filters short descriptions",
			results: []SearchResult{
				{Name: "good", Stars: 5, Description: "A comprehensive tool for code review", Owner: "a", Repo: "b"},
				{Name: "stub", Stars: 5, Description: "test", Owner: "c", Repo: "d"},
			},
			wantCount: 1,
			wantNames: []string{"good"},
		},
		{
			desc: "keeps results with no description (fetch may have failed)",
			results: []SearchResult{
				{Name: "no-desc", Stars: 5, Description: "", Owner: "a", Repo: "b"},
			},
			wantCount: 1,
			wantNames: []string{"no-desc"},
		},
		{
			desc: "filters spam orgs",
			results: []SearchResult{
				{Name: "legit", Stars: 10, Owner: "dev", Repo: "skills"},
				{Name: "spam", Stars: 100, Owner: "inference-sh", Repo: "skills"},
			},
			wantCount: 1,
			wantNames: []string{"legit"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := filterLowQuality(tt.results)
			if len(got) != tt.wantCount {
				t.Fatalf("got %d results, want %d", len(got), tt.wantCount)
			}
			for i, name := range tt.wantNames {
				if got[i].Name != name {
					t.Errorf("result[%d].Name = %q, want %q", i, got[i].Name, name)
				}
			}
		})
	}
}

func TestIsLowQualityResult(t *testing.T) {
	tests := []struct {
		desc   string
		result SearchResult
		want   bool
	}{
		{"zero stars", SearchResult{Stars: 0, Owner: "x", Repo: "y"}, true},
		{"one star", SearchResult{Stars: 1, Owner: "x", Repo: "y"}, false},
		{"good result", SearchResult{Stars: 10, Description: "A useful skill", Owner: "x", Repo: "y"}, false},
		{"short desc", SearchResult{Stars: 5, Description: "hi", Owner: "x", Repo: "y"}, true},
		{"9-char desc", SearchResult{Stars: 5, Description: "123456789", Owner: "x", Repo: "y"}, true},
		{"10-char desc ok", SearchResult{Stars: 5, Description: "1234567890", Owner: "x", Repo: "y"}, false},
		{"empty desc ok", SearchResult{Stars: 5, Description: "", Owner: "x", Repo: "y"}, false},
		{"spam org", SearchResult{Stars: 100, Owner: "inference-sh", Repo: "skills"}, true},
		{"spam org case insensitive", SearchResult{Stars: 100, Owner: "Inference-SH", Repo: "skills"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := isLowQualityResult(tt.result)
			if got != tt.want {
				t.Errorf("isLowQualityResult() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDedupeEquivalentSkills_PrefersTrustedSource(t *testing.T) {
	results := []SearchResult{
		{
			Name:        "frontend-design",
			Description: "Create distinctive, production-grade frontend interfaces.",
			Source:      "random/app/.claude/skills/frontend-design",
			Owner:       "random",
			Repo:        "app",
			Path:        ".claude/skills/frontend-design",
			Stars:       5000,
		},
		{
			Name:        "frontend-design",
			Description: "Create distinctive, production-grade frontend interfaces.",
			Source:      "anthropics/skills/skills/frontend-design",
			Owner:       "anthropics",
			Repo:        "skills",
			Path:        "skills/frontend-design",
			Stars:       10,
		},
	}

	got := dedupeEquivalentSkills(results)
	if len(got) != 1 {
		t.Fatalf("got %d results, want 1", len(got))
	}
	if got[0].Source != "anthropics/skills/skills/frontend-design" {
		t.Fatalf("source = %q, want trusted source", got[0].Source)
	}
}

func TestDedupeEquivalentSkills_MergesExtendedDescriptions(t *testing.T) {
	results := []SearchResult{
		{
			Name:        "frontend-design",
			Description: "Create distinctive, production-grade frontend interfaces with high design quality. Use this skill when the user asks to build web components, pages, artifacts, posters, or applications.",
			Source:      "copy/app/.agent/skills/frontend-design",
			Owner:       "copy",
			Repo:        "app",
			Path:        ".agent/skills/frontend-design",
			Stars:       50,
		},
		{
			Name:        "frontend-design",
			Description: "Create distinctive, production-grade frontend interfaces with high design quality. Use this skill when the user asks to build web components, pages, artifacts, posters, or applications (examples include websites, landing pages, dashboards, React components, HTML/CSS layouts, or when styling/beautifying any web UI). Generates creative, polished code and UI design that avoids generic AI aesthetics.",
			Source:      "anthropics/skills/skills/frontend-design",
			Owner:       "anthropics",
			Repo:        "skills",
			Path:        "skills/frontend-design",
		},
	}

	got := dedupeEquivalentSkills(results)
	if len(got) != 1 {
		t.Fatalf("got %d results, want 1", len(got))
	}
	if got[0].Source != "anthropics/skills/skills/frontend-design" {
		t.Fatalf("source = %q, want trusted source", got[0].Source)
	}
}

func TestDedupeEquivalentSkills_KeepsDistinctDescriptions(t *testing.T) {
	results := []SearchResult{
		{Name: "frontend-design", Description: "Create bold UI designs.", Source: "owner/a/skills/frontend-design", Owner: "owner", Repo: "a", Path: "skills/frontend-design"},
		{Name: "frontend-design", Description: "Create frontend design docs.", Source: "owner/b/skills/frontend-design", Owner: "owner", Repo: "b", Path: "skills/frontend-design"},
	}

	got := dedupeEquivalentSkills(results)
	if len(got) != 2 {
		t.Fatalf("got %d results, want 2", len(got))
	}
}
