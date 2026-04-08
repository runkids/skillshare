package config

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/install"
)

func TestReconcileProjectSkills_AddsNewSkill(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".skillshare", "skills")

	// Create a skill directory on disk
	skillPath := filepath.Join(skillsDir, "my-skill")
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &ProjectConfig{
		Targets: []ProjectTargetEntry{{Name: "claude"}},
	}
	// Pre-populate store with the entry (simulating post-install state)
	store := install.NewMetadataStore()
	store.Set("my-skill", &install.MetadataEntry{Source: "github.com/user/repo"})

	if err := ReconcileProjectSkills(root, cfg, store, skillsDir); err != nil {
		t.Fatalf("ReconcileProjectSkills failed: %v", err)
	}

	if !store.Has("my-skill") {
		t.Fatal("expected store to have 'my-skill'")
	}
	entry := store.Get("my-skill")
	if entry.Source != "github.com/user/repo" {
		t.Errorf("expected source 'github.com/user/repo', got %q", entry.Source)
	}
}

func TestReconcileProjectSkills_UpdatesExistingSource(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".skillshare", "skills")

	skillPath := filepath.Join(skillsDir, "my-skill")
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &ProjectConfig{
		Targets: []ProjectTargetEntry{{Name: "claude"}},
	}
	store := install.NewMetadataStore()
	store.Set("my-skill", &install.MetadataEntry{Source: "github.com/user/repo-v1"})

	if err := ReconcileProjectSkills(root, cfg, store, skillsDir); err != nil {
		t.Fatalf("ReconcileProjectSkills failed: %v", err)
	}

	entry := store.Get("my-skill")
	if entry == nil {
		t.Fatal("expected store to have 'my-skill'")
	}
	if entry.Source != "github.com/user/repo-v1" {
		t.Errorf("expected source 'github.com/user/repo-v1', got %q", entry.Source)
	}
}

func TestReconcileProjectSkills_SkipsNoMeta(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".skillshare", "skills")

	// Create a skill directory without metadata in the store
	skillPath := filepath.Join(skillsDir, "local-skill")
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillPath, "SKILL.md"), []byte("# Local skill"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &ProjectConfig{
		Targets: []ProjectTargetEntry{{Name: "claude"}},
	}
	store := install.NewMetadataStore()

	if err := ReconcileProjectSkills(root, cfg, store, skillsDir); err != nil {
		t.Fatalf("ReconcileProjectSkills failed: %v", err)
	}

	if len(store.List()) != 0 {
		t.Errorf("expected 0 entries (no meta), got %d", len(store.List()))
	}
}

func TestReconcileProjectSkills_EmptyDir(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".skillshare", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &ProjectConfig{}
	store := install.NewMetadataStore()

	if err := ReconcileProjectSkills(root, cfg, store, skillsDir); err != nil {
		t.Fatalf("ReconcileProjectSkills failed: %v", err)
	}

	if len(store.List()) != 0 {
		t.Errorf("expected 0 entries, got %d", len(store.List()))
	}
}

func TestReconcileProjectSkills_MissingDir(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".skillshare", "skills") // does not exist

	cfg := &ProjectConfig{}
	store := install.NewMetadataStore()

	if err := ReconcileProjectSkills(root, cfg, store, skillsDir); err != nil {
		t.Fatalf("ReconcileProjectSkills should not fail for missing dir: %v", err)
	}
}

func TestReconcileProjectSkills_NestedSkillSetsGroup(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".skillshare", "skills")

	// Create a nested skill: tools/my-skill
	skillPath := filepath.Join(skillsDir, "tools", "my-skill")
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &ProjectConfig{
		Targets: []ProjectTargetEntry{{Name: "claude"}},
	}
	store := install.NewMetadataStore()
	store.Set("my-skill", &install.MetadataEntry{
		Source: "github.com/user/repo",
		Group:  "tools",
	})

	if err := ReconcileProjectSkills(root, cfg, store, skillsDir); err != nil {
		t.Fatalf("ReconcileProjectSkills failed: %v", err)
	}

	entry := store.Get("my-skill")
	if entry == nil {
		t.Fatal("expected store to have 'my-skill'")
	}
	if entry.Group != "tools" {
		t.Errorf("expected group 'tools', got %q", entry.Group)
	}
}

func TestReconcileProjectSkills_PrunesStaleEntries(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".skillshare", "skills")

	// Create only one skill on disk
	skillPath := filepath.Join(skillsDir, "alive-skill")
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &ProjectConfig{
		Targets: []ProjectTargetEntry{{Name: "claude"}},
	}
	store := install.NewMetadataStore()
	store.Set("alive-skill", &install.MetadataEntry{Source: "github.com/user/alive"})
	store.Set("deleted-skill", &install.MetadataEntry{Source: "github.com/user/deleted"})

	if err := ReconcileProjectSkills(root, cfg, store, skillsDir); err != nil {
		t.Fatalf("ReconcileProjectSkills failed: %v", err)
	}

	names := store.List()
	if len(names) != 1 {
		t.Fatalf("expected 1 entry after prune, got %d: %v", len(names), names)
	}
	if !store.Has("alive-skill") {
		t.Error("expected alive-skill to survive prune")
	}
	if store.Has("deleted-skill") {
		t.Error("expected deleted-skill to be pruned")
	}
}
