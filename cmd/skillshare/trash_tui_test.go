package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"skillshare/internal/trash"
)

func TestTrashTUIRestoreDestination_UsesAgentDestination(t *testing.T) {
	model := newTrashTUIModel(nil, "", "", "/tmp/skills", "/tmp/agents", "", "global")
	model.confirmEntries = []trash.TrashEntry{{Name: "tutor", Kind: "agent"}}

	got := model.restoreDestination()
	if !strings.Contains(got, "/tmp/agents") || strings.Contains(got, "/tmp/skills") {
		t.Fatalf("agent restore should go to the agent destination only, got %q", got)
	}
}

func TestTrashTUIRestoreDestination_ShowsMixedDestinations(t *testing.T) {
	model := newTrashTUIModel(nil, "", "", "/tmp/skills", "/tmp/agents", "", "global")
	model.confirmEntries = []trash.TrashEntry{{Name: "demo-skill", Kind: "skill"}, {Name: "tutor", Kind: "agent"}}

	got := model.restoreDestination()
	if !strings.Contains(got, "skills to /tmp/skills") || !strings.Contains(got, "agents to /tmp/agents") {
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
