//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

// The command contract deliberately differs for read discovery and mutations.
// Server rows for the same five-entry layout live in handler_skillfollow_test.go.
func TestSkillfollowBehaviorMatrix(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	external := filepath.Join(sb.Root, "external")
	for _, rel := range []string{"repo/a", "group/c", "hidden/secret"} {
		path := filepath.Join(external, rel)
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte("---\nname: "+filepath.Base(rel)+"\ndescription: Fixture\n---\n# Fixture\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	testutil.RunGit(t, external, "init", "repo")
	for name, target := range map[string]string{"_repo": "repo", "group": "group", "undeclared": "hidden"} {
		if err := os.Symlink(filepath.Join(external, target), filepath.Join(sb.SourcePath, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(sb.SourcePath, "rejected"), []byte("not a directory"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sb.SourcePath, ".skillfollow"), []byte("_repo\ngroup\nmissing\nrejected\n"), 0644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(sb.Home, ".claude", "skills")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	sb.WriteConfig("source: " + sb.SourcePath + "\nmode: merge\ntargets:\n  claude:\n    path: " + target + "\n")
	snapshot := func(root string) map[string]string {
		t.Helper()
		files := map[string]string{}
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if info.Mode()&os.ModeSymlink != 0 {
				link, err := os.Readlink(path)
				files[path] = "link:" + link
				return err
			}
			body, err := os.ReadFile(path)
			files[path] = string(body)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		return files
	}
	sourceBefore, externalBefore := snapshot(sb.SourcePath), snapshot(external)
	// The list suffix prints the canonical target; on macOS the temp dir is a symlink.
	resolvedExternal, err := filepath.EvalSymlinks(external)
	if err != nil {
		t.Fatal(err)
	}
	matrix := []struct {
		name     string
		args     []string
		visible  []string
		absent   []string
		reported []string
		refused  bool
		json     bool
	}{
		{"list", []string{"list", "--json"}, []string{"_repo/a", "group/c"}, []string{"secret"}, []string{"\"repoName\": \"_repo\""}, false, true},
		{"list-text", []string{"list", "--no-tui"}, []string{"_repo", "group"}, []string{"secret"}, []string{"→ " + filepath.Join(resolvedExternal, "repo")}, false, false},
		// 3b: the missing declaration pauses prune, and status says so next to the entry states.
		{"status", []string{"status", "--json"}, []string{"_repo"}, []string{"secret"}, []string{"\"skill_count\": 2", "missing", "invalid-target", "prune_paused"}, false, true},
		// 3c: check reports the followed repo as a tracked repo; a local followed group is not a tracked repo.
		{"check", []string{"check", "--json"}, []string{"\"_repo\""}, []string{"group/c", "secret"}, []string{"\"status\": \"dirty\""}, false, true},
		// 3c: --all reaches the followed repo and refuses it per item (the fixture repo is dirty), with no mutation.
		{"update-all-dry-run", []string{"update", "--all", "--dry-run"}, nil, []string{"group/c", "secret"}, []string{"followed repository update refused", "has uncommitted changes"}, true, false},
		// 3c: audit discovers the followed repo's two skills and the followed group's skill through their resolved roots.
		{"audit", []string{"audit", "--no-tui"}, nil, []string{"secret"}, []string{"3 skills"}, false, false},
		{"doctor", []string{"doctor", "--json"}, []string{"_repo: followed", "group: followed"}, []string{"secret"}, []string{"missing: missing", "rejected: invalid-target", "undeclared: not followed"}, false, true},
		{"uninstall-dry-run", []string{"uninstall", "group/c", "--dry-run"}, []string{"group"}, nil, []string{"is a link; edit its target directly"}, true, false},
		{"uninstall-basename-dry-run", []string{"uninstall", "c", "--dry-run"}, nil, nil, []string{"is a link; edit its target directly"}, true, false},
		{"uninstall-glob-dry-run", []string{"uninstall", "g*", "--dry-run"}, nil, nil, []string{"is a link; edit its target directly"}, true, false},
		{"uninstall-repo-dry-run", []string{"uninstall", "_repo", "--dry-run"}, nil, nil, []string{"is a link; edit its target directly"}, true, false},
		{"uninstall-all-dry-run", []string{"uninstall", "--all", "--dry-run"}, nil, nil, []string{"is a link; edit its target directly"}, true, false},
		{"diff", []string{"diff", "--no-tui"}, []string{"_repo__a", "group__c"}, []string{"secret"}, []string{"New"}, false, false},
		// 3e: sync writes only the target; the missing declaration pauses prune.
		{"sync", []string{"sync", "--json"}, []string{"claude"}, []string{"secret"}, []string{"\"prune_paused\": [", "missing (missing)"}, false, true},
		// 3e: after sync, target views count the followed links as linked, not local.
		{"target-info", []string{"target", "claude"}, nil, []string{"local"}, []string{"2 linked"}, false, false},
		{"target-list", []string{"target", "list", "--no-tui"}, nil, []string{"local"}, []string{"2 shared"}, false, false},
	}
	// setup prepares target state a row observes; target writes leave the snapshots alone.
	setup := map[string]func(){
		// A managed link whose text is the resolved target counts as linked only with the follow set.
		"sync": func() {
			if err := os.Symlink(filepath.Join(external, "group", "c"), filepath.Join(target, "group__c")); err != nil {
				t.Fatal(err)
			}
		},
	}
	for _, row := range matrix {
		t.Run(row.name, func(t *testing.T) {
			if prepare := setup[row.name]; prepare != nil {
				prepare()
			}
			args := append(append([]string{}, row.args...), "-g")
			result := sb.RunCLI(args...)
			if row.refused {
				result.AssertFailure(t)
			} else if row.name != "doctor" {
				result.AssertSuccess(t)
			}
			if row.json && !json.Valid([]byte(result.Stdout)) {
				t.Fatalf("invalid JSON: %s\n%s", result.Stdout, result.Stderr)
			}
			output := result.Stdout + result.Stderr
			for _, value := range append(row.visible, row.reported...) {
				if !strings.Contains(output, value) {
					t.Errorf("missing %q: %s", value, output)
				}
			}
			for _, value := range row.absent {
				if strings.Contains(output, value) {
					t.Errorf("unexpected %q: %s", value, output)
				}
			}
			if !reflect.DeepEqual(sourceBefore, snapshot(sb.SourcePath)) || !reflect.DeepEqual(externalBefore, snapshot(external)) {
				t.Fatal("read/dry-run changed source or external content")
			}
		})
	}
}
