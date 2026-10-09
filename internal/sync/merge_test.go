package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/utils"
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

	result, err := SyncTargetMerge("test", target, src, false, false, "")
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
		if !utils.IsLinkMode(linkPath, info.Mode()) {
			t.Errorf("expected %s to be a symlink", name)
		}
	}
}

func TestSyncTargetMerge_AlreadyLinked(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha")
	target := config.TargetConfig{Path: tgt, Mode: "merge"}

	// First sync
	if _, err := SyncTargetMerge("test", target, src, false, false, ""); err != nil {
		t.Fatal(err)
	}

	// Second sync — should report already linked
	result, err := SyncTargetMerge("test", target, src, false, false, "")
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
	result, err := SyncTargetMerge("test", target, src, false, false, "")
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

	result, err := SyncTargetMerge("test", target, src, false, false, "")
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

	result, err := SyncTargetMerge("test", target, src, false, true, "")
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
	if !utils.IsLinkMode(filepath.Join(tgt, "alpha"), info.Mode()) {
		t.Error("expected symlink after force replace")
	}
}

// setupCopyModeCopy leaves tgt/name as copy mode would: a copy of the source
// skill and a manifest entry holding the source checksum.
func setupCopyModeCopy(t *testing.T, src, tgt, name string) string {
	t.Helper()
	patterns := DefaultFileIgnorePatterns()
	sum, err := DirChecksumWithIgnore(filepath.Join(src, name), patterns)
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(tgt, name)
	if err := copySkillToTarget(filepath.Join(src, name), dst, "", patterns); err != nil {
		t.Fatal(err)
	}
	m := &Manifest{Managed: map[string]string{name: sum}, Mtimes: map[string]int64{name: 42}, Naming: map[string]string{name: "flat"}}
	if err := WriteManifest(tgt, m); err != nil {
		t.Fatal(err)
	}
	return dst
}

// A copy the manifest records with a checksum was made by copy mode, so
// switching back to merge turns it into a link without --force.
func TestSyncTargetMerge_ReplacesCopyModeCopy(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha")
	target := config.TargetConfig{Path: tgt, Mode: "merge"}

	copyDir := setupCopyModeCopy(t, src, tgt, "alpha")

	result, err := SyncTargetMerge("test", target, src, false, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Updated) != 1 || len(result.Skipped) != 0 {
		t.Errorf("expected copy-mode copy updated, got updated=%v skipped=%v", result.Updated, result.Skipped)
	}
	if !utils.IsSymlinkOrJunction(copyDir) {
		t.Error("expected a link after switching back to merge")
	}
	manifest, _ := ReadManifest(tgt)
	if manifest.Managed["alpha"] != "symlink" {
		t.Errorf("manifest should record a link, got %q", manifest.Managed["alpha"])
	}
	if _, ok := manifest.Mtimes["alpha"]; ok {
		t.Error("copy-mode mtime should be forgotten once the entry is a link")
	}
	if _, ok := manifest.Naming["alpha"]; ok {
		t.Error("copy-mode naming should be forgotten once the entry is a link")
	}
}

// A parked copy left by a crashed earlier run may be the only one; never drop it.
func TestReplaceWithLink_KeepsLeftoverParkedCopy(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "alpha")
	leftover := parkedPath(dir)
	os.MkdirAll(dir, 0755)
	os.MkdirAll(leftover, 0755)
	os.WriteFile(filepath.Join(leftover, "SKILL.md"), []byte("only copy"), 0644)

	if err := replaceWithLink(dir, func() error { return nil }); err == nil {
		t.Fatal("expected an error for the leftover parked copy")
	}
	if got, _ := os.ReadFile(filepath.Join(leftover, "SKILL.md")); string(got) != "only copy" {
		t.Errorf("leftover parked copy was touched: %q", got)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("the folder must stay in place: %v", err)
	}
}

// A flattened nested skill can be near the 255-byte name limit; the parked name must not grow with it.
func TestReplaceWithLink_LongEntryName(t *testing.T) {
	dir := filepath.Join(t.TempDir(), strings.Repeat("a", 250))
	os.MkdirAll(dir, 0755)

	if err := replaceWithLink(dir, func() error { return os.Symlink(t.TempDir(), dir) }); err != nil {
		t.Fatalf("replace failed: %v", err)
	}
	if !utils.IsSymlinkOrJunction(dir) {
		t.Error("expected a link after the replacement")
	}
}

func TestReplaceWithLink_KeepsFolderWhenLinkFails(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "alpha")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("mine"), 0644)

	err := replaceWithLink(dir, func() error { return os.ErrPermission })
	if err == nil {
		t.Fatal("expected the link error")
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "SKILL.md")); string(got) != "mine" {
		t.Errorf("folder not restored after a failed link: %q", got)
	}
	if entries, _ := os.ReadDir(filepath.Dir(dir)); len(entries) != 1 {
		t.Errorf("parked copy left behind: %v", entries)
	}
}

func TestSyncTargetMerge_ReplacesCopyModeCopy_DryRun(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha")
	target := config.TargetConfig{Path: tgt, Mode: "merge"}

	copyDir := setupCopyModeCopy(t, src, tgt, "alpha")

	result, err := SyncTargetMerge("test", target, src, true, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Updated) != 1 {
		t.Errorf("expected dry-run to report the copy as updated, got %v", result.Updated)
	}
	if utils.IsSymlinkOrJunction(copyDir) {
		t.Error("dry-run must leave the copy in place")
	}
}

// A copy edited since copy mode made it is the user's work now; only --force replaces it.
func TestSyncTargetMerge_KeepsEditedCopyModeCopy(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha")
	target := config.TargetConfig{Path: tgt, Mode: "merge"}

	copyDir := setupCopyModeCopy(t, src, tgt, "alpha")
	if err := os.WriteFile(filepath.Join(copyDir, "SKILL.md"), []byte("edited"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := SyncTargetMerge("test", target, src, false, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Skipped) != 1 || len(result.Updated) != 0 {
		t.Errorf("expected edited copy kept, got skipped=%v updated=%v", result.Skipped, result.Updated)
	}
	if got, _ := os.ReadFile(filepath.Join(copyDir, "SKILL.md")); string(got) != "edited" {
		t.Errorf("edited copy was changed: %q", got)
	}
}

// A folder the manifest records as a link is not a copy-mode copy; it is the user's.
func TestSyncTargetMerge_KeepsFolderRecordedAsLink(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha")
	target := config.TargetConfig{Path: tgt, Mode: "merge"}

	os.MkdirAll(filepath.Join(tgt, "alpha"), 0755)
	if err := WriteManifest(tgt, &Manifest{Managed: map[string]string{"alpha": "symlink"}}); err != nil {
		t.Fatal(err)
	}

	result, err := SyncTargetMerge("test", target, src, false, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Skipped) != 1 || len(result.Updated) != 0 {
		t.Errorf("expected folder kept, got skipped=%v updated=%v", result.Skipped, result.Updated)
	}
}

func TestSyncTargetMerge_DryRun(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha")
	target := config.TargetConfig{Path: tgt, Mode: "merge"}

	result, err := SyncTargetMerge("test", target, src, true, false, "")
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

	result, err := SyncTargetMerge("test", target, src, false, false, "")
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
	result, err := SyncTargetMerge("test", target, src, false, false, "")
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
	if _, err := SyncTargetMerge("test", target, src, false, false, ""); err != nil {
		t.Fatal(err)
	}

	// Add an orphan symlink pointing into source (simulates deleted skill)
	orphanSrc := filepath.Join(src, "deleted-skill")
	os.MkdirAll(orphanSrc, 0755)
	os.Symlink(orphanSrc, filepath.Join(tgt, "deleted-skill"))
	os.RemoveAll(orphanSrc) // Now it's a broken link to source

	result, err := PruneOrphanLinks(tgt, src, nil, nil, "test", "", false, false)
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

	if _, err := SyncTargetMerge("test", target, src, false, false, ""); err != nil {
		t.Fatal(err)
	}

	// Add a local (non-symlink) directory
	os.MkdirAll(filepath.Join(tgt, "my-local-skill"), 0755)

	result, err := PruneOrphanLinks(tgt, src, nil, nil, "test", "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	// Local dir should be kept and recorded in LocalDirs (not Warnings)
	if len(result.Removed) != 0 {
		t.Errorf("expected 0 removed (local kept), got %d: %v", len(result.Removed), result.Removed)
	}
	if len(result.LocalDirs) != 1 {
		t.Errorf("expected 1 local dir, got %d", len(result.LocalDirs))
	}
	if len(result.Warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d: %v", len(result.Warnings), result.Warnings)
	}
}

func TestPruneOrphanLinks_RemovesExcluded(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha", "beta")
	target := config.TargetConfig{Path: tgt, Mode: "merge"}

	// Sync both skills
	if _, err := SyncTargetMerge("test", target, src, false, false, ""); err != nil {
		t.Fatal(err)
	}

	// Now prune with beta excluded
	result, err := PruneOrphanLinks(tgt, src, nil, []string{"beta"}, "test", "", false, false)
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

	// Excluded skill should also be removed from manifest.
	m, err := ReadManifest(tgt)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m.Managed["beta"]; ok {
		t.Error("expected beta to be removed from manifest after exclude prune")
	}
	if _, ok := m.Managed["alpha"]; !ok {
		t.Error("expected alpha to remain in manifest")
	}
}

func TestPruneOrphanLinks_KeepsExternal(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha")
	target := config.TargetConfig{Path: tgt, Mode: "merge"}

	if _, err := SyncTargetMerge("test", target, src, false, false, ""); err != nil {
		t.Fatal(err)
	}

	// Add an external symlink (points outside source)
	externalDir := filepath.Join(t.TempDir(), "external-skill")
	os.MkdirAll(externalDir, 0755)
	os.Symlink(externalDir, filepath.Join(tgt, "ext-skill"))

	result, err := PruneOrphanLinks(tgt, src, nil, nil, "test", "", false, false)
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

// A live link into the source that skillshare never created belongs to the
// user; pruning it by name destroys configuration it does not own.
func TestPruneOrphanLinks_KeepsUnmanagedLiveLinkToSource(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha", "handmade")
	target := config.TargetConfig{Path: tgt, Mode: "merge", Include: []string{"alpha"}}

	// skillshare manages only alpha, so "handmade" is never linked or tracked.
	if _, err := SyncTargetMerge("test", target, src, false, false, ""); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(filepath.Join(src, "handmade"), filepath.Join(tgt, "handmade")); err != nil {
		t.Fatal(err)
	}

	result, err := PruneOrphanLinks(tgt, src, []string{"alpha"}, nil, "test", "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(tgt, "handmade")); err != nil {
		t.Error("expected unmanaged live link to source to be kept")
	}
	if len(result.Removed) != 0 {
		t.Errorf("expected 0 removed, got %d: %v", len(result.Removed), result.Removed)
	}
	if len(result.LocalDirs) != 1 {
		t.Errorf("expected 1 local entry reported, got %d: %v", len(result.LocalDirs), result.LocalDirs)
	}

	// --force still reclaims the name, so the escape hatch survives the guard.
	result, err = PruneOrphanLinks(tgt, src, []string{"alpha"}, nil, "test", "", false, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Removed) != 1 || result.Removed[0] != "handmade" {
		t.Errorf("expected force to prune handmade, got %v", result.Removed)
	}
}

func TestPruneOrphanLinks_ExcludedManagedDir_Removed(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha", "beta")
	target := config.TargetConfig{Path: tgt, Mode: "merge"}

	// Sync both skills → manifest has both
	if _, err := SyncTargetMerge("test", target, src, false, false, ""); err != nil {
		t.Fatal(err)
	}

	// Replace beta symlink with real directory (simulate copy-mode residue)
	os.Remove(filepath.Join(tgt, "beta"))
	os.MkdirAll(filepath.Join(tgt, "beta"), 0755)
	os.WriteFile(filepath.Join(tgt, "beta", "SKILL.md"), []byte("# residue"), 0644)

	// Prune with beta excluded — real directory should be removed via manifest
	result, err := PruneOrphanLinks(tgt, src, nil, []string{"beta"}, "test", "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Removed) != 1 || result.Removed[0] != "beta" {
		t.Errorf("expected beta removed, got %v", result.Removed)
	}
	if _, err := os.Lstat(filepath.Join(tgt, "beta")); !os.IsNotExist(err) {
		t.Error("expected beta directory to be removed")
	}

	// Manifest should not contain beta
	m, _ := ReadManifest(tgt)
	if _, ok := m.Managed["beta"]; ok {
		t.Error("expected beta removed from manifest")
	}
	if _, ok := m.Managed["alpha"]; !ok {
		t.Error("expected alpha to remain in manifest")
	}
}

func TestSyncTargetMerge_WritesManifest(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha", "beta")
	target := config.TargetConfig{Path: tgt, Mode: "merge"}

	_, err := SyncTargetMerge("test", target, src, false, false, "")
	if err != nil {
		t.Fatal(err)
	}

	// Manifest should exist and contain the linked skills
	m, err := ReadManifest(tgt)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Managed) != 2 {
		t.Errorf("expected 2 managed entries, got %d: %v", len(m.Managed), m.Managed)
	}
	for _, name := range []string{"alpha", "beta"} {
		if v, ok := m.Managed[name]; !ok {
			t.Errorf("expected %s in manifest", name)
		} else if v != "symlink" {
			t.Errorf("expected manifest value 'symlink' for %s, got %q", name, v)
		}
	}
}

func TestSyncTargetMerge_DryRun_NoManifest(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha")
	target := config.TargetConfig{Path: tgt, Mode: "merge"}

	_, err := SyncTargetMerge("test", target, src, true, false, "")
	if err != nil {
		t.Fatal(err)
	}

	// Manifest should NOT exist after dry-run
	p := filepath.Join(tgt, ManifestFile)
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Error("manifest should not be written in dry-run mode")
	}
}

func TestPruneOrphanLinks_ManifestTrackedDir_Removed(t *testing.T) {
	src, tgt := setupMergeTest(t, "my-skill")
	target := config.TargetConfig{Path: tgt, Mode: "merge"}

	// Sync to create symlink + manifest
	if _, err := SyncTargetMerge("test", target, src, false, false, ""); err != nil {
		t.Fatal(err)
	}

	// Replace symlink with a real directory (simulates copy mode residue)
	os.Remove(filepath.Join(tgt, "my-skill"))
	os.MkdirAll(filepath.Join(tgt, "my-skill"), 0755)
	os.WriteFile(filepath.Join(tgt, "my-skill", "SKILL.md"), []byte("# Copy"), 0644)

	// Remove source skill (simulates uninstall)
	os.RemoveAll(filepath.Join(src, "my-skill"))

	// Prune should remove the directory because it's in the manifest
	result, err := PruneOrphanLinks(tgt, src, nil, nil, "test", "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Removed) != 1 {
		t.Errorf("expected 1 removed (manifest-tracked dir), got %d: %v", len(result.Removed), result.Removed)
	}
	if _, err := os.Stat(filepath.Join(tgt, "my-skill")); !os.IsNotExist(err) {
		t.Error("manifest-tracked orphan directory should have been removed")
	}
}

func TestPruneOrphanLinks_NoManifest_KeepsUnknownDir(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha")

	// Do NOT sync (no manifest exists)
	// Just create a real directory in target
	os.MkdirAll(filepath.Join(tgt, "unknown-skill"), 0755)

	result, err := PruneOrphanLinks(tgt, src, nil, nil, "test", "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	// Without manifest, unknown directory should be kept and recorded as local
	if len(result.Removed) != 0 {
		t.Errorf("expected 0 removed (no manifest, local dir kept), got %d", len(result.Removed))
	}
	if len(result.LocalDirs) != 1 {
		t.Errorf("expected 1 local dir, got %d", len(result.LocalDirs))
	}
	if len(result.Warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d: %v", len(result.Warnings), result.Warnings)
	}
}

func TestPruneOrphanLinks_ManifestCleanedAfterPrune(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha", "beta")
	target := config.TargetConfig{Path: tgt, Mode: "merge"}

	// Sync to create links + manifest with alpha and beta
	if _, err := SyncTargetMerge("test", target, src, false, false, ""); err != nil {
		t.Fatal(err)
	}

	// Remove beta from source
	os.RemoveAll(filepath.Join(src, "beta"))

	// Prune — beta symlink should be removed and manifest updated
	_, err := PruneOrphanLinks(tgt, src, nil, nil, "test", "", false, false)
	if err != nil {
		t.Fatal(err)
	}

	// Manifest should no longer contain beta
	m, err := ReadManifest(tgt)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m.Managed["beta"]; ok {
		t.Error("beta should have been removed from manifest after prune")
	}
	if _, ok := m.Managed["alpha"]; !ok {
		t.Error("alpha should still be in manifest")
	}
}

func TestPruneOrphanLinks_HiddenManagedEntryPruned(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha", ".system/example")
	target := config.TargetConfig{Path: tgt, Mode: "merge"}

	if _, err := SyncTargetMerge("test", target, src, false, false, ""); err != nil {
		t.Fatal(err)
	}
	os.RemoveAll(filepath.Join(src, ".system"))

	if _, err := PruneOrphanLinks(tgt, src, nil, nil, "test", "", false, false); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Lstat(filepath.Join(tgt, ".system__example")); !os.IsNotExist(err) {
		t.Error(".system__example should have been pruned from target")
	}
	m, err := ReadManifest(tgt)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m.Managed[".system__example"]; ok {
		t.Error(".system__example should have been removed from manifest after prune")
	}
}

func TestPruneOrphanLinks_NonExistentTarget(t *testing.T) {
	result, err := PruneOrphanLinks("/nonexistent", "/nonexistent/src", nil, nil, "test", "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Removed) != 0 {
		t.Errorf("expected 0 removed for non-existent target, got %d", len(result.Removed))
	}
}
