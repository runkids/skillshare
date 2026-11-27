package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"skillshare/internal/config"
)

func TestExtrasTUI_TargetlessInterruptedVisibility(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, state := range []string{"remote-marker", "remote-ordinary", "local-marker"} {
			t.Run(fmt.Sprintf("project=%t/%s", project, state), func(t *testing.T) {
				base := t.TempDir()
				t.Setenv("HOME", base)
				t.Setenv("SKILLSHARE_CONFIG", filepath.Join(base, "config.yaml"))
				t.Setenv("XDG_STATE_HOME", filepath.Join(base, "state"))
				entry := config.ExtraConfig{Name: "rules", Targets: []config.ExtraTargetConfig{}}
				if state != "local-marker" {
					entry.Remote, entry.Branch = "example-org/hub/rules", "main"
				}
				cfg := &config.Config{Source: filepath.Join(base, "skills"), Extras: []config.ExtraConfig{entry}}
				pc := &config.ProjectConfig{Extras: []config.ExtraConfig{entry}}
				if err := os.MkdirAll(cfg.Source, 0755); err != nil {
					t.Fatal(err)
				}
				model := extrasListTUIModel{cfg: cfg, cwd: base}
				root := cfg.EffectiveExtrasSource()
				if project {
					if err := pc.Save(base); err != nil {
						t.Fatal(err)
					}
					model.projCfg, root = pc, pc.EffectiveExtrasSource(base)
				} else if err := cfg.Save(); err != nil {
					t.Fatal(err)
				}
				source := filepath.Join(root, "rules")
				if err := os.MkdirAll(source, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(source, "rule.md"), []byte("live candidate"), 0644); err != nil {
					t.Fatal(err)
				}
				if state != "remote-ordinary" {
					stage := filepath.Join(root, ".skillshare-staging")
					if err := os.MkdirAll(stage, 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(stage, "publishing-rules.json"), []byte(`{"version":1,"name":"rules","remote":"example-org/hub/rules","branch":"main","commit":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","had_previous":false,"old":null}`), 0600); err != nil {
						t.Fatal(err)
					}
				}
				// Snapshot all fixture records, source bytes and directory presence.
				snapshot := func() map[string]string {
					files := map[string]string{}
					err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
						if err != nil {
							return err
						}
						if d.IsDir() {
							files[path] = "directory"
							return nil
						}
						data, err := os.ReadFile(path)
						files[path] = string(data)
						return err
					})
					if err != nil {
						t.Fatal(err)
					}
					return files
				}
				before := snapshot()
				old := lockRemoteExtraRoot
				lockRemoteExtraRoot = func(string) (func(), error) {
					t.Error("ordinary TUI sync acquired mutation lock")
					return func() {}, nil
				}
				t.Cleanup(func() { lockRemoteExtraRoot = old })
				message, err := model.doSync("rules", "")
				if state == "remote-marker" {
					if err == nil || !strings.Contains(err.Error(), "has an interrupted publication") {
						t.Errorf("targetless TUI hides interruption: message=%q err=%v", message, err)
					}
				} else if err != nil || !strings.Contains(message, "0 target(s)") {
					t.Errorf("ordinary/local targetless behavior changed: message=%q err=%v", message, err)
				}
				if after := snapshot(); !reflect.DeepEqual(before, after) {
					t.Error("targetless TUI sync changed fixture files, records or directories")
				}
			})
		}
	}
}
