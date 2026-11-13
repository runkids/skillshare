package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/profile"
	ssync "skillshare/internal/sync"
)

func TestDiffProfileReasons(t *testing.T) {
	for _, scenario := range []string{"global", "project", "global-include", "project-include", "global-exclude", "project-exclude"} {
		t.Run(scenario, func(t *testing.T) {
			project := strings.HasPrefix(scenario, "project")
			root := t.TempDir()
			t.Setenv("HOME", root)
			for _, key := range []string{"CODEX_HOME", "CLAUDE_CONFIG_DIR", "PI_CODING_AGENT_DIR"} {
				t.Setenv(key, "")
			}
			t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
			source, target := filepath.Join(root, "source"), filepath.Join(root, "target")
			path := filepath.Join(root, "config.yaml")
			text := fmt.Sprintf("source: %s\nmode: copy\ntargets:\n  custom:\n    skills:\n      path: %s\n      exclude: [target-only]\n", source, target)
			if project {
				path = config.ProjectConfigPath(root)
				text = fmt.Sprintf("sources:\n  skills: %s\ntargets:\n  - name: custom\n    skills:\n      path: %s\n      mode: copy\n      exclude: [target-only]\n", source, target)
			}
			t.Setenv("SKILLSHARE_CONFIG", path)
			if strings.HasSuffix(scenario, "include") {
				text = strings.ReplaceAll(text, "exclude: [target-only]", "include: [target-only]\n      exclude: [target-only]")
			} else if strings.HasSuffix(scenario, "exclude") {
				text = strings.ReplaceAll(text, "exclude: [target-only]", "exclude: [target-only, other]")
			}
			text += "profiles:\n  frontend:\n    skills:\n      include: [target-only]\n"
			for _, name := range []string{"other", "target-only"} {
				for _, dir := range []string{source, target} {
					if err := os.MkdirAll(filepath.Join(dir, name), 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(dir, name, "SKILL.md"), []byte("# Fixture"), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
			if err := ssync.WriteManifest(target, &ssync.Manifest{Managed: map[string]string{"other": "fixture", "target-only": "fixture"}}); err != nil {
				t.Fatal(err)
			}
			statePath, _, err := config.ProfilePaths(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := profile.WriteState(statePath, profile.State{Version: 1, Active: "frontend"}); err != nil {
				t.Fatal(err)
			}
			var commandErr error
			output := captureStdout(t, func() {
				if project {
					commandErr = cmdDiffProject(root, "custom", kindSkills, diffRenderOpts{jsonOutput: true}, time.Now())
				} else {
					commandErr = cmdDiffGlobal("custom", kindSkills, diffRenderOpts{jsonOutput: true}, time.Now())
				}
			})
			var out diffJSONOutput
			if err := json.Unmarshal([]byte(output), &out); err != nil || commandErr != nil || len(out.Targets) != 1 {
				t.Fatal(output, err, commandErr)
			}
			items := out.Targets[0].Items
			if len(items) != 2 {
				t.Fatal(items)
			}
			for _, item := range items {
				if item.Action != "remove" || !item.IsSync {
					t.Fatal("reason changed operation", item)
				}
				if item.Name == "other" && item.Reason != "not in profile frontend" || item.Name == "target-only" && item.Reason != "orphan (will be pruned)" {
					t.Fatal(item)
				}
			}
		})
	}
}

func TestLabelProfileDeselectedDoesNotCreateActions(t *testing.T) {
	r := targetDiffResult{items: []copyDiffEntry{
		{action: "remove", name: "local", reason: "local only"},
		{action: "modify", name: "edited", reason: "local copy (sync --force to replace)", isSync: true},
	}, syncCount: 1, localCount: 1}
	labelProfileDeselected(&r, map[string]string{"local": "frontend", "edited": "frontend", "absent": "frontend"})
	if len(r.items) != 2 || r.syncCount != 1 || r.localCount != 1 || r.items[0].reason != "local only" || r.items[1].reason != "local copy (sync --force to replace)" {
		t.Fatal("diagnostic changed actions/counts or force affordance", r)
	}
}

func TestDiffProfileDisambiguation(t *testing.T) {
	for _, scope := range []string{"global", "project"} {
		for _, scenario := range []string{"standard/include/collision", "standard/exclude/collision", "prefixed/include/collision", "prefixed/exclude/collision", "standard/include/variant", "standard/exclude/variant", "prefixed/include/variant", "prefixed/exclude/variant"} {
			t.Run(scope+"/"+scenario, func(t *testing.T) {
				parts := strings.Split(scenario, "/")
				naming, filter := parts[0], parts[1]
				root := t.TempDir()
				t.Setenv("HOME", root)
				for _, key := range []string{"CODEX_HOME", "CLAUDE_CONFIG_DIR", "PI_CODING_AGENT_DIR"} {
					t.Setenv(key, "")
				}
				t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
				t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
				source, target := filepath.Join(root, "source"), filepath.Join(root, "target")
				selected, rejected, want := "a/review", "b/review", "review"
				if parts[2] == "variant" {
					selected, rejected = "_repo/skills/claude/review", "_repo/skills/claude-code/review"
					writeSkill(t, filepath.Join(source, "_repo/skills/codex"), "review")
					if naming == "prefixed" {
						want = "repo-review"
					}
				}
				for _, rel := range []string{selected, rejected} {
					writeSkill(t, filepath.Join(source, filepath.Dir(rel)), "review")
				}
				sc := config.ResourceTargetConfig{Path: target, Mode: "copy", TargetNaming: naming}
				pattern := strings.ReplaceAll(selected, "/", "__")
				if filter == "include" {
					sc.Include = []string{pattern}
				} else {
					pattern = strings.ReplaceAll(rejected, "/", "__")
					sc.Exclude = []string{pattern}
				}
				path := filepath.Join(root, "config.yaml")
				text := fmt.Sprintf("source: %s\nmode: copy\ntargets:\n  claude:\n    skills:\n      path: %s\n      target_naming: %s\n      %s: [%s]\n", source, target, naming, filter, pattern)
				if scope == "project" {
					path = config.ProjectConfigPath(root)
					text = fmt.Sprintf("sources:\n  skills: %s\ntargets:\n  - name: claude\n    skills:\n      path: %s\n      mode: copy\n      target_naming: %s\n      %s: [%s]\n", source, target, naming, filter, pattern)
				}
				t.Setenv("SKILLSHARE_CONFIG", path)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(text+"profiles:\n  empty:\n    skills:\n      include: []\n"), 0600); err != nil {
					t.Fatal(err)
				}
				skills, err := ssync.DiscoverSourceSkills(source)
				if err != nil {
					t.Fatal(err)
				}
				if parts[2] == "variant" {
					for _, skill := range skills {
						if skill.LogicalID == "" || !skill.UsesVariantRouting() {
							t.Fatalf("fixture did not discover actual variants: %+v", skill)
						}
					}
				}
				publication := ssync.SyncSkillTarget(ssync.SkillTarget{Name: "claude", Target: config.TargetConfig{Skills: &sc}, Mode: "copy"}, skills, ssync.SkillRunOptions{Source: source})
				if publication.Err != nil || publication.PruneErr != nil || len(publication.Linked) != 1 || publication.Linked[0] != want {
					t.Fatalf("managed publication: %+v", publication)
				}
				manifest, err := ssync.ReadManifest(target)
				if err != nil || len(manifest.Managed) != 1 || manifest.Managed[want] == "" {
					t.Fatalf("expected one real managed target entry: %+v, %v", manifest, err)
				}
				statePath, _, err := config.ProfilePaths(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := profile.WriteState(statePath, profile.State{Version: 1, Active: "empty"}); err != nil {
					t.Fatal(err)
				}
				var commandErr error
				output := captureStdout(t, func() {
					if scope == "project" {
						commandErr = cmdDiffProject(root, "claude", kindSkills, diffRenderOpts{jsonOutput: true}, time.Now())
					} else {
						commandErr = cmdDiffGlobal("claude", kindSkills, diffRenderOpts{jsonOutput: true}, time.Now())
					}
				})
				var out diffJSONOutput
				if err := json.Unmarshal([]byte(output), &out); err != nil || commandErr != nil || len(out.Targets) != 1 || len(out.Targets[0].Items) != 1 {
					t.Fatal(output, err, commandErr)
				}
				item := out.Targets[0].Items[0]
				if out.Targets[0].Synced || item.Name != want || item.Action != "remove" || !item.IsSync || item.Reason != "not in profile empty" {
					t.Fatalf("filtered managed removal lost identity/action/count/reason: %+v", out.Targets[0])
				}
			})
		}
	}
}
