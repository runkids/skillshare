package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEffectiveGitRoot(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	base := BaseDir()

	cases := []struct {
		gitRoot string
		want    string
	}{
		{"", filepath.Join(base, "skills")},
		{"skills", filepath.Join(base, "skills")},
		{"agents", filepath.Join(base, "agents")},
		{"extras", filepath.Join(base, "extras")},
		{"root", base},
	}
	for _, c := range cases {
		cfg := &Config{GitRoot: c.gitRoot}
		if got := cfg.EffectiveGitRoot(); got != c.want {
			t.Errorf("GitRoot=%q: got %q, want %q", c.gitRoot, got, c.want)
		}
	}
}

func TestValidGitRoot(t *testing.T) {
	for _, s := range []string{"", "skills", "agents", "extras", "root"} {
		if !ValidGitRoot(s) {
			t.Errorf("ValidGitRoot(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"foo", "Root", "all", "global"} {
		if ValidGitRoot(s) {
			t.Errorf("ValidGitRoot(%q) = true, want false", s)
		}
	}
}

func TestGitRootMismatch(t *testing.T) {
	base := t.TempDir()
	skills := filepath.Join(base, "skills")
	agents := filepath.Join(base, "agents")
	for _, d := range []string{skills, agents} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cfg := &Config{
		GitRoot: "agents", // configured to operate on agents/
		Sources: GlobalSources{Skills: skills, Agents: agents},
	}

	// No repo anywhere → no mismatch.
	if _, _, m := cfg.GitRootMismatch(); m {
		t.Errorf("no repo present: expected no mismatch")
	}

	// Repo at skills/ but git_root=agents → mismatch pointing at skills.
	if err := os.MkdirAll(filepath.Join(skills, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	scope, dir, m := cfg.GitRootMismatch()
	if !m || scope != "skills" || dir != skills {
		t.Errorf("expected mismatch at skills, got scope=%q dir=%q mismatch=%v", scope, dir, m)
	}

	// Repo also at agents/ (the configured root) → no mismatch (configured wins).
	if err := os.MkdirAll(filepath.Join(agents, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, m := cfg.GitRootMismatch(); m {
		t.Errorf("configured root has a repo: expected no mismatch")
	}
}
