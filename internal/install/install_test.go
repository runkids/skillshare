package install

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverSkills_RootOnly(t *testing.T) {
	// Setup: repo with SKILL.md at root only
	repoPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoPath, "SKILL.md"), []byte("---\nname: test\n---\n# Test"), 0644); err != nil {
		t.Fatal(err)
	}

	skills := discoverSkills(repoPath, true)
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Path != "." {
		t.Errorf("Path = %q, want %q", skills[0].Path, ".")
	}
}

func TestDiscoverSkills_RootOnly_ExcludeRoot(t *testing.T) {
	// Setup: repo with SKILL.md at root only, includeRoot=false
	repoPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoPath, "SKILL.md"), []byte("---\nname: test\n---\n# Test"), 0644); err != nil {
		t.Fatal(err)
	}

	skills := discoverSkills(repoPath, false)
	if len(skills) != 0 {
		t.Fatalf("expected 0 skills with includeRoot=false, got %d", len(skills))
	}
}

func TestDiscoverSkills_RootAndChildren(t *testing.T) {
	// Setup: repo with SKILL.md at root AND child directories
	repoPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoPath, "SKILL.md"), []byte("---\nname: root\n---\n# Root"), 0644); err != nil {
		t.Fatal(err)
	}

	childDir := filepath.Join(repoPath, "child-skill")
	if err := os.MkdirAll(childDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(childDir, "SKILL.md"), []byte("---\nname: child\n---\n# Child"), 0644); err != nil {
		t.Fatal(err)
	}

	skills := discoverSkills(repoPath, true)
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}

	// Verify we have both root and child
	var hasRoot, hasChild bool
	for _, s := range skills {
		if s.Path == "." {
			hasRoot = true
		}
		if s.Path == "child-skill" && s.Name == "child-skill" {
			hasChild = true
		}
	}
	if !hasRoot {
		t.Error("missing root skill (Path='.')")
	}
	if !hasChild {
		t.Error("missing child skill (Path='child-skill')")
	}
}

func TestDiscoverLocal_ExplicitSkillTarget_ReturnsOnlyRootSkill(t *testing.T) {
	repoPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoPath, "SKILL.md"), []byte("---\nname: root\n---\n# Root"), 0644); err != nil {
		t.Fatal(err)
	}

	childDir := filepath.Join(repoPath, "skills", "child-skill")
	if err := os.MkdirAll(childDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(childDir, "SKILL.md"), []byte("---\nname: child\n---\n# Child"), 0644); err != nil {
		t.Fatal(err)
	}

	agentsDir := filepath.Join(repoPath, "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "helper.md"), []byte("# helper"), 0644); err != nil {
		t.Fatal(err)
	}

	discovery, err := DiscoverLocal(&Source{
		Type:          SourceTypeLocalPath,
		Raw:           repoPath + "/SKILL.md",
		Path:          repoPath,
		Name:          "root",
		ExplicitSkill: true,
	})
	if err != nil {
		t.Fatalf("DiscoverLocal() error = %v", err)
	}

	if len(discovery.Skills) != 1 {
		t.Fatalf("expected explicit skill target to return 1 skill, got %d: %+v", len(discovery.Skills), discovery.Skills)
	}
	if discovery.Skills[0].Path != "." {
		t.Fatalf("expected explicit skill target to keep root skill, got path %q", discovery.Skills[0].Path)
	}
	if len(discovery.Agents) != 0 {
		t.Fatalf("expected explicit skill target to ignore agents, got %d", len(discovery.Agents))
	}
}

// Regression: UI discover endpoint defers CleanupDiscovery; for local sources
// RepoPath is the user's directory and must not be removed.
func TestCleanupDiscovery_PreservesLocalSource(t *testing.T) {
	repoPath := t.TempDir()
	skillFile := filepath.Join(repoPath, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("---\nname: keep-me\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}

	discovery, err := DiscoverLocal(&Source{
		Type: SourceTypeLocalPath,
		Raw:  repoPath,
		Path: repoPath,
		Name: filepath.Base(repoPath),
	})
	if err != nil {
		t.Fatalf("DiscoverLocal: %v", err)
	}
	if discovery.RepoPath != repoPath {
		t.Fatalf("RepoPath = %q, want %q", discovery.RepoPath, repoPath)
	}

	CleanupDiscovery(discovery)

	if _, err := os.Stat(skillFile); err != nil {
		t.Fatalf("local source was removed after CleanupDiscovery: %v", err)
	}
}

func TestCleanupDiscovery_SkipsWhenSourceNil(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "keep.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	CleanupDiscovery(&DiscoveryResult{RepoPath: dir, Source: nil})
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("directory was removed when Source was nil: %v", err)
	}
}

func TestDiscoverSkills_ChildrenOnly(t *testing.T) {
	// Setup: orchestrator repo with no root SKILL.md, only children
	repoPath := t.TempDir()

	for _, name := range []string{"skill-a", "skill-b"} {
		dir := filepath.Join(repoPath, name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\n---\n# "+name), 0644); err != nil {
			t.Fatal(err)
		}
	}

	skills := discoverSkills(repoPath, true)
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}
	for _, s := range skills {
		if s.Path == "." {
			t.Error("should not have root skill when no root SKILL.md exists")
		}
	}
}

func TestDiscoverSkills_SkipsGitDir(t *testing.T) {
	repoPath := t.TempDir()

	// Create .git directory with SKILL.md (should be skipped)
	gitDir := filepath.Join(repoPath, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "SKILL.md"), []byte("---\nname: git\n---"), 0644); err != nil {
		t.Fatal(err)
	}

	skills := discoverSkills(repoPath, true)
	if len(skills) != 0 {
		t.Errorf("expected 0 skills (.git skipped), got %d", len(skills))
	}
}

func TestDiscoverSkills_SkipsTargetDotDirs(t *testing.T) {
	// Simulate target dotdirs being set (as main.go does at startup)
	origDirs := TargetDotDirs
	TargetDotDirs = map[string]bool{".claude": true, ".cursor": true, ".skillshare": true}
	defer func() { TargetDotDirs = origDirs }()

	repoPath := t.TempDir()

	// Create source skills (should be found)
	for _, name := range []string{"adapt", "polish"} {
		dir := filepath.Join(repoPath, "source", "skills", name)
		os.MkdirAll(dir, 0755)
		os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\n---"), 0644)
	}

	// Create target directory copies (should be skipped)
	for _, name := range []string{"adapt", "polish"} {
		dir := filepath.Join(repoPath, ".claude", "skills", name)
		os.MkdirAll(dir, 0755)
		os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\n---"), 0644)
	}

	// Create .skillshare dir (should also be skipped)
	os.MkdirAll(filepath.Join(repoPath, ".skillshare"), 0755)
	os.WriteFile(filepath.Join(repoPath, ".skillshare", "config.yaml"), []byte(""), 0644)

	skills := discoverSkills(repoPath, false)
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills (source only), got %d: %v", len(skills), skills)
	}
	for _, s := range skills {
		if strings.HasPrefix(s.Path, ".claude") {
			t.Errorf("found target dir skill that should have been skipped: %s", s.Path)
		}
	}
}

func TestDiscoverSkills_FindsNonTargetHiddenDirs(t *testing.T) {
	// Simulate target dotdirs being set
	origDirs := TargetDotDirs
	TargetDotDirs = map[string]bool{".claude": true, ".cursor": true}
	defer func() { TargetDotDirs = origDirs }()

	repoPath := t.TempDir()

	// Create hidden dirs with skills (like openai/skills .curated/, .system/)
	// These are NOT target dirs and should be discovered.
	for _, name := range []string{".curated", ".system"} {
		dir := filepath.Join(repoPath, name, "skill-a")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: skill-a\n---"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Also create a .git dir (should still be skipped)
	gitDir := filepath.Join(repoPath, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "SKILL.md"), []byte("---\nname: git\n---"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create .claude target dir (should be skipped)
	dir := filepath.Join(repoPath, ".claude", "skills", "my-skill")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: my-skill\n---"), 0644)

	skills := discoverSkills(repoPath, false)
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills from non-target hidden dirs, got %d: %v", len(skills), skills)
	}
}

func TestResolveSubdir(t *testing.T) {
	t.Run("exact match", func(t *testing.T) {
		repoPath := t.TempDir()
		// Create exact subdir with SKILL.md
		os.MkdirAll(filepath.Join(repoPath, "vue"), 0755)
		os.WriteFile(filepath.Join(repoPath, "vue", "SKILL.md"), []byte("# Vue"), 0644)

		resolved, err := resolveSubdir(repoPath, "vue")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resolved != "vue" {
			t.Errorf("resolved = %q, want %q", resolved, "vue")
		}
	})

	t.Run("fuzzy match via nested skill", func(t *testing.T) {
		repoPath := t.TempDir()
		// Skill lives under skills/ prefix, not at root
		os.MkdirAll(filepath.Join(repoPath, "skills", "vue"), 0755)
		os.WriteFile(filepath.Join(repoPath, "skills", "vue", "SKILL.md"), []byte("# Vue"), 0644)

		resolved, err := resolveSubdir(repoPath, "vue")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resolved != "skills/vue" {
			t.Errorf("resolved = %q, want %q", resolved, "skills/vue")
		}
	})

	t.Run("no match", func(t *testing.T) {
		repoPath := t.TempDir()
		// Empty repo — no skills at all
		_, err := resolveSubdir(repoPath, "nonexistent")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "does not exist") {
			t.Errorf("error = %q, want substring %q", err.Error(), "does not exist")
		}
	})

	t.Run("ambiguous match", func(t *testing.T) {
		repoPath := t.TempDir()
		// Two different paths with same skill name
		os.MkdirAll(filepath.Join(repoPath, "frontend", "pdf"), 0755)
		os.WriteFile(filepath.Join(repoPath, "frontend", "pdf", "SKILL.md"), []byte("# PDF FE"), 0644)
		os.MkdirAll(filepath.Join(repoPath, "backend", "pdf"), 0755)
		os.WriteFile(filepath.Join(repoPath, "backend", "pdf", "SKILL.md"), []byte("# PDF BE"), 0644)

		_, err := resolveSubdir(repoPath, "pdf")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "ambiguous") {
			t.Errorf("error = %q, want substring %q", err.Error(), "ambiguous")
		}
		if !strings.Contains(err.Error(), "frontend/pdf") {
			t.Errorf("error should list candidate 'frontend/pdf': %q", err.Error())
		}
		if !strings.Contains(err.Error(), "backend/pdf") {
			t.Errorf("error should list candidate 'backend/pdf': %q", err.Error())
		}
	})

	t.Run("not a directory", func(t *testing.T) {
		repoPath := t.TempDir()
		// Create a file (not dir) at the subdir path
		os.WriteFile(filepath.Join(repoPath, "vue"), []byte("not a dir"), 0644)

		_, err := resolveSubdir(repoPath, "vue")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "not a directory") {
			t.Errorf("error = %q, want substring %q", err.Error(), "not a directory")
		}
	})
}

func TestWrapGitError(t *testing.T) {
	tests := []struct {
		name       string
		stderr     string
		err        error
		envVars    map[string]string
		tokenAuth  bool
		wantSubstr string
	}{
		{
			name:       "auth failed no token — shows options",
			stderr:     "fatal: Authentication failed for 'https://bitbucket.org/team/repo.git/'",
			err:        errors.New("exit status 128"),
			wantSubstr: "GITHUB_TOKEN",
		},
		{
			name:       "could not read Username no token — shows options",
			stderr:     "fatal: could not read Username for 'https://bitbucket.org': terminal prompts disabled",
			err:        errors.New("exit status 128"),
			wantSubstr: "SSH URL",
		},
		{
			name:       "terminal prompts disabled no token",
			stderr:     "fatal: terminal prompts disabled",
			err:        errors.New("exit status 128"),
			wantSubstr: "authentication required",
		},
		{
			name:       "auth failed with token — token rejected",
			stderr:     "fatal: Authentication failed for 'https://github.com/org/repo.git/'",
			err:        errors.New("exit status 128"),
			envVars:    map[string]string{"GITHUB_TOKEN": "ghp_expired"},
			tokenAuth:  true,
			wantSubstr: "token rejected",
		},
		{
			name:       "auth failed with generic token — token rejected",
			stderr:     "fatal: terminal prompts disabled",
			err:        errors.New("exit status 128"),
			envVars:    map[string]string{"SKILLSHARE_GIT_TOKEN": "custom_tok"},
			tokenAuth:  true,
			wantSubstr: "token rejected",
		},
		{
			name:       "auth failed with unrelated token in env — still auth required",
			stderr:     "fatal: Authentication failed for 'https://bitbucket.org/team/repo.git/'",
			err:        errors.New("exit status 128"),
			envVars:    map[string]string{"GITHUB_TOKEN": "ghp_present_but_not_used"},
			wantSubstr: "authentication required",
		},
		{
			name:       "stderr with token value — sanitized",
			stderr:     "fatal: auth failed for https://x-access-token:ghp_leaked@github.com/",
			err:        errors.New("exit status 128"),
			envVars:    map[string]string{"GITHUB_TOKEN": "ghp_leaked"},
			tokenAuth:  true,
			wantSubstr: "[REDACTED]",
		},
		{
			name:       "ssl self-signed cert — shows SSL options",
			stderr:     "fatal: unable to access 'https://gitlab.internal.com/': SSL certificate problem: self signed certificate",
			err:        errors.New("exit status 128"),
			wantSubstr: "GIT_SSL_CAINFO",
		},
		{
			name:       "ssl unable to get issuer — shows SSL options",
			stderr:     "fatal: unable to access 'https://gitlab.internal.com/': SSL certificate problem: unable to get local issuer certificate",
			err:        errors.New("exit status 128"),
			wantSubstr: "SSL certificate verification failed",
		},
		{
			name:       "ssl certificate verify failed — shows SSL options",
			stderr:     "fatal: unable to access 'https://git.company.com/': server certificate verification failed. CAfile: /etc/ssl/certs/ca-certificates.crt CRLfile: none",
			err:        errors.New("exit status 128"),
			wantSubstr: "GIT_SSL_NO_VERIFY",
		},
		{
			name:       "other stderr",
			stderr:     "fatal: repository not found",
			err:        errors.New("exit status 128"),
			wantSubstr: "repository not found",
		},
		{
			name:       "empty stderr falls back to err",
			stderr:     "",
			err:        errors.New("exit status 1"),
			wantSubstr: "exit status 1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}
			got := WrapGitError(tt.stderr, tt.err, tt.tokenAuth)
			if !strings.Contains(got.Error(), tt.wantSubstr) {
				t.Errorf("WrapGitError() = %q, want substring %q", got.Error(), tt.wantSubstr)
			}
		})
	}
}

func TestExtractGitFatal(t *testing.T) {
	tests := []struct {
		name   string
		stderr string
		want   string
	}{
		{
			name:   "fatal line only",
			stderr: "fatal: repository not found",
			want:   "repository not found",
		},
		{
			name: "divergent branches with hints",
			stderr: "hint: You have divergent branches and need to specify how to reconcile them.\n" +
				"hint: You can do so by running one of the following commands:\n" +
				"hint:\n" +
				"hint:   git config pull.rebase false  # merge\n" +
				"hint:   git config pull.rebase true   # rebase\n" +
				"fatal: Need to specify how to reconcile divergent branches.",
			want: "Need to specify how to reconcile divergent branches.",
		},
		{
			name:   "error prefix",
			stderr: "error: cannot pull with rebase",
			want:   "cannot pull with rebase",
		},
		{
			name:   "no fatal or hint prefix",
			stderr: "some other git output",
			want:   "some other git output",
		},
		{
			name:   "empty string",
			stderr: "",
			want:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractGitFatal(tt.stderr)
			if got != tt.want {
				t.Errorf("extractGitFatal() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGitCommand_SetsEnv(t *testing.T) {
	ctx := context.Background()
	cmd := gitCommand(ctx, "version")

	want := map[string]bool{
		"GIT_TERMINAL_PROMPT=0": false,
		"GIT_ASKPASS=":          false,
		"SSH_ASKPASS=":          false,
	}
	for _, env := range cmd.Env {
		if _, ok := want[env]; ok {
			want[env] = true
		}
	}
	for k, found := range want {
		if !found {
			t.Errorf("gitCommand() missing env %q", k)
		}
	}
}

func TestUpdateTrackedRepo_BeforeHashFailure_FailsClosed(t *testing.T) {
	repoPath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoPath, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	result := &TrackedRepoResult{
		RepoName: "_broken-repo",
		RepoPath: repoPath,
	}

	_, err := updateTrackedRepo(repoPath, result, InstallOptions{})
	if err == nil {
		t.Fatal("expected error when rollback commit cannot be determined")
	}
	if !strings.Contains(err.Error(), "failed to determine rollback commit before update") {
		t.Fatalf("expected rollback commit error, got: %v", err)
	}
}

func TestGetRemoteURL(t *testing.T) {
	dir := t.TempDir()
	// Init a bare git repo and set a remote URL.
	for _, args := range [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "remote", "add", "origin", "https://github.com/org/repo.git"},
	} {
		if out, err := runCmd(args...); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	got := getRemoteURL(dir)
	if got != "https://github.com/org/repo.git" {
		t.Errorf("getRemoteURL() = %q, want %q", got, "https://github.com/org/repo.git")
	}

	// Non-git directory returns empty.
	if got := getRemoteURL(t.TempDir()); got != "" {
		t.Errorf("getRemoteURL(non-git) = %q, want empty", got)
	}
}

func runCmd(args ...string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
