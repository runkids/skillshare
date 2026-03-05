//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
	// JSON mode is fully silent — no progress text on stdout or stderr.
	combined := result.Stdout + result.Stderr
	if strings.Contains(combined, "Browsing popular skills") {
		t.Fatalf("expected no progress text in JSON mode, got:\nstdout: %s\nstderr: %s", result.Stdout, result.Stderr)
	}
}

func TestSearch_HubLabel_ResolvesFromConfig(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	indexPath := writeIndexFile(t, sb.Home, []map[string]string{
		{"name": "my-skill", "description": "A skill", "source": "owner/repo/my-skill"},
	})

	// Write config with a saved hub
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\nhub:\n  hubs:\n    - label: test\n      url: " + indexPath + "\n")

	result := sb.RunCLI("search", "my-skill", "--hub", "test", "--list")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "my-skill")
}

func TestSearch_HubLabel_NotFound(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("search", "x", "--hub", "nonexistent", "--list")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "not found")
}

func TestSearch_HubBare_UsesDefault(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	indexPath := writeIndexFile(t, sb.Home, []map[string]string{
		{"name": "default-skill", "source": "a/b"},
	})

	// Config with default hub set
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\nhub:\n  default: myhub\n  hubs:\n    - label: myhub\n      url: " + indexPath + "\n")

	result := sb.RunCLI("search", "--hub", "--json")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "default-skill")
}

func TestSearch_HubBare_FallbackToCommunity(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// No default set — bare --hub should use community hub (which will fail in offline test,
	// but we can verify the URL is attempted, not that it returned a label error)
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	// This will try to fetch the community hub URL which will fail in offline mode,
	// but it should NOT produce "not found" label error — it should try the URL
	result := sb.RunCLI("search", "--hub", "--json")
	// The search itself may fail (network) but shouldn't error with "not found"
	combined := result.Stdout + result.Stderr
	if strings.Contains(combined, "not found; run") {
		t.Errorf("bare --hub with no default should fallback to community hub, not label error")
	}
}
