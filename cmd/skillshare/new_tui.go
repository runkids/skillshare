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
		title:        "Select a category",
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

// runNewWizard runs the full pattern → category → scaffold TUI wizard.
// Esc at pattern = cancel. Esc at category = back to pattern. Esc at scaffold = back to category.
// Previous selections are shown in the TUI title (inside alt screen).
// Returns ("", "", false) if cancelled at pattern step.
func runNewWizard() (selectedPattern, selectedCategory string, createDirs bool) {
	step := 0 // 0=pattern, 1=category, 2=scaffold
	for {
		switch step {
		case 0: // Pattern selection
			p, err := promptPattern()
			if err != nil || p == "" {
				return "", "", false
			}
			selectedPattern = p
			if p == "none" {
				return p, "", false
			}
			step = 1

		case 1: // Category selection
			title := wizardTitle("Select a category", selectedPattern, "")
			c, err := promptCategoryWithTitle(title)
			if errors.Is(err, errCancelled) {
				step = 0 // back to pattern
				continue
			}
			if err != nil {
				return "", "", false
			}
			selectedCategory = c
			pat := findPattern(selectedPattern)
			if pat != nil && len(pat.ScaffoldDirs) > 0 {
				step = 2
			} else {
				return selectedPattern, selectedCategory, false
			}

		case 2: // Scaffold dirs
			title := wizardTitle("Create recommended directories?", selectedPattern, selectedCategory)
			pat := findPattern(selectedPattern)
			yes, err := promptScaffoldDirsWithTitle(pat, title)
			if errors.Is(err, errCancelled) {
				step = 1 // back to category
				continue
			}
			if err != nil {
				return "", "", false
			}
			return selectedPattern, selectedCategory, yes
		}
	}
}

// wizardTitle builds a TUI title that includes breadcrumbs of previous selections.
func wizardTitle(current, pattern, category string) string {
	check := tc.Green.Render("✓")
	dim := tc.Dim

	var lines []string
	if pattern != "" {
		desc := ""
		if p := findPattern(pattern); p != nil {
			desc = dim.Render(" — " + p.Description)
		}
		lines = append(lines, fmt.Sprintf("%s Pattern: %s%s", check, pattern, desc))
	}
	if category != "" {
		label := category
		for _, c := range skillCategories {
			if c.Key == category {
				label = category + dim.Render(" — "+c.Label)
				break
			}
		}
		lines = append(lines, fmt.Sprintf("%s Category: %s", check, label))
	}
	lines = append(lines, current)
	return strings.Join(lines, "\n")
}

// promptCategoryWithTitle runs promptCategory with a custom title including breadcrumbs.
func promptCategoryWithTitle(title string) (string, error) {
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
		title:        title,
		items:        items,
		singleSelect: true,
		itemName:     "category",
	})
	if err != nil {
		return "", err
	}
	if indices == nil {
		return "", errCancelled
	}
	idx := indices[0]
	if idx == len(skillCategories) {
		return "", nil
	}
	return skillCategories[idx].Key, nil
}

// promptScaffoldDirsWithTitle runs promptScaffoldDirs with a custom title including breadcrumbs.
func promptScaffoldDirsWithTitle(pattern *skillPattern, title string) (bool, error) {
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
		title:        title,
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
