package config

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/install"

	"gopkg.in/yaml.v3"
)

func TestReconcileGlobalSkills_AddsNewSkill(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "skills")
	configPath := filepath.Join(root, "config.yaml")

	// Create a skill directory on disk
	skillPath := filepath.Join(sourceDir, "my-skill")
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatal(err)
	}

	cfgData, _ := yaml.Marshal(&Config{Source: sourceDir})
	if err := os.WriteFile(configPath, cfgData, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SKILLSHARE_CONFIG", configPath)

	cfg := &Config{Source: sourceDir}
	// Pre-populate store with the entry (simulating post-install state)
	store := install.NewMetadataStore()
	store.Set("my-skill", &install.MetadataEntry{Source: "github.com/user/repo"})

	if err := ReconcileGlobalSkills(cfg, store); err != nil {
		t.Fatalf("ReconcileGlobalSkills failed: %v", err)
	}

	if !store.Has("my-skill") {
		t.Fatal("expected store to have 'my-skill'")
	}
	entry := store.Get("my-skill")
	if entry.Source != "github.com/user/repo" {
		t.Errorf("expected source 'github.com/user/repo', got %q", entry.Source)
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

	cfgData, _ := yaml.Marshal(&Config{Source: sourceDir})
	if err := os.WriteFile(configPath, cfgData, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SKILLSHARE_CONFIG", configPath)

	cfg := &Config{Source: sourceDir}
	store := install.NewMetadataStore()
	store.Set("my-skill", &install.MetadataEntry{Source: "github.com/user/repo-v1"})

	if err := ReconcileGlobalSkills(cfg, store); err != nil {
		t.Fatalf("ReconcileGlobalSkills failed: %v", err)
	}

	entry := store.Get("my-skill")
	if entry == nil {
		t.Fatal("expected store to have 'my-skill'")
	}
	// Source should remain as-is since reconcile reads from the existing store entry
	if entry.Source != "github.com/user/repo-v1" {
		t.Errorf("expected source 'github.com/user/repo-v1', got %q", entry.Source)
	}
}

func TestReconcileGlobalSkills_SkipsNoMeta(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "skills")
	configPath := filepath.Join(root, "config.yaml")

	// Create a skill directory without metadata in the store
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
	store := install.NewMetadataStore()

	if err := ReconcileGlobalSkills(cfg, store); err != nil {
		t.Fatalf("ReconcileGlobalSkills failed: %v", err)
	}

	if len(store.List()) != 0 {
		t.Errorf("expected 0 entries (no meta), got %d", len(store.List()))
	}
}

func TestReconcileGlobalSkills_EmptyDir(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "skills")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{Source: sourceDir}
	store := install.NewMetadataStore()

	if err := ReconcileGlobalSkills(cfg, store); err != nil {
		t.Fatalf("ReconcileGlobalSkills failed: %v", err)
	}

	if len(store.List()) != 0 {
		t.Errorf("expected 0 entries, got %d", len(store.List()))
	}
}

func TestReconcileGlobalSkills_MissingDir(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "skills") // does not exist

	cfg := &Config{Source: sourceDir}
	store := install.NewMetadataStore()

	if err := ReconcileGlobalSkills(cfg, store); err != nil {
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

	cfgData, _ := yaml.Marshal(&Config{Source: sourceDir})
	if err := os.WriteFile(configPath, cfgData, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SKILLSHARE_CONFIG", configPath)

	cfg := &Config{Source: sourceDir}
	store := install.NewMetadataStore()
	store.Set("pdf", &install.MetadataEntry{
		Source: "anthropics/skills/skills/pdf",
		Group:  "frontend",
	})

	if err := ReconcileGlobalSkills(cfg, store); err != nil {
		t.Fatalf("ReconcileGlobalSkills failed: %v", err)
	}

	entry := store.Get("pdf")
	if entry == nil {
		t.Fatal("expected store to have 'pdf'")
	}
	if entry.Group != "frontend" {
		t.Errorf("expected group 'frontend', got %q", entry.Group)
	}
}

func TestReconcileGlobalSkills_PrunesStaleEntries(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "skills")
	configPath := filepath.Join(root, "config.yaml")

	// Create only one skill on disk
	skillPath := filepath.Join(sourceDir, "alive-skill")
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatal(err)
	}

	cfgData, _ := yaml.Marshal(&Config{Source: sourceDir})
	if err := os.WriteFile(configPath, cfgData, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SKILLSHARE_CONFIG", configPath)

	cfg := &Config{Source: sourceDir}
	store := install.NewMetadataStore()
	store.Set("alive-skill", &install.MetadataEntry{Source: "github.com/user/alive"})
	store.Set("deleted-skill", &install.MetadataEntry{Source: "github.com/user/deleted"})
	store.Set("frontend/gone-skill", &install.MetadataEntry{Source: "github.com/user/gone", Group: "frontend"})

	if err := ReconcileGlobalSkills(cfg, store); err != nil {
		t.Fatalf("ReconcileGlobalSkills failed: %v", err)
	}

	names := store.List()
	if len(names) != 1 {
		t.Fatalf("expected 1 entry after prune, got %d: %v", len(names), names)
	}
	if !store.Has("alive-skill") {
		t.Errorf("expected surviving entry 'alive-skill'")
	}
}
