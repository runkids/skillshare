package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	xansi "github.com/charmbracelet/x/ansi"

	"skillshare/internal/trash"
)

func TestTrashTUIRestoreDestination_UsesAgentDestination(t *testing.T) {
	model := newTrashTUIModel(nil, "", "", "/tmp/skills", "/tmp/agents", "", "global")
	model.confirmEntries = []trash.TrashEntry{{Name: "tutor", Kind: "agent"}}

	// The destination is shown home-relative, and CI sandboxes use /tmp as HOME.
	got := model.restoreDestination()
	if got != shortenPath("/tmp/agents") {
		t.Fatalf("agent restore should go to the agent destination only, got %q", got)
	}
}

func TestTrashTUIRestoreDestination_ShowsMixedDestinations(t *testing.T) {
	model := newTrashTUIModel(nil, "", "", "/tmp/skills", "/tmp/agents", "", "global")
	model.confirmEntries = []trash.TrashEntry{{Name: "demo-skill", Kind: "skill"}, {Name: "tutor", Kind: "agent"}}

	got := model.restoreDestination()
	if !strings.Contains(got, "skills to "+shortenPath("/tmp/skills")) || !strings.Contains(got, "agents to "+shortenPath("/tmp/agents")) {
		t.Fatalf("mixed restore should name both destinations, got %q", got)
	}
}

func TestTrashTUIDeleteWithoutSelectionAsksAboutTheCursorRow(t *testing.T) {
	model := newTrashTUIModel([]trash.TrashEntry{{Name: "old-tool"}, {Name: "another"}}, "", "", "/tmp/skills", "/tmp/agents", "", "global")

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	got := next.(trashTUIModel)

	if !got.confirming || len(got.confirmEntries) != 1 || got.confirmEntries[0].Name != "old-tool" {
		t.Fatalf("d with nothing selected should ask about the row under the cursor; confirming=%v entries=%v", got.confirming, got.confirmEntries)
	}
}

func TestTrashEnter_OpensTheTrashedFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: old-tool\n---\n# Old tool\n\nKept for reference.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	model := newTrashTUIModel([]trash.TrashEntry{{Name: "old-tool", Path: dir}}, "", "", "/tmp/skills", "/tmp/agents", "", "global")
	next, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	next, _ = next.Update(tea.KeyMsg{Type: tea.KeyEnter})
	view := xansi.Strip(next.View())

	if !strings.Contains(view, "skillshare trash · old-tool · SKILL.md") || !strings.Contains(view, "Kept for reference.") {
		t.Fatalf("enter should open the trashed skill's SKILL.md:\n%s", view)
	}
}

func TestTrashDetail_ShowsEscapeSequencesInThePreviewAsSymbols(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Old\n\x1b]0;pwned\x07\n"), 0644); err != nil {
		t.Fatal(err)
	}
	model := newTrashTUIModel(nil, "", "", "/tmp/skills", "/tmp/agents", "", "global")
	got := model.renderTrashDetailPanel(trash.TrashEntry{Name: "old", Path: dir}, 80)

	if strings.Contains(got, "\x1b]") || !strings.Contains(xansi.Strip(got), "␛]0;pwned␇") {
		t.Fatalf("the preview should show escape sequences as symbols, got %q", got)
	}
}
