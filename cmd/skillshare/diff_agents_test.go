package main

import (
	"os"
	"path/filepath"
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

	r := computeAgentDiff("codex", targetDir, config.ResourceTargetConfig{Extension: "x"}, agents)

	if !r.synced || len(r.items) != 0 {
		t.Fatalf("expected converted output in sync, got %+v", r.items)
	}
}

func TestComputeAgentDiff_ExcludedAgentIsNotPending(t *testing.T) {
	agents := []resource.DiscoveredResource{{FlatName: "helper.md", AbsPath: "/src/helper.md", RelPath: "helper.md"}}

	r := computeAgentDiff("claude", t.TempDir(), config.ResourceTargetConfig{Exclude: []string{"helper*"}}, agents)

	if !r.synced || len(r.items) != 0 {
		t.Fatalf("expected excluded agent to be skipped, got %+v", r.items)
	}
}
