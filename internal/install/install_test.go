package install

import (
	"context"
	"errors"
	"os"
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

func TestDiscoverSkills_SkipsHiddenDirs(t *testing.T) {
	repoPath := t.TempDir()

	// Create .git directory with SKILL.md (should be skipped)
	gitDir := filepath.Join(repoPath, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "SKILL.md"), []byte("---\nname: git\n---"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create .hidden directory with SKILL.md (should be skipped)
	hiddenDir := filepath.Join(repoPath, ".hidden")
	if err := os.MkdirAll(hiddenDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hiddenDir, "SKILL.md"), []byte("---\nname: hidden\n---"), 0644); err != nil {
		t.Fatal(err)
	}

	skills := discoverSkills(repoPath, true)
	if len(skills) != 0 {
		t.Errorf("expected 0 skills (hidden dirs skipped), got %d", len(skills))
	}
}

func TestWrapGitError(t *testing.T) {
	tests := []struct {
		name       string
		stderr     string
		err        error
		wantSubstr string
	}{
		{
			name:       "authentication failed",
			stderr:     "fatal: Authentication failed for 'https://bitbucket.org/team/repo.git/'",
			err:        errors.New("exit status 128"),
			wantSubstr: "git@<host>:<owner>/<repo>.git",
		},
		{
			name:       "could not read Username",
			stderr:     "fatal: could not read Username for 'https://bitbucket.org': terminal prompts disabled",
			err:        errors.New("exit status 128"),
			wantSubstr: "git@<host>:<owner>/<repo>.git",
		},
		{
			name:       "terminal prompts disabled",
			stderr:     "fatal: terminal prompts disabled",
			err:        errors.New("exit status 128"),
			wantSubstr: "authentication required",
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
			got := wrapGitError(tt.stderr, tt.err)
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
