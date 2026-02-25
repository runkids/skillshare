//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

// setupMultiSkillRepo creates a bare remote with two skills under skills/,
// installs both via CLI, then deletes one skill from the remote and pushes.
// Returns (remoteBare, keepName, deletedName).
func setupMultiSkillRepo(t *testing.T, sb *testutil.Sandbox) (string, string, string) {
	t.Helper()

	remoteRepo := filepath.Join(sb.Root, "multi-skill.git")
	workClone := filepath.Join(sb.Root, "work-multi")
	gitInit(t, remoteRepo, true)
	gitClone(t, remoteRepo, workClone)

	// Two skills in the repo
	os.MkdirAll(filepath.Join(workClone, "skills", "keep-skill"), 0755)
	os.WriteFile(filepath.Join(workClone, "skills", "keep-skill", "SKILL.md"),
		[]byte("---\nname: keep-skill\n---\n# Keep"), 0644)
	os.MkdirAll(filepath.Join(workClone, "skills", "stale-skill"), 0755)
	os.WriteFile(filepath.Join(workClone, "skills", "stale-skill", "SKILL.md"),
		[]byte("---\nname: stale-skill\n---\n# Stale"), 0644)
	gitAddCommit(t, workClone, "add two skills")
	gitPush(t, workClone)

	// Install both skills via CLI
	installResult := sb.RunCLI("install", "file://"+remoteRepo+"//skills/keep-skill", "--skip-audit")
	installResult.AssertSuccess(t)
	installResult2 := sb.RunCLI("install", "file://"+remoteRepo+"//skills/stale-skill", "--skip-audit")
	installResult2.AssertSuccess(t)

	// Delete the stale skill from remote and push
	os.RemoveAll(filepath.Join(workClone, "skills", "stale-skill"))
	os.WriteFile(filepath.Join(workClone, "skills", "keep-skill", "SKILL.md"),
		[]byte("---\nname: keep-skill\n---\n# Keep v2"), 0644)
	gitAddCommit(t, workClone, "remove stale-skill, update keep-skill")
	gitPush(t, workClone)

	return remoteRepo, "keep-skill", "stale-skill"
}

func TestUpdate_Prune_RemovesStaleSkill(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	_, keepName, staleName := setupMultiSkillRepo(t, sb)

	// Verify both skills exist
	if _, err := os.Stat(filepath.Join(sb.SourcePath, keepName)); err != nil {
		t.Fatalf("keep-skill should exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(sb.SourcePath, staleName)); err != nil {
		t.Fatalf("stale-skill should exist before prune: %v", err)
	}

	// update --all --prune --skip-audit
	result := sb.RunCLI("update", "--all", "--prune", "--skip-audit")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "pruned")
	result.AssertAnyOutputContains(t, staleName)

	// stale-skill should be removed (moved to trash)
	if _, err := os.Stat(filepath.Join(sb.SourcePath, staleName)); !os.IsNotExist(err) {
		t.Error("stale-skill should have been removed from source")
	}

	// keep-skill should still exist and be updated
	keepContent := sb.ReadFile(filepath.Join(sb.SourcePath, keepName, "SKILL.md"))
	if keepContent == "" {
		t.Fatal("keep-skill should still exist after prune")
	}
}

func TestUpdate_StaleWarning_NoPruneFlag(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	_, _, staleName := setupMultiSkillRepo(t, sb)

	// update --all (without --prune) --skip-audit
	result := sb.RunCLI("update", "--all", "--skip-audit")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "stale")
	result.AssertAnyOutputContains(t, "--prune")
	result.AssertAnyOutputContains(t, staleName)

	// stale-skill should still exist (not pruned)
	if _, err := os.Stat(filepath.Join(sb.SourcePath, staleName)); err != nil {
		t.Error("stale-skill should still exist without --prune flag")
	}
}

func TestUpdate_Prune_RegistryCleanup(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	_, _, staleName := setupMultiSkillRepo(t, sb)

	result := sb.RunCLI("update", "--all", "--prune", "--skip-audit")
	result.AssertSuccess(t)

	// Check registry does not contain the stale skill
	regPath := filepath.Join(filepath.Dir(sb.ConfigPath), "registry.yaml")
	if _, err := os.Stat(regPath); err == nil {
		regContent := sb.ReadFile(regPath)
		if contains(regContent, staleName) {
			t.Errorf("registry should not contain pruned skill %q", staleName)
		}
	}
}

// TestUpdate_Prune_AllStale verifies that when ALL skills from a repo are
// deleted upstream, --prune removes them all without panic or error.
func TestUpdate_Prune_AllStale(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	remoteRepo := filepath.Join(sb.Root, "all-stale.git")
	workClone := filepath.Join(sb.Root, "all-stale-work")
	gitInit(t, remoteRepo, true)
	gitClone(t, remoteRepo, workClone)

	// Two skills, both will be deleted
	for _, name := range []string{"alpha", "beta"} {
		os.MkdirAll(filepath.Join(workClone, "skills", name), 0755)
		os.WriteFile(filepath.Join(workClone, "skills", name, "SKILL.md"),
			[]byte("---\nname: "+name+"\n---\n# "+name), 0644)
	}
	gitAddCommit(t, workClone, "add two skills")
	gitPush(t, workClone)

	// Install both
	for _, name := range []string{"alpha", "beta"} {
		r := sb.RunCLI("install", "file://"+remoteRepo+"//skills/"+name, "--skip-audit")
		r.AssertSuccess(t)
	}

	// Delete ALL skills from remote
	os.RemoveAll(filepath.Join(workClone, "skills", "alpha"))
	os.RemoveAll(filepath.Join(workClone, "skills", "beta"))
	os.MkdirAll(filepath.Join(workClone, "skills"), 0755) // keep dir so git tracks it
	os.WriteFile(filepath.Join(workClone, "skills", ".gitkeep"), []byte(""), 0644)
	gitAddCommit(t, workClone, "remove all skills")
	gitPush(t, workClone)

	result := sb.RunCLI("update", "--all", "--prune", "--skip-audit")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "pruned")

	// Both should be gone
	for _, name := range []string{"alpha", "beta"} {
		if _, err := os.Stat(filepath.Join(sb.SourcePath, name)); !os.IsNotExist(err) {
			t.Errorf("%s should have been pruned", name)
		}
	}

	// Registry should not contain either
	regPath := filepath.Join(filepath.Dir(sb.ConfigPath), "registry.yaml")
	if _, err := os.Stat(regPath); err == nil {
		regContent := sb.ReadFile(regPath)
		for _, name := range []string{"alpha", "beta"} {
			if contains(regContent, name) {
				t.Errorf("registry should not contain pruned skill %q", name)
			}
		}
	}
}

// TestUpdate_Prune_StandaloneSkill verifies Phase 3 stale detection for
// standalone skills (not grouped by repo URL).
func TestUpdate_Prune_StandaloneSkill(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	remoteRepo := filepath.Join(sb.Root, "standalone.git")
	workClone := filepath.Join(sb.Root, "standalone-work")
	gitInit(t, remoteRepo, true)
	gitClone(t, remoteRepo, workClone)

	// Single skill in a unique subdir
	os.MkdirAll(filepath.Join(workClone, "my-skill"), 0755)
	os.WriteFile(filepath.Join(workClone, "my-skill", "SKILL.md"),
		[]byte("---\nname: my-skill\n---\n# My Skill"), 0644)
	gitAddCommit(t, workClone, "add skill")
	gitPush(t, workClone)

	r := sb.RunCLI("install", "file://"+remoteRepo+"//my-skill", "--skip-audit")
	r.AssertSuccess(t)

	// Delete skill from remote
	os.RemoveAll(filepath.Join(workClone, "my-skill"))
	os.WriteFile(filepath.Join(workClone, ".gitkeep"), []byte(""), 0644)
	gitAddCommit(t, workClone, "remove skill")
	gitPush(t, workClone)

	// Without --prune: should warn
	result := sb.RunCLI("update", "--all", "--skip-audit")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "stale")

	// With --prune: should remove
	result2 := sb.RunCLI("update", "--all", "--prune", "--skip-audit")
	result2.AssertSuccess(t)
	result2.AssertAnyOutputContains(t, "Pruned")

	if _, err := os.Stat(filepath.Join(sb.SourcePath, "my-skill")); !os.IsNotExist(err) {
		t.Error("standalone stale skill should have been pruned")
	}
}

// TestUpdate_Prune_NestedIntoSkill verifies that skills installed with --into
// (nested under a group directory) are correctly pruned and cleaned from registry.
func TestUpdate_Prune_NestedIntoSkill(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	remoteRepo := filepath.Join(sb.Root, "nested.git")
	workClone := filepath.Join(sb.Root, "nested-work")
	gitInit(t, remoteRepo, true)
	gitClone(t, remoteRepo, workClone)

	// Two skills
	for _, name := range []string{"keep-nested", "stale-nested"} {
		os.MkdirAll(filepath.Join(workClone, "skills", name), 0755)
		os.WriteFile(filepath.Join(workClone, "skills", name, "SKILL.md"),
			[]byte("---\nname: "+name+"\n---\n# "+name), 0644)
	}
	gitAddCommit(t, workClone, "add skills")
	gitPush(t, workClone)

	// Install both into a group directory
	for _, name := range []string{"keep-nested", "stale-nested"} {
		r := sb.RunCLI("install", "file://"+remoteRepo+"//skills/"+name, "--into", "mygroup", "--skip-audit")
		r.AssertSuccess(t)
	}

	// Verify nested install: skills should be under mygroup/
	if _, err := os.Stat(filepath.Join(sb.SourcePath, "mygroup", "stale-nested")); err != nil {
		t.Fatalf("nested skill should exist: %v", err)
	}

	// Delete stale-nested from remote, update keep-nested
	os.RemoveAll(filepath.Join(workClone, "skills", "stale-nested"))
	os.WriteFile(filepath.Join(workClone, "skills", "keep-nested", "SKILL.md"),
		[]byte("---\nname: keep-nested\n---\n# keep-nested v2"), 0644)
	gitAddCommit(t, workClone, "remove stale-nested")
	gitPush(t, workClone)

	result := sb.RunCLI("update", "--all", "--prune", "--skip-audit")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "pruned")
	result.AssertAnyOutputContains(t, "stale-nested")

	// stale-nested should be gone
	if _, err := os.Stat(filepath.Join(sb.SourcePath, "mygroup", "stale-nested")); !os.IsNotExist(err) {
		t.Error("nested stale skill should have been pruned")
	}

	// keep-nested should still exist
	if _, err := os.Stat(filepath.Join(sb.SourcePath, "mygroup", "keep-nested")); err != nil {
		t.Error("keep-nested should still exist after prune")
	}

	// Registry should not contain stale-nested
	regPath := filepath.Join(filepath.Dir(sb.ConfigPath), "registry.yaml")
	if _, err := os.Stat(regPath); err == nil {
		regContent := sb.ReadFile(regPath)
		if contains(regContent, "stale-nested") {
			t.Error("registry should not contain pruned nested skill")
		}
		if !contains(regContent, "keep-nested") {
			t.Error("registry should still contain keep-nested")
		}
	}
}

// writeMetaForRepo writes metadata matching a repo-installed skill.
func writeMetaForRepo(t *testing.T, skillDir, repoURL, subdir string) {
	t.Helper()
	meta := map[string]any{
		"source":   repoURL + "//" + subdir,
		"type":     "github",
		"repo_url": repoURL,
		"subdir":   subdir,
		"version":  "abc123",
	}
	data, _ := json.Marshal(meta)
	if err := os.WriteFile(filepath.Join(skillDir, ".skillshare-meta.json"), data, 0644); err != nil {
		t.Fatalf("writeMetaForRepo: %v", err)
	}
}
