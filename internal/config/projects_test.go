package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// loadWithProjects writes a global config whose projects section is body, with $ROOT
// replaced by a temp dir holding the folders named in dirs, and loads it.
func loadWithProjects(t *testing.T, body string, dirs ...string) (*Config, string, error) {
	t.Helper()
	root := t.TempDir()
	for _, dir := range append(dirs, "skills") {
		if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(root, "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", path)
	data := "source: $ROOT/skills\ntargets: {}\nprojects:\n" + body
	if err := os.WriteFile(path, []byte(strings.ReplaceAll(data, "$ROOT", root)), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	return cfg, root, err
}

func TestProjects_ExpandIntoOneTargetPerSkillsFolder(t *testing.T) {
	cfg, root, err := loadWithProjects(t, `  $ROOT/app:
    targets: [claude, codex, cursor]
    skills:
      mode: copy
      include: [security__*]
    agents: {}
`, "app")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Targets) != 2 {
		t.Fatalf("targets %v, want app@claude and app@cursor", cfg.Targets)
	}
	claude := cfg.Targets["app@claude"]
	if sc := claude.SkillsConfig(); sc.Path != filepath.Join(root, "app", ".claude", "skills") || sc.Mode != "copy" || len(sc.Include) != 1 {
		t.Errorf("app@claude skills %+v", sc)
	}
	if got := claude.AgentsConfig().Path; got != filepath.Join(root, "app", ".claude", "agents") {
		t.Errorf("app@claude agents path %q", got)
	}
	// codex and cursor share .agents/skills; cursor names the target because it takes agents.
	shared := cfg.Targets["app@cursor"]
	if got := shared.SkillsConfig().Path; got != filepath.Join(root, "app", ".agents", "skills") {
		t.Errorf("app@cursor skills path %q", got)
	}
	if filepath.Clean(shared.ProjectRoot()) != filepath.Join(root, "app") {
		t.Errorf("project root %q", shared.ProjectRoot())
	}
}

func TestProjects_SaveKeepsProjectTargetsOutOfTargets(t *testing.T) {
	cfg, _, err := loadWithProjects(t, "  $ROOT/app:\n    targets: [claude]\n    skills: {}\n", "app")
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(ConfigPath())
	if strings.Contains(string(data), "app@claude") || !strings.Contains(string(data), "skills: {}") {
		t.Fatalf("saved config:\n%s", data)
	}
}

func TestProjects_AgentsOnlyExcludesEverySkill(t *testing.T) {
	cfg, _, err := loadWithProjects(t, "  $ROOT/app:\n    targets: [claude, codex]\n    agents: {}\n", "app")
	if err != nil {
		t.Fatal(err)
	}
	// codex has no project agents folder, so it gets no target at all.
	if len(cfg.Targets) != 1 {
		t.Fatalf("targets %v", cfg.Targets)
	}
	claude := cfg.Targets["app@claude"]
	if got := claude.SkillsConfig().Exclude; len(got) != 1 || got[0] != "*" {
		t.Errorf("skills exclude %v", got)
	}
}

func TestProjects_MissingFolderGetsNoTargets(t *testing.T) {
	cfg, _, err := loadWithProjects(t, "  $ROOT/gone:\n    targets: [claude]\n    skills: {}\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Targets) != 0 || len(cfg.MissingProjects()) != 1 {
		t.Fatalf("targets %v, missing %v", cfg.Targets, cfg.MissingProjects())
	}
}

func TestProjects_Rejected(t *testing.T) {
	for name, body := range map[string]string{
		"relative root":  "  work/app:\n    targets: [claude]\n",
		"unknown target": "  $ROOT/app:\n    targets: [nope]\n",
		"explicit path":  "  $ROOT/app:\n    targets: [claude]\n    skills:\n      path: /tmp/x\n",
		"same name":      "  $ROOT/app:\n    targets: [claude]\n  $ROOT/other/app:\n    targets: [claude]\n",
		"name with @":    "  $ROOT/app:\n    name: a@b\n    targets: [claude]\n",
	} {
		if _, _, err := loadWithProjects(t, body, "app", "other/app"); err == nil {
			t.Errorf("%s: loaded", name)
		}
	}
}

func TestMatchesTargetName_SeesThroughProjectTargets(t *testing.T) {
	if !MatchesTargetName("claude", "app@claude") || !MatchesTargetName("codex", "app@cursor") || MatchesTargetName("pi", "app@claude") {
		t.Fatal("project target matching is wrong")
	}
}

func TestConvertibleProjects_GroupsCustomPathTargetsByFolder(t *testing.T) {
	root := filepath.Join(t.TempDir(), "legacy")
	same := ResourceTargetConfig{Mode: "copy", Include: []string{"team-*"}}
	target := func(rel string, rc ResourceTargetConfig) TargetConfig {
		rc.Path = filepath.Join(root, rel)
		return TargetConfig{Skills: &rc}
	}
	cfg := &Config{Targets: map[string]TargetConfig{
		"skill-path01": target(".agents/skills", same),
		"claude-legacy": {
			Skills: target(".claude/skills", same).Skills,
			Agents: &ResourceTargetConfig{Path: filepath.Join(root, ".claude", "agents")},
		},
		"elsewhere": target("custom/skills", same),
	}}

	found := cfg.ConvertibleProjects()

	if len(found) != 1 || found[0].Root != root || strings.Join(found[0].Targets, ",") != "claude-legacy,skill-path01" {
		t.Fatalf("found %+v", found)
	}
	project := found[0].Project
	if strings.Join(project.Targets, ",") != "claude,universal" || project.Skills.Mode != "copy" || project.Skills.Path != "" || project.Agents == nil {
		t.Errorf("project %+v skills %+v", project, project.Skills)
	}
}

func TestConvertibleProjects_SkipsFoldersWhoseTargetsDisagree(t *testing.T) {
	root := filepath.Join(t.TempDir(), "legacy")
	cfg := &Config{Targets: map[string]TargetConfig{
		"a": {Skills: &ResourceTargetConfig{Path: filepath.Join(root, ".claude", "skills"), Mode: "copy"}},
		"b": {Skills: &ResourceTargetConfig{Path: filepath.Join(root, ".agents", "skills"), Mode: "merge"}},
	}}
	if found := cfg.ConvertibleProjects(); len(found) != 0 {
		t.Fatalf("found %+v", found)
	}
}

func TestValidateProjects_PrefixedNamingNeedsCopyMode(t *testing.T) {
	cfg, root, err := loadWithProjects(t, "", "app")
	if err != nil {
		t.Fatal(err)
	}
	project := func(mode string) map[string]ManagedProject {
		return map[string]ManagedProject{filepath.Join(root, "app"): {
			Targets: []string{"claude"},
			Skills:  &ResourceTargetConfig{Mode: mode, TargetNaming: "prefixed"},
		}}
	}
	if err := cfg.ValidateProjects(project("")); err == nil || !strings.Contains(err.Error(), "requires copy mode") {
		t.Fatalf("merge default: err = %v, want copy-mode error", err)
	}
	if err := cfg.ValidateProjects(project("copy")); err != nil {
		t.Fatalf("copy: err = %v", err)
	}
}

func TestProjects_AgentsOnlyIgnoresPrefixedNaming(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"app", "skills"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(root, "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", path)
	data := "source: " + root + "/skills\nmode: merge\ntarget_naming: prefixed\ntargets: {}\nprojects:\n  " + root + "/app:\n    targets: [claude]\n    agents: {}\n"
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	// The project syncs no skill, so the inherited naming must not block its agents.
	if _, invalid, err := ValidateConfigForSync(cfg); err != nil || invalid["app@claude"] != nil {
		t.Fatalf("err = %v, invalid = %v; want app@claude valid", err, invalid)
	}
}

func TestConvertibleProjects_SharedAgentIdentityDoesNotMakeRootMixed(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", path)
	data := fmt.Sprintf(`source: %s
targets:
  app-codex:
    agent: codex
    config_dir: %s
    path: %s
  app-universal:
    path: %s
  app-claude:
    agent: claude
    config_dir: %s
    path: %s
`, filepath.Join(root, "skills"),
		filepath.Join(root, "app", ".codex"),
		filepath.Join(root, "app", ".agents", "skills"),
		filepath.Join(root, "app", ".agents", "skills"),
		filepath.Join(root, "app", ".claude"),
		filepath.Join(root, "app", ".claude", "skills"))
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	found := cfg.ConvertibleProjects()
	if len(found) != 1 || len(found[0].Targets) != 3 {
		t.Fatalf("convertible projects = %+v", found)
	}
	if found[0].Project.Skills.Agent != "" || len(found[0].Project.Skills.SharedTargets) != 0 {
		t.Fatalf("identity leaked: %+v", found[0].Project.Skills)
	}
}
