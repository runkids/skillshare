package sync

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/utils"
)

func TestProfileAwareWriters_MutationBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, writer, seed, stage, mutation, outcome string
		grant, legacy, force, dry                    bool
	}{
		{name: "merge_stale_clean", writer: "merge", seed: "copy", stage: "merge_replace", mutation: "edit", outcome: "kept"},
		{name: "copy_refresh_edit", seed: "copy", stage: "copy_remove", mutation: "edit", outcome: "kept"},
		{name: "merge_second_discard_edit", writer: "merge", seed: "copy", stage: "merge_replace", mutation: "edit", outcome: "kept", grant: true},
		{name: "copy_second_discard_edit", seed: "copy", stage: "copy_remove", mutation: "edit", outcome: "kept", grant: true},
		{name: "absent_directory_collision", stage: "copy_write", mutation: "edit", outcome: "blocked"},
		{name: "absent_external_link_collision", stage: "copy_write", mutation: "external", outcome: "blocked"},
		{name: "refresh_recreated_modified", seed: "copy", stage: "copy_write", mutation: "edit", outcome: "kept"},
		{name: "discard_recreated_modified", seed: "copy", stage: "copy_write", mutation: "edit", outcome: "kept", grant: true},
		{name: "discard_recreated_matching_grant", seed: "copy", stage: "copy_write", mutation: "first_edit", outcome: "kept", grant: true},
		{name: "refresh_recreated_clean", seed: "copy", stage: "copy_write", mutation: "clean", outcome: "retry"},
		{name: "conversion_recreated_managed_link", seed: "link", stage: "copy_write", mutation: "managed_link", outcome: "retry"},
		{name: "merge_reformat_external", writer: "merge", seed: "reformat", stage: "merge_reformat", mutation: "external", outcome: "blocked"},
		{name: "merge_unlink_file", writer: "merge", seed: "wrong_link", stage: "merge_unlink", mutation: "file", outcome: "blocked"},
		{name: "copy_unlink_external", seed: "link", stage: "copy_unlink", mutation: "external", outcome: "blocked"},
		{name: "clean_merge", writer: "merge", seed: "copy", outcome: "success"},
		{name: "clean_refresh", seed: "copy", outcome: "success"},
		{name: "absent_create", outcome: "success"},
		{name: "unchanged_merge_discard", writer: "merge", seed: "copy", outcome: "success", grant: true},
		{name: "unchanged_copy_discard", seed: "copy", outcome: "success", grant: true},
		{name: "legacy_merge_edit", writer: "merge", seed: "copy", stage: "merge_replace", mutation: "edit", outcome: "local", legacy: true},
		{name: "legacy_copy_edit", seed: "copy", stage: "copy_remove", mutation: "edit", outcome: "success", legacy: true},
		{name: "force_merge_edit", writer: "merge", seed: "copy", stage: "merge_replace", mutation: "edit", outcome: "success", force: true},
		{name: "force_copy_edit", seed: "copy", stage: "copy_remove", mutation: "edit", outcome: "success", force: true},
		{name: "dry_merge", writer: "merge", seed: "copy", outcome: "success", dry: true},
		{name: "dry_refresh", seed: "copy", outcome: "success", dry: true},
		{name: "dry_create", outcome: "success", dry: true},
		{name: "dry_modified", seed: "edited", outcome: "kept", dry: true},
		{name: "dry_unmanaged", seed: "unmanaged", outcome: "blocked", dry: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			read := func(path string) string {
				t.Helper()
				data, err := os.ReadFile(path)
				must(err)
				return string(data)
			}
			snapshot := func(root string) map[string]string {
				tree := make(map[string]string)
				err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
					if os.IsNotExist(err) && path == root {
						return nil
					}
					if err != nil {
						return err
					}
					info, err := entry.Info()
					must(err)
					value := info.Mode().String()
					if utils.IsLinkMode(path, info.Mode()) {
						link, err := os.Readlink(path)
						must(err)
						value += ":" + link
					} else if !entry.IsDir() {
						value += ":" + read(path)
					}
					rel, err := filepath.Rel(root, path)
					must(err)
					tree[rel] = value
					return nil
				})
				must(err)
				return tree
			}
			source, target := setupMergeTest(t, "demo")
			entry, skill := filepath.Join(target, "demo"), filepath.Join(source, "demo")
			original := read(filepath.Join(skill, "SKILL.md"))
			cfg := config.TargetConfig{Path: target}
			projectRoot := ""
			if tc.seed == "copy" || tc.seed == "edited" {
				_, err := SyncTargetCopy("claude", cfg, source, false, false)
				must(err)
			} else if tc.seed == "link" || tc.seed == "reformat" || tc.seed == "wrong_link" {
				_, err := SyncTargetMerge("claude", cfg, source, false, false, "")
				must(err)
				if tc.seed == "reformat" {
					projectRoot = filepath.Dir(source)
				}
				if tc.seed == "wrong_link" {
					other := filepath.Join(source, "not-a-skill")
					must(os.MkdirAll(other, 0755))
					must(os.Remove(entry))
					must(os.Symlink(other, entry))
				}
			}
			var discard []DiscardEntry
			if tc.grant || tc.seed == "edited" || tc.seed == "unmanaged" {
				writeSkillMD(t, entry, "first edit")
				if tc.grant {
					fingerprint, err := CopyOutputFingerprint(entry)
					must(err)
					discard = []DiscardEntry{{Target: "claude", Dir: target, Entry: "demo", Fingerprint: fingerprint}}
				}
			}
			if tc.writer != "merge" && (tc.seed == "copy" || tc.seed == "edited") {
				writeSkillMD(t, skill, "---\nname: demo\n---\nchanged source\n")
				manifest, err := ReadManifest(target)
				must(err)
				delete(manifest.Mtimes, "demo") // Force the checksum path without clock assumptions.
				must(WriteManifest(target, manifest))
			}
			if tc.seed == "" {
				writeSkillMD(t, filepath.Join(source, "zeta"), "---\nname: zeta\n---\nsecond selected\n")
			}
			skills, err := DiscoverSourceSkills(source)
			must(err)
			records := func() string {
				manifest, err := ReadManifest(target)
				must(err)
				manifest.Remove("zeta") // The second selected entry may complete independently.
				data, err := json.Marshal([]any{manifest.Managed, manifest.Mtimes, manifest.Sources, manifest.Naming, manifest.Outputs})
				must(err)
				return string(data)
			}
			beforeRecords, beforeTree := records(), snapshot(target)
			sourceBefore := snapshot(source)
			outside := t.TempDir()
			writeSkillMD(t, outside, "external authored content")
			outsideBefore := snapshot(outside)
			preserved := snapshot(entry)
			fingerprint := ""
			fired := false
			callback := func(stage string) {
				if tc.dry {
					t.Fatal("dry-run invoked mutation callback", stage)
				}
				if fired || stage != tc.stage {
					return
				}
				fired = true
				switch tc.mutation {
				case "edit", "clean", "first_edit":
					text := "later edit"
					if tc.mutation == "clean" {
						text = original
					} else if tc.mutation == "first_edit" {
						text = "first edit"
					}
					writeSkillMD(t, entry, text)
					if tc.mutation == "edit" {
						must(os.WriteFile(filepath.Join(entry, "sentinel"), []byte("authored extra"), 0644))
					}
				case "external", "managed_link", "file":
					if _, err := os.Lstat(entry); err == nil {
						must(os.Remove(entry))
					} else if !os.IsNotExist(err) {
						t.Fatal(err)
					}
					if tc.mutation == "file" {
						must(os.WriteFile(entry, []byte("authored file"), 0644))
					} else {
						dest := outside
						if tc.mutation == "managed_link" {
							dest = skill
						}
						must(os.Symlink(dest, entry))
					}
				}
				preserved = snapshot(entry)
				if tc.outcome == "kept" {
					fingerprint, err = CopyOutputFingerprint(entry)
					must(err)
					if tc.mutation == "first_edit" && fingerprint != discard[0].Fingerprint {
						t.Fatal("recreated edit no longer matches its prior discard grant")
					}
				}
			}
			var kept, blocked []EntryChange
			var skipped, success []string
			step := "refresh"
			if tc.writer == "merge" {
				step = "convert"
				result, runErr := syncTargetMergeWithPolicy("claude", cfg, skills, source, tc.dry, tc.force, projectRoot, !tc.legacy, discard, callback)
				err = runErr
				if result != nil {
					kept, blocked, skipped = result.KeptModified, result.Blocked, result.Skipped
					success = append(append(result.Linked, result.Updated...), result.Converted...)
				}
			} else {
				result, runErr := syncTargetCopyWithSkillsOptions("claude", cfg, skills, source, tc.dry, tc.force, nil, CopyOptions{ProfileAware: !tc.legacy, Discard: discard}, callback)
				err = runErr
				if result != nil {
					kept, blocked, skipped = result.KeptModified, result.Blocked, result.Skipped
					success = append(append(result.Copied, result.Updated...), result.Converted...)
				}
			}
			if fired != (tc.stage != "" && !tc.dry) {
				t.Fatal("mutation boundary not reached", tc.stage, fired, err)
			}
			if tc.outcome == "retry" {
				if err == nil || !strings.Contains(err.Error(), "destination changed") || !strings.Contains(err.Error(), "retry") {
					t.Errorf("expected contextual retry error, got %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if tc.dry {
				if !reflect.DeepEqual(beforeTree, snapshot(target)) {
					t.Error("dry-run changed target or manifest")
				}
			} else if tc.outcome != "success" {
				if !reflect.DeepEqual(preserved, snapshot(entry)) || records() != beforeRecords {
					t.Error("entry bytes/topology or manifest records changed")
				}
			}
			if !reflect.DeepEqual(sourceBefore, snapshot(source)) || !reflect.DeepEqual(outsideBefore, snapshot(outside)) {
				t.Error("source or external referent changed")
			}
			if tc.outcome == "success" {
				if !slices.Contains(success, "demo") || len(kept)+len(blocked) != 0 {
					t.Fatal("successful operation not reported", success, kept, blocked)
				}
				if !tc.dry {
					if tc.writer == "merge" {
						if !utils.IsSymlinkOrJunction(entry) {
							t.Error("clean conversion did not install link")
						}
					} else {
						manifest, err := ReadManifest(target)
						must(err)
						if !manifest.OwnsCopy("demo", entry) || manifest.Outputs["demo"] == "" || read(filepath.Join(entry, "SKILL.md")) != read(filepath.Join(skill, "SKILL.md")) {
							t.Error("successful copy lacks correct bytes/output ownership")
						}
					}
				}
			} else if slices.Contains(success, "demo") {
				t.Error("denied entry reported as successful", success)
			}
			if tc.outcome == "kept" || tc.outcome == "blocked" {
				changes, class := kept, EntryModifiedCopy
				if tc.outcome == "blocked" {
					changes, class = blocked, EntryUnmanaged
				}
				if tc.dry && class == EntryModifiedCopy {
					fingerprint, err = CopyOutputFingerprint(entry)
					must(err)
				}
				if len(changes) != 1 || len(kept)+len(blocked) != 1 || !reflect.DeepEqual(skipped, []string{"demo"}) {
					t.Errorf("expected exactly one kept/blocked/skip, got %v %v %v", kept, blocked, skipped)
				} else if c := changes[0]; c.Target != "claude" || c.Entry != "demo" || c.Step != step || c.Class != class || c.Reason == "" || c.Fingerprint != fingerprint {
					t.Errorf("wrong current refusal: %+v; fingerprint=%s", c, fingerprint)
				}
			}
			if tc.outcome == "local" && (!reflect.DeepEqual(skipped, []string{"demo"}) || len(kept)+len(blocked) != 0) {
				t.Error("legacy merge no longer keeps an edited copy locally", skipped, kept, blocked)
			}
			if tc.seed == "" && !tc.dry && (!slices.Contains(success, "zeta") || read(filepath.Join(target, "zeta", "SKILL.md")) != read(filepath.Join(source, "zeta", "SKILL.md"))) {
				t.Error("second selected entry did not complete")
			}
		})
	}
}

func TestProfileAwareWriters_KeptEntryProducesWarning(t *testing.T) {
	result := SkillTargetResult{Name: "claude", KeptModified: []EntryChange{{Target: "claude", Entry: "demo", Step: "prune", Reason: "edited"}}}
	result.addProfileWarnings()
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "demo") || !strings.Contains(result.Warnings[0], "edited") {
		t.Fatalf("missing actionable warning: %v", result.Warnings)
	}
}

func TestProfileAwareWriters_PruneRejectsUnreadableManifest(t *testing.T) {
	target := t.TempDir()
	if err := os.Mkdir(filepath.Join(target, ManifestFile), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := PruneOrphanLinksWithSkills(PruneOptions{TargetPath: target, SourcePath: t.TempDir(), ProfileAware: true}); err == nil {
		t.Fatal("profile-aware prune accepted an unreadable manifest")
	}
}

func TestProfileAwareWriters_FailedUnlinkDoesNotWriteThroughLink(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("directory permissions cannot exercise unlink failure")
	}
	source, target := t.TempDir(), t.TempDir()
	skill := filepath.Join(source, "demo")
	if err := os.MkdirAll(skill, 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(skill, "SKILL.md")
	content := "---\nname: demo\n---\nsource\n"
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(skill, filepath.Join(target, "demo")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := WriteManifest(target, &Manifest{Managed: map[string]string{"demo": manifestSymlink}}); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(target, 0555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(target, 0755)
	skills, err := DiscoverSourceSkills(source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SyncTargetCopyWithSkillsOptions("claude", config.TargetConfig{Path: target}, skills, source, false, false, nil, CopyOptions{ProfileAware: true}); err == nil {
		t.Fatal("failed unlink was ignored")
	}
	if data, err := os.ReadFile(file); err != nil || string(data) != content {
		t.Fatalf("source changed through target link: %q, %v", data, err)
	}
}

func TestProfileAwareWriters_MovedCleanupReportsKeptCopy(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, ".skillshare", "skills")
	dir := filepath.Join(root, ".goose", "skills")
	entry := filepath.Join(dir, "demo")
	if err := os.MkdirAll(entry, 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(entry, "SKILL.md")
	if err := os.WriteFile(file, []byte("clean"), 0644); err != nil {
		t.Fatal(err)
	}
	sum, err := CopyOutputFingerprint(entry)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteManifest(dir, &Manifest{Managed: map[string]string{"demo": "source"}, Outputs: map[string]string{"demo": sum}}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("edited"), 0644); err != nil {
		t.Fatal(err)
	}
	result := cleanMovedProjectDirsWithPolicy(root, source, []MovedTarget{{Name: "goose", Path: filepath.Join(root, ".agents", "skills")}}, false, true, nil)
	if len(result.Outcomes) != 1 || len(result.Outcomes[0].KeptModified) != 1 || !strings.Contains(result.Outcomes[0].KeptModified[0].Reason, dir) {
		t.Fatalf("kept copy not reported once: %+v", result)
	}
	if data, err := os.ReadFile(file); err != nil || string(data) != "edited" {
		t.Fatalf("moved copy changed: %q, %v", data, err)
	}
}

func TestProfileAwareWriters_PreserveCopyRefreshAndPrune(t *testing.T) {
	source := t.TempDir()
	target := filepath.Join(t.TempDir(), "target")
	for _, name := range []string{"keep", "edit"} {
		path := filepath.Join(source, name, "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("---\nname: "+name+"\n---\noriginal\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	tc := config.TargetConfig{Path: target}
	if _, err := SyncTargetCopy("claude", tc, source, false, false); err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(target, "edit", "SKILL.md")
	if err := os.WriteFile(entry, []byte("my edit"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "edit", "SKILL.md"), []byte("---\nname: edit\n---\nchanged source\n"), 0644); err != nil {
		t.Fatal(err)
	}
	manifest, err := ReadManifest(target)
	if err != nil {
		t.Fatal(err)
	}
	delete(manifest.Mtimes, "edit")
	if err := WriteManifest(target, manifest); err != nil {
		t.Fatal(err)
	}
	skills, err := DiscoverSourceSkills(source)
	if err != nil {
		t.Fatal(err)
	}
	result, err := SyncTargetCopyWithSkillsOptions("claude", tc, skills, source, false, false, nil, CopyOptions{ProfileAware: true})
	if err != nil || len(result.KeptModified) != 1 {
		t.Fatalf("refresh: %+v, %v", result, err)
	}
	plannedFingerprint, err := CopyOutputFingerprint(filepath.Join(target, "edit"))
	if err != nil || result.KeptModified[0].Fingerprint != plannedFingerprint {
		t.Fatalf("plan did not retain the observed fingerprint: %+v, %v", result.KeptModified[0], err)
	}
	if data, err := os.ReadFile(entry); err != nil || string(data) != "my edit" {
		t.Fatalf("edited refresh overwritten: %q, %v", data, err)
	}
	sc := tc.SkillsConfig()
	sc.Exclude = []string{"edit"}
	pruned, err := pruneOrphanCopiesWithPolicy("claude", sc, skills, false, source, true, nil, nil)
	if err != nil || len(pruned.KeptModified) != 1 {
		t.Fatalf("prune: %+v, %v", pruned, err)
	}
	if data, err := os.ReadFile(entry); err != nil || string(data) != "my edit" {
		t.Fatalf("edited prune removed: %q, %v", data, err)
	}
	m, err := ReadManifest(target)
	if err != nil || m.Managed["edit"] == "" {
		t.Fatalf("kept entry lost manifest record: %+v, %v", m, err)
	}
}

func TestProfileAwareWriters_ReportsEditedCopyOnMtimeFastPath(t *testing.T) {
	source := t.TempDir()
	target := filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(filepath.Join(source, "demo"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "demo", "SKILL.md"), []byte("---\nname: demo\n---\noriginal\n"), 0644); err != nil {
		t.Fatal(err)
	}
	tc := config.TargetConfig{Path: target}
	if _, err := SyncTargetCopy("claude", tc, source, false, false); err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(target, "demo", "SKILL.md")
	if err := os.WriteFile(entry, []byte("local edit"), 0644); err != nil {
		t.Fatal(err)
	}
	skills, err := DiscoverSourceSkills(source)
	if err != nil {
		t.Fatal(err)
	}
	result, err := SyncTargetCopyWithSkillsOptions("claude", tc, skills, source, false, false, nil, CopyOptions{ProfileAware: true})
	if err != nil || len(result.KeptModified) != 1 {
		t.Fatalf("edited fast-path entry not reported: %+v, %v", result, err)
	}
	if data, err := os.ReadFile(entry); err != nil || string(data) != "local edit" {
		t.Fatalf("edited entry changed: %q, %v", data, err)
	}
}

func TestProfileAwareWriters_ForceOnlyOverridesSelectedEntries(t *testing.T) {
	source, target := t.TempDir(), filepath.Join(t.TempDir(), "target")
	for _, name := range []string{"selected", "departing"} {
		dir := filepath.Join(source, name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\n---\noriginal\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	tc := config.TargetConfig{Path: target}
	if _, err := SyncTargetCopy("claude", tc, source, false, false); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"selected", "departing"} {
		if err := os.WriteFile(filepath.Join(target, name, "SKILL.md"), []byte("local edit"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	skills, err := DiscoverSourceSkills(source)
	if err != nil {
		t.Fatal(err)
	}
	tc.Exclude = []string{"departing"}
	result := SyncSkillTarget(SkillTarget{Name: "claude", Target: tc, Mode: "copy"}, skills, SkillRunOptions{Source: source, ProfileAware: true, Force: true})
	if result.Err != nil || len(result.KeptModified) != 1 || result.KeptModified[0].Entry != "departing" || len(result.Refreshed) != 1 || result.Refreshed[0] != "selected" {
		t.Fatalf("force/prune result: %+v", result)
	}
	if data, err := os.ReadFile(filepath.Join(target, "selected", "SKILL.md")); err != nil || !strings.Contains(string(data), "original") {
		t.Fatalf("selected force did not refresh: %q, %v", data, err)
	}
	if data, err := os.ReadFile(filepath.Join(target, "departing", "SKILL.md")); err != nil || string(data) != "local edit" {
		t.Fatalf("force changed departing edit: %q, %v", data, err)
	}
}

func TestProfileAwareWriters_PruneLegacyRewriteWithCustomIgnore(t *testing.T) {
	sourceRoot, target := t.TempDir(), t.TempDir()
	source := filepath.Join(sourceRoot, "_repo", "demo")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("---\nname: demo\n---\noriginal\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "private.tmp"), []byte("ignored"), 0644); err != nil {
		t.Fatal(err)
	}
	ignore := append(DefaultFileIgnorePatterns(), "private.tmp")
	entry := filepath.Join(target, "repo-demo")
	if err := copySkillToTarget(source, entry, "repo-demo", ignore); err != nil {
		t.Fatal(err)
	}
	sum, err := DirChecksumWithIgnore(source, ignore)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteManifest(target, &Manifest{Managed: map[string]string{"repo-demo": sum}, Naming: map[string]string{"repo-demo": "prefixed"}}); err != nil {
		t.Fatal(err)
	}
	skills := []DiscoveredSkill{{SourcePath: source, RelPath: "_repo/demo", FlatName: "_repo__demo", IsInRepo: true, RepoRelPath: "_repo"}}
	migrationTarget := t.TempDir()
	if err := copySkillToTarget(source, filepath.Join(migrationTarget, "repo-demo"), "repo-demo", ignore); err != nil {
		t.Fatal(err)
	}
	migrationManifest := &Manifest{Managed: map[string]string{"repo-demo": sum}, Naming: map[string]string{"repo-demo": "prefixed"}}
	resolved := ResolvedTargetSkill{Skill: skills[0], SkillName: "demo", TargetName: "demo"}
	name, err := selectActiveTargetNameForSync("copy", migrationTarget, resolved, map[string]bool{"demo": true}, migrationManifest, false, true, ignore)
	if err != nil || name != "demo" {
		t.Fatalf("clean legacy copy with custom ignore was not renamed: %s, %v", name, err)
	}
	// A prior naming migration may already have moved the folder while retaining
	// its old prefixed output until the next writer refreshes it.
	delete(migrationManifest.Outputs, "demo")
	if err := WriteManifest(migrationTarget, migrationManifest); err != nil {
		t.Fatal(err)
	}
	tc := config.TargetConfig{Skills: &config.ResourceTargetConfig{Path: migrationTarget, TargetNaming: "standard"}}
	merge, err := SyncTargetMergeWithPolicy("claude", tc, skills, sourceRoot, false, false, "", true, nil, ignore)
	if err != nil || len(merge.Updated) != 1 || merge.Updated[0] != "demo" {
		t.Fatalf("clean renamed legacy copy was not converted: %+v, %v", merge, err)
	}
	opts := PruneOptions{TargetPath: target, SourcePath: sourceRoot, TargetName: "claude", TargetNaming: "standard", Exclude: []string{"_repo__demo"}, Skills: skills, ProfileAware: true, IgnorePatterns: ignore}
	result, err := PruneOrphanLinksWithSkills(opts)
	if err != nil || len(result.Removed) != 1 || result.Removed[0] != "repo-demo" {
		t.Fatalf("clean legacy copy was not pruned: %+v, %v", result, err)
	}
}

func TestProfileAwareWriters_MergePruneKeepsEditedCopy(t *testing.T) {
	root := t.TempDir()
	source, target := filepath.Join(root, "source"), filepath.Join(root, "target")
	if err := os.MkdirAll(filepath.Join(target, "edit"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(target, "edit")
	if err := os.WriteFile(filepath.Join(entry, "SKILL.md"), []byte("clean"), 0644); err != nil {
		t.Fatal(err)
	}
	sum, err := CopyOutputFingerprint(entry)
	if err != nil {
		t.Fatal(err)
	}
	m := &Manifest{Managed: map[string]string{"edit": "source"}, Outputs: map[string]string{"edit": sum}}
	if err := WriteManifest(target, m); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(entry, "SKILL.md"), []byte("edited"), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := PruneOrphanLinksWithSkills(PruneOptions{TargetPath: target, SourcePath: source, ProfileAware: true})
	if err != nil || len(result.KeptModified) != 1 {
		t.Fatalf("merge prune: %+v, %v", result, err)
	}
	if data, err := os.ReadFile(filepath.Join(entry, "SKILL.md")); err != nil || string(data) != "edited" {
		t.Fatalf("merge prune removed edit: %q, %v", data, err)
	}
	m, err = ReadManifest(target)
	if err != nil || m.Managed["edit"] == "" {
		t.Fatalf("merge prune dropped record: %+v, %v", m, err)
	}
}

func TestProfileAwareWriters_DepartingUnmanagedEntryIsNotBlocked(t *testing.T) {
	target, source := t.TempDir(), t.TempDir()
	entry := filepath.Join(target, "departed")
	if err := os.MkdirAll(entry, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(entry, "SKILL.md"), []byte("local"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := WriteManifest(target, &Manifest{Managed: map[string]string{"departed": manifestSymlink}}); err != nil {
		t.Fatal(err)
	}
	result, err := pruneOrphanCopiesWithPolicy("claude", config.ResourceTargetConfig{Path: target}, nil, false, source, true, nil, nil)
	if err != nil || len(result.Blocked) != 0 || len(result.Removed) != 0 {
		t.Fatalf("unmanaged departure blocked switch: %+v, %v", result, err)
	}
	if data, err := os.ReadFile(filepath.Join(entry, "SKILL.md")); err != nil || string(data) != "local" {
		t.Fatalf("unmanaged departure changed: %q, %v", data, err)
	}
}

func TestProfileAwareWriters_NamingMigrationKeepsEditedCopy(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source", "demo")
	target := filepath.Join(root, "target")
	entry := filepath.Join(target, "team-demo")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("---\nname: demo\n---\noriginal\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := copySkillToTarget(source, entry, "team-demo", DefaultFileIgnorePatterns()); err != nil {
		t.Fatal(err)
	}
	sum, err := CopyOutputFingerprint(entry)
	if err != nil {
		t.Fatal(err)
	}
	m := &Manifest{Managed: map[string]string{"team-demo": "source"}, Outputs: map[string]string{"team-demo": sum}, Naming: map[string]string{"team-demo": "prefixed"}}
	if err := os.WriteFile(filepath.Join(entry, "SKILL.md"), []byte("edited"), 0644); err != nil {
		t.Fatal(err)
	}
	skill := ResolvedTargetSkill{Skill: DiscoveredSkill{SourcePath: source, FlatName: "_team__demo", IsInRepo: true, RepoRelPath: "_team"}, SkillName: "demo", TargetName: "demo"}
	name, err := selectActiveTargetNameForSync("copy", target, skill, map[string]bool{"demo": true}, m, false, true, nil)
	if err != nil || name != "team-demo" {
		t.Fatalf("edited legacy entry renamed: %s, %v", name, err)
	}
	if _, err := os.Stat(entry); err != nil {
		t.Fatal(err)
	}
}

func TestProfileAwareWriters_NamingMigrationRecordsVerifiedLegacyOutput(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source", "_team", "demo")
	target := filepath.Join(root, "target")
	entry := filepath.Join(target, "team-demo")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("---\nname: demo\n---\noriginal\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := copySkillToTarget(source, entry, "team-demo", DefaultFileIgnorePatterns()); err != nil {
		t.Fatal(err)
	}
	sum, err := DirChecksumWithIgnore(source, DefaultFileIgnorePatterns())
	if err != nil {
		t.Fatal(err)
	}
	m := &Manifest{Managed: map[string]string{"team-demo": sum}, Naming: map[string]string{"team-demo": "prefixed"}}
	skill := ResolvedTargetSkill{Skill: DiscoveredSkill{SourcePath: source, FlatName: "_team__demo", IsInRepo: true, RepoRelPath: "_team"}, SkillName: "demo", TargetName: "demo"}
	name, err := selectActiveTargetNameForSync("copy", target, skill, map[string]bool{"demo": true}, m, false, true, nil)
	if err != nil || name != "demo" {
		t.Fatalf("legacy copy was not renamed: %s, %v", name, err)
	}
	if !m.OwnsCopy("demo", filepath.Join(target, "demo")) {
		t.Fatal("verified output lost during rename")
	}
}

func TestProfileAwareWriters_MergeKeepsUnmanagedLink(t *testing.T) {
	source := t.TempDir()
	target := filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(filepath.Join(source, "demo"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "demo", "SKILL.md"), []byte("---\nname: demo\n---\nDemo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	entry := filepath.Join(target, "demo")
	if err := os.Symlink(outside, entry); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	skills, err := DiscoverSourceSkills(source)
	if err != nil {
		t.Fatal(err)
	}
	result, err := SyncTargetMergeWithPolicy("claude", config.TargetConfig{Path: target}, skills, source, false, false, "", true, nil)
	if err != nil || len(result.Blocked) != 1 {
		t.Fatalf("merge collision: %+v, %v", result, err)
	}
	link, err := os.Readlink(entry)
	if err != nil || link != outside {
		t.Fatalf("unmanaged link changed: %s, %v", link, err)
	}
}

func TestProfileAwareWriters_CopyBlocksUnmanagedCollision(t *testing.T) {
	source, target := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(source, "demo"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "demo", "SKILL.md"), []byte("---\nname: demo\n---\nsource\n"), 0644); err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(target, "demo")
	if err := os.WriteFile(entry, []byte("local"), 0644); err != nil {
		t.Fatal(err)
	}
	skills, err := DiscoverSourceSkills(source)
	if err != nil {
		t.Fatal(err)
	}
	result, err := SyncTargetCopyWithSkillsOptions("claude", config.TargetConfig{Path: target}, skills, source, false, false, nil, CopyOptions{ProfileAware: true, Discard: []DiscardEntry{{Target: "claude", Dir: target, Entry: "demo", Fingerprint: "anything"}}})
	if err != nil || len(result.Blocked) != 1 {
		t.Fatalf("unmanaged collision not blocked: %+v, %v", result, err)
	}
	if data, err := os.ReadFile(entry); err != nil || string(data) != "local" {
		t.Fatalf("unmanaged file changed: %q, %v", data, err)
	}
}

func TestProfileAwareWriters_MergeDiscardRechecksFingerprint(t *testing.T) {
	for _, stale := range []bool{false, true} {
		t.Run(map[bool]string{false: "unchanged", true: "changed_after_plan"}[stale], func(t *testing.T) {
			source, target := t.TempDir(), t.TempDir()
			probe := filepath.Join(t.TempDir(), "probe")
			if err := os.Symlink(source, probe); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
			skill := filepath.Join(source, "demo")
			entry := filepath.Join(target, "demo")
			if err := os.MkdirAll(skill, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: demo\n---\nsource\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := copySkillToTarget(skill, entry, "", DefaultFileIgnorePatterns()); err != nil {
				t.Fatal(err)
			}
			clean, err := CopyOutputFingerprint(entry)
			if err != nil {
				t.Fatal(err)
			}
			if err := WriteManifest(target, &Manifest{Managed: map[string]string{"demo": "source"}, Outputs: map[string]string{"demo": clean}}); err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(entry, "SKILL.md")
			if err := os.WriteFile(file, []byte("first edit"), 0644); err != nil {
				t.Fatal(err)
			}
			planned, err := CopyOutputFingerprint(entry)
			if err != nil {
				t.Fatal(err)
			}
			if stale {
				if err := os.WriteFile(file, []byte("later edit"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			skills, err := DiscoverSourceSkills(source)
			if err != nil {
				t.Fatal(err)
			}
			result, err := SyncTargetMergeWithPolicy("claude", config.TargetConfig{Path: target}, skills, source, false, false, "", true, []DiscardEntry{{Target: "claude", Dir: target, Entry: "demo", Fingerprint: planned}})
			if err != nil {
				t.Fatal(err)
			}
			if stale {
				data, err := os.ReadFile(file)
				if err != nil || string(data) != "later edit" || len(result.KeptModified) != 1 {
					t.Fatalf("later edit not preserved: %q, %+v, %v", data, result, err)
				}
			} else if !utils.IsSymlinkOrJunction(entry) || len(result.Converted) != 1 || result.Converted[0] != "demo" {
				t.Fatalf("verified edit was not replaced: %+v", result)
			}
		})
	}
}

func TestProfileAwareWriters_ManagedLinkConvertsToCopy(t *testing.T) {
	source, target := t.TempDir(), t.TempDir()
	skill := filepath.Join(source, "demo")
	if err := os.MkdirAll(skill, 0755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: demo\n---\nsource\n"
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(target, "demo")
	if err := os.Symlink(skill, entry); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := WriteManifest(target, &Manifest{Managed: map[string]string{"demo": manifestSymlink}}); err != nil {
		t.Fatal(err)
	}
	skills, err := DiscoverSourceSkills(source)
	if err != nil {
		t.Fatal(err)
	}
	result, err := SyncTargetCopyWithSkillsOptions("claude", config.TargetConfig{Path: target}, skills, source, false, false, nil, CopyOptions{ProfileAware: true})
	if err != nil || len(result.Copied) != 1 || len(result.Converted) != 1 || result.Converted[0] != "demo" || utils.IsSymlinkOrJunction(entry) {
		t.Fatalf("managed link was not converted: %+v, %v", result, err)
	}
	manifest, err := ReadManifest(target)
	if err != nil || manifest.Outputs["demo"] == "" || !manifest.OwnsCopy("demo", entry) {
		t.Fatalf("converted copy not recorded: %+v, %v", manifest, err)
	}
	if data, err := os.ReadFile(filepath.Join(skill, "SKILL.md")); err != nil || string(data) != content {
		t.Fatalf("source changed: %q, %v", data, err)
	}
}

func TestProfileAwareWriters_ConfiguredTargetAlias(t *testing.T) {
	source, outside := t.TempDir(), t.TempDir()
	target := filepath.Join(t.TempDir(), "target")
	if runtime.GOOS == "windows" {
		if err := utils.CreateJunction(target, outside); err != nil {
			t.Fatal(err)
		}
	} else if err := os.Symlink(outside, target); err != nil {
		t.Fatal(err)
	}
	rootLink, err := os.Readlink(target)
	if err != nil {
		t.Fatal(err)
	}
	for root, content := range map[string]string{source: "source content", outside: "authored content"} {
		entry := filepath.Join(root, "demo")
		if err := os.MkdirAll(entry, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(entry, "SKILL.md"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	skills, err := DiscoverSourceSkills(source)
	if err != nil || len(skills) != 1 {
		t.Fatalf("setup requires one source skill: %v, %v", skills, err)
	}
	tc := config.TargetConfig{Path: target}
	copyResult, err := SyncTargetCopyWithSkillsOptions("claude", tc, skills, source, false, false, nil, CopyOptions{ProfileAware: true})
	if err != nil || len(copyResult.Blocked) != 1 || copyResult.Blocked[0].Class != EntryUnmanaged {
		t.Fatalf("copy must accept configured alias and block unmanaged child: %+v, %v", copyResult, err)
	}
	mergeResult, err := SyncTargetMergeWithPolicy("claude", tc, skills, source, false, false, "", true, nil)
	if err != nil || len(mergeResult.Blocked) != 1 || mergeResult.Blocked[0].Class != EntryUnmanaged {
		t.Fatalf("merge must accept configured alias and block unmanaged child: %+v, %v", mergeResult, err)
	}
	entry := filepath.Join(outside, "demo")
	if data, err := os.ReadFile(filepath.Join(entry, "SKILL.md")); err != nil || string(data) != "authored content" || utils.IsSymlinkOrJunction(entry) {
		t.Fatalf("configured alias lost unmanaged child: %q, %v", data, err)
	}
	if after, err := os.Readlink(target); err != nil || after != rootLink || !utils.IsSymlinkOrJunction(target) {
		t.Fatalf("configured root alias changed: %q, %v", after, err)
	}
}

func TestProfileAwareWriters_DiscardDirectory(t *testing.T) {
	for _, step := range []string{"refresh", "convert", "prune_links", "prune_copies", "moved"} {
		for _, matching := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/matching=%v", step, matching), func(t *testing.T) {
				must := func(err error) {
					t.Helper()
					if err != nil {
						t.Fatal(err)
					}
				}
				if step == "convert" {
					probe := filepath.Join(t.TempDir(), "probe")
					if err := os.Symlink(t.TempDir(), probe); err != nil {
						t.Skipf("symlink unavailable: %v", err)
					}
				}
				source, target := setupMergeTest(t, "demo")
				root := t.TempDir()
				if step == "moved" {
					target = filepath.Join(root, ".goose", "skills")
				}
				name := "goose"
				cfg := config.TargetConfig{Path: target}
				_, err := SyncTargetCopy(name, cfg, source, false, false)
				must(err)
				manifest, err := ReadManifest(target)
				must(err)
				delete(manifest.Mtimes, "demo") // Force checksum refresh without a clock assumption.
				must(WriteManifest(target, manifest))
				entry := filepath.Join(target, "demo")
				writeSkillMD(t, entry, "authored")
				fingerprint, err := CopyOutputFingerprint(entry)
				must(err)
				dir := target
				if !matching {
					dir = t.TempDir()
				}
				grants := []DiscardEntry{{Target: name, Dir: dir, Entry: "demo", Fingerprint: fingerprint}}
				writeSkillMD(t, filepath.Join(source, "demo"), "---\nname: demo\n---\nchanged source\n")
				skills, err := DiscoverSourceSkills(source)
				must(err)
				var kept []EntryChange
				switch step {
				case "refresh":
					result, err := SyncTargetCopyWithSkillsOptions(name, cfg, skills, source, false, false, nil, CopyOptions{ProfileAware: true, Discard: grants})
					must(err)
					kept = result.KeptModified
				case "convert":
					result, err := SyncTargetMergeWithPolicy(name, cfg, skills, source, false, false, "", true, grants)
					must(err)
					kept = result.KeptModified
				case "prune_links":
					result, err := PruneOrphanLinksWithSkills(PruneOptions{TargetPath: target, TargetName: name, SourcePath: source, ManagedOnly: true, ProfileAware: true, Discard: grants})
					must(err)
					kept = result.KeptModified
				case "prune_copies":
					result, err := pruneOrphanCopiesWithPolicy(name, cfg.SkillsConfig(), nil, false, source, true, grants, nil)
					must(err)
					kept = result.KeptModified
				case "moved":
					result := cleanMovedProjectDirsWithPolicy(root, source, []MovedTarget{{Name: name, Path: filepath.Join(root, ".agents", "skills")}}, false, true, grants)
					for _, row := range result.Outcomes {
						kept = append(kept, row.KeptModified...)
					}
				}
				if !matching {
					data, err := os.ReadFile(filepath.Join(entry, "SKILL.md"))
					must(err)
					if string(data) != "authored" || len(kept) != 1 || kept[0].Dir != target {
						t.Fatalf("wrong-directory grant changed edit or lost its location: %q, %+v", data, kept)
					}
				} else if len(kept) != 0 {
					t.Fatalf("verified directory was kept: %+v", kept)
				} else if step == "convert" && !utils.IsSymlinkOrJunction(entry) {
					t.Fatal("verified copy was not converted")
				} else if step == "refresh" {
					data, err := os.ReadFile(filepath.Join(entry, "SKILL.md"))
					must(err)
					if string(data) == "authored" {
						t.Fatal("verified copy was not refreshed")
					}
				} else if step != "convert" {
					_, err := os.Lstat(entry)
					if !os.IsNotExist(err) {
						t.Fatalf("verified departed copy remains: %v", err)
					}
				}
			})
		}
	}
}

func TestProfileAwareWriters_BinaryCopyPreservation(t *testing.T) {
	for _, scenario := range []string{"copy_refresh", "merge_refresh", "copy_prune", "merge_prune", "moved_prune"} {
		t.Run(scenario, func(t *testing.T) {
			source, target := t.TempDir(), t.TempDir()
			root := t.TempDir()
			if scenario == "moved_prune" {
				target = filepath.Join(root, ".goose", "skills")
			}
			writeSkillMD(t, filepath.Join(source, "demo"), "---\nname: demo\n---\nsource\n")
			asset := filepath.Join(source, "demo", "a")
			if err := os.WriteFile(asset, []byte("x\x00b\x00y"), 0600); err != nil {
				t.Fatal(err)
			}
			tc := config.TargetConfig{Path: target}
			if _, err := SyncTargetCopy("goose", tc, source, false, false); err != nil {
				t.Fatal(err)
			}
			entry := filepath.Join(target, "demo")
			if err := os.WriteFile(filepath.Join(entry, "a"), []byte("x"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(entry, "b"), []byte("y"), 0600); err != nil {
				t.Fatal(err)
			}
			manifest, err := ReadManifest(target)
			if err != nil {
				t.Fatal(err)
			}
			delete(manifest.Mtimes, "demo")
			if err := WriteManifest(target, manifest); err != nil {
				t.Fatal(err)
			}
			skills, err := DiscoverSourceSkills(source)
			if err != nil {
				t.Fatal(err)
			}
			if strings.HasSuffix(scenario, "prune") {
				tc.Exclude = []string{"demo"}
			}
			kept := 0
			switch {
			case scenario == "moved_prune":
				result := cleanMovedProjectDirsWithPolicy(root, source, []MovedTarget{{Name: "goose", Path: filepath.Join(root, ".agents", "skills")}}, false, true, nil)
				for _, out := range result.Outcomes {
					kept += len(out.KeptModified)
				}
			case strings.HasPrefix(scenario, "merge"):
				out, err := SyncTargetMergeWithPolicy("goose", tc, skills, source, false, false, "", true, nil)
				if err != nil {
					t.Fatal(err)
				}
				kept = len(out.KeptModified)
				if scenario == "merge_prune" {
					pruned, err := PruneOrphanLinksWithSkills(PruneOptions{TargetName: "goose", TargetPath: target, SourcePath: source, Skills: skills, Exclude: tc.Exclude, ProfileAware: true})
					if err != nil {
						t.Fatal(err)
					}
					kept += len(pruned.KeptModified)
				}
			default:
				out, err := SyncTargetCopyWithSkillsOptions("goose", tc, skills, source, false, false, nil, CopyOptions{ProfileAware: true})
				if err != nil {
					t.Fatal(err)
				}
				kept = len(out.KeptModified)
				if strings.HasSuffix(scenario, "prune") {
					out, err := pruneOrphanCopiesWithPolicy("goose", tc.SkillsConfig(), skills, false, source, true, nil, nil)
					if err != nil {
						t.Fatal(err)
					}
					kept += len(out.KeptModified)
				}
			}
			for name, want := range map[string]string{"a": "x", "b": "y"} {
				data, err := os.ReadFile(filepath.Join(entry, name))
				if err != nil || string(data) != want {
					t.Errorf("%s lost authored bytes: %q %v", name, data, err)
				}
			}
			if kept != 1 {
				t.Errorf("kept outcomes=%d, want one modified copy", kept)
			}
		})
	}
}

func TestProfileAwareWriters_MovedLegacyReplay(t *testing.T) {
	for _, available := range []bool{false, true} {
		t.Run(fmt.Sprint(available), func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, ".skillshare", "skills")
			skill := filepath.Join(source, "demo")
			dir := filepath.Join(root, ".goose", "skills")
			entry := filepath.Join(dir, "demo")
			writeSkillMD(t, skill, "---\nname: demo\n---\nsource\n")
			if err := copySkillToTarget(skill, entry, "", DefaultFileIgnorePatterns()); err != nil {
				t.Fatal(err)
			}
			sum, err := DirChecksumWithIgnore(skill, DefaultFileIgnorePatterns())
			if err != nil {
				t.Fatal(err)
			}
			if err := WriteManifest(dir, &Manifest{Managed: map[string]string{"demo": sum}}); err != nil {
				t.Fatal(err)
			}
			skills := []DiscoveredSkill{{SourcePath: skill, RelPath: "demo", FlatName: "demo"}}
			if !available {
				skills = nil
			}
			moved := []MovedTarget{{Name: "goose", Path: filepath.Join(root, ".agents", "skills")}}
			out := cleanMovedProjectDirsWithPolicy(root, source, moved, false, true, nil, PruneOptions{Skills: skills})
			_, statErr := os.Stat(entry)
			if available {
				if !os.IsNotExist(statErr) || len(out.Outcomes) != 0 {
					t.Fatalf("reproducible former copy not cleaned: %+v %v", out, statErr)
				}
			} else if statErr != nil || len(out.Outcomes) != 1 || len(out.Outcomes[0].KeptModified) != 1 {
				t.Fatalf("unverifiable former copy not kept: %+v %v", out, statErr)
			}
		})
	}
}

func TestProfileAwareWriters_LegacyDuplicateBasenameIdentity(t *testing.T) {
	for _, output := range []string{"missing", "bare"} {
		for _, producer := range []string{"a", "z"} {
			for _, scenario := range []string{"selected_merge", "copy_orphan", "merge_orphan", "moved", "edited_copy", "changed_source"} {
				t.Run(output+"/"+producer+"/"+scenario, func(t *testing.T) {
					root := t.TempDir()
					source := filepath.Join(root, "source")
					contents := map[string]string{
						"a": "---\nname: demo\n---\nfirst candidate bytes\n",
						"z": "---\nname: demo\n---\nlast candidate bytes\n",
					}
					for _, prefix := range []string{"a", "z"} {
						writeSkillMD(t, filepath.Join(source, prefix, "demo"), contents[prefix])
					}
					skills, err := DiscoverSourceSkills(source)
					if err != nil || len(skills) != 2 || skills[0].RelPath != "a/demo" || skills[1].RelPath != "z/demo" {
						t.Fatalf("duplicate basenames must have deterministic source order: %+v, %v", skills, err)
					}
					dir := filepath.Join(root, ".goose", "skills")
					entry := filepath.Join(dir, "demo")
					chosen := filepath.Join(source, producer, "demo")
					target := config.TargetConfig{Skills: &config.ResourceTargetConfig{Path: dir, TargetNaming: "standard", Include: []string{producer + "__demo"}}}
					// The ordinary copy writer produces the source checksum, naming, and deployed bytes.
					seed, err := SyncTargetCopy("goose", target, source, false, false)
					if err != nil || !reflect.DeepEqual(seed.Copied, []string{"demo"}) {
						t.Fatalf("default writer did not publish the selected source: %+v, %v", seed, err)
					}
					manifest, err := ReadManifest(dir)
					if err != nil {
						t.Fatal(err)
					}
					sourceSum, err := DirChecksumWithIgnore(chosen, DefaultFileIgnorePatterns())
					if err != nil || manifest.Managed["demo"] != sourceSum || manifest.Naming["demo"] != "standard" {
						t.Fatalf("default manifest lost producing source identity: %+v, %v", manifest, err)
					}
					if data, err := os.ReadFile(filepath.Join(entry, "SKILL.md")); err != nil || string(data) != contents[producer] {
						t.Fatalf("default writer deployed the wrong source bytes: %q, %v", data, err)
					}
					// Model the supported pre-output and pre-tag upgrade records; retain writer metadata.
					delete(manifest.Outputs, "demo")
					if output == "bare" {
						legacy, err := DirChecksumWithIgnore(entry, DefaultFileIgnorePatterns())
						if err != nil {
							t.Fatal(err)
						}
						manifest.Outputs["demo"] = legacy
					}
					if err := WriteManifest(dir, manifest); err != nil {
						t.Fatal(err)
					}
					manifest, err = ReadManifest(dir)
					if err != nil || !manifest.OwnsCopy("demo", entry) {
						t.Fatalf("supported legacy upgrade record is not owned: %+v, %v", manifest, err)
					}
					wantBytes := contents[producer]
					if scenario == "edited_copy" {
						wantBytes = "authored deployed bytes\n"
						writeSkillMD(t, entry, wantBytes)
					} else if scenario == "changed_source" {
						writeSkillMD(t, chosen, "---\nname: demo\n---\nchanged source bytes\n")
					}
					clean := scenario != "edited_copy" && scenario != "changed_source"
					resolution, err := ResolveTargetSkillsForTarget("goose", target.SkillsConfig(), skills)
					if err != nil || len(resolution.Skills) != 1 || resolution.Skills[0].Skill.SourcePath != chosen {
						t.Fatalf("selected writer resolved the wrong duplicate basename: %+v, %v", resolution, err)
					}
					if scenario == "copy_orphan" || scenario == "merge_orphan" || scenario == "moved" {
						resolution, err = ResolveTargetSkillsForTarget("goose", target.SkillsConfig(), nil)
						if err != nil {
							t.Fatal(err)
						}
					}
					before := snapshotDetachTree(t, root)
					classification, allowed := PreviewEntry("demo", dir, source, skills, manifest, nil)
					wantClass := EntryModifiedCopy
					if clean {
						wantClass = EntryCleanCopy
					}
					if classification.Class != wantClass || allowed != clean {
						t.Errorf("preview did not resolve recorded source and reproduce deployed bytes: %+v allowed=%v, want %s/%v", classification, allowed, wantClass, clean)
					}
					keptPreview := PreviewPreservedEntries("goose", dir, source, "merge", resolution, skills, manifest, nil)
					if clean && len(keptPreview) != 0 || !clean && (len(keptPreview) != 1 || keptPreview[0].Class != EntryModifiedCopy) {
						t.Errorf("preview preservation disagrees with legacy reproducibility: %+v", keptPreview)
					}
					if !reflect.DeepEqual(before, snapshotDetachTree(t, root)) {
						t.Error("legacy preview changed source, deployed copy, or manifest")
					}
					sourceBefore := snapshotDetachTree(t, source)
					var removed, converted []string
					var kept []EntryChange
					switch scenario {
					case "copy_orphan":
						out, err := pruneOrphanCopiesWithPolicy("goose", target.SkillsConfig(), nil, false, source, true, nil, nil, PruneOptions{Skills: skills})
						if err != nil {
							t.Fatal(err)
						}
						removed, kept = out.Removed, out.KeptModified
					case "merge_orphan":
						out, err := PruneOrphanLinksWithSkills(PruneOptions{TargetName: "goose", TargetPath: dir, SourcePath: source, TargetNaming: "standard", Skills: []DiscoveredSkill{}, VerificationSkills: skills, ProfileAware: true})
						if err != nil {
							t.Fatal(err)
						}
						removed, kept = out.Removed, out.KeptModified
					case "moved":
						moved := []MovedTarget{{Name: "goose", Path: filepath.Join(root, ".agents", "skills"), TargetNaming: "standard"}}
						if preview := PreviewMovedProjectEntries(root, source, moved, skills, nil); len(preview) != 0 {
							t.Errorf("moved preview falsely preserves reproducible legacy copy: %+v", preview)
						}
						out := cleanMovedProjectDirsWithPolicy(root, source, moved, false, true, nil, PruneOptions{Skills: skills})
						if len(out.Outcomes) != 0 || len(out.Messages) != 1 {
							t.Errorf("moved owner did not clean exactly the verified copy: %+v", out)
						}
					default:
						out, err := SyncTargetMergeWithPolicy("goose", target, skills, source, false, false, "", true, nil)
						if err != nil {
							t.Fatal(err)
						}
						converted, kept = out.Updated, out.KeptModified
						if len(out.Blocked) != 0 {
							t.Errorf("legacy copy unexpectedly blocked: %+v", out)
						}
					}
					if !reflect.DeepEqual(sourceBefore, snapshotDetachTree(t, source)) {
						t.Error("legacy operation changed either candidate source")
					}
					if !clean {
						if len(kept) != 1 || kept[0].Class != EntryModifiedCopy || len(converted)+len(removed) != 0 {
							t.Errorf("unreproducible legacy copy was not preserved: kept=%+v converted=%v removed=%v", kept, converted, removed)
						}
						if data, err := os.ReadFile(filepath.Join(entry, "SKILL.md")); err != nil || string(data) != wantBytes || utils.IsSymlinkOrJunction(entry) {
							t.Errorf("legacy verification lost deployed bytes: %q, %v", data, err)
						}
						after, err := ReadManifest(dir)
						if err != nil || !reflect.DeepEqual(after.Managed, manifest.Managed) || !reflect.DeepEqual(after.Outputs, manifest.Outputs) || !reflect.DeepEqual(after.Naming, manifest.Naming) || !reflect.DeepEqual(after.Mtimes, manifest.Mtimes) || !reflect.DeepEqual(after.Sources, manifest.Sources) {
							t.Errorf("denied legacy operation changed ownership records: %+v, %v", after, err)
						}
					} else if scenario == "selected_merge" {
						if !reflect.DeepEqual(converted, []string{"demo"}) || len(kept) != 0 || !utils.IsSymlinkOrJunction(entry) {
							t.Errorf("selected writer disagrees with clean legacy preview: converted=%v kept=%+v", converted, kept)
						}
						link, err := utils.ResolveLinkTarget(entry)
						if err != nil || !utils.PathsEqual(utils.ResolveSymlink(link), utils.ResolveSymlink(chosen)) {
							t.Errorf("selected writer converted to wrong duplicate source: %q, %v", link, err)
						}
					} else {
						if _, err := os.Lstat(entry); !os.IsNotExist(err) || len(kept) != 0 {
							t.Errorf("reproducible orphan was preserved: kept=%+v, %v", kept, err)
						}
						if scenario != "moved" && !reflect.DeepEqual(removed, []string{"demo"}) {
							t.Errorf("orphan owner failed to report removal: %v", removed)
						}
					}
				})
			}
		}
	}
}
