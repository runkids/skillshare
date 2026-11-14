package profileapply

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/profile"
	ssync "skillshare/internal/sync"
	"skillshare/internal/testutil"
)

func TestProfileApply_CLIAPIConcurrency(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprint(project), func(t *testing.T) {
			f := newFixture(t, project, "copy")
			recorded, release := make(chan struct{}), make(chan struct{})
			first, second := make(chan error, 1), make(chan error, 1)
			go func() {
				_, err := Apply(context.Background(), Request{Scope: f.scope, Name: "a", afterRecord: func() { close(recorded); <-release }})
				first <- err
			}()
			<-recorded
			// The API's ordinary-sync path and CLI's switch share this lock.
			// A second definition transaction cannot slip between record/apply.
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			_, blocked := Mutate(ctx, f.scope, profile.Mutation{Op: "rename", Name: "a", NewName: "renamed"})
			cancel()
			go func() {
				_, err := Apply(context.Background(), Request{Scope: f.scope, Name: "b"})
				second <- err
			}()
			close(release)
			if err := <-first; err != nil {
				t.Fatal(err)
			}
			if err := <-second; err != nil {
				t.Fatal(err)
			}
			if blocked == nil {
				t.Fatal("definition mutation bypassed running switch")
			}
			v, err := Load(f.scope)
			if err != nil || v.State.Active != "b" || v.State.Pending {
				t.Fatalf("state not serialized: %+v %v", v, err)
			}
			if _, err := os.Lstat(filepath.Join(f.target, "alpha")); !os.IsNotExist(err) {
				t.Fatal("first profile outlived second switch")
			}
			if read(t, filepath.Join(f.target, "beta", "SKILL.md")) != read(t, filepath.Join(f.source, "beta", "SKILL.md")) {
				t.Fatal("state/target disagreement")
			}
		})
	}
}

func TestProfileSync_LockThroughCallback(t *testing.T) {
	f := newFixture(t, false, "copy")
	if _, err := Apply(context.Background(), Request{Scope: f.scope, Name: "a", NoSync: true}); err != nil {
		t.Fatal(err)
	}
	_, err := WithSkillSync(context.Background(), SyncRequest{Scope: f.scope, After: func(_ *View, _ *Result) error {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		if _, err := Mutate(ctx, f.scope, profile.Mutation{Op: "create", Name: "competing"}); err == nil {
			t.Fatal("ordinary sync released lock before callback")
		}
		return errors.New("remaining skill work failed")
	}})
	if err == nil {
		t.Fatal("callback failure hidden")
	}
	statePath, _, _ := config.ProfilePaths(f.path)
	// Eligible skills completed; a failure of unrelated ordinary-sync work
	// does not undo their settlement. The lock still spans that work.
	state, err := profile.ReadState(statePath)
	if err != nil || state.Pending {
		t.Fatal("eligible skills did not settle", state, err)
	}
	if _, err := WithSkillSync(context.Background(), SyncRequest{Scope: f.scope}); err != nil {
		t.Fatal(err)
	}
	state, err = profile.ReadState(statePath)
	if err != nil || state.Pending {
		t.Fatal("successful retry did not settle", state, err)
	}
}

func TestProfileSync_PreflightRefusesBeforeWrites(t *testing.T) {
	f := newFixture(t, false, "copy")
	if _, err := Apply(context.Background(), Request{Scope: f.scope, Name: "a", NoSync: true}); err != nil {
		t.Fatal(err)
	}
	before := tree(t, filepath.Dir(f.source))
	_, err := WithSkillSync(context.Background(), SyncRequest{Scope: f.scope, Before: func(_ *View) error {
		return errors.New("native settings conflict")
	}})
	if err == nil || !strings.Contains(err.Error(), "native settings conflict") {
		t.Fatal("preflight refusal lost", err)
	}
	if !reflect.DeepEqual(before, tree(t, filepath.Dir(f.source))) {
		t.Fatal("preflight refusal changed state or targets")
	}
}

func TestProfileApply_ConfigWideFailurePrecedesWrites(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprint(project), func(t *testing.T) {
			f := newFixture(t, project, "copy")
			write(t, f.path, read(t, f.path)+"target_naming: invalid\n")
			if _, err := Apply(context.Background(), Request{Scope: f.scope, Name: "a"}); err == nil {
				t.Fatal("switch ignored config-wide validation")
			}
			if _, err := WithSkillSync(context.Background(), SyncRequest{Scope: f.scope}); err == nil {
				t.Fatal("ordinary sync ignored config-wide validation")
			}
			if _, err := os.Lstat(f.target); !os.IsNotExist(err) {
				t.Fatal("invalid configuration wrote targets", err)
			}
			state, _, _ := config.ProfilePaths(f.path)
			if _, err := os.Lstat(state); !os.IsNotExist(err) {
				t.Fatal("invalid configuration recorded a switch", err)
			}
		})
	}
}

func TestProfileApply_EditBetweenPlanAndApply(t *testing.T) {
	f := newFixture(t, false, "copy")
	if _, err := Apply(context.Background(), Request{Scope: f.scope, Name: "a"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(f.target, "alpha", "SKILL.md")
	out, err := Apply(context.Background(), Request{Scope: f.scope, Name: "b", afterRecord: func() {
		write(t, path, "authored after plan")
	}})
	if !errors.Is(err, ErrIncomplete) || out.Complete || !out.Selection.Pending || read(t, path) != "authored after plan" {
		t.Fatalf("post-plan edit lost or reported complete: %+v %v", out, err)
	}
}

func TestProfileApply_IncompleteOutcome(t *testing.T) {
	for _, failure := range []string{"prune", "remove", "kept", "blocked", "target", "source"} {
		t.Run(failure, func(t *testing.T) {
			row := ssync.TargetResult{Target: "custom", Linked: []string{"alpha"}}
			executed := ssync.ExecuteResult{}
			switch failure {
			case "prune":
				row.PruneError = "cannot read manifest"
			case "remove":
				row.FailedRemovals = []string{"departing"}
			case "kept":
				row.KeptModified = []ssync.EntryChange{{Target: "custom", Entry: "edited", Class: ssync.EntryModifiedCopy}}
			case "blocked":
				row.Blocked = []ssync.EntryChange{{Target: "custom", Entry: "local", Class: ssync.EntryUnmanaged}}
			case "target":
				executed.Failed = []ssync.TargetFailure{{Target: "bad", Error: "cannot create root"}}
			}
			executed.Results = []ssync.TargetResult{row}
			var result Result
			result.outcome(executed, failure == "source")
			if result.Complete || len(result.Targets) == 0 || len(result.Targets[0].Added) != 1 {
				t.Fatalf("failure lost completed work or reported success: %+v", result)
			}
			if failure != "target" && result.Targets[0].State != "incomplete" {
				t.Fatalf("failure not marked incomplete: %+v", result)
			}
		})
	}
}

func TestProfileApply_JSONContract(t *testing.T) {
	f := newFixture(t, false, "copy")
	out, err := Apply(context.Background(), Request{Scope: f.scope, Name: "a"})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"targets", "blocked", "symlink_targets", "snapshot"} {
		if len(result[key]) == 0 || string(result[key]) == "null" {
			t.Fatalf("%s must be an array: %s", key, data)
		}
	}
	var targets []map[string]json.RawMessage
	if err := json.Unmarshal(result["targets"], &targets); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"added", "refreshed", "removed", "converted", "kept_modified"} {
		if len(targets[0][key]) == 0 || string(targets[0][key]) == "null" {
			t.Fatalf("target %s must be an array: %s", key, data)
		}
	}
}

func TestProfiles_StateMatrix(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, tc := range []struct {
			name, skills string
			count        int
		}{
			{"unrestricted", "{}", 2},
			{"empty", "{skills: {}}", 0},
			{"blank", "{skills: {include: [' ', '']}}", 0},
			{"inert", "{skills: {include: [], exclude: [alpha]}}", 0},
			{"filtered", "{skills: {include: ['*'], exclude: [beta]}}", 1},
		} {
			t.Run(fmt.Sprintf("%v/%s", project, tc.name), func(t *testing.T) {
				f := newFixture(t, project, "copy")
				write(t, f.path, read(t, f.path)+"  matrix: "+tc.skills+"\n")
				out, err := Apply(context.Background(), Request{Scope: f.scope, Name: "matrix", AllowEmpty: true})
				if err != nil || !out.Complete || out.Profile.Selected != tc.count {
					t.Fatalf("selection: %+v %v", out, err)
				}
				v, err := Load(f.scope)
				if err != nil {
					t.Fatal(err)
				}
				discovery, err := v.Discover()
				if err != nil {
					t.Fatal(err)
				}
				counts, err := v.TargetCounts(discovery.Skills)
				if err != nil || counts["custom"] != tc.count {
					t.Fatal("show/runner disagreement", counts, err)
				}
			})
		}
	}
}

func TestProfiles_SharedFolderExecution(t *testing.T) {
	f := newFixture(t, false, "merge")
	text := strings.Replace(read(t, f.path), "targets:\n", "targets:\n  second:\n    skills:\n      path: "+f.target+"\n      mode: merge\n", 1)
	text = strings.Replace(text, "include: [alpha]", "include: ['*']", 1)
	write(t, f.path, text)
	write(t, filepath.Join(f.source, "alpha", "SKILL.md"), "---\nname: alpha\nmetadata:\n  targets: [custom]\n---\n# alpha\n")
	write(t, filepath.Join(f.source, "beta", "SKILL.md"), "---\nname: beta\nmetadata:\n  targets: [second]\n---\n# beta\n")
	for i := 0; i < 2; i++ {
		out, err := Apply(context.Background(), Request{Scope: f.scope, Name: "a"})
		if err != nil || !out.Complete {
			t.Fatal(out, err)
		}
		for _, name := range []string{"alpha", "beta"} {
			if _, err := os.Stat(filepath.Join(f.target, name, "SKILL.md")); err != nil {
				t.Fatalf("shared-folder peer pruned %s: %v", name, err)
			}
		}
	}
}

func TestProfileApply_ReplayScratch(t *testing.T) {
	f := newFixture(t, false, "copy")
	text := strings.Replace(read(t, f.path), "mode: copy", "mode: copy\n      target_naming: prefixed", 1)
	text = strings.Replace(text, "include: [alpha]", "include: ['*']", 1)
	write(t, f.path, text)
	// Nested source produces a rewritten prefixed name and exercises legacy replay.
	write(t, filepath.Join(f.source, "_team", "nested", "leaf", "SKILL.md"), "---\nname: leaf\n---\n# nested\n")
	if _, err := Apply(context.Background(), Request{Scope: f.scope, Name: "a"}); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(f.target, ssync.ManifestFile)
	// Use the existing manifest decoder so only outputs is removed; no invented metadata.
	manifest, err := ssync.ReadManifest(f.target)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Outputs = nil
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	write(t, manifestPath, string(data))
	if manifest.Managed["team-leaf"] == "" {
		t.Fatal("fixture did not produce a rewritten tracked copy")
	}
	scratch := t.TempDir()
	t.Setenv("TMPDIR", scratch)
	t.Setenv("TMP", scratch)
	t.Setenv("TEMP", scratch)
	before := tree(t, filepath.Dir(f.source))
	if _, err := Apply(context.Background(), Request{Scope: f.scope, Name: "b", DryRun: true}); err != nil {
		t.Fatal(err)
	}
	if after := tree(t, filepath.Dir(f.source)); !reflect.DeepEqual(before, after) {
		t.Fatal("legacy preview changed retained tree")
	}
	entries, err := os.ReadDir(scratch)
	if err != nil || len(entries) != 0 {
		t.Fatal("replay scratch retained", entries, err)
	}
	// A non-directory temp root makes replay impossible even under root CI.
	badTemp := filepath.Join(t.TempDir(), "file")
	write(t, badTemp, "not a directory")
	t.Setenv("TMPDIR", badTemp)
	t.Setenv("TMP", badTemp)
	t.Setenv("TEMP", badTemp)
	if _, err := Apply(context.Background(), Request{Scope: f.scope, Name: "b", DryRun: true}); !errors.Is(err, ErrRefused) {
		t.Fatalf("unverifiable legacy copy accepted: %v", err)
	}
}

func TestProfileSync_DefinitionsWithoutActiveKeepLegacyPolicy(t *testing.T) {
	f := newFixture(t, false, "copy")
	if _, err := WithSkillSync(context.Background(), SyncRequest{Scope: f.scope}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(f.target, "alpha", "SKILL.md")
	write(t, path, "edited")
	write(t, filepath.Join(f.source, "alpha", "SKILL.md"), "---\nname: alpha\n---\nchanged\n")
	if _, err := WithSkillSync(context.Background(), SyncRequest{Scope: f.scope}); err != nil {
		t.Fatal(err)
	}
	if read(t, path) == "edited" {
		t.Fatal("inactive definitions changed ordinary copy policy")
	}
	_, lock, _ := config.ProfilePaths(f.path)
	if _, err := os.Stat(lock); err != nil {
		t.Fatal("definitions-only skill sync did not take lock", err)
	}
	if _, err := WithSkillSync(context.Background(), SyncRequest{Scope: f.scope, DiscardEdits: true}); !errors.Is(err, ErrRefused) {
		t.Fatal("discard accepted without active/pending state", err)
	}
}

func TestProfiles_PreservationTransitions(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprint(project), func(t *testing.T) {
			f := newFixture(t, project, "copy")
			if _, err := Apply(context.Background(), Request{Scope: f.scope, Name: "a"}); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(f.target, "alpha", "SKILL.md")
			write(t, path, "authored")
			// Same selected skill in a second profile: changed source must not
			// authorize replacing an edited output during selected refresh.
			write(t, f.path, read(t, f.path)+"  same:\n    skills:\n      include: [alpha]\n")
			write(t, filepath.Join(f.source, "alpha", "SKILL.md"), "---\nname: alpha\n---\nchanged source\n")
			before := tree(t, f.target)
			if _, err := Apply(context.Background(), Request{Scope: f.scope, Name: "same"}); !errors.Is(err, ErrRefused) {
				t.Fatal("selected edit not refused", err)
			}
			if !reflect.DeepEqual(before, tree(t, f.target)) {
				t.Fatal("refused switch changed edited tree")
			}
			// A supported target mode change is represented by the serialized
			// mode it writes; ownership comes from the existing copy manifest.
			write(t, f.path, strings.Replace(read(t, f.path), "mode: copy", "mode: merge", 1))
			if _, err := Apply(context.Background(), Request{Scope: f.scope, Name: "b"}); !errors.Is(err, ErrRefused) {
				t.Fatal("mode transition lost departing edit", err)
			}
			out, err := WithSkillSync(context.Background(), SyncRequest{Scope: f.scope})
			if err != nil || out.Complete || read(t, path) != "authored" {
				t.Fatal("ordinary merge converted edited copy", out, err)
			}
			// Literal profile edit is shipped in core; add/remove CLI comes in 3.
			if _, err := Mutate(context.Background(), f.scope, profile.Mutation{Op: "edit", Name: "a", RemoveInclude: []string{"alpha"}}); err != nil {
				t.Fatal(err)
			}
			out, err = WithSkillSync(context.Background(), SyncRequest{Scope: f.scope})
			if err != nil || out.Complete || read(t, path) != "authored" {
				t.Fatal("active definition removal pruned edit", out, err)
			}
			// An unrecorded nested-looking folder is never heuristic ownership.
			write(t, filepath.Join(f.target, "team__private", "SKILL.md"), "unmanaged")
			if _, err := WithSkillSync(context.Background(), SyncRequest{Scope: f.scope}); err != nil {
				t.Fatal(err)
			}
			if read(t, filepath.Join(f.target, "team__private", "SKILL.md")) != "unmanaged" {
				t.Fatal("flat-name heuristic deleted local folder")
			}
		})
	}
}

func TestProfiles_PrefixedRoundTrip(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprint(project), func(t *testing.T) {
			f := newFixture(t, project, "copy")
			text := strings.Replace(read(t, f.path), "mode: copy", "mode: copy\n      target_naming: prefixed", 1)
			text = strings.Replace(text, "include: [alpha]", "include: ['*']", 1)
			write(t, f.path, text)
			write(t, filepath.Join(f.source, "_team", "nested", "leaf", "SKILL.md"), "---\nname: leaf\n---\n# nested\n")
			for _, name := range []string{"a", "b", "a"} {
				if _, err := Apply(context.Background(), Request{Scope: f.scope, Name: name}); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(f.target, "team-leaf", "SKILL.md")
			content := read(t, path)
			if !strings.Contains(content, "name: team-leaf") {
				t.Fatal("fixture was not rewritten", content)
			}
			write(t, path, strings.Replace(content, "name: team-leaf", "name: authored", 1))
			before := tree(t, f.target)
			if _, err := Apply(context.Background(), Request{Scope: f.scope, Name: "b"}); !errors.Is(err, ErrRefused) {
				t.Fatal("generated name edit was not protected", err)
			}
			if !reflect.DeepEqual(before, tree(t, f.target)) {
				t.Fatal("refusal changed edited target")
			}
		})
	}
}

func TestProfiles_LegacyDeclaredNameSwitch(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprint(project), func(t *testing.T) {
			f := newFixture(t, project, "copy")
			text := strings.Replace(read(t, f.path), "mode: copy", "mode: copy\n      target_naming: prefixed", 1)
			write(t, f.path, strings.Replace(text, "include: [alpha]", "include: [_repo__folder]", 1))
			write(t, filepath.Join(f.source, "_repo", "folder", "SKILL.md"), "---\nname: actual\n---\nproducer bytes\n")
			testutil.RunGit(t, filepath.Join(f.source, "_repo"), "init", "--quiet")
			cfg := config.TargetConfig{Skills: &config.ResourceTargetConfig{Path: f.target, TargetNaming: "prefixed", Include: []string{"_repo__folder"}}}
			if _, err := ssync.SyncTargetCopy("custom", cfg, f.source, false, false); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(f.target, "repo-actual", "SKILL.md")
			if got := read(t, path); got != "---\nname: repo-actual\n---\nproducer bytes\n" {
				t.Fatal("ordinary producer did not rewrite declared name", got)
			}
			manifest, err := ssync.ReadManifest(f.target)
			if err != nil || manifest.Managed["repo-actual"] == "" || manifest.Naming["repo-actual"] != "prefixed" {
				t.Fatal("ordinary producer evidence missing", manifest, err)
			}
			delete(manifest.Outputs, "repo-actual")
			if err := ssync.WriteManifest(f.target, manifest); err != nil {
				t.Fatal(err)
			}
			sourceBefore, targetBefore := tree(t, f.source), tree(t, f.target)
			if _, err := Apply(context.Background(), Request{Scope: f.scope, Name: "a", DryRun: true}); err != nil {
				t.Fatal("clean declared-name preview refused", err)
			}
			if !reflect.DeepEqual(targetBefore, tree(t, f.target)) {
				t.Fatal("switch preview changed legacy target")
			}
			if _, err := Apply(context.Background(), Request{Scope: f.scope, Name: "a"}); err != nil {
				t.Fatal("clean declared-name switch refused", err)
			}
			if got := read(t, path); got != "---\nname: repo-actual\n---\nproducer bytes\n" {
				t.Fatal("switch changed clean rewritten producer bytes", got)
			}
			write(t, path, "authored declared-name edit")
			statePath, _, _ := config.ProfilePaths(f.path)
			configBefore, stateBefore := read(t, f.path), read(t, statePath)
			targetBefore = tree(t, f.target)
			if _, err := Apply(context.Background(), Request{Scope: f.scope, Name: "b"}); !errors.Is(err, ErrRefused) {
				t.Fatal("edited declared-name switch not refused", err)
			}
			if !reflect.DeepEqual(targetBefore, tree(t, f.target)) || read(t, f.path) != configBefore || read(t, statePath) != stateBefore {
				t.Fatal("refused switch changed target, definition or selection")
			}
			if !reflect.DeepEqual(sourceBefore, tree(t, f.source)) {
				t.Fatal("profile operation changed original producer or tracked metadata")
			}
		})
	}
}

func TestProfiles_StandardNamingUsesFlatMembership(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, mode := range []string{"merge", "copy"} {
			t.Run(fmt.Sprintf("%v/%s", project, mode), func(t *testing.T) {
				f := newFixture(t, project, mode)
				text := strings.Replace(read(t, f.path), "mode: "+mode, "mode: "+mode+"\n      target_naming: standard", 1)
				text = strings.Replace(text, "include: [alpha]", "include: [nested__leaf]", 1)
				write(t, f.path, text)
				write(t, filepath.Join(f.source, "nested", "leaf", "SKILL.md"), "---\nname: leaf\n---\n# leaf\n")
				if _, err := Apply(context.Background(), Request{Scope: f.scope, Name: "a"}); err != nil {
					t.Fatal(err)
				}
				if _, err := os.Stat(filepath.Join(f.target, "leaf", "SKILL.md")); err != nil {
					t.Fatal("flat membership was confused with output name", err)
				}
				if _, err := Apply(context.Background(), Request{Scope: f.scope, Name: "b"}); err != nil {
					t.Fatal(err)
				}
				if _, err := os.Lstat(filepath.Join(f.target, "leaf")); !os.IsNotExist(err) {
					t.Fatal("standard entry not pruned", err)
				}
			})
		}
	}
}

func TestProfileApply_LegacyConfigAndRegisteredScope(t *testing.T) {
	f := newFixture(t, false, "merge")
	root := filepath.Dir(f.source)
	project := filepath.Join(root, "registered")
	write(t, filepath.Join(project, ".claude", "skills", "local", "SKILL.md"), "project-owned")
	// These are actual legacy config fields; read-only switching may interpret
	// them in memory but must not run their normal migration/registry adoption.
	text := fmt.Sprintf("source: %s\nmode: merge\ntargets:\n  custom:\n    path: %s\nprojects:\n  %s:\n    targets: [claude]\n    skills: {}\n", f.source, f.target, project)
	text += "profiles:\n  a:\n    skills:\n      include: [alpha]\n  empty:\n    skills: {}\n"
	write(t, f.path, text)
	write(t, filepath.Join(f.source, ".skillignore"), "ignored\n")
	write(t, filepath.Join(f.source, "ignored", "SKILL.md"), "ignored bytes")
	write(t, filepath.Join(root, "agents", "reviewer.md"), "pending agent bytes")
	protected := []string{f.source, project, filepath.Join(root, "agents")}
	before := make([]map[string]string, len(protected))
	for i, path := range protected {
		before[i] = tree(t, path)
	}
	for _, request := range []Request{{Scope: f.scope, Name: "a", DryRun: true}, {Scope: f.scope, Name: "empty"}, {Scope: f.scope, Name: "a"}} {
		_, err := Apply(context.Background(), request)
		if request.Name == "empty" {
			if !errors.Is(err, ErrRefused) {
				t.Fatal(err)
			}
		} else if err != nil {
			t.Fatal(err)
		}
		if read(t, f.path) != text {
			t.Fatal("switch migrated config")
		}
		for i, path := range protected {
			if !reflect.DeepEqual(before[i], tree(t, path)) {
				t.Fatalf("switch changed excluded tree %s", path)
			}
		}
	}
}
