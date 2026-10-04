package main

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/resource"
	"skillshare/internal/sync"
)

func TestCountLinkedAgents_CountsFallbackCopiesAsCopyMode(t *testing.T) {
	t.Cleanup(sync.SetFileLinksForTest(false))
	src, tgt := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(src, "tutor.md"), []byte("# Tutor"), 0644)
	agents := []resource.DiscoveredResource{{FlatName: "tutor.md", AbsPath: filepath.Join(src, "tutor.md")}}
	if _, err := sync.SyncAgents(agents, src, tgt, "merge", false, false); err != nil {
		t.Fatal(err)
	}

	mode, status := agentStatusLabel(config.ResourceTargetConfig{})

	if n := countLinkedAgents(config.ResourceTargetConfig{}, tgt, agents); n != 1 || mode != "copy" || status != "copied" {
		t.Errorf("linked = %d, mode = %q, status = %q; want 1 copy in sync", n, mode, status)
	}
}

func TestAgentStatusPreservedFallbackCopy(t *testing.T) {
	t.Cleanup(sync.SetFileLinksForTest(false))
	src, tgt := t.TempDir(), t.TempDir()
	path := filepath.Join(src, "tutor.md")
	os.WriteFile(path, []byte("same"), 0644)
	os.WriteFile(filepath.Join(tgt, "tutor.md"), []byte("same"), 0644)
	agents := []resource.DiscoveredResource{{FlatName: "tutor.md", AbsPath: path}}
	preserved := 0
	if n := countLinkedAgents(config.ResourceTargetConfig{}, tgt, agents, &preserved); n != 1 || preserved != 1 {
		t.Fatalf("current=%d preserved=%d", n, preserved)
	}
	if got := agentCountLabel(1, 1, preserved); got != "0/1 linked, 1 local preserved" {
		t.Fatal(got)
	}
}
