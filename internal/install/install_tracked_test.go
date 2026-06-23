package install

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// makeRemote creates a bare git remote with a SKILL.md on the default branch,
// plus an optional extra branch if extraBranch != "".
func makeRemote(t *testing.T, extraBranch string) (remoteURL string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmp := t.TempDir()
	work := filepath.Join(tmp, "work")
	remote := filepath.Join(tmp, "remote.git")

	mustRunGit(t, "", "init", "-b", "main", work)
	mustRunGit(t, work, "config", "user.email", "test@test.com")
	mustRunGit(t, work, "config", "user.name", "Test")

	skillFile := filepath.Join(work, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("# test skill"), 0644); err != nil {
		t.Fatal(err)
	}
	mustRunGit(t, work, "add", ".")
	mustRunGit(t, work, "commit", "-m", "init")

	if extraBranch != "" {
		mustRunGit(t, work, "checkout", "-b", extraBranch)
		if err := os.WriteFile(skillFile, []byte("# test skill on "+extraBranch), 0644); err != nil {
			t.Fatal(err)
		}
		mustRunGit(t, work, "add", ".")
		mustRunGit(t, work, "commit", "-m", "branch commit")
		mustRunGit(t, work, "checkout", "main")
	}

	mustRunGit(t, "", "clone", "--bare", work, remote)
	return "file://" + remote
}

// TestRehydrateMissingTrackedRepos_ReclonesAbsent verifies that a tracked repo
// declared in metadata but absent on disk is re-cloned (issue #212).
func TestRehydrateMissingTrackedRepos_ReclonesAbsent(t *testing.T) {
	remoteURL := makeRemote(t, "")
	sourceDir := t.TempDir()

	// Declare the tracked repo in metadata WITHOUT cloning it (fresh machine).
	store := LoadMetadataOrNew(sourceDir)
	store.Set("_team-skills", &MetadataEntry{Source: remoteURL, Tracked: true})
	if err := store.Save(sourceDir); err != nil {
		t.Fatalf("save metadata: %v", err)
	}

	results, err := RehydrateMissingTrackedRepos(sourceDir, ParseOptions{}, InstallOptions{SkipAudit: true})
	if err != nil {
		t.Fatalf("RehydrateMissingTrackedRepos() error = %v", err)
	}
	if len(results) != 1 || results[0].Action != "rehydrated" {
		t.Fatalf("unexpected results: %+v", results)
	}
	if _, err := os.Stat(filepath.Join(sourceDir, "_team-skills", ".git")); err != nil {
		t.Fatalf("expected cloned repo at _team-skills: %v", err)
	}
	// After rehydration nothing should remain missing.
	if missing, _ := GetMissingTrackedRepos(sourceDir); len(missing) != 0 {
		t.Fatalf("expected no missing repos after rehydrate, got %+v", missing)
	}
}

// TestInstallTrackedRepo_NameFromSource verifies that when opts.Name is empty,
// the directory name is taken from source.Name rather than being derived from
// the remote URL via TrackName().
func TestInstallTrackedRepo_NameFromSource(t *testing.T) {
	remoteURL := makeRemote(t, "")
	sourceDir := t.TempDir()

	source := &Source{
		Type:     SourceTypeGitHTTPS,
		Raw:      remoteURL,
		CloneURL: remoteURL,
		Name:     "my-custom-name", // explicit name; should win over TrackName()
	}

	result, err := InstallTrackedRepo(source, sourceDir, InstallOptions{})
	if err != nil {
		t.Fatalf("InstallTrackedRepo() error = %v", err)
	}
	if result.Action != "cloned" {
		t.Fatalf("Action = %q, want %q", result.Action, "cloned")
	}

	want := filepath.Join(sourceDir, "_my-custom-name")
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("expected repo at %q but stat failed: %v", want, err)
	}
	// Confirm the URL-derived name was NOT used.
	unwanted := filepath.Join(sourceDir, "_"+source.TrackName())
	if _, err := os.Stat(unwanted); err == nil {
		t.Fatalf("repo should NOT exist at URL-derived path %q", unwanted)
	}
}

// TestInstallTrackedRepo_BranchFromSource verifies that when opts.Branch is
// empty, the repo is cloned onto source.Branch rather than the remote default.
func TestInstallTrackedRepo_BranchFromSource(t *testing.T) {
	const featureBranch = "feature/my-work"
	remoteURL := makeRemote(t, featureBranch)
	sourceDir := t.TempDir()

	source := &Source{
		Type:     SourceTypeGitHTTPS,
		Raw:      remoteURL,
		CloneURL: remoteURL,
		Name:     "mybranch-skill",
		Branch:   featureBranch, // should be used; opts.Branch is empty
	}

	result, err := InstallTrackedRepo(source, sourceDir, InstallOptions{})
	if err != nil {
		t.Fatalf("InstallTrackedRepo() error = %v", err)
	}
	if result.Action != "cloned" {
		t.Fatalf("Action = %q, want %q", result.Action, "cloned")
	}

	repoPath := filepath.Join(sourceDir, "_mybranch-skill")
	if _, err := os.Stat(repoPath); err != nil {
		t.Fatalf("expected repo at %q: %v", repoPath, err)
	}

	out, err2 := exec.Command("git", "-C", repoPath, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err2 != nil {
		t.Fatalf("git rev-parse failed: %v", err2)
	}
	got := strings.TrimSpace(string(out))
	if got != featureBranch {
		t.Errorf("cloned branch = %q, want %q", got, featureBranch)
	}
}
