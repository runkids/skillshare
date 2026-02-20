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

func TestDiscoverSkills_FindsHiddenDirs(t *testing.T) {
	repoPath := t.TempDir()

	// Create hidden dirs with skills (like openai/skills .curated/, .system/)
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

	skills := discoverSkills(repoPath, false)
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills from hidden dirs, got %d: %v", len(skills), skills)
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
			got := wrapGitError(tt.stderr, tt.err, tt.tokenAuth)
			if !strings.Contains(got.Error(), tt.wantSubstr) {
				t.Errorf("wrapGitError() = %q, want substring %q", got.Error(), tt.wantSubstr)
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
