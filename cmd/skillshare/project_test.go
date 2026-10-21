package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/projects"
	ssync "skillshare/internal/sync"
)

func TestProjectParsePreservesValuesAndTerminator(t *testing.T) {
	o, err := parseProjectArgs([]string{"edit", "--name", "-p", "--target", "claude", "--", "--json"})
	if err != nil || o.json || o.token != "--json" || o.values["--name"][0] != "-p" {
		t.Fatalf("%+v %v", o, err)
	}
	o, err = parseProjectArgs([]string{"--name=--json", "edit", "app"})
	if err != nil || o.json || o.values["--name"][0] != "--json" {
		t.Fatalf("%+v %v", o, err)
	}
}

func TestProjectBareHelpSubcommand(t *testing.T) {
	var err error
	output := captureStdout(t, func() {
		err = cmdProject([]string{"help"})
	})
	if err != nil {
		t.Fatalf("project help returned error: %v", err)
	}
	if !strings.Contains(output, "Manage project folders") {
		t.Fatalf("project help output missing heading: %s", output)
	}
}

func TestProjectPatchKeepsResourceSettingsAndCopiesLists(t *testing.T) {
	entry := config.ManagedProject{Targets: []string{"claude"}, Skills: &config.ResourceTargetConfig{Mode: "copy", Include: []string{"old"}}, Agents: &config.ResourceTargetConfig{Mode: "merge"}}
	original := *entry.Skills
	o, err := parseProjectArgs([]string{"edit", "app", "--skills", "--skills-include", "new", "--agents-exclude", "ignore"})
	if err != nil {
		t.Fatal(err)
	}
	next, err := patchProject(entry, o)
	if err != nil {
		t.Fatal(err)
	}
	if next.Skills.Mode != "copy" || !reflect.DeepEqual(next.Skills.Include, []string{"new"}) || !reflect.DeepEqual(entry.Skills.Include, original.Include) || len(entry.Agents.Exclude) != 0 {
		t.Fatalf("next %+v original %+v", next, entry)
	}
}

func TestProjectPatchExplicitEnablePreservesDisabledSettings(t *testing.T) {
	for _, kind := range []string{"skills", "agents"} {
		for _, flag := range []string{"--" + kind, "--name=renamed", "--" + kind + "-include=new"} {
			t.Run(kind+flag, func(t *testing.T) {
				rc := &config.ResourceTargetConfig{Mode: "copy", Include: []string{"old"}}
				rc.SetEnabled(false)
				entry := config.ManagedProject{Targets: []string{"claude"}}
				if kind == "skills" {
					entry.Skills = rc
				} else {
					entry.Agents = rc
				}
				o, err := parseProjectArgs([]string{"edit", "app", flag})
				if err != nil {
					t.Fatal(err)
				}
				next, err := patchProject(entry, o)
				if err != nil {
					t.Fatal(err)
				}
				got := next.Skills
				if kind == "agents" {
					got = next.Agents
				}
				wantEnabled := flag != "--name=renamed"
				wantInclude := []string{"old"}
				if strings.Contains(flag, "-include=") {
					wantInclude = []string{"new"}
				}
				if !reflect.DeepEqual(got.Include, wantInclude) {
					t.Errorf("settings lost: got %v want %v", got.Include, wantInclude)
				}
				if got.IsEnabled() != wantEnabled || got.Mode != "copy" || rc.IsEnabled() || rc.Include[0] != "old" {
					t.Fatalf("explicit enable=%v: next=%+v original=%+v", wantEnabled, got, rc)
				}
			})
		}
	}
}

func TestProjectTextPreparedSyncDiagnostics(t *testing.T) {
	root := filepath.Join(t.TempDir(), "RUNNER~1", "app")
	expanded := loadProjectConfig(t, root, "claude, cursor")
	r := projectResult{Result: projects.Result{Expanded: expanded}, Sync: &ssync.ExecuteResult{
		Unmatched:       []ssync.TargetUnmatchedInclude{{Target: "app@claude", Patterns: []string{"typo"}, Suggestions: []string{"team_alpha"}, All: true}},
		FolderConflicts: []config.SkillsFolderConflict{{Targets: []string{"app@claude", "ordinary"}, Path: "/prepared/skills", Stop: []string{"ordinary", "app@claude"}}},
		PathOverlap:     2,
	}}
	output := stripANSIWarnings(captureStdout(t, func() {
		if err := renderProjectResult(projectOptions{action: "sync", dryRun: true}, r, nil, time.Now()); err != nil {
			t.Fatal(err)
		}
	}))
	for _, want := range []string{`app@claude: include filter "typo" matches no skill`, `did you mean "team_alpha"`, "app@claude and ordinary sync skills to /prepared/skills with different settings", "keep one: skillshare target ordinary --skills=false", "keep one: skillshare project edit " + root + " --target cursor", "2 targets share skill folders", "skillshare doctor"} {
		if !strings.Contains(output, want) {
			t.Errorf("missing %q in %s", want, output)
		}
	}
}

func TestProjectPatchRejectsRootNames(t *testing.T) {
	for _, name := range []string{"~other", "./relative", "C:work"} {
		o, err := parseProjectArgs([]string{"edit", "app", "--name", name})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := patchProject(config.ManagedProject{}, o); err == nil {
			t.Fatalf("accepted root-looking name %q", name)
		}
	}
}
func TestProjectParseRejectsDisabledResourceSettings(t *testing.T) {
	for _, kind := range []string{"skills", "agents"} {
		if _, err := parseProjectArgs([]string{"edit", "app", "--no-" + kind, "--" + kind + "-mode=copy"}); err == nil {
			t.Fatalf("accepted disable plus %s setting", kind)
		}
	}
}
func TestProjectParseOperandCountMessages(t *testing.T) {
	if _, err := parseProjectArgs([]string{"list", "app"}); err == nil || err.Error() != "project list takes no project argument" {
		t.Fatalf("list with an operand: %v", err)
	}
	if _, err := parseProjectArgs([]string{"show"}); err == nil || err.Error() != "project show requires exactly one project argument" {
		t.Fatalf("show without an operand: %v", err)
	}
}

func TestProjectPartialEditRendering(t *testing.T) {
	r := projectResult{Result: projects.Result{Item: &projects.Item{Name: "app", Targets: []string{"claude"}}, Affected: []string{"/app"}, Registration: "kept", Detach: &projects.DetachReport{Removed: []string{"/app/manifest"}}}}
	failed := fmt.Errorf("save denied")
	output := captureStdout(t, func() { _ = renderProjectResult(projectOptions{action: "edit", json: true}, r, failed, time.Now()) })
	var got map[string]any
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatal(err)
	}
	if got["applied"] != false || got["registration"] != "kept" || got["detach"] == nil {
		t.Fatalf("false persisted state: %s", output)
	}
	output = stripANSIWarnings(captureStdout(t, func() { _ = renderProjectResult(projectOptions{action: "edit"}, r, failed, time.Now()) }))
	for _, want := range []string{"Removed", "/app/manifest", "Registration", "kept"} {
		if !strings.Contains(output, want) {
			t.Fatalf("missing %q: %s", want, output)
		}
	}
	r.Registration = "updated"
	output = captureStdout(t, func() { _ = renderProjectResult(projectOptions{action: "edit", json: true}, r, failed, time.Now()) })
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatal(err)
	}
	if got["applied"] != true {
		t.Fatal("saved then failed sync lost applied state", output)
	}
}

func TestProjectDisabledResourceJSON(t *testing.T) {
	output := captureStdout(t, func() {
		_ = renderProjectResult(projectOptions{action: "edit", json: true}, projectResult{Result: projects.Result{Item: &projects.Item{Name: "app", Skills: &projects.Resource{Disabled: true}, Agents: &projects.Resource{Disabled: true}}}}, nil, time.Now())
	})
	var got map[string]any
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatal(err)
	}
	item := got["project"].(map[string]any)
	if item["skills"] != nil || item["agents"] != nil {
		t.Fatal("disabled resources indistinguishable from enabled:", output)
	}
}

func TestProjectSkipsMaintenanceOnlyForReadsPreviewsAndJSON(t *testing.T) {
	for args, want := range map[string]bool{
		"":                         true,
		"list":                     true,
		"show app":                 true,
		"--help":                   true,
		"sync app --dry-run":       true,
		"edit app --name x --json": true,
		"remove app --yes --json":  true,
		"edit app --target --json": false,
		"add /p --target claude":   false,
		"edit app --name x":        false,
		"sync app":                 false,
		"remove app --yes":         false,
	} {
		if got := projectSkipsMaintenance(strings.Fields(args)); got != want {
			t.Errorf("%q: got %v, want %v", args, got, want)
		}
	}
}

func TestProjectPatchAgentSymlinkFiltersOnlyRejectsFlagsThatAddThem(t *testing.T) {
	stored := config.ManagedProject{Targets: []string{"claude"}, Agents: &config.ResourceTargetConfig{Mode: "symlink", Include: []string{"team-*"}}}
	for _, tc := range []struct {
		args    []string
		wantErr bool
	}{
		{[]string{"edit", "app", "--name", "renamed"}, false},
		{[]string{"edit", "app", "--target", "claude,cursor"}, false},
		{[]string{"edit", "app", "--agents-include", "other-*"}, true},
		{[]string{"edit", "app", "--agents-mode", "symlink"}, true},
		{[]string{"edit", "app", "--agents-mode", "merge"}, false},
		{[]string{"edit", "app", "--agents-include="}, false},
	} {
		o, err := parseProjectArgs(tc.args)
		if err != nil {
			t.Fatal(err)
		}
		_, err = patchProject(stored, o)
		if (err != nil) != tc.wantErr {
			t.Errorf("%v: err = %v, want error %v", tc.args, err, tc.wantErr)
		}
	}
}
