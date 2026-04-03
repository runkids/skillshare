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
	reg := &Registry{}

	if err := ReconcileProjectSkills(root, cfg, reg, skillsDir); err != nil {
		t.Fatalf("ReconcileProjectSkills failed: %v", err)
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
	}
	reg := &Registry{
		Skills: []SkillEntry{{Name: "my-skill", Source: "github.com/user/repo-v1"}},
	}

	if err := ReconcileProjectSkills(root, cfg, reg, skillsDir); err != nil {
		t.Fatalf("ReconcileProjectSkills failed: %v", err)
	}

	if len(reg.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(reg.Skills))
	}
	if reg.Skills[0].Source != "github.com/user/repo-v2" {
		t.Errorf("expected updated source 'github.com/user/repo-v2', got %q", reg.Skills[0].Source)
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
	reg := &Registry{}

	if err := ReconcileProjectSkills(root, cfg, reg, skillsDir); err != nil {
		t.Fatalf("ReconcileProjectSkills failed: %v", err)
	}

	if len(reg.Skills) != 0 {
		t.Errorf("expected 0 skills (no meta), got %d", len(reg.Skills))
	}
}

func TestReconcileProjectSkills_EmptyDir(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".skillshare", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &ProjectConfig{}
	reg := &Registry{}

	if err := ReconcileProjectSkills(root, cfg, reg, skillsDir); err != nil {
		t.Fatalf("ReconcileProjectSkills failed: %v", err)
	}

	if len(reg.Skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(reg.Skills))
	}
}

func TestReconcileProjectSkills_MissingDir(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".skillshare", "skills") // does not exist

	cfg := &ProjectConfig{}
	reg := &Registry{}

	if err := ReconcileProjectSkills(root, cfg, reg, skillsDir); err != nil {
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
	meta := map[string]string{"source": "github.com/user/repo"}
	data, _ := json.Marshal(meta)
	if err := os.WriteFile(filepath.Join(skillPath, ".skillshare-meta.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &ProjectConfig{
		Targets: []ProjectTargetEntry{{Name: "claude"}},
	}
	reg := &Registry{}

	if err := ReconcileProjectSkills(root, cfg, reg, skillsDir); err != nil {
		t.Fatalf("ReconcileProjectSkills failed: %v", err)
	}

	if len(reg.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(reg.Skills))
	}
	if reg.Skills[0].Name != "my-skill" {
		t.Errorf("expected bare name 'my-skill', got %q", reg.Skills[0].Name)
	}
	if reg.Skills[0].Group != "tools" {
		t.Errorf("expected group 'tools', got %q", reg.Skills[0].Group)
	}
}

func TestReconcileProjectSkills_PrunesStalePreservesAgents(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".skillshare", "skills")

	// Create only one skill on disk
	skillPath := filepath.Join(skillsDir, "alive-skill")
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatal(err)
	}
	meta := map[string]string{"source": "github.com/user/alive"}
	data, _ := json.Marshal(meta)
	if err := os.WriteFile(filepath.Join(skillPath, ".skillshare-meta.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &ProjectConfig{
		Targets: []ProjectTargetEntry{{Name: "claude"}},
	}
	// Registry: alive skill + stale skill + agent (should survive prune)
	reg := &Registry{
		Skills: []SkillEntry{
			{Name: "alive-skill", Source: "github.com/user/alive"},
			{Name: "deleted-skill", Source: "github.com/user/deleted"},
			{Name: "my-agent", Kind: "agent", Source: "github.com/user/agent"},
		},
	}

	if err := ReconcileProjectSkills(root, cfg, reg, skillsDir); err != nil {
		t.Fatalf("ReconcileProjectSkills failed: %v", err)
	}

	if len(reg.Skills) != 2 {
		t.Fatalf("expected 2 entries (alive-skill + agent), got %d: %+v", len(reg.Skills), reg.Skills)
	}

	names := map[string]bool{}
	for _, s := range reg.Skills {
		names[s.Name] = true
	}
	if !names["alive-skill"] {
		t.Error("expected alive-skill to survive prune")
	}
	if !names["my-agent"] {
		t.Error("expected agent entry to survive prune")
	}
	if names["deleted-skill"] {
		t.Error("expected deleted-skill to be pruned")
	}
}

func TestReconcileProjectSkills_MigratesLegacySlashName(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".skillshare", "skills")

	skillPath := filepath.Join(skillsDir, "tools", "my-skill")
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatal(err)
	}
	meta := map[string]string{"source": "github.com/user/repo"}
	data, _ := json.Marshal(meta)
	if err := os.WriteFile(filepath.Join(skillPath, ".skillshare-meta.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	// Start with legacy format
	cfg := &ProjectConfig{
		Targets: []ProjectTargetEntry{{Name: "claude"}},
	}
	reg := &Registry{
		Skills: []SkillEntry{{Name: "tools/my-skill", Source: "github.com/user/repo"}},
	}

	if err := ReconcileProjectSkills(root, cfg, reg, skillsDir); err != nil {
		t.Fatalf("ReconcileProjectSkills failed: %v", err)
	}

	if len(reg.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(reg.Skills))
	}
	if reg.Skills[0].Name != "my-skill" {
		t.Errorf("expected migrated name 'my-skill', got %q", reg.Skills[0].Name)
	}
	if reg.Skills[0].Group != "tools" {
		t.Errorf("expected migrated group 'tools', got %q", reg.Skills[0].Group)
	}
}
