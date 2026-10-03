package main

import (
	"strings"
	"testing"
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
