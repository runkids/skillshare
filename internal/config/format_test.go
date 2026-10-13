package config

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
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

const mergeListConfig = "targets:\n  a: &a\n    skills:\n      mode: copy\n  b: &b\n    skills:\n      mode: merge\n    agents:\n      path: /tmp/agents\n  claude:\n    <<: [*a, *b]\n"

const mergeConfig = "targets:\n  base: &base\n    skills:\n      mode: copy\n  claude:\n    # inherits base\n    <<: *base\n    skills:\n      path: /tmp/claude\n  codex:\n    <<: *base\n"

// remarshalMerged is remarshalConfig plus the round trip a merge key must
// keep: what is written loads as the config that was meant to be saved, which
// is what writing it with every merge expanded loads as.
func remarshalMerged[T any](t *testing.T, existing string, header []byte, edit func(*T)) (string, T) {
	t.Helper()
	got, cfg := remarshalConfig(t, existing, header, edit)
	var meant T
	if err := yaml.Unmarshal([]byte(existing), &meant); err != nil {
		t.Fatal(err)
	}
	edit(&meant)
	expanded, err := marshalConfig(&meant, header, nil)
	if err != nil {
		t.Fatal(err)
	}
	var want T
	if err := yaml.Unmarshal(expanded, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("written config loads differently from its expanded form:\nwritten:\n%s\nexpanded:\n%s", got, expanded)
	}
	return got, cfg
}

func TestMarshalConfig_MergeKeySurvivesEdit(t *testing.T) {
	got, cfg := remarshalMerged(t, mergeConfig, schemaComment, func(c *Config) {
		c.Mode = "copy"
	})
	if !strings.Contains(got, strings.TrimPrefix(mergeConfig, "targets:\n")) {
		t.Errorf("merge keys should survive with only local keys written:\n%s", got)
	}
	if m := cfg.Targets["codex"].Skills.Mode; m != "copy" {
		t.Errorf("codex mode = %q, want copy", m)
	}
}

func TestMarshalConfig_MergeKeyListSurvivesEdit(t *testing.T) {
	got, cfg := remarshalMerged(t, mergeListConfig, schemaComment, func(c *Config) {
		c.Mode = "copy"
	})
	if !strings.Contains(got, "  claude:\n    <<: [*a, *b]\n") {
		t.Errorf("merge list should survive:\n%s", got)
	}
	claude := cfg.Targets["claude"]
	if claude.Skills.Mode != "copy" || claude.Agents.Path != "/tmp/agents" {
		t.Errorf("claude = %+v / %+v, want skills from a and agents from b", claude.Skills, claude.Agents)
	}
}

func TestMarshalConfig_MergeKeyInNestedMappingSurvivesEdit(t *testing.T) {
	existing := "targets:\n  base:\n    skills: &shared\n      mode: copy\n      include:\n        - demo\n  claude:\n    skills:\n      <<: *shared\n      path: /tmp/claude\n"
	got, cfg := remarshalMerged(t, existing, schemaComment, func(c *Config) {
		c.Mode = "copy"
	})
	if !strings.Contains(got, "    skills: &shared\n") || !strings.Contains(got, "    skills:\n      <<: *shared\n      path: /tmp/claude\n") {
		t.Errorf("nested merge key should survive:\n%s", got)
	}
	if inc := cfg.Targets["claude"].Skills.Include; !slices.Equal(inc, []string{"demo"}) {
		t.Errorf("claude include = %v, want [demo]", inc)
	}
}

func TestMarshalConfig_ProjectMergeKeySurvivesEdit(t *testing.T) {
	existing := "projects:\n  /p/one: &one\n    targets:\n      - claude\n    skills:\n      include:\n        - demo\n  /p/two:\n    <<: *one\n    name: two\n"
	got, cfg := remarshalMerged(t, existing, schemaComment, func(c *Config) {
		c.Mode = "copy"
	})
	if !strings.Contains(got, "  /p/two:\n    <<: *one\n    name: two\n") {
		t.Errorf("project merge key should survive:\n%s", got)
	}
	if two := cfg.Projects["/p/two"].Targets; !slices.Equal(two, []string{"claude"}) {
		t.Errorf("/p/two targets = %v, want [claude]", two)
	}
}

func TestMarshalConfig_ProjectConfigMergeKeySurvivesEdit(t *testing.T) {
	existing := "targets:\n  - name: claude\n    skills: &sk\n      mode: copy\n  - name: codex\n    skills:\n      <<: *sk\n      path: /tmp/codex\n"
	got, cfg := remarshalMerged(t, existing, projectSchemaComment, func(c *ProjectConfig) {
		c.TargetNaming = "prefixed"
	})
	if !strings.Contains(got, "skills: &sk\n") || !strings.Contains(got, "      <<: *sk\n      path: /tmp/codex\n") {
		t.Errorf("merge key in a target list item should survive:\n%s", got)
	}
	if m := cfg.Targets[1].Skills.Mode; m != "copy" {
		t.Errorf("codex mode = %q, want copy", m)
	}
}

func TestMarshalConfig_EditedAnchorReachesMergingMapping(t *testing.T) {
	// A merge copies keys the mapping does not set itself, so the edit reaches
	// codex and not claude, whose own skills replace the merged ones.
	got, cfg := remarshalConfig(t, mergeConfig, schemaComment, func(c *Config) {
		c.Targets["base"].Skills.Mode = "symlink"
	})
	if !strings.Contains(got, "  claude:\n    # inherits base\n    <<: *base\n    skills:\n      path: /tmp/claude\n  codex:\n    <<: *base\n") {
		t.Errorf("merge keys should survive an edit of the anchored mapping:\n%s", got)
	}
	if m := cfg.Targets["codex"].Skills.Mode; m != "symlink" {
		t.Errorf("codex mode = %q, want symlink", m)
	}
	if claude := cfg.Targets["claude"].Skills; claude.Mode != "" || claude.Path != "/tmp/claude" {
		t.Errorf("claude skills = %+v, want only its own path", claude)
	}
}

func TestMarshalConfig_LocalOverrideEqualToMergedValueStaysLocal(t *testing.T) {
	existing := "targets:\n  base: &base\n    skills:\n      mode: copy\n  claude:\n    <<: *base\n    skills:\n      mode: copy\n"
	got, cfg := remarshalConfig(t, existing, schemaComment, func(c *Config) {
		c.Targets["base"].Skills.Mode = "symlink"
	})
	if !strings.Contains(got, "  claude:\n    <<: *base\n    skills:\n      mode: copy\n") {
		t.Errorf("an override must stay in claude:\n%s", got)
	}
	if m := cfg.Targets["claude"].Skills.Mode; m != "copy" {
		t.Errorf("claude mode = %q, want its own copy", m)
	}
}

func TestMarshalConfig_EditedMergedValueBecomesLocalOverride(t *testing.T) {
	got, cfg := remarshalMerged(t, mergeConfig, schemaComment, func(c *Config) {
		c.Targets["codex"].Skills.Mode = "symlink"
	})
	if !strings.Contains(got, "  codex:\n    <<: *base\n    skills:\n      mode: symlink\n") {
		t.Errorf("codex should keep the merge key and write its own skills:\n%s", got)
	}
	if m := cfg.Targets["base"].Skills.Mode; m != "copy" {
		t.Errorf("base mode = %q, want copy", m)
	}
}

func TestMarshalConfig_RemovedAnchorWritesMergedKeysOut(t *testing.T) {
	got, cfg := remarshalMerged(t, mergeConfig, schemaComment, func(c *Config) {
		delete(c.Targets, "base")
	})
	if strings.Contains(got, "<<") || strings.Contains(got, "*base") {
		t.Errorf("the anchor is gone, so the merged keys must be written out:\n%s", got)
	}
	if m := cfg.Targets["codex"].Skills.Mode; m != "copy" {
		t.Errorf("codex mode = %q, want copy", m)
	}
}

func TestMarshalConfig_RemovedOneMergeSourceKeepsTheOther(t *testing.T) {
	got, cfg := remarshalMerged(t, mergeListConfig, schemaComment, func(c *Config) {
		delete(c.Targets, "a")
	})
	if !strings.Contains(got, "  claude:\n    <<: [*b]\n    skills:\n      mode: copy\n") {
		t.Errorf("claude should merge b and keep the skills it took from a:\n%s", got)
	}
	if m := cfg.Targets["claude"].Skills.Mode; m != "copy" {
		t.Errorf("claude mode = %q, want copy", m)
	}
}

func TestMarshalConfig_ClearedMergedKeyIsNotMergedBack(t *testing.T) {
	existing := "targets:\n  base: &base\n    path: /tmp/base\n    mode: copy\n  claude:\n    <<: *base\n"
	got, cfg := remarshalMerged(t, existing, schemaComment, func(c *Config) {
		c.Targets["claude"] = TargetConfig{Path: "/tmp/base"}
	})
	if strings.Contains(got, "<<") {
		t.Errorf("claude cleared mode, which a merge would bring back:\n%s", got)
	}
	if m := cfg.Targets["claude"].Mode; m != "" {
		t.Errorf("claude mode = %q, want it cleared", m)
	}
}

func TestMarshalConfig_AnchorAfterMergeInOutputMovesToIt(t *testing.T) {
	// Targets are written in name order, so claude comes before zbase and the
	// anchor must move to the merge key instead of following it.
	existing := "targets:\n  zbase: &base\n    skills:\n      mode: copy\n  claude:\n    <<: *base\n    skills:\n      path: /tmp/claude\n"
	got, cfg := remarshalMerged(t, existing, schemaComment, func(c *Config) {
		c.Mode = "copy"
	})
	if !strings.Contains(got, "    <<: &base\n") || !strings.Contains(got, "  zbase: *base\n") {
		t.Errorf("anchor should sit on the first merge key:\n%s", got)
	}
	if m := cfg.Targets["zbase"].Skills.Mode; m != "copy" {
		t.Errorf("zbase mode = %q, want copy", m)
	}
	if again, _ := remarshalMerged(t, got, schemaComment, func(*Config) {}); again != got {
		t.Errorf("a second write should keep the moved anchor:\n%s", again)
	}
	// zbase is now an alias, but it is still where the file's anchor came from.
	edited, _ := remarshalConfig(t, got, schemaComment, func(c *Config) {
		c.Targets["zbase"].Skills.Exclude = []string{"old-*"}
	})
	if !strings.Contains(edited, "    <<: &base\n      skills:\n        mode: copy\n        exclude:\n          - old-*\n") || !strings.Contains(edited, "  zbase: *base\n") {
		t.Errorf("editing the alias of a moved anchor should keep the merge:\n%s", edited)
	}
}

func TestMarshalConfig_EditedFirstAliasOfMovedAnchorIsNotTheAnchor(t *testing.T) {
	// m and p are aliases of the anchor an earlier write put on claude's merge
	// key. m is edited, so it gets its own copy and p still stands for the anchor.
	existing := "targets:\n  claude:\n    <<: &base\n      skills:\n        mode: copy\n    skills:\n      path: /tmp/claude\n  m: *base\n  p: *base\n"
	got, cfg := remarshalConfig(t, existing, schemaComment, func(c *Config) {
		c.Targets["m"].Skills.Mode = "symlink"
	})
	if !strings.Contains(got, "  m:\n    skills:\n      mode: symlink\n") || !strings.Contains(got, "    <<: &base\n      skills:\n        mode: copy\n") || !strings.Contains(got, "  p: *base\n") {
		t.Errorf("m should keep its own copy and p the anchor:\n%s", got)
	}
	if m := cfg.Targets["p"].Skills.Mode; m != "copy" {
		t.Errorf("p mode = %q, want copy", m)
	}
}

func TestMarshalConfig_ChainedMergeKeysSurviveEdit(t *testing.T) {
	existing := "targets:\n  b1: &b1\n    path: /tmp/b1\n    mode: copy\n  b2: &b2\n    <<: *b1\n    agents:\n      path: /tmp/agents\n  claude:\n    <<: *b2\n    skills:\n      path: /tmp/claude\n"
	got, cfg := remarshalMerged(t, existing, schemaComment, func(c *Config) {
		c.Mode = "copy"
	})
	if !strings.Contains(got, "  b2: &b2\n    <<: *b1\n    agents:\n      path: /tmp/agents\n  claude:\n    <<: *b2\n    skills:\n      path: /tmp/claude\n") {
		t.Errorf("a merge source that merges another should survive:\n%s", got)
	}
	if m := cfg.Targets["claude"].Mode; m != "copy" {
		t.Errorf("claude mode = %q, want copy", m)
	}
}

// editEntry changes one entry of a config map that holds structs.
func editEntry[K comparable, V any](m map[K]V, key K, edit func(*V)) {
	v := m[key]
	edit(&v)
	m[key] = v
}

// claude sets its own agents, codex only merges.
const mergeAgentsConfig = "targets:\n  base: &base\n    agents:\n      mode: copy\n  claude:\n    <<: *base\n    agents:\n      mode: copy\n  codex:\n    <<: *base\n"

func TestMarshalConfig_KeyAddedToAnchorReachesMergingMapping(t *testing.T) {
	existing := "targets:\n  base: &base\n    skills:\n      mode: copy\n  claude:\n    <<: *base\n    agents:\n      path: /tmp/claude-agents\n  codex:\n    <<: *base\n"
	got, cfg := remarshalConfig(t, existing, schemaComment, func(c *Config) {
		editEntry(c.Targets, "base", func(b *TargetConfig) { b.Agents = &ResourceTargetConfig{Mode: "copy"} })
	})
	if !strings.Contains(got, "  base: &base\n    skills:\n      mode: copy\n    agents:\n      mode: copy\n  claude:\n    <<: *base\n    agents:\n      path: /tmp/claude-agents\n  codex:\n    <<: *base\n") {
		t.Errorf("merge keys should survive and base should carry the new key:\n%s", got)
	}
	if a := cfg.Targets["codex"].Agents; a == nil || a.Mode != "copy" {
		t.Errorf("codex agents = %+v, want mode copy from base", a)
	}
	if a := cfg.Targets["claude"].Agents; a.Path != "/tmp/claude-agents" || a.Mode != "" {
		t.Errorf("claude agents = %+v, want only its own path", a)
	}
}

func TestMarshalConfig_KeyRemovedFromAnchorReachesMergingMapping(t *testing.T) {
	got, cfg := remarshalConfig(t, mergeAgentsConfig, schemaComment, func(c *Config) {
		editEntry(c.Targets, "base", func(b *TargetConfig) { b.Agents = nil })
	})
	if !strings.Contains(got, "  claude:\n    <<: *base\n    agents:\n      mode: copy\n  codex:\n    <<: *base\n") {
		t.Errorf("claude set agents itself and must keep them, codex only its merge key:\n%s", got)
	}
	if a := cfg.Targets["codex"].Agents; a != nil {
		t.Errorf("codex agents = %+v, want none", a)
	}
}

func TestMarshalConfig_KeyRemovedFromAnchorKeepsValueTheCommandEdited(t *testing.T) {
	got, cfg := remarshalConfig(t, mergeAgentsConfig, schemaComment, func(c *Config) {
		editEntry(c.Targets, "base", func(b *TargetConfig) { b.Agents = nil })
		c.Targets["codex"].Agents.Mode = "symlink"
	})
	if !strings.Contains(got, "  codex:\n    <<: *base\n    agents:\n      mode: symlink\n") {
		t.Errorf("codex was edited and must keep its agents:\n%s", got)
	}
	if m := cfg.Targets["claude"].Agents.Mode; m != "copy" {
		t.Errorf("claude agents mode = %q, want its own copy", m)
	}
}

func TestMarshalConfig_KeyAddedToMergingMappingStaysLocal(t *testing.T) {
	got, _ := remarshalMerged(t, mergeConfig, schemaComment, func(c *Config) {
		editEntry(c.Targets, "codex", func(x *TargetConfig) { x.Agents = &ResourceTargetConfig{Path: "/tmp/codex-agents"} })
	})
	if !strings.Contains(got, "  codex:\n    <<: *base\n    agents:\n      path: /tmp/codex-agents\n") {
		t.Errorf("codex should keep the merge key and write the new key:\n%s", got)
	}
}

func TestMarshalConfig_KeyRemovedFromMergingMappingStaysRemoved(t *testing.T) {
	existing := "targets:\n  base: &base\n    skills:\n      mode: copy\n  claude:\n    <<: *base\n    agents:\n      path: /tmp/claude-agents\n"
	got, _ := remarshalMerged(t, existing, schemaComment, func(c *Config) {
		editEntry(c.Targets, "claude", func(x *TargetConfig) { x.Agents = nil })
	})
	if !strings.Contains(got, "  claude:\n    <<: *base\n") || strings.Contains(got, "agents") {
		t.Errorf("claude should keep only its merge key:\n%s", got)
	}
}

func TestMarshalConfig_ChangedLocalKeyStaysLocal(t *testing.T) {
	got, _ := remarshalMerged(t, mergeConfig, schemaComment, func(c *Config) {
		c.Targets["claude"].Skills.Path = "/tmp/other"
	})
	if !strings.Contains(got, "  claude:\n    # inherits base\n    <<: *base\n    skills:\n      path: /tmp/other\n") {
		t.Errorf("claude should keep the merge key and its changed skills:\n%s", got)
	}
}

func TestMarshalConfig_MergingMappingEditedWhileAnchorEditedKeepsItsValue(t *testing.T) {
	got, cfg := remarshalConfig(t, mergeConfig, schemaComment, func(c *Config) {
		c.Targets["base"].Skills.Mode = "symlink"
		c.Targets["codex"].Skills.Mode = "merge"
	})
	if !strings.Contains(got, "  codex:\n    <<: *base\n    skills:\n      mode: merge\n") {
		t.Errorf("codex was edited and must keep its own value:\n%s", got)
	}
	if m := cfg.Targets["base"].Skills.Mode; m != "symlink" {
		t.Errorf("base mode = %q, want symlink", m)
	}
}

func TestMarshalConfig_KeyOfSecondMergeSourceChangedReachesMergingMapping(t *testing.T) {
	got, cfg := remarshalConfig(t, mergeListConfig, schemaComment, func(c *Config) {
		c.Targets["b"].Agents.Path = "/tmp/new-agents"
	})
	if !strings.Contains(got, "  claude:\n    <<: [*a, *b]\n") {
		t.Errorf("merge list should survive:\n%s", got)
	}
	if p := cfg.Targets["claude"].Agents.Path; p != "/tmp/new-agents" {
		t.Errorf("claude agents path = %q, want the edited one from b", p)
	}
}

func TestMarshalConfig_KeyOfSecondMergeSourceRemovedReachesMergingMapping(t *testing.T) {
	got, cfg := remarshalConfig(t, mergeListConfig, schemaComment, func(c *Config) {
		editEntry(c.Targets, "b", func(b *TargetConfig) { b.Agents = nil })
	})
	if !strings.Contains(got, "  claude:\n    <<: [*a, *b]\n") || strings.Contains(got, "/tmp/agents") {
		t.Errorf("claude should keep the merge list and lose the removed key:\n%s", got)
	}
	if c := cfg.Targets["claude"]; c.Agents != nil || c.Skills.Mode != "copy" {
		t.Errorf("claude = %+v, want no agents and skills from a", c)
	}
}

func TestMarshalConfig_KeyAddedToSecondMergeSourceReachesMergingMapping(t *testing.T) {
	got, cfg := remarshalConfig(t, mergeListConfig, schemaComment, func(c *Config) {
		editEntry(c.Targets, "b", func(b *TargetConfig) { b.Path = "/tmp/b" })
	})
	if !strings.Contains(got, "  claude:\n    <<: [*a, *b]\n") {
		t.Errorf("merge list should survive:\n%s", got)
	}
	if p := cfg.Targets["claude"].Path; p != "/tmp/b" {
		t.Errorf("claude path = %q, want /tmp/b from b", p)
	}
}

func TestMarshalConfig_KeyAddedToFirstMergeSourceOverridesSecond(t *testing.T) {
	_, cfg := remarshalConfig(t, mergeListConfig, schemaComment, func(c *Config) {
		editEntry(c.Targets, "a", func(a *TargetConfig) { a.Agents = &ResourceTargetConfig{Path: "/tmp/a-agents"} })
	})
	if p := cfg.Targets["claude"].Agents.Path; p != "/tmp/a-agents" {
		t.Errorf("claude agents path = %q, want the first source's", p)
	}
}

func TestMarshalConfig_KeyRemovedFromFirstMergeSourceFallsToSecond(t *testing.T) {
	got, cfg := remarshalConfig(t, mergeListConfig, schemaComment, func(c *Config) {
		editEntry(c.Targets, "a", func(a *TargetConfig) { a.Skills = nil })
	})
	if !strings.Contains(got, "  claude:\n    <<: [*a, *b]\n") {
		t.Errorf("merge list should survive:\n%s", got)
	}
	if m := cfg.Targets["claude"].Skills.Mode; m != "merge" {
		t.Errorf("claude skills mode = %q, want b's now that a no longer gives skills", m)
	}
}

func TestMarshalConfig_KeyAddedToNestedAnchorReachesMergingMapping(t *testing.T) {
	existing := "targets:\n  base:\n    skills: &shared\n      mode: copy\n  claude:\n    skills:\n      <<: *shared\n      path: /tmp/claude\n  codex:\n    skills:\n      <<: *shared\n"
	got, cfg := remarshalConfig(t, existing, schemaComment, func(c *Config) {
		c.Targets["base"].Skills.Include = []string{"demo"}
	})
	if !strings.Contains(got, "    skills:\n      <<: *shared\n      path: /tmp/claude\n  codex:\n    skills:\n      <<: *shared\n") {
		t.Errorf("nested merge keys should survive:\n%s", got)
	}
	for _, name := range []string{"claude", "codex"} {
		if inc := cfg.Targets[name].Skills.Include; !slices.Equal(inc, []string{"demo"}) {
			t.Errorf("%s include = %v, want [demo]", name, inc)
		}
	}
}

func TestMarshalConfig_KeyRemovedFromNestedAnchorReachesMergingMapping(t *testing.T) {
	existing := "targets:\n  base:\n    skills: &shared\n      mode: copy\n      include:\n        - demo\n  claude:\n    skills:\n      <<: *shared\n      path: /tmp/claude\n"
	got, cfg := remarshalConfig(t, existing, schemaComment, func(c *Config) {
		c.Targets["base"].Skills.Include = nil
	})
	if !strings.Contains(got, "    skills:\n      <<: *shared\n      path: /tmp/claude\n") || strings.Contains(got, "demo") {
		t.Errorf("claude should keep the merge key and lose include:\n%s", got)
	}
	if inc := cfg.Targets["claude"].Skills.Include; len(inc) != 0 {
		t.Errorf("claude include = %v, want none", inc)
	}
}

func TestMarshalConfig_KeyAddedToAnchoredProjectReachesMergingProject(t *testing.T) {
	existing := "projects:\n  /p/one: &one\n    targets:\n      - claude\n  /p/two:\n    <<: *one\n    name: two\n"
	got, cfg := remarshalConfig(t, existing, schemaComment, func(c *Config) {
		editEntry(c.Projects, "/p/one", func(p *ManagedProject) { p.Agents = &ResourceTargetConfig{Mode: "copy"} })
	})
	if !strings.Contains(got, "  /p/two:\n    <<: *one\n    name: two\n") {
		t.Errorf("project merge key should survive:\n%s", got)
	}
	if a := cfg.Projects["/p/two"].Agents; a == nil || a.Mode != "copy" {
		t.Errorf("/p/two agents = %+v, want mode copy from /p/one", a)
	}
}

func TestMarshalConfig_KeyRemovedFromAnchoredProjectReachesMergingProject(t *testing.T) {
	existing := "projects:\n  /p/one: &one\n    targets:\n      - claude\n    skills:\n      include:\n        - demo\n  /p/two:\n    <<: *one\n    name: two\n"
	got, cfg := remarshalConfig(t, existing, schemaComment, func(c *Config) {
		editEntry(c.Projects, "/p/one", func(p *ManagedProject) { p.Skills = nil })
	})
	if !strings.Contains(got, "  /p/two:\n    <<: *one\n    name: two\n") || strings.Contains(got, "demo") {
		t.Errorf("/p/two should keep the merge key and lose skills:\n%s", got)
	}
	if s := cfg.Projects["/p/two"].Skills; s != nil {
		t.Errorf("/p/two skills = %+v, want none", s)
	}
}

func TestMarshalConfig_KeyAddedToAnchorInProjectConfigReachesMergingTarget(t *testing.T) {
	existing := "targets:\n  - name: claude\n    skills: &sk\n      mode: copy\n  - name: codex\n    skills:\n      <<: *sk\n      path: /tmp/codex\n"
	got, cfg := remarshalConfig(t, existing, projectSchemaComment, func(c *ProjectConfig) {
		c.Targets[0].Skills.Include = []string{"demo"}
	})
	if !strings.Contains(got, "      <<: *sk\n      path: /tmp/codex\n") {
		t.Errorf("merge key in a target list item should survive:\n%s", got)
	}
	if inc := cfg.Targets[1].Skills.Include; !slices.Equal(inc, []string{"demo"}) {
		t.Errorf("codex include = %v, want [demo]", inc)
	}
}

func TestMarshalConfig_KeyRemovedFromAnchorInProjectConfigReachesMergingTarget(t *testing.T) {
	existing := "targets:\n  - name: claude\n    skills: &sk\n      mode: copy\n      include:\n        - demo\n  - name: codex\n    skills:\n      <<: *sk\n      path: /tmp/codex\n"
	got, cfg := remarshalConfig(t, existing, projectSchemaComment, func(c *ProjectConfig) {
		c.Targets[0].Skills.Include = nil
	})
	if !strings.Contains(got, "      <<: *sk\n      path: /tmp/codex\n") || strings.Contains(got, "demo") {
		t.Errorf("codex should keep the merge key and lose include:\n%s", got)
	}
	if inc := cfg.Targets[1].Skills.Include; len(inc) != 0 {
		t.Errorf("codex include = %v, want none", inc)
	}
}

func TestMarshalConfig_KeyAddedToAnchoredTargetListItemReachesMergingItem(t *testing.T) {
	existing := "targets:\n  - &base\n    name: claude\n    skills:\n      mode: copy\n  - <<: *base\n    name: codex\n"
	got, cfg := remarshalConfig(t, existing, projectSchemaComment, func(c *ProjectConfig) {
		c.Targets[0].Agents = &ResourceTargetConfig{Mode: "copy"}
	})
	if !strings.Contains(got, "  - <<: *base\n    name: codex\n") {
		t.Errorf("merge key in a list item should survive:\n%s", got)
	}
	if a := cfg.Targets[1].Agents; a == nil || a.Mode != "copy" {
		t.Errorf("codex agents = %+v, want mode copy from claude", a)
	}
}

func TestMarshalConfig_KeyAddedToChainedAnchorReachesLastMergingMapping(t *testing.T) {
	existing := "targets:\n  b1: &b1\n    path: /tmp/b1\n  b2: &b2\n    <<: *b1\n    agents:\n      path: /tmp/agents\n  claude:\n    <<: *b2\n    skills:\n      path: /tmp/claude\n"
	got, cfg := remarshalConfig(t, existing, schemaComment, func(c *Config) {
		editEntry(c.Targets, "b1", func(b *TargetConfig) { b.Mode = "copy" })
	})
	if !strings.Contains(got, "  b2: &b2\n    <<: *b1\n    agents:\n      path: /tmp/agents\n  claude:\n    <<: *b2\n    skills:\n      path: /tmp/claude\n") {
		t.Errorf("chained merge keys should survive:\n%s", got)
	}
	if m := cfg.Targets["claude"].Mode; m != "copy" {
		t.Errorf("claude mode = %q, want copy through b2", m)
	}
}

func TestMarshalConfig_KeyRemovedFromChainedAnchorReachesLastMergingMapping(t *testing.T) {
	existing := "targets:\n  b1: &b1\n    path: /tmp/b1\n    mode: copy\n  b2: &b2\n    <<: *b1\n    agents:\n      path: /tmp/agents\n  claude:\n    <<: *b2\n    skills:\n      path: /tmp/claude\n"
	got, cfg := remarshalConfig(t, existing, schemaComment, func(c *Config) {
		editEntry(c.Targets, "b1", func(b *TargetConfig) { b.Mode = "" })
	})
	if !strings.Contains(got, "  claude:\n    <<: *b2\n    skills:\n      path: /tmp/claude\n") || strings.Contains(got, "mode:") {
		t.Errorf("claude should keep its merge key and lose the removed key:\n%s", got)
	}
	if m := cfg.Targets["claude"].Mode; m != "" {
		t.Errorf("claude mode = %q, want none", m)
	}
}

// assertCommentsOnce fails unless every `# marker` of existing (a single word
// after the hash) is in got exactly once: a comment must neither be lost nor
// written twice.
func assertCommentsOnce(t *testing.T, existing, got string) {
	t.Helper()
	markers := regexp.MustCompile(`# [a-z0-9-]+`).FindAllString(existing, -1)
	if len(markers) == 0 {
		t.Fatal("fixture has no comments")
	}
	for _, m := range markers {
		// A marker followed by a name character is another marker.
		if n := len(regexp.MustCompile(regexp.QuoteMeta(m)+`(?:\s|$)`).FindAllString(got, -1)); n != 1 {
			t.Errorf("comment %q is written %d times, want once:\n%s", m, n, got)
		}
	}
}

func TestMarshalConfig_CommentsOnMergeListItemsSurviveEdit(t *testing.T) {
	existing := "targets:\n  a: &a\n    skills:\n      mode: copy\n  b: &b\n    agents:\n      path: /tmp/agents\n  claude:\n    <<:\n      # about-a\n      - *a # line-a\n      - *b # line-b\n      # foot-b\n    skills:\n      path: /tmp/claude\n"
	got, _ := remarshalMerged(t, existing, schemaComment, func(c *Config) {
		c.Mode = "copy"
	})
	assertCommentsOnce(t, existing, got)
	if want := "    <<:\n      # about-a\n      - *a # line-a\n      - *b # line-b\n      # foot-b\n"; !strings.Contains(got, want) {
		t.Errorf("items of the merge list should keep their comments in place:\n%s", got)
	}
}

func TestMarshalConfig_CommentsOnFlowMergeListItemsSurviveEdit(t *testing.T) {
	existing := "targets:\n  a: &a\n    skills:\n      mode: copy\n  b: &b\n    agents:\n      path: /tmp/agents\n  claude:\n    <<: [*a, # line-a\n      *b] # line-list\n"
	got, _ := remarshalMerged(t, existing, schemaComment, func(c *Config) {
		c.Mode = "copy"
	})
	assertCommentsOnce(t, existing, got)
}

const commentedMergeList = "targets:\n  a: &a\n    skills:\n      mode: copy\n  b: &b\n    agents:\n      path: /tmp/agents\n  claude:\n    <<:\n      - *a # line-a\n      - *b # line-b\n"

func TestMarshalConfig_CommentOnRemovedMergeSourceMovesToNextItem(t *testing.T) {
	existing := commentedMergeList
	got, _ := remarshalConfig(t, existing, schemaComment, func(c *Config) {
		delete(c.Targets, "a")
	})
	assertCommentsOnce(t, existing, got)
	if want := "      # line-a\n      - *b # line-b\n"; !strings.Contains(got, want) {
		t.Errorf("the comment of the dropped item should head the next one:\n%s", got)
	}
}

func TestMarshalConfig_CommentOnLastRemovedMergeSourceEndsTheList(t *testing.T) {
	existing := commentedMergeList
	got, _ := remarshalConfig(t, existing, schemaComment, func(c *Config) {
		delete(c.Targets, "b")
	})
	assertCommentsOnce(t, existing, got)
	if want := "      - *a # line-a\n      # line-b\n"; !strings.Contains(got, want) {
		t.Errorf("the comment of the dropped last item should follow the one before it:\n%s", got)
	}
}

func TestMarshalConfig_CommentOnMergeSourceDroppedByClearedKeyIsKept(t *testing.T) {
	// The command clears a key only a has, so a is no longer merged and b stays.
	existing := "targets:\n  a: &a\n    path: /tmp/a\n  b: &b\n    mode: copy\n  claude:\n    <<:\n      - *a # line-a\n      - *b # line-b\n"
	got, _ := remarshalConfig(t, existing, schemaComment, func(c *Config) {
		editEntry(c.Targets, "claude", func(t *TargetConfig) { t.Path = "" })
	})
	assertCommentsOnce(t, existing, got)
}

func TestMarshalConfig_CommentsOfInlineMergeSourceFollowItsAnchor(t *testing.T) {
	// zbase stands for the anchor an earlier write put on claude's merge key.
	// Both writes must keep the comments of the mapping's own keys.
	existing := "targets:\n  claude:\n    <<: &base # line-merge\n      # about-mode\n      skills:\n        mode: copy # line-mode\n    skills:\n      path: /tmp/claude\n  zbase: *base # line-alias\n"
	got, _ := remarshalMerged(t, existing, schemaComment, func(c *Config) {
		c.Mode = "copy"
	})
	assertCommentsOnce(t, existing, got)
	again, _ := remarshalMerged(t, got, schemaComment, func(*Config) {})
	assertCommentsOnce(t, existing, again)
}

func TestMarshalConfig_CommentOfRemovedLastLocalKeyStaysWhenOnlyMergedKeysRemain(t *testing.T) {
	existing := "targets:\n  base: &base\n    path: /tmp/base\n    mode: copy\n  claude:\n    <<: *base\n    # about-files\n    files:\n      - a.md # line-files\n    # foot-files\n"
	got, _ := remarshalMerged(t, existing, schemaComment, func(c *Config) {
		editEntry(c.Targets, "claude", func(t *TargetConfig) { t.Files = nil })
	})
	assertCommentsOnce(t, existing, got)
	if want := "  claude:\n    # about-files\n    # line-files\n    # foot-files\n    <<: *base\n"; !strings.Contains(got, want) {
		t.Errorf("claude should keep only its merge key, with the comments in front:\n%s", got)
	}
}

func TestMarshalConfig_CommentOfRemovedKeyFollowsLastKeptKeyWhenMergedKeysFollow(t *testing.T) {
	// path is local and kept; the removed key's comments would land on the
	// last key, which the merge supplies, and must follow path instead.
	existing := "targets:\n  base: &base\n    mode: copy\n    include: [x]\n  claude:\n    <<: *base\n    path: /tmp/claude # line-path\n    files:\n      - a.md # line-files\n    # foot-files\n"
	got, _ := remarshalMerged(t, existing, schemaComment, func(c *Config) {
		editEntry(c.Targets, "claude", func(t *TargetConfig) { t.Files = nil })
	})
	assertCommentsOnce(t, existing, got)
	if want := "    path: /tmp/claude # line-path\n    # line-files\n    # foot-files\n"; !strings.Contains(got, want) {
		t.Errorf("the comments should follow the last kept key:\n%s", got)
	}
}

func TestMarshalConfig_CommentAfterAnchorKeepsFirstKeyComment(t *testing.T) {
	// yaml.v3 parses the comment after an anchor as the line comment of the
	// first key, which then collides with the one on that key's value.
	existing := "targets:\n  base: &base # line-anchor\n    # about-path\n    path: /tmp/base # line-path\n    mode: copy\n  claude:\n    <<: *base\n"
	got, _ := remarshalMerged(t, existing, schemaComment, func(c *Config) {
		c.Mode = "copy"
	})
	assertCommentsOnce(t, existing, got)
	if want := "  base: &base\n    # line-anchor\n    # about-path\n    path: /tmp/base # line-path\n"; !strings.Contains(got, want) {
		t.Errorf("the comment after the anchor should head the first key, not follow another key:\n%s", got)
	}
}

func TestMarshalConfig_CommentAfterAnchorOfMergingMappingIsKept(t *testing.T) {
	// The anchored mapping's first key is a merge key, whose value is an alias.
	existing := "targets:\n  b1: &b1\n    path: /tmp/b1\n  b2: &b2 # line-anchor\n    # about-merge\n    <<: *b1 # line-merge\n    mode: copy\n  claude:\n    <<: *b2\n"
	got, _ := remarshalMerged(t, existing, schemaComment, func(c *Config) {
		c.Mode = "copy"
	})
	assertCommentsOnce(t, existing, got)
	if want := "  b2: &b2\n    # line-anchor\n    # about-merge\n    <<: *b1 # line-merge\n"; !strings.Contains(got, want) {
		t.Errorf("the comment after the anchor should head the merge key:\n%s", got)
	}
}

func TestMarshalConfig_CommentOnAliasEditedIntoOwnCopyIsKept(t *testing.T) {
	existing := "targets:\n  a: &a\n    skills:\n      mode: copy\n  b: *a # line-alias\n"
	got, _ := remarshalConfig(t, existing, schemaComment, func(c *Config) {
		c.Targets["b"].Skills.Mode = "symlink"
	})
	assertCommentsOnce(t, existing, got)
}

func TestMarshalConfig_CommentOnMergeKeyFollowsMovedAnchor(t *testing.T) {
	// claude sorts before zbase, so the anchor moves onto its merge key, which
	// then holds a block mapping that has no place for a line comment.
	existing := "targets:\n  zbase: &base # line-anchor\n    # about-path\n    path: /tmp/base # line-path\n  claude:\n    <<: *base # line-merge\n    skills:\n      path: /tmp/claude\n"
	got, _ := remarshalMerged(t, existing, schemaComment, func(c *Config) {
		c.Mode = "copy"
	})
	assertCommentsOnce(t, existing, got)
	again, _ := remarshalMerged(t, got, schemaComment, func(*Config) {})
	assertCommentsOnce(t, existing, again)
}

func TestMarshalConfig_CommentOnKeyEmptiedByEditIsKept(t *testing.T) {
	existing := "targets:\n  claude: # line-claude\n    path: /tmp/claude # line-path\n  codex:\n    path: /tmp/codex\n"
	got, _ := remarshalConfig(t, existing, schemaComment, func(c *Config) {
		c.Targets["claude"] = TargetConfig{}
	})
	assertCommentsOnce(t, existing, got)
	if want := "  claude: {} # line-claude\n"; !strings.Contains(got, want) {
		t.Errorf("the key's comment should follow its empty value:\n%s", got)
	}
}

func TestMarshalConfig_CommentsOfMergeKeyInsideInlineSourceAreKept(t *testing.T) {
	// The merged mapping is written inline on claude's merge key and merges a
	// mapping itself; zbase is the alias an earlier write left of its anchor.
	existing := "targets:\n  b1: &b1\n    path: /tmp/b1\n  claude:\n    <<: &mid\n      # about-merge\n      <<: *b1 # line-merge\n      mode: copy # line-mode\n    skills:\n      path: /tmp/claude\n  zmid: *mid\n"
	got, _ := remarshalMerged(t, existing, schemaComment, func(c *Config) {
		c.Mode = "copy"
	})
	assertCommentsOnce(t, existing, got)
}

func TestMarshalConfig_CommentsOnMergeListItemsStayPutWhenAnchorsMoveOntoThem(t *testing.T) {
	// za and zb sort after claude, so their anchors move onto its merge items.
	existing := "targets:\n  za: &a\n    skills:\n      mode: copy\n  zb: &b\n    agents:\n      path: /tmp/agents\n  claude:\n    <<:\n      # about-a\n      - *a # line-a\n      - *b # line-b\n"
	got, _ := remarshalMerged(t, existing, schemaComment, func(c *Config) {
		c.Mode = "copy"
	})
	assertCommentsOnce(t, existing, got)
	if again, _ := remarshalMerged(t, got, schemaComment, func(*Config) {}); again != got {
		t.Errorf("a second write should keep every comment where it is:\n%s\nthen:\n%s", got, again)
	}
}

func TestMarshalConfig_CommentsOfRemovedKeyStayOnMergeKeyWhenAnchorMovesOntoIt(t *testing.T) {
	existing := "targets:\n  zshared: &shared\n    skills:\n      path: /tmp/shared\n  claude:\n    <<: *shared\n    # about-files\n    files:\n      - a.md # line-files\n    # foot-files\n"
	got, _ := remarshalMerged(t, existing, schemaComment, func(c *Config) {
		editEntry(c.Targets, "claude", func(t *TargetConfig) { t.Files = nil })
	})
	assertCommentsOnce(t, existing, got)
	if again, _ := remarshalMerged(t, got, schemaComment, func(*Config) {}); again != got {
		t.Errorf("a second write should keep the comments on claude:\n%s\nthen:\n%s", got, again)
	}
	if strings.Index(got, "# foot-files") > strings.Index(got, "<<:") {
		t.Errorf("the comments should stay in front of the merge key, not move into the merged mapping:\n%s", got)
	}
}
