package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func hooksNode(t *testing.T) yaml.Node {
	t.Helper()
	var wrapper struct {
		Hooks yaml.Node `yaml:"hooks"`
	}
	if err := yaml.Unmarshal([]byte("hooks:\n  claude:\n    - name: lint\n"), &wrapper); err != nil {
		t.Fatal(err)
	}
	return wrapper.Hooks
}

func TestSave_OrdersAndSpacesSections(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", cfgPath)

	cfg := &Config{
		Hooks:   hooksNode(t),
		Sources: GlobalSources{Skills: "/src/skills"},
		Mode:    "merge",
		Targets: map[string]TargetConfig{
			"claude": {Skills: &ResourceTargetConfig{Path: "/home/u/.claude/skills"}},
		},
		Ignore: []string{"tmp"},
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	want := string(schemaComment) + `
sources:
  skills: /src/skills

mode: merge

targets:
  claude:
    skills:
      path: /home/u/.claude/skills

hooks:
  claude:
    - name: lint

ignore:
  - tmp
`
	if got := string(data); got != want {
		t.Errorf("Save output mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestProjectSaveIn_OrdersAndSpacesSections(t *testing.T) {
	dir := t.TempDir()
	cfg := &ProjectConfig{
		Hooks:   hooksNode(t),
		Targets: []ProjectTargetEntry{{Name: "claude"}},
	}
	if err := cfg.SaveIn(dir); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	want := string(projectSchemaComment) + "\ntargets:\n  - claude\n\nhooks:\n"
	if got := string(data); !strings.HasPrefix(got, want) {
		t.Errorf("project config should start with targets then hooks\ngot:\n%s", got)
	}
}

func TestMarshalConfig_KeepsOrderWhenSortingWouldBreakAnAlias(t *testing.T) {
	var cfg Config
	src := "hooks: &shared\n  claude: []\nmcp:\n  servers: *shared\nsources:\n  skills: /src\n"
	if err := yaml.Unmarshal([]byte(src), &cfg); err != nil {
		t.Fatal(err)
	}
	data, err := marshalConfig(&cfg, schemaComment, nil)
	if err != nil {
		t.Fatal(err)
	}
	var reloaded Config
	if err := yaml.Unmarshal(data, &reloaded); err != nil {
		t.Errorf("saved config does not load: %v\n%s", err, data)
	}
}

func TestSave_KeepsCommentsOfTheExistingFile(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", cfgPath)
	src := string(schemaComment) + `
# BEGIN HUMAN-ONLY (generated)
sources:
  skills: /src/skills # where skills live
# END HUMAN-ONLY

mode: merge

targets:
  claude:
    skills:
      path: /home/u/.claude/skills
`
	if err := os.WriteFile(cfgPath, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Mode = "copy"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if strings.Count(got, "yaml-language-server") != 1 {
		t.Errorf("schema header should appear once:\n%s", got)
	}
	if !strings.Contains(got, "mode: copy") {
		t.Errorf("changed value missing:\n%s", got)
	}
	for _, want := range []string{"# BEGIN HUMAN-ONLY (generated)\nsources:", "/src/skills # where skills live", "# END HUMAN-ONLY\n\nmode: copy"} {
		if !strings.Contains(got, want) {
			t.Errorf("comment %q lost:\n%s", want, got)
		}
	}
}

func TestMarshalConfig_CommentOnReorderedKeyFollowsIt(t *testing.T) {
	existing := []byte(string(schemaComment) + "\n# kept on mode\nmode: merge\nsources:\n  skills: /src\n")
	cfg := Config{Mode: "merge", Sources: GlobalSources{Skills: "/src"}}
	data, err := marshalConfig(&cfg, schemaComment, existing)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if strings.Count(got, "yaml-language-server") != 1 || !strings.Contains(got, "# kept on mode\nmode: merge") {
		t.Errorf("comment should move with the key, header once:\n%s", got)
	}
}

func TestMarshalConfig_CommentOnListItemSurvivesListEdit(t *testing.T) {
	existing := []byte("ignore:\n  - tmp # scratch\n  - build\n")
	cfg := Config{Ignore: []string{"build", "tmp"}}
	data, err := marshalConfig(&cfg, schemaComment, existing)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "- tmp # scratch") {
		t.Errorf("item comment lost:\n%s", data)
	}
}

func TestMarshalConfig_CommentOfRemovedKeyMovesToNextKey(t *testing.T) {
	existing := []byte("# BEGIN block\nmode: merge # was merge\n# END block\nsources:\n  skills: /src\n")
	cfg := Config{Sources: GlobalSources{Skills: "/src"}}
	data, err := marshalConfig(&cfg, schemaComment, existing)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "# BEGIN block\n# was merge\n# END block\nsources:") {
		t.Errorf("comments of the removed key should sit in front of the next key:\n%s", data)
	}
}

func TestMarshalConfig_CommentOfRemovedLastKeyEndsTheFile(t *testing.T) {
	existing := []byte("sources:\n  skills: /src\n# BEGIN block\nmode: merge # note\n# END block\n")
	cfg := Config{Sources: GlobalSources{Skills: "/src"}}
	data, err := marshalConfig(&cfg, schemaComment, existing)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(strings.TrimSpace(string(data)), "# BEGIN block\n# note\n# END block") {
		t.Errorf("comments of a removed last key should end the file in source order:\n%s", data)
	}
}

func TestMarshalConfig_ListOfMappingsMatchesByName(t *testing.T) {
	existing := []byte("targets:\n  - files:\n      - a\n    name: first # one\n  - files:\n      - b\n    name: second # two\n")
	var cfg ProjectConfig
	if err := yaml.Unmarshal(existing, &cfg); err != nil {
		t.Fatal(err)
	}
	cfg.Targets = cfg.Targets[1:]
	data, err := marshalConfig(&cfg, projectSchemaComment, existing)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "# one\n  - files:") || !strings.Contains(got, "second # two") {
		t.Errorf("the removed target's comment should lead the next one, whose own comment stays:\n%s", got)
	}
}

func TestMarshalConfig_ScalarAndObjectTargetShareComments(t *testing.T) {
	existing := []byte("targets:\n  - claude # mine\n")
	cfg := ProjectConfig{Targets: []ProjectTargetEntry{{Name: "claude", Files: []string{"a"}}}}
	data, err := marshalConfig(&cfg, projectSchemaComment, existing)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "# mine") {
		t.Errorf("comment should survive the switch to object form:\n%s", data)
	}
}

func TestMarshalConfig_RemovedNestedEntryKeepsCommentOrder(t *testing.T) {
	existing := []byte("sources:\n  skills: /src\ntargets:\n  claude:\n    skills:\n      # BEGIN\n      path: /a # note\n      # END\n")
	cfg := Config{Sources: GlobalSources{Skills: "/src"}}
	data, err := marshalConfig(&cfg, schemaComment, existing)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "# BEGIN\n# note\n# END") {
		t.Errorf("nested comments should keep their order:\n%s", data)
	}
}

const sharedIncludeConfig = "targets:\n  a:\n    skills:\n      path: /tmp/a\n      include: &shared\n        - demo\n  b:\n    skills:\n      path: /tmp/b\n      include: *shared\n"

// remarshalConfig loads existing, applies edit, writes it back, and returns
// the bytes plus the config read from them, so a test can check both the text
// and what a later command would load.
func remarshalConfig[T any](t *testing.T, existing string, header []byte, edit func(*T)) (string, T) {
	t.Helper()
	var cfg T
	if err := yaml.Unmarshal([]byte(existing), &cfg); err != nil {
		t.Fatal(err)
	}
	edit(&cfg)
	data, err := marshalConfig(&cfg, header, []byte(existing))
	if err != nil {
		t.Fatal(err)
	}
	var reloaded T
	if err := yaml.Unmarshal(data, &reloaded); err != nil {
		t.Fatalf("written config does not load: %v\n%s", err, data)
	}
	return string(data), reloaded
}

func assertAnchorAndAlias(t *testing.T, got string) {
	t.Helper()
	if !strings.Contains(got, "include: &shared\n") || !strings.Contains(got, "include: *shared\n") {
		t.Errorf("anchor and alias should survive:\n%s", got)
	}
}

func TestMarshalConfig_TargetAnchorSurvivesEdit(t *testing.T) {
	got, cfg := remarshalConfig(t, sharedIncludeConfig, schemaComment, func(c *Config) {
		c.Targets["a"].Skills.Exclude = []string{"old-*"}
	})
	assertAnchorAndAlias(t, got)
	if b := cfg.Targets["b"].Skills.Include; !slices.Equal(b, []string{"demo"}) {
		t.Errorf("b include = %v, want [demo]", b)
	}
}

func TestMarshalConfig_EditedAnchorReachesAlias(t *testing.T) {
	got, cfg := remarshalConfig(t, sharedIncludeConfig, schemaComment, func(c *Config) {
		c.Targets["a"].Skills.Include = []string{"demo", "extra"}
	})
	assertAnchorAndAlias(t, got)
	if b := cfg.Targets["b"].Skills.Include; !slices.Equal(b, []string{"demo", "extra"}) {
		t.Errorf("b include = %v, want [demo extra]", b)
	}
}

func TestMarshalConfig_EditedAliasStopsAliasing(t *testing.T) {
	got, cfg := remarshalConfig(t, sharedIncludeConfig, schemaComment, func(c *Config) {
		c.Targets["b"].Skills.Include = []string{"other"}
	})
	if strings.Contains(got, "*shared") {
		t.Errorf("b was edited and must not alias a:\n%s", got)
	}
	if a := cfg.Targets["a"].Skills.Include; !slices.Equal(a, []string{"demo"}) {
		t.Errorf("a include = %v, want [demo]", a)
	}
	if b := cfg.Targets["b"].Skills.Include; !slices.Equal(b, []string{"other"}) {
		t.Errorf("b include = %v, want [other]", b)
	}
}

func TestMarshalConfig_AnchorLaterInOutputThanAlias(t *testing.T) {
	// Targets are written in name order, so b comes before z and the anchor
	// must move to b instead of leaving an alias in front of it.
	existing := "targets:\n  z:\n    skills:\n      path: /tmp/z\n      include: &shared\n        - demo\n  b:\n    skills:\n      path: /tmp/b # b path\n      include: *shared\n"
	got, cfg := remarshalConfig(t, existing, schemaComment, func(c *Config) {
		c.Targets["z"].Skills.Exclude = []string{"old-*"}
	})
	if !strings.Contains(got, "path: /tmp/b # b path\n      include: &shared\n") || !strings.Contains(got, "include: *shared\n") {
		t.Errorf("anchor should sit on the first target and keep its comments:\n%s", got)
	}
	for name, target := range cfg.Targets {
		if inc := target.Skills.Include; !slices.Equal(inc, []string{"demo"}) {
			t.Errorf("%s include = %v, want [demo]", name, inc)
		}
	}
}

func TestMarshalConfig_ProjectAnchorsSurviveEdit(t *testing.T) {
	existing := "projects:\n  /p/one:\n    targets:\n      - claude\n    skills:\n      include: &shared\n        - demo\n  /p/two:\n    targets:\n      - claude\n    skills:\n      include: *shared\n"
	got, cfg := remarshalConfig(t, existing, schemaComment, func(c *Config) {
		p := c.Projects["/p/one"]
		p.Name = "one"
		c.Projects["/p/one"] = p
	})
	assertAnchorAndAlias(t, got)
	if two := cfg.Projects["/p/two"].Skills.Include; !slices.Equal(two, []string{"demo"}) {
		t.Errorf("/p/two include = %v, want [demo]", two)
	}
}

func TestMarshalConfig_ProjectConfigTargetAnchorSurvivesEdit(t *testing.T) {
	existing := "targets:\n  - name: claude\n    skills:\n      include: &shared\n        - demo\n  - name: codex\n    skills:\n      include: *shared\n"
	got, cfg := remarshalConfig(t, existing, projectSchemaComment, func(c *ProjectConfig) {
		c.Targets[0].Skills.Mode = "copy"
	})
	assertAnchorAndAlias(t, got)
	if codex := cfg.Targets[1].Skills.Include; !slices.Equal(codex, []string{"demo"}) {
		t.Errorf("codex include = %v, want [demo]", codex)
	}
}

func TestMarshalConfig_RemovedAnchorTargetLeavesAliasItsOwnList(t *testing.T) {
	got, cfg := remarshalConfig(t, sharedIncludeConfig, schemaComment, func(c *Config) {
		delete(c.Targets, "a")
	})
	if strings.Contains(got, "*shared") {
		t.Errorf("the anchor is gone, so b must hold its own list:\n%s", got)
	}
	if b := cfg.Targets["b"].Skills.Include; !slices.Equal(b, []string{"demo"}) {
		t.Errorf("b include = %v, want [demo]", b)
	}
}

func TestMarshalConfig_AliasAsListItemSurvivesEdit(t *testing.T) {
	existing := "targets:\n  a:\n    skills:\n      path: /tmp/a\n      include:\n        - &d demo\n        - other\n      exclude:\n        - *d\n"
	got, cfg := remarshalConfig(t, existing, schemaComment, func(c *Config) {
		c.Targets["a"].Skills.Mode = "copy"
	})
	if !strings.Contains(got, "- &d demo\n") || !strings.Contains(got, "- *d\n") {
		t.Errorf("list item anchor and alias should survive:\n%s", got)
	}
	if ex := cfg.Targets["a"].Skills.Exclude; !slices.Equal(ex, []string{"demo"}) {
		t.Errorf("exclude = %v, want [demo]", ex)
	}
}
