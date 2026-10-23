package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/projects"
	"skillshare/internal/resource"
	ssync "skillshare/internal/sync"
)

// projectSyncFixture writes a global config and keeps backups and state in the test.
func projectSyncFixture(t *testing.T, body func(source string) string) (path, dir string) {
	t.Helper()
	dir = t.TempDir()
	t.Setenv("HOME", filepath.Join(dir, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	source := filepath.Join(dir, "source")
	if err := os.MkdirAll(filepath.Join(source, "alpha"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "alpha", "SKILL.md"), []byte("---\nname: alpha\n---\n# Alpha\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"app", "other"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	path = filepath.Join(dir, "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", path)
	if err := os.WriteFile(path, []byte(body(source)), 0644); err != nil {
		t.Fatal(err)
	}
	return path, dir
}

func TestSyncProject_OnlySelectedTargets(t *testing.T) {
	path, dir := projectSyncFixture(t, func(source string) string {
		root := filepath.ToSlash(filepath.Dir(source))
		return "source: " + filepath.ToSlash(source) + "\ntargets: {}\nprojects:\n  " + root + "/app:\n    targets: [claude]\n    skills: {mode: copy}\n  " + root + "/other:\n    targets: [claude]\n    skills: {mode: copy}\n"
	})
	root := filepath.Join(dir, "app")
	cfg, err := config.LoadReadOnlyPath(path)
	if err != nil {
		t.Fatal(err)
	}
	key := filepath.ToSlash(root)
	synced, err := syncProject(cfg, key, false)
	if err != nil {
		t.Fatal(err)
	}
	if synced == nil || len(synced.Results) != 1 || synced.Results[0].Target != "app@claude" || len(synced.Failed) != 0 {
		t.Fatalf("sync selected wrong targets: %+v", synced)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "skills", "alpha", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "other", ".claude")); !os.IsNotExist(err) {
		t.Fatalf("other project was touched: %v", err)
	}
	if err := os.Rename(root, root+"-gone"); err != nil {
		t.Fatal(err)
	}
	if _, err := syncProject(cfg, key, false); !errors.Is(err, projects.ErrFolderNotFound) {
		t.Fatalf("missing root silently synced: %v", err)
	}
}

// --sync --dry-run previews the unsaved candidate, including renamed targets.
func TestSyncProject_UnsavedRenamedCandidate(t *testing.T) {
	path, dir := projectSyncFixture(t, func(source string) string {
		return "source: " + filepath.ToSlash(source) + "\ntargets: {}\nprojects:\n  " + filepath.ToSlash(filepath.Join(filepath.Dir(source), "app")) + ":\n    targets: [claude, codex]\n    skills: {}\n"
	})
	root := filepath.Join(dir, "app")
	key := filepath.ToSlash(root)
	cfg, err := config.LoadReadOnlyPath(path)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := projects.PrepareProjects(path, cfg, func(p map[string]config.ManagedProject) error {
		entry := p[key]
		entry.Name, entry.Targets = "renamed", []string{"claude"}
		p[key] = entry
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := syncProject(candidate.Save.Expanded, key, true)
	if err != nil {
		t.Fatal(err)
	}
	if preview == nil || len(preview.Results) != 1 || preview.Results[0].Target != "renamed@claude" {
		t.Fatalf("sync used stale target names/groups: %+v", preview)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude")); !os.IsNotExist(err) {
		t.Fatalf("preview created target: %v", err)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, before) {
		t.Fatal("sync preview changed config")
	}
}

func TestSyncProject_DeclaredResources(t *testing.T) {
	for _, tc := range []struct {
		name   string
		agents bool
	}{
		{"agents only", true}, {"both disabled", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path, dir := projectSyncFixture(t, func(source string) string {
				raw := "source: " + filepath.ToSlash(source) + "\nmode: symlink\ntargets: {}\nprojects:\n  " + filepath.ToSlash(filepath.Join(filepath.Dir(source), "app")) + ":\n    targets: [claude]\n"
				if tc.agents {
					raw += "    agents: {}\n"
				}
				return raw
			})
			root := filepath.Join(dir, "app")
			cfg, err := config.LoadReadOnlyPath(path)
			if err != nil {
				t.Fatal(err)
			}
			synced, err := syncProject(cfg, filepath.ToSlash(root), false)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(filepath.Join(root, ".claude", "skills")); !os.IsNotExist(err) {
				t.Fatalf("skills-off project wrote skills: %v %+v", err, synced)
			}
			if synced == nil {
				t.Fatal("missing empty sync result")
			}
			if !tc.agents && len(synced.Results) > 0 {
				t.Fatalf("project with nothing enabled executed: %+v", synced)
			}
		})
	}
}

// Failed targets must not hide completed targets in the operations log.
func TestProjectSyncLogStatus(t *testing.T) {
	for _, action := range []string{"sync", "add", "edit"} {
		for _, tc := range []struct {
			name, targets, resources string
			flags                    []string
			blocked                  []string
			status                   string
		}{
			{"success", "claude,cursor", "skills: {mode: copy}", []string{"--skills-mode=copy"}, nil, "ok"},
			{"partial", "claude,cursor", "skills: {mode: copy}", []string{"--skills-mode=copy"}, []string{".claude"}, "partial"},
			{"all failed", "claude,cursor", "skills: {mode: copy}", []string{"--skills-mode=copy"}, []string{".claude", ".agents"}, "error"},
			{"agents only with unsupported tool", "claude,codex", "agents: {mode: copy}", []string{"--no-skills", "--agents-mode=copy"}, []string{".claude"}, "error"},
			{"agents with skills disabled", "claude,codex", "skills: {enabled: false}\n    agents: {mode: copy}", nil, []string{".claude"}, "error"},
			{"agents partial", "claude,cursor", "agents: {mode: copy}", []string{"--no-skills", "--agents-mode=copy"}, []string{".claude"}, "partial"},
			{"skills and agents fail same target", "claude,cursor", "skills: {mode: copy}\n    agents: {mode: copy}", []string{"--skills-mode=copy", "--agents-mode=copy"}, []string{".claude"}, "partial"},
		} {
			// add has no flag that preserves an explicitly disabled skills object.
			if action == "add" && tc.flags == nil {
				continue
			}
			t.Run(action+"/"+tc.name, func(t *testing.T) {
				path, dir := projectSyncFixture(t, func(source string) string {
					root := filepath.ToSlash(filepath.Join(filepath.Dir(source), "app"))
					return fmt.Sprintf("source: %q\nagents_source: %q\ntargets: {}\nprojects:\n  %q:\n    targets: [%s]\n    %s\n", filepath.ToSlash(source), filepath.ToSlash(filepath.Join(filepath.Dir(source), "agents")), root, tc.targets, tc.resources)
				})
				root := filepath.Join(dir, "app")
				agents := filepath.Join(dir, "agents")
				if err := os.MkdirAll(agents, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(agents, "review.md"), []byte("---\nname: review\n---\n# Review\n"), 0644); err != nil {
					t.Fatal(err)
				}
				for _, tool := range tc.blocked {
					if err := os.WriteFile(filepath.Join(root, tool), []byte("local file"), 0644); err != nil {
						t.Fatal(err)
					}
				}
				args := []string{action, root, "--json"}
				if action != "sync" {
					args = append(args, "--sync")
				}
				if action == "add" {
					args = append(args, "--target="+tc.targets)
					args = append(args, tc.flags...)
				}
				var syncErr error
				output := captureStdout(t, func() { syncErr = cmdProject(args) })
				if (syncErr != nil) != (tc.status != "ok") {
					t.Fatalf("sync error = %v, want status %s", syncErr, tc.status)
				}
				if tc.name == "agents only with unsupported tool" {
					var result struct {
						Sync struct{ Results, Failed []any } `json:"sync"`
					}
					if err := json.Unmarshal([]byte(output), &result); err != nil {
						t.Fatal(err)
					}
					if len(result.Sync.Results) != 0 || len(result.Sync.Failed) != 1 {
						t.Fatalf("want one failed runnable target and no completed work: %s", output)
					}
				}
				entries, err := oplog.Read(path, oplog.OpsFile, 1)
				if err != nil || len(entries) != 1 {
					t.Fatalf("entries = %+v, err = %v", entries, err)
				}
				if got := entries[0]; got.Command != "sync" || got.Status != tc.status {
					t.Fatalf("log = %+v, want sync %s", got, tc.status)
				}
			})
		}
	}
}

func TestSyncProject_ReturnsDiscoveredAgents(t *testing.T) {
	path, dir := projectSyncFixture(t, func(source string) string {
		return "source: " + filepath.ToSlash(source) + "\ntargets: {}\nprojects:\n  " + filepath.ToSlash(filepath.Join(filepath.Dir(source), "app")) + ":\n    targets: [claude]\n    agents: {}\n"
	})
	cfg, err := config.LoadReadOnlyPath(path)
	if err != nil {
		t.Fatal(err)
	}
	agents := t.TempDir()
	cfg.AgentsSource = agents
	for _, name := range []string{"keep.md", "ignored.md"} {
		if err := os.WriteFile(filepath.Join(agents, name), []byte("# Agent\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(agents, ".agentignore"), []byte("ignored.md\n"), 0644); err != nil {
		t.Fatal(err)
	}
	synced, err := syncProject(cfg, filepath.ToSlash(filepath.Join(dir, "app")), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(synced.AllAgents) != 2 || !slices.ContainsFunc(synced.AllAgents, func(a resource.DiscoveredResource) bool {
		return a.RelPath == "ignored.md" && a.Disabled
	}) {
		t.Fatalf("sync discarded the full agent snapshot: %+v", synced.AllAgents)
	}
	// Rendering must keep the synced snapshot even if the source changes.
	if err := os.Remove(filepath.Join(agents, "ignored.md")); err != nil {
		t.Fatal(err)
	}
	output := captureStdout(t, func() {
		err = renderProjectResult(projectOptions{action: "sync", json: true}, projectResult{
			Result: projects.Result{Expanded: cfg}, Sync: synced,
		}, nil, time.Now())
	})
	if err != nil {
		t.Fatal(err)
	}
	var got projectResultJSON
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatal(err)
	}
	if got.Sync == nil || got.Sync.AgentIgnoredCount != 1 || !slices.Equal(got.Sync.AgentIgnoredSkills, []string{"ignored.md"}) {
		t.Fatalf("project JSON lost the synced agent snapshot: %s", output)
	}
}

func TestProjectSyncPayload_ReportsSyncedAgentSnapshot(t *testing.T) {
	r := &ssync.ExecuteResult{AllAgents: []resource.DiscoveredResource{{RelPath: "ignored.md", Disabled: true}, {RelPath: "keep.md"}}}
	got := projectSyncPayload(r, t.TempDir())
	if got.AgentIgnoredCount != 1 || !slices.Equal(got.AgentIgnoredSkills, []string{"ignored.md"}) {
		t.Fatalf("agent ignore report = %+v", got.IgnoreReport)
	}
}
