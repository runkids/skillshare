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
		{"list-text", []string{"list", "--no-tui"}, []string{"_repo", "group"}, []string{"secret"}, []string{"→ " + filepath.Join(external, "repo")}, false, false},
		// pending 3b: ownership/prune aggregates; 3a already discovers followed skills.
		{"status", []string{"status", "--json"}, []string{"_repo"}, []string{"secret"}, []string{"\"skill_count\": 2", "missing", "invalid-target"}, false, true},
		// pending 3c: currently neither followed repos nor local followed groups are checked.
		{"check", []string{"check", "--json"}, nil, []string{"\"_repo\"", "group/c", "secret"}, []string{"\"tracked_repos\": []"}, false, true},
		// pending 3c: --all currently sees no updatable entries through links.
		{"update-all-dry-run", []string{"update", "--all", "--dry-run"}, nil, []string{"_repo", "group/c", "secret"}, []string{"Nothing to update"}, false, false},
		// pending 3c: CLI audit discovery is handed off with repo-root audit fixes.
		{"audit", []string{"audit", "--no-tui"}, nil, []string{"_repo/a", "group/c", "secret"}, []string{"No skills"}, false, false},
		{"doctor", []string{"doctor", "--json"}, []string{"_repo: followed", "group: followed"}, []string{"secret"}, []string{"missing: missing", "rejected: invalid-target", "undeclared: not followed"}, false, true},
		{"uninstall-dry-run", []string{"uninstall", "group/c", "--dry-run"}, []string{"group"}, nil, []string{"is a link; edit its target directly"}, true, false},
		{"uninstall-basename-dry-run", []string{"uninstall", "c", "--dry-run"}, nil, nil, []string{"is a link; edit its target directly"}, true, false},
		{"uninstall-glob-dry-run", []string{"uninstall", "g*", "--dry-run"}, nil, nil, []string{"is a link; edit its target directly"}, true, false},
		{"uninstall-repo-dry-run", []string{"uninstall", "_repo", "--dry-run"}, nil, nil, []string{"is a link; edit its target directly"}, true, false},
		{"uninstall-all-dry-run", []string{"uninstall", "--all", "--dry-run"}, nil, nil, []string{"is a link; edit its target directly"}, true, false},
		{"diff", []string{"diff", "--no-tui"}, []string{"_repo__a", "group__c"}, []string{"secret"}, []string{"New"}, false, false},
	}
	for _, row := range matrix {
		t.Run(row.name, func(t *testing.T) {
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
