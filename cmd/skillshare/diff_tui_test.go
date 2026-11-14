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
	if !strings.Contains(out, "demo-skill files") {
		t.Fatalf("detail view missing files header:\n%s", out)
	}
	if !strings.Contains(out, "local.txt") {
		t.Fatalf("detail view missing file path:\n%s", out)
	}
}

func TestBuildDiffDetail_ProfilePruneKeepsSyncHint(t *testing.T) {
	m := newDiffTUIModel([]targetDiffResult{{
		name: "claude", mode: "copy", syncCount: 1,
		items: []copyDiffEntry{{name: "review", action: "remove", reason: "not in profile frontend", isSync: true}},
	}})
	m.refreshDetailCache()
	out := m.buildDiffDetail()
	if !strings.Contains(out, "not in profile frontend") || !strings.Contains(out, "skillshare sync") {
		t.Fatalf("profile removal lost its reason or sync next step:\n%s", out)
	}
	if strings.Contains(out, "sync --force") {
		t.Fatalf("profile removal must not suggest force:\n%s", out)
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

func TestDiffTUI_KeptOnlyTarget(t *testing.T) {
	results := []targetDiffResult{{
		name: "claude",
		mode: "copy",
		items: []copyDiffEntry{
			{name: "emil-design-prototype", action: "remove", reason: "local folder; the skill stays at prototype", keptAt: "prototype"},
		},
	}}
	m := newDiffTUIModel(results)
	m.refreshDetailCache()

	if _, desc := (diffTargetItem{result: results[0]}).row(); !strings.Contains(desc, "1 kept") {
		t.Errorf("row = %q, want 1 kept", desc)
	}
	if title := m.renderTitleLine(); !strings.Contains(title, "1 kept under old name") || strings.Contains(title, "differ") {
		t.Errorf("title = %q, want a kept count and no differ", title)
	}
	detail := m.buildDiffDetail()
	for _, want := range []string{"emil-design-prototype (stays at prototype)", "after renaming or removing the local folders"} {
		if !strings.Contains(detail, want) {
			t.Errorf("detail missing %q:\n%s", want, detail)
		}
	}
}

func TestDiffTUI_PreservationPresentation(t *testing.T) {
	const reason = "kept: copy was edited or could not be verified; collect or remove manually, or use verified --discard-edits"
	entry := copyDiffEntry{name: "alpha", action: "kept", reason: reason}
	result := targetDiffResult{name: "custom", mode: "copy", synced: true, items: []copyDiffEntry{entry}}
	m := newDiffTUIModel([]targetDiffResult{result})
	m.refreshDetailCache()
	if _, desc := (diffTargetItem{result: result}).row(); !strings.Contains(desc, "kept") {
		t.Errorf("kept target row: %q", desc)
	}
	title := m.renderTitleLine()
	if !strings.Contains(title, "kept") || strings.Contains(title, "under old name") {
		t.Errorf("preservation confused with a naming migration: %s", title)
	}
	for _, category := range categorizeItems([]copyDiffEntry{entry}) {
		if strings.Contains(category.label, "old name") || category.kind == "orphan" {
			t.Errorf("kept copy miscategorized: %+v", category)
		}
	}
	m.expandedSkill = "alpha"
	// Destination-only expansion lists existing files with delete actions.
	m.expandedFiles = []fileDiffEntry{{RelPath: "SKILL.md", Action: "delete"}}
	detail := m.buildDiffDetail()
	for _, want := range []string{
		"Kept 1 skill", "copy was edited or could not be verified", "alpha",
		strings.TrimPrefix(reason, "kept: "),
		"collect or remove manually", "verified --discard-edits",
		"alpha files (kept)", "· SKILL.md", "Next", "skillshare collect",
	} {
		if !strings.Contains(detail, want) {
			t.Errorf("kept detail missing %q:\n%s", want, detail)
		}
	}
	for _, wrong := range []string{"Kept: kept:", "--discard-edits 1 skill", "- SKILL.md", "sync --force", "stays at"} {
		if strings.Contains(detail, wrong) {
			t.Errorf("kept detail has misleading action/header %q:\n%s", wrong, detail)
		}
	}
	if strings.Contains(m.renderTitleLine(), "to sync") || result.inSync() {
		t.Error("kept inventory became sync work or disappeared as in sync")
	}
	if wire := diffItemToJSON(entry); wire.Action != "kept" || wire.IsSync || wire.Reason != reason {
		t.Errorf("presentation changed raw preservation data: %+v", wire)
	}

	t.Run("reason_groups", func(t *testing.T) {
		groups := categorizeItems([]copyDiffEntry{
			entry,
			{name: "beta", action: "kept", reason: reason},
			{name: "gamma", action: "kept", reason: "kept: entry is not a recorded copy; collect or remove manually"},
		})
		if len(groups) != 2 || len(groups[0].names) != 2 || len(groups[1].names) != 1 {
			t.Fatalf("preservation reason grouping changed: %+v", groups)
		}
		for _, group := range groups {
			if group.label != "Kept" {
				t.Errorf("recovery prose still used as category heading: %+v", group)
			}
		}
	})

	t.Run("actual_changes", func(t *testing.T) {
		m := newDiffTUIModel([]targetDiffResult{{
			name: "custom", mode: "copy", syncCount: 1,
			items: []copyDiffEntry{{name: "alpha", action: "modify", reason: "content changed", isSync: true}},
		}})
		m.refreshDetailCache()
		m.expandedSkill = "alpha"
		m.expandedFiles = []fileDiffEntry{
			{RelPath: "added.md", Action: "add"},
			{RelPath: "removed.md", Action: "delete"},
			{RelPath: "changed.md", Action: "modify"},
		}
		detail := m.buildDiffDetail()
		for _, want := range []string{"alpha files", "+ added.md", "- removed.md", "~ changed.md"} {
			if !strings.Contains(detail, want) {
				t.Errorf("actual change icon missing %q:\n%s", want, detail)
			}
		}
		if strings.Contains(detail, "files (kept)") {
			t.Error("actual changes relabeled as kept inventory")
		}
	})
}

func TestDiffTUI_PreservedInventoryExpansionIdentity(t *testing.T) {
	root := t.TempDir()
	files := []string{"current-only.md", "former-only.md"}
	entries := make([]copyDiffEntry, 2)
	for i, dir := range []string{"current", "former"} {
		path := filepath.Join(root, dir)
		writeTestFile(t, path, files[i], "authored "+dir)
		entries[i] = copyDiffEntry{name: "alpha", action: "kept", reason: "kept: edited copy (in " + path + ")", dstDir: path}
	}
	m := newDiffTUIModel([]targetDiffResult{{name: "goose", mode: "copy", items: entries}})
	m.refreshDetailCache()
	for i := range entries {
		msg, ok := expandDiffCmd(&entries[i])().(diffExpandMsg)
		if !ok || msg.skill != "alpha" || msg.diff != "" || len(msg.files) != 1 || msg.files[0].RelPath != files[i] {
			t.Fatalf("expansion lost actual destination inventory: %+v", msg)
		}
		m.loading = true
		updated, _ := m.Update(msg)
		m = updated.(diffTUIModel)
		detail := stripANSIWarnings(m.buildDiffDetail())
		if !strings.Contains(detail, "· "+files[i]) || strings.Contains(detail, "- "+files[i]) || strings.Contains(detail, files[1-i]) {
			t.Errorf("expansion reused a same-name inventory from another directory:\n%s", detail)
		}
		for _, entry := range entries {
			if !strings.Contains(detail, strings.TrimPrefix(entry.reason, "kept: ")) {
				t.Errorf("expansion lost distinct directory reason %q:\n%s", entry.reason, detail)
			}
		}
		if strings.Contains(m.renderTitleLine(), "to sync") {
			t.Error("expanded preservation became sync work")
		}
		if data, err := os.ReadFile(filepath.Join(entries[i].dstDir, files[i])); err != nil || string(data) != "authored "+filepath.Base(entries[i].dstDir) {
			t.Errorf("expansion wrote authored destination: %q, %v", data, err)
		}
	}
}
