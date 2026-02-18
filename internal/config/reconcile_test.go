package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestReconcileGlobalSkills_AddsNewSkill(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "skills")
	configPath := filepath.Join(root, "config.yaml")

	// Create a skill with install metadata
	skillPath := filepath.Join(sourceDir, "my-skill")
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatal(err)
	}
	meta := map[string]string{"source": "github.com/user/repo"}
	data, _ := json.Marshal(meta)
	if err := os.WriteFile(filepath.Join(skillPath, ".skillshare-meta.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	// Write initial config
	cfgData, _ := yaml.Marshal(&Config{Source: sourceDir})
	if err := os.WriteFile(configPath, cfgData, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SKILLSHARE_CONFIG", configPath)

	cfg := &Config{Source: sourceDir}

	if err := ReconcileGlobalSkills(cfg); err != nil {
		t.Fatalf("ReconcileGlobalSkills failed: %v", err)
	}

	if len(cfg.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(cfg.Skills))
	}
	if cfg.Skills[0].Name != "my-skill" {
		t.Errorf("expected skill name 'my-skill', got %q", cfg.Skills[0].Name)
	}
	if cfg.Skills[0].Source != "github.com/user/repo" {
		t.Errorf("expected source 'github.com/user/repo', got %q", cfg.Skills[0].Source)
	}
}

func TestReconcileGlobalSkills_UpdatesExistingSource(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "skills")
	configPath := filepath.Join(root, "config.yaml")

	skillPath := filepath.Join(sourceDir, "my-skill")
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatal(err)
	}
	meta := map[string]string{"source": "github.com/user/repo-v2"}
	data, _ := json.Marshal(meta)
	if err := os.WriteFile(filepath.Join(skillPath, ".skillshare-meta.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	cfgData, _ := yaml.Marshal(&Config{Source: sourceDir})
	if err := os.WriteFile(configPath, cfgData, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SKILLSHARE_CONFIG", configPath)

	cfg := &Config{
		Source: sourceDir,
		Skills: []SkillEntry{{Name: "my-skill", Source: "github.com/user/repo-v1"}},
	}

	if err := ReconcileGlobalSkills(cfg); err != nil {
		t.Fatalf("ReconcileGlobalSkills failed: %v", err)
	}

	if len(cfg.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(cfg.Skills))
	}
	if cfg.Skills[0].Source != "github.com/user/repo-v2" {
		t.Errorf("expected updated source 'github.com/user/repo-v2', got %q", cfg.Skills[0].Source)
	}
}

func TestReconcileGlobalSkills_SkipsNoMeta(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "skills")
	configPath := filepath.Join(root, "config.yaml")

	// Create a skill directory without metadata
	skillPath := filepath.Join(sourceDir, "local-skill")
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillPath, "SKILL.md"), []byte("# Local skill"), 0644); err != nil {
		t.Fatal(err)
	}

	cfgData, _ := yaml.Marshal(&Config{Source: sourceDir})
	if err := os.WriteFile(configPath, cfgData, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SKILLSHARE_CONFIG", configPath)

	cfg := &Config{Source: sourceDir}

	if err := ReconcileGlobalSkills(cfg); err != nil {
		t.Fatalf("ReconcileGlobalSkills failed: %v", err)
	}

	if len(cfg.Skills) != 0 {
		t.Errorf("expected 0 skills (no meta), got %d", len(cfg.Skills))
	}
}

func TestReconcileGlobalSkills_EmptyDir(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "skills")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{Source: sourceDir}

	if err := ReconcileGlobalSkills(cfg); err != nil {
		t.Fatalf("ReconcileGlobalSkills failed: %v", err)
	}

	if len(cfg.Skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(cfg.Skills))
	}
}

func TestReconcileGlobalSkills_MissingDir(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "skills") // does not exist

	cfg := &Config{Source: sourceDir}

	if err := ReconcileGlobalSkills(cfg); err != nil {
		t.Fatalf("ReconcileGlobalSkills should not fail for missing dir: %v", err)
	}
}
