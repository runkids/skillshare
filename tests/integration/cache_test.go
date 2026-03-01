//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestCache_List_Empty(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("cache", "list")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Discovery Cache")
	result.AssertAnyOutputContains(t, "(none)")
	result.AssertAnyOutputContains(t, "Total:")
}

func TestCache_Clean_Empty(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("cache", "clean", "--yes")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "No cache files")
}

func TestCache_List_WithDiscoveryCache(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("test-skill", map[string]string{
		"SKILL.md": "---\nname: test-skill\n---\n# Test",
	})

	claudeDir := filepath.Join(sb.Home, ".claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeDir + `
`)

	// Run sync to trigger full Discover() which writes L2 disk cache
	// (list uses DiscoverLite which is L1-only)
	syncResult := sb.RunCLI("sync")
	syncResult.AssertSuccess(t)

	// Now check cache list shows the gob file
	result := sb.RunCLI("cache", "list")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "discovery-")
	result.AssertAnyOutputContains(t, "1 skills")
	result.AssertAnyOutputContains(t, "valid")
}

func TestCache_Clean_RemovesAll(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("cache-skill", map[string]string{
		"SKILL.md": "---\nname: cache-skill\n---\n# Cache",
	})

	claudeDir := filepath.Join(sb.Home, ".claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeDir + `
`)

	// Trigger full Discover() via sync to write L2 disk cache
	sb.RunCLI("sync")

	// Verify gob exists
	cacheDir := filepath.Join(sb.Home, ".cache", "skillshare")
	matches, _ := filepath.Glob(filepath.Join(cacheDir, "discovery-*.gob"))
	if len(matches) == 0 {
		t.Fatal("expected at least one discovery gob after sync")
	}

	// Clean all
	result := sb.RunCLI("cache", "clean", "--yes")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Removed")

	// Verify gob is gone
	matches, _ = filepath.Glob(filepath.Join(cacheDir, "discovery-*.gob"))
	if len(matches) != 0 {
		t.Errorf("expected 0 gob files after clean, got %d", len(matches))
	}
}

func TestCache_Clean_OrphanOnly(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("orphan-skill", map[string]string{
		"SKILL.md": "---\nname: orphan-skill\n---\n# Orphan",
	})

	claudeDir := filepath.Join(sb.Home, ".claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeDir + `
`)

	// Trigger full Discover() via sync to write L2 disk cache
	sb.RunCLI("sync")

	// With valid source still existing, --orphan should find nothing
	result := sb.RunCLI("cache", "clean", "--orphan", "--yes")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "No orphan")

	// Gob files should still exist
	cacheDir := filepath.Join(sb.Home, ".cache", "skillshare")
	matches, _ := filepath.Glob(filepath.Join(cacheDir, "discovery-*.gob"))
	if len(matches) == 0 {
		t.Error("gob files should still exist after --orphan when source is valid")
	}
}

func TestCache_Help(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("cache", "--help")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Usage:")
	result.AssertAnyOutputContains(t, "clean")
	result.AssertAnyOutputContains(t, "list")
}

func TestCache_UnknownSubcommand(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("cache", "bogus")
	result.AssertFailure(t)
}

func TestCache_List_OrphanAfterSourceRemoved(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("gone-skill", map[string]string{
		"SKILL.md": "---\nname: gone-skill\n---\n# Gone",
	})

	claudeDir := filepath.Join(sb.Home, ".claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeDir + `
`)

	// Sync writes L2 disk cache with RootDir = sb.SourcePath
	syncResult := sb.RunCLI("sync")
	syncResult.AssertSuccess(t)

	// Remove the source directory — gob now points to nonexistent path
	os.RemoveAll(sb.SourcePath)

	result := sb.RunCLI("cache", "list")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "orphan")
}

func TestCache_Clean_ThenListEmpty(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("clean-skill", map[string]string{
		"SKILL.md": "---\nname: clean-skill\n---\n# Clean",
	})

	claudeDir := filepath.Join(sb.Home, ".claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeDir + `
`)

	sb.RunCLI("sync")

	// Clean all
	cleanResult := sb.RunCLI("cache", "clean", "--yes")
	cleanResult.AssertSuccess(t)

	// List should show (none) for discovery
	listResult := sb.RunCLI("cache", "list")
	listResult.AssertSuccess(t)
	listResult.AssertAnyOutputContains(t, "(none)")
}

func TestCache_Clean_OrphanRemovesOnlyOrphans(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create skill and sync to produce a valid cache gob
	sb.CreateSkill("keep-skill", map[string]string{
		"SKILL.md": "---\nname: keep-skill\n---\n# Keep",
	})

	claudeDir := filepath.Join(sb.Home, ".claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeDir + `
`)

	sb.RunCLI("sync")

	// Manually plant an orphan gob file (RootDir doesn't exist)
	cacheDir := filepath.Join(sb.Home, ".cache", "skillshare")
	orphanGob := filepath.Join(cacheDir, "discovery-orphantest.gob")
	os.WriteFile(orphanGob, []byte("not a real gob"), 0644)

	// Verify we have at least 2 gob files (1 real + 1 fake)
	matches, _ := filepath.Glob(filepath.Join(cacheDir, "discovery-*.gob"))
	if len(matches) < 2 {
		t.Fatalf("expected at least 2 gob files, got %d", len(matches))
	}

	// Clean orphans only — the corrupt fake gob is NOT an orphan (it has decode error),
	// so only true orphans would be cleaned. Our valid gob should survive.
	result := sb.RunCLI("cache", "clean", "--orphan", "--yes")
	result.AssertSuccess(t)

	// The valid cache should still exist (its source dir is still there)
	matches, _ = filepath.Glob(filepath.Join(cacheDir, "discovery-*.gob"))
	validFound := false
	for _, m := range matches {
		if filepath.Base(m) != "discovery-orphantest.gob" {
			validFound = true
		}
	}
	if !validFound {
		t.Error("valid cache gob should not have been removed by --orphan clean")
	}
}

func TestCache_NoTUI_Fallback(t *testing.T) {
	// When run without a TTY (i.e., in tests), `cache` with no args
	// should fallback to `cache list` instead of TUI.
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create a fake UI cache version dir for coverage
	uiDir := filepath.Join(sb.Home, ".cache", "skillshare", "ui", "v0.1.0")
	os.MkdirAll(uiDir, 0755)
	os.WriteFile(filepath.Join(uiDir, "index.html"), []byte("<html/>"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("cache")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Discovery Cache")
	result.AssertAnyOutputContains(t, "UI Cache")
	result.AssertAnyOutputContains(t, "v0.1.0")
}
