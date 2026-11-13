package sync

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/utils"
)

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
	result, err := SyncTargetCopyWithSkillsOptions("claude", config.TargetConfig{Path: target}, skills, source, false, false, nil, CopyOptions{ProfileAware: true, Discard: []DiscardEntry{{Target: "claude", Entry: "demo", Fingerprint: "anything"}}})
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
			result, err := SyncTargetMergeWithPolicy("claude", config.TargetConfig{Path: target}, skills, source, false, false, "", true, []DiscardEntry{{Target: "claude", Entry: "demo", Fingerprint: planned}})
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

func TestProfileAwareWriters_RefusesExternalTargetRoot(t *testing.T) {
	source, outside := t.TempDir(), t.TempDir()
	target := filepath.Join(t.TempDir(), "target")
	if err := os.Symlink(outside, target); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	tc := config.TargetConfig{Path: target}
	if _, err := SyncTargetCopyWithSkillsOptions("claude", tc, nil, source, false, false, nil, CopyOptions{ProfileAware: true}); err == nil {
		t.Fatal("copy followed unmanaged target root")
	}
	if _, err := SyncTargetMergeWithPolicy("claude", tc, nil, source, false, false, "", true, nil); err == nil {
		t.Fatal("merge followed unmanaged target root")
	}
}
