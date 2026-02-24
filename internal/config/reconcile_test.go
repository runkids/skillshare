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

	// Write initial config (needed for ConfigPath() resolution)
	cfgData, _ := yaml.Marshal(&Config{Source: sourceDir})
	if err := os.WriteFile(configPath, cfgData, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SKILLSHARE_CONFIG", configPath)

	cfg := &Config{Source: sourceDir}
	reg := &Registry{}

	if err := ReconcileGlobalSkills(cfg, reg); err != nil {
		t.Fatalf("ReconcileGlobalSkills failed: %v", err)
	}

	if len(reg.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(reg.Skills))
	}
	if reg.Skills[0].Name != "my-skill" {
		t.Errorf("expected skill name 'my-skill', got %q", reg.Skills[0].Name)
	}
	if reg.Skills[0].Source != "github.com/user/repo" {
		t.Errorf("expected source 'github.com/user/repo', got %q", reg.Skills[0].Source)
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

	cfg := &Config{Source: sourceDir}
	reg := &Registry{
		Skills: []SkillEntry{{Name: "my-skill", Source: "github.com/user/repo-v1"}},
	}

	if err := ReconcileGlobalSkills(cfg, reg); err != nil {
		t.Fatalf("ReconcileGlobalSkills failed: %v", err)
	}

	if len(reg.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(reg.Skills))
	}
	if reg.Skills[0].Source != "github.com/user/repo-v2" {
		t.Errorf("expected updated source 'github.com/user/repo-v2', got %q", reg.Skills[0].Source)
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
	reg := &Registry{}

	if err := ReconcileGlobalSkills(cfg, reg); err != nil {
		t.Fatalf("ReconcileGlobalSkills failed: %v", err)
	}

	if len(reg.Skills) != 0 {
		t.Errorf("expected 0 skills (no meta), got %d", len(reg.Skills))
	}
}

func TestReconcileGlobalSkills_EmptyDir(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "skills")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{Source: sourceDir}
	reg := &Registry{}

	if err := ReconcileGlobalSkills(cfg, reg); err != nil {
		t.Fatalf("ReconcileGlobalSkills failed: %v", err)
	}

	if len(reg.Skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(reg.Skills))
	}
}

func TestReconcileGlobalSkills_MissingDir(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "skills") // does not exist

	cfg := &Config{Source: sourceDir}
	reg := &Registry{}

	if err := ReconcileGlobalSkills(cfg, reg); err != nil {
		t.Fatalf("ReconcileGlobalSkills should not fail for missing dir: %v", err)
	}
}

func TestReconcileGlobalSkills_NestedSkillSetsGroup(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "skills")
	configPath := filepath.Join(root, "config.yaml")

	// Create a nested skill: frontend/pdf
	skillPath := filepath.Join(sourceDir, "frontend", "pdf")
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatal(err)
	}
	meta := map[string]string{"source": "anthropics/skills/skills/pdf"}
	data, _ := json.Marshal(meta)
	if err := os.WriteFile(filepath.Join(skillPath, ".skillshare-meta.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	cfgData, _ := yaml.Marshal(&Config{Source: sourceDir})
	if err := os.WriteFile(configPath, cfgData, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SKILLSHARE_CONFIG", configPath)

	cfg := &Config{Source: sourceDir}
	reg := &Registry{}

	if err := ReconcileGlobalSkills(cfg, reg); err != nil {
		t.Fatalf("ReconcileGlobalSkills failed: %v", err)
	}

	if len(reg.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(reg.Skills))
	}
	if reg.Skills[0].Name != "pdf" {
		t.Errorf("expected bare name 'pdf', got %q", reg.Skills[0].Name)
	}
	if reg.Skills[0].Group != "frontend" {
		t.Errorf("expected group 'frontend', got %q", reg.Skills[0].Group)
	}
	if reg.Skills[0].FullName() != "frontend/pdf" {
		t.Errorf("expected FullName 'frontend/pdf', got %q", reg.Skills[0].FullName())
	}
}

func TestReconcileGlobalSkills_MigratesLegacySlashName(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "skills")
	configPath := filepath.Join(root, "config.yaml")

	// Create nested skill on disk
	skillPath := filepath.Join(sourceDir, "frontend", "pdf")
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatal(err)
	}
	meta := map[string]string{"source": "anthropics/skills/skills/pdf"}
	data, _ := json.Marshal(meta)
	if err := os.WriteFile(filepath.Join(skillPath, ".skillshare-meta.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	cfgData, _ := yaml.Marshal(&Config{Source: sourceDir})
	if err := os.WriteFile(configPath, cfgData, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SKILLSHARE_CONFIG", configPath)

	// Start with legacy format: name contains slash, no group
	cfg := &Config{Source: sourceDir}
	reg := &Registry{
		Skills: []SkillEntry{{Name: "frontend/pdf", Source: "anthropics/skills/skills/pdf"}},
	}

	if err := ReconcileGlobalSkills(cfg, reg); err != nil {
		t.Fatalf("ReconcileGlobalSkills failed: %v", err)
	}

	if len(reg.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(reg.Skills))
	}
	if reg.Skills[0].Name != "pdf" {
		t.Errorf("expected migrated name 'pdf', got %q", reg.Skills[0].Name)
	}
	if reg.Skills[0].Group != "frontend" {
		t.Errorf("expected migrated group 'frontend', got %q", reg.Skills[0].Group)
	}
}
