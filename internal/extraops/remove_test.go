package extraops

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/install"
	syncpkg "skillshare/internal/sync"
	"skillshare/internal/utils"
)

func removalFixture(t *testing.T) RemoveRequest {
	t.Helper()
	base := t.TempDir()
	t.Setenv("HOME", base)
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "state"))
	t.Setenv("SKILLSHARE_CONFIG", filepath.Join(base, "config.yaml"))
	root := filepath.Join(base, "extras")
	source := filepath.Join(root, "rules")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "rule.md"), []byte("rules"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Source: filepath.Join(base, "skills"), Extras: []config.ExtraConfig{{Name: "rules", Remote: "example-org/ai-hub", Targets: []config.ExtraTargetConfig{}}, {Name: "other"}}}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	hash, err := utils.FileHashFormatted(filepath.Join(source, "rule.md"))
	if err != nil {
		t.Fatal(err)
	}
	store := install.NewMetadataStore()
	store.Set("rules", &install.MetadataEntry{Kind: "extra", Source: "example-org/ai-hub", Commit: strings.Repeat("a", 40), FileHashes: map[string]string{"rule.md": hash}})
	store.Set("other", &install.MetadataEntry{Commit: strings.Repeat("b", 40)})
	store.AuditAccepted = map[string][]string{"rules": {"synthetic"}, "other": {"keep"}}
	if err := store.Save(root); err != nil {
		t.Fatal(err)
	}
	return RemoveRequest{Extras: cfg.Extras, Name: "rules", SourceDir: source, ExtrasRoot: root, TargetPath: func(p string) string { return p }, Load: func() ([]config.ExtraConfig, error) {
		fresh, err := config.LoadReadOnly()
		if err != nil {
			return nil, err
		}
		return fresh.Extras, nil
	}, Save: func(remaining []config.ExtraConfig) error {
		fresh, err := config.LoadReadOnly()
		if err != nil {
			return err
		}
		fresh.Extras = remaining
		return fresh.Save()
	}}
}

func TestRemove_RemoteUneditedAndEdited(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, outcome := range []string{"unedited", "edited", "missing"} {
			t.Run(fmt.Sprintf("project=%t/%s", project, outcome), func(t *testing.T) {
				req := removalFixture(t)
				base := filepath.Dir(req.ExtrasRoot)
				if project {
					req.ProjectRoot = base
					pc := &config.ProjectConfig{Extras: req.Extras}
					if err := pc.Save(base); err != nil {
						t.Fatal(err)
					}
					req.Load = func() ([]config.ExtraConfig, error) {
						fresh, err := config.LoadProjectReadOnly(base)
						if err != nil {
							return nil, err
						}
						return fresh.Extras, nil
					}
					req.Save = func(entries []config.ExtraConfig) error {
						fresh, err := config.LoadProjectReadOnly(base)
						if err != nil {
							return err
						}
						fresh.Extras = entries
						return fresh.Save(base)
					}
					lock := &install.Lock{Version: 1, Extras: map[string]install.LockEntry{"rules": {Source: "example-org/ai-hub", Commit: strings.Repeat("a", 40)}, "other": {Source: "example-org/other", Commit: strings.Repeat("b", 40)}}}
					if err := lock.Save(filepath.Join(base, ".skillshare")); err != nil {
						t.Fatal(err)
					}
				}
				if outcome == "edited" {
					if err := os.WriteFile(filepath.Join(req.SourceDir, "rule.md"), []byte("edit"), 0644); err != nil {
						t.Fatal(err)
					}
				} else if outcome == "missing" {
					if err := os.RemoveAll(req.SourceDir); err != nil {
						t.Fatal(err)
					}
				}
				result, err := Remove(req)
				if err != nil {
					t.Fatal(err)
				}
				if result.Remote != "example-org/ai-hub" || result.Commit != strings.Repeat("a", 40) || result.DeletedDir != (outcome == "unedited") || result.KeptEditedDir != (outcome == "edited") || result.MissingDir != (outcome == "missing") {
					t.Fatalf("result: %+v", result)
				}
				_, err = os.Stat(req.SourceDir)
				if outcome == "edited" && err != nil || outcome != "edited" && !os.IsNotExist(err) {
					t.Fatalf("directory: %v", err)
				}
				if outcome == "edited" {
					if body, err := os.ReadFile(filepath.Join(req.SourceDir, "rule.md")); err != nil || string(body) != "edit" {
						t.Fatalf("edited bytes lost: %q %v", body, err)
					}
				}
				entries, err := req.Load()
				if err != nil || len(entries) != 1 || entries[0].Name != "other" {
					t.Fatalf("config: %+v %v", entries, err)
				}
				store, _, err := install.ReadMetadata(req.ExtrasRoot)
				if err != nil || store.GetExact("rules") != nil || len(store.AuditAccepted["rules"]) != 0 || store.GetExact("other") == nil || len(store.AuditAccepted["other"]) != 1 {
					t.Fatalf("metadata: %+v %v", store, err)
				}
				if project {
					lock, err := install.LoadLock(filepath.Join(base, ".skillshare"))
					if err != nil || lock.ExtraCommitFor("rules", "example-org/ai-hub") != "" || lock.ExtraCommitFor("other", "example-org/other") != strings.Repeat("b", 40) {
						t.Fatalf("pin isolation: %+v %v", lock, err)
					}
				}
			})
		}
	}
}

func TestRemove_CleansOnlyOwnedLinksKeepsOutputs(t *testing.T) {
	t.Run("copied output without symlink privilege", func(t *testing.T) {
		req := removalFixture(t)
		target := t.TempDir()
		entry := req.Extras[0]
		entry.Targets = []config.ExtraTargetConfig{{Path: target, Mode: "copy"}}
		cfg, err := config.LoadReadOnly()
		if err != nil {
			t.Fatal(err)
		}
		cfg.Extras[0] = entry
		if err := cfg.Save(); err != nil {
			t.Fatal(err)
		}
		req.Extras = cfg.Extras
		copied, err := syncpkg.SyncExtraTarget(entry, entry.Targets[0], req.SourceDir, target, "copy", nil, syncpkg.ExtraSyncOptions{})
		if err != nil || len(copied.Errors) != 0 {
			t.Fatalf("supported copy fixture: %+v %v", copied, err)
		}
		result, err := Remove(req)
		if err != nil || !result.DeletedDir || !reflect.DeepEqual(result.KeptOutputs, []string{target}) || len(result.Warnings) == 0 {
			t.Fatalf("copy result: %+v %v", result, err)
		}
		if body, err := os.ReadFile(filepath.Join(target, "rule.md")); err != nil || string(body) != "rules" {
			t.Fatalf("copied bytes lost: %q %v", body, err)
		}
	})
	t.Run("whole links and copied outputs", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("symlink privileges vary")
		}
		req := removalFixture(t)
		merge := filepath.Join(filepath.Dir(req.ExtrasRoot), "merge")
		if err := os.Mkdir(merge, 0755); err != nil {
			t.Fatal(err)
		}
		owned := filepath.Join(merge, "rule.md")
		if err := os.Symlink(filepath.Join(req.SourceDir, "rule.md"), owned); err != nil {
			t.Fatal(err)
		}
		sentinel := filepath.Join(filepath.Dir(merge), "sentinel")
		if err := os.WriteFile(sentinel, []byte("keep"), 0644); err != nil {
			t.Fatal(err)
		}
		other := filepath.Join(merge, "other.md")
		if err := os.Symlink(sentinel, other); err != nil {
			t.Fatal(err)
		}
		linked := filepath.Join(filepath.Dir(merge), "linked")
		if err := os.Symlink(req.SourceDir, linked); err != nil {
			t.Fatal(err)
		}
		copyPath := filepath.Join(filepath.Dir(merge), "copied")
		if err := os.Mkdir(copyPath, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(copyPath, "rule.md"), []byte("keep copy"), 0644); err != nil {
			t.Fatal(err)
		}
		cfg, err := config.LoadReadOnly()
		if err != nil {
			t.Fatal(err)
		}
		cfg.Extras[0].Targets = []config.ExtraTargetConfig{{Path: merge}, {Path: linked, Mode: "symlink"}, {Path: copyPath, Mode: "copy"}}
		if err := cfg.Save(); err != nil {
			t.Fatal(err)
		}
		req.Extras = cfg.Extras
		result, err := Remove(req)
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range []string{owned, linked} {
			if _, err := os.Lstat(path); !os.IsNotExist(err) {
				t.Fatalf("owned link kept: %s %v", path, err)
			}
		}
		if _, err := os.Lstat(other); err != nil {
			t.Fatal("foreign link removed")
		}
		if data, err := os.ReadFile(filepath.Join(copyPath, "rule.md")); err != nil || string(data) != "keep copy" {
			t.Fatal("copy removed")
		}
		if !reflect.DeepEqual(result.KeptOutputs, []string{copyPath}) || len(result.Warnings) == 0 {
			t.Fatalf("kept outputs missing: %+v", result)
		}
	})
	t.Run("hidden ownership from supported sync", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("symlink privileges vary")
		}
		for _, project := range []bool{false, true} {
			for _, remote := range []bool{false, true} {
				t.Run(fmt.Sprintf("project=%t/remote=%t", project, remote), func(t *testing.T) {
					req := removalFixture(t)
					base := filepath.Dir(req.ExtrasRoot)
					merge, copied := filepath.Join(base, "merge"), filepath.Join(base, "copy")
					entry := req.Extras[0]
					if !remote {
						entry.Remote = ""
					}
					entry.Targets = []config.ExtraTargetConfig{{Path: merge, Mode: "merge"}, {Path: copied, Mode: "copy"}}
					paths := []string{".rules.md", ".nested/.inside.md", ".nested/visible.md"}
					hashes := map[string]string{}
					for _, name := range append([]string{"rule.md"}, paths...) {
						file := filepath.Join(req.SourceDir, name)
						if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(file, []byte("owned fixture"), 0644); err != nil {
							t.Fatal(err)
						}
						hash, err := utils.FileHashFormatted(file)
						if err != nil {
							t.Fatal(err)
						}
						hashes[filepath.ToSlash(name)] = hash
					}
					cfg, err := config.LoadReadOnly()
					if err != nil {
						t.Fatal(err)
					}
					cfg.Extras[0] = entry
					req.Extras = cfg.Extras
					if project {
						req.ProjectRoot = base
						pc := &config.ProjectConfig{Extras: cfg.Extras}
						if err := pc.Save(base); err != nil {
							t.Fatal(err)
						}
						req.Load = func() ([]config.ExtraConfig, error) {
							fresh, err := config.LoadProjectReadOnly(base)
							if err != nil {
								return nil, err
							}
							return fresh.Extras, nil
						}
						req.Save = func(entries []config.ExtraConfig) error {
							fresh, err := config.LoadProjectReadOnly(base)
							if err != nil {
								return err
							}
							fresh.Extras = entries
							return fresh.Save(base)
						}
						req.Extras[0].Targets[0].Path, req.Extras[0].Targets[1].Path = "merge", "copy"
						pc.Extras = req.Extras
						if err := pc.Save(base); err != nil {
							t.Fatal(err)
						}
						req.TargetPath = func(path string) string { return filepath.Join(base, path) }
					} else if err := cfg.Save(); err != nil {
						t.Fatal(err)
					}
					store := install.NewMetadataStore()
					store.Set("rules", &install.MetadataEntry{Kind: "extra", Source: entry.Remote, Commit: strings.Repeat("a", 40), FileHashes: hashes})
					if err := store.Save(req.ExtrasRoot); err != nil {
						t.Fatal(err)
					}
					for i, target := range entry.Targets {
						path := []string{merge, copied}[i]
						result, err := syncpkg.SyncExtraTarget(entry, target, req.SourceDir, path, target.Mode, nil, syncpkg.ExtraSyncOptions{ProjectRoot: req.ProjectRoot})
						if err != nil || len(result.Errors) > 0 {
							t.Fatalf("supported sync producer: %+v %v", result, err)
						}
					}
					for _, name := range paths {
						if info, err := os.Lstat(filepath.Join(merge, name)); err != nil || info.Mode()&os.ModeSymlink == 0 {
							t.Fatalf("sync did not create owned %s: %v", name, err)
						}
					}
					foreign := filepath.Join(merge, ".foreign.md")
					foreignLink := filepath.Join(merge, ".foreign-link.md")
					if err := os.WriteFile(foreign, []byte("keep foreign"), 0644); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(foreign, foreignLink); err != nil {
						t.Fatal(err)
					}
					result, err := Remove(req)
					if err != nil {
						t.Fatal(err)
					}
					for _, name := range paths {
						_, err := os.Lstat(filepath.Join(merge, name))
						if remote && !os.IsNotExist(err) || !remote && err != nil {
							t.Fatalf("hidden cleanup remote=%t/%s: %v", remote, name, err)
						}
						if body, err := os.ReadFile(filepath.Join(copied, name)); err != nil || string(body) != "owned fixture" {
							t.Fatalf("copied output lost: %q %v", body, err)
						}
					}
					if info, err := os.Lstat(foreignLink); err != nil || info.Mode()&os.ModeSymlink == 0 {
						t.Fatalf("foreign link replaced: %v", err)
					}
					if body, err := os.ReadFile(foreignLink); err != nil || string(body) != "keep foreign" {
						t.Fatalf("foreign bytes changed: %q %v", body, err)
					}
					_, err = os.Stat(req.SourceDir)
					if remote && !os.IsNotExist(err) || !remote && err != nil {
						t.Fatalf("source retention remote=%t: %v", remote, err)
					}
					if remote && (!result.DeletedDir || len(result.Warnings) == 0 || !reflect.DeepEqual(result.KeptOutputs, []string{copied})) {
						t.Fatalf("cleanup outcomes: %+v", result)
					}
				})
			}
		}
	})
}

func TestRemove_CleanupFailureRetainsRecords(t *testing.T) {
	req := removalFixture(t)
	old := restoreFileTargets
	restoreFileTargets = func(config.ExtraConfig, string, func(string) string) (int, error) {
		return 0, errors.New("synthetic restore failure")
	}
	defer func() { restoreFileTargets = old }()
	before, err := os.ReadFile(config.ReadOnlyConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Remove(req); err == nil || !strings.Contains(err.Error(), "target cleanup failed") {
		t.Fatalf("error: %v", err)
	}
	after, err := os.ReadFile(config.ReadOnlyConfigPath())
	if err != nil || string(after) != string(before) {
		t.Fatal("config changed")
	}
	if _, err := os.Stat(req.SourceDir); err != nil {
		t.Fatal("source removed")
	}
	store, _, err := install.ReadMetadata(req.ExtrasRoot)
	if err != nil || store.GetExact("rules") == nil {
		t.Fatal("metadata removed")
	}
}

func TestRemove_SaveFailureRestoresRecords(t *testing.T) {
	req := removalFixture(t)
	req.Save = func([]config.ExtraConfig) error { return errors.New("synthetic save failure") }
	before, err := os.ReadFile(filepath.Join(req.ExtrasRoot, install.MetadataFileName))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Remove(req); err == nil {
		t.Fatal("save failure ignored")
	}
	after, err := os.ReadFile(filepath.Join(req.ExtrasRoot, install.MetadataFileName))
	if err != nil || string(before) != string(after) {
		t.Fatal("metadata changed")
	}
	if _, err := os.Stat(req.SourceDir); err != nil {
		t.Fatal("source removed")
	}
}

func TestRemove_ReloadsUnderOwningLock(t *testing.T) {
	t.Run("baseline", func(t *testing.T) {
		req := removalFixture(t)
		old := lockExtrasRoot
		lockExtrasRoot = func(root string) (func(), error) {
			if root != req.ExtrasRoot {
				t.Fatal("wrong root")
			}
			// Deterministic save between the caller's resolution and acquisition.
			cfg, err := config.LoadReadOnly()
			if err != nil {
				t.Fatal(err)
			}
			cfg.Extras[0].Targets = []config.ExtraTargetConfig{{Path: filepath.Join(filepath.Dir(root), "new-copy"), Mode: "copy"}}
			cfg.Extras = append(cfg.Extras, config.ExtraConfig{Name: "concurrent"})
			if err := cfg.Save(); err != nil {
				t.Fatal(err)
			}
			return func() {}, nil
		}
		defer func() { lockExtrasRoot = old }()
		result, err := Remove(req)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.KeptOutputs) != 1 || len(result.Remaining) != 2 || result.Remaining[1].Name != "concurrent" {
			t.Fatalf("used stale entries/targets: %+v", result)
		}
	})
	t.Run("TestRemove_ChangedOriginAfterLockRefused", func(t *testing.T) {
		req := removalFixture(t)
		req.Load = func() ([]config.ExtraConfig, error) {
			return []config.ExtraConfig{{Name: "rules", Remote: "example-org/different"}}, nil
		}
		if _, err := Remove(req); err == nil || !strings.Contains(err.Error(), "changed") {
			t.Fatalf("changed origin: %v", err)
		}
		if _, err := os.Stat(req.SourceDir); err != nil {
			t.Fatal("source removed")
		}
	})
}

func TestRemove_LocalDoesNotLockOrDelete(t *testing.T) {
	req := removalFixture(t)
	req.Extras[0].Remote = ""
	req.Load = nil
	old := lockExtrasRoot
	lockExtrasRoot = func(string) (func(), error) { t.Fatal("local removal took root lock"); return nil, nil }
	defer func() { lockExtrasRoot = old }()
	result, err := Remove(req)
	if err != nil {
		t.Fatal(err)
	}
	if result.DeletedDir || result.KeptEditedDir {
		t.Fatalf("local behavior changed: %+v", result)
	}
	if _, err := os.Stat(req.SourceDir); err != nil {
		t.Fatal("local source deleted")
	}
}

func TestRemove_PartialDeleteFailureRestoresSourceAndProjectRecords(t *testing.T) {
	req := removalFixture(t)
	req.ProjectRoot = filepath.Dir(req.ExtrasRoot)
	lockDir := filepath.Join(req.ProjectRoot, ".skillshare")
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := config.ProjectConfigPath(req.ProjectRoot)
	project := &config.ProjectConfig{Extras: req.Extras}
	if err := project.Save(req.ProjectRoot); err != nil {
		t.Fatal(err)
	}
	req.Load = func() ([]config.ExtraConfig, error) {
		fresh, err := config.LoadProjectReadOnly(req.ProjectRoot)
		if err != nil {
			return nil, err
		}
		return fresh.Extras, nil
	}
	lock := &install.Lock{Version: 1, Extras: map[string]install.LockEntry{"rules": {Source: "example-org/ai-hub", Commit: strings.Repeat("a", 40)}}}
	if err := lock.Save(lockDir); err != nil {
		t.Fatal(err)
	}
	if err := install.UpdateGitIgnore(req.ProjectRoot, "extras/rules"); err != nil {
		t.Fatal(err)
	}
	paths := []string{configPath, filepath.Join(req.ExtrasRoot, install.MetadataFileName), filepath.Join(lockDir, install.LockFileName), filepath.Join(req.ProjectRoot, ".gitignore")}
	before := map[string]string{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		before[path] = string(data)
	}
	req.Save = func(remaining []config.ExtraConfig) error {
		fresh, err := config.LoadProjectReadOnly(req.ProjectRoot)
		if err != nil {
			return err
		}
		fresh.Extras = remaining
		return fresh.Save(req.ProjectRoot)
	}
	old := removeExtraDir
	removeExtraDir = func(dir string) error {
		if err := os.Remove(filepath.Join(dir, "rule.md")); err != nil {
			return err
		}
		return errors.New("synthetic partial deletion")
	}
	defer func() { removeExtraDir = old }()
	if _, err := Remove(req); err == nil || !strings.Contains(err.Error(), "restored") {
		t.Fatalf("partial deletion: %v", err)
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != before[path] {
			t.Fatalf("record not restored %s: %s %v", path, data, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(req.SourceDir, "rule.md"))
	if err != nil || string(data) != "rules" {
		t.Fatalf("source not restored: %s %v", data, err)
	}
}

func TestWithoutUnchangedExtra(t *testing.T) {
	expected := config.ExtraConfig{Name: "rules", Remote: "example-org/ai-hub"}
	entries := []config.ExtraConfig{{Name: "other", Targets: []config.ExtraTargetConfig{{Path: "new"}}}, expected}
	got, err := WithoutUnchangedExtra(entries, expected)
	if err != nil || len(got) != 1 || got[0].Targets[0].Path != "new" {
		t.Fatalf("fresh merge: %+v %v", got, err)
	}
	entries[1].Targets = []config.ExtraTargetConfig{{Path: "changed"}}
	if _, err := WithoutUnchangedExtra(entries, expected); err == nil {
		t.Fatal("changed own entry removed")
	}
}

func TestRemove_ChangedEntryAtSaveRetained(t *testing.T) {
	req := removalFixture(t)
	expected := req.Extras[0]
	req.Save = func([]config.ExtraConfig) error {
		fresh, err := config.LoadReadOnly()
		if err != nil {
			return err
		}
		fresh.Extras[0].Branch = "changed-after-load"
		if err := fresh.Save(); err != nil {
			return err
		}
		_, err = WithoutUnchangedExtra(fresh.Extras, expected)
		return err
	}
	if _, err := Remove(req); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("conflict: %v", err)
	}
	fresh, err := config.LoadReadOnly()
	if err != nil || fresh.Extras[0].Branch != "changed-after-load" {
		t.Fatalf("conflicting entry overwritten: %+v %v", fresh, err)
	}
	if _, err := os.Stat(req.SourceDir); err != nil {
		t.Fatal("source removed", err)
	}
}

func TestRemove_PreWriteRefusalLeavesOwnedOutputs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires symlink privilege")
	}
	for _, mode := range []string{"merge", "symlink"} {
		t.Run(mode, func(t *testing.T) {
			req := removalFixture(t)
			target := filepath.Join(filepath.Dir(req.ExtrasRoot), "target")
			link, source := target, req.SourceDir
			if mode == "merge" {
				if err := os.Mkdir(target, 0755); err != nil {
					t.Fatal(err)
				}
				link, source = filepath.Join(target, "rule.md"), filepath.Join(req.SourceDir, "rule.md")
			}
			if err := os.Symlink(source, link); err != nil {
				t.Fatal(err)
			}
			cfg, err := config.LoadReadOnly()
			if err != nil {
				t.Fatal(err)
			}
			cfg.Extras[0].Targets = []config.ExtraTargetConfig{{Path: target, Mode: mode}}
			if err := cfg.Save(); err != nil {
				t.Fatal(err)
			}
			req.Extras = cfg.Extras
			expected := cfg.Extras[0]
			req.Save = func([]config.ExtraConfig) error {
				fresh, err := config.LoadReadOnly()
				if err != nil {
					return err
				}
				fresh.Extras[0].Branch = "intervening-edit"
				if err := fresh.Save(); err != nil {
					return err
				}
				_, err = WithoutUnchangedExtra(fresh.Extras, expected)
				return err
			}
			if _, err := Remove(req); !errors.Is(err, install.ErrExtraConfigNotWritten) {
				t.Fatalf("refusal: %v", err)
			}
			if dest, err := os.Readlink(link); err != nil || dest != source {
				t.Fatalf("owned output changed: %q %v", dest, err)
			}
			if data, err := os.ReadFile(filepath.Join(target, "rule.md")); err != nil || string(data) != "rules" {
				t.Fatal("configured output lost", err)
			}
		})
	}
}
