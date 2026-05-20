package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Issue #157: skills: section in config.yaml must survive LoadProject round-trip.
func TestLoadProject_PreservesSkills(t *testing.T) {
	root := t.TempDir()
	skillshareDir := filepath.Join(root, ".skillshare")
	if err := os.MkdirAll(skillshareDir, 0755); err != nil {
		t.Fatal(err)
	}

	configYAML := `targets:
  - claude
skills:
  - name: pdf
    source: anthropic/skills/pdf
  - name: next-best-practices
    source: vercel-labs/next-skills/next-best-practices
    group: frontend
audit:
  block_threshold: CRITICAL
`
	configPath := filepath.Join(skillshareDir, "config.yaml")
	os.WriteFile(configPath, []byte(configYAML), 0644)

	cfg, err := LoadProject(root)
	if err != nil {
		t.Fatalf("LoadProject failed: %v", err)
	}

	if len(cfg.Skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(cfg.Skills))
	}
	if cfg.Skills[0].Name != "pdf" {
		t.Errorf("skills[0].Name = %q, want %q", cfg.Skills[0].Name, "pdf")
	}
	if cfg.Skills[0].Source != "anthropic/skills/pdf" {
		t.Errorf("skills[0].Source = %q, want %q", cfg.Skills[0].Source, "anthropic/skills/pdf")
	}
	if cfg.Skills[1].Group != "frontend" {
		t.Errorf("skills[1].Group = %q, want %q", cfg.Skills[1].Group, "frontend")
	}

	// Verify config.yaml on disk still contains skills: section
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "skills:") {
		t.Error("config.yaml on disk should still contain 'skills:' after LoadProject")
	}
}

// Issue #157: skills survive Save → LoadProject round-trip.
func TestProjectConfig_SkillsRoundTrip(t *testing.T) {
	root := t.TempDir()
	skillshareDir := filepath.Join(root, ".skillshare")
	if err := os.MkdirAll(skillshareDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &ProjectConfig{
		Targets: []ProjectTargetEntry{{Name: "claude"}},
		Skills: []SkillEntry{
			{Name: "pdf", Source: "anthropic/skills/pdf"},
			{Name: "my-tool", Source: "org/repo/my-tool", Tracked: true, Group: "tools"},
		},
	}

	if err := cfg.Save(root); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := LoadProject(root)
	if err != nil {
		t.Fatalf("LoadProject after Save failed: %v", err)
	}

	if len(loaded.Skills) != 2 {
		t.Fatalf("expected 2 skills after round-trip, got %d", len(loaded.Skills))
	}
	if loaded.Skills[0].Name != "pdf" || loaded.Skills[0].Source != "anthropic/skills/pdf" {
		t.Errorf("skills[0] mismatch: %+v", loaded.Skills[0])
	}
	if loaded.Skills[1].Tracked != true || loaded.Skills[1].Group != "tools" {
		t.Errorf("skills[1] mismatch: %+v", loaded.Skills[1])
	}
}

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

func TestProjectEffectiveSkillsSource_Default(t *testing.T) {
	cfg := &ProjectConfig{}
	root := "/project"
	want := filepath.Join(root, ".skillshare", "skills")
	if got := cfg.EffectiveSkillsSource(root); got != want {
		t.Errorf("EffectiveSkillsSource() = %q, want %q", got, want)
	}
}

func TestProjectEffectiveSkillsSource_Custom(t *testing.T) {
	cfg := &ProjectConfig{Sources: ProjectSources{Skills: "./docs/skills"}}
	root := "/project"
	want := filepath.Join(root, "docs", "skills")
	if got := cfg.EffectiveSkillsSource(root); got != want {
		t.Errorf("EffectiveSkillsSource() = %q, want %q", got, want)
	}
}

func TestProjectEffectiveAgentsSource_Default(t *testing.T) {
	cfg := &ProjectConfig{}
	root := "/project"
	want := filepath.Join(root, ".skillshare", "agents")
	if got := cfg.EffectiveAgentsSource(root); got != want {
		t.Errorf("EffectiveAgentsSource() = %q, want %q", got, want)
	}
}

func TestProjectEffectiveAgentsSource_Custom(t *testing.T) {
	cfg := &ProjectConfig{Sources: ProjectSources{Agents: "ai/agents"}}
	root := "/project"
	want := filepath.Join(root, "ai", "agents")
	if got := cfg.EffectiveAgentsSource(root); got != want {
		t.Errorf("EffectiveAgentsSource() = %q, want %q", got, want)
	}
}

func TestProjectEffectiveExtrasSource_Default(t *testing.T) {
	cfg := &ProjectConfig{}
	root := "/project"
	want := filepath.Join(root, ".skillshare", "extras")
	if got := cfg.EffectiveExtrasSource(root); got != want {
		t.Errorf("EffectiveExtrasSource() = %q, want %q", got, want)
	}
}

func TestProjectEffectiveExtrasSource_Custom(t *testing.T) {
	cfg := &ProjectConfig{Sources: ProjectSources{Extras: "./docs/extras"}}
	root := "/project"
	want := filepath.Join(root, "docs", "extras")
	if got := cfg.EffectiveExtrasSource(root); got != want {
		t.Errorf("EffectiveExtrasSource() = %q, want %q", got, want)
	}
}

func TestProjectEffectiveSkillsSource_Absolute(t *testing.T) {
	cfg := &ProjectConfig{Sources: ProjectSources{Skills: "/opt/shared/skills"}}
	got := cfg.EffectiveSkillsSource("/project")
	if got != "/opt/shared/skills" {
		t.Errorf("absolute path should pass through, got %q", got)
	}
}

func TestProjectGitignoreTarget_UnderSkillshare(t *testing.T) {
	root := "/project"
	source := filepath.Join(root, ".skillshare", "skills")
	dir, prefix := ProjectGitignoreTarget(root, source)
	if dir != filepath.Join(root, ".skillshare") {
		t.Errorf("dir = %q, want .skillshare dir", dir)
	}
	if prefix != "skills" {
		t.Errorf("prefix = %q, want %q", prefix, "skills")
	}
}

func TestProjectGitignoreTarget_InsideProject(t *testing.T) {
	root := "/project"
	source := filepath.Join(root, "docs", "skills")
	dir, prefix := ProjectGitignoreTarget(root, source)
	if dir != root {
		t.Errorf("dir = %q, want %q", dir, root)
	}
	if prefix != "docs/skills" {
		t.Errorf("prefix = %q, want %q", prefix, "docs/skills")
	}
}

func TestProjectGitignoreTarget_OutsideProject(t *testing.T) {
	root := "/project/app"
	source := "/shared/skills"
	dir, prefix := ProjectGitignoreTarget(root, source)
	if dir != "" {
		t.Errorf("dir should be empty for external source, got %q", dir)
	}
	if prefix != "" {
		t.Errorf("prefix should be empty for external source, got %q", prefix)
	}
}

func TestLoadProject_Sources(t *testing.T) {
	root := t.TempDir()
	skillshareDir := filepath.Join(root, ".skillshare")
	os.MkdirAll(skillshareDir, 0755)

	os.WriteFile(filepath.Join(skillshareDir, "config.yaml"), []byte(`sources:
  skills: ./docs/skills
  agents: ./ai/agents
targets:
  - claude
`), 0644)

	cfg, err := LoadProject(root)
	if err != nil {
		t.Fatalf("LoadProject failed: %v", err)
	}
	if cfg.Sources.Skills != "./docs/skills" {
		t.Errorf("Sources.Skills = %q, want %q", cfg.Sources.Skills, "./docs/skills")
	}
	if cfg.Sources.Agents != "./ai/agents" {
		t.Errorf("Sources.Agents = %q, want %q", cfg.Sources.Agents, "./ai/agents")
	}
	if cfg.Sources.Extras != "" {
		t.Errorf("Sources.Extras should be empty, got %q", cfg.Sources.Extras)
	}

	wantSkills := filepath.Join(root, "docs", "skills")
	if got := cfg.EffectiveSkillsSource(root); got != wantSkills {
		t.Errorf("EffectiveSkillsSource() = %q, want %q", got, wantSkills)
	}

	wantExtras := filepath.Join(root, ".skillshare", "extras")
	if got := cfg.EffectiveExtrasSource(root); got != wantExtras {
		t.Errorf("EffectiveExtrasSource() = %q, want %q (default)", got, wantExtras)
	}
}
