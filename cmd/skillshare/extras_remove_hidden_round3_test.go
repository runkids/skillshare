package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/install"
	syncpkg "skillshare/internal/sync"
	"skillshare/internal/utils"
)

func TestExtrasRemove_HiddenOwnedLinksCLIAndTUI(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX symlink fixture; no native Windows assertion")
	}
	for _, project := range []bool{false, true} {
		for _, remote := range []bool{false, true} {
			for _, tui := range []bool{false, true} {
				t.Run(fmt.Sprintf("project=%t/remote=%t/tui=%t", project, remote, tui), func(t *testing.T) {
					base := t.TempDir()
					t.Setenv("HOME", base)
					t.Setenv("SKILLSHARE_CONFIG", filepath.Join(base, "config.yaml"))
					t.Setenv("XDG_STATE_HOME", filepath.Join(base, "state"))
					target := filepath.Join(base, "target")
					entry := config.ExtraConfig{Name: "rules", Targets: []config.ExtraTargetConfig{{Path: target, Mode: "merge"}}}
					if remote {
						entry.Remote = "example-org/ai-hub"
					}
					cfg := &config.Config{Source: filepath.Join(base, "skills"), Extras: []config.ExtraConfig{entry}}
					if err := os.MkdirAll(cfg.Source, 0755); err != nil {
						t.Fatal(err)
					}
					pc := &config.ProjectConfig{Extras: []config.ExtraConfig{entry}}
					root := cfg.EffectiveExtrasSource()
					model := extrasListTUIModel{cfg: cfg, cwd: base}
					if project {
						root = pc.EffectiveExtrasSource(base)
						model = extrasListTUIModel{projCfg: pc, cwd: base}
						if err := pc.Save(base); err != nil {
							t.Fatal(err)
						}
					} else if err := cfg.Save(); err != nil {
						t.Fatal(err)
					}
					source := filepath.Join(root, "rules")
					hashes := map[string]string{}
					// Full hidden ownership policy lives in extraops; this is the real adapter witness.
					paths := []string{".nested/.inside.md"}
					for _, name := range paths {
						path := filepath.Join(source, name)
						if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(path, []byte("owned fixture"), 0644); err != nil {
							t.Fatal(err)
						}
						hash, err := utils.FileHashFormatted(path)
						if err != nil {
							t.Fatal(err)
						}
						hashes[filepath.ToSlash(name)] = hash
					}
					if remote {
						store := install.NewMetadataStore()
						store.Set("rules", &install.MetadataEntry{Kind: "extra", Source: entry.Remote, Commit: strings.Repeat("a", 40), FileHashes: hashes})
						if err := store.Save(root); err != nil {
							t.Fatal(err)
						}
					}
					result, err := syncpkg.SyncExtraTarget(entry, entry.Targets[0], source, target, "merge", nil, syncpkg.ExtraSyncOptions{})
					if err != nil || len(result.Errors) != 0 {
						t.Fatalf("supported hidden sync fixture: %+v %v", result, err)
					}
					for _, name := range paths {
						if info, err := os.Lstat(filepath.Join(target, name)); err != nil || info.Mode()&os.ModeSymlink == 0 {
							t.Fatalf("hidden sync did not create owned link %s: %v", name, err)
						}
					}
					foreign := filepath.Join(target, ".foreign.md")
					if err := os.WriteFile(foreign, []byte("keep foreign"), 0644); err != nil {
						t.Fatal(err)
					}
					foreignLink := filepath.Join(target, ".foreign-link.md")
					if err := os.Symlink(foreign, foreignLink); err != nil {
						t.Fatal(err)
					}
					if tui {
						_, err = model.doRemove("rules")
					} else if project {
						err = extrasRemoveProject(base, "rules", true, time.Now())
					} else {
						err = extrasRemoveGlobal("rules", true, time.Now())
					}
					if err != nil {
						t.Fatal(err)
					}
					for _, name := range paths {
						_, err := os.Lstat(filepath.Join(target, name))
						if remote && !os.IsNotExist(err) || !remote && err != nil {
							t.Errorf("hidden cleanup remote=%t path=%s: %v", remote, name, err)
						}
					}
					if body, err := os.ReadFile(foreignLink); err != nil || string(body) != "keep foreign" {
						t.Errorf("foreign hidden content/link changed: %q %v", body, err)
					}
					_, err = os.Stat(source)
					if remote && !os.IsNotExist(err) || !remote && err != nil {
						t.Errorf("source removal remote=%t: %v", remote, err)
					}
				})
			}
		}
	}
}
