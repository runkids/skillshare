package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"skillshare/internal/config"
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

func TestProjectSyncPayload_ReportsSyncedAgentSnapshot(t *testing.T) {
	r := &ssync.ExecuteResult{AllAgents: []resource.DiscoveredResource{{RelPath: "ignored.md", Disabled: true}, {RelPath: "keep.md"}}}
	got := projectSyncPayload(r, t.TempDir())
	if got.AgentIgnoredCount != 1 || !slices.Equal(got.AgentIgnoredSkills, []string{"ignored.md"}) {
		t.Fatalf("agent ignore report = %+v", got.IgnoreReport)
	}
}
