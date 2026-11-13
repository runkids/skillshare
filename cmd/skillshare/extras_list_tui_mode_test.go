package main

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
)

func TestExtrasListTUI_SetModeRejectsSymlinkForFilteredTarget(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".skillshare"), 0755)
	os.WriteFile(filepath.Join(dir, ".skillshare", "config.yaml"), []byte(`extras:
  - name: docs
    targets:
      - path: .claude/docs
        include: [index.md]
`), 0644)
	m := extrasListTUIModel{projCfg: &config.ProjectConfig{}, cwd: dir}

	if _, err := m.doSetMode("docs", ".claude/docs", "symlink"); err == nil {
		t.Fatal("expected symlink to be refused for a target with filters")
	}
}

func TestExtrasListTUI_FlattenUsesFreshMode(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	cfg := &config.ProjectConfig{Extras: []config.ExtraConfig{{Name: "rules", Remote: "example-org/ai-hub", Targets: []config.ExtraTargetConfig{{Path: "target", Mode: "symlink"}}}}}
	if err := cfg.Save(root); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(config.ProjectConfigPath(root))
	if err != nil {
		t.Fatal(err)
	}
	model := extrasListTUIModel{projCfg: cfg, cwd: root}
	msg := model.doFlattenToggle("rules", extrasTargetInfo{Path: "target", Mode: "merge"})().(extrasActionDoneMsg)
	if msg.err == nil {
		t.Fatal("stale mode allowed flatten on current symlink")
	}
	after, err := os.ReadFile(config.ProjectConfigPath(root))
	if err != nil || string(after) != string(before) {
		t.Fatal("invalid mutation saved", err)
	}
}

func TestExtrasListTUI_CollectUsesFreshSource(t *testing.T) {
	for _, project := range []bool{false, true} {
		root := t.TempDir()
		t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
		t.Setenv("SKILLSHARE_CONFIG", filepath.Join(root, "global.yaml"))
		oldRoot, newRoot, target := filepath.Join(root, "old"), filepath.Join(root, "new"), filepath.Join(root, "target")
		if err := os.MkdirAll(target, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(target, "collected.md"), []byte("fixture"), 0644); err != nil {
			t.Fatal(err)
		}
		entry := config.ExtraConfig{Name: "rules", Remote: "example-org/ai-hub", Targets: []config.ExtraTargetConfig{{Path: target}}}
		stale := &config.Config{Source: filepath.Join(root, "skills"), Sources: config.GlobalSources{Extras: oldRoot}, Extras: []config.ExtraConfig{entry}}
		model := extrasListTUIModel{cfg: stale, cwd: root, sourceFunc: func(config.ExtraConfig) string { return filepath.Join(oldRoot, "rules") }}
		if project {
			pc := &config.ProjectConfig{Extras: []config.ExtraConfig{entry}}
			model.projCfg = pc
			pc.Sources.Extras = newRoot
			if err := pc.Save(root); err != nil {
				t.Fatal(err)
			}
		} else {
			fresh := *stale
			fresh.Sources.Extras = newRoot
			if err := fresh.Save(); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := model.doCollect("rules", target); err != nil {
			t.Fatal(err)
		}
		if data, err := os.ReadFile(filepath.Join(newRoot, "rules", "collected.md")); err != nil || string(data) != "fixture" {
			t.Fatal("fresh source unused", err)
		}
		if _, err := os.Stat(oldRoot); !os.IsNotExist(err) {
			t.Fatal("stale source written", err)
		}
	}
}
