package config

import (
	"os"
	"path/filepath"
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
	data, err := marshalConfig(&cfg, schemaComment)
	if err != nil {
		t.Fatal(err)
	}
	var reloaded Config
	if err := yaml.Unmarshal(data, &reloaded); err != nil {
		t.Errorf("saved config does not load: %v\n%s", err, data)
	}
}
