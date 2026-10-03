package plugin

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/flock"
)

func (f *piFixture) settingsPath() string { return filepath.Join(f.agentDir, "settings.json") }

func (f *piFixture) rawGlobal(content string) {
	f.t.Helper()
	writeTree(f.t, f.agentDir, map[string]string{"settings.json": content})
}

func (f *piFixture) read(file string) string {
	f.t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		f.t.Fatal(err)
	}
	return string(data)
}

func (f *piFixture) apply(changes ...PiExtensionChange) (*PiExtensionsPlan, error) {
	f.t.Helper()
	plan, err := f.svc.PreviewPiExtensions(context.Background(), "pi", changes)
	if err != nil {
		return nil, err
	}
	return f.svc.ApplyPiExtensions(context.Background(), "pi", changes, plan.Revision)
}

func change(f *piFixture, index int, rel, action string) PiExtensionChange {
	return PiExtensionChange{Index: index, Source: f.pkg, Path: rel, Action: action}
}

// S14: only the extensions list changes; every other byte of the file, including
// other resource filters, [] and unknown keys, stays as written.
func TestPiExtensionsApplyChangesOnlyTheExtensionsList(t *testing.T) {
	f := newPiFixture(t)
	before := "{\n\t\"x-top\": 1,\n\t\"packages\": [\n\t\t{\"source\": \"" + f.pkg + "\", \"extensions\": [\"-extensions/b.ts\", \"!extensions/zz-*.ts\"], \"skills\": [], \"prompts\": [\"!prompts/commit.md\"], \"themes\": [], \"x-acme\": {\"keep\": true, \"nested\": [1]}},\n\t\t\"npm:@other/pkg\"\n\t],\n\t\"theme\": \"dark\"\n}\n"
	f.rawGlobal(before)
	if _, err := f.apply(change(f, 0, "extensions/b.ts", "select"), change(f, 0, "extensions/c.ts", "exclude")); err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(before, `["-extensions/b.ts", "!extensions/zz-*.ts"]`, `["!extensions/zz-*.ts","+extensions/b.ts","-extensions/c.ts"]`, 1)
	if got := f.read(f.settingsPath()); got != want {
		t.Fatalf("file:\n%s\nwant:\n%s", got, want)
	}
	assertRows(t, selections(f.view("pi").Packages[0]), "extensions/a.ts:loads", "extensions/b.ts:loads", "extensions/c.ts:skipped")
}

func TestPiExtensionsApplyTurnsAStringEntryIntoAnObject(t *testing.T) {
	f := newPiFixture(t)
	f.rawGlobal(`{"packages": ["` + f.pkg + `"]}`)
	if _, err := f.apply(change(f, 0, "extensions/a.ts", "exclude")); err != nil {
		t.Fatal(err)
	}
	if got, want := f.read(f.settingsPath()), `{"packages": [{"source":"`+f.pkg+`","extensions":["-extensions/a.ts"]}]}`; got != want {
		t.Fatalf("file %s, want %s", got, want)
	}
}

// Removing the last rule drops the key, so the package default applies again, but
// the entry stays an object: Pi's own toggle would collapse it and lose x-keep.
func TestPiExtensionsDefaultRemovesTheRuleWithoutCollapsingTheEntry(t *testing.T) {
	f := newPiFixture(t)
	f.rawGlobal(`{"packages": [{"source": "` + f.pkg + `", "extensions": ["-extensions/a.ts"], "x-keep": 1}]}`)
	if _, err := f.apply(change(f, 0, "extensions/a.ts", "default")); err != nil {
		t.Fatal(err)
	}
	if got, want := f.read(f.settingsPath()), `{"packages": [{"source": "`+f.pkg+`", "x-keep": 1}]}`; got != want {
		t.Fatalf("file %s, want %s", got, want)
	}
}

func TestPiExtensionsDefaultRemovesARuleForAMissingFile(t *testing.T) {
	f := newPiFixture(t)
	f.rawGlobal(`{"packages": [{"source": "` + f.pkg + `", "extensions": ["-extensions/gone.ts", "-extensions/a.ts"]}]}`)
	if _, err := f.apply(change(f, 0, "extensions/gone.ts", "default")); err != nil {
		t.Fatal(err)
	}
	if got := f.read(f.settingsPath()); !strings.Contains(got, `"extensions": ["-extensions/a.ts"]`) {
		t.Fatalf("file %s", got)
	}
}

// With "extensions": [] nothing loads; adding "+a" would turn every extension back
// on (an empty include list means all), so the change is refused, even if a
// client sends it for a row the view shows without a switch.
func TestPiExtensionsRefusesAChangeThatMovesOtherExtensions(t *testing.T) {
	f := newPiFixture(t)
	before := `{"packages": [{"source": "` + f.pkg + `", "extensions": []}]}`
	f.rawGlobal(before)
	_, err := f.svc.PreviewPiExtensions(context.Background(), "pi", []PiExtensionChange{change(f, 0, "extensions/a.ts", "select")})
	if err == nil || !strings.Contains(err.Error(), "can't be changed here") {
		t.Fatalf("err = %v", err)
	}
	if f.read(f.settingsPath()) != before {
		t.Fatal("file changed")
	}
}

func TestPiExtensionsPreviewWritesNothing(t *testing.T) {
	f := newPiFixture(t)
	before := `{"packages": [{"source": "` + f.pkg + `", "extensions": ["-extensions/b.ts"]}]}`
	f.rawGlobal(before)
	plan, err := f.svc.PreviewPiExtensions(context.Background(), "pi", []PiExtensionChange{change(f, 0, "extensions/b.ts", "select")})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Entries) != 1 || !slices.Equal(plan.Entries[0].Before, []string{"-extensions/b.ts"}) || !slices.Equal(plan.Entries[0].After, []string{"+extensions/b.ts"}) {
		t.Fatalf("plan: %+v", plan.Entries)
	}
	if len(plan.Rows) != 1 || plan.Rows[0].Before != "skipped" || plan.Rows[0].After != "loads" {
		t.Fatalf("rows: %+v", plan.Rows)
	}
	entries, _ := os.ReadDir(f.agentDir)
	if len(entries) != 1 || f.read(f.settingsPath()) != before {
		t.Fatalf("preview touched the agent directory: %v", entries)
	}
	if _, err := os.Stat(f.svc.StateDir); !os.IsNotExist(err) {
		t.Fatalf("preview wrote state: %v", err)
	}
}

func TestPiExtensionsApplyRefusesAFileChangedSincePreview(t *testing.T) {
	f := newPiFixture(t)
	f.rawGlobal(`{"packages": [{"source": "` + f.pkg + `", "extensions": ["-extensions/b.ts"]}]}`)
	changes := []PiExtensionChange{change(f, 0, "extensions/b.ts", "select")}
	plan, err := f.svc.PreviewPiExtensions(context.Background(), "pi", changes)
	if err != nil {
		t.Fatal(err)
	}
	edited := `{"theme": "light", "packages": [{"source": "` + f.pkg + `", "extensions": ["-extensions/b.ts"]}]}`
	f.rawGlobal(edited)
	if _, err := f.svc.ApplyPiExtensions(context.Background(), "pi", changes, plan.Revision); !errors.Is(err, ErrPiExtensionsStale) {
		t.Fatalf("err = %v", err)
	}
	if f.read(f.settingsPath()) != edited {
		t.Fatal("a stale apply wrote the file")
	}
}

func TestPiExtensionsApplyRefusesWhileALockIsHeld(t *testing.T) {
	locks := map[string]func(f *piFixture) func(){
		"Pi's settings lock": func(f *piFixture) func() {
			if err := os.Mkdir(f.settingsPath()+".lock", 0o755); err != nil {
				t.Fatal(err)
			}
			return func() {
				if _, err := os.Stat(f.settingsPath() + ".lock"); err != nil {
					t.Fatal("an unowned Pi lock was removed")
				}
			}
		},
		"another plugin operation": func(f *piFixture) func() {
			writeTree(t, filepath.Dir(f.svc.ConfigPath), map[string]string{filepath.Base(f.svc.ConfigPath) + ".plugins.lock": ""})
			return func() {
				if _, err := os.Stat(f.svc.ConfigPath + ".plugins.lock"); err != nil {
					t.Fatal("an unowned plugin lock was removed")
				}
			}
		},
		"another Skillshare process": func(f *piFixture) func() {
			l := flock.New(f.settingsPath() + ".skillshare-plugin.lock")
			if ok, err := l.TryLock(); err != nil || !ok {
				t.Fatal("cannot hold the lock")
			}
			return func() { _ = l.Unlock() }
		},
	}
	for name, hold := range locks {
		t.Run(name, func(t *testing.T) {
			f := newPiFixture(t)
			before := `{"packages": [{"source": "` + f.pkg + `", "extensions": ["-extensions/b.ts"]}]}`
			f.rawGlobal(before)
			changes := []PiExtensionChange{change(f, 0, "extensions/b.ts", "select")}
			plan, err := f.svc.PreviewPiExtensions(context.Background(), "pi", changes)
			if err != nil {
				t.Fatal(err)
			}
			check := hold(f)
			if _, err := f.svc.ApplyPiExtensions(context.Background(), "pi", changes, plan.Revision); !errors.Is(err, ErrPiExtensionsBusy) {
				t.Fatalf("err = %v", err)
			}
			check()
			if f.read(f.settingsPath()) != before {
				t.Fatal("file changed under a lock")
			}
		})
	}
}

func TestPiExtensionsApplyReleasesItsLocksAndKeepsABackup(t *testing.T) {
	f := newPiFixture(t)
	f.rawGlobal(`{"packages": [{"source": "` + f.pkg + `", "extensions": ["-extensions/b.ts"]}]}`)
	if err := os.Chmod(f.settingsPath(), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := f.apply(change(f, 0, "extensions/b.ts", "select")); err != nil {
		t.Fatal(err)
	}
	for _, lock := range []string{f.settingsPath() + ".lock", f.svc.ConfigPath + ".plugins.lock"} {
		if _, err := os.Stat(lock); !os.IsNotExist(err) {
			t.Fatalf("%s left behind", lock)
		}
	}
	if info, _ := os.Stat(f.settingsPath()); info.Mode().Perm() != 0o640 {
		t.Fatalf("mode %v", info.Mode())
	}
	backups, _ := filepath.Glob(filepath.Join(f.svc.StateDir, "pi-extensions", "backups", "*.json"))
	if len(backups) != 1 {
		t.Fatalf("backups: %v", backups)
	}
	if data := f.read(backups[0]); !strings.Contains(data, `"-extensions/b.ts"`) || !strings.Contains(data, `"+extensions/b.ts"`) {
		t.Fatalf("backup: %s", data)
	}
}

func TestPiExtensionsWritesAreRefusedWhereReadOnly(t *testing.T) {
	cases := map[string]func(f *piFixture){
		"unsupported Pi": func(f *piFixture) { f.version = "0.99.1" },
		"Pi missing":     func(f *piFixture) { f.version = "" },
		"project with unsupported Pi": func(f *piFixture) {
			f.svc.ProjectRoot, f.version = filepath.Join(f.home, "code", "acme"), "0.99.1"
		},
		"settings not JSON": func(f *piFixture) { f.rawGlobal("{\n// comment\n\"packages\": [\"" + f.pkg + "\"]}") },
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			f := newPiFixture(t)
			f.rawGlobal(`{"packages": ["` + f.pkg + `"]}`)
			project := filepath.Join(f.home, "code", "acme", ".pi", "settings.json")
			projectBefore := `{"packages": ["` + f.pkg + `"]}`
			writeTree(t, filepath.Dir(project), map[string]string{"settings.json": projectBefore})
			setup(f)
			globalBefore := f.read(f.settingsPath())
			changes := []PiExtensionChange{change(f, 0, "extensions/a.ts", "exclude")}
			if _, err := f.svc.PreviewPiExtensions(context.Background(), "pi", changes); !errors.Is(err, ErrPiExtensionsReadOnly) {
				t.Fatalf("preview err = %v", err)
			}
			if _, err := f.svc.ApplyPiExtensions(context.Background(), "pi", changes, "any"); !errors.Is(err, ErrPiExtensionsReadOnly) {
				t.Fatalf("apply err = %v", err)
			}
			if f.read(f.settingsPath()) != globalBefore || f.read(project) != projectBefore {
				t.Fatal("a read-only target was written")
			}
		})
	}
}

func TestPiExtensionsRejectsInvalidChanges(t *testing.T) {
	f := newPiFixture(t)
	f.global(map[string]any{"packages": []any{map[string]any{"source": f.pkg, "extensions": []string{"!extensions/**"}}}})
	for name, c := range map[string]PiExtensionChange{
		"unknown row":          change(f, 0, "extensions/b.ts", "select"),
		"entry moved":          {Index: 0, Source: "/elsewhere", Path: "extensions/b.ts", Action: "select"},
		"no such entry":        {Index: 3, Source: f.pkg, Path: "extensions/b.ts", Action: "select"},
		"no such extension":    change(f, 0, "extensions/zz.ts", "select"),
		"default with no rule": change(f, 0, "extensions/b.ts", "default"),
		"unknown action":       change(f, 0, "extensions/b.ts", "enable"),
	} {
		if _, err := f.svc.PreviewPiExtensions(context.Background(), "pi", []PiExtensionChange{c}); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := f.svc.PreviewPiExtensions(context.Background(), "pi", nil); err == nil {
		t.Error("an empty change list was accepted")
	}
}

// Pi breaks a settings lock it finds stale and takes it; an apply that lost its
// lock that way, or whose lock someone else touched, must write nothing and leave
// the new owner's lock alone.
func TestPiExtensionsApplyRefusesWhenItsPiLockIsTakenOver(t *testing.T) {
	takeovers := map[string]func(t *testing.T, dir string){
		"replaced by Pi": func(t *testing.T, dir string) {
			if err := os.Remove(dir); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(dir, 0o755); err != nil {
				t.Fatal(err)
			}
		},
		"touched by another owner": func(t *testing.T, dir string) {
			old := time.Now().Add(-time.Minute)
			if err := os.Chtimes(dir, old, old); err != nil {
				t.Fatal(err)
			}
		},
		"removed": func(t *testing.T, dir string) {
			if err := os.Remove(dir); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, takeover := range takeovers {
		t.Run(name, func(t *testing.T) {
			f := newPiFixture(t)
			before := `{"packages": [{"source": "` + f.pkg + `", "extensions": ["-extensions/b.ts"]}]}`
			f.rawGlobal(before)
			var after os.FileInfo
			piBeforeWrite = func(dir string) {
				takeover(t, dir)
				after, _ = os.Lstat(dir)
			}
			t.Cleanup(func() { piBeforeWrite = func(string) {} })
			if _, err := f.apply(change(f, 0, "extensions/b.ts", "select")); !errors.Is(err, ErrPiExtensionsBusy) {
				t.Fatalf("err = %v", err)
			}
			if f.read(f.settingsPath()) != before {
				t.Fatal("the file was written after the lock was lost")
			}
			// Whatever holds the lock path now is not provably ours, so it stays as it is.
			now, err := os.Lstat(f.settingsPath() + ".lock")
			switch {
			case after == nil && !os.IsNotExist(err):
				t.Fatal("a lock directory appeared")
			case after != nil && (err != nil || !os.SameFile(now, after) || !now.ModTime().Equal(after.ModTime())):
				t.Fatal("a lock Skillshare no longer owns was removed or changed")
			}
		})
	}
}

func TestPiNativeLockRenewsItsMtimeWhileHeld(t *testing.T) {
	piLockRefresh = 10 * time.Millisecond
	t.Cleanup(func() { piLockRefresh = 2 * time.Second })
	dir := filepath.Join(t.TempDir(), "settings.json.lock")
	l, err := acquirePiNativeLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	first := l.ours.ModTime()
	deadline := time.Now().Add(5 * time.Second)
	for {
		info, err := os.Lstat(dir)
		if err != nil {
			t.Fatal(err)
		}
		if info.ModTime().After(first) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the lock mtime was never renewed")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err := l.verify(); err != nil {
		t.Fatalf("a renewed lock failed verification: %v", err)
	}
	l.release()
	if _, err := os.Lstat(dir); !os.IsNotExist(err) {
		t.Fatal("the lock was not released")
	}
}

func TestPiNativeLockRefusesAWriteWhenTooOldToBeSafe(t *testing.T) {
	piLockRefresh = time.Hour
	t.Cleanup(func() { piLockRefresh = 2 * time.Second })
	dir := filepath.Join(t.TempDir(), "settings.json.lock")
	l, err := acquirePiNativeLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer l.release()
	old := time.Now().Add(-(piLockStale - piLockMargin + time.Second))
	if err := os.Chtimes(dir, old, old); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Lstat(dir)
	l.mu.Lock()
	l.mtime = info.ModTime()
	l.mu.Unlock()
	if err := l.verify(); !errors.Is(err, ErrPiExtensionsBusy) {
		t.Fatalf("err = %v", err)
	}
}

// A revision is the preview of one change list on one target.
// The settings file is locked, but the package is not: Apply rebuilds the plan
// right before the write, so a package change that alters what was reviewed
// refuses it.
func TestPiExtensionsApplyRevalidatesThePackageAtTheWrite(t *testing.T) {
	for name, tc := range map[string]struct {
		entry, manifest string
		change          func(f *piFixture)
	}{
		"changed file removed": {entry: `{"source": "%s"}`, change: func(f *piFixture) {
			if err := os.Remove(filepath.Join(f.pkg, "extensions", "a.ts")); err != nil {
				f.t.Fatal(err)
			}
		}},
		"manifest changed": {entry: `{"source": "%s"}`, change: func(f *piFixture) {
			writeTree(f.t, f.pkg, map[string]string{"package.json": `{"name":"tools","pi":{"extensions":["extensions/b.ts"]}}`})
		}},
		// Under a manifest that lists only extensions, a string entry ignores a skills
		// folder that an object entry would load, so the conversion is no longer safe.
		"convention folder before string conversion": {entry: `"%s"`, manifest: `{"name":"tools","pi":{"extensions":["extensions"]}}`, change: func(f *piFixture) {
			writeTree(f.t, f.pkg, map[string]string{"skills/new/SKILL.md": "---\nname: new\ndescription: fixture\n---\n"})
		}},
	} {
		t.Run(name, func(t *testing.T) {
			f := newPiFixture(t)
			for _, dir := range []string{"skills", "prompts"} {
				if err := os.RemoveAll(filepath.Join(f.pkg, dir)); err != nil {
					t.Fatal(err)
				}
			}
			if tc.manifest != "" {
				writeTree(t, f.pkg, map[string]string{"package.json": tc.manifest})
			}
			before := `{"packages": [` + strings.ReplaceAll(tc.entry, "%s", f.pkg) + `]}`
			f.rawGlobal(before)
			piBeforeWrite = func(string) { tc.change(f) }
			t.Cleanup(func() { piBeforeWrite = func(string) {} })
			if _, err := f.apply(change(f, 0, "extensions/a.ts", "exclude")); !errors.Is(err, ErrPiExtensionsStale) {
				t.Fatalf("err = %v", err)
			}
			if f.read(f.settingsPath()) != before {
				t.Fatal("the file was written after the package changed")
			}
		})
	}
}

func TestPiExtensionsApplyRefusesARevisionOfAnotherPreview(t *testing.T) {
	f := newPiFixture(t)
	before := `{"packages": [{"source": "` + f.pkg + `", "extensions": ["-extensions/b.ts"]}]}`
	f.rawGlobal(before)
	previewed := []PiExtensionChange{change(f, 0, "extensions/b.ts", "select")}
	plan, err := f.svc.PreviewPiExtensions(context.Background(), "pi", previewed)
	if err != nil {
		t.Fatal(err)
	}
	other := []PiExtensionChange{change(f, 0, "extensions/a.ts", "exclude")}
	if _, err := f.svc.ApplyPiExtensions(context.Background(), "pi", other, plan.Revision); !errors.Is(err, ErrPiExtensionsStale) {
		t.Fatalf("err = %v", err)
	}
	if f.read(f.settingsPath()) != before {
		t.Fatal("an unpreviewed change was written")
	}
}

func TestPiExtensionsAbortedApplyKeepsBackupHistory(t *testing.T) {
	for _, scope := range []string{"global", "project-new", "project-existing"} {
		for _, failure := range []string{"package", "lock", "write"} {
			t.Run(scope+"/"+failure, func(t *testing.T) {
				f, root := projectFixture(t)
				f.global(map[string]any{"packages": []any{f.pkg}})
				file := f.projectFile(root)
				if scope == "global" {
					f.svc.ProjectRoot = ""
					file = f.settingsPath()
				} else if scope == "project-existing" {
					f.writeJSON(file, map[string]any{"theme": "dark"})
				}
				same := unchanged(t, f.settingsPath(), file)
				dir := filepath.Join(f.svc.StateDir, "pi-extensions", "backups")
				old := filepath.Join(dir, "previous.json")
				writeTree(t, dir, map[string]string{"previous.json": "previous successful record"})
				oldSame := unchanged(t, old)
				checkCount := func(want int) {
					t.Helper()
					entries, err := os.ReadDir(dir)
					if err != nil || len(entries) != want {
						t.Fatalf("backup records: %v, err=%v; want %d", entries, err, want)
					}
				}
				injected := errors.New("injected settings write failure")
				writes := 0
				piBeforeWrite = func(lock string) {
					checkCount(2) // The abort happens after this operation records its change.
					switch failure {
					case "package":
						if err := os.Remove(filepath.Join(f.pkg, "extensions", "a.ts")); err != nil {
							t.Fatal(err)
						}
					case "lock":
						old := time.Now().Add(-time.Minute)
						if err := os.Chtimes(lock, old, old); err != nil {
							t.Fatal(err)
						}
					}
				}
				if failure == "write" {
					fail := func() error { writes++; checkCount(2); return injected }
					piWriteGlobal = func(string, []byte, os.FileMode) error { return fail() }
					piWriteProject = func(*os.Root, string, []byte, os.FileMode, bool) error { return fail() }
				}
				t.Cleanup(func() {
					piBeforeWrite = func(string) {}
					piWriteGlobal, piWriteProject = atomicNativeWrite, rootAtomicWrite
				})
				c := PiExtensionChange{Scope: "global", Index: 0, Source: f.pkg, Path: "extensions/a.ts", Action: "exclude"}
				_, err := f.projectApply(c)
				want := ErrPiExtensionsStale
				if failure == "lock" {
					want = ErrPiExtensionsBusy
				} else if failure == "write" {
					want = injected
					if writes != 1 {
						t.Fatalf("settings writes: %d, want 1", writes)
					}
				}
				if !errors.Is(err, want) {
					t.Fatalf("err=%v, want %v", err, want)
				}
				same()
				oldSame()
				checkCount(1)
			})
		}
	}
}

func TestPiExtensionsApplyNeverPrunesBackups(t *testing.T) {
	f := newPiFixture(t)
	f.rawGlobal(`{"packages": [{"source": "` + f.pkg + `", "extensions": ["-extensions/b.ts"]}]}`)
	dir := filepath.Join(f.svc.StateDir, "pi-extensions", "backups")
	suffix := hash([]byte(f.settingsPath()))[:8]
	old := map[string]string{}
	for i := range 25 {
		old[strconv.Itoa(i+1)+"-"+suffix+".json"] = "{}\n"
	}
	writeTree(t, dir, old)
	if _, err := f.apply(change(f, 0, "extensions/b.ts", "select")); err != nil {
		t.Fatal(err)
	}
	if backups, _ := filepath.Glob(filepath.Join(dir, "*.json")); len(backups) != 26 {
		t.Fatalf("backups: %d", len(backups))
	}
}

// Pi's own proper-lockfile must still see the lock as held after its 10-second
// stale age, and take it once released. It needs Pi's install and Node, so it
// runs only with PI_ROOT set (the runbook's environment).
func TestPiNativeLockHoldsAgainstPi(t *testing.T) {
	root := os.Getenv("PI_ROOT")
	if root == "" {
		t.Skip("PI_ROOT not set")
	}
	file := filepath.Join(t.TempDir(), "settings.json")
	writeTree(t, filepath.Dir(file), map[string]string{"settings.json": "{}\n"})
	probe := func() string {
		cmd := exec.Command("node", filepath.Join("..", "..", "scripts", "pi", "lock-probe.mjs"), file)
		cmd.Env = append(os.Environ(), "PI_ROOT="+root)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("probe: %v %s", err, out)
		}
		return strings.TrimSpace(string(out))
	}
	l, err := acquirePiNativeLock(file + ".lock")
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(piLockStale + 2*time.Second)
	if got := probe(); got != "locked" {
		l.release()
		t.Fatalf("Pi after the stale age: %s", got)
	}
	if err := l.verify(); err != nil {
		t.Fatalf("lock lost: %v", err)
	}
	l.release()
	if got := probe(); got != "acquired" {
		t.Fatalf("Pi after release: %s", got)
	}
}

// Pi matches +/- rules against the file's absolute path too; such a rule belongs
// to the discovered file's row, and changing that row replaces it.
func TestPiExtensionsAbsoluteExactRuleBelongsToItsFile(t *testing.T) {
	for _, action := range []string{"default", "select"} {
		t.Run(action, func(t *testing.T) {
			f := newPiFixture(t)
			abs := "-" + filepath.ToSlash(filepath.Join(f.pkg, "extensions", "a.ts"))
			f.global(map[string]any{"packages": []any{map[string]any{"source": f.pkg, "extensions": []string{abs, "!extensions/c.ts"}}}})
			assertRows(t, selections(f.view("pi").Packages[0]), "extensions/a.ts:skipped", "extensions/b.ts:loads", "extensions/c.ts:skipped")
			plan, err := f.apply(change(f, 0, "extensions/a.ts", action))
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(plan.Rows[0].Removed, []string{abs}) || plan.Rows[0].After != "loads" {
				t.Fatalf("row: %+v", plan.Rows[0])
			}
			want := []string{"!extensions/c.ts"}
			if action == "select" {
				want = append(want, "+extensions/a.ts")
			}
			if !slices.Equal(plan.Entries[0].After, want) {
				t.Fatalf("after: %v", plan.Entries[0].After)
			}
			assertRows(t, selections(f.view("pi").Packages[0]), "extensions/a.ts:loads", "extensions/b.ts:loads", "extensions/c.ts:skipped")
		})
	}
}

// Removing an exact rule hands the file back to the rules that remain, which is
// not necessarily the package default.
func TestPiExtensionsRemovingARuleFollowsTheRemainingRules(t *testing.T) {
	for want, rules := range map[string][]string{
		"loads":   {"extensions/*.ts", "-extensions/a.ts"},
		"skipped": {"!extensions/a.ts", "+extensions/a.ts"},
	} {
		t.Run(want, func(t *testing.T) {
			f := newPiFixture(t)
			f.global(map[string]any{"packages": []any{map[string]any{"source": f.pkg, "extensions": rules}}})
			plan, err := f.apply(change(f, 0, "extensions/a.ts", "default"))
			if err != nil {
				t.Fatal(err)
			}
			if plan.Rows[0].After != want || !slices.Equal(plan.Entries[0].After, rules[:1]) {
				t.Fatalf("plan: %+v %v", plan.Rows[0], plan.Entries[0].After)
			}
		})
	}
}

// Any rule added to [] changes every extension of the package, so its rows have no switch.
func TestPiExtensionsEmptyListRowsAreReadOnly(t *testing.T) {
	f := newPiFixture(t)
	f.global(map[string]any{"packages": []any{map[string]any{"source": f.pkg, "extensions": []string{}}}})
	for _, r := range f.view("pi").Packages[0].Rows {
		if r.Editable || r.Origin != "emptyList" {
			t.Fatalf("row: %+v", r)
		}
	}
}

// otherResourcePackage is a package whose manifest names only its extension while
// it also has a skill and a prompt in the convention folders. A string entry loads
// neither; an object entry (which a first rule needs) loads both (Pi 0.99.2).
func otherResourcePackage(t *testing.T, root string, pi string) {
	t.Helper()
	writeTree(t, root, map[string]string{
		"package.json":           `{"name":"fixture","pi":` + pi + `}`,
		"extensions/a.ts":        "throw new Error('never imported')",
		"skills/review/SKILL.md": "---\nname: review\ndescription: fixture\n---\n",
		"prompts/commit.md":      "fixture\n",
	})
}

func TestPiExtensionsConversionThatWouldLoadOtherResourcesIsRefused(t *testing.T) {
	f := newPiFixture(t)
	pkg := filepath.Join(f.home, "pkgs", "partial")
	otherResourcePackage(t, pkg, `{"extensions":["extensions/a.ts"]}`)
	before := `{"packages": ["` + pkg + `"]}`
	f.rawGlobal(before)
	p := f.view("pi").Packages[0]
	if p.ReadOnly != "otherResources" || len(p.Rows) != 1 || p.Rows[0].Editable {
		t.Fatalf("package: %+v", p)
	}
	ch := PiExtensionChange{Index: 0, Source: pkg, Path: "extensions/a.ts", Action: "exclude"}
	if _, err := f.svc.PreviewPiExtensions(context.Background(), "pi", []PiExtensionChange{ch}); err == nil {
		t.Fatal("previewed a conversion that would load the skill and the prompt")
	}
	if _, err := f.svc.ApplyPiExtensions(context.Background(), "pi", []PiExtensionChange{ch}, "any"); err == nil {
		t.Fatal("applied a conversion that would load the skill and the prompt")
	}
	if got := f.read(f.settingsPath()); got != before {
		t.Fatalf("settings changed:\n%s", got)
	}
}

// The same entry stays controllable whenever converting it leaves the other
// resources as they were.
func TestPiExtensionsConversionThatKeepsOtherResourcesIsAllowed(t *testing.T) {
	cases := map[string]func(t *testing.T, pkg string) string{
		"no manifest": func(t *testing.T, pkg string) string {
			writeTree(t, pkg, map[string]string{"package.json": `{"name":"fixture"}`, "extensions/a.ts": "x", "skills/review/SKILL.md": "x", "prompts/commit.md": "x"})
			return `"` + pkg + `"`
		},
		"manifest declares every other resource": func(t *testing.T, pkg string) string {
			otherResourcePackage(t, pkg, `{"extensions":["extensions/a.ts"],"skills":[],"prompts":["prompts/commit.md"]}`)
			return `"` + pkg + `"`
		},
		"extension-only package": func(t *testing.T, pkg string) string {
			writeTree(t, pkg, map[string]string{"package.json": `{"name":"fixture","pi":{"extensions":["extensions/a.ts"]}}`, "extensions/a.ts": "x"})
			return `"` + pkg + `"`
		},
		"already an object entry": func(t *testing.T, pkg string) string {
			otherResourcePackage(t, pkg, `{"extensions":["extensions/a.ts"]}`)
			return `{"source": "` + pkg + `"}`
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			f := newPiFixture(t)
			pkg := filepath.Join(f.home, "pkgs", "fixture")
			f.rawGlobal(`{"packages": [` + setup(t, pkg) + `]}`)
			p := f.view("pi").Packages[0]
			if p.ReadOnly != "" || len(p.Rows) != 1 || !p.Rows[0].Editable {
				t.Fatalf("package: %+v", p)
			}
			if _, err := f.apply(PiExtensionChange{Index: 0, Source: pkg, Path: "extensions/a.ts", Action: "exclude"}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Pi loads a local source that is one file as that file and ignores any filter,
// so there is nothing to switch.
func TestPiExtensionsSingleFileSourceIsReadOnly(t *testing.T) {
	f := newPiFixture(t)
	file := filepath.Join(f.pkg, "extensions", "a.ts")
	f.rawGlobal(`{"packages": ["` + file + `"]}`)
	p := f.view("pi").Packages[0]
	if p.ReadOnly != "singleFile" || len(p.Rows) != 1 || p.Rows[0].Editable || p.Rows[0].Selection != "loads" {
		t.Fatalf("package: %+v", p)
	}
}

// piGuard refuses these on its own too, whatever the view said.
func TestPiGuardRefusesReadOnlyPackages(t *testing.T) {
	changes := []PiExtensionChange{{Path: "extensions/a.ts", Action: "exclude"}}
	for name, p := range map[string]*piPackage{"single file": {single: true}, "other resources": {convertLoadsOthers: true}} {
		if piGuard(p, piEntry{}, true, []string{"-extensions/a.ts"}, changes) == nil {
			t.Errorf("%s: allowed", name)
		}
	}
	if err := piGuard(&piPackage{convertLoadsOthers: true}, piEntry{object: true}, true, []string{"-extensions/a.ts"}, changes); err != nil && strings.Contains(err.Error(), "skills") {
		t.Errorf("an object entry was refused for other resources: %v", err)
	}
}

// Pi hands any single file to its extension loader and lists it as enabled, so
// that is what the row shows; whether it loads is never known here.
func TestPiExtensionsSingleNonCodeFileShowsPisSelection(t *testing.T) {
	f := newPiFixture(t)
	file := filepath.Join(f.home, "notes", "notes.md")
	writeTree(t, filepath.Dir(file), map[string]string{"notes.md": "fixture\n"})
	f.rawGlobal(`{"packages": ["` + file + `"]}`)
	p := f.view("pi").Packages[0]
	if p.ReadOnly != "singleFile" || len(p.Rows) != 1 || p.Rows[0].Selection != "loads" || p.Rows[0].Editable {
		t.Fatalf("package: %+v", p)
	}
}

// A themes folder counts like skills and prompts, and so does a link or anything
// else at that name: equivalence is not shown, so the string entry is read-only
// while its extensions stay listed.
func TestPiExtensionsConversionWithThemesOrALinkIsReadOnly(t *testing.T) {
	cases := map[string]func(t *testing.T, pkg string){
		"themes folder": func(t *testing.T, pkg string) {
			writeTree(t, pkg, map[string]string{"themes/dark.json": "{}"})
		},
		"skills link leaving the package": func(t *testing.T, pkg string) {
			outside := filepath.Join(filepath.Dir(pkg), "outside-skills")
			writeTree(t, outside, map[string]string{"review/SKILL.md": "x"})
			if err := os.Symlink(outside, filepath.Join(pkg, "skills")); err != nil {
				t.Fatal(err)
			}
		},
		"dangling prompts link": func(t *testing.T, pkg string) {
			if err := os.Symlink(filepath.Join(pkg, "nowhere"), filepath.Join(pkg, "prompts")); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, add := range cases {
		t.Run(name, func(t *testing.T) {
			f := newPiFixture(t)
			pkg := filepath.Join(f.home, "pkgs", "fixture")
			writeTree(t, pkg, map[string]string{"package.json": `{"name":"fixture","pi":{"extensions":["extensions/a.ts"]}}`, "extensions/a.ts": "x"})
			add(t, pkg)
			f.rawGlobal(`{"packages": ["` + pkg + `"]}`)
			p := f.view("pi").Packages[0]
			if p.Problem != "" || p.ReadOnly != "otherResources" || len(p.Rows) != 1 || p.Rows[0].Selection != "loads" || p.Rows[0].Editable {
				t.Fatalf("package: %+v", p)
			}
		})
	}
}

// encodingPackage has extensions/a.ts and a file named with a literal U+FFFD.
func encodingPackage(t *testing.T, root, pi string) {
	t.Helper()
	writeTree(t, root, map[string]string{
		"package.json":    `{"name":"fixture","pi":` + pi + `}`,
		"extensions/a.ts": "x",
		"extensions/�.ts": "x",
	})
}

// Go decodes an unpaired surrogate escape as U+FFFD, Pi (JavaScript) keeps it, so
// "!extensions/\ud800.ts" excludes nothing in Pi but would read here as excluding
// the U+FFFD file. Such an entry is left unread: no rules, no rows, no write.
func TestPiExtensionsUnpairedSurrogateRuleIsUnsupported(t *testing.T) {
	f := newPiFixture(t)
	pkg := filepath.Join(f.home, "pkgs", "enc")
	encodingPackage(t, pkg, `{"extensions":["extensions"]}`)
	before := `{"packages": [{"source": "` + pkg + `", "extensions": ["*", "!extensions/\ud800.ts", "-extensions/a.ts"]}]}`
	f.rawGlobal(before)
	p := f.view("pi").Packages[0]
	if p.Problem != "unsupportedEntry" || len(p.Rules) != 0 || len(p.Rows) != 0 {
		t.Fatalf("package: %+v", p)
	}
	ch := PiExtensionChange{Index: 0, Source: pkg, Path: "extensions/a.ts", Action: "select"}
	if _, err := f.svc.PreviewPiExtensions(context.Background(), "pi", []PiExtensionChange{ch}); err == nil {
		t.Fatal("previewed an entry whose rules can't be read losslessly")
	}
	if _, err := f.svc.ApplyPiExtensions(context.Background(), "pi", []PiExtensionChange{ch}, "any"); err == nil {
		t.Fatal("applied an entry whose rules can't be read losslessly")
	}
	if got := f.read(f.settingsPath()); got != before {
		t.Fatalf("settings changed:\n%s", got)
	}
}

// Strings that decode the same in Go and in Pi stay supported.
func TestPiExtensionsLosslessStringsStaySupported(t *testing.T) {
	cases := map[string]string{
		"surrogate pair":    `"-extensions/😀.ts"`,
		"literal U+FFFD":    "\"-extensions/�.ts\"",
		"escaped U+FFFD":    `"-extensions/�.ts"`,
		"escaped backslash": `"-extensions/\\ud800.ts"`,
		"emoji and BMP":     "\"-extensions/\U0001F600é.ts\"",
	}
	for name, rule := range cases {
		t.Run(name, func(t *testing.T) {
			f := newPiFixture(t)
			pkg := filepath.Join(f.home, "pkgs", "enc")
			encodingPackage(t, pkg, `{"extensions":["extensions"]}`)
			f.rawGlobal(`{"packages": [{"source": "` + pkg + `", "extensions": [` + rule + `]}]}`)
			if p := f.view("pi").Packages[0]; p.Problem != "" || len(p.Rules) != 1 {
				t.Fatalf("package: %+v", p)
			}
		})
	}
}

func TestPiExtensionsUnpairedSurrogateElsewhereIsUnsupported(t *testing.T) {
	t.Run("manifest declaration", func(t *testing.T) {
		f := newPiFixture(t)
		pkg := filepath.Join(f.home, "pkgs", "enc")
		encodingPackage(t, pkg, `{"extensions":["extensions/a.ts","extensions/\ud800.ts"]}`)
		f.rawGlobal(`{"packages": ["` + pkg + `"]}`)
		if p := f.view("pi").Packages[0]; p.Problem == "" || len(p.Rows) != 0 {
			t.Fatalf("package: %+v", p)
		}
	})
	t.Run("top-level extensions", func(t *testing.T) {
		f := newPiFixture(t)
		writeTree(t, filepath.Join(f.agentDir, "extensions"), map[string]string{"a.ts": "x", "�.ts": "x"})
		f.rawGlobal(`{"extensions": ["!\ud800.ts"]}`)
		if folder := f.view("pi").Folders[0]; folder.Problem != "unsupportedEntry" || len(folder.Rows) != 0 {
			t.Fatalf("folder: %+v", folder)
		}
	})
	t.Run("source", func(t *testing.T) {
		f := newPiFixture(t)
		f.rawGlobal(`{"packages": ["./pkgs/\udc00", {"source": "./pkgs/\udc00"}]}`)
		for _, p := range f.view("pi").Packages {
			if p.Problem != "unsupportedEntry" {
				t.Fatalf("package: %+v", p)
			}
		}
	})
}

// encoding/json reads null as "", which Pi would not take as a source.
func TestPiExtensionsEmptySourceIsUnsupported(t *testing.T) {
	f := newPiFixture(t)
	f.rawGlobal(`{"packages": [null, "", "  ", {"source": null}, {"source": " "}, {"source": ""}]}`)
	v := f.view("pi")
	if len(v.Packages) != 6 {
		t.Fatalf("packages: %+v", v.Packages)
	}
	for _, p := range v.Packages {
		if p.Problem != "unsupportedEntry" || len(p.Rows) != 0 {
			t.Fatalf("package: %+v", p)
		}
	}
}

func TestPiLossless(t *testing.T) {
	cases := map[string]bool{
		`"a"`:             true,
		`["😀", "é"]`:      true,
		`"\\ud800"`:       true,  // a backslash, then the text ud800
		`"\\\ud800"`:      false, // a backslash, then an unpaired surrogate
		`"\ud800"`:        false,
		`"\udc00"`:        false,
		`"\ud800A"`:       false,
		`"\ud800x"`:       false,
		"\"�\"":           true,
		"\"\xff\"":        false, // invalid UTF-8
		`{"k": "\ud800"}`: false,
	}
	for raw, want := range cases {
		if got := piLossless([]byte(raw)); got != want {
			t.Errorf("piLossless(%s) = %v, want %v", raw, got, want)
		}
	}
}
