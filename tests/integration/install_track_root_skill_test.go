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
