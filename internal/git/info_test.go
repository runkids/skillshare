package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initTestRepo creates a temporary git repo with one commit
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %s\n%s", args, err, out)
		}
	}

	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "test")
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test"), 0644)
	run("add", "-A")
	run("commit", "-m", "initial")

	return dir
}

func TestIsRepo(t *testing.T) {
	repo := initTestRepo(t)
	if !IsRepo(repo) {
		t.Error("expected IsRepo to return true for a git repo")
	}

	notRepo := t.TempDir()
	if IsRepo(notRepo) {
		t.Error("expected IsRepo to return false for a non-repo dir")
	}
}

func TestHasRemote(t *testing.T) {
	repo := initTestRepo(t)
	if HasRemote(repo) {
		t.Error("expected HasRemote to return false for repo without remote")
	}

	// Add a remote
	cmd := exec.Command("git", "remote", "add", "origin", "https://example.com/repo.git")
	cmd.Dir = repo
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	if !HasRemote(repo) {
		t.Error("expected HasRemote to return true after adding remote")
	}
}

func TestGetCurrentBranch(t *testing.T) {
	repo := initTestRepo(t)
	branch, err := GetCurrentBranch(repo)
	if err != nil {
		t.Fatal(err)
	}
	// Default branch could be main or master depending on git config
	if branch != "main" && branch != "master" {
		t.Errorf("unexpected branch name: %s", branch)
	}
}

func TestStageAndCommit(t *testing.T) {
	repo := initTestRepo(t)

	// Create a new file
	os.WriteFile(filepath.Join(repo, "new.txt"), []byte("hello"), 0644)

	// Stage all
	if err := StageAll(repo); err != nil {
		t.Fatalf("StageAll failed: %v", err)
	}

	// Commit
	if err := Commit(repo, "add new file"); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Should be clean now
	dirty, err := IsDirty(repo)
	if err != nil {
		t.Fatal(err)
	}
	if dirty {
		t.Error("expected repo to be clean after commit")
	}
}

func TestGetStatus(t *testing.T) {
	repo := initTestRepo(t)

	// Clean repo
	status, err := GetStatus(repo)
	if err != nil {
		t.Fatal(err)
	}
	if status != "" {
		t.Errorf("expected empty status for clean repo, got: %q", status)
	}

	// Create untracked file
	os.WriteFile(filepath.Join(repo, "untracked.txt"), []byte("x"), 0644)
	status, err = GetStatus(repo)
	if err != nil {
		t.Fatal(err)
	}
	if status == "" {
		t.Error("expected non-empty status after adding untracked file")
	}
}

func TestIsDirtyAndGetDirtyFiles(t *testing.T) {
	repo := initTestRepo(t)

	dirty, err := IsDirty(repo)
	if err != nil {
		t.Fatal(err)
	}
	if dirty {
		t.Error("expected clean repo")
	}

	// Modify a file
	os.WriteFile(filepath.Join(repo, "README.md"), []byte("# modified"), 0644)

	dirty, err = IsDirty(repo)
	if err != nil {
		t.Fatal(err)
	}
	if !dirty {
		t.Error("expected dirty repo")
	}

	files, err := GetDirtyFiles(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Error("expected at least one dirty file")
	}
}

func addRemote(t *testing.T, repoPath, remoteURL string) {
	t.Helper()
	cmd := exec.Command("git", "remote", "add", "origin", remoteURL)
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to add remote: %v (%s)", err, strings.TrimSpace(string(out)))
	}
}

func TestAuthEnvForRepo_UsesGitHubToken(t *testing.T) {
	repo := initTestRepo(t)
	addRemote(t, repo, "https://github.com/org/private-repo.git")
	t.Setenv("GITHUB_TOKEN", "ghp_test_token_123")

	env := AuthEnvForRepo(repo)
	if len(env) != 3 {
		t.Fatalf("expected auth env with 3 entries, got %d: %v", len(env), env)
	}
	if !strings.Contains(env[1], "url.https://x-access-token:ghp_test_token_123@github.com/.insteadOf") {
		t.Fatalf("unexpected auth key env: %q", env[1])
	}
	if !strings.Contains(env[2], "https://github.com/") {
		t.Fatalf("unexpected auth value env: %q", env[2])
	}
}

func TestAuthEnvForRepo_UsesGitLabToken(t *testing.T) {
	repo := initTestRepo(t)
	addRemote(t, repo, "https://gitlab.com/group/private-repo.git")
	t.Setenv("GITLAB_TOKEN", "glpat_test_token_123")

	env := AuthEnvForRepo(repo)
	if len(env) != 3 {
		t.Fatalf("expected auth env with 3 entries, got %d: %v", len(env), env)
	}
	if !strings.Contains(env[1], "url.https://oauth2:glpat_test_token_123@gitlab.com/.insteadOf") {
		t.Fatalf("unexpected auth key env: %q", env[1])
	}
	if !strings.Contains(env[2], "https://gitlab.com/") {
		t.Fatalf("unexpected auth value env: %q", env[2])
	}
}

func TestAuthEnvForRepo_NoTokenReturnsNil(t *testing.T) {
	repo := initTestRepo(t)
	addRemote(t, repo, "https://github.com/org/private-repo.git")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("SKILLSHARE_GIT_TOKEN", "")

	env := AuthEnvForRepo(repo)
	if env != nil {
		t.Fatalf("expected nil auth env without tokens, got: %v", env)
	}
}

func TestAuthEnvForRepo_SSHRemote_ReturnsNil(t *testing.T) {
	repo := initTestRepo(t)
	addRemote(t, repo, "git@github.com:org/private-repo.git")
	t.Setenv("GITHUB_TOKEN", "ghp_test_token_123")
	t.Setenv("SKILLSHARE_GIT_TOKEN", "generic-token")

	env := AuthEnvForRepo(repo)
	if env != nil {
		t.Fatalf("expected nil auth env for SSH remote, got: %v", env)
	}
}

func TestGetRemoteDefaultBranch_UsesOriginHEAD(t *testing.T) {
	remote := createBareRemoteWithBranch(t, "trunk", map[string]string{
		"README.md": "# trunk\n",
	})
	repo := cloneRepo(t, remote)

	branch, err := GetRemoteDefaultBranch(repo)
	if err != nil {
		t.Fatalf("GetRemoteDefaultBranch failed: %v", err)
	}
	if branch != "trunk" {
		t.Fatalf("expected trunk, got %q", branch)
	}
}

func TestGetRemoteDefaultBranch_FallbackMainMaster(t *testing.T) {
	for _, wantBranch := range []string{"main", "master"} {
		t.Run(wantBranch, func(t *testing.T) {
			remote := createBareRemoteWithBranch(t, wantBranch, map[string]string{
				"README.md": "# " + wantBranch + "\n",
			})
			repo := cloneRepo(t, remote)
			runGit(t, repo, "update-ref", "-d", "refs/remotes/origin/HEAD")

			branch, err := GetRemoteDefaultBranch(repo)
			if err != nil {
				t.Fatalf("GetRemoteDefaultBranch failed: %v", err)
			}
			if branch != wantBranch {
				t.Fatalf("expected %s, got %q", wantBranch, branch)
			}
		})
	}
}

func TestGetRemoteDefaultBranch_FallbackFirstRemoteBranch(t *testing.T) {
	remote := createBareRemoteWithBranch(t, "release", map[string]string{
		"README.md": "# release\n",
	})
	repo := cloneRepo(t, remote)
	runGit(t, repo, "update-ref", "-d", "refs/remotes/origin/HEAD")

	branch, err := GetRemoteDefaultBranch(repo)
	if err != nil {
		t.Fatalf("GetRemoteDefaultBranch failed: %v", err)
	}
	if branch != "release" {
		t.Fatalf("expected release, got %q", branch)
	}
}

func TestHasRemoteSkillDirs(t *testing.T) {
	remote := createBareRemoteWithBranch(t, "main", map[string]string{
		"README.md":            "# docs\n",
		"skill-one/SKILL.md":   "# one\n",
		"skill-two/README.txt": "x\n",
	})
	repo := cloneRepo(t, remote)

	hasSkills, err := HasRemoteSkillDirs(repo, "main")
	if err != nil {
		t.Fatalf("HasRemoteSkillDirs failed: %v", err)
	}
	if !hasSkills {
		t.Fatal("expected remote to have skill directories")
	}
}

func TestHasLocalSkillDirs(t *testing.T) {
	repo := initTestRepo(t)

	hasSkills, err := HasLocalSkillDirs(repo)
	if err != nil {
		t.Fatalf("HasLocalSkillDirs failed: %v", err)
	}
	if hasSkills {
		t.Fatal("expected no local skill directories in fresh repo")
	}

	if err := os.MkdirAll(filepath.Join(repo, "my-skill"), 0o755); err != nil {
		t.Fatalf("failed to create local skill dir: %v", err)
	}
	hasSkills, err = HasLocalSkillDirs(repo)
	if err != nil {
		t.Fatalf("HasLocalSkillDirs failed: %v", err)
	}
	if !hasSkills {
		t.Fatal("expected local skill directory to be detected")
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v (%s)", args, err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out))
}

func createBareRemoteWithBranch(t *testing.T, branch string, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	runGit(t, "", "init", "--bare", remote)

	seed := filepath.Join(root, "seed")
	runGit(t, "", "clone", remote, seed)
	runGit(t, seed, "config", "user.email", "test@test.com")
	runGit(t, seed, "config", "user.name", "test")
	for rel, content := range files {
		path := filepath.Join(seed, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("failed to create dir for %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", rel, err)
		}
	}
	runGit(t, seed, "add", "-A")
	runGit(t, seed, "commit", "-m", "seed "+branch)
	runGit(t, seed, "push", "origin", "HEAD:"+branch)
	runGit(t, remote, "symbolic-ref", "HEAD", "refs/heads/"+branch)
	return remote
}

func cloneRepo(t *testing.T, remote string) string {
	t.Helper()

	repo := filepath.Join(t.TempDir(), "repo")
	runGit(t, "", "clone", remote, repo)
	return repo
}
