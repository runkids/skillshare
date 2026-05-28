package config

import (
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
