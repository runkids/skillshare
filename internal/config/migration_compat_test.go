package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// setConfigEnv points SKILLSHARE_CONFIG at a temp file for Load() tests.
// Returns cleanup function that restores the original value.
func setConfigEnv(t *testing.T, path string) {
	t.Helper()
	old := os.Getenv("SKILLSHARE_CONFIG")
	t.Setenv("SKILLSHARE_CONFIG", path)
	_ = old
}

// ---------- Global config migration ----------

func TestGlobalMigration_AllOldFormats(t *testing.T) {
	tests := []struct {
		name       string
		yaml       string
		wantPath   string
		wantMode   string
		wantIncl   []string
		wantExcl   []string
		wantTarget string // which target to check (default: "claude")
	}{
		{
			name:     "path only",
			yaml:     "source: /skills\ntargets:\n  claude:\n    path: ~/.claude/skills\n",
			wantPath: "~/.claude/skills",
			wantMode: "",
		},
		{
			name:     "path + mode",
			yaml:     "source: /skills\ntargets:\n  claude:\n    path: ~/.claude/skills\n    mode: merge\n",
			wantPath: "~/.claude/skills",
			wantMode: "merge",
		},
		{
			name:     "all flat fields",
			yaml:     "source: /skills\ntargets:\n  claude:\n    path: ~/.claude/skills\n    mode: symlink\n    include: [team-*]\n    exclude: [wip-*]\n",
			wantPath: "~/.claude/skills",
			wantMode: "symlink",
			wantIncl: []string{"team-*"},
			wantExcl: []string{"wip-*"},
		},
		{
			name:     "mode only (no path)",
			yaml:     "source: /skills\ntargets:\n  claude:\n    mode: copy\n",
			wantPath: "",
			wantMode: "copy",
		},
		{
			name:     "include/exclude only",
			yaml:     "source: /skills\ntargets:\n  claude:\n    include: [a, b]\n    exclude: [c]\n",
			wantPath: "",
			wantMode: "",
			wantIncl: []string{"a", "b"},
			wantExcl: []string{"c"},
		},
		{
			name:     "empty object",
			yaml:     "source: /skills\ntargets:\n  claude: {}\n",
			wantPath: "",
			wantMode: "",
		},
		{
			name:       "multiple targets mixed",
			yaml:       "source: /skills\ntargets:\n  claude:\n    path: /a\n    mode: merge\n  cursor:\n    path: /b\n    mode: symlink\n",
			wantTarget: "cursor",
			wantPath:   "/b",
			wantMode:   "symlink",
		},
		{
			name:     "new format (already migrated)",
			yaml:     "source: /skills\ntargets:\n  claude:\n    skills:\n      path: ~/.claude/skills\n      mode: merge\n",
			wantPath: "~/.claude/skills",
			wantMode: "merge",
		},
		{
			name:     "mixed format (flat path + skills mode)",
			yaml:     "source: /skills\ntargets:\n  claude:\n    path: /custom\n    skills:\n      mode: symlink\n",
			wantPath: "/custom",
			wantMode: "symlink",
		},
		{
			name:     "mixed format (skills has path, flat has mode)",
			yaml:     "source: /skills\ntargets:\n  claude:\n    mode: copy\n    skills:\n      path: /from-skills\n",
			wantPath: "/from-skills",
			wantMode: "copy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg Config
			if err := yaml.Unmarshal([]byte(tt.yaml), &cfg); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			migrateTargetConfigs(cfg.Targets)

			target := "claude"
			if tt.wantTarget != "" {
				target = tt.wantTarget
			}
			tc, ok := cfg.Targets[target]
			if !ok {
				t.Fatalf("target %q not found", target)
			}

			sc := tc.SkillsConfig()

			// Verify values
			if sc.Path != tt.wantPath {
				t.Errorf("path = %q, want %q", sc.Path, tt.wantPath)
			}
			if sc.Mode != tt.wantMode {
				t.Errorf("mode = %q, want %q", sc.Mode, tt.wantMode)
			}
			if !reflect.DeepEqual(sc.Include, tt.wantIncl) {
				if len(sc.Include) != 0 || len(tt.wantIncl) != 0 {
					t.Errorf("include = %v, want %v", sc.Include, tt.wantIncl)
				}
			}
			if !reflect.DeepEqual(sc.Exclude, tt.wantExcl) {
				if len(sc.Exclude) != 0 || len(tt.wantExcl) != 0 {
					t.Errorf("exclude = %v, want %v", sc.Exclude, tt.wantExcl)
				}
			}

			// Flat fields must be cleared
			if tc.Path != "" {
				t.Errorf("flat Path not cleared: %q", tc.Path)
			}
			if tc.Mode != "" {
				t.Errorf("flat Mode not cleared: %q", tc.Mode)
			}
			if len(tc.Include) > 0 {
				t.Errorf("flat Include not cleared: %v", tc.Include)
			}
			if len(tc.Exclude) > 0 {
				t.Errorf("flat Exclude not cleared: %v", tc.Exclude)
			}
		})
	}
}

func TestGlobalMigration_RoundTrip(t *testing.T) {
	// Old format → migrate → marshal → unmarshal → same values
	oldYAML := `source: /skills
targets:
  claude:
    path: ~/.claude/skills
    mode: merge
    include: [team-*]
    exclude: [wip-*]
  cursor:
    path: ~/.cursor/skills
    mode: symlink
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(oldYAML), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	migrateTargetConfigs(cfg.Targets)

	// Marshal (new format)
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Should contain skills: sub-key, not flat path:
	s := string(data)
	if strings.Contains(s, "    path:") && !strings.Contains(s, "skills:") {
		t.Fatal("marshaled YAML still has flat path without skills")
	}
	if !strings.Contains(s, "skills:") {
		t.Fatal("marshaled YAML missing skills: sub-key")
	}

	// Unmarshal again
	var cfg2 Config
	if err := yaml.Unmarshal(data, &cfg2); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	migrateTargetConfigs(cfg2.Targets)

	// Values should match
	for _, name := range []string{"claude", "cursor"} {
		t1 := cfg.Targets[name]
		t2 := cfg2.Targets[name]
		sc1 := t1.SkillsConfig()
		sc2 := t2.SkillsConfig()
		if sc1.Path != sc2.Path {
			t.Errorf("%s path: %q vs %q", name, sc1.Path, sc2.Path)
		}
		if sc1.Mode != sc2.Mode {
			t.Errorf("%s mode: %q vs %q", name, sc1.Mode, sc2.Mode)
		}
		if !reflect.DeepEqual(sc1.Include, sc2.Include) {
			t.Errorf("%s include: %v vs %v", name, sc1.Include, sc2.Include)
		}
		if !reflect.DeepEqual(sc1.Exclude, sc2.Exclude) {
			t.Errorf("%s exclude: %v vs %v", name, sc1.Exclude, sc2.Exclude)
		}
	}
}

// ---------- Project config migration ----------

func TestProjectMigration_AllOldFormats(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		wantName string
		wantPath string
		wantMode string
		wantIncl []string
		wantExcl []string
		idx      int // which target to check (default: 0)
	}{
		{
			name:     "string form",
			yaml:     "targets:\n  - claude\n",
			wantName: "claude",
			wantPath: "",
			wantMode: "",
		},
		{
			name:     "name only object",
			yaml:     "targets:\n  - name: claude\n",
			wantName: "claude",
		},
		{
			name:     "path only",
			yaml:     "targets:\n  - name: claude\n    path: .claude/skills\n",
			wantName: "claude",
			wantPath: ".claude/skills",
		},
		{
			name:     "path + mode",
			yaml:     "targets:\n  - name: claude\n    path: .claude/skills\n    mode: merge\n",
			wantName: "claude",
			wantPath: ".claude/skills",
			wantMode: "merge",
		},
		{
			name:     "all flat fields",
			yaml:     "targets:\n  - name: claude\n    path: .claude/skills\n    mode: copy\n    include: [a, b]\n    exclude: [c]\n",
			wantName: "claude",
			wantPath: ".claude/skills",
			wantMode: "copy",
			wantIncl: []string{"a", "b"},
			wantExcl: []string{"c"},
		},
		{
			name:     "mode only (no path)",
			yaml:     "targets:\n  - name: claude\n    mode: symlink\n",
			wantName: "claude",
			wantMode: "symlink",
		},
		{
			name:     "mixed: string + object",
			yaml:     "targets:\n  - claude\n  - name: cursor\n    path: .cursor/skills\n    mode: copy\n",
			idx:      1,
			wantName: "cursor",
			wantPath: ".cursor/skills",
			wantMode: "copy",
		},
		{
			name:     "new format (already migrated)",
			yaml:     "targets:\n  - name: claude\n    skills:\n      path: .claude/skills\n      mode: merge\n",
			wantName: "claude",
			wantPath: ".claude/skills",
			wantMode: "merge",
		},
		{
			name:     "mixed format (flat path + skills mode)",
			yaml:     "targets:\n  - name: claude\n    path: .custom/skills\n    skills:\n      mode: symlink\n",
			wantName: "claude",
			wantPath: ".custom/skills",
			wantMode: "symlink",
		},
		{
			name:     "mixed format (skills path + flat mode)",
			yaml:     "targets:\n  - name: claude\n    mode: copy\n    skills:\n      path: .from-skills\n",
			wantName: "claude",
			wantPath: ".from-skills",
			wantMode: "copy",
		},
		{
			name:     "mixed format (flat include + skills exclude)",
			yaml:     "targets:\n  - name: claude\n    include: [a]\n    skills:\n      exclude: [b]\n",
			wantName: "claude",
			wantIncl: []string{"a"},
			wantExcl: []string{"b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			cfgDir := filepath.Join(root, ".skillshare")
			os.MkdirAll(cfgDir, 0755)
			os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(tt.yaml), 0644)

			cfg, err := LoadProject(root)
			if err != nil {
				t.Fatalf("LoadProject: %v", err)
			}

			idx := tt.idx
			if idx >= len(cfg.Targets) {
				t.Fatalf("target index %d out of range (have %d)", idx, len(cfg.Targets))
			}
			entry := cfg.Targets[idx]

			if entry.Name != tt.wantName {
				t.Errorf("name = %q, want %q", entry.Name, tt.wantName)
			}

			sc := entry.SkillsConfig()

			if sc.Path != tt.wantPath {
				t.Errorf("path = %q, want %q", sc.Path, tt.wantPath)
			}
			if sc.Mode != tt.wantMode {
				t.Errorf("mode = %q, want %q", sc.Mode, tt.wantMode)
			}
			if !reflect.DeepEqual(sc.Include, tt.wantIncl) {
				if len(sc.Include) != 0 || len(tt.wantIncl) != 0 {
					t.Errorf("include = %v, want %v", sc.Include, tt.wantIncl)
				}
			}
			if !reflect.DeepEqual(sc.Exclude, tt.wantExcl) {
				if len(sc.Exclude) != 0 || len(tt.wantExcl) != 0 {
					t.Errorf("exclude = %v, want %v", sc.Exclude, tt.wantExcl)
				}
			}

			// Flat fields must be cleared (except string form which has no fields)
			if entry.Path != "" {
				t.Errorf("flat Path not cleared: %q", entry.Path)
			}
			if entry.Mode != "" {
				t.Errorf("flat Mode not cleared: %q", entry.Mode)
			}
			if len(entry.Include) > 0 {
				t.Errorf("flat Include not cleared: %v", entry.Include)
			}
			if len(entry.Exclude) > 0 {
				t.Errorf("flat Exclude not cleared: %v", entry.Exclude)
			}
		})
	}
}

func TestProjectMigration_RoundTrip(t *testing.T) {
	oldYAML := `targets:
  - claude
  - name: cursor
    path: .cursor/skills
    mode: copy
    include: [team-*]
    exclude: [wip-*]
  - name: custom-ide
    path: .custom/skills
`
	root := t.TempDir()
	cfgDir := filepath.Join(root, ".skillshare")
	os.MkdirAll(cfgDir, 0755)
	cfgPath := filepath.Join(cfgDir, "config.yaml")
	os.WriteFile(cfgPath, []byte(oldYAML), 0644)

	// First load (triggers migration + persist)
	cfg, err := LoadProject(root)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}

	// Verify cursor has correct values
	cursor := cfg.Targets[1]
	sc := cursor.SkillsConfig()
	if sc.Path != ".cursor/skills" || sc.Mode != "copy" {
		t.Fatalf("cursor: path=%q mode=%q", sc.Path, sc.Mode)
	}

	// Read persisted file
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read persisted: %v", err)
	}
	s := string(data)

	// Should have skills: sub-key, not flat path: at target level
	if !strings.Contains(s, "skills:") {
		t.Fatal("persisted YAML missing skills: sub-key")
	}

	// Second load from persisted file
	cfg2, err := LoadProject(root)
	if err != nil {
		t.Fatalf("second LoadProject: %v", err)
	}

	// Same values
	cursor2 := cfg2.Targets[1]
	sc2 := cursor2.SkillsConfig()
	if sc.Path != sc2.Path || sc.Mode != sc2.Mode {
		t.Errorf("round-trip mismatch: %+v vs %+v", sc, sc2)
	}
	if !reflect.DeepEqual(sc.Include, sc2.Include) || !reflect.DeepEqual(sc.Exclude, sc2.Exclude) {
		t.Errorf("round-trip filter mismatch")
	}
}

// ---------- Registry kind migration ----------

func TestRegistryMigration_OldFormatPreservesEntries(t *testing.T) {
	dir := t.TempDir()
	// Old format: no kind field
	os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte(`skills:
  - name: my-skill
    source: github.com/user/repo
    tracked: true
  - name: another
    source: local
    group: frontend
`), 0644)

	reg, err := LoadRegistry(dir)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	if len(reg.Skills) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(reg.Skills))
	}

	// All should default to "skill"
	for _, e := range reg.Skills {
		if e.EffectiveKind() != "skill" {
			t.Errorf("%s: EffectiveKind() = %q, want skill", e.Name, e.EffectiveKind())
		}
	}

	// Verify other fields preserved
	if reg.Skills[0].Tracked != true {
		t.Error("tracked flag lost")
	}
	if reg.Skills[1].Group != "frontend" {
		t.Errorf("group = %q, want frontend", reg.Skills[1].Group)
	}
}

// ==========================================================================
// Category 1: Global Load() full path — rewrite, ~ expansion, built-in backfill
// ==========================================================================

func TestGlobalLoad_OldFormat_PersistsNewFormat(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "skills")
	os.MkdirAll(sourceDir, 0755)
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte("source: "+sourceDir+"\ntargets:\n  claude:\n    path: "+sourceDir+"\n    mode: merge\n"), 0644)

	setConfigEnv(t, cfgPath)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// SkillsConfig should have the values
	ct := cfg.Targets["claude"]
	sc := ct.SkillsConfig()
	if sc.Path == "" {
		t.Fatal("SkillsConfig().Path is empty after Load()")
	}
	if sc.Mode != "merge" {
		t.Fatalf("mode = %q", sc.Mode)
	}

	// Persisted file should contain skills: sub-key
	data, _ := os.ReadFile(cfgPath)
	s := string(data)
	if !strings.Contains(s, "skills:") {
		t.Fatal("persisted config missing skills: sub-key")
	}
	// Should NOT have flat path/mode at target level (outside skills:)
	assertNoFlatTargetKeys(t, s)
}

func TestGlobalLoad_BuiltinTarget_NoPath_BackfillsDefault(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "skills")
	os.MkdirAll(sourceDir, 0755)
	cfgPath := filepath.Join(dir, "config.yaml")
	// claude target with mode but no path
	os.WriteFile(cfgPath, []byte("source: "+sourceDir+"\ntargets:\n  claude:\n    skills:\n      mode: merge\n"), 0644)

	setConfigEnv(t, cfgPath)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	ct := cfg.Targets["claude"]
	sc := ct.SkillsConfig()
	if sc.Path == "" {
		t.Fatal("built-in target should have path backfilled from defaults")
	}
	// Should contain "claude" in the path (from targets.yaml)
	if !strings.Contains(sc.Path, "claude") {
		t.Fatalf("backfilled path doesn't look like claude default: %q", sc.Path)
	}
}

func TestGlobalLoad_TildeExpansion(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "skills")
	os.MkdirAll(sourceDir, 0755)
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte("source: "+sourceDir+"\ntargets:\n  claude:\n    path: ~/.claude/skills\n"), 0644)

	setConfigEnv(t, cfgPath)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	ct := cfg.Targets["claude"]
	sc := ct.SkillsConfig()
	if strings.HasPrefix(sc.Path, "~") {
		t.Fatalf("~ not expanded in skills path: %q", sc.Path)
	}
}

// ==========================================================================
// Category 2: Project LoadProject() + ResolveProjectTargets()
// ==========================================================================

func TestProjectMigration_StringTarget_Resolvable(t *testing.T) {
	root := t.TempDir()
	cfgDir := filepath.Join(root, ".skillshare")
	os.MkdirAll(cfgDir, 0755)
	os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte("targets:\n  - claude\n"), 0644)

	cfg, err := LoadProject(root)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}

	targets, err := ResolveProjectTargets(root, cfg)
	if err != nil {
		t.Fatalf("ResolveProjectTargets: %v", err)
	}

	tc, ok := targets["claude"]
	if !ok {
		t.Fatal("claude target not resolved")
	}
	sc := tc.SkillsConfig()
	if sc.Path == "" {
		t.Fatal("resolved claude target has empty path")
	}
	if !filepath.IsAbs(sc.Path) {
		t.Fatalf("resolved path should be absolute, got %q", sc.Path)
	}
}

func TestProjectMigration_OldFlatFormat_Resolvable(t *testing.T) {
	root := t.TempDir()
	cfgDir := filepath.Join(root, ".skillshare")
	os.MkdirAll(cfgDir, 0755)
	os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte("targets:\n  - name: claude\n    path: .claude/skills\n    mode: copy\n    include: [team-*]\n"), 0644)

	cfg, err := LoadProject(root)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}

	targets, err := ResolveProjectTargets(root, cfg)
	if err != nil {
		t.Fatalf("ResolveProjectTargets: %v", err)
	}

	tc := targets["claude"]
	sc := tc.SkillsConfig()
	if !filepath.IsAbs(sc.Path) {
		t.Fatalf("resolved path should be absolute, got %q", sc.Path)
	}
	if sc.Mode != "copy" {
		t.Fatalf("mode = %q, want copy", sc.Mode)
	}
	if !reflect.DeepEqual(sc.Include, []string{"team-*"}) {
		t.Fatalf("include = %v", sc.Include)
	}
}

func TestProjectMigration_AliasTarget_Resolvable(t *testing.T) {
	root := t.TempDir()
	cfgDir := filepath.Join(root, ".skillshare")
	os.MkdirAll(cfgDir, 0755)
	// claude-code is an alias for claude
	os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte("targets:\n  - claude-code\n"), 0644)

	cfg, err := LoadProject(root)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}

	targets, err := ResolveProjectTargets(root, cfg)
	if err != nil {
		t.Fatalf("ResolveProjectTargets: %v", err)
	}

	// claude-code is an alias for claude — should resolve under the alias name
	tc, ok := targets["claude-code"]
	if !ok {
		t.Fatal("alias target 'claude-code' not resolved")
	}
	sc := tc.SkillsConfig()
	if sc.Path == "" {
		t.Fatal("alias target should have a resolved path")
	}
	if !filepath.IsAbs(sc.Path) {
		t.Fatalf("alias target path should be absolute, got %q", sc.Path)
	}
}

func TestProjectMigration_CustomTarget_WithPath_Resolvable(t *testing.T) {
	root := t.TempDir()
	cfgDir := filepath.Join(root, ".skillshare")
	os.MkdirAll(cfgDir, 0755)
	os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte("targets:\n  - name: my-custom-ide\n    path: .my-ide/skills\n"), 0644)

	cfg, err := LoadProject(root)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}

	targets, err := ResolveProjectTargets(root, cfg)
	if err != nil {
		t.Fatalf("ResolveProjectTargets: %v", err)
	}

	tc := targets["my-custom-ide"]
	sc := tc.SkillsConfig()
	if !filepath.IsAbs(sc.Path) {
		t.Fatalf("custom target path should be absolute, got %q", sc.Path)
	}
	wantSuffix := filepath.FromSlash(".my-ide/skills")
	if !strings.HasSuffix(sc.Path, wantSuffix) {
		t.Fatalf("path should end with %s, got %q", wantSuffix, sc.Path)
	}
}

// ==========================================================================
// Category 3: Round-trip — assert NO flat keys in persisted output
// ==========================================================================

func TestGlobalMigration_RoundTrip_NoFlatKeys(t *testing.T) {
	oldYAML := `source: /skills
targets:
  claude:
    path: /claude/skills
    mode: merge
    include: [team-*]
    exclude: [wip-*]
`
	var cfg Config
	yaml.Unmarshal([]byte(oldYAML), &cfg)
	migrateTargetConfigs(cfg.Targets)

	data, _ := yaml.Marshal(&cfg)
	s := string(data)

	if !strings.Contains(s, "skills:") {
		t.Fatal("missing skills: sub-key")
	}
	assertNoFlatTargetKeys(t, s)
}

func TestProjectMigration_RoundTrip_NoFlatKeys(t *testing.T) {
	root := t.TempDir()
	cfgDir := filepath.Join(root, ".skillshare")
	os.MkdirAll(cfgDir, 0755)
	cfgPath := filepath.Join(cfgDir, "config.yaml")
	os.WriteFile(cfgPath, []byte(`targets:
  - name: claude
    path: .claude/skills
    mode: copy
    include: [a]
    exclude: [b]
  - cursor
`), 0644)

	// Load triggers migration + persist
	LoadProject(root)

	data, _ := os.ReadFile(cfgPath)
	s := string(data)

	if !strings.Contains(s, "skills:") {
		t.Fatal("persisted YAML missing skills: sub-key")
	}

	// Parse the persisted YAML and check no target-level flat keys remain
	var raw struct {
		Targets []yaml.Node `yaml:"targets"`
	}
	yaml.Unmarshal(data, &raw)
	for _, node := range raw.Targets {
		if node.Kind != yaml.MappingNode {
			continue
		}
		for i := 0; i < len(node.Content)-1; i += 2 {
			key := node.Content[i].Value
			if key == "path" || key == "mode" || key == "include" || key == "exclude" {
				t.Fatalf("persisted YAML still has flat key %q at target level", key)
			}
		}
	}
}

// ==========================================================================
// Category 4: Mixed-format merge edge cases
// ==========================================================================

func TestMixedFormat_IncludeExclude_Merge(t *testing.T) {
	// Flat include + skills exclude → both should be present after merge
	targets := map[string]TargetConfig{
		"test": {
			Include: []string{"from-flat"},
			Skills:  &ResourceTargetConfig{Exclude: []string{"from-skills"}},
		},
	}
	migrateTargetConfigs(targets)

	tc := targets["test"]
	sc := tc.SkillsConfig()
	if !reflect.DeepEqual(sc.Include, []string{"from-flat"}) {
		t.Errorf("include = %v, want [from-flat]", sc.Include)
	}
	if !reflect.DeepEqual(sc.Exclude, []string{"from-skills"}) {
		t.Errorf("exclude = %v, want [from-skills]", sc.Exclude)
	}
	if len(tc.Include) > 0 || len(tc.Exclude) > 0 {
		t.Error("flat fields should be cleared")
	}
}

func TestMixedFormat_SkillsFieldsTakePrecedence(t *testing.T) {
	// Both flat and skills have values → skills wins (no overwrite)
	targets := map[string]TargetConfig{
		"test": {
			Path:    "/old",
			Mode:    "symlink",
			Include: []string{"old-*"},
			Exclude: []string{"old-x"},
			Skills: &ResourceTargetConfig{
				Path:    "/new",
				Mode:    "copy",
				Include: []string{"new-*"},
				Exclude: []string{"new-x"},
			},
		},
	}
	migrateTargetConfigs(targets)

	tt := targets["test"]
	sc := tt.SkillsConfig()
	if sc.Path != "/new" {
		t.Errorf("path = %q, want /new (skills should win)", sc.Path)
	}
	if sc.Mode != "copy" {
		t.Errorf("mode = %q, want copy (skills should win)", sc.Mode)
	}
	if !reflect.DeepEqual(sc.Include, []string{"new-*"}) {
		t.Errorf("include = %v (skills should win)", sc.Include)
	}
	if !reflect.DeepEqual(sc.Exclude, []string{"new-x"}) {
		t.Errorf("exclude = %v (skills should win)", sc.Exclude)
	}
}

func TestMixedFormat_ProjectEntry_PartialMerge(t *testing.T) {
	root := t.TempDir()
	cfgDir := filepath.Join(root, ".skillshare")
	os.MkdirAll(cfgDir, 0755)
	// skills has mode, flat has path and include
	os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(`targets:
  - name: claude
    path: .custom/path
    include: [team-*]
    skills:
      mode: copy
`), 0644)

	cfg, _ := LoadProject(root)
	entry := cfg.Targets[0]
	sc := entry.SkillsConfig()

	if sc.Path != ".custom/path" {
		t.Errorf("path = %q, want .custom/path (merged from flat)", sc.Path)
	}
	if sc.Mode != "copy" {
		t.Errorf("mode = %q, want copy (from skills)", sc.Mode)
	}
	if !reflect.DeepEqual(sc.Include, []string{"team-*"}) {
		t.Errorf("include = %v, want [team-*] (merged from flat)", sc.Include)
	}
}

// assertNoFlatTargetKeys checks that marshaled config YAML does NOT contain
// flat path/mode/include/exclude at the target level (outside skills: block).
func assertNoFlatTargetKeys(t *testing.T, yamlStr string) {
	t.Helper()
	// Parse and inspect the targets section
	var raw struct {
		Targets map[string]yaml.Node `yaml:"targets"`
	}
	if err := yaml.Unmarshal([]byte(yamlStr), &raw); err != nil {
		// Might be project format (array), try that
		var rawArr struct {
			Targets []yaml.Node `yaml:"targets"`
		}
		if err2 := yaml.Unmarshal([]byte(yamlStr), &rawArr); err2 != nil {
			return // can't parse, skip
		}
		for _, node := range rawArr.Targets {
			checkNodeNoFlatKeys(t, node)
		}
		return
	}
	for _, node := range raw.Targets {
		checkNodeNoFlatKeys(t, node)
	}
}

func checkNodeNoFlatKeys(t *testing.T, node yaml.Node) {
	t.Helper()
	if node.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		key := node.Content[i].Value
		if key == "path" || key == "mode" || key == "include" || key == "exclude" {
			t.Errorf("marshaled YAML has flat key %q at target level (should be under skills:)", key)
		}
	}
}
