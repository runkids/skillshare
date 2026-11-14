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
