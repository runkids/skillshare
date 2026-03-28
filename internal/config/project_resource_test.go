package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectTarget_NewFormat(t *testing.T) {
	root := t.TempDir()
	cfgDir := filepath.Join(root, ".skillshare")
	os.MkdirAll(cfgDir, 0755)
	os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(`targets:
  - name: claude
    skills:
      mode: merge
    agents:
      path: .claude/agents
`), 0644)

	cfg, err := LoadProject(root)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	if len(cfg.Targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(cfg.Targets))
	}
	entry := cfg.Targets[0]
	if entry.Name != "claude" {
		t.Fatalf("name = %q", entry.Name)
	}
	if entry.Skills == nil || entry.Skills.Mode != "merge" {
		t.Fatalf("skills.mode = %v", entry.Skills)
	}
	if entry.Agents == nil || entry.Agents.Path != ".claude/agents" {
		t.Fatalf("agents = %v", entry.Agents)
	}
}

func TestProjectTarget_OldFormat_Migrated(t *testing.T) {
	root := t.TempDir()
	cfgDir := filepath.Join(root, ".skillshare")
	os.MkdirAll(cfgDir, 0755)
	os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(`targets:
  - name: claude
    path: .claude/commands
    mode: merge
`), 0644)

	cfg, err := LoadProject(root)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	entry := cfg.Targets[0]
	if entry.Skills == nil {
		t.Fatal("old flat fields should be migrated to skills sub-key")
	}
	if entry.Skills.Path != ".claude/commands" {
		t.Fatalf("skills.path = %q", entry.Skills.Path)
	}
	if entry.Skills.Mode != "merge" {
		t.Fatalf("skills.mode = %q", entry.Skills.Mode)
	}
	// Flat fields should be cleared
	if entry.Path != "" {
		t.Fatalf("flat path should be empty, got %q", entry.Path)
	}
}

func TestProjectTarget_StringFormat(t *testing.T) {
	root := t.TempDir()
	cfgDir := filepath.Join(root, ".skillshare")
	os.MkdirAll(cfgDir, 0755)
	os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(`targets:
  - claude
`), 0644)

	cfg, err := LoadProject(root)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	entry := cfg.Targets[0]
	if entry.Name != "claude" {
		t.Fatalf("name = %q", entry.Name)
	}
	// String format = all defaults, no sub-keys
	if entry.Skills != nil && entry.Skills.Path != "" {
		t.Fatal("string format should have no explicit skills config")
	}
}

func TestProjectTarget_MarshalYAML_NewFormat(t *testing.T) {
	entry := ProjectTargetEntry{
		Name:   "claude",
		Skills: &ResourceTargetConfig{Mode: "merge"},
		Agents: &ResourceTargetConfig{Path: ".claude/agents"},
	}
	result, err := entry.MarshalYAML()
	if err != nil {
		t.Fatalf("MarshalYAML: %v", err)
	}
	obj, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if obj["name"] != "claude" {
		t.Fatalf("name = %v", obj["name"])
	}
	if obj["skills"] == nil {
		t.Fatal("expected skills key in marshaled output")
	}
	if obj["agents"] == nil {
		t.Fatal("expected agents key in marshaled output")
	}
	// Should NOT have flat path/mode keys
	if _, has := obj["path"]; has {
		t.Fatal("should not have flat path key")
	}
}

func TestProjectTarget_MarshalYAML_StringFormat(t *testing.T) {
	entry := ProjectTargetEntry{Name: "claude"}
	result, err := entry.MarshalYAML()
	if err != nil {
		t.Fatalf("MarshalYAML: %v", err)
	}
	str, ok := result.(string)
	if !ok {
		t.Fatalf("expected string, got %T", result)
	}
	if str != "claude" {
		t.Fatalf("expected 'claude', got %q", str)
	}
}

func TestProjectTarget_SkillsConfig_Accessor(t *testing.T) {
	// New format with skills sub-key
	entry := ProjectTargetEntry{
		Name:   "claude",
		Skills: &ResourceTargetConfig{Path: ".claude/commands", Mode: "merge"},
	}
	sc := entry.SkillsConfig()
	if sc.Path != ".claude/commands" || sc.Mode != "merge" {
		t.Fatalf("SkillsConfig() = %+v", sc)
	}

	// Legacy format without skills sub-key (flat fields)
	entry2 := ProjectTargetEntry{Name: "claude", Path: ".old/path", Mode: "symlink"}
	sc2 := entry2.SkillsConfig()
	if sc2.Path != ".old/path" || sc2.Mode != "symlink" {
		t.Fatalf("SkillsConfig() legacy = %+v", sc2)
	}

	// Empty entry
	entry3 := ProjectTargetEntry{Name: "claude"}
	sc3 := entry3.SkillsConfig()
	if sc3.Path != "" || sc3.Mode != "" {
		t.Fatalf("SkillsConfig() empty = %+v", sc3)
	}
}

func TestProjectTarget_AgentsConfig_Accessor(t *testing.T) {
	entry := ProjectTargetEntry{
		Name:   "claude",
		Agents: &ResourceTargetConfig{Path: ".claude/agents", Exclude: []string{"draft-*"}},
	}
	ac := entry.AgentsConfig()
	if ac.Path != ".claude/agents" {
		t.Fatalf("AgentsConfig().Path = %q", ac.Path)
	}
	if len(ac.Exclude) != 1 || ac.Exclude[0] != "draft-*" {
		t.Fatalf("AgentsConfig().Exclude = %v", ac.Exclude)
	}

	// No agents config
	entry2 := ProjectTargetEntry{Name: "claude"}
	ac2 := entry2.AgentsConfig()
	if ac2.Path != "" {
		t.Fatalf("AgentsConfig() should be empty, got %+v", ac2)
	}
}

func TestProjectTarget_MarshalYAML_OldFlatFields_WrittenAsSkills(t *testing.T) {
	// If somehow flat fields are still set (pre-migration), MarshalYAML should
	// still write them as flat fields for backward compat (migration happens on unmarshal)
	entry := ProjectTargetEntry{
		Name: "claude",
		Path: ".claude/commands",
		Mode: "merge",
	}
	result, err := entry.MarshalYAML()
	if err != nil {
		t.Fatalf("MarshalYAML: %v", err)
	}
	obj, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	// Flat fields with no Skills sub-key → write as flat fields (backward compat)
	if obj["name"] != "claude" {
		t.Fatalf("name = %v", obj["name"])
	}
}

func TestProjectTarget_MarshalYAML_SkillsOnlyMode(t *testing.T) {
	// Skills sub-key with only mode (no path) should still produce object format
	entry := ProjectTargetEntry{
		Name:   "cursor",
		Skills: &ResourceTargetConfig{Mode: "symlink"},
	}
	result, err := entry.MarshalYAML()
	if err != nil {
		t.Fatalf("MarshalYAML: %v", err)
	}
	obj, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T (value=%v)", result, result)
	}
	if obj["name"] != "cursor" {
		t.Fatalf("name = %v", obj["name"])
	}
	if obj["skills"] == nil {
		t.Fatal("expected skills key")
	}
}

func TestProjectTarget_MixedFormat_MergesIntoSkills(t *testing.T) {
	root := t.TempDir()
	cfgDir := filepath.Join(root, ".skillshare")
	os.MkdirAll(cfgDir, 0755)
	// Mixed: flat path + skills block
	os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(`targets:
  - name: claude
    path: .custom/skills
    skills:
      mode: symlink
`), 0644)

	cfg, err := LoadProject(root)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	entry := cfg.Targets[0]

	// Flat path should be merged into skills (skills.path was empty)
	if entry.Skills == nil {
		t.Fatal("expected skills sub-key")
	}
	if entry.Skills.Path != ".custom/skills" {
		t.Fatalf("skills.path should be merged from flat, got %q", entry.Skills.Path)
	}
	if entry.Skills.Mode != "symlink" {
		t.Fatalf("skills.mode should be preserved, got %q", entry.Skills.Mode)
	}
	// Flat fields should be cleared
	if entry.Path != "" {
		t.Fatalf("flat path should be empty, got %q", entry.Path)
	}
}
