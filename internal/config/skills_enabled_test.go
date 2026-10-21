package config

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSkillsEnabled_DefaultsToTrue(t *testing.T) {
	tc := TargetConfig{Skills: &ResourceTargetConfig{Path: "/x"}}
	if !tc.SkillsConfig().IsEnabled() {
		t.Fatal("absent enabled should mean true")
	}
	legacy := TargetConfig{Path: "/x"}
	if !legacy.SkillsConfig().IsEnabled() {
		t.Fatal("legacy flat target should be enabled")
	}
}

func TestSkillsEnabled_FalseParsedAndKeepsSettings(t *testing.T) {
	var cfg Config
	err := yaml.Unmarshal([]byte(`targets:
  my-tool:
    skills: {path: /my/skills, mode: copy, include: [team-*], enabled: false}
`), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	tc := cfg.Targets["my-tool"]
	sc := tc.SkillsConfig()
	if sc.IsEnabled() {
		t.Fatal("expected skills disabled")
	}
	if sc.Path != "/my/skills" || sc.Mode != "copy" || len(sc.Include) != 1 {
		t.Fatalf("settings lost: %+v", sc)
	}
}

func TestSkillsEnabled_OnlyEnabledFalseIsNotEmpty(t *testing.T) {
	off := false
	if (ResourceTargetConfig{Enabled: &off}).IsEmpty() {
		t.Fatal("enabled:false must not count as empty")
	}
}

func TestSkillsEnabled_ProjectRoundTrip(t *testing.T) {
	root := t.TempDir()
	off := false
	cfg := &ProjectConfig{Targets: []ProjectTargetEntry{
		{Name: "gemini", Skills: &ResourceTargetConfig{Enabled: &off}},
	}}
	if err := cfg.Save(root); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadProject(root)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Targets[0].SkillsConfig().IsEnabled() {
		data, _ := os.ReadFile(filepath.Join(root, ".skillshare", "config.yaml"))
		t.Fatalf("enabled:false lost on round trip:\n%s", data)
	}
	resolved, err := ResolveProjectTargets(root, loaded)
	if err != nil {
		t.Fatal(err)
	}
	if gemini := resolved["gemini"]; gemini.SkillsConfig().IsEnabled() {
		t.Fatal("ResolveProjectTargets dropped enabled:false")
	}
}

func TestValidateConfig_RejectsAgentsEnabled(t *testing.T) {
	src := t.TempDir()
	off := false
	cfg := &Config{Source: src, Targets: map[string]TargetConfig{
		"claude": {Agents: &ResourceTargetConfig{Enabled: &off}},
	}}
	_, err := ValidateConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "agents.enabled") {
		t.Fatalf("expected agents.enabled error, got %v", err)
	}
}

func TestValidateProjectConfig_RejectsAgentsEnabled(t *testing.T) {
	root := t.TempDir()
	off := false
	cfg := &ProjectConfig{Targets: []ProjectTargetEntry{
		{Name: "claude", Agents: &ResourceTargetConfig{Enabled: &off}},
	}}
	_, err := ValidateProjectConfig(cfg, root)
	if err == nil || !strings.Contains(err.Error(), "agents.enabled") {
		t.Fatalf("expected agents.enabled error, got %v", err)
	}
}

func TestValidateConfig_CustomDisabledTargetStillNeedsPath(t *testing.T) {
	src := t.TempDir()
	off := false
	cfg := &Config{Source: src, Targets: map[string]TargetConfig{
		"my-tool": {Skills: &ResourceTargetConfig{Enabled: &off}},
	}}
	if _, err := ValidateConfig(cfg); err == nil || !strings.Contains(err.Error(), "missing path") {
		t.Fatalf("expected missing path error, got %v", err)
	}
}

func disabledAt(path string) TargetConfig {
	off := false
	return TargetConfig{Skills: &ResourceTargetConfig{Path: path, Enabled: &off}}
}

func TestDetectPathOverlap_IgnoresDisabledTargets(t *testing.T) {
	involved := DetectPathOverlap(map[string]TargetConfig{
		"gemini":    disabledAt("~/.gemini/skills"),
		"universal": {Skills: &ResourceTargetConfig{Path: "~/.agents/skills"}},
	}, false, nil)
	if len(involved) != 0 {
		t.Fatalf("disabled scanner should not overlap, got %v", involved)
	}
	involved = DetectPathOverlap(map[string]TargetConfig{
		"codex":     disabledAt("~/.agents/skills"),
		"universal": {Skills: &ResourceTargetConfig{Path: "~/.agents/skills"}},
	}, false, nil)
	if len(involved) != 0 {
		t.Fatalf("disabled writer should not overlap, got %v", involved)
	}
}

func TestSkillsPathKeptBy_IgnoresDisabledOthers(t *testing.T) {
	targets := map[string]TargetConfig{
		"universal": {Skills: &ResourceTargetConfig{Path: "/shared"}},
		"codex":     disabledAt("/shared"),
	}
	if got := SkillsPathKeptBy(targets, "universal", nil); got != "" {
		t.Fatalf("disabled target should not keep folder, got %q", got)
	}
	if got := SkillsPathKeptBy(targets, "codex", nil); got != "universal" {
		t.Fatalf("enabled universal keeps folder, got %q", got)
	}
}

// Windows folders are case-insensitive: a case-different spelling is the same
// folder, so target remove keeps it rather than unlinking what the other uses.
func TestSkillsPathKeptBy_WindowsCaseFolds(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("case-insensitive paths are Windows-only")
	}
	targets := map[string]TargetConfig{
		"universal": {Skills: &ResourceTargetConfig{Path: `C:\Users\Me\.agents\skills`}},
		"codex":     {Skills: &ResourceTargetConfig{Path: `c:\users\me\.Agents\Skills`}},
	}
	if got := SkillsPathKeptBy(targets, "codex", nil); got != "universal" {
		t.Fatalf("case-different folder not shared, got %q", got)
	}
}

func TestSkillsReaders_Global(t *testing.T) {
	targets := map[string]TargetConfig{
		"universal": {Skills: &ResourceTargetConfig{Path: "~/.agents/skills"}},
		"gemini":    disabledAt("~/.gemini/skills"),
		"pi":        disabledAt("~/.pi/agent/skills"),
		"claude":    {Skills: &ResourceTargetConfig{Path: "~/.claude/skills"}},
		"my-tool":   disabledAt("/my/skills"),
	}
	also := SkillsAlsoReadBy(targets, "universal", "")
	for _, want := range []string{"gemini", "pi"} {
		if !slices.Contains(also, want) {
			t.Errorf("SkillsAlsoReadBy(universal) = %v, missing %s", also, want)
		}
	}
	if slices.Contains(also, "my-tool") || slices.Contains(also, "claude") {
		t.Errorf("SkillsAlsoReadBy(universal) = %v, has unexpected entries", also)
	}
	if !slices.IsSorted(also) {
		t.Errorf("SkillsAlsoReadBy not sorted: %v", also)
	}
	if got := SkillsReadFrom(targets, "gemini", ""); !slices.Equal(got, []string{"universal"}) {
		t.Errorf("SkillsReadFrom(gemini) = %v, want [universal]", got)
	}
	if got := SkillsReadFrom(targets, "my-tool", ""); len(got) != 0 {
		t.Errorf("custom target reads nothing known, got %v", got)
	}
	if got := SkillsAlsoReadBy(targets, "claude", ""); len(got) != 0 {
		t.Errorf("SkillsAlsoReadBy(claude) = %v, want none", got)
	}
}

func TestSkillsReadFrom_SameFolderIsNotAnotherSource(t *testing.T) {
	targets := map[string]TargetConfig{
		"universal": {Skills: &ResourceTargetConfig{Path: "~/.agents/skills"}},
		"codex":     {Skills: &ResourceTargetConfig{Path: "~/.agents/skills"}},
	}
	if got := SkillsReadFrom(targets, "codex", ""); len(got) != 0 {
		t.Errorf("codex sharing universal's folder reads no second copy, got %v", got)
	}
	targets["codex"] = disabledAt("~/.agents/skills")
	if got := SkillsReadFrom(targets, "codex", ""); !slices.Equal(got, []string{"universal"}) {
		t.Errorf("codex with skills off still reads universal's folder, got %v", got)
	}
}

func TestSkillsReaders_Project(t *testing.T) {
	root := t.TempDir()
	targets := map[string]TargetConfig{
		"universal": {Skills: &ResourceTargetConfig{Path: filepath.Join(root, ".agents", "skills")}},
		"gemini":    disabledAt(filepath.Join(root, ".gemini", "skills")),
	}
	if got := SkillsAlsoReadBy(targets, "universal", root); !slices.Equal(got, []string{"gemini"}) {
		t.Errorf("SkillsAlsoReadBy(universal) = %v, want [gemini]", got)
	}
	if got := SkillsReadFrom(targets, "gemini", root); !slices.Equal(got, []string{"universal"}) {
		t.Errorf("SkillsReadFrom(gemini) = %v, want [universal]", got)
	}
}

func TestSkillsReadFrom_UnconfiguredTool(t *testing.T) {
	targets := map[string]TargetConfig{
		"universal": {Skills: &ResourceTargetConfig{Path: "~/.agents/skills"}},
	}
	if got := SkillsReadFrom(targets, "gemini", ""); !slices.Equal(got, []string{"universal"}) {
		t.Errorf("SkillsReadFrom(unconfigured gemini) = %v, want [universal]", got)
	}
	targets["universal"] = disabledAt("~/.agents/skills")
	if got := SkillsReadFrom(targets, "gemini", ""); len(got) != 0 {
		t.Errorf("disabled universal is not a source, got %v", got)
	}
}
