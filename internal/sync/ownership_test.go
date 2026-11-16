package sync

// Regression tests: sync and prune must not delete or re-point entries that
// skillshare did not create. Links are made with createLink, so on Windows
// without Developer Mode they are junctions and these tests still run.

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/utils"
)

func dirLink(t *testing.T, link, target string) {
	t.Helper()
	if err := createLink(link, target, false); err != nil {
		t.Fatalf("create link %s -> %s: %v", link, target, err)
	}
}

func recordInManifest(t *testing.T, targetPath, name string) {
	t.Helper()
	m, err := ReadManifest(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	m.Managed[name] = manifestSymlink
	if err := WriteManifest(targetPath, m); err != nil {
		t.Fatal(err)
	}
}

func TestPruneKeepsUnmanagedFoldersWithSkillshareLikeNames(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha")
	names := []string{"team__shared", "_drafts"}
	for _, name := range names {
		writeSkillMD(t, filepath.Join(tgt, name), "---\nname: "+name+"\n---\nmine")
	}

	res, err := PruneOrphanLinks(tgt, src, nil, nil, "test", "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Removed) != 0 {
		t.Errorf("removed %v; folders skillshare did not create must be kept", res.Removed)
	}
	for _, name := range names {
		if _, err := os.Stat(filepath.Join(tgt, name, "SKILL.md")); err != nil {
			t.Errorf("%s was deleted: %v", name, err)
		}
		if !slices.Contains(res.LocalDirs, name) {
			t.Errorf("%s not reported as a local entry: %v", name, res.LocalDirs)
		}
	}
	if len(res.Warnings) != len(names) {
		t.Errorf("want one warning per kept look-alike, got %v", res.Warnings)
	}
}

func TestPruneForceStillRemovesSkillshareLikeNames(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha")
	writeSkillMD(t, filepath.Join(tgt, "team__old"), "---\nname: old\n---\nold")

	res, err := PruneOrphanLinks(tgt, src, nil, nil, "test", "", false, true)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(res.Removed, "team__old") {
		t.Errorf("--force should remove team__old, removed %v", res.Removed)
	}
}

func TestPruneKeepsBrokenLinkItDidNotCreate(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha")
	gone := filepath.Join(t.TempDir(), "gone")
	if err := os.MkdirAll(gone, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(tgt, "broken-link")
	dirLink(t, link, gone)
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}

	res, err := PruneOrphanLinks(tgt, src, nil, nil, "test", "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(link); err != nil {
		t.Fatalf("broken link skillshare did not create was removed: %v", err)
	}
	if len(res.Removed) != 0 || len(res.Warnings) != 1 {
		t.Errorf("want kept with one warning, got removed=%v warnings=%v", res.Removed, res.Warnings)
	}
}

func TestPruneRemovesBrokenLinkItCreated(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha")
	oldSource := filepath.Join(t.TempDir(), "old-source", "moved")
	if err := os.MkdirAll(oldSource, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(tgt, "moved")
	dirLink(t, link, oldSource)
	recordInManifest(t, tgt, "moved")
	if err := os.RemoveAll(oldSource); err != nil {
		t.Fatal(err)
	}

	res, err := PruneOrphanLinks(tgt, src, nil, nil, "test", "", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(res.Removed, "moved") {
		t.Errorf("a broken link in the manifest should be removed, removed %v", res.Removed)
	}
}

func TestSyncMergeKeepsSameNameLinkFromAnotherTool(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha")
	other := filepath.Join(t.TempDir(), "other-store", "alpha")
	writeSkillMD(t, other, "---\nname: alpha\n---\nanother manager's copy")
	link := filepath.Join(tgt, "alpha")
	dirLink(t, link, other)

	res, err := SyncTargetMerge("test", config.TargetConfig{Path: tgt, Mode: "merge"}, src, false, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(res.Skipped, "alpha") || slices.Contains(res.Updated, "alpha") {
		t.Errorf("want alpha skipped, got skipped=%v updated=%v", res.Skipped, res.Updated)
	}
	if got, err := utils.ResolveLinkTarget(link); err != nil || !utils.PathsEqual(got, other) {
		t.Errorf("link was re-pointed: %q (%v), want %q", got, err, other)
	}
	m, _ := ReadManifest(tgt)
	if _, ok := m.Managed["alpha"]; ok {
		t.Error("a link skillshare kept must not be recorded as its own")
	}
}

func TestSyncMergeForceReplacesSameNameLinkFromAnotherTool(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha")
	other := filepath.Join(t.TempDir(), "other-store", "alpha")
	writeSkillMD(t, other, "---\nname: alpha\n---\nanother manager's copy")
	dirLink(t, filepath.Join(tgt, "alpha"), other)

	res, err := SyncTargetMerge("test", config.TargetConfig{Path: tgt, Mode: "merge"}, src, false, true, "")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(res.Updated, "alpha") {
		t.Errorf("--force should re-point alpha, updated=%v", res.Updated)
	}
	if _, err := os.Stat(filepath.Join(other, "SKILL.md")); err != nil {
		t.Errorf("the other tool's files must survive a re-point: %v", err)
	}
}

// A link into the source is skillshare's even when the manifest is gone and
// the source is reached through an alias, so it is still repaired.
func TestSyncMergeRepointsUnrecordedLinkIntoAliasedSource(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha")
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(src, alias); err != nil {
		t.Skipf("directory symlinks unavailable: %v", err)
	}
	// The link is made while its destination exists: Windows cannot make a
	// junction to a missing folder.
	renamed := filepath.Join(src, "renamed-away")
	if err := os.MkdirAll(renamed, 0o755); err != nil {
		t.Fatal(err)
	}
	dirLink(t, filepath.Join(tgt, "alpha"), renamed)
	if err := os.RemoveAll(renamed); err != nil {
		t.Fatal(err)
	}

	res, err := SyncTargetMerge("test", config.TargetConfig{Path: tgt, Mode: "merge"}, alias, false, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(res.Updated, "alpha") {
		t.Errorf("want alpha re-pointed, got updated=%v skipped=%v", res.Updated, res.Skipped)
	}
}

// Project mode writes relative links. One that predates the manifest and now
// points at a skill that moved inside the source is still skillshare's.
func TestSyncMergeRepointsUnrecordedRelativeLinkIntoSource(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, ".skillshare", "skills")
	tgt := filepath.Join(root, ".claude", "skills")
	writeSkillMD(t, filepath.Join(src, "alpha"), "---\nname: alpha\n---\n# alpha")
	if err := os.MkdirAll(tgt, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(tgt, "alpha")
	if err := os.Symlink(filepath.Join("..", "..", ".skillshare", "skills", "old-alpha"), link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	res, err := SyncTargetMerge("test", config.TargetConfig{Path: tgt, Mode: "merge"}, src, false, false, root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(res.Updated, "alpha") {
		t.Fatalf("want alpha re-pointed, got updated=%v skipped=%v", res.Updated, res.Skipped)
	}
	if _, err := os.Stat(filepath.Join(link, "SKILL.md")); err != nil {
		t.Errorf("alpha does not resolve to the source skill: %v", err)
	}
}

func TestSyncCopyKeepsSameNameLinkFromAnotherTool(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha")
	other := filepath.Join(t.TempDir(), "other-store", "alpha")
	writeSkillMD(t, other, "---\nname: alpha\n---\nanother manager's copy")
	link := filepath.Join(tgt, "alpha")
	dirLink(t, link, other)

	res, err := SyncTargetCopy("test", config.TargetConfig{Path: tgt, Mode: "copy"}, src, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(res.KeptLocal, "alpha") {
		t.Errorf("want alpha kept local, got %+v", res)
	}
	if got, err := utils.ResolveLinkTarget(link); err != nil || !utils.PathsEqual(got, other) {
		t.Errorf("link was replaced: %q (%v), want a link to %q", got, err, other)
	}
}

func TestSyncCopyForceReplacesSameNameLinkFromAnotherTool(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha")
	other := filepath.Join(t.TempDir(), "other-store", "alpha")
	writeSkillMD(t, other, "---\nname: alpha\n---\nanother manager's copy")
	link := filepath.Join(tgt, "alpha")
	dirLink(t, link, other)

	if _, err := SyncTargetCopy("test", config.TargetConfig{Path: tgt, Mode: "copy"}, src, false, true); err != nil {
		t.Fatal(err)
	}
	if utils.IsSymlinkOrJunction(link) {
		t.Error("--force should replace the link with a copy")
	}
	if _, err := os.Stat(filepath.Join(other, "SKILL.md")); err != nil {
		t.Errorf("the other tool's files must survive: %v", err)
	}
}

// ManagedOnly cleans directories that belong to another tool: the manifest is
// the only proof an external broken link is skillshare's.
func TestPruneManagedOnlyRemovesOnlyRecordedBrokenLinks(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha")
	gone := filepath.Join(t.TempDir(), "gone")
	for _, name := range []string{"ours", "theirs"} {
		if err := os.MkdirAll(filepath.Join(gone, name), 0o755); err != nil {
			t.Fatal(err)
		}
		dirLink(t, filepath.Join(tgt, name), filepath.Join(gone, name))
	}
	recordInManifest(t, tgt, "ours")
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}

	res, err := PruneOrphanLinksWithSkills(PruneOptions{TargetPath: tgt, SourcePath: src, TargetName: "test", ManagedOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(res.Removed, []string{"ours"}) {
		t.Errorf("removed %v, want only the recorded link", res.Removed)
	}
	if _, err := os.Lstat(filepath.Join(tgt, "theirs")); err != nil {
		t.Errorf("unrecorded broken link was removed: %v", err)
	}
}

// Only flat naming produces names like team__x, so under standard naming such
// a folder is an ordinary local folder: kept, and not worth a warning.
func TestPruneStandardNamingKeepsLookAlikeFolderQuietly(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha")
	writeSkillMD(t, filepath.Join(tgt, "team__shared"), "---\nname: team__shared\n---\nmine")

	res, err := PruneOrphanLinks(tgt, src, nil, nil, "test", "standard", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Removed) != 0 || len(res.Warnings) != 0 || !slices.Contains(res.LocalDirs, "team__shared") {
		t.Errorf("want kept as local without a warning, got %+v", res)
	}
}

func TestPruneDryRunForceReportsButKeepsUnownedEntries(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha")
	writeSkillMD(t, filepath.Join(tgt, "team__old"), "---\nname: old\n---\nold")
	gone := filepath.Join(t.TempDir(), "gone")
	if err := os.MkdirAll(gone, 0o755); err != nil {
		t.Fatal(err)
	}
	dirLink(t, filepath.Join(tgt, "broken-link"), gone)
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}

	res, err := PruneOrphanLinks(tgt, src, nil, nil, "test", "", true, true)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(res.Removed)
	if !slices.Equal(res.Removed, []string{"broken-link", "team__old"}) {
		t.Errorf("dry run should report both, got %v", res.Removed)
	}
	for _, name := range []string{"broken-link", "team__old"} {
		if _, err := os.Lstat(filepath.Join(tgt, name)); err != nil {
			t.Errorf("dry run removed %s: %v", name, err)
		}
	}
}

// The manifest still records the copy that copy mode made, but the user has
// since replaced it with a link of their own. That link is not skillshare's.
func TestSyncCopyKeepsLinkThatReplacedARecordedCopy(t *testing.T) {
	src, tgt := setupMergeTest(t, "alpha")
	target := config.TargetConfig{Path: tgt, Mode: "copy"}
	if _, err := SyncTargetCopy("test", target, src, false, false); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(t.TempDir(), "other-store", "alpha")
	writeSkillMD(t, other, "---\nname: alpha\n---\nanother manager's copy")
	link := filepath.Join(tgt, "alpha")
	if err := os.RemoveAll(link); err != nil {
		t.Fatal(err)
	}
	dirLink(t, link, other)

	if _, err := SyncTargetCopy("test", target, src, false, false); err != nil {
		t.Fatal(err)
	}
	if got, err := utils.ResolveLinkTarget(link); err != nil || !utils.PathsEqual(got, other) {
		t.Errorf("link was replaced: %q (%v), want a link to %q", got, err, other)
	}
}
