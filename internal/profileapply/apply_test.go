package profileapply

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"skillshare/internal/backup"
	"skillshare/internal/config"
	"skillshare/internal/profile"
	ssync "skillshare/internal/sync"
)

type fixture struct {
	scope                Scope
	source, target, path string
}

func newFixture(t *testing.T, project bool, mode string) fixture {
	t.Helper()
	root := t.TempDir()
	for _, key := range []string{"HOME", "USERPROFILE", "APPDATA", "LOCALAPPDATA"} {
		t.Setenv(key, root)
	}
	for _, key := range []string{"CODEX_HOME", "CLAUDE_CONFIG_DIR", "PI_CODING_AGENT_DIR"} {
		t.Setenv(key, "")
	}
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	f := fixture{source: filepath.Join(root, "skills"), target: filepath.Join(root, "target"), path: filepath.Join(root, "config.yaml")}
	definitions := "profiles:\n  a:\n    skills:\n      include: [alpha]\n  b:\n    skills:\n      include: [beta]\n  empty:\n    skills:\n      include: []\n"
	if project {
		f.scope.ProjectRoot = root
		f.path = config.ProjectConfigPath(root)
		write(t, f.path, fmt.Sprintf("sources:\n  skills: %s\ntargets:\n  - name: custom\n    skills:\n      path: %s\n      mode: %s\n%s", f.source, f.target, mode, definitions))
	} else {
		write(t, f.path, fmt.Sprintf("source: %s\ntargets:\n  custom:\n    skills:\n      path: %s\n      mode: %s\n%s", f.source, f.target, mode, definitions))
	}
	f.scope.ConfigPath = f.path
	t.Setenv("SKILLSHARE_CONFIG", f.path)
	for _, name := range []string{"alpha", "beta"} {
		write(t, filepath.Join(f.source, name, "SKILL.md"), "---\nname: "+name+"\n---\n# "+name+"\n")
	}
	return f
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestProfileSync_DiscardEditsMovedDirectories(t *testing.T) {
	for _, name := range []string{"goose", "cline"} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, true, "copy")
			write(t, f.path, strings.Replace(read(t, f.path), "  - name: custom\n    skills:\n      path: "+f.target, "  - name: "+name+"\n    skills:", 1))
			if _, err := Apply(context.Background(), Request{Scope: f.scope, Name: "a"}); err != nil {
				t.Fatal(err)
			}
			v, err := Load(f.scope)
			if err != nil {
				t.Fatal(err)
			}
			target := v.Config.Targets[name]
			current := target.SkillsConfig().Path
			dirs := map[string]string{name: current}
			for _, rel := range config.AlsoScansProject(name) {
				dirs[backup.MovedEntryName(name, rel)] = filepath.Join(f.scope.ProjectRoot, filepath.FromSlash(rel))
			}
			for key, dir := range dirs {
				manifest := &ssync.Manifest{Managed: map[string]string{}, Outputs: map[string]string{}}
				for _, entry := range []string{"alpha", "retired"} {
					write(t, filepath.Join(dir, entry, "SKILL.md"), read(t, filepath.Join(f.source, "alpha", "SKILL.md")))
					sum, err := ssync.CopyOutputFingerprint(filepath.Join(dir, entry))
					if err != nil {
						t.Fatal(err)
					}
					manifest.Managed[entry], manifest.Outputs[entry] = sum, sum
					write(t, filepath.Join(dir, entry, "SKILL.md"), "authored "+key+"/"+entry)
				}
				if err := ssync.WriteManifest(dir, manifest); err != nil {
					t.Fatal(err)
				}
				write(t, filepath.Join(dir, "local", "notes.txt"), "unmanaged "+key)
			}
			const sourceContent = "---\nname: alpha\n---\n# changed source\n"
			write(t, filepath.Join(f.source, "alpha", "SKILL.md"), sourceContent)
			before := tree(t, f.scope.ProjectRoot)
			preview, err := WithSkillSync(context.Background(), SyncRequest{Scope: f.scope, DiscardEdits: true, DryRun: true})
			if err != nil || len(preview.Snapshot) != 0 || !reflect.DeepEqual(before, tree(t, f.scope.ProjectRoot)) {
				t.Fatalf("dry discard wrote files: %+v, %v", preview, err)
			}
			if len(preview.Blocked) != 2*len(dirs) {
				t.Fatalf("dry plan omitted actual-folder edits: %+v", preview)
			}
			configBefore, stateBefore := read(t, f.path), read(t, v.StatePath)
			out, err := WithSkillSync(context.Background(), SyncRequest{Scope: f.scope, DiscardEdits: true})
			if err != nil || !out.Complete || len(out.Snapshot) != len(dirs) {
				t.Fatalf("actual-folder snapshots: %+v, %v; want %d", out, err, len(dirs))
			}
			currentAfter := tree(t, current)
			seen := make(map[string]bool)
			for _, saved := range out.Snapshot {
				key := filepath.Base(saved.Path)
				dir, ok := dirs[key]
				if !ok || seen[key] || saved.Target != name {
					t.Fatalf("wrong or duplicate snapshot identity: %+v", saved)
				}
				seen[key] = true
				if read(t, filepath.Join(saved.Path, "alpha", "SKILL.md")) != "authored "+key+"/alpha" || read(t, filepath.Join(saved.Path, "retired", "SKILL.md")) != "authored "+key+"/retired" || saved.Restore != v.RestoreCommand(key, saved.Path) {
					t.Fatalf("snapshot bytes or recovery command: %+v", saved)
				}
				resolved, err := backup.SkillsEntryDir(f.scope.ProjectRoot, key, v.Config.Targets)
				if err != nil || resolved != dir {
					t.Fatalf("recovery destination %q: %q, %v", key, resolved, err)
				}
				if read(t, filepath.Join(dir, "local", "notes.txt")) != "unmanaged "+key {
					t.Fatal("discard changed unmanaged content")
				}
				if dir == current {
					if _, err := os.Lstat(filepath.Join(dir, "retired")); !os.IsNotExist(err) {
						t.Fatalf("current departed copy remains: %v", err)
					}
					if read(t, filepath.Join(dir, "alpha", "SKILL.md")) != read(t, filepath.Join(f.source, "alpha", "SKILL.md")) {
						t.Fatal("current copy was not refreshed")
					}
					continue
				}
				for _, entry := range []string{"alpha", "retired"} {
					if _, err := os.Lstat(filepath.Join(dir, entry)); !os.IsNotExist(err) {
						t.Fatalf("listed moved copy remains: %s, %v", entry, err)
					}
				}
				if err := backup.RestoreToPath(filepath.Dir(saved.Path), key, resolved, backup.RestoreOptions{Force: true}); err != nil {
					t.Fatal(err)
				}
				if read(t, filepath.Join(dir, "alpha", "SKILL.md")) != "authored "+key+"/alpha" || read(t, filepath.Join(dir, "retired", "SKILL.md")) != "authored "+key+"/retired" || !reflect.DeepEqual(currentAfter, tree(t, current)) {
					t.Fatal("moved recovery lost bytes or changed current folder")
				}
			}
			if read(t, filepath.Join(f.source, "alpha", "SKILL.md")) != sourceContent || read(t, f.path) != configBefore || read(t, v.StatePath) != stateBefore {
				t.Fatal("discard changed source, config or settled state")
			}
		})
	}
}

func TestProfileSync_MovedCleanupScope(t *testing.T) {
	for _, action := range []string{"unselected", "owned", "switch"} {
		t.Run(action, func(t *testing.T) {
			f := newFixture(t, true, "copy")
			dir := filepath.Join(f.scope.ProjectRoot, ".goose", "skills")
			if action == "owned" {
				write(t, f.path, strings.ReplaceAll(read(t, f.path), f.target, dir))
			}
			write(t, f.path, strings.Replace(read(t, f.path), "profiles:", "  - name: goose\n    skills:\n      mode: copy\nprofiles:", 1))
			if _, err := Apply(context.Background(), Request{Scope: f.scope, Name: "a"}); err != nil {
				t.Fatal(err)
			}
			write(t, filepath.Join(dir, "retired", "SKILL.md"), "managed leftover")
			sum, err := ssync.CopyOutputFingerprint(filepath.Join(dir, "retired"))
			if err != nil {
				t.Fatal(err)
			}
			if err := ssync.WriteManifest(dir, &ssync.Manifest{Managed: map[string]string{"retired": sum}, Outputs: map[string]string{"retired": sum}}); err != nil {
				t.Fatal(err)
			}
			before := tree(t, dir)
			if action == "switch" {
				_, err = Apply(context.Background(), Request{Scope: f.scope, Name: "a"})
			} else {
				selected := "custom"
				if action == "owned" {
					selected = "goose"
				}
				_, err = WithSkillSync(context.Background(), SyncRequest{Scope: f.scope, Targets: []string{selected}})
			}
			if err != nil || !reflect.DeepEqual(before, tree(t, dir)) {
				t.Fatalf("%s cleanup changed protected folder: %v", action, err)
			}
		})
	}
}

func TestProfileApply_RoundTripAndPending(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, mode := range []string{"merge", "copy"} {
			t.Run(fmt.Sprintf("project=%v/%s", project, mode), func(t *testing.T) {
				f := newFixture(t, project, mode)
				original := read(t, f.path)
				for _, name := range []string{"a", "b", "a"} {
					out, err := Apply(context.Background(), Request{Scope: f.scope, Name: name})
					if err != nil || !out.Complete || out.Profile.Pending {
						t.Fatalf("%s: %+v, %v", name, out, err)
					}
					present, absent := "alpha", "beta"
					if name == "b" {
						present, absent = absent, present
					}
					if read(t, filepath.Join(f.target, present, "SKILL.md")) != read(t, filepath.Join(f.source, present, "SKILL.md")) {
						t.Fatal("round trip changed content")
					}
					if _, err := os.Lstat(filepath.Join(f.target, absent)); !os.IsNotExist(err) {
						t.Fatalf("departed entry remains: %v", err)
					}
				}
				out, err := Apply(context.Background(), Request{Scope: f.scope, None: true, NoSync: true})
				if err != nil || !out.Profile.Pending {
					t.Fatalf("no-sync: %+v, %v", out, err)
				}
				retry, err := WithSkillSync(context.Background(), SyncRequest{Scope: f.scope})
				if err != nil || !retry.Complete || retry.Profile.Pending {
					t.Fatalf("retry: %+v, %v", retry, err)
				}
				if read(t, f.path) != original {
					t.Fatal("switch rewrote config")
				}
			})
		}
	}
}

func TestProfileApply_GuardsAndScope(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, action := range []string{"read", "preview", "missing", "empty"} {
			t.Run(fmt.Sprintf("%v/%s", project, action), func(t *testing.T) {
				f := newFixture(t, project, "copy")
				root := filepath.Dir(f.source)
				before := tree(t, root)
				r := Request{Scope: f.scope, Name: "a", DryRun: action == "preview"}
				if action == "missing" || action == "empty" {
					r.Name = action
				}
				var err error
				if action == "read" {
					_, err = Load(f.scope)
				} else {
					_, err = Apply(context.Background(), r)
				}
				if action == "missing" || action == "empty" {
					if err == nil {
						t.Fatal("guard accepted")
					}
					_, lockPath, _ := config.ProfilePaths(f.path)
					if got := read(t, lockPath); got != "" {
						t.Fatal("lock contains data")
					}
					// The retained lock and its parent directories are the only
					// permitted new paths after a non-dry refusal.
					after := tree(t, root)
					for path := lockPath; path != root && path != filepath.Dir(path); path = filepath.Dir(path) {
						rel, _ := filepath.Rel(root, path)
						if _, existed := before[rel]; !existed {
							before[rel] = after[rel]
						}
					}
				} else if err != nil {
					t.Fatal(err)
				}
				if after := tree(t, root); !reflect.DeepEqual(before, after) {
					t.Fatalf("scope changed: before=%v after=%v", before, after)
				}
			})
		}
	}
}

// tree records link destinations without following them, file bytes and modes.
func tree(t *testing.T, root string) map[string]string {
	t.Helper()
	paths := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		value := info.Mode().String()
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			value += ":" + link
		} else if !entry.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value += ":" + string(data)
		}
		paths[rel] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

func TestProfileApply_SnapshotAndRestore(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprint(project), func(t *testing.T) {
			f := newFixture(t, project, "copy")
			if _, err := Apply(context.Background(), Request{Scope: f.scope, Name: "a"}); err != nil {
				t.Fatal(err)
			}
			edited := filepath.Join(f.target, "alpha", "SKILL.md")
			write(t, edited, "authored content")
			write(t, filepath.Join(f.source, "alpha", "SKILL.md"), "---\nname: alpha\n---\nchanged source\n")
			statePath, _, _ := config.ProfilePaths(f.path)
			previous := read(t, statePath)
			if _, err := Apply(context.Background(), Request{Scope: f.scope, Name: "b"}); !errors.Is(err, ErrRefused) {
				t.Fatalf("edited copy accepted: %v", err)
			}
			if read(t, edited) != "authored content" || read(t, statePath) != previous {
				t.Fatal("refusal wrote content or state")
			}
			ordinary, err := WithSkillSync(context.Background(), SyncRequest{Scope: f.scope})
			if err != nil || ordinary.Complete || read(t, edited) != "authored content" {
				t.Fatalf("ordinary sync lost edit: %+v, %v", ordinary, err)
			}
			out, err := Apply(context.Background(), Request{Scope: f.scope, Name: "b", DiscardEdits: true})
			if err != nil || !out.Complete || len(out.Snapshot) != 1 {
				t.Fatalf("discard: %+v, %v", out, err)
			}
			snapshot := out.Snapshot[0]
			if read(t, filepath.Join(snapshot.Path, "alpha", "SKILL.md")) != "authored content" {
				t.Fatal("snapshot lost edit")
			}
			if err := backup.RestoreToPath(filepath.Dir(snapshot.Path), snapshot.Target, f.target, backup.RestoreOptions{Force: true}); err != nil {
				t.Fatal(err)
			}
			if read(t, edited) != "authored content" {
				t.Fatal("restore lost edit")
			}
		})
	}
}

func TestProfileApply_SnapshotFailureAndSecondEdit(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, failure := range []string{"create", "missing", "stale", "after-snapshot"} {
			t.Run(fmt.Sprintf("%v/%s", project, failure), func(t *testing.T) {
				f := newFixture(t, project, "copy")
				if _, err := Apply(context.Background(), Request{Scope: f.scope, Name: "a"}); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(f.target, "alpha", "SKILL.md")
				write(t, path, "first edit")
				statePath, _, _ := config.ProfilePaths(f.path)
				previous := read(t, statePath)
				r := Request{Scope: f.scope, Name: "b", DiscardEdits: true}
				switch failure {
				case "create":
					r.snapshot = func(_, _, _ string) (string, error) { return "", errors.New("snapshot failed") }
				case "missing":
					r.snapshot = func(_, _, _ string) (string, error) { return "", nil }
				case "stale":
					stale := t.TempDir()
					write(t, filepath.Join(stale, "alpha", "SKILL.md"), "stale bytes")
					r.snapshot = func(_, _, _ string) (string, error) { return stale, nil }
				case "after-snapshot":
					r.afterSnapshot = func() { write(t, path, "second edit") }
				}
				out, err := Apply(context.Background(), r)
				if err == nil {
					t.Fatalf("unsafe discard completed: %+v", out)
				}
				if failure == "after-snapshot" {
					state, _ := profile.ReadState(statePath)
					if !state.Pending || state.Active != "b" || read(t, path) != "second edit" {
						t.Fatal("second edit was lost or incomplete state cleared")
					}
				} else if read(t, statePath) != previous || read(t, path) != "first edit" {
					t.Fatal("failed snapshot changed state or target")
				}
			})
		}
	}
}

func TestProfileApply_InterruptedRecovery(t *testing.T) {
	f := newFixture(t, false, "copy")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := Apply(ctx, Request{Scope: f.scope, Name: "a", afterRecord: cancel})
	if err == nil {
		t.Fatal("interrupted apply reported complete")
	}
	path, _, _ := config.ProfilePaths(f.path)
	state, err := profile.ReadState(path)
	if err != nil || !state.Pending || state.Active != "a" {
		t.Fatalf("interruption state: %+v, %v", state, err)
	}
	out, err := WithSkillSync(context.Background(), SyncRequest{Scope: f.scope, Targets: []string{"absent"}})
	if err != nil || out.Profile == nil || !out.Profile.Pending {
		t.Fatalf("subset settled pending: %+v, %v", out, err)
	}
	out, err = WithSkillSync(context.Background(), SyncRequest{Scope: f.scope})
	if err != nil || !out.Complete || out.Profile.Pending {
		t.Fatalf("retry did not settle: %+v, %v", out, err)
	}
}

func TestProfileApply_LockReloadAndTimeout(t *testing.T) {
	f := newFixture(t, false, "copy")
	_, lockPath, _ := config.ProfilePaths(f.path)
	unlock, err := profile.Lock(context.Background(), lockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := Apply(ctx, Request{Scope: f.scope, Name: "a"}); err == nil {
		t.Fatal("switch bypassed lock")
	}
	// Previews never wait on a held lock.
	if _, err := Apply(context.Background(), Request{Scope: f.scope, Name: "b", DryRun: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(f.target); !os.IsNotExist(err) {
		t.Fatal("lock refusal or preview wrote targets")
	}
	unlock()
	if _, err := Apply(context.Background(), Request{Scope: f.scope, Name: "a"}); err != nil {
		t.Fatal(err)
	}
	out, err := WithSkillSync(context.Background(), SyncRequest{Scope: f.scope})
	if err != nil || out.Profile.Name != "a" {
		t.Fatalf("sync used stale state: %+v %v", out, err)
	}
}

func TestProfileApply_ExcludedScope(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprint(project), func(t *testing.T) {
			f := newFixture(t, project, "copy")
			missing := filepath.Join(filepath.Dir(f.target), "missing-symlink")
			off := filepath.Join(filepath.Dir(f.target), "off")
			agents := filepath.Join(filepath.Dir(f.target), "agents")
			write(t, filepath.Join(agents, "agent.md"), "pending agent bytes")
			text := read(t, f.path)
			if project {
				text += fmt.Sprintf("  # scope fixture\n")
				text = strings.Replace(text, "targets:\n", fmt.Sprintf("targets:\n  - name: linked\n    skills:\n      path: %s\n      mode: symlink\n  - name: off\n    skills:\n      path: %s\n      enabled: false\n", missing, off), 1)
			} else {
				text = strings.Replace(text, "targets:\n", fmt.Sprintf("targets:\n  linked:\n    skills:\n      path: %s\n      mode: symlink\n  off:\n    skills:\n      path: %s\n      enabled: false\n", missing, off), 1)
			}
			write(t, f.path, text)
			for _, request := range []Request{{Scope: f.scope, Name: "a", DryRun: true}, {Scope: f.scope, Name: "empty"}, {Scope: f.scope, Name: "b"}} {
				out, err := Apply(context.Background(), request)
				if request.Name == "empty" {
					if !errors.Is(err, ErrRefused) {
						t.Fatal(err)
					}
				} else if err != nil {
					t.Fatal(err)
				}
				if len(out.SymlinkTargets) != 1 {
					t.Fatalf("symlink exclusion absent: %+v", out)
				}
				for _, path := range []string{missing, off} {
					if _, err := os.Lstat(path); !os.IsNotExist(err) {
						t.Fatalf("excluded target touched: %s", path)
					}
				}
				if read(t, f.path) != text || read(t, filepath.Join(agents, "agent.md")) != "pending agent bytes" {
					t.Fatal("scope changed")
				}
			}
		})
	}
}

func TestProfileApply_IncompleteAndUnmanaged(t *testing.T) {
	f := newFixture(t, false, "copy")
	write(t, filepath.Join(f.target, "alpha", "SKILL.md"), "unmanaged")
	if _, err := Apply(context.Background(), Request{Scope: f.scope, Name: "a", DiscardEdits: true}); !errors.Is(err, ErrRefused) {
		t.Fatalf("discard covered unmanaged folder: %v", err)
	}
	if read(t, filepath.Join(f.target, "alpha", "SKILL.md")) != "unmanaged" {
		t.Fatal("unmanaged content changed")
	}
	// An independent target still completes when another target cannot be written.
	bad := filepath.Join(filepath.Dir(f.target), "dangling")
	if err := os.Symlink(filepath.Join(filepath.Dir(f.target), "not-created"), bad); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	text := strings.Replace(read(t, f.path), "targets:\n", fmt.Sprintf("targets:\n  bad:\n    skills:\n      path: %s\n      mode: copy\n", bad), 1)
	write(t, f.path, text)
	out, err := Apply(context.Background(), Request{Scope: f.scope, Name: "b"})
	if err == nil || out.Complete || !out.Profile.Pending {
		t.Fatalf("partial result: %+v %v", out, err)
	}
	if read(t, filepath.Join(f.target, "beta", "SKILL.md")) != read(t, filepath.Join(f.source, "beta", "SKILL.md")) {
		t.Fatal("other target did not apply")
	}
}

func TestProfileApply_RefusalCodes(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprint(project), func(t *testing.T) {
			f := newFixture(t, project, "copy")
			check := func(err error, code RefusalCode, reason string) {
				t.Helper()
				var refused *Refusal
				if !errors.Is(err, ErrRefused) || !errors.As(err, &refused) || refused.Code != code || refused.Reason != reason || err.Error() != reason {
					t.Fatalf("refusal = %#v, %v; want %s: %s", refused, err, code, reason)
				}
			}
			_, err := Apply(context.Background(), Request{Scope: f.scope, Name: "empty"})
			check(err, "profile_empty", `profile "empty" selects no enabled skills; unmatched patterns: ; use --allow-empty to confirm`)
			_, err = WithSkillSync(context.Background(), SyncRequest{Scope: f.scope, DiscardEdits: true})
			check(err, "profile_refused", "--discard-edits requires an active profile or a pending profile switch")
			write(t, filepath.Join(f.target, "alpha", "SKILL.md"), "unmanaged")
			_, err = Apply(context.Background(), Request{Scope: f.scope, Name: "a"})
			check(err, "profile_blocked", "custom/alpha: entry is not a recorded copy; use skillshare collect, or --discard-edits for modified managed copies")

			f = newFixture(t, project, "copy")
			if _, err := Apply(context.Background(), Request{Scope: f.scope, Name: "a"}); err != nil {
				t.Fatal(err)
			}
			write(t, filepath.Join(f.target, "alpha", "SKILL.md"), "authored")
			_, err = Apply(context.Background(), Request{Scope: f.scope, Name: "b", DiscardEdits: true,
				snapshot: func(_, _, _ string) (string, error) { return "", errors.New("snapshot failed") }})
			check(err, "profile_refused", "snapshot custom failed (no edits discarded): snapshot failed")
			_, err = Apply(context.Background(), Request{Scope: f.scope})
			check(err, "profile_refused", "choose a profile name or --none; --no-sync cannot discard edits")
			_, err = Apply(context.Background(), Request{Scope: f.scope, Name: "missing"})
			var unavailable *profile.UnavailableError
			if !errors.Is(err, profile.ErrProfileUnavailable) || !errors.As(err, &unavailable) || errors.Is(err, ErrRefused) {
				t.Fatalf("unavailable profile changed error class: %v", err)
			}
			_, lockPath, _ := config.ProfilePaths(f.path)
			unlock, err := profile.Lock(context.Background(), lockPath)
			if err != nil {
				t.Fatal(err)
			}
			defer unlock()
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			_, err = Apply(ctx, Request{Scope: f.scope, Name: "b"})
			check(err, "profile_locked", fmt.Sprintf("another sync or profile switch is running for this config (lock %s): context canceled", lockPath))
		})
	}
}

func TestProfileRefusal_LiteralCompatibility(t *testing.T) {
	for _, code := range []RefusalCode{"", RefusalGeneric, "future_code"} {
		// Zero and unknown codes remain available for consumers' generic fallback.
		err := error(&Refusal{Reason: "same reason", Code: code})
		var refused *Refusal
		if !errors.Is(err, ErrRefused) || !errors.As(err, &refused) || refused.Code != code || refused.Reason != "same reason" || err.Error() != "same reason" {
			t.Fatalf("literal compatibility: %#v, %v", refused, err)
		}
	}
	legacy := &Refusal{Reason: "legacy reason"}
	if legacy.Code != "" || legacy.Error() != "legacy reason" || legacy.Unwrap() != ErrRefused {
		t.Fatalf("legacy literal changed: %#v", legacy)
	}
}
