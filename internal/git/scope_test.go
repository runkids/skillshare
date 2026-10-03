package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func readGitignore(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	return string(b)
}

func TestWriteScopeGitignore_FreshRootExcludesConfig(t *testing.T) {
	dir := t.TempDir()
	if err := WriteScopeGitignore(dir, "root"); err != nil {
		t.Fatal(err)
	}
	got := readGitignore(t, dir)
	if !strings.Contains(got, "config.yaml") {
		t.Errorf("root .gitignore must exclude config.yaml, got:\n%s", got)
	}
}

func TestWriteScopeGitignore_FreshNonRootOmitsConfig(t *testing.T) {
	dir := t.TempDir()
	if err := WriteScopeGitignore(dir, "skills"); err != nil {
		t.Fatal(err)
	}
	got := readGitignore(t, dir)
	if strings.Contains(got, "config.yaml") {
		t.Errorf("non-root .gitignore must not mention config.yaml, got:\n%s", got)
	}
}

func TestWriteScopeGitignore_AppendsToExistingAtRoot(t *testing.T) {
	dir := t.TempDir()
	gi := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(gi, []byte("node_modules/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteScopeGitignore(dir, "root"); err != nil {
		t.Fatal(err)
	}
	got := readGitignore(t, dir)
	if !strings.Contains(got, "config.yaml") {
		t.Errorf("existing root .gitignore must gain config.yaml, got:\n%s", got)
	}
	if !strings.Contains(got, "node_modules/") {
		t.Errorf("existing entries must be preserved, got:\n%s", got)
	}
}

func TestWriteScopeGitignore_IdempotentAtRoot(t *testing.T) {
	dir := t.TempDir()
	if err := WriteScopeGitignore(dir, "root"); err != nil {
		t.Fatal(err)
	}
	if err := WriteScopeGitignore(dir, "root"); err != nil {
		t.Fatal(err)
	}
	got := readGitignore(t, dir)
	if n := strings.Count(got, "config.yaml"); n != 1 {
		t.Errorf("config.yaml must appear exactly once, got %d:\n%s", n, got)
	}
}

func gitExec(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestNestedRepos_DetectsSubdirGitOnly(t *testing.T) {
	dir := t.TempDir()
	mustMkdir := func(p string) {
		if err := os.MkdirAll(filepath.Join(dir, p), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mustMkdir(".git")              // the scope repo's own .git — must be ignored
	mustMkdir("skills/.git")       // a nested repo — must be detected
	mustMkdir("old/.git.disabled") // already disabled — must be ignored
	mustMkdir("plain/references")  // a plain skill dir — must be ignored

	got, err := NestedRepos(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "skills" {
		t.Errorf("want [skills], got %v", got)
	}
}

func TestEnsureConfigUntracked_RemovesTrackedConfig(t *testing.T) {
	dir := t.TempDir()
	gitExec(t, dir, "init")
	gitExec(t, dir, "config", "user.email", "t@t.com")
	gitExec(t, dir, "config", "user.name", "t")
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("k: v\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitExec(t, dir, "add", "config.yaml")
	gitExec(t, dir, "commit", "-m", "leak config")

	removed, err := EnsureConfigUntracked(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !removed {
		t.Error("expected removed=true for a tracked config.yaml")
	}
	if isTracked(dir, "config.yaml") {
		t.Error("config.yaml must no longer be tracked")
	}
	if _, err := os.Stat(cfgPath); err != nil {
		t.Error("config.yaml file must remain on disk after untracking")
	}
	if gi := readGitignore(t, dir); !strings.Contains(gi, "config.yaml") {
		t.Errorf(".gitignore must contain config.yaml, got:\n%s", gi)
	}
	if removed, _ := EnsureConfigUntracked(dir); removed {
		t.Error("second call must be a no-op (removed=false)")
	}
}

func TestEnsureConfigUntracked_OverridesNegatedIgnore(t *testing.T) {
	dir := t.TempDir()
	gitExec(t, dir, "init")
	gitExec(t, dir, "config", "user.email", "t@t.com")
	gitExec(t, dir, "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("config.yaml\n!config.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("k: v\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitExec(t, dir, "add", "-A")
	gitExec(t, dir, "commit", "-m", "leak config")

	if _, err := EnsureConfigUntracked(dir); err != nil {
		t.Fatal(err)
	}
	gitExec(t, dir, "add", "-A")
	if isTracked(dir, "config.yaml") {
		t.Fatalf("git add -A tracked config.yaml again; .gitignore:\n%s", readGitignore(t, dir))
	}
}

func TestDisableNestedRepo_RenamesAndRefusesClobber(t *testing.T) {
	dir := t.TempDir()
	subGit := filepath.Join(dir, "skills", ".git")
	if err := os.MkdirAll(subGit, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := DisableNestedRepo(dir, "skills"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(subGit); !os.IsNotExist(err) {
		t.Error("skills/.git must be renamed away")
	}
	if _, err := os.Stat(filepath.Join(dir, "skills", ".git.disabled")); err != nil {
		t.Error("skills/.git.disabled must exist after disabling")
	}
	if gi := readGitignore(t, dir); !strings.Contains(gi, ".git.disabled") {
		t.Errorf(".gitignore must contain .git.disabled, got:\n%s", gi)
	}

	// A re-created .git plus an existing .git.disabled must not be clobbered.
	if err := os.MkdirAll(subGit, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := DisableNestedRepo(dir, "skills"); err == nil {
		t.Error("expected an error when .git.disabled already exists")
	}
}

// TestDisableNestedRepo_DropsStaleGitlink covers the already-broken repair case:
// a parent repo that already committed the nested directory as a gitlink (the
// real-world symptom). Disabling must drop the stale gitlink so a subsequent
// `git add -A` re-tracks the directory's files as blobs instead of leaving an
// empty submodule.
func TestDisableNestedRepo_DropsStaleGitlink(t *testing.T) {
	dir := t.TempDir()
	gitExec(t, dir, "init")
	gitExec(t, dir, "config", "user.email", "t@t.com")
	gitExec(t, dir, "config", "user.name", "t")

	// A nested repo with a file, then commit it from the parent — this records
	// "skills" as a gitlink (160000) in the parent index.
	sub := filepath.Join(dir, "skills")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	gitExec(t, sub, "init")
	gitExec(t, sub, "config", "user.email", "t@t.com")
	gitExec(t, sub, "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(sub, "SKILL.md"), []byte("# skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitExec(t, sub, "add", "-A")
	gitExec(t, sub, "commit", "-m", "nested")
	gitExec(t, dir, "add", "skills")
	gitExec(t, dir, "commit", "-m", "record gitlink")

	if !isGitlink(dir, "skills") {
		t.Fatal("setup failed: skills should be a gitlink before disabling")
	}

	if err := DisableNestedRepo(dir, "skills"); err != nil {
		t.Fatal(err)
	}
	if isGitlink(dir, "skills") {
		t.Error("stale gitlink must be dropped from the index after disabling")
	}

	// After re-staging, the directory's files are tracked as blobs.
	gitExec(t, dir, "add", "-A")
	out, err := exec.Command("git", "-C", dir, "ls-files", "skills/SKILL.md").Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		t.Errorf("skills/SKILL.md must be tracked as a file after disabling, got %q (err %v)", out, err)
	}
}

func TestRemoteTracksConfig(t *testing.T) {
	dir := t.TempDir()
	gitExec(t, dir, "init")
	gitExec(t, dir, "config", "user.email", "t@t.com")
	gitExec(t, dir, "config", "user.name", "t")

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitExec(t, dir, "add", "README.md")
	gitExec(t, dir, "commit", "-m", "init")

	if RemoteTracksConfig(dir, "HEAD") {
		t.Error("expected RemoteTracksConfig to be false when config.yaml is not tracked")
	}

	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("source: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitExec(t, dir, "add", "config.yaml")
	gitExec(t, dir, "commit", "-m", "add config")

	if !RemoteTracksConfig(dir, "HEAD") {
		t.Error("expected RemoteTracksConfig to be true when config.yaml is tracked")
	}
}

func TestHasLocalRootConfig(t *testing.T) {
	dir := t.TempDir()
	if HasLocalRootConfig(dir) {
		t.Error("empty dir must not report local root config")
	}

	// .gitignore ignoring config.yaml
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("config.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !HasLocalRootConfig(dir) {
		t.Error("dir with config.yaml in .gitignore must report local root config")
	}

	// Remove .gitignore, create config.yaml
	os.Remove(filepath.Join(dir, ".gitignore"))
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("source: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !HasLocalRootConfig(dir) {
		t.Error("dir with config.yaml on disk must report local root config")
	}
}

// rootScopeRepoTrackingRemote makes a root-scope repo (config.yaml ignored,
// a local config.yaml on disk) whose main branch tracks a bare remote.
func rootScopeRepoTrackingRemote(t *testing.T) (repo, remote string) {
	t.Helper()
	remote = filepath.Join(t.TempDir(), "remote.git")
	runGit(t, "", "init", "--bare", "-b", "main", remote)
	repo = t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "test@test.com")
	runGit(t, repo, "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("config.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", ".gitignore")
	runGit(t, repo, "commit", "-m", "scaffold")
	runGit(t, repo, "remote", "add", "origin", remote)
	runGit(t, repo, "push", "-u", "origin", "main")
	if err := os.WriteFile(filepath.Join(repo, "config.yaml"), []byte("LOCAL-config\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo, remote
}

// pushFromOtherClone commits files from a separate clone of remote, force-adding
// them so an ignored config.yaml gets tracked, as another machine might.
func pushFromOtherClone(t *testing.T, remote string, files map[string]string) {
	t.Helper()
	other := filepath.Join(t.TempDir(), "other")
	runGit(t, "", "clone", remote, other)
	runGit(t, other, "config", "user.email", "other@test.com")
	runGit(t, other, "config", "user.name", "other")
	for rel, content := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(other, rel)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(other, rel), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit(t, other, "add", "-f", rel)
	}
	runGit(t, other, "commit", "-m", "other machine")
	runGit(t, other, "push", "origin", "main")
}

func TestKeepLocalConfig_SurvivesPullThatTracksConfig(t *testing.T) {
	repo, remote := rootScopeRepoTrackingRemote(t)
	pushFromOtherClone(t, remote, map[string]string{"config.yaml": "remote-config\n"})
	restore, err := KeepLocalConfig(repo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PullWithEnv(repo, nil); err != nil {
		t.Fatalf("PullWithEnv() error: %v", err)
	}
	tracked, err := restore()
	got, _ := os.ReadFile(filepath.Join(repo, "config.yaml"))
	if err != nil || !tracked || string(got) != "LOCAL-config\n" {
		t.Fatalf("restore() = %v, %v; config.yaml = %q, want the local copy kept and tracking reported", tracked, err, got)
	}
}

func TestKeepLocalConfig_RestoresSymlinkedConfig(t *testing.T) {
	repo, remote := rootScopeRepoTrackingRemote(t)
	pushFromOtherClone(t, remote, map[string]string{"config.yaml": "remote-config\n"})
	real := filepath.Join(t.TempDir(), "dotfiles-config.yaml")
	if err := os.WriteFile(real, []byte("LOCAL-config\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(repo, "config.yaml")
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	restore, err := KeepLocalConfig(repo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PullWithEnv(repo, nil); err != nil {
		t.Fatalf("PullWithEnv() error: %v", err)
	}
	if _, err := restore(); err != nil {
		t.Fatal(err)
	}
	if target, err := os.Readlink(link); err != nil || target != real {
		t.Fatalf("config.yaml symlink = %q, %v; want it pointing at %q", target, err, real)
	}
}

func TestKeepLocalConfig_ReportsNothingWhenRemoteLeavesConfigAlone(t *testing.T) {
	repo, remote := rootScopeRepoTrackingRemote(t)
	pushFromOtherClone(t, remote, map[string]string{"skills/a/SKILL.md": "# a\n"})
	restore, err := KeepLocalConfig(repo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PullWithEnv(repo, nil); err != nil {
		t.Fatalf("PullWithEnv() error: %v", err)
	}
	if tracked, err := restore(); err != nil || tracked {
		t.Fatalf("restore() = %v, %v; want false, nil", tracked, err)
	}
}

func TestKeepLocalConfig_LeavesEditsToUntrackedConfigAlone(t *testing.T) {
	repo, remote := rootScopeRepoTrackingRemote(t)
	pushFromOtherClone(t, remote, map[string]string{"skills/a/SKILL.md": "# a\n"})
	restore, err := KeepLocalConfig(repo)
	if err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(repo, "config.yaml")
	if err := os.WriteFile(cfg, []byte("EDITED-during-pull\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := PullWithEnv(repo, nil); err != nil {
		t.Fatalf("PullWithEnv() error: %v", err)
	}
	if _, err := restore(); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(cfg); string(got) != "EDITED-during-pull\n" {
		t.Fatalf("config.yaml = %q; want the edit made during the pull kept", got)
	}
}

func TestKeepLocalConfig_LeavesEditsToTrackedConfigGitLeftAlone(t *testing.T) {
	repo, remote := rootScopeRepoTrackingRemote(t)
	pushFromOtherClone(t, remote, map[string]string{"config.yaml": "LOCAL-config\n"})
	if _, err := PullWithEnv(repo, nil); err != nil {
		t.Fatalf("PullWithEnv() error: %v", err)
	}
	pushFromOtherClone(t, remote, map[string]string{"skills/a/SKILL.md": "# a\n"})
	restore, err := KeepLocalConfig(repo)
	if err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(repo, "config.yaml")
	if err := os.WriteFile(cfg, []byte("EDITED-during-pull\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := PullWithEnv(repo, nil); err != nil {
		t.Fatalf("PullWithEnv() error: %v", err)
	}
	if _, err := restore(); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(cfg); string(got) != "EDITED-during-pull\n" {
		t.Fatalf("config.yaml = %q; want the edit made during the pull kept", got)
	}
}

func TestKeepLocalConfig_IgnoresConfigTrackedOnlyLocally(t *testing.T) {
	repo, _ := rootScopeRepoTrackingRemote(t)
	runGit(t, repo, "add", "-f", "config.yaml")
	runGit(t, repo, "commit", "-m", "track config locally")
	restore, err := KeepLocalConfig(repo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PullWithEnv(repo, nil); err != nil {
		t.Fatalf("PullWithEnv() error: %v", err)
	}
	if tracked, err := restore(); err != nil || tracked {
		t.Fatalf("restore() = %v, %v; want false, nil when only a local commit tracks config.yaml", tracked, err)
	}
}

func TestKeepLocalConfig_RestoresAfterAbortedPull(t *testing.T) {
	repo, remote := rootScopeRepoTrackingRemote(t)
	pushFromOtherClone(t, remote, map[string]string{"config.yaml": "remote-config\n", "skills/a/SKILL.md": "# remote\n"})
	if err := os.MkdirAll(filepath.Join(repo, "skills", "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "skills", "a", "SKILL.md"), []byte("# local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "skills")
	runGit(t, repo, "commit", "-m", "local edit")
	restore, err := KeepLocalConfig(repo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PullWithEnv(repo, nil); err == nil {
		t.Fatal("PullWithEnv() succeeded; want a conflict that aborts the merge")
	}
	if _, err := restore(); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(repo, "config.yaml")); err != nil || string(got) != "LOCAL-config\n" {
		t.Fatalf("config.yaml = %q, %v; want the local copy restored after the aborted pull", got, err)
	}
}

func TestKeepLocalConfig_RestoresModeOfIdenticalCopy(t *testing.T) {
	repo, remote := rootScopeRepoTrackingRemote(t)
	cfg := filepath.Join(repo, "config.yaml")
	if err := os.Chmod(cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	pushFromOtherClone(t, remote, map[string]string{"config.yaml": "LOCAL-config\n"})
	restore, err := KeepLocalConfig(repo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PullWithEnv(repo, nil); err != nil {
		t.Fatalf("PullWithEnv() error: %v", err)
	}
	if _, err := restore(); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(cfg); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("config.yaml mode = %v, %v; want 0600 kept", info.Mode().Perm(), err)
	}
}

func TestKeepLocalConfig_RestoresOverTrackedDirectory(t *testing.T) {
	repo, remote := rootScopeRepoTrackingRemote(t)
	pushFromOtherClone(t, remote, map[string]string{"config.yaml/file": "remote\n"})
	restore, err := KeepLocalConfig(repo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PullWithEnv(repo, nil); err != nil {
		t.Fatalf("PullWithEnv() error: %v", err)
	}
	if _, err := restore(); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(repo, "config.yaml")); err != nil || string(got) != "LOCAL-config\n" {
		t.Fatalf("config.yaml = %q, %v; want the local copy restored over the tracked directory", got, err)
	}
	if removed, err := EnsureConfigUntracked(repo); err != nil || !removed {
		t.Fatalf("EnsureConfigUntracked() = %v, %v; want the tracked directory untracked", removed, err)
	}
}

func TestKeepLocalConfig_KeepsEditMadeAfterCheckout(t *testing.T) {
	repo, remote := rootScopeRepoTrackingRemote(t)
	pushFromOtherClone(t, remote, map[string]string{"config.yaml": "remote-config\n"})
	restore, err := KeepLocalConfig(repo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PullWithEnv(repo, nil); err != nil {
		t.Fatalf("PullWithEnv() error: %v", err)
	}
	cfg := filepath.Join(repo, "config.yaml")
	if err := os.WriteFile(cfg, []byte("NEWER-local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := restore(); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(cfg); string(got) != "NEWER-local\n" {
		t.Fatalf("config.yaml = %q; want the copy written after checkout kept", got)
	}
}

func TestKeepLocalConfig_KeepsSymlinkMadeAfterCheckout(t *testing.T) {
	repo, remote := rootScopeRepoTrackingRemote(t)
	pushFromOtherClone(t, remote, map[string]string{"config.yaml": "remote-config\n"})
	restore, err := KeepLocalConfig(repo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PullWithEnv(repo, nil); err != nil {
		t.Fatalf("PullWithEnv() error: %v", err)
	}
	newer := filepath.Join(t.TempDir(), "dotfiles-config.yaml")
	if err := os.WriteFile(newer, []byte("NEWER-local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(repo, "config.yaml")
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(newer, link); err != nil {
		t.Fatal(err)
	}
	if _, err := restore(); err != nil {
		t.Fatal(err)
	}
	if target, err := os.Readlink(link); err != nil || target != newer {
		t.Fatalf("config.yaml symlink = %q, %v; want the link made after checkout kept", target, err)
	}
}

func TestConfigGitignoreNeedsRepair_ExplicitEntryUnderBroaderRule(t *testing.T) {
	dir := t.TempDir()
	gitExec(t, dir, "init")
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !ConfigGitignoreNeedsRepair(dir) {
		t.Fatal("ConfigGitignoreNeedsRepair() = false; want true when only *.yaml ignores config.yaml")
	}
	if _, err := EnsureConfigUntracked(dir); err != nil {
		t.Fatal(err)
	}
	if ConfigGitignoreNeedsRepair(dir) {
		t.Fatal("ConfigGitignoreNeedsRepair() = true after EnsureConfigUntracked; want false")
	}
}

func TestKeepLocalConfig_ReportsTrackedCopyWithSameContent(t *testing.T) {
	repo, remote := rootScopeRepoTrackingRemote(t)
	pushFromOtherClone(t, remote, map[string]string{"config.yaml": "LOCAL-config\n"})
	restore, err := KeepLocalConfig(repo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PullWithEnv(repo, nil); err != nil {
		t.Fatalf("PullWithEnv() error: %v", err)
	}
	if tracked, err := restore(); err != nil || !tracked {
		t.Fatalf("restore() = %v, %v; want true, nil for a tracked copy with identical bytes", tracked, err)
	}
}
