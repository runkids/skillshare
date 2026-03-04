package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ExtrasConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte(`
source: /tmp/skills
targets:
  claude:
    path: /tmp/claude/skills
extras:
  - name: rules
    targets:
      - path: "~/fake/rules"
      - path: "/tmp/other/rules"
        mode: copy
`), 0644)

	t.Setenv("SKILLSHARE_CONFIG", cfgPath)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Extras) != 1 {
		t.Fatalf("expected 1 extra, got %d", len(cfg.Extras))
	}
	if cfg.Extras[0].Name != "rules" {
		t.Errorf("name = %q, want %q", cfg.Extras[0].Name, "rules")
	}
	if len(cfg.Extras[0].Targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(cfg.Extras[0].Targets))
	}
	// Verify ~ expansion
	home, _ := os.UserHomeDir()
	if cfg.Extras[0].Targets[0].Path != filepath.Join(home, "fake/rules") {
		t.Errorf("path not expanded: %q", cfg.Extras[0].Targets[0].Path)
	}
	// Verify mode
	if cfg.Extras[0].Targets[0].Mode != "" {
		t.Errorf("mode should be empty (default merge), got %q", cfg.Extras[0].Targets[0].Mode)
	}
	if cfg.Extras[0].Targets[1].Mode != "copy" {
		t.Errorf("mode = %q, want %q", cfg.Extras[0].Targets[1].Mode, "copy")
	}
}

func TestLoad_NoExtras(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte(`
source: /tmp/skills
targets:
  claude:
    path: /tmp/claude/skills
`), 0644)

	t.Setenv("SKILLSHARE_CONFIG", cfgPath)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Extras) != 0 {
		t.Errorf("expected 0 extras, got %d", len(cfg.Extras))
	}
}
