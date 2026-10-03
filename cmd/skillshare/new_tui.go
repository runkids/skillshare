package main

import (
	"errors"
	"fmt"
	"strings"
)

var errCancelled = errors.New("cancelled")

// promptPattern asks for one of the skill patterns.
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
		title:        "Pattern",
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

// promptCategory asks for a skill category, with a skip option.
// Returns the selected category key, or "" if skipped.
// Returns errCancelled if user presses Esc/q.
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
		title:        "Category",
		items:        items,
		singleSelect: true,
		itemName:     "category",
	})
	if err != nil {
		return "", err
	}
	if indices == nil {
		return "", errCancelled // user pressed Esc
	}
	idx := indices[0]
	if idx == len(skillCategories) {
		return "", nil // explicitly skipped
	}
	return skillCategories[idx].Key, nil
}

// runNewWizard asks pattern → category → scaffold inline; each answer stays
// on screen as a ✓ line. esc at any step cancels the whole wizard.
// Returns ("", "", false) if cancelled.
func runNewWizard() (selectedPattern, selectedCategory string, createDirs bool) {
	p, err := promptPattern()
	if err != nil || p == "" {
		return "", "", false
	}
	if p == "none" {
		return p, "", false
	}
	c, err := promptCategory()
	if err != nil {
		return "", "", false
	}
	yes, err := promptScaffoldDirs(findPattern(p))
	if err != nil {
		return "", "", false
	}
	return p, c, yes
}

// promptScaffoldDirs asks whether to create the pattern's recommended dirs.
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
