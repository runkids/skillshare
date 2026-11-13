package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/memory"
)

func TestMemoryWriters_RemoteRootLockAndReload(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, remote := range []bool{false, true} {
			for _, command := range []string{"init", "write", "delete", "show", "list", "instructions"} {
				t.Run(fmt.Sprintf("project=%t/remote=%t/%s", project, remote, command), func(t *testing.T) {
					base := t.TempDir()
					t.Setenv("HOME", base)
					t.Setenv("XDG_STATE_HOME", filepath.Join(base, "state"))
					t.Setenv("SKILLSHARE_CONFIG", filepath.Join(base, "config.yaml"))
					t.Chdir(base)
					extra := config.ExtraConfig{Name: "memory", Targets: []config.ExtraTargetConfig{}}
					if remote {
						extra.Remote, extra.Branch = "example-org/ai-hub/memory", "before"
					}
					cfg := &config.Config{Source: filepath.Join(base, "skills"), Extras: []config.ExtraConfig{extra}}
					if err := os.MkdirAll(cfg.Source, 0755); err != nil {
						t.Fatal(err)
					}
					if err := cfg.Save(); err != nil {
						t.Fatal(err)
					}
					mode, root := modeGlobal, filepath.Join(cfg.EffectiveExtrasSource(), "memory")
					if project {
						pc := &config.ProjectConfig{Extras: []config.ExtraConfig{extra}}
						if err := pc.Save(base); err != nil {
							t.Fatal(err)
						}
						mode, root = modeProject, filepath.Join(pc.EffectiveExtrasSource(base), "memory")
					}
					if err := memory.Init(root); err != nil {
						t.Fatal(err)
					}
					note, err := memory.Write(root, "note.md", "# Fixture\n", "")
					if err != nil {
						t.Fatal(err)
					}
					input := filepath.Join(base, "input.md")
					if err := os.WriteFile(input, []byte("# Updated\n"), 0644); err != nil {
						t.Fatal(err)
					}
					calls, releases := 0, 0
					publishedRoot := filepath.Join(base, "published-extras", "memory")
					old := lockRemoteExtraRoot
					lockRemoteExtraRoot = func(locked string) (func(), error) {
						calls++
						if locked != filepath.Dir(root) {
							t.Errorf("wrong root: %s", locked)
						}
						if command != "init" {
							if err := memory.Init(publishedRoot); err != nil {
								return nil, err
							}
							if _, err := memory.Write(publishedRoot, "note.md", "# Fixture\n", ""); err != nil {
								return nil, err
							}
						}
						if project {
							fresh, err := config.LoadProjectReadOnly(base)
							if err != nil {
								return nil, err
							}
							fresh.Extras[0].Branch = "published"
							fresh.Sources.Extras = "published-extras"
							if err := fresh.Save(base); err != nil {
								return nil, err
							}
						} else {
							fresh, err := config.LoadReadOnly()
							if err != nil {
								return nil, err
							}
							fresh.Extras[0].Branch = "published"
							fresh.Sources.Extras = filepath.Dir(publishedRoot)
							if err := fresh.Save(); err != nil {
								return nil, err
							}
						}
						return func() { releases++ }, nil
					}
					t.Cleanup(func() { lockRemoteExtraRoot = old })
					opts := memoryOptions{command: command, json: true}
					if command == "write" || command == "delete" || command == "show" {
						opts.path = "note.md"
					}
					if command == "write" || command == "delete" {
						opts.version = note.Version
					}
					if command == "write" {
						opts.from = input
					}
					if err := runMemory(mode, opts); err != nil {
						t.Fatal(err)
					}
					want := 0
					if remote && (command == "init" || command == "write" || command == "delete") {
						want = 1
					}
					if calls != want || releases != want {
						t.Errorf("locks/releases=%d/%d want=%d", calls, releases, want)
					}
					if want != 0 {
						var entries []config.ExtraConfig
						if project {
							fresh, err := config.LoadProjectReadOnly(base)
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
						if entries[0].Branch != "published" {
							t.Error("stale declaration overwrote publication")
						}
						if command == "init" {
							if _, err := os.Stat(filepath.Join(publishedRoot, "INDEX.md")); err != nil {
								t.Fatalf("init ignored published root: %v", err)
							}
						} else if command == "write" {
							got, err := memory.Read(publishedRoot, "note.md")
							if err != nil || got.Content != "# Updated\n" {
								t.Fatalf("write ignored published root: %+v %v", got, err)
							}
						} else if _, err := os.Stat(filepath.Join(publishedRoot, "note.md")); !os.IsNotExist(err) {
							t.Fatalf("delete ignored published root: %v", err)
						}
						got, err := memory.Read(root, "note.md")
						if err != nil || got.Content != "# Fixture\n" {
							t.Fatalf("stale root was mutated: %+v %v", got, err)
						}
					}
				})
			}
		}
	}
}
