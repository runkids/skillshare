package main

import "testing"

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
