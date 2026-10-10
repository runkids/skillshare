package skillmove

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/projectdir"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/sync"
)

type fixture struct {
	source string
	store  *install.MetadataStore
	opts   Options
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	source := filepath.Join(t.TempDir(), "skills")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	store := install.NewMetadataStore()
	return &fixture{source: source, store: store, opts: Options{SourceDir: source, Store: store}}
}

func (f *fixture) path(rel string) string { return filepath.Join(f.source, filepath.FromSlash(rel)) }

// skill creates a skill whose SKILL.md name is its folder name.
func (f *fixture) skill(t *testing.T, rel string) {
	t.Helper()
	f.file(t, rel+"/SKILL.md", "---\nname: "+filepath.Base(rel)+"\n---\n# "+rel+"\n")
}

func (f *fixture) file(t *testing.T, rel, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(f.path(rel)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.path(rel), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// record stores an install record keyed by the full path, as install writes it.
func (f *fixture) record(rel string) {
	group := ""
	if i := strings.LastIndex(rel, "/"); i >= 0 {
		group = rel[:i]
	}
	f.store.Set(rel, &install.MetadataEntry{Source: "github.com/user/repo/" + filepath.Base(rel), Group: group})
}

// repo makes dir a tracked checkout.
func (f *fixture) repo(t *testing.T, rel string) {
	t.Helper()
	dir := f.path(rel)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
}

// link makes rel a link to a folder outside the source and follows it.
func (f *fixture) link(t *testing.T, rel string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("needs symlink rights")
	}
	target := filepath.Join(t.TempDir(), "checkout", "linked-skill")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("---\nname: linked-skill\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Dir(target), f.path(rel)); err != nil {
		t.Fatal(err)
	}
	f.opts.Follow = sourcewalk.NewFollow(f.source, nil)
}

func (f *fixture) discover(t *testing.T) []sync.DiscoveredSkill {
	t.Helper()
	skills, err := sync.DiscoverSourceSkillsAll(f.source, sourcewalk.Options{Follow: f.opts.Follow})
	if err != nil {
		t.Fatal(err)
	}
	return skills
}

func (f *fixture) plan(t *testing.T, dest string, names ...string) []Planned {
	t.Helper()
	return Plan(f.discover(t), names, dest, f.opts)
}

func (f *fixture) exists(rel string) bool {
	_, err := os.Lstat(f.path(rel))
	return err == nil
}

func wantRefusal(t *testing.T, p Planned, code Code) {
	t.Helper()
	if p.Err == nil || p.Err.Code != code {
		t.Fatalf("%s: err = %v, want code %s", p.Name, p.Err, code)
	}
}

func wantMoved(t *testing.T, p Planned) {
	t.Helper()
	if p.Err != nil {
		t.Fatalf("%s: unexpected refusal %s: %v", p.Name, p.Err.Code, p.Err)
	}
}

func TestRun_SkillWithRecordKeepsRecordAndAcceptances(t *testing.T) {
	f := newFixture(t)
	f.skill(t, "demo")
	f.record("demo")
	f.store.AuditAccepted = map[string][]string{"demo": {"accepted-key"}}

	planned := f.plan(t, "grp", "demo")
	wantMoved(t, planned[0])
	out := Run(planned, f.opts)

	if out.Err != nil {
		t.Fatal(out.Err)
	}
	wantMoved(t, out.Items[0])
	if f.exists("demo") || !f.exists("grp/demo/SKILL.md") {
		t.Fatal("the skill did not move on disk")
	}
	got := f.store.Get("grp/demo")
	if got == nil || got.Source != "github.com/user/repo/demo" || got.Group != "grp" {
		t.Errorf("record at the new path = %+v, want the old record with group grp", got)
	}
	if f.store.Has("demo") {
		t.Error("record stayed at the old key")
	}
	if got := f.store.AuditAccepted["grp/demo"]; !reflect.DeepEqual(got, []string{"accepted-key"}) {
		t.Errorf("accepted findings = %v, want them carried", got)
	}
	saved, err := install.LoadMetadata(f.source)
	if err != nil || !saved.Has("grp/demo") || saved.Has("demo") {
		t.Errorf("saved store keys = %v (err %v), want only grp/demo", saved.List(), err)
	}
}

func TestRun_SkillWithoutRecordMoves(t *testing.T) {
	f := newFixture(t)
	f.skill(t, "mine")

	out := Run(f.plan(t, "grp", "mine"), f.opts)

	wantMoved(t, out.Items[0])
	if out.Items[0].Records != 0 || !f.exists("grp/mine/SKILL.md") {
		t.Errorf("records = %d, moved = %v; want a record-less skill moved", out.Items[0].Records, f.exists("grp/mine"))
	}
}

func TestRun_RecordWithoutHashesOrWithStaleHashesFollows(t *testing.T) {
	f := newFixture(t)
	f.skill(t, "edited")
	f.skill(t, "bare")
	f.store.Set("edited", &install.MetadataEntry{Source: "s/edited", FileHashes: map[string]string{"SKILL.md": "not-the-current-hash"}})
	f.store.Set("bare", &install.MetadataEntry{Source: "s/bare"})

	out := Run(f.plan(t, "grp", "edited", "bare"), f.opts)

	if out.Err != nil {
		t.Fatal(out.Err)
	}
	for _, key := range []string{"grp/edited", "grp/bare"} {
		if !f.store.Has(key) {
			t.Errorf("record %q did not follow the move; keys = %v", key, f.store.List())
		}
	}
}

func TestRun_LegacyBasenameKeyIsRekeyedWithGroup(t *testing.T) {
	f := newFixture(t)
	f.skill(t, "frontend/demo")
	f.store.Set("demo", &install.MetadataEntry{Source: "s/demo", Group: "frontend"})
	f.store.AuditAccepted = map[string][]string{"frontend/demo": {"k"}}

	out := Run(f.plan(t, "archive", "frontend/demo"), f.opts)

	wantMoved(t, out.Items[0])
	got := f.store.Get("archive/demo")
	if got == nil || got.Group != "archive" || f.store.Has("demo") {
		t.Fatalf("keys = %v, entry = %+v; want the legacy key replaced by archive/demo", f.store.List(), got)
	}
	if len(f.store.AuditAccepted["archive/demo"]) != 1 {
		t.Errorf("accepted findings = %v", f.store.AuditAccepted)
	}
}

func TestRun_NestedSkillsMoveWithTheirParent(t *testing.T) {
	f := newFixture(t)
	f.skill(t, "foo")
	f.skill(t, "foo/sub")
	f.record("foo")
	f.record("foo/sub")
	f.store.AuditAccepted = map[string][]string{"foo/sub": {"nested-key"}}

	planned := f.plan(t, "grp", "foo")
	wantMoved(t, planned[0])
	if len(planned[0].Skills) != 2 || planned[0].Records != 2 {
		t.Fatalf("skills = %d, records = %d; want the parent and its nested skill", len(planned[0].Skills), planned[0].Records)
	}
	out := Run(planned, f.opts)

	if out.Err != nil {
		t.Fatal(out.Err)
	}
	for _, key := range []string{"grp/foo", "grp/foo/sub"} {
		if !f.store.Has(key) {
			t.Errorf("missing record %q; keys = %v", key, f.store.List())
		}
	}
	if len(f.store.AuditAccepted["grp/foo/sub"]) != 1 {
		t.Errorf("nested accepted findings = %v", f.store.AuditAccepted)
	}
}

func TestRun_FolderMovesWholeWithEverythingInIt(t *testing.T) {
	f := newFixture(t)
	f.skill(t, "frontend/a")
	f.skill(t, "frontend/deep/b")
	f.file(t, "frontend/notes.txt", "keep me")
	if err := os.MkdirAll(f.path("frontend/empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	f.record("frontend/a")
	f.record("frontend/deep/b")

	planned := f.plan(t, "archive", "frontend")
	wantMoved(t, planned[0])
	if !planned[0].Folder || len(planned[0].Skills) != 2 || planned[0].To != "archive/frontend" {
		t.Fatalf("plan = %+v, want a folder of two skills going to archive/frontend", planned[0])
	}
	out := Run(planned, f.opts)

	if out.Err != nil {
		t.Fatal(out.Err)
	}
	for _, rel := range []string{"archive/frontend/a/SKILL.md", "archive/frontend/deep/b/SKILL.md", "archive/frontend/notes.txt", "archive/frontend/empty"} {
		if !f.exists(rel) {
			t.Errorf("%s did not move with the folder", rel)
		}
	}
	if f.exists("frontend") {
		t.Error("the old folder is still there")
	}
	for _, key := range []string{"archive/frontend/a", "archive/frontend/deep/b"} {
		if got := f.store.Get(key); got == nil || got.Group != groupOf(key) {
			t.Errorf("record %q = %+v, want it re-keyed with its group", key, got)
		}
	}
}

func groupOf(key string) string { return filepath.ToSlash(filepath.Dir(key)) }

func TestPlan_FolderWithTrackedRepoIsRefusedWholeBeforeAnyRename(t *testing.T) {
	f := newFixture(t)
	f.skill(t, "frontend/a")
	f.repo(t, "frontend/vendor/_lib") // no skill of its own: discovery cannot see it
	f.skill(t, "other")
	f.record("frontend/a")

	out := Run(f.plan(t, "archive", "frontend", "other"), f.opts)

	wantRefusal(t, out.Items[0], CodeInsideTrackedRepo)
	wantMoved(t, out.Items[1]) // a refused name does not stop the others
	if !f.exists("frontend/a/SKILL.md") || f.exists("archive/frontend") {
		t.Error("part of the folder moved although the folder was refused")
	}
	if !f.store.Has("frontend/a") {
		t.Error("the refused folder's record was touched")
	}
	if !f.exists("archive/other") {
		t.Error("the unrelated name was not moved")
	}
}

func TestPlan_Refusals(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, f *fixture)
		dest  string
		names []string
		code  Code
	}{
		{"skill_not_found", func(t *testing.T, f *fixture) {}, "grp", []string{"ghost"}, CodeNotFound},
		{"folder without skills is not found", func(t *testing.T, f *fixture) {
			if err := os.MkdirAll(f.path("empty-folder"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, "grp", []string{"empty-folder"}, CodeNotFound},
		{"parent traversal", func(t *testing.T, f *fixture) { f.skill(t, "demo") }, "grp", []string{"../demo"}, CodeNotFound},
		{"ambiguous_name", func(t *testing.T, f *fixture) {
			f.skill(t, "a/demo")
			f.skill(t, "b/demo")
		}, "grp", []string{"demo"}, CodeAmbiguousName},
		{"dest_exists skill", func(t *testing.T, f *fixture) {
			f.skill(t, "demo")
			f.skill(t, "grp/demo")
		}, "grp", []string{"demo"}, CodeDestExists},
		{"dest_exists folder", func(t *testing.T, f *fixture) {
			f.skill(t, "demo")
			if err := os.MkdirAll(f.path("grp/demo"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, "grp", []string{"demo"}, CodeDestExists},
		{"inside_tracked_repo", func(t *testing.T, f *fixture) {
			f.repo(t, "_team")
			f.skill(t, "_team/foo")
		}, "grp", []string{"_team__foo"}, CodeInsideTrackedRepo},
		{"tracked repo itself", func(t *testing.T, f *fixture) {
			f.repo(t, "_team")
			f.skill(t, "_team/foo")
		}, "grp", []string{"_team"}, CodeInsideTrackedRepo},
		{"dest_inside_tracked_repo", func(t *testing.T, f *fixture) {
			f.repo(t, "_team")
			f.skill(t, "demo")
		}, "_team/sub", []string{"demo"}, CodeDestInsideTrackedRepo},
		{"dest_is_skill", func(t *testing.T, f *fixture) {
			f.skill(t, "demo")
			f.skill(t, "grp")
		}, "grp/inner", []string{"demo"}, CodeDestIsSkill},
		{"invalid_dest segment", func(t *testing.T, f *fixture) { f.skill(t, "demo") }, "grp/_x", []string{"demo"}, CodeInvalidDest},
		{"invalid_dest dotdot", func(t *testing.T, f *fixture) { f.skill(t, "demo") }, "../out", []string{"demo"}, CodeInvalidDest},
		{"invalid_dest empty", func(t *testing.T, f *fixture) { f.skill(t, "demo") }, "", []string{"demo"}, CodeInvalidDest},
		{"invalid_dest is a file", func(t *testing.T, f *fixture) {
			f.skill(t, "demo")
			f.file(t, "grp", "plain file")
		}, "grp", []string{"demo"}, CodeInvalidDest},
		{"dest_inside_source_folder", func(t *testing.T, f *fixture) {
			f.skill(t, "frontend/a")
		}, "frontend/sub", []string{"frontend"}, CodeDestInsideSource},
		{"dest_inside_source_folder into itself", func(t *testing.T, f *fixture) {
			f.skill(t, "frontend/a")
		}, "frontend", []string{"frontend"}, CodeDestInsideSource},
		{"duplicate_dest", func(t *testing.T, f *fixture) {
			f.skill(t, "frontend/foo")
			f.skill(t, "backend/foo")
		}, "archive", []string{"frontend/foo", "backend/foo"}, CodeDuplicateDest},
		{"overlapping_sources child", func(t *testing.T, f *fixture) {
			f.skill(t, "foo")
			f.skill(t, "foo/sub")
		}, "archive", []string{"foo", "foo/sub"}, CodeOverlappingSources},
		{"overlapping_sources same name twice", func(t *testing.T, f *fixture) {
			f.skill(t, "foo")
		}, "archive", []string{"foo", "foo"}, CodeOverlappingSources},
		{"same_folder", func(t *testing.T, f *fixture) { f.skill(t, "grp/demo") }, "grp", []string{"demo"}, CodeSameFolder},
		{"same_folder at the root", func(t *testing.T, f *fixture) { f.skill(t, "demo") }, ".", []string{"demo"}, CodeSameFolder},
		{"ambiguous_record", func(t *testing.T, f *fixture) {
			f.skill(t, "grp/demo")
			f.record("grp/demo")
			f.store.Set("demo", &install.MetadataEntry{Source: "s/stale", Group: "grp"})
		}, "archive", []string{"grp/demo"}, CodeAmbiguousRecord},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			c.setup(t, f)

			planned := f.plan(t, c.dest, c.names...)

			wantRefusal(t, planned[len(planned)-1], c.code)
			out := Run(planned, f.opts)
			for _, item := range out.Items {
				if item.Err == nil {
					t.Fatalf("Run moved %s that Plan refused", item.Name)
				}
			}
		})
	}
}

func TestPlan_LinkedFolderRefusals(t *testing.T) {
	t.Run("skill below a followed link", func(t *testing.T) {
		f := newFixture(t)
		f.link(t, "_dev")
		f.skill(t, "demo")

		planned := f.plan(t, "grp", "_dev/linked-skill")

		wantRefusal(t, planned[0], CodeLinkedFolder)
	})
	t.Run("the link itself", func(t *testing.T) {
		f := newFixture(t)
		f.link(t, "_dev")

		wantRefusal(t, f.plan(t, "grp", "_dev")[0], CodeLinkedFolder)
	})
	t.Run("dest below a link", func(t *testing.T) {
		f := newFixture(t)
		f.link(t, "dev")
		f.skill(t, "demo")

		wantRefusal(t, f.plan(t, "dev/inner", "demo")[0], CodeLinkedFolder)
	})
	t.Run("link at the root of a moved folder", func(t *testing.T) {
		f := newFixture(t)
		f.link(t, "dev")
		f.skill(t, "demo")

		planned := f.plan(t, "grp", "dev/linked-skill")

		wantRefusal(t, planned[0], CodeLinkedFolder)
	})
}

func TestPlan_NameResolution(t *testing.T) {
	f := newFixture(t)
	f.skill(t, "frontend/react")
	f.skill(t, "tools/frontend") // a skill whose base name equals a folder's path
	f.skill(t, "solo")

	cases := map[string]string{
		"frontend/react":  "frontend/react", // path
		"frontend__react": "frontend/react", // flat name
		"react":           "frontend/react", // unique base name
		"frontend/":       "frontend",       // trailing slash; exact folder wins over a base name
		"solo":            "solo",
	}
	for name, want := range cases {
		planned := f.plan(t, "archive", name)
		wantMoved(t, planned[0])
		if planned[0].From != want {
			t.Errorf("%q resolved to %q, want %q", name, planned[0].From, want)
		}
	}
}

func TestPlan_FlatNameCollisionIsForceable(t *testing.T) {
	f := newFixture(t)
	f.skill(t, "grp__demo") // a top-level skill already holds the flat name
	f.skill(t, "demo")
	f.opts.Targets = []Target{{Name: "claude", Config: config.ResourceTargetConfig{TargetNaming: "flat"}}}

	planned := f.plan(t, "grp", "demo")
	wantRefusal(t, planned[0], CodeNameCollision)

	f.opts.Force = true
	planned = f.plan(t, "grp", "demo")
	wantMoved(t, planned[0])
	if len(planned[0].Warnings) == 0 {
		t.Error("an accepted collision left no warning")
	}
}

// TestPlan_StandardNamingCollisionExistedBeforeTheMove: standard naming names
// an entry by the skill's own name, which a move never changes, so a collision
// there was there before and does not block the move.
func TestPlan_StandardNamingCollisionExistedBeforeTheMove(t *testing.T) {
	f := newFixture(t)
	f.skill(t, "a/demo")
	f.skill(t, "b/demo")
	f.opts.Targets = []Target{{Name: "claude", Config: config.ResourceTargetConfig{TargetNaming: "standard"}}}

	planned := f.plan(t, "c", "a/demo")

	wantMoved(t, planned[0])
}

func TestPlan_FilterWarningsInBothDirections(t *testing.T) {
	f := newFixture(t)
	f.skill(t, "demo")
	f.skill(t, "other")
	f.opts.Targets = []Target{
		{Name: "named", Config: config.ResourceTargetConfig{Include: []string{"demo"}}},     // synced before, not after
		{Name: "dropped", Config: config.ResourceTargetConfig{Exclude: []string{"grp__*"}}}, // synced before, excluded after
		{Name: "added", Config: config.ResourceTargetConfig{Include: []string{"grp__*"}}},   // not synced before, synced after
		{Name: "same", Config: config.ResourceTargetConfig{}},
	}

	planned := f.plan(t, "grp", "demo")

	wantMoved(t, planned[0])
	joined := strings.Join(planned[0].Warnings, "\n")
	for _, target := range []string{"target named", "target dropped", "target added"} {
		if !strings.Contains(joined, target) {
			t.Errorf("no warning for %q in:\n%s", target, joined)
		}
	}
	if strings.Contains(joined, "target same") {
		t.Errorf("a target whose filters decide the same got a warning:\n%s", joined)
	}
}

func TestPlan_SkillignoreWarningsAndLiteralRewrite(t *testing.T) {
	f := newFixture(t)
	f.skill(t, "demo")
	f.skill(t, "globbed")
	f.file(t, ".skillignore", "# keep\ndemo\n!other\n/globbed\n")
	f.skill(t, "other")

	planned := f.plan(t, "grp", "demo", "globbed")
	wantMoved(t, planned[0])
	wantMoved(t, planned[1])
	if len(planned[0].Warnings) != 0 {
		t.Errorf("a literal rule follows the skill, yet warned: %v", planned[0].Warnings)
	}
	if got := strings.Join(planned[1].Warnings, "\n"); !strings.Contains(got, "grp/globbed becomes enabled") {
		t.Errorf("warnings = %q, want the glob that no longer matches", got)
	}
	Run(planned, f.opts)

	data, err := os.ReadFile(f.path(".skillignore"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "# keep\ngrp/demo\n!other\n/globbed\n"; string(data) != want {
		t.Errorf(".skillignore = %q, want the literal rewritten in place: %q", data, want)
	}
}

func TestRun_DryRunChangesNothing(t *testing.T) {
	f := newFixture(t)
	f.skill(t, "demo")
	f.record("demo")
	f.opts.DryRun = true

	out := Run(f.plan(t, "grp", "demo"), f.opts)

	wantMoved(t, out.Items[0])
	if !f.exists("demo/SKILL.md") || f.exists("grp") {
		t.Error("dry run moved files")
	}
	if !f.store.Has("demo") || f.store.Has("grp/demo") {
		t.Error("dry run re-keyed the store")
	}
	if _, err := os.Stat(filepath.Join(f.source, install.MetadataFileName)); err == nil {
		t.Error("dry run wrote .metadata.json")
	}
}

func TestRun_FailedRenameChangesNothingForThatName(t *testing.T) {
	f := newFixture(t)
	f.skill(t, "demo")
	f.skill(t, "ok")
	f.record("demo")
	f.record("ok")
	planned := f.plan(t, "grp", "demo", "ok")
	// A file where the destination folder should go: the plan was made before.
	f.file(t, "grp", "in the way")

	out := Run(planned, f.opts)

	wantRefusal(t, out.Items[0], CodeMoveFailed)
	wantRefusal(t, out.Items[1], CodeMoveFailed)
	if !f.exists("demo/SKILL.md") || !f.store.Has("demo") || f.store.Has("grp/demo") {
		t.Error("a failed move left the skill or its record half-moved")
	}
}

func TestRun_ReconcileRunsAfterRecordsAreSaved(t *testing.T) {
	f := newFixture(t)
	f.skill(t, "demo")
	f.record("demo")
	calls := 0
	f.opts.Reconcile = func() error {
		calls++
		saved, err := install.LoadMetadata(f.source)
		if err != nil || !saved.Has("grp/demo") {
			t.Errorf("reconcile saw keys %v (err %v), want the re-keyed store already on disk", saved.List(), err)
		}
		return errors.New("reconcile failed")
	}

	out := Run(f.plan(t, "grp", "demo"), f.opts)

	if calls != 1 {
		t.Errorf("reconcile ran %d times, want once", calls)
	}
	wantMoved(t, out.Items[0])
	if out.Err == nil || !strings.Contains(out.Err.Error(), "reconcile") {
		t.Errorf("Err = %v, want the reconcile failure reported without undoing the move", out.Err)
	}
}

func TestRun_ProjectModeCarriesPinGroupAndGitignore(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, ".skillshare", "skills")
	skill := filepath.Join(source, "demo")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: demo\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := projectdir.Resolve(root)
	const pinned = "0123456789abcdef0123456789abcdef01234567"
	lock := &install.Lock{Skills: map[string]install.LockEntry{"demo": {Source: "github.com/user/repo/demo", Commit: pinned}}}
	if err := lock.Save(dir); err != nil {
		t.Fatal(err)
	}
	gitignore := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(gitignore, []byte("# BEGIN SKILLSHARE MANAGED - DO NOT EDIT\nskills/demo/\n# END SKILLSHARE MANAGED\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.ProjectConfig{
		Targets: []config.ProjectTargetEntry{{Name: "claude"}},
		Skills:  []config.SkillEntry{{Name: "demo", Source: "github.com/user/repo/demo"}},
	}
	store := install.NewMetadataStore()
	store.Set("demo", &install.MetadataEntry{Source: "github.com/user/repo/demo", Commit: strings.Repeat("b", 40)})
	gitDir, prefix := config.ProjectGitignoreTarget(root, source)
	o := Options{
		SourceDir: source, Store: store, ProjectRoot: root,
		GitignoreDir: gitDir, GitignorePrefix: prefix,
		Reconcile: func() error { return config.ReconcileProjectSkills(root, cfg, store, source) },
	}
	discovered, err := sync.DiscoverSourceSkillsAll(source)
	if err != nil {
		t.Fatal(err)
	}

	out := Run(Plan(discovered, []string{"demo"}, "grp", o), o)

	if out.Err != nil {
		t.Fatal(out.Err)
	}
	if len(cfg.Skills) != 1 || cfg.Skills[0].Group != "grp" {
		t.Errorf("config skills = %+v, want one entry in group grp", cfg.Skills)
	}
	got, err := install.LoadLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Skills["grp/demo"].Commit != pinned || len(got.Skills) != 1 {
		t.Errorf("lock = %+v, want the older pin carried to grp/demo", got.Skills)
	}
	data, _ := os.ReadFile(gitignore)
	if strings.Contains(string(data), "skills/demo/") || !strings.Contains(string(data), "skills/grp/demo/") {
		t.Errorf(".gitignore = %q, want the entry swapped", data)
	}
}

// TestRun_NestedRecordsSurviveReconcileUnderMovedParent is the end-to-end
// scenario of the reconcile change: after the move reconcile walks, stops at
// the installed parent and must not prune the nested record with its findings.
func TestRun_NestedRecordsSurviveReconcileUnderMovedParent(t *testing.T) {
	f := newFixture(t)
	f.skill(t, "foo")
	f.skill(t, "foo/sub")
	f.record("foo")
	f.record("foo/sub")
	f.store.AuditAccepted = map[string][]string{"foo/sub": {"nested-key"}}
	t.Setenv("SKILLSHARE_CONFIG", filepath.Join(t.TempDir(), "config.yaml"))
	f.opts.Reconcile = func() error {
		return config.ReconcileGlobalSkills(&config.Config{Source: f.source}, f.store)
	}

	out := Run(f.plan(t, "grp", "foo"), f.opts)

	if out.Err != nil {
		t.Fatal(out.Err)
	}
	if !f.store.Has("grp/foo/sub") || len(f.store.AuditAccepted["grp/foo/sub"]) != 1 {
		t.Errorf("keys = %v, accepted = %v; want the nested record and its findings kept", f.store.List(), f.store.AuditAccepted)
	}
}

// Force only accepts a name collision: it never turns a refused destination or
// batch conflict into a move.
func TestPlan_ForceDoesNotAcceptOtherRefusals(t *testing.T) {
	f := newFixture(t)
	f.skill(t, "frontend/foo")
	f.skill(t, "backend/foo")
	f.skill(t, "taken")
	f.skill(t, "archive/taken")
	f.opts.Force = true

	dup := f.plan(t, "archive", "frontend/foo", "backend/foo")
	wantRefusal(t, dup[0], CodeDuplicateDest)
	wantRefusal(t, dup[1], CodeDuplicateDest)

	wantRefusal(t, f.plan(t, "archive", "taken")[0], CodeDestExists)
}

// An install --into folder below a source link is the install's business: it
// writes through a followed link, or sourcefs refuses it. A skill folder is
// refused either way.
func TestCheckDest_InstallLeavesLinksToTheInstall(t *testing.T) {
	f := newFixture(t)
	f.link(t, "dev")
	f.skill(t, "holder")

	if _, r := CheckDest("dev/inner", f.opts); r == nil || r.Code != CodeLinkedFolder {
		t.Fatalf("move: refusal = %v, want linked_folder", r)
	}
	f.opts.Install = true
	if _, r := CheckDest("dev/inner", f.opts); r != nil {
		t.Errorf("install: refusal = %v, want none", r)
	}
	if _, r := CheckDest("holder/inner", f.opts); r == nil || r.Code != CodeDestIsSkill {
		t.Errorf("install into a skill: refusal = %v, want dest_is_skill", r)
	}
}

// An absolute destination or name is refused, not read as a path in the source.
func TestPlan_AbsolutePathsAreNotRelativeToTheSource(t *testing.T) {
	f := newFixture(t)
	f.skill(t, "demo")
	f.skill(t, "tmp/x")

	wantRefusal(t, f.plan(t, "/etc/ssh", "demo")[0], CodeInvalidDest)
	wantRefusal(t, f.plan(t, "grp", "/tmp/x")[0], CodeNotFound)
}

// A literal line follows its skill and its folder, and so does a negation that
// keeps one skill of a disabled folder on.
func TestRun_SkillignoreFollowsNegationAndFolder(t *testing.T) {
	f := newFixture(t)
	f.skill(t, "legacy/tools/a")
	f.skill(t, "grp/demo")
	f.file(t, ".skillignore", "legacy/tools\ngrp/*\n!grp/demo\n")

	planned := f.plan(t, "archive", "legacy/tools", "grp/demo")
	wantMoved(t, planned[0])
	wantMoved(t, planned[1])
	out := Run(planned, f.opts)

	if out.Err != nil {
		t.Fatal(out.Err)
	}
	data, _ := os.ReadFile(f.path(".skillignore"))
	if want := "archive/tools\ngrp/*\n!archive/demo\n"; string(data) != want {
		t.Errorf(".skillignore = %q, want %q", data, want)
	}
}

// A record below an installed skill that has no SKILL.md of its own is no
// discovered skill, yet its lock pin must follow the move.
func TestRun_PinOfRecordWithoutSkillFollows(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, ".skillshare", "skills")
	for _, rel := range []string{"foo", "foo/data"} {
		if err := os.MkdirAll(filepath.Join(source, rel), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(source, "foo", "SKILL.md"), []byte("---\nname: foo\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := install.NewMetadataStore()
	store.Set("foo", &install.MetadataEntry{Source: "s/foo"})
	store.Set("foo/data", &install.MetadataEntry{Source: "s/data", Group: "foo"})
	dir := projectdir.Resolve(root)
	const pinned = "0123456789abcdef0123456789abcdef01234567"
	lock := &install.Lock{Skills: map[string]install.LockEntry{"foo/data": {Source: "s/data", Commit: pinned}}}
	if err := lock.Save(dir); err != nil {
		t.Fatal(err)
	}
	o := Options{SourceDir: source, Store: store, ProjectRoot: root}
	discovered, err := sync.DiscoverSourceSkillsAll(source)
	if err != nil {
		t.Fatal(err)
	}

	out := Run(Plan(discovered, []string{"foo"}, "grp", o), o)

	if out.Err != nil {
		t.Fatal(out.Err)
	}
	got, _ := install.LoadLock(dir)
	if got.Skills["grp/foo/data"].Commit != pinned || len(got.Skills) != 1 {
		t.Errorf("lock = %+v, want the pin under grp/foo/data", got.Skills)
	}
}
