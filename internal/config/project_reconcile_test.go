package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReconcileProjectSkills_AddsNewSkill(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".skillshare", "skills")

	// Create a skill with install metadata
	skillPath := filepath.Join(skillsDir, "my-skill")
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatal(err)
	}
	meta := map[string]string{"source": "github.com/user/repo"}
	data, _ := json.Marshal(meta)
	if err := os.WriteFile(filepath.Join(skillPath, ".skillshare-meta.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &ProjectConfig{
		Targets: []ProjectTargetEntry{{Name: "claude"}},
	}

	if err := ReconcileProjectSkills(root, cfg, skillsDir); err != nil {
		t.Fatalf("ReconcileProjectSkills failed: %v", err)
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

func TestReconcileProjectSkills_UpdatesExistingSource(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".skillshare", "skills")

	skillPath := filepath.Join(skillsDir, "my-skill")
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatal(err)
	}
	meta := map[string]string{"source": "github.com/user/repo-v2"}
	data, _ := json.Marshal(meta)
	if err := os.WriteFile(filepath.Join(skillPath, ".skillshare-meta.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &ProjectConfig{
		Targets: []ProjectTargetEntry{{Name: "claude"}},
		Skills:  []SkillEntry{{Name: "my-skill", Source: "github.com/user/repo-v1"}},
	}

	if err := ReconcileProjectSkills(root, cfg, skillsDir); err != nil {
		t.Fatalf("ReconcileProjectSkills failed: %v", err)
	}

	if len(cfg.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(cfg.Skills))
	}
	if cfg.Skills[0].Source != "github.com/user/repo-v2" {
		t.Errorf("expected updated source 'github.com/user/repo-v2', got %q", cfg.Skills[0].Source)
	}
}

func TestReconcileProjectSkills_SkipsNoMeta(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".skillshare", "skills")

	// Create a skill directory without metadata
	skillPath := filepath.Join(skillsDir, "local-skill")
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatal(err)
	}
	// Write a SKILL.md but no meta file
	if err := os.WriteFile(filepath.Join(skillPath, "SKILL.md"), []byte("# Local skill"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &ProjectConfig{
		Targets: []ProjectTargetEntry{{Name: "claude"}},
	}

	if err := ReconcileProjectSkills(root, cfg, skillsDir); err != nil {
		t.Fatalf("ReconcileProjectSkills failed: %v", err)
	}

	if len(cfg.Skills) != 0 {
		t.Errorf("expected 0 skills (no meta), got %d", len(cfg.Skills))
	}
}

func TestReconcileProjectSkills_EmptyDir(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".skillshare", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &ProjectConfig{}

	if err := ReconcileProjectSkills(root, cfg, skillsDir); err != nil {
		t.Fatalf("ReconcileProjectSkills failed: %v", err)
	}

	if len(cfg.Skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(cfg.Skills))
	}
}

func TestReconcileProjectSkills_MissingDir(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".skillshare", "skills") // does not exist

	cfg := &ProjectConfig{}

	if err := ReconcileProjectSkills(root, cfg, skillsDir); err != nil {
		t.Fatalf("ReconcileProjectSkills should not fail for missing dir: %v", err)
	}
}
