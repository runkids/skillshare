//go:build online

package integration

import (
	"path/filepath"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/testutil"
)

// TestInstall_RemoteGitHubSubdir_DryRun validates a network-backed install path.
// This test is excluded from default runs and only enabled with -tags online.
func TestInstall_RemoteGitHubSubdir_DryRun(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("install", "runkids/skillshare/skills/skillshare", "--dry-run")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Dry run — nothing was written")
}

// TestInstall_RemoteGitHub_Clone installs a real skill from GitHub and verifies the files land.
func TestInstall_RemoteGitHub_Clone(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("install", "runkids/skillshare/skills/skillshare")

	result.AssertSuccess(t)

	// Verify the skill directory was created in source
	skillDir := filepath.Join(sb.SourcePath, "skillshare")
	if !sb.FileExists(skillDir) {
		t.Errorf("expected skill directory %s to exist after install", skillDir)
	}
	// Verify SKILL.md exists inside the installed skill
	if !sb.FileExists(filepath.Join(skillDir, "SKILL.md")) {
		t.Errorf("expected SKILL.md inside installed skill directory")
	}
}

// TestInstall_RemoteGitHub_Track clones an entire repo with --track and verifies the tracked directory.
func TestInstall_RemoteGitHub_Track(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// This repository intentionally contains malicious-pattern fixtures in tests/docs.
	// Use --force so this test validates track mechanics, not audit blocking policy.
	result := sb.RunCLI("install", "runkids/skillshare", "--track", "--name", "test-tracked", "--force")

	result.AssertSuccess(t)

	// Tracked repos get _ prefix
	trackedDir := filepath.Join(sb.SourcePath, "_test-tracked")
	if !sb.FileExists(trackedDir) {
		t.Errorf("expected tracked directory %s to exist", trackedDir)
	}
	// Should be a git clone
	if !sb.FileExists(filepath.Join(trackedDir, ".git")) {
		t.Errorf("expected .git inside tracked directory (should be a git clone)")
	}
	// Output should mention skill count
	result.AssertAnyOutputContains(t, "skill")
}

func TestInstall_GitHubSubdirViaAPI(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	source := "majiayu000/claude-skill-registry/skills/documents/atlassian-search"
	result := sb.RunCLI("install", source)
	result.AssertSuccess(t)

	skillDir := filepath.Join(sb.SourcePath, "atlassian-search")
	if !sb.FileExists(skillDir) {
		t.Fatalf("expected skill directory %s to exist", skillDir)
	}
	if !sb.FileExists(filepath.Join(skillDir, "SKILL.md")) {
		t.Fatalf("expected SKILL.md in %s", skillDir)
	}
	if sb.FileExists(filepath.Join(skillDir, ".git")) {
		t.Fatalf("did not expect .git directory for subdir API install")
	}

	// Verify metadata in centralized .metadata.json store
	store, storeErr := install.LoadMetadata(sb.SourcePath)
	if storeErr != nil {
		t.Fatalf("failed to load metadata store: %v", storeErr)
	}
	entry := store.Get("atlassian-search")
	if entry == nil {
		t.Fatal("expected metadata entry for atlassian-search in centralized store")
	}
	if entry.Source != "github.com/majiayu000/claude-skill-registry/skills/documents/atlassian-search" {
		t.Fatalf("expected source to preserve subdir, got: %s", entry.Source)
	}
	if entry.Subdir != "skills/documents/atlassian-search" {
		t.Fatalf("expected subdir to match install path, got: %s", entry.Subdir)
	}
}
