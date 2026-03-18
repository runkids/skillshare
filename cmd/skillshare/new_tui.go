package main

import (
	"errors"
	"fmt"
	"strings"
)

var errCancelled = errors.New("cancelled")

// promptPattern shows a single-select TUI listing all skill patterns.
// Returns the selected pattern name, or "" if cancelled.
func promptPattern() (string, error) {
	items := make([]checklistItemData, len(skillPatterns))
	for i, p := range skillPatterns {
		items[i] = checklistItemData{
			label: p.Name,
			desc:  p.Description,
		}
	}

	indices, err := runChecklistTUI(checklistConfig{
		title:        "Select a design pattern",
		items:        items,
		singleSelect: true,
		itemName:     "pattern",
	})
	if err != nil {
		return "", err
	}
	if indices == nil {
		return "", nil
	}
	return skillPatterns[indices[0]].Name, nil
}

// promptCategory shows a single-select TUI listing skill categories plus a skip option.
// Returns the selected category key, or "" if cancelled/skipped.
func promptCategory() (string, error) {
	items := make([]checklistItemData, len(skillCategories)+1)
	for i, c := range skillCategories {
		items[i] = checklistItemData{
			label: c.Key,
			desc:  c.Label,
		}
	}
	items[len(skillCategories)] = checklistItemData{
		label: "(skip)",
		desc:  "No category",
	}

	indices, err := runChecklistTUI(checklistConfig{
		title:        "Select a category",
		items:        items,
		singleSelect: true,
		itemName:     "category",
	})
	if err != nil {
		return "", err
	}
	if indices == nil {
		return "", nil
	}
	idx := indices[0]
	if idx == len(skillCategories) {
		return "", nil // skipped
	}
	return skillCategories[idx].Key, nil
}

// promptScaffoldDirs shows a Yes/No TUI asking whether to create recommended dirs.
// Returns true if user selects Yes, false+nil if No, false+errCancelled if cancelled.
func promptScaffoldDirs(pattern *skillPattern) (bool, error) {
	if pattern == nil || len(pattern.ScaffoldDirs) == 0 {
		return false, nil
	}

	dirList := strings.Join(pattern.ScaffoldDirs, ", ")
	desc := fmt.Sprintf("Directories: %s", dirList)

	items := []checklistItemData{
		{label: "Yes", desc: desc},
		{label: "No", desc: "Skip scaffold directories"},
	}

	indices, err := runChecklistTUI(checklistConfig{
		title:        "Create recommended directories?",
		items:        items,
		singleSelect: true,
		itemName:     "option",
	})
	if err != nil {
		return false, err
	}
	if indices == nil {
		return false, errCancelled
	}
	return indices[0] == 0, nil
}
