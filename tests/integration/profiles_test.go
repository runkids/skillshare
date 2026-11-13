//go:build !online

package integration

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"skillshare/internal/backup"
	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/profile"
	ssync "skillshare/internal/sync"
	"skillshare/internal/testutil"
	"skillshare/internal/utils"
)

type profileFixture struct {
	sb                                 *testutil.Sandbox
	root, source, target, flag, config string
}

func newProfileFixture(t *testing.T, project bool, mode string) profileFixture {
	t.Helper()
	sb := testutil.NewSandbox(t)
	t.Cleanup(sb.Cleanup)
	// Explicit fixture homes isolate every agent override inherited by CI.
	for _, key := range []string{"CODEX_HOME", "CLAUDE_CONFIG_DIR", "PI_CODING_AGENT_DIR"} {
		sb.SetEnv(key, "")
	}
	f := profileFixture{sb: sb, root: sb.Root, source: sb.SourcePath, target: filepath.Join(sb.Home, "target"), flag: "-g", config: sb.ConfigPath}
	if project {
		f.root = sb.SetupProjectDir()
		f.source = filepath.Join(f.root, ".skillshare", "skills")
		f.target = filepath.Join(f.root, "target")
		f.flag = "-p"
		f.config = filepath.Join(f.root, ".skillshare", "config.yaml")
	}
	text := fmt.Sprintf("source: %s\nmode: %s\ntargets:\n  custom:\n    skills:\n      path: %s\n", f.source, mode, f.target)
	if project {
		text = fmt.Sprintf("targets:\n  - name: custom\n    skills:\n      path: %s\n      mode: %s\n", f.target, mode)
	}
	text += "profiles:\n  a:\n    skills:\n      include: [alpha]\n  b:\n    skills:\n      include: [beta]\n  all: {}\n  empty:\n    skills:\n      include: []\n"
	if project {
		sb.WriteProjectConfig(f.root, text)
	} else {
		sb.WriteConfig(text)
	}
	for _, name := range []string{"alpha", "beta"} {
		path := filepath.Join(f.source, name)
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte("---\nname: "+name+"\n---\n# "+name+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func (f profileFixture) run(args ...string) *testutil.Result {
	return f.sb.RunCLIInDir(f.root, append(args, f.flag)...)
}

func TestProfileCRUD(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprint(project), func(t *testing.T) {
			f := newProfileFixture(t, project, "copy")
			f.run("profile", "create", "work", "--add-include", "alpha").AssertSuccess(t)
			show := f.run("profile", "show", "work", "--json")
			show.AssertSuccess(t)
			show.AssertOutputContains(t, "alpha")
			f.run("profile", "use", "work", "--no-sync").AssertSuccess(t)
			f.run("profile", "rename", "work", "renamed").AssertSuccess(t)
			f.run("profile", "delete", "renamed").AssertFailure(t)
			f.run("sync", "--json").AssertSuccess(t)
			f.run("profile", "use", "--none").AssertSuccess(t)
			f.run("profile", "delete", "renamed").AssertSuccess(t)
			for _, args := range [][]string{{"create", "none"}, {"create", "Upper"}, {"create", "bad", "--add-include", "["}, {"create", "exclude", "--add-exclude", "x"}} {
				f.run(append([]string{"profile"}, args...)...).AssertFailure(t)
			}
		})
	}
}

func TestProfiles_ProjectSourceAvailability(t *testing.T) {
	for _, action := range []string{"use", "sync"} {
		for _, sourceState := range []string{"missing", "empty"} {
			t.Run(action+"/"+sourceState, func(t *testing.T) {
				f := newProfileFixture(t, true, "copy")
				f.run("profile", "use", "a").AssertSuccess(t)
				f.run("profile", "use", "a", "--no-sync").AssertSuccess(t)
				statePath, _, err := config.ProfilePaths(f.config)
				if err != nil {
					t.Fatal(err)
				}
				beforeState := f.sb.ReadFile(statePath)
				state, err := profile.ReadState(statePath)
				if err != nil || state.Active != "a" || !state.Pending {
					t.Fatalf("setup requires pending a: %+v, %v", state, err)
				}
				entry := filepath.Join(f.target, "alpha", "SKILL.md")
				beforeBytes := f.sb.ReadFile(entry)
				manifest := filepath.Join(f.target, ssync.ManifestFile)
				beforeManifest := f.sb.ReadFile(manifest)
				beforeConfig := f.sb.ReadFile(f.config)
				if err := os.Rename(f.source, f.source+"-offline"); err != nil {
					t.Fatal(err)
				}
				if sourceState == "empty" {
					if err := os.Mkdir(f.source, 0755); err != nil {
						t.Fatal(err)
					}
				}
				args := []string{"sync", "--json"}
				if action == "use" {
					args = []string{"profile", "use", "b", "--json"}
				}
				out := f.run(args...)
				afterBytes, readErr := os.ReadFile(entry)
				if sourceState == "missing" {
					out.AssertFailure(t)
					if readErr != nil || string(afterBytes) != beforeBytes {
						t.Errorf("missing source changed managed copy: %q, %v", afterBytes, readErr)
					}
					if f.sb.ReadFile(manifest) != beforeManifest || f.sb.ReadFile(statePath) != beforeState {
						t.Error("missing source changed manifest or settled pending state")
					}
				} else {
					out.AssertSuccess(t)
					if !os.IsNotExist(readErr) {
						t.Errorf("readable empty source did not prune managed copy: %v", readErr)
					}
					wantActive := "a"
					if action == "use" {
						wantActive = "b"
					}
					state, err = profile.ReadState(statePath)
					if err != nil || state.Active != wantActive || state.Pending {
						t.Errorf("empty-source sync did not settle %s: %+v, %v", wantActive, state, err)
					}
				}
				if f.sb.ReadFile(f.config) != beforeConfig || f.sb.ReadFile(filepath.Join(f.source+"-offline", "alpha", "SKILL.md")) != beforeBytes {
					t.Error("source availability check changed config or offline source")
				}
			})
		}
	}
}

func TestProfiles_DotfilesTargetRoot(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, mode := range []string{"copy", "merge"} {
			t.Run(fmt.Sprintf("%v/%s", project, mode), func(t *testing.T) {
				f := newProfileFixture(t, project, mode)
				realTarget := filepath.Join(f.root, "dotfiles-target")
				if err := os.MkdirAll(realTarget, 0755); err != nil {
					t.Fatal(err)
				}
				if runtime.GOOS == "windows" {
					if err := utils.CreateJunction(f.target, realTarget); err != nil {
						t.Fatal(err)
					}
				} else {
					f.sb.CreateSymlink(realTarget, f.target)
				}
				rootLink, err := os.Readlink(f.target)
				if err != nil {
					t.Fatal(err)
				}
				local := filepath.Join(realTarget, "local", "SKILL.md")
				f.sb.WriteFile(local, "unmanaged local content")
				f.run("sync", "--json").AssertSuccess(t)
				if f.sb.ReadFile(filepath.Join(realTarget, "beta", "SKILL.md")) != f.sb.ReadFile(filepath.Join(f.source, "beta", "SKILL.md")) {
					t.Fatal("ordinary sync did not populate configured alias")
				}
				f.run("profile", "use", "a", "--json").AssertSuccess(t)
				if _, err := os.Lstat(filepath.Join(realTarget, "beta")); !os.IsNotExist(err) {
					t.Errorf("profile did not prune managed beta through alias: %v", err)
				}
				if f.sb.ReadFile(filepath.Join(realTarget, "alpha", "SKILL.md")) != f.sb.ReadFile(filepath.Join(f.source, "alpha", "SKILL.md")) || f.sb.ReadFile(local) != "unmanaged local content" {
					t.Error("alias switch lost selected or unmanaged child content")
				}
				if after, err := os.Readlink(f.target); err != nil || after != rootLink || !utils.IsSymlinkOrJunction(f.target) {
					t.Errorf("configured root alias changed: %q, %v", after, err)
				}
			})
		}
	}
}

func TestProfiles_UseJSON(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, mode := range []string{"copy", "merge"} {
			t.Run(fmt.Sprintf("%v/%s", project, mode), func(t *testing.T) {
				f := newProfileFixture(t, project, mode)
				f.run("profile", "use", "a", "--json").AssertSuccess(t)
				for _, preview := range []bool{true, false} {
					t.Run(fmt.Sprintf("preview=%v", preview), func(t *testing.T) {
						args := []string{"profile", "use", "b", "--json"}
						if preview {
							args = append(args, "--dry-run")
						}
						out := f.run(args...)
						out.AssertSuccess(t)
						var body struct {
							Complete bool
							Profile  struct {
								Previous, Active string
								Pending          bool
							}
						}
						// Unmarshal consumes the whole stdout, rejecting chatter or a second response.
						if err := json.Unmarshal([]byte(out.Stdout), &body); err != nil {
							t.Errorf("profile use must emit one JSON response: %v; stdout=%q", err, out.Stdout)
						} else if !body.Complete || body.Profile.Previous != "a" || body.Profile.Active != "b" || body.Profile.Pending {
							t.Errorf("switch result lost selection/outcome: %s", out.Stdout)
						}
						if strings.TrimSpace(out.Stderr) != "" {
							t.Errorf("JSON switch leaked diagnostics: %s", out.Stderr)
						}
						want, absent := "beta", "alpha"
						if preview {
							want, absent = absent, want
						}
						if f.sb.ReadFile(filepath.Join(f.target, want, "SKILL.md")) != f.sb.ReadFile(filepath.Join(f.source, want, "SKILL.md")) {
							t.Error("JSON switch changed selected contents")
						}
						if _, err := os.Lstat(filepath.Join(f.target, absent)); !os.IsNotExist(err) {
							t.Errorf("preview/apply left wrong deployment: %s, %v", absent, err)
						}
					})
				}
			})
		}
	}
}

func TestProfiles_UseJSONRefusal(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprint(project), func(t *testing.T) {
			f := newProfileFixture(t, project, "copy")
			out := f.run("profile", "use", "missing", "--json")
			out.AssertFailure(t)
			var body struct{ Error string }
			if err := json.Unmarshal([]byte(out.Stdout), &body); err != nil {
				t.Fatalf("refusal must emit one JSON response: %v; stdout=%q", err, out.Stdout)
			}
			for _, detail := range []string{"missing", "not defined", f.config, "skillshare profile use <name>", "skillshare profile use --none"} {
				if !strings.Contains(body.Error, detail) {
					t.Errorf("JSON error lost refusal/repair %q: %s", detail, out.Stdout)
				}
			}
			if strings.TrimSpace(out.Stderr) != "" {
				t.Errorf("JSON refusal also emitted plain diagnostics: %s", out.Stderr)
			}
		})
	}
}

func TestProfiles_Agreement(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, mode := range []string{"copy", "merge"} {
			t.Run(fmt.Sprintf("%v/%s", project, mode), func(t *testing.T) {
				f := newProfileFixture(t, project, mode)
				before := f.sb.ReadFile(f.config)
				captureTarget := func() map[string]string {
					tree := make(map[string]string)
					err := filepath.WalkDir(f.target, func(path string, entry fs.DirEntry, err error) error {
						if err != nil {
							return err
						}
						rel, err := filepath.Rel(f.target, path)
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
							if entry.Name() == ssync.ManifestFile {
								var manifest map[string]json.RawMessage
								if err := json.Unmarshal(data, &manifest); err != nil {
									return err
								}
								// The writer refreshes only this wall-clock field on an idle sync.
								delete(manifest, "updated_at")
								data, err = json.Marshal(manifest)
								if err != nil {
									return err
								}
							}
							value += ":" + string(data)
						}
						tree[rel] = value
						return nil
					})
					if err != nil {
						t.Fatal(err)
					}
					return tree
				}
				for _, name := range []string{"a", "b", "a"} {
					f.run("profile", "use", name, "--json").AssertSuccess(t)
					want, absent := "alpha", "beta"
					if name == "b" {
						want, absent = absent, want
					}
					if f.sb.ReadFile(filepath.Join(f.target, want, "SKILL.md")) != f.sb.ReadFile(filepath.Join(f.source, want, "SKILL.md")) {
						t.Fatal("contents differ")
					}
					if _, err := os.Lstat(filepath.Join(f.target, absent)); !os.IsNotExist(err) {
						t.Fatalf("departure not pruned: %v", err)
					}
					diff := f.run("diff", "--json")
					diff.AssertSuccess(t)
					var d struct {
						Targets []struct {
							Synced bool `json:"synced"`
						} `json:"targets"`
					}
					if err := json.Unmarshal([]byte(diff.Stdout), &d); err != nil || len(d.Targets) != 1 || !d.Targets[0].Synced {
						t.Fatalf("diff disagrees: %s %v", diff.Stdout, err)
					}
					status := f.run("status", "--json")
					status.AssertSuccess(t)
					var s struct {
						Profile struct {
							Selected, Total int
							Pending         bool
						}
						Targets []struct {
							SyncedCount int `json:"synced_count"`
						}
					}
					if err := json.Unmarshal([]byte(status.Stdout), &s); err != nil || s.Profile.Selected != 1 || s.Profile.Total != 2 || s.Profile.Pending || len(s.Targets) != 1 || s.Targets[0].SyncedCount != 1 {
						t.Fatalf("status disagrees: %s %v", status.Stdout, err)
					}
					treeBefore := captureTarget()
					if _, ok := treeBefore[ssync.ManifestFile]; !ok {
						t.Fatal("managed manifest missing before second sync")
					}
					contentBefore := f.sb.ReadFile(filepath.Join(f.target, want, "SKILL.md"))
					second := f.run("sync", "--json")
					second.AssertSuccess(t)
					var sync struct {
						Linked, Updated, Pruned int
						Profile                 struct{ Pending bool }
					}
					wantLinked := 0
					if mode == "merge" {
						// Merge counts an already correct link in Linked, as the baseline does.
						wantLinked = 1
					}
					if err := json.Unmarshal([]byte(second.Stdout), &sync); err != nil || sync.Linked != wantLinked || sync.Updated != 0 || sync.Pruned != 0 || sync.Profile.Pending {
						t.Fatalf("second sync counters disagree: %s %v", second.Stdout, err)
					}
					if treeAfter := captureTarget(); !reflect.DeepEqual(treeBefore, treeAfter) {
						t.Fatalf("second sync changed target tree or managed manifest: before=%v after=%v", treeBefore, treeAfter)
					}
					if f.sb.ReadFile(filepath.Join(f.target, want, "SKILL.md")) != contentBefore {
						t.Fatal("second sync changed selected content")
					}
				}
				if f.sb.ReadFile(f.config) != before {
					t.Fatal("switch rewrote config")
				}
			})
		}
	}
}

func TestProfiles_PreservationAndDiscard(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, jsonOutput := range []bool{false, true} {
			t.Run(fmt.Sprintf("%v/json=%v", project, jsonOutput), func(t *testing.T) {
				f := newProfileFixture(t, project, "copy")
				f.run("profile", "use", "a").AssertSuccess(t)
				path := filepath.Join(f.target, "alpha", "SKILL.md")
				if err := os.WriteFile(path, []byte("authored edit"), 0644); err != nil {
					t.Fatal(err)
				}
				f.run("profile", "use", "b").AssertFailure(t)
				f.run("sync", "--json").AssertSuccess(t)
				if f.sb.ReadFile(path) != "authored edit" {
					t.Fatal("edit lost")
				}
				args := []string{"profile", "use", "b", "--discard-edits"}
				if jsonOutput {
					args = append(args, "--json")
				}
				out := f.run(args...)
				out.AssertSuccess(t)
				out.AssertOutputContains(t, "restore")
				out.AssertOutputContains(t, "retention")
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatal("discard did not remove listed copy")
				}
				backupRoot := backup.BackupDir()
				if project {
					backupRoot = backup.ProjectBackupDir(f.root)
				}
				entries, err := filepath.Glob(filepath.Join(backupRoot, "*", "custom", "alpha", "SKILL.md"))
				if err != nil || len(entries) != 1 {
					t.Fatal("discard snapshot missing", entries, err)
				}
				if f.sb.ReadFile(entries[0]) != "authored edit" {
					t.Fatal("snapshot did not contain planned bytes")
				}
				stamp := filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(entries[0]))))
				f.run("restore", "custom", "--from", stamp, "--force").AssertSuccess(t)
				if f.sb.ReadFile(path) != "authored edit" {
					t.Fatal("restore did not recover discarded bytes")
				}
			})
		}
	}
}

func TestProfiles_MovedDiscardRecovery(t *testing.T) {
	for _, jsonOutput := range []bool{false, true} {
		t.Run(fmt.Sprintf("json=%v", jsonOutput), func(t *testing.T) {
			f := newProfileFixture(t, true, "copy")
			f.sb.WriteFile(f.config, strings.Replace(f.sb.ReadFile(f.config), "  - name: custom\n    skills:\n      path: "+f.target, "  - name: goose\n    skills:", 1))
			f.target = filepath.Join(f.root, ".agents", "skills")
			f.run("profile", "use", "a").AssertSuccess(t)
			dir := filepath.Join(f.root, ".goose", "skills")
			file := filepath.Join(dir, "alpha", "SKILL.md")
			f.sb.WriteFile(file, f.sb.ReadFile(filepath.Join(f.source, "alpha", "SKILL.md")))
			sum, err := ssync.CopyOutputFingerprint(filepath.Dir(file))
			if err != nil {
				t.Fatal(err)
			}
			if err := ssync.WriteManifest(dir, &ssync.Manifest{Managed: map[string]string{"alpha": sum}, Outputs: map[string]string{"alpha": sum}}); err != nil {
				t.Fatal(err)
			}
			cleanPreview := f.run("diff", "--json")
			cleanPreview.AssertSuccess(t)
			if !json.Valid([]byte(cleanPreview.Stdout)) {
				t.Fatalf("clean moved preview emitted diagnostics into JSON: %s", cleanPreview.Stdout)
			}
			f.sb.WriteFile(file, "authored moved edit")
			manifestBefore := f.sb.ReadFile(filepath.Join(dir, ssync.ManifestFile))
			preview := f.run("diff", "--json")
			preview.AssertSuccess(t)
			var diff struct {
				Targets []struct {
					Items []struct {
						Action, Name, Reason string
						IsSync               bool `json:"is_sync"`
					}
				}
			}
			if err := json.Unmarshal([]byte(preview.Stdout), &diff); err != nil || len(diff.Targets) != 1 {
				t.Fatalf("moved preview JSON: %s %v", preview.Stdout, err)
			}
			found := false
			for _, item := range diff.Targets[0].Items {
				if item.Name == "alpha" && item.Action == "kept" && !item.IsSync && strings.Contains(item.Reason, dir) {
					found = true
				}
			}
			if !found || f.sb.ReadFile(file) != "authored moved edit" || f.sb.ReadFile(filepath.Join(dir, ssync.ManifestFile)) != manifestBefore {
				t.Fatal("moved preview omitted kept reason or wrote target/manifest")
			}

			args := []string{"sync", "--discard-edits"}
			if jsonOutput {
				args = append(args, "--json")
			}
			out := f.run(args...)
			out.AssertSuccess(t)
			key := backup.MovedEntryName("goose", ".goose/skills")
			paths, err := filepath.Glob(filepath.Join(backup.ProjectBackupDir(f.root), "*", key, "alpha", "SKILL.md"))
			if err != nil || len(paths) != 1 || f.sb.ReadFile(paths[0]) != "authored moved edit" {
				t.Fatalf("actual-folder snapshot missing or wrong: %v, %v", paths, err)
			}
			if _, err := os.Lstat(filepath.Dir(file)); !os.IsNotExist(err) {
				t.Fatalf("discard kept listed moved copy: %v", err)
			}
			saved := filepath.Dir(filepath.Dir(paths[0]))
			stamp := filepath.Base(filepath.Dir(saved))
			command := ""
			if jsonOutput {
				var body struct {
					Snapshot []struct{ Target, Path, Restore string }
				}
				if err := json.Unmarshal([]byte(out.Stdout), &body); err != nil || len(body.Snapshot) != 1 || body.Snapshot[0].Path != saved || body.Snapshot[0].Target != "goose" {
					t.Fatalf("snapshot JSON: %s, %v", out.Stdout, err)
				}
				command = body.Snapshot[0].Restore
			} else {
				index := strings.Index(out.Stdout, "skillshare restore ")
				if index < 0 || len(strings.Fields(out.Stdout[index:])) < 7 {
					t.Fatalf("missing recovery command: %s", out.Stdout)
				}
				command = strings.Join(strings.Fields(out.Stdout[index:])[:7], " ")
			}
			if command != "skillshare restore "+key+" --from "+stamp+" -p --force" {
				t.Fatalf("wrong emitted recovery: %q", command)
			}
			current, err := ssync.CopyOutputFingerprint(f.target)
			if err != nil {
				t.Fatal(err)
			}
			manifest := f.sb.ReadFile(filepath.Join(f.target, ssync.ManifestFile))
			configBefore := f.sb.ReadFile(f.config)
			f.sb.RunCLIInDir(f.root, strings.Fields(command)[1:]...).AssertSuccess(t)
			after, err := ssync.CopyOutputFingerprint(f.target)
			if err != nil || after != current || f.sb.ReadFile(filepath.Join(f.target, ssync.ManifestFile)) != manifest || f.sb.ReadFile(f.config) != configBefore || f.sb.ReadFile(file) != "authored moved edit" {
				t.Fatalf("recovery lost edits or changed current folder/config: %v", err)
			}
			if f.sb.ReadFile(filepath.Join(f.source, "alpha", "SKILL.md")) != "---\nname: alpha\n---\n# alpha\n" {
				t.Fatal("recovery changed source")
			}
		})
	}
}

func TestProfiles_DesiredObserved(t *testing.T) {
	f := newProfileFixture(t, false, "copy")
	f.run("profile", "use", "all").AssertSuccess(t)
	f.run("profile", "use", "a", "--no-sync").AssertSuccess(t)
	status := f.run("status", "--json")
	status.AssertSuccess(t)
	var s struct {
		Profile struct {
			Selected int
			Pending  bool
		}
		Targets []struct {
			SyncedCount int `json:"synced_count"`
		}
	}
	if err := json.Unmarshal([]byte(status.Stdout), &s); err != nil || s.Profile.Selected != 1 || !s.Profile.Pending || s.Targets[0].SyncedCount != 2 {
		t.Fatalf("desired count replaced observed: %s %v", status.Stdout, err)
	}
	f.run("sync", "--json").AssertSuccess(t)
	f.run("profile", "list", "--json").AssertSuccess(t)
}

func TestProfiles_UnavailableAndRepair(t *testing.T) {
	for _, state := range []string{"missing", "unusable", "unreadable"} {
		t.Run(state, func(t *testing.T) {
			f := newProfileFixture(t, false, "copy")
			f.run("profile", "use", "a").AssertSuccess(t)
			text := f.sb.ReadFile(f.config)
			switch state {
			case "missing":
				text = strings.Replace(text, "  a:", "  renamed:", 1)
			case "unusable":
				text = strings.Replace(text, "  a:\n", "  a:\n    future: value\n", 1)
			case "unreadable":
				// A directory at the state path fails even under privileged CI.
				statePath, _, err := config.ProfilePaths(f.config)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(statePath); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(statePath, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(f.config, []byte(text), 0644); err != nil {
				t.Fatal(err)
			}
			f.run("sync", "--json").AssertFailure(t)
			f.run("diff", "--json").AssertFailure(t)
			f.run("status").AssertSuccess(t)
			f.run("status", "--json").AssertSuccess(t)
			f.run("profile", "list", "--json").AssertSuccess(t)
			if state != "unreadable" {
				f.run("profile", "use", "b").AssertSuccess(t)
			}
		})
	}
}

func TestProfiles_ShowUsesDefinitionRules(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprint(project), func(t *testing.T) {
			f := newProfileFixture(t, project, "copy")
			text := f.sb.ReadFile(f.config) + "  inert:\n    skills:\n      include: []\n      exclude: [alpha]\n  blank:\n    skills:\n      include: [' ', '']\n"
			if err := os.WriteFile(f.config, []byte(text), 0644); err != nil {
				t.Fatal(err)
			}
			for _, tc := range []struct {
				name     string
				selected int
				inert    bool
			}{{"all", 2, false}, {"empty", 0, false}, {"blank", 0, false}, {"inert", 0, true}, {"a", 1, false}} {
				result := f.run("profile", "show", tc.name, "--json")
				result.AssertSuccess(t)
				var out struct {
					Profiles []struct {
						Selected []string
						Skills   *struct{ Include, Exclude []string }
						Inert    []string `json:"inert_excludes"`
						Targets  map[string]int
					}
				}
				if err := json.Unmarshal([]byte(result.Stdout), &out); err != nil || len(out.Profiles) != 1 {
					t.Fatal(result.Stdout, err)
				}
				row := out.Profiles[0]
				if len(row.Selected) != tc.selected || row.Targets["custom"] != tc.selected || (len(row.Inert) > 0) != tc.inert {
					t.Fatalf("show disagrees: %s", result.Stdout)
				}
				if tc.name == "all" && row.Skills != nil {
					t.Fatal("pass-through definition lost its distinction from empty")
				}
			}
		})
	}
}

func TestProfiles_Composition(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprint(project), func(t *testing.T) {
			f := newProfileFixture(t, project, "copy")
			// Federation's hub is an ordinary repository. Use a local repository
			// so selection is tested independently of any network or account.
			repo := filepath.Join(f.sb.Root, "hub-skill")
			f.sb.WriteFile(filepath.Join(repo, "SKILL.md"), "---\nname: hub-skill\n---\n# Version 1\n")
			initGitRepo(t, repo)
			f.run("install", "file://"+repo).AssertSuccess(t)
			metadata := filepath.Join(f.source, install.MetadataFileName)
			protected := []string{f.config, metadata}
			if project {
				protected = append(protected, filepath.Join(f.root, ".skillshare", "skills.lock.json"))
			}
			before := make(map[string]string)
			for _, path := range protected {
				before[path] = f.sb.ReadFile(path)
			}
			f.run("profile", "use", "all").AssertSuccess(t)
			f.run("profile", "use", "a").AssertSuccess(t)
			if _, err := os.Lstat(filepath.Join(f.target, "hub-skill")); !os.IsNotExist(err) {
				t.Fatal("hub skill was not deselected", err)
			}
			if !strings.Contains(f.sb.ReadFile(filepath.Join(f.source, "hub-skill", "SKILL.md")), "Version 1") {
				t.Fatal("switch removed installed source")
			}
			for path, content := range before {
				if f.sb.ReadFile(path) != content {
					t.Fatalf("switch changed %s", path)
				}
			}
			f.sb.WriteFile(filepath.Join(repo, "SKILL.md"), "---\nname: hub-skill\n---\n# Version 2\n")
			testutil.RunGit(t, repo, "add", "SKILL.md")
			testutil.RunGit(t, repo, "commit", "-m", "Update fixture")
			f.run("install", "hub-skill", "--update").AssertSuccess(t)
			if !strings.Contains(f.sb.ReadFile(filepath.Join(f.source, "hub-skill", "SKILL.md")), "Version 2") {
				t.Fatal("deselected source could not update")
			}
			f.run("profile", "use", "all").AssertSuccess(t)
			if !strings.Contains(f.sb.ReadFile(filepath.Join(f.target, "hub-skill", "SKILL.md")), "Version 2") {
				t.Fatal("updated source did not redeploy")
			}
		})
	}
}

func TestProfiles_SharedFolderVariants(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, mode := range []string{"copy", "merge"} {
			t.Run(fmt.Sprintf("%v/%s", project, mode), func(t *testing.T) {
				f := newProfileFixture(t, project, mode)
				text := fmt.Sprintf("source: %s\nmode: %s\ntargets:\n  codex:\n    skills:\n      path: %s\n  universal:\n    skills:\n      path: %s\n", f.source, mode, f.target, f.target)
				if project {
					text = fmt.Sprintf("targets:\n  - name: codex\n    skills:\n      path: %s\n      mode: %s\n  - name: universal\n    skills:\n      path: %s\n      mode: %s\n", f.target, mode, f.target, mode)
				}
				text += "profiles:\n  variants:\n    skills:\n      include: ['_repo__*']\n  plain:\n    skills:\n      include: [alpha]\n"
				f.sb.WriteFile(f.config, text)
				for _, agent := range []string{"claude", "codex"} {
					f.sb.WriteFile(filepath.Join(f.source, "_repo", "internal", "skills", agent, "fix", "SKILL.md"), "# Instructions for "+agent)
				}
				initGitRepo(t, filepath.Join(f.source, "_repo"))
				for _, name := range []string{"variants", "plain", "variants"} {
					f.run("profile", "use", name).AssertSuccess(t)
					f.run("sync", "--json").AssertSuccess(t)
					codex := filepath.Join(f.target, "_repo__internal__skills__codex__fix", "SKILL.md")
					if name == "plain" {
						if _, err := os.Stat(codex); !os.IsNotExist(err) {
							t.Fatal("departing variant retained", err)
						}
					} else if f.sb.ReadFile(codex) != "# Instructions for codex" {
						t.Fatal("shared peer lost routed variant")
					}
					f.run("diff", "--json").AssertSuccess(t)
				}
			})
		}
	}
}

func TestProfiles_DesiredObservedUnmanaged(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprint(project), func(t *testing.T) {
			f := newProfileFixture(t, project, "copy")
			f.run("profile", "use", "a", "--no-sync").AssertSuccess(t)
			f.sb.WriteFile(filepath.Join(f.target, "alpha", "SKILL.md"), "unmanaged bytes")
			f.run("sync", "--json").AssertSuccess(t)
			result := f.run("status", "--json")
			result.AssertSuccess(t)
			var out struct {
				Profile struct {
					Selected int
					Pending  bool
				}
				Targets []struct {
					SyncedCount int `json:"synced_count"`
				}
			}
			if err := json.Unmarshal([]byte(result.Stdout), &out); err != nil || out.Profile.Selected != 1 || !out.Profile.Pending || len(out.Targets) != 1 || out.Targets[0].SyncedCount != 0 {
				t.Fatalf("unmanaged target was reported as deployed: %s %v", result.Stdout, err)
			}
			if f.sb.ReadFile(filepath.Join(f.target, "alpha", "SKILL.md")) != "unmanaged bytes" {
				t.Fatal("sync changed unmanaged bytes")
			}
		})
	}
}

func TestProfiles_OrdinarySyncBacksUpBeforeRefresh(t *testing.T) {
	f := newProfileFixture(t, false, "copy")
	f.run("profile", "use", "a").AssertSuccess(t)
	old := f.sb.ReadFile(filepath.Join(f.target, "alpha", "SKILL.md"))
	f.sb.WriteFile(filepath.Join(f.source, "alpha", "SKILL.md"), "---\nname: alpha\n---\n# changed source\n")
	f.run("sync").AssertSuccess(t)
	paths, err := filepath.Glob(filepath.Join(backup.BackupDir(), "*", "custom", "alpha", "SKILL.md"))
	if err != nil || len(paths) != 1 {
		t.Fatal("pre-sync backup missing", paths, err)
	}
	if f.sb.ReadFile(paths[0]) != old {
		t.Fatal("automatic backup captured refreshed content")
	}
	if f.sb.ReadFile(filepath.Join(f.target, "alpha", "SKILL.md")) == old {
		t.Fatal("fixture did not refresh")
	}
}

func TestProfiles_SyncJSONBlockedEntries(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, mode := range []string{"copy", "merge"} {
			t.Run(fmt.Sprintf("%v/%s", project, mode), func(t *testing.T) {
				f := newProfileFixture(t, project, mode)
				f.run("profile", "use", "all", "--no-sync").AssertSuccess(t)
				path := filepath.Join(f.target, "alpha", "SKILL.md")
				f.sb.WriteFile(path, "unmanaged bytes")
				step := "refresh"
				if mode == "merge" {
					step = "convert"
				}
				want := ssync.EntryChange{Target: "custom", Entry: "alpha", Step: step, Class: ssync.EntryUnmanaged, Reason: "entry is not a recorded copy"}
				for _, dryRun := range []bool{true, false} {
					args := []string{"sync", "--json"}
					if dryRun {
						args = append(args, "--dry-run")
					}
					result := f.run(args...)
					result.AssertSuccess(t)
					var out struct {
						Details []struct {
							Blocked      []ssync.EntryChange `json:"blocked"`
							KeptModified []ssync.EntryChange `json:"kept_modified"`
						}
					}
					if err := json.Unmarshal([]byte(result.Stdout), &out); err != nil || len(out.Details) != 1 || len(out.Details[0].Blocked) != 1 || out.Details[0].Blocked[0] != want || len(out.Details[0].KeptModified) != 0 {
						t.Fatalf("blocked record missing or reclassified: %s, %v", result.Stdout, err)
					}
					if f.sb.ReadFile(path) != "unmanaged bytes" {
						t.Fatal("sync changed unmanaged bytes")
					}
					if dryRun {
						if _, err := os.Stat(filepath.Join(f.target, "beta")); !os.IsNotExist(err) {
							t.Fatalf("preview wrote ordinary skill: %v", err)
						}
					} else if f.sb.ReadFile(filepath.Join(f.target, "beta", "SKILL.md")) != f.sb.ReadFile(filepath.Join(f.source, "beta", "SKILL.md")) {
						t.Fatal("unblocked skill did not sync")
					}
				}
				f.run("profile", "use", "b", "--no-sync").AssertSuccess(t)
				result := f.run("sync", "--json")
				result.AssertSuccess(t)
				if strings.Contains(result.Stdout, `"blocked"`) {
					t.Fatalf("empty blocked field was not omitted: %s", result.Stdout)
				}
			})
		}
	}
}

func TestProfiles_BinaryCopySwitchPreservesEdits(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, mode := range []string{"copy", "merge"} {
			t.Run(fmt.Sprintf("%v/%s", project, mode), func(t *testing.T) {
				f := newProfileFixture(t, project, "copy")
				asset := filepath.Join(f.source, "alpha", "a")
				if err := os.WriteFile(asset, []byte("x\x00b\x00y"), 0600); err != nil {
					t.Fatal(err)
				}
				f.run("profile", "use", "a").AssertSuccess(t)
				configBytes, err := os.ReadFile(f.config)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(f.config, []byte(strings.ReplaceAll(string(configBytes), "mode: copy", "mode: "+mode)), 0600); err != nil {
					t.Fatal(err)
				}
				entry := filepath.Join(f.target, "alpha")
				for name, data := range map[string]string{"a": "x", "b": "y"} {
					if err := os.WriteFile(filepath.Join(entry, name), []byte(data), 0600); err != nil {
						t.Fatal(err)
					}
				}
				statePath, _, _ := config.ProfilePaths(f.config)
				before, err := os.ReadFile(statePath)
				if err != nil {
					t.Fatal(err)
				}
				out := f.run("profile", "use", "b", "--json")
				out.AssertFailure(t)
				for name, want := range map[string]string{"a": "x", "b": "y"} {
					data, err := os.ReadFile(filepath.Join(entry, name))
					if err != nil || string(data) != want {
						t.Errorf("switch lost %s: %q %v", name, data, err)
					}
				}
				after, err := os.ReadFile(statePath)
				if err != nil || string(after) != string(before) {
					t.Error("refused switch changed local state", err)
				}
				if _, err := os.Lstat(filepath.Join(f.target, "beta")); !os.IsNotExist(err) {
					t.Error("refused switch deployed beta", err)
				}
			})
		}
	}
}

func TestProfiles_DiffKeptCopyAgreement(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, mode := range []string{"copy", "merge"} {
			for _, scenario := range []string{"retained", "departing", "pending_none", "alias_retained", "alias_pending_none"} {
				t.Run(fmt.Sprintf("%v/%s/%s", project, mode, scenario), func(t *testing.T) {
					f := newProfileFixture(t, project, "copy")
					alias := strings.HasPrefix(scenario, "alias_")
					var rootLink string
					if alias {
						outside := filepath.Join(f.root, "dotfiles-target")
						if err := os.MkdirAll(outside, 0755); err != nil {
							t.Fatal(err)
						}
						if runtime.GOOS == "windows" {
							if err := utils.CreateJunction(f.target, outside); err != nil {
								t.Fatal(err)
							}
						} else {
							f.sb.CreateSymlink(outside, f.target)
						}
						var err error
						rootLink, err = os.Readlink(f.target)
						if err != nil {
							t.Fatal(err)
						}
					}
					f.run("profile", "use", "a").AssertSuccess(t)
					configBytes, err := os.ReadFile(f.config)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(f.config, []byte(strings.ReplaceAll(string(configBytes), "mode: copy", "mode: "+mode)), 0600); err != nil {
						t.Fatal(err)
					}
					if scenario == "pending_none" || scenario == "alias_pending_none" {
						f.run("profile", "use", "--none", "--no-sync").AssertSuccess(t)
					}
					path := filepath.Join(f.target, "alpha", "SKILL.md")
					if err := os.WriteFile(path, []byte("authored alpha"), 0600); err != nil {
						t.Fatal(err)
					}
					switch scenario {
					case "departing":
						f.run("profile", "edit", "a", "--remove-include", "alpha", "--add-include", "beta").AssertSuccess(t)
					}
					manifestPath := filepath.Join(f.target, ssync.ManifestFile)
					manifestBefore, err := os.ReadFile(manifestPath)
					if err != nil {
						t.Fatal(err)
					}
					statePath, _, err := config.ProfilePaths(f.config)
					if err != nil {
						t.Fatal(err)
					}
					stateBefore := f.sb.ReadFile(statePath)
					configBefore := f.sb.ReadFile(f.config)
					sourceBefore := f.sb.ReadFile(filepath.Join(f.source, "alpha", "SKILL.md"))
					for _, jsonOutput := range []bool{false, true} {
						args := []string{"diff"}
						if jsonOutput {
							args = append(args, "--json")
						}
						preview := f.run(args...)
						preview.AssertSuccess(t)
						if !jsonOutput {
							if !strings.Contains(strings.ToLower(preview.Stdout), "kept") || strings.Contains(preview.Stdout, "will be pruned") {
								t.Errorf("text preview hides preserved edit: %s", preview.Stdout)
							}
							continue
						}
						var payload struct {
							Targets []struct {
								Mode  string
								Error string
								Items []struct {
									Name   string
									Action string
									Reason string
									IsSync bool `json:"is_sync"`
								}
							}
						}
						if err := json.Unmarshal([]byte(preview.Stdout), &payload); err != nil {
							t.Fatal(err)
						}
						found := false
						for _, target := range payload.Targets {
							if alias && (target.Mode != mode || target.Error != "") {
								t.Errorf("configured alias mode/error mismatch: %+v", target)
							}
							for _, item := range target.Items {
								if item.Name == "alpha" {
									found = true
									if item.Action != "kept" || item.IsSync || item.Reason == "" {
										t.Errorf("JSON predicts a write for preserved alpha: %+v", item)
									}
								}
							}
						}
						if !found {
							t.Error("JSON preview omitted kept alpha")
						}
					}
					manifestAfter, err := os.ReadFile(manifestPath)
					if err != nil || string(manifestAfter) != string(manifestBefore) {
						t.Fatal("read-only preview changed manifest", err)
					}
					if alias {
						if after, err := os.Readlink(f.target); err != nil || after != rootLink || !utils.IsSymlinkOrJunction(f.target) {
							t.Errorf("preview changed root alias: %q, %v", after, err)
						}
						if f.sb.ReadFile(path) != "authored alpha" || f.sb.ReadFile(statePath) != stateBefore || f.sb.ReadFile(f.config) != configBefore || f.sb.ReadFile(filepath.Join(f.source, "alpha", "SKILL.md")) != sourceBefore {
							t.Error("alias preview wrote target, state, config or source")
						}
					}
					out := f.run("sync", "--json")
					out.AssertSuccess(t)
					if !strings.Contains(out.Stdout, "kept_modified") {
						t.Errorf("sync omitted preservation outcome: %s", out.Stdout)
					}
					if alias {
						if after, err := os.Readlink(f.target); err != nil || after != rootLink || !utils.IsSymlinkOrJunction(f.target) {
							t.Errorf("sync changed root alias: %q, %v", after, err)
						}
					}
					if data, err := os.ReadFile(path); err != nil || string(data) != "authored alpha" {
						t.Fatalf("sync disagrees with preservation: %q %v", data, err)
					}
				})
			}
		}
	}
}

func TestProfiles_FutureDefinitionRawRepair(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprint(project), func(t *testing.T) {
			f := newProfileFixture(t, project, "copy")
			data, err := os.ReadFile(f.config)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(f.config, append(data, []byte("  future: # shared comment\n    new_rule: [alpha]\n")...), 0600); err != nil {
				t.Fatal(err)
			}
			f.run("profile", "rename", "future", "renamed").AssertSuccess(t)
			show := f.run("profile", "show", "renamed", "--json")
			show.AssertSuccess(t)
			f.run("profile", "use", "renamed").AssertFailure(t)
			f.run("profile", "edit", "renamed", "--add-include", "beta").AssertFailure(t)
			f.run("profile", "delete", "renamed").AssertSuccess(t)
			after, err := os.ReadFile(f.config)
			if err != nil || strings.Contains(string(after), "new_rule:") || !strings.Contains(string(after), "include: [alpha]") {
				t.Fatal("raw delete changed unrelated definitions", err)
			}
		})
	}
}
