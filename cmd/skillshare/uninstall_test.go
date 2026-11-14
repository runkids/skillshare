package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"skillshare/internal/install"
	"skillshare/internal/oplog"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/uninstall"
)

func TestNormalizeUninstallName_WindowsNestedPath(t *testing.T) {
	got := normalizeUninstallNameForSeparator(`coding\_skill`, '\\')
	if got != "coding/_skill" {
		t.Fatalf("normalizeUninstallNameForSeparator() = %q, want %q", got, "coding/_skill")
	}
}

func TestUninstall_ResultOutput(t *testing.T) {
	targets := []*uninstallTarget{{name: "_record", declaration: true}, {name: "plain"}, {name: "_restored", declaration: true}}
	results := []uninstall.Result{{}, {}, {Err: uninstall.ErrCheckoutRestored}}
	succeeded, failed := collectUninstallResults(targets, results, nil)
	removed, records := uninstallNames(succeeded)
	output := captureStdout(t, func() {
		_ = uninstallOutputJSON(removed, records, failed, 0, false, time.Now(), nil)
	})
	var body struct {
		Removed, Failed []string
		RecordOnly      []string `json:"record_only"`
	}
	if err := json.Unmarshal([]byte(output), &body); err != nil || !reflect.DeepEqual(body.Removed, []string{"_record", "plain"}) || !reflect.DeepEqual(body.RecordOnly, []string{"_record"}) || len(body.Failed) != 1 || !strings.Contains(body.Failed[0], "_restored: "+uninstall.ErrCheckoutRestored.Error()) {
		t.Fatalf("wrong translated results: %s %v", output, err)
	}
	// The log translates the same successful subset; refused records are absent.
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	for _, dryRun := range []bool{true, false} {
		count := len(succeeded)
		if dryRun {
			count = 0
		}
		logUninstallOp(cfgPath, []string{"_record", "plain", "_restored"}, count, records, dryRun, time.Now(), nil)
		entries, err := oplog.Read(cfgPath, oplog.OpsFile, 1)
		if err != nil || len(entries) != 1 {
			t.Fatalf("read uninstall log: %v %v", entries, err)
		}
		args := entries[0].Args
		if !reflect.DeepEqual(args["record_only"], []any{"_record"}) || (args["dry_run"] == true) != dryRun {
			t.Fatalf("wrong uninstall log: %+v", entries[0])
		}
		if dryRun && args["succeeded"] != nil {
			t.Fatalf("preview claims completed removal: %+v", entries[0])
		}
	}
}

func TestNormalizeUninstallName_UnixPreservesLiteralBackslash(t *testing.T) {
	got := normalizeUninstallNameForSeparator(`coding\_skill`, '/')
	if got != `coding\_skill` {
		t.Fatalf("normalizeUninstallNameForSeparator() = %q, want %q", got, `coding\_skill`)
	}
}

// --- resolveUninstallByGlob tests ---

func TestResolveUninstallByGlob_MatchesDirs(t *testing.T) {
	src := t.TempDir()
	for _, name := range []string{"core-auth", "core-db", "utils"} {
		os.MkdirAll(filepath.Join(src, name), 0755)
	}

	targets, err := resolveUninstallByGlob("core-*", src, sourcewalk.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(targets))
	}
	names := map[string]bool{}
	for _, tgt := range targets {
		names[tgt.name] = true
	}
	if !names["core-auth"] || !names["core-db"] {
		t.Errorf("expected core-auth and core-db, got %v", names)
	}
}

func TestResolveUninstallByGlob_DetectsTrackedRepos(t *testing.T) {
	src := t.TempDir()
	// Tracked repo (has .git)
	repoDir := filepath.Join(src, "_team-skills")
	os.MkdirAll(filepath.Join(repoDir, ".git"), 0755)
	// Regular skill
	os.MkdirAll(filepath.Join(src, "_team-docs"), 0755)

	targets, err := resolveUninstallByGlob("_team-*", src, sourcewalk.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(targets))
	}
	for _, tgt := range targets {
		if tgt.name == "_team-skills" && !tgt.isTrackedRepo {
			t.Error("_team-skills should be detected as tracked repo")
		}
		if tgt.name == "_team-docs" && tgt.isTrackedRepo {
			t.Error("_team-docs should not be detected as tracked repo")
		}
	}
}

func TestResolveUninstallByGlob_CaseInsensitive(t *testing.T) {
	src := t.TempDir()
	os.MkdirAll(filepath.Join(src, "Core-Auth"), 0755)
	os.MkdirAll(filepath.Join(src, "core-db"), 0755)

	targets, err := resolveUninstallByGlob("core-*", src, sourcewalk.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 2 {
		t.Fatalf("expected 2 case-insensitive matches, got %d", len(targets))
	}
}

func TestResolveUninstallByGlob_NoMatch(t *testing.T) {
	src := t.TempDir()
	os.MkdirAll(filepath.Join(src, "utils"), 0755)

	targets, err := resolveUninstallByGlob("core-*", src, sourcewalk.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 0 {
		t.Errorf("expected 0 matches, got %d", len(targets))
	}
}

func TestResolveUninstallByGlob_SkipsFiles(t *testing.T) {
	src := t.TempDir()
	os.MkdirAll(filepath.Join(src, "core-skill"), 0755)
	os.WriteFile(filepath.Join(src, "core-file.txt"), []byte("not a dir"), 0644)

	targets, err := resolveUninstallByGlob("core-*", src, sourcewalk.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 match (dirs only), got %d", len(targets))
	}
	if targets[0].name != "core-skill" {
		t.Errorf("expected core-skill, got %s", targets[0].name)
	}
}

func TestResolveUninstallTarget_TrackedNames(t *testing.T) {
	for _, tc := range []struct {
		name, input, existing, want string
		repo                        bool
	}{
		{"nested shorthand", "org/team", "", "org/_team", true},
		{"nested explicit", "org/_team", "", "org/_team", true},
		{"trailing slash", "org/team/", "", "org/_team", true},
		{"nested skill wins", "org/team", "org/team", "org/team", false},
		{"nested folder wins", "org/folder", "org/folder", "org/folder", false},
		{"top-level skill wins", "team", "team", "team", false},
		{"top-level shorthand", "team", "", "_team", true},
		{"top-level explicit", "_team", "", "_team", true},
		{"plain git checkout", "plain", "plain/.git", "plain", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := t.TempDir()
			for _, name := range []string{"_team/.git", "org/_team/.git", "org/_folder/.git", tc.existing} {
				if name != "" {
					if err := os.MkdirAll(filepath.Join(src, name), 0755); err != nil {
						t.Fatal(err)
					}
				}
			}
			if tc.existing == "org/team" || tc.existing == "team" {
				if err := os.WriteFile(filepath.Join(src, tc.existing, "SKILL.md"), []byte("# Skill"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			got, err := resolveUninstallTarget(tc.input, src, "source", sourcewalk.Options{})
			if err != nil {
				t.Fatal(err)
			}
			if got.name != tc.want || got.path != filepath.Join(src, tc.want) || got.isTrackedRepo != tc.repo {
				t.Fatalf("target = %+v, want name %q, tracked %v", got, tc.want, tc.repo)
			}
		})
	}
}

func TestResolveUninstallTarget_AmbiguousNestedName(t *testing.T) {
	src := t.TempDir()
	for _, name := range []string{"a/_team/.git", "b/_team/.git"} {
		if err := os.MkdirAll(filepath.Join(src, name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := resolveUninstallTarget("team", src, "source", sourcewalk.Options{}); err == nil {
		t.Fatal("ambiguous shorthand must fail")
	}
}

func TestResolveUninstallName_DeclarationWiring(t *testing.T) {
	src := t.TempDir()
	store := install.NewMetadataStore()
	store.Set("_team", &install.MetadataEntry{Source: "unreachable", Tracked: true})
	mode := &uninstallMode{sourceDir: src, sourceLabel: "source", store: store, declarations: true, gitignoreEntry: func(*uninstallTarget) string { return "must-not-collect" }}
	got, err := resolveUninstallName("_team", mode)
	if err != nil {
		t.Fatal(err)
	}
	item := mode.items([]*uninstallTarget{got})[0]
	if !item.MetadataOnly || item.Repo || item.Gitignore != "" || item.Name != "_team" || item.Path != filepath.Join(src, "_team") {
		t.Fatalf("wrong shared item translation: %+v", item)
	}
	if summary := summarizeUninstallTargets([]*uninstallTarget{got}); summary.trackedRepos != 1 || summary.groups != 0 {
		t.Fatalf("wrong declaration summary: %+v", summary)
	}
}
