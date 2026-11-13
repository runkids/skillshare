package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/utils"
)

func TestRemoteExtraWritersTakeRootLock(t *testing.T) {
	for _, remote := range []bool{false, true} {
		for _, writer := range []string{"add-target", "remove-target", "mode", "collect", "tui-mode", "tui-flatten", "tui-collect"} {
			t.Run(writer+map[bool]string{false: "/local", true: "/remote"}[remote], func(t *testing.T) {
				root := t.TempDir()
				t.Setenv("HOME", root)
				t.Setenv("SKILLSHARE_CONFIG", filepath.Join(root, "global.yaml"))
				t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
				pc := &config.ProjectConfig{Extras: []config.ExtraConfig{{Name: "rules", Targets: []config.ExtraTargetConfig{{Path: "target-one"}, {Path: "target-two"}}}}}
				if remote {
					pc.Extras[0].Remote = "example-org/ai-hub"
					pc.Extras[0].Branch = "main"
				}
				if err := pc.Save(root); err != nil {
					t.Fatal(err)
				}
				source := filepath.Join(pc.EffectiveExtrasSource(root), "rules")
				if err := os.MkdirAll(source, 0755); err != nil {
					t.Fatal(err)
				}
				for _, target := range []string{"target-one", "target-two"} {
					if err := os.Mkdir(filepath.Join(root, target), 0755); err != nil {
						t.Fatal(err)
					}
				}
				called := 0
				old := lockRemoteExtraRoot
				lockRemoteExtraRoot = func(path string) (func(), error) {
					called++
					if path != pc.EffectiveExtrasSource(root) {
						t.Fatalf("wrong root %q", path)
					}
					return func() {}, nil
				}
				defer func() { lockRemoteExtraRoot = old }()
				// Chdir is per-test only; none of these cases run in parallel.
				t.Chdir(root)
				var err error
				model := extrasListTUIModel{projCfg: pc, cwd: root, sourceFunc: func(e config.ExtraConfig) string {
					return config.ResolveExtrasSourceDirProject(e, pc.EffectiveExtrasSource(root), root)
				}}
				switch writer {
				case "add-target":
					err = cmdExtrasAddTarget([]string{"rules", "--add-target", "target-new", "-p"})
				case "remove-target":
					err = cmdExtrasRemoveTarget([]string{"rules", "--remove-target", "target-two", "-p"})
				case "mode":
					err = cmdExtrasMode([]string{"rules", "--mode", "copy", "--target", "target-one", "-p"})
				case "collect":
					err = extrasCollectProject(root, "rules", "target-one", false, false, time.Now())
				case "tui-mode":
					_, err = model.doSetMode("rules", "target-one", "copy")
				case "tui-flatten":
					msg := model.doFlattenToggle("rules", extrasTargetInfo{Path: "target-one", Mode: "merge"})()
					err = msg.(extrasActionDoneMsg).err
				case "tui-collect":
					_, err = model.doCollect("rules", "target-one")
				}
				if err != nil {
					t.Fatal(err)
				}
				want := 0
				if remote {
					want = 1
				}
				if called != want {
					t.Fatalf("root locks=%d, want %d", called, want)
				}
				cfg, err := config.LoadProjectReadOnly(root)
				if err != nil {
					t.Fatal(err)
				}
				if cfg.Extras[0].Remote != pc.Extras[0].Remote || cfg.Extras[0].Branch != pc.Extras[0].Branch {
					t.Fatal("origin lost")
				}
			})
		}
	}
}

func TestRemoteExtraMutationReloadsAfterLock(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	pc := &config.ProjectConfig{Extras: []config.ExtraConfig{{Name: "rules", Remote: "example-org/ai-hub", Branch: "before", Targets: []config.ExtraTargetConfig{}}}}
	if err := pc.Save(root); err != nil {
		t.Fatal(err)
	}
	old := lockRemoteExtraRoot
	lockRemoteExtraRoot = func(string) (func(), error) {
		fresh, err := config.LoadProjectReadOnly(root)
		if err != nil {
			t.Fatal(err)
		}
		fresh.Extras[0].Branch = "published"
		if err := fresh.Save(root); err != nil {
			t.Fatal(err)
		}
		return func() {}, nil
	}
	defer func() { lockRemoteExtraRoot = old }()
	if err := cmdExtrasAddTarget([]string{"rules", "--add-target", "target", "-p"}); err != nil {
		t.Fatal(err)
	}
	fresh, err := config.LoadProjectReadOnly(root)
	if err != nil || fresh.Extras[0].Branch != "published" || len(fresh.Extras[0].Targets) != 1 {
		t.Fatalf("stale config saved: %+v %v", fresh, err)
	}
}

func TestExtrasTUI_RemoveRemote(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	pc := &config.ProjectConfig{Extras: []config.ExtraConfig{{Name: "rules", Remote: "example-org/ai-hub", Targets: []config.ExtraTargetConfig{}}}}
	if err := pc.Save(root); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(pc.EffectiveExtrasSource(root), "rules")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(source, "rule.md")
	if err := os.WriteFile(file, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	hash, err := utils.FileHashFormatted(file)
	if err != nil {
		t.Fatal(err)
	}
	store := install.NewMetadataStore()
	store.Set("rules", &install.MetadataEntry{Kind: "extra", Source: "example-org/ai-hub", Commit: strings.Repeat("a", 40), FileHashes: map[string]string{"rule.md": hash}})
	if err := store.Save(pc.EffectiveExtrasSource(root)); err != nil {
		t.Fatal(err)
	}
	model := extrasListTUIModel{projCfg: pc, cwd: root}
	msg, err := model.doRemove("rules")
	if err != nil || !strings.Contains(msg, "Deleted") || !strings.Contains(msg, "aaaaaaa") {
		t.Fatalf("TUI removal: %q %v", msg, err)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("source remains: %v", err)
	}
}
