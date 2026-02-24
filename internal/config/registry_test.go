package config

import (
	"os"
	"path/filepath"
	"testing"
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
