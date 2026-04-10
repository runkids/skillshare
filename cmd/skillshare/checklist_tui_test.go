package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestChecklistItem_Title_MultiSelect(t *testing.T) {
	item := checklistItem{idx: 0, label: "claude", selected: false, singleSelect: false}
	if got := item.Title(); got != "[ ] claude" {
		t.Errorf("unselected multi-select: got %q, want %q", got, "[ ] claude")
	}

	item.selected = true
	if got := item.Title(); got != "[x] claude" {
		t.Errorf("selected multi-select: got %q, want %q", got, "[x] claude")
	}
}

func TestChecklistItem_Title_SingleSelect(t *testing.T) {
	item := checklistItem{idx: 0, label: "merge", selected: false, singleSelect: true}
	if got := item.Title(); got != "○ merge" {
		t.Errorf("unselected single-select: got %q, want %q", got, "○ merge")
	}

	item.selected = true
	if got := item.Title(); got != "● merge" {
		t.Errorf("selected single-select: got %q, want %q", got, "● merge")
	}
}

func TestChecklistItem_PreSelected(t *testing.T) {
	cfg := checklistConfig{
		title: "Test",
		items: []checklistItemData{
			{label: "a", preSelected: false},
			{label: "b", preSelected: true},
			{label: "c", preSelected: true},
		},
	}
	m := newChecklistModel(cfg)
	if m.selCount != 2 {
		t.Errorf("selCount: got %d, want 2", m.selCount)
	}
	if !m.selected[1] || !m.selected[2] {
		t.Errorf("items 1 and 2 should be pre-selected")
	}
	if m.selected[0] {
		t.Errorf("item 0 should not be pre-selected")
	}
}

func TestChecklistItem_PreSelected_SingleSelectUsesFirstAndMovesCursor(t *testing.T) {
	cfg := checklistConfig{
		title:        "Test",
		singleSelect: true,
		items: []checklistItemData{
			{label: "a", preSelected: false},
			{label: "b", preSelected: true},
			{label: "c", preSelected: true},
		},
	}
	m := newChecklistModel(cfg)
	if m.selCount != 1 {
		t.Fatalf("selCount: got %d, want 1", m.selCount)
	}
	if !m.selected[1] {
		t.Fatalf("item 1 should be selected")
	}
	if m.list.Index() != 1 {
		t.Fatalf("cursor index: got %d, want 1", m.list.Index())
	}
}

func TestChecklistItem_SingleSelectDefaultsToFirst(t *testing.T) {
	cfg := checklistConfig{
		title:        "Test",
		singleSelect: true,
		items: []checklistItemData{
			{label: "a"},
			{label: "b"},
		},
	}
	m := newChecklistModel(cfg)
	if m.selCount != 1 {
		t.Fatalf("selCount: got %d, want 1", m.selCount)
	}
	if !m.selected[0] {
		t.Fatalf("item 0 should be selected by default")
	}
	if m.list.Index() != 0 {
		t.Fatalf("cursor index: got %d, want 0", m.list.Index())
	}
}

func TestChecklistItem_SingleSelectNavigationUpdatesSelection(t *testing.T) {
	cfg := checklistConfig{
		title:        "Test",
		singleSelect: true,
		items: []checklistItemData{
			{label: "a", preSelected: true},
			{label: "b"},
			{label: "c"},
		},
	}
	model := newChecklistModel(cfg)
	modelAny, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	m := modelAny.(checklistModel)

	if !m.selected[1] {
		t.Fatalf("item 1 should be selected after moving cursor down")
	}
	if m.selected[0] {
		t.Fatalf("item 0 should no longer be selected after moving cursor down")
	}
	if m.list.Index() != 1 {
		t.Fatalf("cursor index: got %d, want 1", m.list.Index())
	}
}

func TestChecklistItem_SingleSelectEnterConfirmsFocusedItem(t *testing.T) {
	cfg := checklistConfig{
		title:        "Test",
		singleSelect: true,
		items: []checklistItemData{
			{label: "a", preSelected: true},
			{label: "b"},
		},
	}
	model := newChecklistModel(cfg)
	modelAny, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	m := modelAny.(checklistModel)
	modelAny, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = modelAny.(checklistModel)

	if len(m.result) != 1 || m.result[0] != 1 {
		t.Fatalf("result: got %v, want [1]", m.result)
	}
}

func TestChecklistItem_SingleSelectSpaceDoesNotClearSelection(t *testing.T) {
	cfg := checklistConfig{
		title:        "Test",
		singleSelect: true,
		items: []checklistItemData{
			{label: "a", preSelected: true},
			{label: "b"},
		},
	}
	model := newChecklistModel(cfg)
	modelAny, _ := model.Update(tea.KeyMsg{Type: tea.KeySpace})
	m := modelAny.(checklistModel)

	if m.selCount != 1 {
		t.Fatalf("selCount: got %d, want 1", m.selCount)
	}
	if !m.selected[0] {
		t.Fatalf("item 0 should remain selected after pressing space")
	}
}

func TestChecklistItem_FilterValue(t *testing.T) {
	item := checklistItem{idx: 0, label: "claude", desc: "AI assistant"}
	if got := item.FilterValue(); got != "claude AI assistant" {
		t.Errorf("with desc: got %q, want %q", got, "claude AI assistant")
	}

	item.desc = ""
	if got := item.FilterValue(); got != "claude" {
		t.Errorf("no desc: got %q, want %q", got, "claude")
	}
}
