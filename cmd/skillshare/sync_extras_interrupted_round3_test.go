package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"skillshare/internal/config"
)

func TestSyncExtras_InterruptedVisibilityCLIAndTUI(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, action := range []string{"plain", "json", "all-json", "tui"} {
			t.Run(fmt.Sprintf("project=%t/%s", project, action), func(t *testing.T) {
				base := t.TempDir()
				t.Setenv("HOME", base)
				t.Setenv("SKILLSHARE_CONFIG", filepath.Join(base, "config.yaml"))
				t.Setenv("XDG_STATE_HOME", filepath.Join(base, "state"))
				entry := config.ExtraConfig{Name: "rules", Remote: "file://" + filepath.Join(base, "unavailable-remote"), Branch: "main", Targets: []config.ExtraTargetConfig{{Path: filepath.Join(base, "target"), Mode: "copy"}}}
				cfg := &config.Config{Source: filepath.Join(base, "skills"), Extras: []config.ExtraConfig{entry}}
				pc := &config.ProjectConfig{Extras: []config.ExtraConfig{entry}}
				if err := os.MkdirAll(cfg.Source, 0755); err != nil {
					t.Fatal(err)
				}
				var model extrasListTUIModel
				root := cfg.EffectiveExtrasSource()
				if project {
					if err := pc.Save(base); err != nil {
						t.Fatal(err)
					}
					root = pc.EffectiveExtrasSource(base)
					model = extrasListTUIModel{projCfg: pc, cwd: base}
				} else {
					if err := cfg.Save(); err != nil {
						t.Fatal(err)
					}
					model = extrasListTUIModel{cfg: cfg, cwd: base}
				}
				source := filepath.Join(root, "rules")
				stage := filepath.Join(root, ".skillshare-staging")
				for _, dir := range []string{source, stage} {
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.WriteFile(filepath.Join(source, "rule.md"), []byte("candidate at live boundary"), 0644); err != nil {
					t.Fatal(err)
				}
				marker := filepath.Join(stage, "publishing-rules.json")
				data, err := json.Marshal(map[string]any{"version": 1, "name": "rules", "remote": entry.Remote, "branch": "main", "commit": strings.Repeat("a", 40), "had_previous": false, "old": nil})
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(marker, data, 0600); err != nil {
					t.Fatal(err)
				}
				// Existing mutation seam must remain unused by ordinary sync.
				old := lockRemoteExtraRoot
				lockRemoteExtraRoot = func(string) (func(), error) { t.Error("sync acquired mutation lock"); return func() {}, nil }
				t.Cleanup(func() { lockRemoteExtraRoot = old })
				var runErr error
				output := captureStdout(t, func() {
					switch action {
					case "tui":
						var message string
						message, runErr = model.doSync("rules", "")
						fmt.Print(message)
					case "all-json":
						projectRoot := ""
						if project {
							projectRoot = base
						}
						entries, err := runExtrasSyncEntries([]config.ExtraConfig{entry}, func(config.ExtraConfig) string { return source }, false, false, projectRoot, nil)
						runErr = err
						encoded, err := json.Marshal(entries)
						if err != nil {
							t.Fatal(err)
						}
						fmt.Print(string(encoded))
					default:
						if project {
							runErr = cmdSyncExtrasProject(base, false, false, action == "json", false, time.Now())
						} else {
							runErr = cmdSyncExtrasGlobal(false, false, action == "json", false, time.Now())
						}
					}
				})
				messages := fmt.Sprint(output, runErr)
				if action == "json" || action == "all-json" {
					var entries []syncExtrasJSONEntry
					if action == "json" {
						var decoded syncExtrasJSONOutput
						if err := json.Unmarshal([]byte(output), &decoded); err != nil {
							t.Fatal(err)
						}
						entries = decoded.Extras
					} else if err := json.Unmarshal([]byte(output), &entries); err != nil {
						t.Fatal(err)
					}
					messages = fmt.Sprint(runErr)
					for _, entry := range entries {
						messages += strings.Join(entry.Warnings, "\n")
						for _, target := range entry.Targets {
							messages += target.Error + strings.Join(target.Warnings, "\n")
						}
					}
				}
				if !regexp.MustCompile(`(?i)\binterrupted\b`).MatchString(messages) {
					t.Errorf("sync hides interrupted provenance: output=%q err=%v", output, runErr)
				}
				if after, err := os.ReadFile(marker); err != nil || string(after) != string(data) {
					t.Errorf("ordinary sync changed publication marker: %q %v", after, err)
				}
			})
		}
	}
}
