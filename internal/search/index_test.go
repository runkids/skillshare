package search

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestIndex(t *testing.T, dir, filename, content string) string {
	t.Helper()
	p := filepath.Join(dir, filename)
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", filename, err)
	}
	return p
}

func TestSearchFromIndexURL_LocalFile(t *testing.T) {
	dir := t.TempDir()
	indexPath := writeTestIndex(t, dir, "index.json", `{
		"schemaVersion": 1,
		"skills": [
			{"name": "react-patterns", "description": "React perf", "source": "facebook/react/.claude/skills/react-patterns"},
			{"name": "deploy-helper", "description": "K8s deploy", "source": "gitlab.com/ops/skills/deploy-helper"}
		]
	}`)

	results, err := SearchFromIndexURL("react", 20, indexPath)
	if err != nil {
		t.Fatalf("SearchFromIndexURL: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Name != "react-patterns" {
		t.Errorf("name = %q, want react-patterns", results[0].Name)
	}
}

func TestSearchFromIndexURL_EmptyQuery(t *testing.T) {
	dir := t.TempDir()
	indexPath := writeTestIndex(t, dir, "index.json", `{
		"schemaVersion": 1,
		"skills": [
			{"name": "alpha", "source": "a/b"},
			{"name": "beta", "source": "c/d"},
			{"name": "gamma", "source": "e/f"}
		]
	}`)

	results, err := SearchFromIndexURL("", 20, indexPath)
	if err != nil {
		t.Fatalf("SearchFromIndexURL: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3 (all skills)", len(results))
	}
}

func TestSearchFromIndexURL_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	indexPath := writeTestIndex(t, dir, "index.json", `{not valid json}`)

	_, err := SearchFromIndexURL("test", 20, indexPath)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestSearchFromIndexURL_LimitRespected(t *testing.T) {
	dir := t.TempDir()
	indexPath := writeTestIndex(t, dir, "index.json", `{
		"schemaVersion": 1,
		"skills": [
			{"name": "a", "source": "x/a"},
			{"name": "b", "source": "x/b"},
			{"name": "c", "source": "x/c"},
			{"name": "d", "source": "x/d"},
			{"name": "e", "source": "x/e"}
		]
	}`)

	results, err := SearchFromIndexURL("", 2, indexPath)
	if err != nil {
		t.Fatalf("SearchFromIndexURL: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2 (limited)", len(results))
	}
}

func TestSearchFromIndexURL_SourcePathJoin(t *testing.T) {
	dir := t.TempDir()
	indexPath := writeTestIndex(t, dir, "index.json", `{
		"schemaVersion": 1,
		"sourcePath": "/home/user/skills",
		"skills": [
			{"name": "my-skill", "source": "_team/frontend/my-skill"}
		]
	}`)

	results, err := SearchFromIndexURL("", 20, indexPath)
	if err != nil {
		t.Fatalf("SearchFromIndexURL: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	want := filepath.Join("/home/user/skills", "_team/frontend/my-skill")
	if results[0].Source != want {
		t.Errorf("source = %q, want %q", results[0].Source, want)
	}
}

func TestSearchFromIndexURL_NoSourcePath(t *testing.T) {
	dir := t.TempDir()
	indexPath := writeTestIndex(t, dir, "index.json", `{
		"schemaVersion": 1,
		"skills": [
			{"name": "remote", "source": "github.com/owner/repo/skill"}
		]
	}`)

	results, err := SearchFromIndexURL("", 20, indexPath)
	if err != nil {
		t.Fatalf("SearchFromIndexURL: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Source != "github.com/owner/repo/skill" {
		t.Errorf("source = %q, want original", results[0].Source)
	}
}

func TestSearchFromIndexURL_EmptyURL(t *testing.T) {
	_, err := SearchFromIndexURL("test", 20, "")
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestSearchFromIndexURL_SkipsEmptyName(t *testing.T) {
	dir := t.TempDir()
	indexPath := writeTestIndex(t, dir, "index.json", `{
		"schemaVersion": 1,
		"skills": [
			{"name": "", "source": "x/y"},
			{"name": "valid", "source": "a/b"}
		]
	}`)

	results, err := SearchFromIndexURL("", 20, indexPath)
	if err != nil {
		t.Fatalf("SearchFromIndexURL: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1 (skip empty name)", len(results))
	}
}

func TestSearchFromIndexURL_RiskFields(t *testing.T) {
	dir := t.TempDir()
	indexPath := writeTestIndex(t, dir, "index.json", `{
		"schemaVersion": 1,
		"skills": [
			{"name": "safe-skill", "source": "a/b", "riskScore": 0, "riskLabel": "clean"},
			{"name": "risky-skill", "source": "c/d", "riskScore": 42, "riskLabel": "medium"},
			{"name": "unaudited-skill", "source": "e/f"}
		]
	}`)

	results, err := SearchFromIndexURL("", 20, indexPath)
	if err != nil {
		t.Fatalf("SearchFromIndexURL: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}

	// Results are sorted by name: risky, safe, unaudited.
	risky := results[0] // risky-skill
	safe := results[1]  // safe-skill
	unaud := results[2] // unaudited-skill

	if risky.Name != "risky-skill" {
		t.Fatalf("expected risky-skill first (sorted), got %q", risky.Name)
	}

	// safe-skill: riskScore=0, riskLabel=clean
	if safe.RiskScore == nil {
		t.Fatal("safe-skill: riskScore should not be nil")
	}
	if *safe.RiskScore != 0 {
		t.Errorf("safe-skill: riskScore = %d, want 0", *safe.RiskScore)
	}
	if safe.RiskLabel != "clean" {
		t.Errorf("safe-skill: riskLabel = %q, want 'clean'", safe.RiskLabel)
	}

	// risky-skill: riskScore=42, riskLabel=medium
	if risky.RiskScore == nil {
		t.Fatal("risky-skill: riskScore should not be nil")
	}
	if *risky.RiskScore != 42 {
		t.Errorf("risky-skill: riskScore = %d, want 42", *risky.RiskScore)
	}
	if risky.RiskLabel != "medium" {
		t.Errorf("risky-skill: riskLabel = %q, want 'medium'", risky.RiskLabel)
	}

	// unaudited-skill: riskScore=nil, riskLabel=""
	if unaud.RiskScore != nil {
		t.Errorf("unaudited-skill: riskScore should be nil, got %d", *unaud.RiskScore)
	}
	if unaud.RiskLabel != "" {
		t.Errorf("unaudited-skill: riskLabel should be empty, got %q", unaud.RiskLabel)
	}
}

func TestIsRelativeSource(t *testing.T) {
	tests := []struct {
		source string
		want   bool
	}{
		// Relative paths (should return true)
		{"_team/frontend/skill", true},
		{"my-skill", true},
		{"subdir/skill", true},
		{"owner/repo/path", true}, // No dot in first segment → GitHub shorthand, but still "relative" in index context

		// Absolute paths (should return false)
		{"/shared/nfs/skills/x", false},
		{"~/skills/x", false},

		// Remote URLs (should return false)
		{"git@gitlab.com:team/repo.git//x", false},
		{"http://example.com/index.json", false},
		{"https://gitlab.com/team/repo/x", false},
		{"file:///path/x", false},

		// Domain detection (should return false)
		{"gitlab.com/team/repo/x", false},
		{"bitbucket.org/team/repo/x", false},
		{"gitea.company.com/team/repo/x", false},
		{"github.com/owner/repo/skill", false},

		// Windows paths (should return false)
		{`C:\skills\foo`, false},
		{`D:/projects/skill`, false},
		{`\\server\share\skills\foo`, false},
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			got := isRelativeSource(tt.source)
			if got != tt.want {
				t.Errorf("isRelativeSource(%q) = %v, want %v", tt.source, got, tt.want)
			}
		})
	}
}
