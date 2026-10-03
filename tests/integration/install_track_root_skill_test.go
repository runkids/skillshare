//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/testutil"
)

// setupBareRepoWithRootSkill creates a bare git repo whose SKILL.md sits at
// the repository root (non-spec layout). Returns the file:// URL for cloning.
//
// Mirrors real-world repos like op7418/guizang-ppt-skill that put SKILL.md
// alongside README/LICENSE at the repo root instead of in a subdirectory.
func setupBareRepoWithRootSkill(t *testing.T, sb *testutil.Sandbox, name string) string {
	t.Helper()

	remoteDir := filepath.Join(sb.Root, name+"-remote.git")
	run(t, "", "git", "init", "--bare", "--initial-branch=main", remoteDir)

	workDir := filepath.Join(sb.Root, name+"-work")
	run(t, sb.Root, "git", "clone", remoteDir, workDir)

	os.WriteFile(filepath.Join(workDir, "SKILL.md"),
		[]byte("---\nname: "+name+"\ndescription: root-level skill\n---\n# "+name+"\n"), 0644)
	os.WriteFile(filepath.Join(workDir, "README.md"),
		[]byte("# "+name+"\n"), 0644)
	run(t, workDir, "git", "add", "-A")
	run(t, workDir, "git", "commit", "-m", "initial")
	run(t, workDir, "git", "push", "origin", "HEAD:main")

	return "file://" + remoteDir
}

// setupBareRepoWithNestedAndRootSkill creates a bare git repo with BOTH a
// root-level SKILL.md AND a nested skill under foo/. Used to verify that
// the nested-or-root fallback only kicks in when there are NO nested skills.
func setupBareRepoWithNestedAndRootSkill(t *testing.T, sb *testutil.Sandbox, name string) string {
	t.Helper()

	remoteDir := filepath.Join(sb.Root, name+"-remote.git")
	run(t, "", "git", "init", "--bare", "--initial-branch=main", remoteDir)

	workDir := filepath.Join(sb.Root, name+"-work")
	run(t, sb.Root, "git", "clone", remoteDir, workDir)

	os.WriteFile(filepath.Join(workDir, "SKILL.md"),
		[]byte("---\nname: root-orchestrator\n---\n# root\n"), 0644)
	os.MkdirAll(filepath.Join(workDir, "foo"), 0755)
	os.WriteFile(filepath.Join(workDir, "foo", "SKILL.md"),
		[]byte("---\nname: foo\n---\n# foo\n"), 0644)
	run(t, workDir, "git", "add", "-A")
	run(t, workDir, "git", "commit", "-m", "initial")
	run(t, workDir, "git", "push", "origin", "HEAD:main")

	return "file://" + remoteDir
}

// setupBareRepoEmpty creates a bare git repo with no SKILL.md anywhere.
// Used to verify next-steps messaging when a tracked repo has no skills.
func setupBareRepoEmpty(t *testing.T, sb *testutil.Sandbox, name string) string {
	t.Helper()

	remoteDir := filepath.Join(sb.Root, name+"-remote.git")
	run(t, "", "git", "init", "--bare", "--initial-branch=main", remoteDir)

	workDir := filepath.Join(sb.Root, name+"-work")
	run(t, sb.Root, "git", "clone", remoteDir, workDir)

	os.WriteFile(filepath.Join(workDir, "README.md"), []byte("# "+name+"\n"), 0644)
	run(t, workDir, "git", "add", "-A")
	run(t, workDir, "git", "commit", "-m", "initial")
	run(t, workDir, "git", "push", "origin", "HEAD:main")

	return "file://" + remoteDir
}

// TestInstall_Track_RootSkillMd_WritesMetadata verifies that installing a
// tracked repo whose SKILL.md is at the repo root persists a metadata entry.
// Regression test for issue #163.
func TestInstall_Track_RootSkillMd_WritesMetadata(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	repoURL := setupBareRepoWithRootSkill(t, sb, "root-skill")

	result := sb.RunCLI("install", repoURL, "--track", "--name", "root-tracked")
	result.AssertSuccess(t)

	repoPath := filepath.Join(sb.SourcePath, "_root-tracked")
	if !sb.FileExists(repoPath) {
		t.Fatalf("tracked repo should be cloned to %s", repoPath)
	}

	store, err := install.LoadMetadata(sb.SourcePath)
	if err != nil {
		t.Fatalf("failed to load metadata: %v", err)
	}

	entry := store.Get("_root-tracked")
	if entry == nil {
		t.Fatalf("metadata entry '_root-tracked' missing — got keys: %v", store.List())
	}
	if !entry.Tracked {
		t.Errorf("entry.Tracked = false, want true")
	}
	if entry.Source == "" {
		t.Errorf("entry.Source is empty — expected git remote URL")
	}
	if entry.Branch == "" {
		t.Errorf("entry.Branch is empty — expected current branch (main)")
	}
	if len(entry.FileHashes) == 0 {
		t.Fatalf("entry.FileHashes is empty — expected tracked root skill hashes")
	}
	if _, ok := entry.FileHashes["SKILL.md"]; !ok {
		t.Errorf("entry.FileHashes missing SKILL.md: %#v", entry.FileHashes)
	}

	doctorResult := sb.RunCLI("doctor")
	doctorResult.AssertSuccess(t)
	doctorResult.AssertOutputNotContains(t, "missing file hashes")
}

// TestUpdate_Track_RootSkillMd_BackfillsMissingFileHashes verifies that
// update refreshes metadata for existing tracked root-skill repos installed by
// versions that wrote source/tracked/branch but no file_hashes.
func TestUpdate_Track_RootSkillMd_BackfillsMissingFileHashes(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	repoURL := setupBareRepoWithRootSkill(t, sb, "root-backfill")

	installResult := sb.RunCLI("install", repoURL, "--track", "--name", "backfill-tracked")
	installResult.AssertSuccess(t)

	store, err := install.LoadMetadata(sb.SourcePath)
	if err != nil {
		t.Fatalf("failed to load metadata: %v", err)
	}
	entry := store.Get("_backfill-tracked")
	if entry == nil {
		t.Fatal("metadata entry '_backfill-tracked' missing")
	}
	entry.FileHashes = nil
	if err := store.Save(sb.SourcePath); err != nil {
		t.Fatalf("failed to save metadata without hashes: %v", err)
	}

	updateResult := sb.RunCLI("update", "_backfill-tracked")
	updateResult.AssertSuccess(t)

	store, err = install.LoadMetadata(sb.SourcePath)
	if err != nil {
		t.Fatalf("failed to reload metadata: %v", err)
	}
	entry = store.Get("_backfill-tracked")
	if entry == nil {
		t.Fatal("metadata entry '_backfill-tracked' missing after update")
	}
	if len(entry.FileHashes) == 0 {
		t.Fatalf("entry.FileHashes is empty after update")
	}
	if _, ok := entry.FileHashes["SKILL.md"]; !ok {
		t.Errorf("entry.FileHashes missing SKILL.md after update: %#v", entry.FileHashes)
	}
}

// TestInstall_Track_RootSkillMd_ShowsInStatus verifies that a tracked repo
// with only a root SKILL.md still appears in `status --json` under
// tracked_repos. Regression test for issue #163.
func TestInstall_Track_RootSkillMd_ShowsInStatus(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	repoURL := setupBareRepoWithRootSkill(t, sb, "root-status")

	installResult := sb.RunCLI("install", repoURL, "--track", "--name", "status-tracked")
	installResult.AssertSuccess(t)

	statusResult := sb.RunCLI("status", "--json")
	statusResult.AssertSuccess(t)

	var output struct {
		TrackedRepos []struct {
			Name       string `json:"name"`
			SkillCount int    `json:"skill_count"`
			Dirty      bool   `json:"dirty"`
		} `json:"tracked_repos"`
	}
	if err := json.Unmarshal([]byte(statusResult.Stdout), &output); err != nil {
		t.Fatalf("failed to parse status --json: %v\nstdout: %s", err, statusResult.Stdout)
	}

	found := false
	for _, r := range output.TrackedRepos {
		if r.Name == "_status-tracked" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("tracked_repos should contain '_status-tracked', got %+v", output.TrackedRepos)
	}
}

// TestInstall_Track_RootSkillMd_ReportsOneSkill verifies that the install
// output correctly reports 1 skill (not 0) for a tracked repo whose only
// SKILL.md is at the repo root. Phase B / AC1.
func TestInstall_Track_RootSkillMd_ReportsOneSkill(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	repoURL := setupBareRepoWithRootSkill(t, sb, "root-count")

	result := sb.RunCLI("install", repoURL, "--track", "--name", "count-tracked")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "1 skill")
	result.AssertOutputNotContains(t, "0 skills")
}

// TestSync_TrackedRootSkillMd_LinksTarget verifies that `skillshare sync`
// distributes a tracked repo's root SKILL.md as a flat-name symlink under
// the target's skills directory. Phase B / AC2.
func TestSync_TrackedRootSkillMd_LinksTarget(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	claudePath := sb.CreateTarget("claude")
	sb.WriteConfig("source: " + sb.SourcePath + `
targets:
  claude:
    path: ` + claudePath + `
    mode: merge
`)

	repoURL := setupBareRepoWithRootSkill(t, sb, "root-sync")

	installResult := sb.RunCLI("install", repoURL, "--track", "--name", "sync-tracked")
	installResult.AssertSuccess(t)

	syncResult := sb.RunCLI("sync")
	syncResult.AssertSuccess(t)

	targetLink := filepath.Join(claudePath, "_sync-tracked")
	info, err := os.Lstat(targetLink)
	if err != nil {
		t.Fatalf("expected symlink at %s, got error: %v", targetLink, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected %s to be a symlink, got mode %v", targetLink, info.Mode())
	}

	skillFile := filepath.Join(targetLink, "SKILL.md")
	if _, err := os.Stat(skillFile); err != nil {
		t.Errorf("SKILL.md not readable through target symlink: %v", err)
	}
}

// TestStatus_TrackedRootSkillMd_SkillCountOne verifies that status --json
// reports skill_count: 1 (not 0) for a tracked repo with only a root
// SKILL.md. Phase B / AC3.
func TestStatus_TrackedRootSkillMd_SkillCountOne(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	repoURL := setupBareRepoWithRootSkill(t, sb, "root-count-status")

	installResult := sb.RunCLI("install", repoURL, "--track", "--name", "scnt-tracked")
	installResult.AssertSuccess(t)

	statusResult := sb.RunCLI("status", "--json")
	statusResult.AssertSuccess(t)

	var output struct {
		TrackedRepos []struct {
			Name       string `json:"name"`
			SkillCount int    `json:"skill_count"`
		} `json:"tracked_repos"`
	}
	if err := json.Unmarshal([]byte(statusResult.Stdout), &output); err != nil {
		t.Fatalf("failed to parse status --json: %v\nstdout: %s", err, statusResult.Stdout)
	}

	for _, r := range output.TrackedRepos {
		if r.Name == "_scnt-tracked" {
			if r.SkillCount != 1 {
				t.Errorf("skill_count = %d for root-only tracked repo, want 1", r.SkillCount)
			}
			return
		}
	}
	t.Errorf("tracked_repos did not contain '_scnt-tracked': %+v", output.TrackedRepos)
}

// TestSync_TrackedNestedWithRootSkill_LinksBoth lock-in test: a tracked repo
// with BOTH root SKILL.md AND nested SKILL.md → BOTH are linked at the target.
// This is the existing sync contract: every SKILL.md inside a tracked repo
// is an independent skill, including the root one. Phase B / AC4.
func TestSync_TrackedNestedWithRootSkill_LinksBoth(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	claudePath := sb.CreateTarget("claude")
	sb.WriteConfig("source: " + sb.SourcePath + `
targets:
  claude:
    path: ` + claudePath + `
    mode: merge
`)

	repoURL := setupBareRepoWithNestedAndRootSkill(t, sb, "mixed")

	installResult := sb.RunCLI("install", repoURL, "--track", "--name", "mixed-tracked")
	installResult.AssertSuccess(t)

	syncResult := sb.RunCLI("sync")
	syncResult.AssertSuccess(t)

	nestedLink := filepath.Join(claudePath, "_mixed-tracked__foo")
	if _, err := os.Lstat(nestedLink); err != nil {
		t.Errorf("expected nested skill symlink at %s, got: %v", nestedLink, err)
	}

	rootLink := filepath.Join(claudePath, "_mixed-tracked")
	if _, err := os.Lstat(rootLink); err != nil {
		t.Errorf("expected root-skill symlink at %s, got: %v", rootLink, err)
	}
}

// TestInstall_Track_EmptyRepo_NextSteps verifies that installing a tracked
// repo with no SKILL.md anywhere does not suggest running sync. Phase B / AC5.
func TestInstall_Track_EmptyRepo_NextSteps(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	repoURL := setupBareRepoEmpty(t, sb, "empty-tracked")

	result := sb.RunCLI("install", repoURL, "--track", "--name", "et-tracked")
	result.AssertSuccess(t)

	result.AssertOutputNotContains(t, "Run 'skillshare sync'")
}
