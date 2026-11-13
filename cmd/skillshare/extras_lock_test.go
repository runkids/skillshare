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
}

func TestExtrasCollect_RemoteStaysModified(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(map[bool]string{false: "global", true: "project"}[project], func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("HOME", root)
			t.Setenv("SKILLSHARE_CONFIG", filepath.Join(root, "config.yaml"))
			t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
			entry := config.ExtraConfig{Name: "rules", Remote: "example-org/ai-hub", Branch: "team", Targets: []config.ExtraTargetConfig{{Path: filepath.Join(root, "target"), Mode: "copy"}}}
			cfg := &config.Config{Source: filepath.Join(root, "skills"), Extras: []config.ExtraConfig{entry}}
			pc := &config.ProjectConfig{Extras: []config.ExtraConfig{entry}}
			extrasRoot := cfg.EffectiveExtrasSource()
			if project {
				extrasRoot = pc.EffectiveExtrasSource(root)
				if err := pc.Save(root); err != nil {
					t.Fatal(err)
				}
			} else if err := cfg.Save(); err != nil {
				t.Fatal(err)
			}
			source := filepath.Join(extrasRoot, "rules")
			if err := os.MkdirAll(source, 0755); err != nil {
				t.Fatal(err)
			}
			original := filepath.Join(source, "original.md")
			if err := os.WriteFile(original, []byte("original"), 0644); err != nil {
				t.Fatal(err)
			}
			hash, err := utils.FileHashFormatted(original)
			if err != nil {
				t.Fatal(err)
			}
			store := install.NewMetadataStore()
			store.Set("rules", &install.MetadataEntry{Kind: "extra", Source: entry.Remote, Commit: strings.Repeat("a", 40), FileHashes: map[string]string{"original.md": hash}})
			if err := store.Save(extrasRoot); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(entry.Targets[0].Path, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(entry.Targets[0].Path, "collected.md"), []byte("local addition"), 0644); err != nil {
				t.Fatal(err)
			}
			var collectErr error
			if project {
				collectErr = extrasCollectProject(root, "rules", entry.Targets[0].Path, false, false, time.Now())
			} else {
				collectErr = extrasCollectGlobal("rules", entry.Targets[0].Path, false, false, time.Now())
			}
			if collectErr != nil {
				t.Fatal(collectErr)
			}
			state := install.InspectRemoteExtra(extrasRoot, "rules")
			if state.Err != nil || !state.Modified {
				t.Fatalf("collect did not mark local modification: %+v", state)
			}
			var entries []config.ExtraConfig
			if project {
				fresh, err := config.LoadProjectReadOnly(root)
				if err != nil {
					t.Fatal(err)
				}
				entries = fresh.Extras
			} else {
				fresh, err := config.LoadReadOnly()
				if err != nil {
					t.Fatal(err)
				}
				entries = fresh.Extras
			}
			if len(entries) != 1 || entries[0].Remote != entry.Remote || entries[0].Branch != entry.Branch {
				t.Fatalf("origin changed: %+v", entries)
			}
		})
	}
}

func TestExtrasSource_KeepsRemote(t *testing.T) {
	for _, modern := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy", true: "sources"}[modern], func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("HOME", root)
			t.Setenv("SKILLSHARE_CONFIG", filepath.Join(root, "config.yaml"))
			t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
			cfg := &config.Config{Source: filepath.Join(root, "skills"), Extras: []config.ExtraConfig{{Name: "rules", Remote: "example-org/ai-hub", Branch: "team", Targets: []config.ExtraTargetConfig{{Path: filepath.Join(root, "target")}}}}}
			if modern {
				cfg.Sources.Extras = filepath.Join(root, "before")
			}
			if err := cfg.Save(); err != nil {
				t.Fatal(err)
			}
			if err := cmdExtrasSource([]string{filepath.Join(root, "after"), "-g"}); err != nil {
				t.Fatal(err)
			}
			fresh, err := config.LoadReadOnly()
			if err != nil {
				t.Fatal(err)
			}
			if fresh.EffectiveExtrasSource() != filepath.Join(root, "after") || len(fresh.Extras) != 1 || fresh.Extras[0].Remote != cfg.Extras[0].Remote || fresh.Extras[0].Branch != "team" || len(fresh.Extras[0].Targets) != 1 {
				t.Fatalf("source move lost entries: %+v", fresh)
			}
			before, err := os.ReadFile(config.ConfigPath())
			if err != nil {
				t.Fatal(err)
			}
			if err := cmdExtrasSource([]string{filepath.Join(root, "refused"), "-p"}); err == nil {
				t.Fatal("project source accepted")
			}
			after, err := os.ReadFile(config.ConfigPath())
			if err != nil || string(before) != string(after) {
				t.Fatal("project refusal changed global config", err)
			}
		})
	}
}

// Combined with install's real onWait contention test, this pins the actual
// consumer's fresh read and source resolution after its lock boundary.
func TestExtrasWriters_ReadAfterPublication(t *testing.T) {
	t.Run("baseline", func(t *testing.T) {
		for _, writer := range []string{"add-target", "collect"} {
			t.Run(writer, func(t *testing.T) {
				root := t.TempDir()
				t.Chdir(root)
				t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
				pc := &config.ProjectConfig{Extras: []config.ExtraConfig{{Name: "rules", Remote: "example-org/ai-hub", Branch: "before", Targets: []config.ExtraTargetConfig{{Path: "target", Mode: "copy"}}}}}
				if err := pc.Save(root); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(filepath.Join(root, "target"), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "target", "collected.md"), []byte("local collection"), 0644); err != nil {
					t.Fatal(err)
				}
				entered, release := make(chan struct{}), make(chan struct{})
				old := lockRemoteExtraRoot
				lockRemoteExtraRoot = func(string) (func(), error) { close(entered); <-release; return func() {}, nil }
				done := make(chan error, 1)
				released, completed := false, false
				defer func() {
					if !released {
						close(release)
					}
					if !completed {
						select {
						case <-done:
						case <-time.After(10 * time.Second):
							t.Error("writer did not finish during cleanup")
						}
					}
					lockRemoteExtraRoot = old
				}()
				go func() {
					if writer == "add-target" {
						done <- cmdExtrasAddTarget([]string{"rules", "--add-target", "new-target", "-p"})
					} else {
						done <- extrasCollectProject(root, "rules", "target", false, false, time.Now())
					}
				}()
				select {
				case <-entered:
				case <-time.After(5 * time.Second):
					t.Fatal("writer bypassed acquisition barrier")
				}
				fresh, err := config.LoadProjectReadOnly(root)
				if err != nil {
					t.Fatal(err)
				}
				fresh.Extras[0].Branch = "published"
				if err := fresh.Save(root); err != nil {
					t.Fatal(err)
				}
				source := filepath.Join(fresh.EffectiveExtrasSource(root), "rules")
				if err := os.MkdirAll(source, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(source, "published.md"), []byte("new tree"), 0644); err != nil {
					t.Fatal(err)
				}
				close(release)
				released = true
				var writerErr error
				select {
				case writerErr = <-done:
					completed = true
				case <-time.After(10 * time.Second):
					t.Fatal("writer did not finish after release")
				}
				if err := writerErr; err != nil {
					t.Fatal(err)
				}
				fresh, err = config.LoadProjectReadOnly(root)
				if err != nil || fresh.Extras[0].Branch != "published" {
					t.Fatalf("stale entry saved: %+v %v", fresh, err)
				}
				if writer == "add-target" && len(fresh.Extras[0].Targets) != 2 {
					t.Fatal("new target lost")
				}
				if writer == "collect" {
					if data, err := os.ReadFile(filepath.Join(source, "collected.md")); err != nil || string(data) != "local collection" {
						t.Fatal("collection missed live source", err)
					}
				}
			})
		}
	})
	t.Run("TestRemoteExtraMutationReloadsAfterLock", func(t *testing.T) {
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
	})
}
