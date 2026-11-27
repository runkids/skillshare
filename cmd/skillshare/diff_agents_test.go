package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/resource"
	"skillshare/internal/sync"
)

func TestComputeAgentDiff_ExtensionOutputInSync(t *testing.T) {
	srcDir, targetDir := t.TempDir(), t.TempDir()
	srcFile := filepath.Join(srcDir, "reviewer.md")
	os.WriteFile(srcFile, []byte("# Reviewer"), 0644)
	agents := []resource.DiscoveredResource{{FlatName: "reviewer.md", AbsPath: srcFile, RelPath: "reviewer.md"}}
	spec := &sync.ExtensionSpec{Run: []string{"cat"}, Dir: srcDir, Name: "id", OutputExt: "toml"}
	if _, err := sync.SyncAgentsTransform(agents, srcDir, targetDir, "copy", spec, false, false); err != nil {
		t.Fatal(err)
	}

	r := computeAgentDiff("codex", targetDir, srcDir, config.ResourceTargetConfig{Extension: "x"}, agents)

	if !r.synced || len(r.items) != 0 {
		t.Fatalf("expected converted output in sync, got %+v", r.items)
	}
}

func TestComputeAgentDiff_ExcludedAgentIsNotPending(t *testing.T) {
	agents := []resource.DiscoveredResource{{FlatName: "helper.md", AbsPath: "/src/helper.md", RelPath: "helper.md"}}

	r := computeAgentDiff("claude", t.TempDir(), t.TempDir(), config.ResourceTargetConfig{Exclude: []string{"helper*"}}, agents)

	if !r.synced || len(r.items) != 0 {
		t.Fatalf("expected excluded agent to be skipped, got %+v", r.items)
	}
}

// The diff lists an orphan agent link only when sync would prune it: a broken
// link, or one into the source. A live link to another location stays.
func TestComputeAgentDiff_ListsOnlyOrphanLinksSyncPrunes(t *testing.T) {
	srcDir, targetDir, other := t.TempDir(), t.TempDir(), t.TempDir()
	for _, file := range []string{filepath.Join(srcDir, "stale.md"), filepath.Join(other, "theirs.md")} {
		if err := os.WriteFile(file, []byte("# Agent"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	links := map[string]string{
		"theirs.md": filepath.Join(other, "theirs.md"),
		"broken.md": filepath.Join(other, "gone.md"),
	}
	for name, dest := range links {
		if err := os.Symlink(dest, filepath.Join(targetDir, name)); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}
	// A project target links into the source with a relative path.
	rel, err := filepath.Rel(targetDir, filepath.Join(srcDir, "stale.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(rel, filepath.Join(targetDir, "stale.md")); err != nil {
		t.Fatal(err)
	}

	merge := computeAgentDiff("claude", targetDir, srcDir, config.ResourceTargetConfig{}, nil)
	copyMode := computeAgentDiff("claude", targetDir, srcDir, config.ResourceTargetConfig{Mode: "copy"}, nil)

	var listed []string
	for _, item := range merge.items {
		listed = append(listed, item.action+" "+item.name)
	}
	slices.Sort(listed)
	if !slices.Equal(listed, []string{"remove broken.md", "remove stale.md"}) {
		t.Errorf("merge listed %v, want only the two links sync prunes", listed)
	}
	if len(copyMode.items) != 0 {
		t.Errorf("copy mode prunes no links, but the diff listed %+v", copyMode.items)
	}
}
