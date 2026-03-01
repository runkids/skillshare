package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

func TestEnterKey_TriggersLoadingForLocalOnly(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("hello"), 0o644)

	results := []targetDiffResult{
		{
			name:   "claude",
			mode:   "merge",
			synced: false,
			items: []copyDiffEntry{
				{name: "my-skill", action: "remove", reason: "local only", dstDir: tmpDir},
			},
			localCount: 1,
		},
	}
	m := newDiffTUIModel(results)
	// Simulate WindowSizeMsg to init list
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = m2.(diffTUIModel)

	// Press Enter
	m3, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = m3.(diffTUIModel)

	if !m.loading {
		t.Fatal("expected loading=true after Enter on local-only item")
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd after Enter")
	}

	// Simulate receiving the diffExpandMsg (run the cmd synchronously)
	// The cmd is a tea.Batch, so we need to extract and run expandDiffCmd directly
	entry := copyDiffEntry{name: "my-skill", action: "remove", reason: "local only", dstDir: tmpDir}
	msg := expandDiffCmd(&entry)()
	m4, _ := m.Update(msg)
	m = m4.(diffTUIModel)

	if m.loading {
		t.Fatal("expected loading=false after diffExpandMsg")
	}
	if m.expandedSkill != "my-skill" {
		t.Fatalf("expandedSkill = %q, want my-skill", m.expandedSkill)
	}
	if len(m.expandedFiles) == 0 {
		t.Fatal("expected expandedFiles to be populated")
	}

	// Verify view contains the file
	out := m.buildDiffDetail()
	if !strings.Contains(out, "README.md") {
		t.Fatalf("detail view missing file:\n%s", out)
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
