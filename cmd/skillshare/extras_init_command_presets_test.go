package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
)

func TestResolveExtraInitTargetPresets_CommandsGlobal(t *testing.T) {
	got, err := resolveExtraInitTargetPresets("commands", []string{"claude", "cursor", "codex", "~/.custom/commands"}, modeGlobal)
	if err != nil {
		t.Fatalf("resolve presets: %v", err)
	}
	want := []string{"~/.claude/commands", "~/.cursor/commands", "~/.codex/prompts", "~/.custom/commands"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("targets = %#v, want %#v", got, want)
	}
}

func TestResolveExtraInitTargetPresets_CommandsProject(t *testing.T) {
	got, err := resolveExtraInitTargetPresets("commands", []string{"claude-code", "cursor"}, modeProject)
	if err != nil {
		t.Fatalf("resolve presets: %v", err)
	}
	want := []string{".claude/commands", ".cursor/commands"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("targets = %#v, want %#v", got, want)
	}
}

func TestResolveExtraInitTargetPresets_ProjectCodexRejected(t *testing.T) {
	_, err := resolveExtraInitTargetPresets("commands", []string{"codex"}, modeProject)
	if err == nil {
		t.Fatal("expected project codex preset to fail")
	}
	if !strings.Contains(err.Error(), "global-only") {
		t.Fatalf("expected global-only error, got %v", err)
	}
}

func TestResolveExtraInitTargetPresets_NonCommandsAreRawPaths(t *testing.T) {
	got, err := resolveExtraInitTargetPresets("rules", []string{"claude", "cursor"}, modeGlobal)
	if err != nil {
		t.Fatalf("resolve presets: %v", err)
	}
	want := []string{"claude", "cursor"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("targets = %#v, want %#v", got, want)
	}
}

func TestCmdExtrasInit_CommandsGlobalPresets(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	t.Setenv("HOME", home)
	t.Setenv("SKILLSHARE_CONFIG", filepath.Join(root, "config.yaml"))

	cfg := &config.Config{
		Source:  filepath.Join(root, "skills"),
		Targets: map[string]config.TargetConfig{},
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	if err := cmdExtrasInit([]string{"--global", "commands", "--target", "claude", "--target", "cursor", "--target", "codex"}); err != nil {
		t.Fatalf("extras init: %v", err)
	}

	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(loaded.Extras) != 1 {
		t.Fatalf("expected one extra, got %d", len(loaded.Extras))
	}
	got := extraTargetPaths(loaded.Extras[0])
	want := []string{
		filepath.Join(home, ".claude", "commands"),
		filepath.Join(home, ".cursor", "commands"),
		filepath.Join(home, ".codex", "prompts"),
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("targets = %#v, want %#v", got, want)
	}
}

func TestCmdExtrasInit_CommandsProjectPresets(t *testing.T) {
	root := t.TempDir()
	prevCWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir project root: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(prevCWD)
	})

	projCfg := &config.ProjectConfig{Targets: []config.ProjectTargetEntry{{Name: "claude"}, {Name: "cursor"}}}
	if err := projCfg.Save(root); err != nil {
		t.Fatalf("save project config: %v", err)
	}

	if err := cmdExtrasInit([]string{"--project", "commands", "--target", "claude", "--target", "cursor"}); err != nil {
		t.Fatalf("extras init: %v", err)
	}

	loaded, err := config.LoadProject(root)
	if err != nil {
		t.Fatalf("load project config: %v", err)
	}
	if len(loaded.Extras) != 1 {
		t.Fatalf("expected one extra, got %d", len(loaded.Extras))
	}
	got := extraTargetPaths(loaded.Extras[0])
	want := []string{".claude/commands", ".cursor/commands"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("targets = %#v, want %#v", got, want)
	}
}

func extraTargetPaths(extra config.ExtraConfig) []string {
	paths := make([]string, 0, len(extra.Targets))
	for _, target := range extra.Targets {
		paths = append(paths, target.Path)
	}
	return paths
}
