package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestTargetConfig_NewFormat_SkillsAndAgents(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte(`source: /skills
targets:
  claude:
    skills:
      path: /claude/commands
      mode: merge
    agents:
      path: /claude/agents
      exclude: [draft-*]
`), 0644)

	data, _ := os.ReadFile(cfgPath)
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	migrateTargetConfigs(cfg.Targets)
	tc := cfg.Targets["claude"]

	if tc.Skills == nil {
		t.Fatal("expected skills sub-key")
	}
	if tc.Skills.Path != "/claude/commands" {
		t.Fatalf("skills.path = %q", tc.Skills.Path)
	}
	if tc.Skills.Mode != "merge" {
		t.Fatalf("skills.mode = %q", tc.Skills.Mode)
	}
	if tc.Agents == nil {
		t.Fatal("expected agents sub-key")
	}
	if tc.Agents.Path != "/claude/agents" {
		t.Fatalf("agents.path = %q", tc.Agents.Path)
	}
	if len(tc.Agents.Exclude) != 1 || tc.Agents.Exclude[0] != "draft-*" {
		t.Fatalf("agents.exclude = %v", tc.Agents.Exclude)
	}
}

func TestTargetConfig_OldFormat_MigratedToSkills(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte(`source: /skills
targets:
  claude:
    path: /claude/commands
    mode: merge
    include: [my-skill]
`), 0644)

	data, _ := os.ReadFile(cfgPath)
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	migrateTargetConfigs(cfg.Targets)
	tc := cfg.Targets["claude"]

	if tc.Skills == nil {
		t.Fatal("expected skills sub-key from migration")
	}
	if tc.Skills.Path != "/claude/commands" {
		t.Fatalf("skills.path = %q", tc.Skills.Path)
	}
	if tc.Skills.Mode != "merge" {
		t.Fatalf("skills.mode = %q", tc.Skills.Mode)
	}
	if len(tc.Skills.Include) != 1 || tc.Skills.Include[0] != "my-skill" {
		t.Fatalf("skills.include = %v", tc.Skills.Include)
	}
	if tc.Path != "" {
		t.Fatalf("flat path should be empty after migration, got %q", tc.Path)
	}
}

func TestTargetConfig_EmptyObject(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte(`source: /skills
targets:
  claude: {}
`), 0644)

	data, _ := os.ReadFile(cfgPath)
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	migrateTargetConfigs(cfg.Targets)
	tc := cfg.Targets["claude"]
	if tc.Skills != nil && tc.Skills.Path != "" {
		t.Fatalf("empty target should have nil or empty skills, got path=%q", tc.Skills.Path)
	}
}

func TestTargetConfig_SkillsConfig_Accessor(t *testing.T) {
	// New format with skills sub-key
	tc := TargetConfig{
		Skills: &ResourceTargetConfig{Path: "/new/path", Mode: "symlink"},
	}
	sc := tc.SkillsConfig()
	if sc.Path != "/new/path" || sc.Mode != "symlink" {
		t.Fatalf("SkillsConfig() = %+v", sc)
	}

	// Legacy format without skills sub-key (pre-migration, or SkillsConfig called before migration)
	tc2 := TargetConfig{Path: "/old/path", Mode: "merge"}
	sc2 := tc2.SkillsConfig()
	if sc2.Path != "/old/path" || sc2.Mode != "merge" {
		t.Fatalf("SkillsConfig() legacy = %+v", sc2)
	}
}

func TestTargetConfig_AgentsConfig_Accessor(t *testing.T) {
	tc := TargetConfig{
		Agents: &ResourceTargetConfig{Path: "/agents", Exclude: []string{"draft-*"}},
	}
	ac := tc.AgentsConfig()
	if ac.Path != "/agents" {
		t.Fatalf("AgentsConfig().Path = %q", ac.Path)
	}

	// No agents config
	tc2 := TargetConfig{}
	ac2 := tc2.AgentsConfig()
	if ac2.Path != "" {
		t.Fatalf("AgentsConfig() should be empty, got %+v", ac2)
	}
}

func TestMigrateTargetConfigs_MixedFormat(t *testing.T) {
	// Target has BOTH flat path and skills sub-key — flat path should merge into skills
	targets := map[string]TargetConfig{
		"claude": {
			Path:   "/custom/path",
			Skills: &ResourceTargetConfig{Mode: "symlink"},
		},
	}
	migrateTargetConfigs(targets)

	tc := targets["claude"]
	if tc.Path != "" {
		t.Fatalf("flat Path should be cleared, got %q", tc.Path)
	}
	if tc.Skills.Path != "/custom/path" {
		t.Fatalf("skills.Path should be merged from flat, got %q", tc.Skills.Path)
	}
	if tc.Skills.Mode != "symlink" {
		t.Fatalf("skills.Mode should be preserved, got %q", tc.Skills.Mode)
	}
}

func TestMigrateTargetConfigs_MixedFormat_NoOverwrite(t *testing.T) {
	// Skills already has path — flat path should NOT overwrite
	targets := map[string]TargetConfig{
		"claude": {
			Path:   "/old/path",
			Skills: &ResourceTargetConfig{Path: "/new/path", Mode: "merge"},
		},
	}
	migrateTargetConfigs(targets)

	tc := targets["claude"]
	if tc.Skills.Path != "/new/path" {
		t.Fatalf("skills.Path should keep existing value, got %q", tc.Skills.Path)
	}
}
