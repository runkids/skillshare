package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoadRegistry_Empty(t *testing.T) {
	dir := t.TempDir()
	reg, err := LoadRegistry(dir)
	if err != nil {
		t.Fatalf("LoadRegistry should succeed for missing file: %v", err)
	}
	if len(reg.Skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(reg.Skills))
	}
}

func TestRegistry_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	reg := &Registry{
		Skills: []SkillEntry{
			{Name: "my-skill", Source: "github.com/user/repo"},
			{Name: "nested", Source: "github.com/org/team", Tracked: true, Group: "frontend"},
		},
	}
	if err := reg.Save(dir); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	path := filepath.Join(dir, "registry.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("registry.yaml not created: %v", err)
	}

	loaded, err := LoadRegistry(dir)
	if err != nil {
		t.Fatalf("LoadRegistry failed: %v", err)
	}
	if len(loaded.Skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(loaded.Skills))
	}
	if loaded.Skills[0].Name != "my-skill" {
		t.Errorf("expected 'my-skill', got %q", loaded.Skills[0].Name)
	}
	if loaded.Skills[1].Group != "frontend" {
		t.Errorf("expected group 'frontend', got %q", loaded.Skills[1].Group)
	}
}

func TestMigrateGlobalSkillsToRegistry(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	sourceDir := filepath.Join(dir, "skills")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write old-format config with skills[]
	oldConfig := "source: " + sourceDir + "\nskills:\n  - name: my-skill\n    source: github.com/user/repo\n"
	if err := os.WriteFile(configPath, []byte(oldConfig), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SKILLSHARE_CONFIG", configPath)

	_, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// registry.yaml should exist with the skill
	reg, err := LoadRegistry(dir)
	if err != nil {
		t.Fatalf("LoadRegistry failed: %v", err)
	}
	if len(reg.Skills) != 1 {
		t.Fatalf("expected 1 skill in registry, got %d", len(reg.Skills))
	}
	if reg.Skills[0].Name != "my-skill" {
		t.Errorf("expected 'my-skill', got %q", reg.Skills[0].Name)
	}

	// Re-read config.yaml — should no longer contain skills key
	data, _ := os.ReadFile(configPath)
	var check map[string]any
	yaml.Unmarshal(data, &check)
	if _, hasSkills := check["skills"]; hasSkills {
		t.Error("config.yaml should not contain skills: after migration")
	}
}

func TestMigrateGlobalSkills_NoMigrationWhenRegistryExists(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	sourceDir := filepath.Join(dir, "skills")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write config with skills (old format)
	oldConfig := "source: " + sourceDir + "\nskills:\n  - name: stale\n    source: old\n"
	if err := os.WriteFile(configPath, []byte(oldConfig), 0644); err != nil {
		t.Fatal(err)
	}

	// Pre-existing registry.yaml — should NOT be overwritten
	reg := &Registry{Skills: []SkillEntry{{Name: "real", Source: "github.com/real"}}}
	if err := reg.Save(dir); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SKILLSHARE_CONFIG", configPath)

	_, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// registry should still have "real", not "stale"
	loaded, err := LoadRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Skills) != 1 || loaded.Skills[0].Name != "real" {
		t.Errorf("registry should be untouched, got: %+v", loaded.Skills)
	}
}

func TestMigrateProjectSkillsToRegistry(t *testing.T) {
	root := t.TempDir()
	skillshareDir := filepath.Join(root, ".skillshare")
	if err := os.MkdirAll(skillshareDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(skillshareDir, "config.yaml")

	// Write old-format project config with skills
	oldConfig := "targets:\n  - claude\nskills:\n  - name: my-skill\n    source: github.com/user/repo\n"
	if err := os.WriteFile(configPath, []byte(oldConfig), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadProject(root)
	if err != nil {
		t.Fatalf("LoadProject failed: %v", err)
	}

	reg, err := LoadRegistry(skillshareDir)
	if err != nil {
		t.Fatalf("LoadRegistry failed: %v", err)
	}
	if len(reg.Skills) != 1 {
		t.Fatalf("expected 1 skill in registry, got %d", len(reg.Skills))
	}
	if reg.Skills[0].Name != "my-skill" {
		t.Errorf("expected 'my-skill', got %q", reg.Skills[0].Name)
	}
}
