package sync

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"skillshare/internal/resource"
)

// withoutFileLinks simulates Windows without Developer Mode.
func withoutFileLinks(t *testing.T) {
	t.Helper()
	t.Cleanup(SetFileLinksForTest(false))
}

func isRegularFile(t *testing.T, path string) bool {
	t.Helper()
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular()
}

func agentFixture(t *testing.T) (src, tgt string, agents []resource.DiscoveredResource) {
	t.Helper()
	src, tgt = t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(src, "tutor.md"), []byte("v1"), 0644)
	agents = []resource.DiscoveredResource{{FlatName: "tutor.md", AbsPath: filepath.Join(src, "tutor.md")}}
	return src, tgt, agents
}

func TestSyncAgents_MergeCopiesAndUpdatesWithoutFileLinks(t *testing.T) {
	withoutFileLinks(t)
	src, tgt, agents := agentFixture(t)
	target := filepath.Join(tgt, "tutor.md")

	if _, err := SyncAgents(agents, src, tgt, "merge", false, false); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(agents[0].AbsPath, []byte("v2"), 0644)
	res, err := SyncAgents(agents, src, tgt, "merge", false, false)
	if err != nil {
		t.Fatal(err)
	}

	if !isRegularFile(t, target) || readFile(t, target) != "v2" || len(res.Updated) != 1 {
		t.Errorf("target = %q regular=%v, result = %+v; want an updated copy", readFile(t, target), isRegularFile(t, target), res)
	}
}

func TestSyncAgents_MergeWithoutFileLinksReplacesLinkWithCopy(t *testing.T) {
	withoutFileLinks(t)
	src, tgt, agents := agentFixture(t)
	target := filepath.Join(tgt, "tutor.md")
	os.Symlink(agents[0].AbsPath, target)

	if _, err := SyncAgents(agents, src, tgt, "merge", false, false); err != nil {
		t.Fatal(err)
	}

	if !isRegularFile(t, target) || readFile(t, agents[0].AbsPath) != "v1" {
		t.Error("link to the source file was not replaced by a copy")
	}
}

func TestSyncAgents_MergeWithoutFileLinksKeepsLocalAgents(t *testing.T) {
	withoutFileLinks(t)
	src, tgt, agents := agentFixture(t)
	os.WriteFile(filepath.Join(tgt, "tutor.md"), []byte("mine"), 0644)
	os.WriteFile(filepath.Join(tgt, "local.md"), []byte("local"), 0644)

	res, err := SyncAgents(agents, src, tgt, "merge", false, false)
	if err != nil {
		t.Fatal(err)
	}
	PruneOrphanAgentLinks(tgt, src, agents, false)

	if readFile(t, filepath.Join(tgt, "tutor.md")) != "mine" || readFile(t, filepath.Join(tgt, "local.md")) != "local" || len(res.Skipped) != 1 {
		t.Errorf("local agents changed; result = %+v", res)
	}
}

func TestPruneOrphanAgentLinks_RemovesTrackedCopies(t *testing.T) {
	withoutFileLinks(t)
	src, tgt, agents := agentFixture(t)
	SyncAgents(agents, src, tgt, "merge", false, false)

	removed, _ := PruneOrphanAgentLinks(tgt, src, nil, false)

	if _, err := os.Stat(filepath.Join(tgt, "tutor.md")); len(removed) != 1 || !os.IsNotExist(err) {
		t.Errorf("removed = %v, want the orphaned copy pruned", removed)
	}
}

func TestSyncAgents_MergeRelinksCopiesOnceFileLinksWork(t *testing.T) {
	src, tgt, agents := agentFixture(t)
	restore := SetFileLinksForTest(false)
	SyncAgents(agents, src, tgt, "merge", false, false)
	restore()

	if _, err := SyncAgents(agents, src, tgt, "merge", false, false); err != nil {
		t.Fatal(err)
	}

	if isRegularFile(t, filepath.Join(tgt, "tutor.md")) {
		t.Error("copy was not relinked after file links became available")
	}
}

func TestSyncExtraFile_LinkModesCopyWithoutFileLinks(t *testing.T) {
	withoutFileLinks(t)
	for _, mode := range []string{"merge", "symlink"} {
		t.Run(mode, func(t *testing.T) {
			src, tgt := setupExtraFileTest(t, "# agents")
			f := NewExtraFile(src, "AGENTS.md", tgt, "", mode)

			res, err := SyncExtraFile(f, false, "")
			if err != nil {
				t.Fatal(err)
			}

			target := filepath.Join(tgt, "AGENTS.md")
			if f.Mode != "copy" || !isRegularFile(t, target) || readFile(t, target) != "# agents" || len(res.Warnings) != 1 {
				t.Errorf("mode = %q, regular = %v, warnings = %v; want a copy", f.Mode, isRegularFile(t, target), res.Warnings)
			}
		})
	}
}

func TestSyncExtraFile_WithoutFileLinksReplacesLinkWithCopy(t *testing.T) {
	withoutFileLinks(t)
	src, tgt := setupExtraFileTest(t, "# agents")
	target := filepath.Join(tgt, "AGENTS.md")
	os.Symlink(filepath.Join(src, "AGENTS.md"), target)
	f := NewExtraFile(src, "AGENTS.md", tgt, "", "merge")

	if _, err := SyncExtraFile(f, false, ""); err != nil {
		t.Fatal(err)
	}

	if !isRegularFile(t, target) || ExtraFileStatus(f) != "synced" {
		t.Errorf("regular = %v, status = %q; want a synced copy", isRegularFile(t, target), ExtraFileStatus(f))
	}
}

func TestSyncExtra_MergeCopiesUpdatesAndPrunesWithoutFileLinks(t *testing.T) {
	withoutFileLinks(t)
	src, tgt := setupExtrasTest(t, map[string]string{"a.md": "v1", "b.md": "b"})
	if _, err := SyncExtra(src, tgt, "merge", false, false, false, "", nil); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(src, "a.md"), []byte("v2"), 0644)
	os.Remove(filepath.Join(src, "b.md"))

	res, err := SyncExtra(src, tgt, "merge", false, false, false, "", nil)
	if err != nil {
		t.Fatal(err)
	}

	_, bErr := os.Stat(filepath.Join(tgt, "b.md"))
	if readFile(t, filepath.Join(tgt, "a.md")) != "v2" || !os.IsNotExist(bErr) || res.Pruned != 1 {
		t.Errorf("a.md = %q, b.md err = %v, result = %+v; want updated and pruned copies", readFile(t, filepath.Join(tgt, "a.md")), bErr, res)
	}
}

func TestRestoreExtraTarget_WithoutFileLinksPutsBackUserFile(t *testing.T) {
	withoutFileLinks(t)
	src, tgt := setupExtraFileTest(t, "# agents")
	target := filepath.Join(tgt, "AGENTS.md")
	os.WriteFile(target, []byte("mine"), 0644)
	f := NewExtraFile(src, "AGENTS.md", tgt, "", "merge")
	SyncExtraFile(f, false, "")

	if _, err := RestoreExtraTarget(f); err != nil {
		t.Fatal(err)
	}

	if got := readFile(t, target); got != "mine" {
		t.Errorf("restored %q, want the pre-attach file", got)
	}
}

func TestSyncExtraFile_CopyReplacesOwnStaleCopySilently(t *testing.T) {
	for _, mode := range []string{"copy", "merge"} { // merge copies without file links
		t.Run(mode, func(t *testing.T) {
			withoutFileLinks(t)
			src, tgt := setupExtraFileTest(t, "v1")
			target := filepath.Join(tgt, "AGENTS.md")
			f := NewExtraFile(src, "AGENTS.md", tgt, "", mode)
			SyncExtraFile(f, false, "")
			os.WriteFile(filepath.Join(src, "AGENTS.md"), []byte("v2"), 0644)

			res, err := SyncExtraFile(f, false, "")
			if err != nil {
				t.Fatal(err)
			}

			for _, w := range res.Warnings {
				if strings.Contains(w, "backed up") {
					t.Errorf("unexpected warning %q", w)
				}
			}
			if readFile(t, target) != "v2" || len(driftBackups(t, target)) != 0 {
				t.Errorf("target = %q, drift = %v; want v2 and no drift backup", readFile(t, target), driftBackups(t, target))
			}
		})
	}
}

func TestSyncExtraFile_CopyStillBacksUpUserEdit(t *testing.T) {
	src, tgt := setupExtraFileTest(t, "v1")
	target := filepath.Join(tgt, "AGENTS.md")
	f := NewExtraFile(src, "AGENTS.md", tgt, "", "copy")
	SyncExtraFile(f, false, "")
	os.WriteFile(target, []byte("edited"), 0644)

	if _, err := SyncExtraFile(f, false, ""); err != nil {
		t.Fatal(err)
	}

	if got := driftBackups(t, target); len(got) != 1 || got[0] != "edited" {
		t.Errorf("drift = %v, want the edit", got)
	}
}

func TestSyncedAgentCopies_CountsOnlyCurrentCopies(t *testing.T) {
	withoutFileLinks(t)
	src, tgt, agents := agentFixture(t)
	SyncAgents(agents, src, tgt, "merge", false, false)
	if n := SyncedAgentCopies(tgt, agents); n != 1 {
		t.Fatalf("after sync = %d, want 1", n)
	}

	os.WriteFile(agents[0].AbsPath, []byte("v2"), 0644)

	if n := SyncedAgentCopies(tgt, agents); n != 0 {
		t.Errorf("stale copy counted: %d, want 0", n)
	}
}

func TestSyncedExtensionOutputs_CountsTrackedRenamedOutputs(t *testing.T) {
	src, tgt, agents := agentFixture(t)
	spec := &ExtensionSpec{Run: []string{"cat"}, Dir: src, Name: "id", OutputExt: "toml"}
	if _, err := SyncAgentsTransform(agents, src, tgt, "copy", spec, false, false); err != nil {
		t.Fatal(err)
	}
	if n := SyncedAgentCopies(tgt, agents); n != 0 {
		t.Fatalf("source-name count = %d, want 0 (output is tutor.toml)", n)
	}
	if n := SyncedExtensionOutputs(tgt, agents); n != 1 {
		t.Fatalf("after sync = %d, want 1", n)
	}

	os.WriteFile(filepath.Join(tgt, "tutor.toml"), []byte("edited"), 0644)

	if n := SyncedExtensionOutputs(tgt, agents); n != 0 {
		t.Errorf("edited output counted: %d, want 0", n)
	}
}

func TestSyncedExtensionOutputs_OneOutputPerAgent(t *testing.T) {
	src, tgt, agents := agentFixture(t)
	spec := &ExtensionSpec{Run: []string{"cat"}, Dir: src, Name: "id", OutputExt: "toml"}
	if _, err := SyncAgentsTransform(agents, src, tgt, "copy", spec, false, false); err != nil {
		t.Fatal(err)
	}
	// A second agent flattening to the same name must not reuse the one output.
	twins := append(agents, agents[0])

	if n := SyncedExtensionOutputs(tgt, twins); n != 1 {
		t.Errorf("colliding agents = %d, want 1", n)
	}
}

func TestSyncedExtensionOutputs_ChangedSourceIsDrift(t *testing.T) {
	src, tgt, agents := agentFixture(t)
	spec := &ExtensionSpec{Run: []string{"cat"}, Dir: src, Name: "id", OutputExt: "toml"}
	if _, err := SyncAgentsTransform(agents, src, tgt, "copy", spec, false, false); err != nil {
		t.Fatal(err)
	}

	os.WriteFile(agents[0].AbsPath, []byte("v2"), 0644)

	if n := SyncedExtensionOutputs(tgt, agents); n != 0 {
		t.Fatalf("output of an old source counted: %d, want 0", n)
	}
	if _, err := SyncAgentsTransform(agents, src, tgt, "copy", spec, false, false); err != nil {
		t.Fatal(err)
	}
	if n := SyncedExtensionOutputs(tgt, agents); n != 1 {
		t.Errorf("after resync = %d, want 1", n)
	}
}

func TestSyncAgents_MergeWithoutFileLinksPreservesIdenticalLocalFile(t *testing.T) {
	withoutFileLinks(t)
	src, tgt, agents := agentFixture(t)
	p := filepath.Join(tgt, "tutor.md")
	os.WriteFile(p, []byte("v1"), 0644)
	if _, err := SyncAgents(agents, src, tgt, "merge", false, false); err != nil {
		t.Fatal(err)
	}
	if _, err := PruneOrphanAgentLinks(tgt, src, nil, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("pre-existing local file deleted: %v", err)
	}
}

func TestSyncExtra_MergeWithoutFileLinksPreservesIdenticalLocalFile(t *testing.T) {
	withoutFileLinks(t)
	src, tgt := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(src, "a.md"), []byte("local"), 0644)
	p := filepath.Join(tgt, "a.md")
	os.WriteFile(p, []byte("local"), 0644)
	if _, err := SyncExtra(src, tgt, "merge", false, false, false, "", nil); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(src, "a.md"))
	if _, err := SyncExtra(src, tgt, "merge", false, false, false, "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("pre-existing local file deleted: %v", err)
	}
}

// One failed removal must not stop the remaining orphans from being pruned.
func TestCopyTracker_PruneOrphansContinuesAfterRemoveFailure(t *testing.T) {
	skipUnlessPermissionsEnforced(t)
	dir := t.TempDir()
	locked := filepath.Join(dir, "locked")
	os.MkdirAll(locked, 0755)
	os.WriteFile(filepath.Join(locked, "a.md"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir, "b.md"), []byte("b"), 0644)
	tracker := loadCopyTracker(dir)
	tracker.record(filepath.Join("locked", "a.md"))
	tracker.record("b.md")
	os.Chmod(locked, 0555)
	t.Cleanup(func() { os.Chmod(locked, 0755) })

	removed, err := tracker.pruneOrphans(nil, false)
	if err == nil {
		t.Error("expected an error for the locked copy")
	}
	if len(removed) != 1 || removed[0] != "b.md" {
		t.Errorf("expected b.md pruned despite the failure, got %v", removed)
	}
}

func TestCopyTracker_PruneOrphansPreservesOwnershipOnRemoveFailure(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("requires a read-only proc file")
	}
	tracker := loadCopyTracker("/proc")
	tracker.record("version")
	if !tracker.owns("version") {
		t.Fatal("expected readable proc file")
	}
	tracker.changed = false
	removed, err := tracker.pruneOrphans(nil, false)
	if err == nil || !strings.Contains(err.Error(), "failed to remove orphaned copy version") {
		t.Fatalf("expected removal error, got %v", err)
	}
	if len(removed) != 0 || !tracker.owns("version") || tracker.changed {
		t.Fatalf("failed removal lost ownership or reported success: removed=%v changed=%v", removed, tracker.changed)
	}
}

func TestCopyFallbackMissingManifestIsSyncedButUnowned(t *testing.T) {
	withoutFileLinks(t)
	src, tgt, agents := agentFixture(t)
	SyncAgents(agents, src, tgt, "merge", false, false)
	os.Remove(filepath.Join(tgt, ".skillshare-manifest.json"))
	res, err := SyncAgents(agents, src, tgt, "merge", false, false)
	if err != nil || len(res.Linked) != 0 || len(res.Skipped) != 1 {
		t.Errorf("result=%+v err=%v", res, err)
	}
	if n := SyncedAgentCopies(tgt, agents); n != 1 {
		t.Errorf("synced=%d", n)
	}
	PruneOrphanAgentLinks(tgt, src, nil, false)
	if got := readFile(t, filepath.Join(tgt, "tutor.md")); got != "v1" {
		t.Fatal(got)
	}
	src, tgt = setupExtrasTest(t, map[string]string{"a.md": "a"})
	SyncExtra(src, tgt, "merge", false, false, false, "", nil)
	os.Remove(filepath.Join(tgt, ".skillshare-manifest.json"))
	extra, err := SyncExtra(src, tgt, "merge", false, false, false, "", nil)
	if err != nil || extra.Synced != 0 || extra.Skipped != 1 || extra.Preserved != 1 {
		t.Errorf("result=%+v err=%v", extra, err)
	}
	if status := CheckSyncStatus([]string{"a.md"}, src, tgt, "merge", false, ""); status != "synced" {
		t.Errorf("status=%q", status)
	}
	os.Remove(filepath.Join(src, "a.md"))
	SyncExtra(src, tgt, "merge", false, false, false, "", nil)
	if got := readFile(t, filepath.Join(tgt, "a.md")); got != "a" {
		t.Fatal(got)
	}
}
