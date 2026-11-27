package install

import (
	"os"
	"path/filepath"
	"testing"
)

// writeSkill commits SKILL.md content at repo/subdir.
func writeSkill(t *testing.T, repo, subdir, body string) {
	t.Helper()
	dir := filepath.Join(repo, filepath.FromSlash(subdir))
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+filepath.Base(dir)+"\n---\n"+body+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "update "+subdir)
}

// The batch fast path must read the centralized store: sidecars no longer
// exist after migration, so reading them made every update --all reinstall
// every skill.
func TestUpdateSkillsFromRepo_SkipsSkillCurrentInStore(t *testing.T) {
	repo := initTestRepo(t)
	writeSkill(t, repo, "skills/my-skill", "# v1")

	sourceDir := t.TempDir()
	dest := filepath.Join(sourceDir, "my-skill")
	targets := map[string]string{"skills/my-skill": dest}
	opts := InstallOptions{Update: true, Force: true, SkipAudit: true, SourceDir: sourceDir}

	first, err := UpdateSkillsFromRepo("file://"+repo, "", targets, opts)
	if err != nil {
		t.Fatalf("initial install: %v", err)
	}
	if first.Results["skills/my-skill"] == nil {
		t.Fatalf("expected initial install, errors=%v", first.Errors)
	}

	second, err := UpdateSkillsFromRepo("file://"+repo, "", targets, opts)
	if err != nil {
		t.Fatalf("second update: %v", err)
	}
	if second.Results["skills/my-skill"] != nil || !second.Current["skills/my-skill"] {
		t.Fatalf("expected unchanged skill to be skipped as current, results=%v current=%v errors=%v",
			second.Results, second.Current, second.Errors)
	}
	if _, err := os.Stat(filepath.Join(dest, MetaFileName)); !os.IsNotExist(err) {
		t.Fatalf("fast path must not create a sidecar, stat err=%v", err)
	}

	// Metadata outlives a deleted install; the update must restore it.
	if err := os.RemoveAll(dest); err != nil {
		t.Fatal(err)
	}
	restored, err := UpdateSkillsFromRepo("file://"+repo, "", targets, opts)
	if err != nil {
		t.Fatalf("restore update: %v", err)
	}
	if restored.Results["skills/my-skill"] == nil {
		t.Fatalf("deleted install must be reinstalled, current=%v errors=%v", restored.Current, restored.Errors)
	}
	if _, err := os.Stat(filepath.Join(dest, "SKILL.md")); err != nil {
		t.Fatalf("expected SKILL.md restored: %v", err)
	}

	opts.Reinstall = true
	forced, err := UpdateSkillsFromRepo("file://"+repo, "", targets, opts)
	if err != nil {
		t.Fatalf("forced update: %v", err)
	}
	if forced.Results["skills/my-skill"] == nil {
		t.Fatalf("Reinstall must bypass the fast path, current=%v errors=%v", forced.Current, forced.Errors)
	}
}

// When HEAD moves but the skills' trees do not, the skip leaves the store
// untouched: nothing about the installed files changed, and a source under
// version control must stay clean (#1041).
func TestUpdateSkillsFromRepo_TreeMatchKeepsStore(t *testing.T) {
	repo := initTestRepo(t)
	writeSkill(t, repo, "skills/alpha", "# alpha v1")
	writeSkill(t, repo, "skills/beta", "# beta v1")

	sourceDir := t.TempDir()
	targets := map[string]string{
		"skills/alpha": filepath.Join(sourceDir, "alpha"),
		"skills/beta":  filepath.Join(sourceDir, "beta"),
	}
	opts := InstallOptions{Update: true, Force: true, SkipAudit: true, SourceDir: sourceDir}
	if _, err := UpdateSkillsFromRepo("file://"+repo, "", targets, opts); err != nil {
		t.Fatal(err)
	}
	metaPath := filepath.Join(sourceDir, MetadataFileName)
	before, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatal(err)
	}

	writeSkill(t, repo, "skills/other", "# unrelated")

	result, err := UpdateSkillsFromRepo("file://"+repo, "", targets, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Current["skills/alpha"] || !result.Current["skills/beta"] {
		t.Fatalf("expected tree-hash match to skip, results=%v errors=%v", result.Results, result.Errors)
	}
	if after, _ := os.ReadFile(metaPath); string(after) != string(before) {
		t.Fatalf("store changed on an unrelated upstream commit:\nbefore: %s\nafter:  %s", before, after)
	}
}

// Grouped skills may still be stored under a legacy basename key with a
// Group field; the lookup must find that entry.
func TestInstalledSkillMeta_LegacyGroupedKey(t *testing.T) {
	sourceDir := t.TempDir()
	dest := filepath.Join(sourceDir, "frontend", "my-skill")
	store := NewMetadataStore()
	store.Set("my-skill", &MetadataEntry{Source: "x", Group: "frontend", Version: "old", TreeHash: "tree"})

	installed := installedSkillMeta(store, sourceDir, dest)
	if installed == nil || installed.Version != "old" || installed.TreeHash != "tree" {
		t.Fatalf("legacy key lookup: got %+v", installed)
	}
}
