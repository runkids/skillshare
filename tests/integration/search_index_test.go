//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func writeIndexFile(t *testing.T, dir string, skills []map[string]string) string {
	t.Helper()
	idx := map[string]any{
		"schemaVersion": 1,
		"skills":        skills,
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		t.Fatalf("marshal index: %v", err)
	}
	path := filepath.Join(dir, "index.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	return path
}

func TestSearch_IndexURL_LocalFile(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	indexPath := writeIndexFile(t, sb.Home, []map[string]string{
		{"name": "react-patterns", "description": "React best practices", "source": "facebook/react/.claude/skills/react-patterns"},
		{"name": "deploy-helper", "description": "K8s deployment", "source": "gitlab.com/ops/skills/deploy-helper"},
	})

	result := sb.RunCLI("search", "react", "--hub", indexPath, "--list")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "react-patterns")
}

func TestSearch_IndexURL_NoResults(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	indexPath := writeIndexFile(t, sb.Home, []map[string]string{
		{"name": "alpha", "source": "a/b"},
	})

	result := sb.RunCLI("search", "zzz-nonexistent", "--hub", indexPath, "--list")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "No skills found")
}

func TestSearch_IndexURL_EqualsSyntax(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	indexPath := writeIndexFile(t, sb.Home, []map[string]string{
		{"name": "test-skill", "description": "A test skill", "source": "owner/repo/test-skill"},
	})

	// Test --hub=value syntax
	result := sb.RunCLI("search", "test", "--hub="+indexPath, "--list")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "test-skill")
}

func TestSearch_IndexURL_BrowseAll(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	indexPath := writeIndexFile(t, sb.Home, []map[string]string{
		{"name": "alpha", "source": "a/b"},
		{"name": "beta", "source": "c/d"},
		{"name": "gamma", "source": "e/f"},
	})

	// Empty query returns all — use --json to avoid interactive prompt
	result := sb.RunCLI("search", "--hub", indexPath, "--json")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "alpha")
	result.AssertAnyOutputContains(t, "beta")
	result.AssertAnyOutputContains(t, "gamma")
}

func TestSearch_IndexURL_SpinnerText(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	indexPath := writeIndexFile(t, sb.Home, []map[string]string{
		{"name": "x", "source": "a/b"},
	})

	result := sb.RunCLI("search", "--hub", indexPath, "--json")
	result.AssertSuccess(t)
	// JSON mode shows progress on stderr
	result.AssertAnyOutputContains(t, "Browsing popular skills")
}
