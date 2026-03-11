package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProject_ExtrasConfig(t *testing.T) {
	root := t.TempDir()
	skillshareDir := filepath.Join(root, ".skillshare")
	if err := os.MkdirAll(skillshareDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(skillshareDir, "config.yaml")

	os.WriteFile(configPath, []byte(`targets:
  - claude
extras:
  - name: rules
    targets:
      - path: "relative/rules"
      - path: "/abs/other/rules"
        mode: copy
`), 0644)

	cfg, err := LoadProject(root)
	if err != nil {
		t.Fatalf("LoadProject failed: %v", err)
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

	// Project extras paths remain relative (no ~ expansion or absolutizing)
	if cfg.Extras[0].Targets[0].Path != "relative/rules" {
		t.Errorf("relative path should be unchanged, got %q", cfg.Extras[0].Targets[0].Path)
	}
	if cfg.Extras[0].Targets[0].Mode != "" {
		t.Errorf("mode should be empty (default merge), got %q", cfg.Extras[0].Targets[0].Mode)
	}
	if cfg.Extras[0].Targets[1].Path != "/abs/other/rules" {
		t.Errorf("absolute path should be unchanged, got %q", cfg.Extras[0].Targets[1].Path)
	}
	if cfg.Extras[0].Targets[1].Mode != "copy" {
		t.Errorf("mode = %q, want %q", cfg.Extras[0].Targets[1].Mode, "copy")
	}
}

func TestLoadProject_NoExtras(t *testing.T) {
	root := t.TempDir()
	skillshareDir := filepath.Join(root, ".skillshare")
	if err := os.MkdirAll(skillshareDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(skillshareDir, "config.yaml")

	os.WriteFile(configPath, []byte(`targets:
  - claude
`), 0644)

	cfg, err := LoadProject(root)
	if err != nil {
		t.Fatalf("LoadProject failed: %v", err)
	}

	if len(cfg.Extras) != 0 {
		t.Errorf("expected 0 extras, got %d", len(cfg.Extras))
	}
}

func TestLoadProject_ExtrasPathsNotExpanded(t *testing.T) {
	root := t.TempDir()
	skillshareDir := filepath.Join(root, ".skillshare")
	if err := os.MkdirAll(skillshareDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(skillshareDir, "config.yaml")

	// Use a tilde path — project config should NOT expand it (unlike global config)
	os.WriteFile(configPath, []byte(`targets:
  - claude
extras:
  - name: mcp-rules
    targets:
      - path: "~/some/rules"
`), 0644)

	cfg, err := LoadProject(root)
	if err != nil {
		t.Fatalf("LoadProject failed: %v", err)
	}

	if len(cfg.Extras) != 1 {
		t.Fatalf("expected 1 extra, got %d", len(cfg.Extras))
	}
	// Path should remain as-is (relative, not expanded)
	if cfg.Extras[0].Targets[0].Path != "~/some/rules" {
		t.Errorf("tilde path should remain unexpanded in project config, got %q", cfg.Extras[0].Targets[0].Path)
	}
}
