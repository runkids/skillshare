package sync

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
)

// setupMergeTest creates isolated source and target directories with skills.
func setupMergeTest(t *testing.T, skillNames ...string) (srcDir, tgtDir string) {
	t.Helper()
	tmp := t.TempDir()
	srcDir = filepath.Join(tmp, "source")
	tgtDir = filepath.Join(tmp, "target")
	os.MkdirAll(srcDir, 0755)
	os.MkdirAll(tgtDir, 0755)
	for _, name := range skillNames {
		writeSkillMD(t, filepath.Join(srcDir, name), "---\nname: "+name+"\n---\n# "+name)
	}
	return
}

func TestSyncTargetMerge_CreatesLinks(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha", "beta")
	target := config.TargetConfig{Path: tgt, Mode: "merge"}

	result, err := SyncTargetMerge("test", target, src, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Linked) != 2 {
		t.Errorf("expected 2 linked, got %d: %v", len(result.Linked), result.Linked)
	}

	// Verify symlinks exist
	for _, name := range []string{"alpha", "beta"} {
		linkPath := filepath.Join(tgt, name)
		info, err := os.Lstat(linkPath)
		if err != nil {
			t.Errorf("expected symlink for %s: %v", name, err)
			continue
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("expected %s to be a symlink", name)
		}
	}
}

func TestSyncTargetMerge_AlreadyLinked(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha")
	target := config.TargetConfig{Path: tgt, Mode: "merge"}

	// First sync
	if _, err := SyncTargetMerge("test", target, src, false, false); err != nil {
		t.Fatal(err)
	}

	// Second sync — should report already linked
	result, err := SyncTargetMerge("test", target, src, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Linked) != 1 {
		t.Errorf("expected 1 already-linked, got %d", len(result.Linked))
	}
	if len(result.Updated) != 0 {
		t.Errorf("expected 0 updated, got %d", len(result.Updated))
	}
}

func TestSyncTargetMerge_FixesBrokenLink(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source")
	tgt := filepath.Join(tmp, "target")
	skillSrc := filepath.Join(src, "skill-a")
	os.MkdirAll(skillSrc, 0755)
	writeSkillMD(t, skillSrc, "---\nname: skill-a\n---\n# Skill A")
	os.MkdirAll(tgt, 0755)

	// Create a broken symlink pointing to wrong location
	wrongTarget := filepath.Join(tmp, "wrong-source", "skill-a")
	os.MkdirAll(wrongTarget, 0755)
	os.Symlink(wrongTarget, filepath.Join(tgt, "skill-a"))

	target := config.TargetConfig{Path: tgt, Mode: "merge"}
	result, err := SyncTargetMerge("test", target, src, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Updated) != 1 {
		t.Errorf("expected 1 updated (fixed broken link), got %d", len(result.Updated))
	}
}

func TestSyncTargetMerge_SkipsLocalCopy(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha")
	target := config.TargetConfig{Path: tgt, Mode: "merge"}

	// Create a local (non-symlink) directory with same name
	localDir := filepath.Join(tgt, "alpha")
	os.MkdirAll(localDir, 0755)
	os.WriteFile(filepath.Join(localDir, "SKILL.md"), []byte("local version"), 0644)

	result, err := SyncTargetMerge("test", target, src, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Skipped) != 1 {
		t.Errorf("expected 1 skipped (local copy preserved), got %d", len(result.Skipped))
	}
}

func TestSyncTargetMerge_ForceReplacesLocal(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha")
	target := config.TargetConfig{Path: tgt, Mode: "merge"}

	// Create a local directory
	localDir := filepath.Join(tgt, "alpha")
	os.MkdirAll(localDir, 0755)
	os.WriteFile(filepath.Join(localDir, "SKILL.md"), []byte("local"), 0644)

	result, err := SyncTargetMerge("test", target, src, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Updated) != 1 {
		t.Errorf("expected 1 updated (force replaced), got %d", len(result.Updated))
	}

	// Verify it's now a symlink
	info, err := os.Lstat(filepath.Join(tgt, "alpha"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink after force replace")
	}
}

func TestSyncTargetMerge_DryRun(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha")
	target := config.TargetConfig{Path: tgt, Mode: "merge"}

	result, err := SyncTargetMerge("test", target, src, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Linked) != 1 {
		t.Errorf("expected 1 linked in dry-run, got %d", len(result.Linked))
	}

	// Verify no symlink was actually created
	if _, err := os.Lstat(filepath.Join(tgt, "alpha")); !os.IsNotExist(err) {
		t.Error("expected no symlink in dry-run mode")
	}
}

func TestSyncTargetMerge_IncludeExclude(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha", "beta", "gamma")
	target := config.TargetConfig{
		Path:    tgt,
		Mode:    "merge",
		Include: []string{"alpha", "beta"},
	}

	result, err := SyncTargetMerge("test", target, src, false, false)
	if err != nil {
		t.Fatal(err)
	}
	// Only alpha and beta should be linked
	if len(result.Linked) != 2 {
		t.Errorf("expected 2 linked (filtered), got %d: %v", len(result.Linked), result.Linked)
	}
	// gamma should not have a symlink
	if _, err := os.Lstat(filepath.Join(tgt, "gamma")); !os.IsNotExist(err) {
		t.Error("expected gamma to not be linked (excluded by filter)")
	}
}

func TestSyncTargetMerge_ConvertsFromSymlinkMode(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source")
	tgt := filepath.Join(tmp, "target")
	os.MkdirAll(src, 0755)
	writeSkillMD(t, filepath.Join(src, "alpha"), "---\nname: alpha\n---\n# Alpha")

	// Target is currently a symlink (symlink mode)
	os.Symlink(src, tgt)

	target := config.TargetConfig{Path: tgt, Mode: "merge"}
	result, err := SyncTargetMerge("test", target, src, false, false)
	if err != nil {
		t.Fatal(err)
	}

	// Should have removed the whole-directory symlink and created per-skill links
	info, err := os.Lstat(tgt)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("expected target to no longer be a symlink after conversion")
	}
	if !info.IsDir() {
		t.Error("expected target to be a directory")
	}
	if len(result.Linked) != 1 {
		t.Errorf("expected 1 linked after conversion, got %d", len(result.Linked))
	}
}

// --- Prune tests ---

func TestPruneOrphanLinks_RemovesOrphans(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha")
	target := config.TargetConfig{Path: tgt, Mode: "merge"}

	// Sync to create links
	if _, err := SyncTargetMerge("test", target, src, false, false); err != nil {
		t.Fatal(err)
	}

	// Add an orphan symlink pointing into source (simulates deleted skill)
	orphanSrc := filepath.Join(src, "deleted-skill")
	os.MkdirAll(orphanSrc, 0755)
	os.Symlink(orphanSrc, filepath.Join(tgt, "deleted-skill"))
	os.RemoveAll(orphanSrc) // Now it's a broken link to source

	result, err := PruneOrphanLinks(tgt, src, nil, nil, "test", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Removed) != 1 {
		t.Errorf("expected 1 removed orphan, got %d: %v", len(result.Removed), result.Removed)
	}
}

func TestPruneOrphanLinks_KeepsLocal(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha")
	target := config.TargetConfig{Path: tgt, Mode: "merge"}

	if _, err := SyncTargetMerge("test", target, src, false, false); err != nil {
		t.Fatal(err)
	}

	// Add a local (non-symlink) directory
	os.MkdirAll(filepath.Join(tgt, "my-local-skill"), 0755)

	result, err := PruneOrphanLinks(tgt, src, nil, nil, "test", false, false)
	if err != nil {
		t.Fatal(err)
	}
	// Local dir should be kept with warning (unknown directory)
	if len(result.Removed) != 0 {
		t.Errorf("expected 0 removed (local kept), got %d: %v", len(result.Removed), result.Removed)
	}
	if len(result.Warnings) != 1 {
		t.Errorf("expected 1 warning for unknown local dir, got %d", len(result.Warnings))
	}
}

func TestPruneOrphanLinks_RemovesExcluded(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha", "beta")
	target := config.TargetConfig{Path: tgt, Mode: "merge"}

	// Sync both skills
	if _, err := SyncTargetMerge("test", target, src, false, false); err != nil {
		t.Fatal(err)
	}

	// Now prune with beta excluded
	result, err := PruneOrphanLinks(tgt, src, nil, []string{"beta"}, "test", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Removed) != 1 {
		t.Errorf("expected 1 removed (excluded skill), got %d: %v", len(result.Removed), result.Removed)
	}

	// Alpha should still exist
	if _, err := os.Lstat(filepath.Join(tgt, "alpha")); err != nil {
		t.Error("expected alpha to still exist")
	}
	// Beta should be removed
	if _, err := os.Lstat(filepath.Join(tgt, "beta")); !os.IsNotExist(err) {
		t.Error("expected beta to be removed")
	}
}

func TestPruneOrphanLinks_KeepsExternal(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha")
	target := config.TargetConfig{Path: tgt, Mode: "merge"}

	if _, err := SyncTargetMerge("test", target, src, false, false); err != nil {
		t.Fatal(err)
	}

	// Add an external symlink (points outside source)
	externalDir := filepath.Join(t.TempDir(), "external-skill")
	os.MkdirAll(externalDir, 0755)
	os.Symlink(externalDir, filepath.Join(tgt, "ext-skill"))

	result, err := PruneOrphanLinks(tgt, src, nil, nil, "test", false, false)
	if err != nil {
		t.Fatal(err)
	}
	// External symlink should be kept with warning
	if _, err := os.Lstat(filepath.Join(tgt, "ext-skill")); err != nil {
		t.Error("expected external symlink to be kept")
	}
	if len(result.Warnings) != 1 {
		t.Errorf("expected 1 warning for external symlink, got %d", len(result.Warnings))
	}
}

func TestPruneOrphanLinks_NonExistentTarget(t *testing.T) {
	result, err := PruneOrphanLinks("/nonexistent", "/nonexistent/src", nil, nil, "test", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Removed) != 0 {
		t.Errorf("expected 0 removed for non-existent target, got %d", len(result.Removed))
	}
}
