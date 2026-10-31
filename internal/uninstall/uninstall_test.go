package uninstall

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/sourcefs"
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
