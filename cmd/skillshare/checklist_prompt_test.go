package main

import (
	"strings"
	"testing"

	xansi "github.com/charmbracelet/x/ansi"
)

func TestChecklistOptions_AlignsDescriptionsAndUsesIndices(t *testing.T) {
	opts := checklistOptions([]checklistItemData{
		{label: "merge", desc: "link each skill"},
		{label: "symlink", desc: "link the folder"},
	})
	if opts[0].Value != "0" || opts[1].Value != "1" {
		t.Fatalf("values = %q, %q; want indices", opts[0].Value, opts[1].Value)
	}
	if !strings.HasPrefix(opts[0].Label, "merge    ") {
		t.Errorf("label %q is not padded to the longest label", opts[0].Label)
	}
}

func TestChecklistDefault_StartsOnFirstPreSelected(t *testing.T) {
	got := checklistDefault([]checklistItemData{{label: "a"}, {label: "b", preSelected: true}, {label: "c", preSelected: true}})
	if got != "1" {
		t.Errorf("default = %q, want 1", got)
	}
}

func TestChecklistAnswerLabel_PluralForMultiSelect(t *testing.T) {
	if got := checklistAnswerLabel(checklistConfig{itemName: "target"}); got != "Targets" {
		t.Errorf("multi label = %q, want Targets", got)
	}
	if got := checklistAnswerLabel(checklistConfig{itemName: "pattern", singleSelect: true}); got != "Pattern" {
		t.Errorf("single label = %q, want Pattern", got)
	}
}

func TestChecklistAnswer_CountsALongPick(t *testing.T) {
	cfg := checklistConfig{itemName: "skill"}
	labels := []string{"agent-browser", "archify", "codebase-design", "improve-react", "react-doctor"}

	if got := xansi.Strip(checklistAnswer(cfg, labels)); got != "5 skills (agent-browser, archify, codebase-design, …)" {
		t.Fatalf("checklistAnswer() = %q", got)
	}
}

func TestChecklistAnswer_NamesAShortPick(t *testing.T) {
	if got := checklistAnswer(checklistConfig{itemName: "skill"}, []string{"archify", "react-doctor"}); got != "archify, react-doctor" {
		t.Fatalf("checklistAnswer() = %q", got)
	}
}
