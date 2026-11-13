package uninstall

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/sourcefs"
	"skillshare/internal/sourcelink"
	"skillshare/internal/trash"
)

type fixture struct {
	source string
	opts   Options
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	source := filepath.Join(t.TempDir(), "skills")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	return &fixture{source: source, opts: Options{
		SourceDir:    source,
		TrashDir:     filepath.Join(t.TempDir(), "trash"),
		Store:        install.NewMetadataStore(),
		GitignoreDir: source,
	}}
}

func (f *fixture) skill(t *testing.T, rel string) Item {
	t.Helper()
	dir := filepath.Join(f.source, filepath.FromSlash(rel))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+rel), 0o644); err != nil {
		t.Fatal(err)
	}
	return Item{Name: rel, Path: dir, TrashName: rel}
}

// repo creates a committed tracked repo holding one skill.
func (f *fixture) repo(t *testing.T, name string) Item {
	t.Helper()
	item := f.skill(t, name)
	runGit(t, item.Path, "init", "-q")
	runGit(t, item.Path, "add", ".")
	runGit(t, item.Path, "-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "-qm", "seed")
	item.Repo, item.Gitignore = true, name
	return item
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func TestRun_RemovesSkillAndWhatInstallRecorded(t *testing.T) {
	f := newFixture(t)
	gone := f.skill(t, "frontend/foo")
	gone.Gitignore = "frontend/foo"
	f.skill(t, "foo")
	f.opts.Store.Set("frontend/foo", &install.MetadataEntry{Group: "frontend", Source: "github.com/acme/foo"})
	f.opts.Store.Set("foo", &install.MetadataEntry{Source: "github.com/acme/other"})
	f.opts.Store.SetTargetOverride("frontend/foo", []string{"claude"})
	if err := install.UpdateGitIgnoreBatch(f.source, []string{"frontend/foo", "foo"}); err != nil {
		t.Fatal(err)
	}

	out := Run([]Item{gone}, f.opts)

	if len(out.Results) != 1 || out.Results[0].Err != nil || out.GitignoreErr != nil || out.SaveErr != nil {
		t.Fatalf("unexpected outcome: %+v", out)
	}
	if exists(gone.Path) {
		t.Error("skill should have left the source")
	}
	if items := trash.List(f.opts.TrashDir); len(items) != 1 || items[0].Name != "frontend/foo" {
		t.Errorf("expected one trash entry named frontend/foo, got %+v", items)
	}
	saved, err := install.LoadMetadata(f.source)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Has("frontend/foo") || !saved.Has("foo") {
		t.Errorf("saved metadata should hold only foo, got %v", saved.List())
	}
	if len(saved.TargetOverrides) != 0 {
		t.Errorf("target override should be gone, got %v", saved.TargetOverrides)
	}
	ignore, _ := os.ReadFile(filepath.Join(f.source, ".gitignore"))
	if strings.Contains(string(ignore), "frontend/foo/") || !strings.Contains(string(ignore), "foo/") {
		t.Errorf(".gitignore should drop only frontend/foo:\n%s", ignore)
	}
}

func TestRun_DirtyRepoNeedsForce(t *testing.T) {
	f := newFixture(t)
	repo := f.repo(t, "_team")
	clean := f.skill(t, "plain")
	f.opts.Store.Set("_team", &install.MetadataEntry{Tracked: true})
	if err := os.WriteFile(filepath.Join(repo.Path, "wip.txt"), []byte("wip"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := Run([]Item{repo, clean}, f.opts)

	if !errors.Is(out.Results[0].Err, ErrDirty) {
		t.Fatalf("dirty repo should be refused with ErrDirty, got %v", out.Results[0].Err)
	}
	if out.Results[1].Err != nil {
		t.Fatalf("the clean skill next to it should still go: %v", out.Results[1].Err)
	}
	if !exists(repo.Path) || !f.opts.Store.Has("_team") {
		t.Fatal("refused repo must keep its directory and metadata")
	}

	f.opts.Force = true
	if out := Run([]Item{repo}, f.opts); out.Results[0].Err != nil {
		t.Fatalf("forced uninstall failed: %v", out.Results[0].Err)
	}
	if exists(repo.Path) || f.opts.Store.Has("_team") {
		t.Error("forced uninstall should remove the repo and its metadata")
	}
}

func TestRun_UnreadableGitStatusNeedsForce(t *testing.T) {
	f := newFixture(t)
	repo := f.repo(t, "_team")
	if err := os.WriteFile(filepath.Join(repo.Path, ".git", "index"), []byte("corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := Run([]Item{repo}, f.opts)

	var statusErr *StatusError
	if !errors.As(out.Results[0].Err, &statusErr) || !exists(repo.Path) {
		t.Fatalf("expected a StatusError and the repo in place, got %v", out.Results[0].Err)
	}
}

func TestRun_MissingItemIsATrashErrorAndPrunesNothing(t *testing.T) {
	f := newFixture(t)
	f.opts.Store.Set("ghost", &install.MetadataEntry{})

	out := Run([]Item{{Name: "ghost", Path: filepath.Join(f.source, "ghost"), TrashName: "ghost"}}, f.opts)

	var trashErr *TrashError
	if !errors.As(out.Results[0].Err, &trashErr) {
		t.Fatalf("expected a TrashError, got %v", out.Results[0].Err)
	}
	if !f.opts.Store.Has("ghost") {
		t.Error("a failed item must keep its metadata")
	}
}

// A followed source link whose folder is itself a skill is the user's
// checkout: uninstall refuses it whether or not it is a git repo, and before
// looking at its git state.
func TestRun_RefusesLinkedSkillRoot(t *testing.T) {
	f := newFixture(t)
	checkout := t.TempDir()
	if err := os.WriteFile(filepath.Join(checkout, "SKILL.md"), []byte("# root"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, checkout, "init", "-q") // untracked SKILL.md: also dirty
	link := filepath.Join(f.source, "_dev")
	if err := os.Symlink(checkout, link); err != nil {
		t.Fatal(err)
	}
	f.opts.Follow = config.SkillsWalk(true, f.source, nil).Follow
	item := Item{Name: "_dev", Path: link, TrashName: "_dev", Repo: true}

	for _, err := range []error{Preflight([]Item{item}, f.opts)[0], Run([]Item{item}, f.opts).Results[0].Err} {
		if !errors.Is(err, sourcefs.ErrLinkedSkillRoot) {
			t.Fatalf("expected ErrLinkedSkillRoot, got %v", err)
		}
	}
	if !exists(link) {
		t.Error("the link must stay")
	}
}

func TestRun_RefusesItemsInsideTrackedRepoEvenWhenForced(t *testing.T) {
	f := newFixture(t)
	f.repo(t, "_team")
	skill := f.skill(t, "_team/foo")
	sub := f.skill(t, "_team/vendor/_lib")
	runGit(t, sub.Path, "init", "-q") // e.g. a submodule checkout
	sub.Repo = true
	f.opts.Force = true
	items := []Item{skill, sub}

	preflight, run := Preflight(items, f.opts), Run(items, f.opts).Results
	for i := range items {
		if !errors.Is(preflight[i], ErrInsideRepo) || !errors.Is(run[i].Err, ErrInsideRepo) {
			t.Fatalf("%s: expected ErrInsideRepo, got preflight %v, run %v", items[i].Name, preflight[i], run[i].Err)
		}
		if !exists(items[i].Path) {
			t.Errorf("%s must stay in the repo", items[i].Name)
		}
	}
}

func TestPreflight_ReportsDirtyRepoEvenWhenForced(t *testing.T) {
	f := newFixture(t)
	dirty := f.repo(t, "_dirty")
	clean := f.repo(t, "_clean")
	plain := f.skill(t, "plain")
	if err := os.WriteFile(filepath.Join(dirty.Path, "wip.txt"), []byte("wip"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.opts.Force = true

	errs := Preflight([]Item{dirty, clean, plain}, f.opts)

	if !errors.Is(errs[0], ErrDirty) || errs[1] != nil || errs[2] != nil {
		t.Fatalf("expected [ErrDirty nil nil], got %v", errs)
	}
	if !exists(dirty.Path) || !exists(clean.Path) || !exists(plain.Path) {
		t.Error("Preflight must not move anything")
	}
}

func TestAgents_MovesFilesAndPrunesMetadata(t *testing.T) {
	source := t.TempDir()
	trashDir := filepath.Join(t.TempDir(), "trash")
	file := filepath.Join(source, "team", "reviewer.md")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("# reviewer"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := install.NewMetadataStore()
	store.Set("team/reviewer", &install.MetadataEntry{Kind: install.MetadataKindAgent})
	store.Set("ghost", &install.MetadataEntry{Kind: install.MetadataKindAgent})

	errs, saveErr := Agents([]Agent{
		{Name: "team/reviewer", File: file},
		{Name: "ghost", File: filepath.Join(source, "ghost.md")},
	}, source, trashDir, store)

	if errs[0] != nil || errs[1] == nil || saveErr != nil {
		t.Fatalf("expected [nil, error] and a clean save, got %v, %v", errs, saveErr)
	}
	if exists(file) {
		t.Error("agent file should have left the source")
	}
	saved, err := install.LoadMetadata(source)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Has("team/reviewer") || !saved.Has("ghost") {
		t.Errorf("saved metadata should keep only the agent that failed, got %v", saved.List())
	}
}

func TestRestore_PutsBackWhatUninstallRemoved(t *testing.T) {
	f := newFixture(t)
	repo := f.skill(t, "_team")
	f.opts.Store.Set("_team", &install.MetadataEntry{Source: "github.com/acme/team", Tracked: true})
	f.opts.Store.SetTargetOverride("_team/foo", []string{"claude"})
	if out := Run([]Item{repo}, f.opts); out.Results[0].Err != nil || out.SaveErr != nil {
		t.Fatalf("unexpected outcome: %+v", out)
	}

	entry := trash.FindByName(f.opts.TrashDir, "_team")
	if err := Restore(entry, f.source, nil, nil); err != nil {
		t.Fatal(err)
	}

	saved, err := install.LoadMetadata(f.source)
	if err != nil {
		t.Fatal(err)
	}
	if e := saved.Get("_team"); e == nil || e.Source != "github.com/acme/team" || !e.Tracked {
		t.Errorf("restored repo should keep its source, got %+v", e)
	}
	if got := saved.TargetOverrides["_team/foo"]; len(got) != 1 {
		t.Errorf("target override should be back, got %v", saved.TargetOverrides)
	}
	if exists(trash.MetadataFile(entry.Path)) {
		t.Error("saved metadata should leave the trash with the entry")
	}
}

func TestRestore_RekeysToTheTrashName(t *testing.T) {
	f := newFixture(t)
	item := f.skill(t, "frontend/foo")
	item.TrashName = "foo"
	f.opts.Store.Set("frontend/foo", &install.MetadataEntry{Group: "frontend", Source: "github.com/acme/foo"})
	if out := Run([]Item{item}, f.opts); out.Results[0].Err != nil {
		t.Fatal(out.Results[0].Err)
	}

	if err := Restore(trash.FindByName(f.opts.TrashDir, "foo"), f.source, nil, f.opts.Store); err != nil {
		t.Fatal(err)
	}

	if e := f.opts.Store.Get("foo"); e == nil || e.Source != "github.com/acme/foo" || e.Group != "" {
		t.Errorf("entry should follow the skill to foo, got %+v", e)
	}
}

func TestRestoreAgent_PutsBackItsMetadata(t *testing.T) {
	source := t.TempDir()
	trashDir := filepath.Join(t.TempDir(), "trash")
	file := filepath.Join(source, "team", "reviewer.md")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("# reviewer"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := install.NewMetadataStore()
	store.Set("team/reviewer", &install.MetadataEntry{Kind: install.MetadataKindAgent, Source: "github.com/acme/agents"})
	if errs, _ := Agents([]Agent{{Name: "team/reviewer", File: file}}, source, trashDir, store); errs[0] != nil {
		t.Fatal(errs[0])
	}

	if err := RestoreAgent(trash.FindByName(trashDir, "team/reviewer"), source, nil); err != nil {
		t.Fatal(err)
	}

	saved, err := install.LoadMetadata(source)
	if err != nil {
		t.Fatal(err)
	}
	if e := saved.Get("team/reviewer"); e == nil || e.Source != "github.com/acme/agents" {
		t.Errorf("restored agent should keep its source, got %+v", e)
	}
}

func TestRun_RekeyLeavesTheLiveLegacyEntryPrunable(t *testing.T) {
	f := newFixture(t)
	item := f.skill(t, "frontend/foo")
	item.TrashName = "foo"
	f.opts.Store.Set("foo", &install.MetadataEntry{Group: "frontend", Source: "github.com/acme/foo"})

	if out := Run([]Item{item}, f.opts); out.Results[0].Err != nil {
		t.Fatal(out.Results[0].Err)
	}

	if f.opts.Store.Has("foo") {
		t.Errorf("legacy entry should be pruned, got %+v", f.opts.Store.Get("foo"))
	}
}

func TestRestore_PutsBackLegacyRepoGroupEntries(t *testing.T) {
	f := newFixture(t)
	repo := f.skill(t, "_team")
	f.opts.Store.Set("foo", &install.MetadataEntry{Group: "team", Tracked: true, Source: "github.com/acme/team"})
	if out := Run([]Item{repo}, f.opts); out.Results[0].Err != nil || f.opts.Store.Has("foo") {
		t.Fatalf("uninstall should prune the legacy entry: %+v", out)
	}

	if err := Restore(trash.FindByName(f.opts.TrashDir, "_team"), f.source, nil, f.opts.Store); err != nil {
		t.Fatal(err)
	}

	if e := f.opts.Store.Get("foo"); e == nil || e.Source != "github.com/acme/team" {
		t.Errorf("legacy repo entry should be back, got %+v", e)
	}
}

func TestRestore_EntryWithoutSavedMetadataStaysLocal(t *testing.T) {
	f := newFixture(t)
	item := f.skill(t, "foo")
	if out := Run([]Item{item}, f.opts); out.Results[0].Err != nil {
		t.Fatal(out.Results[0].Err)
	}

	if err := Restore(trash.FindByName(f.opts.TrashDir, "foo"), f.source, nil, f.opts.Store); err != nil {
		t.Fatal(err)
	}

	if !exists(item.Path) || f.opts.Store.Has("foo") {
		t.Error("an item with nothing recorded should come back as local")
	}
}

func TestForgetUnlinkedName_FailureAndUndo(t *testing.T) {
	for _, fault := range []string{"none", "sidecar", "save", "empty"} {
		t.Run(fault, func(t *testing.T) {
			f := newFixture(t)
			target := t.TempDir()
			if _, err := sourcelink.Create(f.source, nil, target, "_team"); err != nil {
				t.Fatal(err)
			}
			if fault != "empty" {
				f.opts.Store.Set("_team", &install.MetadataEntry{Source: "normalized"})
				f.opts.Store.AuditAccepted = map[string][]string{"_team/child": {"accepted"}}
				f.opts.Store.SetTargetOverride("_team/child", nil)
			}
			if err := f.opts.Store.Save(f.source); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(install.MetadataPath(f.source))
			trashPath, err := sourcelink.Remove(f.source, f.opts.TrashDir, "_team")
			if err != nil {
				t.Fatal(err)
			}
			blocked := trash.MetadataFile(trashPath)
			if fault == "save" {
				blocked = install.MetadataPath(f.source)
				if err := os.Rename(blocked, blocked+".kept"); err != nil {
					t.Fatal(err)
				}
			}
			if fault == "sidecar" || fault == "save" {
				if err := os.Mkdir(blocked, 0755); err != nil {
					t.Fatal(err)
				}
			}
			err = ForgetUnlinkedName(f.source, trashPath, "_team", f.opts.Store)
			if (err != nil) != (fault == "sidecar" || fault == "save") {
				t.Fatalf("fault %s: %v", fault, err)
			}
			if fault == "sidecar" {
				after, readErr := os.ReadFile(install.MetadataPath(f.source))
				if readErr != nil || string(before) != string(after) || !f.opts.Store.Has("_team") || !strings.Contains(err.Error(), "moved to trash but failed to save its metadata") {
					t.Fatalf("sidecar failure pruned metadata: %v", err)
				}
				return
			}
			if fault == "save" {
				if !exists(trash.MetadataFile(trashPath)) {
					t.Fatal("save failure lost undo snapshot")
				}
				if err := os.Remove(blocked); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(blocked+".kept", blocked); err != nil {
					t.Fatal(err)
				}
			}
			if fault == "none" {
				persisted, loadErr := install.LoadMetadata(f.source)
				if loadErr != nil || !persisted.IsEmpty() || !f.opts.Store.IsEmpty() {
					t.Fatalf("successful unlink did not prune memory and disk before undo: %+v %+v %v", f.opts.Store, persisted, loadErr)
				}
			}
			if fault == "empty" && exists(trash.MetadataFile(trashPath)) {
				t.Fatal("empty unlink wrote a sidecar")
			}
			if err := Restore(trash.FindByName(f.opts.TrashDir, "_team"), f.source, nil, nil); err != nil {
				t.Fatal(err)
			}
			after, _ := os.ReadFile(install.MetadataPath(f.source))
			var want, got install.MetadataStore
			if json.Unmarshal(before, &want) != nil || json.Unmarshal(after, &got) != nil || !reflect.DeepEqual(want, got) {
				t.Fatalf("undo did not recover exact metadata: %s", after)
			}
		})
	}
}

func TestRestore_LegacyKeyKeepsALiveRecordOfAnotherPath(t *testing.T) {
	f := newFixture(t)
	item := f.skill(t, "frontend/foo")
	f.opts.Store.Set("foo", &install.MetadataEntry{Group: "frontend", Source: "github.com/acme/old"})
	if out := Run([]Item{item}, f.opts); out.Results[0].Err != nil {
		t.Fatal(out.Results[0].Err)
	}
	f.skill(t, "foo")
	f.opts.Store.Set("foo", &install.MetadataEntry{Source: "github.com/acme/root"})

	if err := Restore(trash.FindByName(f.opts.TrashDir, "frontend/foo"), f.source, nil, f.opts.Store); err != nil {
		t.Fatal(err)
	}

	if e := f.opts.Store.GetExact("foo"); e == nil || e.Source != "github.com/acme/root" {
		t.Errorf("root foo should keep its own record, got %+v", e)
	}
	if e := f.opts.Store.GetExact("frontend/foo"); e == nil || e.Source != "github.com/acme/old" {
		t.Errorf("frontend/foo should get its record back, got %+v", e)
	}
}

func TestRun_MetadataOnlyLeavesOccupant(t *testing.T) {
	for _, scenario := range []string{"absent", "folder", "link", "dangling link"} {
		t.Run(scenario, func(t *testing.T) {
			f := newFixture(t)
			f.opts.Store.Set("_team", &install.MetadataEntry{Source: "file:///nonexistent/team.git", Tracked: true})
			f.opts.Store.Set("_other", &install.MetadataEntry{Source: "keep", Tracked: true})
			f.opts.Store.AuditAccepted = map[string][]string{"_team": {"root"}, "_team/child": {"child"}, "_other": {"keep"}}
			item := Item{Name: "_team", Path: filepath.Join(f.source, "_team"), MetadataOnly: true, Repo: true, Gitignore: "_team"}
			target := t.TempDir()
			switch scenario {
			case "folder":
				item.Path = f.skill(t, "_team").Path
				f.skill(t, "_team/child")
			case "link", "dangling link":
				if scenario == "link" {
					if err := os.WriteFile(filepath.Join(target, "kept"), []byte("untouched"), 0644); err != nil {
						t.Fatal(err)
					}
				} else {
					target = filepath.Join(target, "missing")
				}
				if err := os.Symlink(target, item.Path); err != nil {
					t.Fatal(err)
				}
			}
			before, _ := os.Lstat(item.Path)
			beforeLink, _ := os.Readlink(item.Path)
			if err := f.opts.Store.Save(f.source); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(f.source, ".gitignore"), []byte("_team/\n"), 0644); err != nil {
				t.Fatal(err)
			}
			hostile := item
			hostile.Path = filepath.Join(t.TempDir(), "outside")
			if errs := Preflight([]Item{item, hostile}, f.opts); errs[0] != nil || errs[1] != nil {
				t.Fatalf("preflight touched paths: %v", errs)
			}
			out := Run([]Item{item}, f.opts)
			if out.Results[0].Err != nil || out.SaveErr != nil {
				t.Fatalf("removal = %+v", out)
			}
			saved, err := install.LoadMetadata(f.source)
			if err != nil || saved.Has("_team") || !saved.Has("_other") || len(saved.AuditAccepted) != 1 {
				t.Fatalf("wrong persisted cleanup: %+v,%v", saved, err)
			}
			after, _ := os.Lstat(item.Path)
			afterLink, _ := os.Readlink(item.Path)
			if (before == nil) != (after == nil) || before != nil && before.Mode() != after.Mode() || beforeLink != afterLink {
				t.Fatal("occupant changed")
			}
			if scenario == "folder" {
				for _, rel := range []string{"SKILL.md", "child/SKILL.md"} {
					data, err := os.ReadFile(filepath.Join(item.Path, rel))
					want := "# _team"
					if rel == "child/SKILL.md" {
						want += "/child"
					}
					if err != nil || string(data) != want {
						t.Fatalf("occupant bytes changed: %s,%v", data, err)
					}
				}
			}
			if scenario == "link" {
				if data, err := os.ReadFile(filepath.Join(target, "kept")); err != nil || string(data) != "untouched" {
					t.Fatal("link target changed")
				}
			}
			ignore, err := os.ReadFile(filepath.Join(f.source, ".gitignore"))
			if err != nil || string(ignore) != "_team/\n" || len(trash.List(f.opts.TrashDir)) != 0 {
				t.Fatal("metadata-only removal touched ignore or trash")
			}
		})
	}
}

func TestRun_MetadataOnlyWithoutDeclaration(t *testing.T) {
	f := newFixture(t)
	f.opts.Store.Set("plain", &install.MetadataEntry{Source: "keep"})
	before, _ := json.Marshal(f.opts.Store)
	out := Run([]Item{{Name: "plain", MetadataOnly: true}}, f.opts)
	after, _ := json.Marshal(f.opts.Store)
	if !errors.Is(out.Results[0].Err, ErrNotDeclared) || string(before) != string(after) {
		t.Fatalf("stale removal mutated store: %+v", out)
	}
}

func TestRun_MetadataOnlyRefusesCheckoutRestoredAfterSelection(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for _, kind := range []string{"clean", "dirty", "checkout link", "still absent", "folder", "plain link"} {
		for _, force := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/force=%t", kind, force), func(t *testing.T) {
				f := newFixture(t)
				f.opts.Store.Set("_team", &install.MetadataEntry{Source: "unreachable", Tracked: true, Group: "org"})
				f.opts.Store.AuditAccepted = map[string][]string{"org/_team": {"root"}, "org/_team/child": {"child"}}
				name, _, err := install.FindTrackedDeclaration(f.source, f.opts.Store, "team")
				if err != nil || name != "org/_team" {
					t.Fatalf("absent selection = %q %v", name, err)
				}
				item := Item{Name: name, Path: filepath.Join(t.TempDir(), "unused"), MetadataOnly: true, Gitignore: name}
				if errs := Preflight([]Item{item}, f.opts); errs[0] != nil {
					t.Fatal(errs)
				}
				if err := f.opts.Store.Save(f.source); err != nil {
					t.Fatal(err)
				}
				before, _ := json.Marshal(f.opts.Store)
				metadata, _ := os.ReadFile(install.MetadataPath(f.source))
				ignore := filepath.Join(f.source, ".gitignore")
				if err := os.WriteFile(ignore, []byte("org/_team/\n"), 0644); err != nil {
					t.Fatal(err)
				}
				checkout := filepath.Join(f.source, filepath.FromSlash(name))
				switch kind {
				case "checkout link", "plain link":
					target := t.TempDir()
					if kind == "checkout link" {
						target = newFixture(t).repo(t, "_external").Path
					}
					if err := os.MkdirAll(filepath.Dir(checkout), 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(target, checkout); err != nil {
						t.Fatal(err)
					}
				case "folder":
					f.skill(t, name)
				case "still absent":
					// The selected path remains absent through execution.
				default:
					f.repo(t, name)
				}
				file := filepath.Join(checkout, "SKILL.md")
				if kind == "dirty" {
					if err := os.WriteFile(file, []byte("local edits"), 0644); err != nil {
						t.Fatal(err)
					}
				}
				content, _ := os.ReadFile(file)
				link, _ := os.Readlink(checkout)
				f.opts.Force = force
				out := Run([]Item{item}, f.opts)
				if kind == "still absent" || kind == "folder" || kind == "plain link" {
					saved, err := install.LoadMetadata(f.source)
					if out.Results[0].Err != nil || out.SaveErr != nil || err != nil || saved.Has("_team") || len(saved.AuditAccepted) != 0 {
						t.Fatalf("non-checkout control refused or not forgotten: %+v %v", out, err)
					}
					return
				}
				after, _ := json.Marshal(f.opts.Store)
				if !errors.Is(out.Results[0].Err, ErrCheckoutRestored) || out.SaveErr != nil || string(before) != string(after) {
					t.Fatalf("restored checkout not refused without metadata mutation: %+v", out)
				}
				for path, want := range map[string]string{install.MetadataPath(f.source): string(metadata), ignore: "org/_team/\n", file: string(content)} {
					if got, err := os.ReadFile(path); err != nil || string(got) != want {
						t.Fatalf("refusal changed %s: %q %v", path, got, err)
					}
				}
				if got, _ := os.Readlink(checkout); got != link || !install.IsTrackedCheckout(checkout) || len(trash.List(f.opts.TrashDir)) != 0 {
					t.Fatal("refusal changed checkout, link or trash")
				}
			})
		}
	}
}

func TestRun_MetadataOnlySaveFailure(t *testing.T) {
	f := newFixture(t)
	for _, name := range []string{"_one", "_two"} {
		f.opts.Store.Set(name, &install.MetadataEntry{Source: "unreachable", Tracked: true})
	}
	// A directory at the metadata filename makes atomic publication fail on all platforms.
	if err := os.Mkdir(install.MetadataPath(f.source), 0755); err != nil {
		t.Fatal(err)
	}
	out := Run([]Item{{Name: "_one", MetadataOnly: true}, {Name: "_two", MetadataOnly: true}, {Name: "_gone", MetadataOnly: true}}, f.opts)
	if out.SaveErr == nil || out.Results[0].Err != out.SaveErr || out.Results[1].Err != out.SaveErr || !errors.Is(out.Results[2].Err, ErrNotDeclared) {
		t.Fatalf("false success after save failure: %+v", out)
	}
}
