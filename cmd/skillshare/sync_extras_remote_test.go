package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"skillshare/internal/config"
)

func TestSyncExtras_RemoteMissingGuidance(t *testing.T) {
	for _, project := range []bool{false, true} {
		root := t.TempDir()
		t.Setenv("SKILLSHARE_CONFIG", filepath.Join(root, "config.yaml"))
		entry := config.ExtraConfig{Name: "rules", Remote: "example-org/ai-hub", Branch: "main"}
		var run func(bool) error
		if project {
			cfg := &config.ProjectConfig{Extras: []config.ExtraConfig{entry}}
			if err := cfg.Save(root); err != nil {
				t.Fatal(err)
			}
			run = func(json bool) error { return cmdSyncExtrasProject(root, false, false, json, false, time.Now()) }
		} else {
			cfg := &config.Config{Source: filepath.Join(root, "skills"), Extras: []config.ExtraConfig{entry}}
			if err := os.MkdirAll(cfg.Source, 0755); err != nil {
				t.Fatal(err)
			}
			if err := cfg.Save(); err != nil {
				t.Fatal(err)
			}
			run = func(json bool) error { return cmdSyncExtrasGlobal(false, false, json, false, time.Now()) }
		}
		command := config.ExtraRestoreCommand(entry, project)
		plain := captureStdout(t, func() {
			if err := run(false); err != nil {
				t.Fatal(err)
			}
		})
		if !strings.Contains(plain, command) {
			t.Fatal(plain)
		}
		jsonText := captureStdout(t, func() {
			if err := run(true); err != nil {
				t.Fatal(err)
			}
		})
		var out syncExtrasJSONOutput
		if err := json.Unmarshal([]byte(jsonText), &out); err != nil || len(out.Extras) != 1 || len(out.Extras[0].Warnings) != 1 || !strings.Contains(out.Extras[0].Warnings[0], command) {
			t.Fatal(jsonText, err)
		}
		entries, err := runExtrasSyncEntries([]config.ExtraConfig{entry}, func(config.ExtraConfig) string { return filepath.Join(root, "missing") }, false, false, map[bool]string{true: root}[project], nil)
		if err != nil || len(entries) != 1 || len(entries[0].Warnings) != 1 || !strings.Contains(entries[0].Warnings[0], command) {
			t.Fatal(entries, err)
		}
	}
}
