package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fork patch tests: preserve_tilde_on_save opt-in behavior.

func TestSave_DefaultDoesNotFoldTilde(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", cfgPath)

	home, _ := os.UserHomeDir()
	cfg := &Config{
		Source: filepath.Join(home, "dotfiles", "skills"),
		Targets: map[string]TargetConfig{
			"claude": {Skills: &ResourceTargetConfig{Path: filepath.Join(home, ".claude", "skills")}},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, home) {
		t.Errorf("default Save should keep absolute paths; got:\n%s", got)
	}
	if strings.Contains(got, "~") && !strings.Contains(got, "~ ") {
		// allow incidental ~ chars in comments; flag any ~/ path style
		if strings.Contains(got, "~/") {
			t.Errorf("default Save unexpectedly folded to ~; got:\n%s", got)
		}
	}
}

func TestSave_PreserveTildeOnSave_FoldsHomePaths(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", cfgPath)

	home, _ := os.UserHomeDir()
	cfg := &Config{
		Source:              filepath.Join(home, "dotfiles", "skills"),
		ExtrasSource:        filepath.Join(home, "my-extras"),
		PreserveTildeOnSave: true,
		Targets: map[string]TargetConfig{
			"claude": {Skills: &ResourceTargetConfig{Path: filepath.Join(home, ".claude", "skills")}},
			"cursor": {Skills: &ResourceTargetConfig{Path: filepath.Join(home, ".cursor", "skills")}},
			// Non-home absolute path must NOT be folded
			"weird": {Skills: &ResourceTargetConfig{Path: "/opt/weird/skills"}},
		},
		Extras: []ExtraConfig{
			{
				Name:   "rules",
				Source: filepath.Join(home, "dotfiles", "rules"),
				Targets: []ExtraTargetConfig{
					{Path: filepath.Join(home, ".claude", "rules")},
				},
			},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(cfgPath)
	got := string(data)

	// Home-rooted paths must be folded
	for _, want := range []string{
		"~/dotfiles/skills",
		"~/my-extras",
		"~/.claude/skills",
		"~/.cursor/skills",
		"~/dotfiles/rules",
		"~/.claude/rules",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in saved yaml; got:\n%s", want, got)
		}
	}

	// Non-home absolute path must remain absolute
	if !strings.Contains(got, "/opt/weird/skills") {
		t.Errorf("non-home absolute path should remain absolute; got:\n%s", got)
	}

	// $HOME should not appear anywhere
	if strings.Contains(got, home+"/") || strings.HasSuffix(strings.TrimSpace(got), home) {
		t.Errorf("$HOME prefix should not appear in saved yaml; home=%q got:\n%s", home, got)
	}

	// In-memory cfg must not be mutated
	if strings.HasPrefix(cfg.Source, "~") {
		t.Errorf("in-memory cfg.Source was mutated to %q", cfg.Source)
	}
	if cfg.Targets["claude"].Skills.Path[0] == '~' {
		t.Errorf("in-memory Targets[claude].Skills.Path was mutated")
	}
}

func TestSave_PreserveTildeOnSave_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", cfgPath)

	home, _ := os.UserHomeDir()
	cfg := &Config{
		Source:              filepath.Join(home, "skills"),
		PreserveTildeOnSave: true,
		Targets: map[string]TargetConfig{
			"claude": {Skills: &ResourceTargetConfig{Path: filepath.Join(home, ".claude", "skills")}},
		},
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(cfgPath)

	// Load and Save again; on-disk yaml should be idempotent
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.PreserveTildeOnSave {
		t.Fatal("PreserveTildeOnSave flag should survive roundtrip")
	}
	if err := loaded.Save(); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(cfgPath)
	if string(first) != string(second) {
		t.Errorf("Save→Load→Save is not idempotent\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}
