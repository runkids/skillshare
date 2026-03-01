package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandDiffCmd_ReturnsFilesForDstOnlyEntry(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "local.txt"), []byte("local"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	entry := copyDiffEntry{
		name:   "demo-skill",
		action: "remove",
		reason: "local only",
		dstDir: tmpDir,
	}

	msg := expandDiffCmd(&entry)()
	expanded, ok := msg.(diffExpandMsg)
	if !ok {
		t.Fatalf("expandDiffCmd returned %T, want diffExpandMsg", msg)
	}
	if expanded.skill != "demo-skill" {
		t.Fatalf("expanded skill = %q, want demo-skill", expanded.skill)
	}
	if len(expanded.files) == 0 {
		t.Fatalf("expected file list for dst-only entry, got none")
	}
	if expanded.diff != "" {
		t.Fatalf("expected empty diff for dst-only entry, got %q", expanded.diff)
	}
}

func TestBuildDiffDetail_ShowsExpandedFilesFromModelState(t *testing.T) {
	results := []targetDiffResult{
		{
			name:   "t1",
			mode:   "copy",
			synced: false,
			items: []copyDiffEntry{
				{name: "demo-skill", action: "remove", reason: "local only", dstDir: "/tmp/demo"},
			},
		},
	}
	m := newDiffTUIModel(results)
	m.refreshDetailCache()
	m.expandedSkill = "demo-skill"
	m.expandedFiles = []fileDiffEntry{
		{RelPath: "local.txt", Action: "delete"},
	}

	out := m.buildDiffDetail()
	if !strings.Contains(out, "── demo-skill files ──") {
		t.Fatalf("detail view missing files header:\n%s", out)
	}
	if !strings.Contains(out, "local.txt") {
		t.Fatalf("detail view missing file path:\n%s", out)
	}
}

func TestBuildDiffDetail_ShowsEmptyExpandHint(t *testing.T) {
	results := []targetDiffResult{
		{
			name:   "t1",
			mode:   "copy",
			synced: false,
			items: []copyDiffEntry{
				{name: "demo-skill", action: "remove", reason: "local only", dstDir: "/tmp/demo"},
			},
		},
	}
	m := newDiffTUIModel(results)
	m.refreshDetailCache()
	m.expandedSkill = "demo-skill"

	out := m.buildDiffDetail()
	if !strings.Contains(out, "(No file-level diff available)") {
		t.Fatalf("detail view missing empty expand hint:\n%s", out)
	}
}
