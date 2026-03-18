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
			header := wizardHeader(selectedPattern, "")
			c, err := promptCategoryWith(header)
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
			header := wizardHeader(selectedPattern, selectedCategory)
			pat := findPattern(selectedPattern)
			yes, err := promptScaffoldDirsWith(pat, header)
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

// wizardHeader builds a compact breadcrumb string for the help bar footer.
// e.g. "✓ reviewer → quality"
func wizardHeader(pattern, category string) string {
	check := tc.Green.Render("✓")
	var parts []string
	if pattern != "" {
		parts = append(parts, check+" "+pattern)
	}
	if category != "" {
		parts = append(parts, category)
	}
	return strings.Join(parts, " → ")
}

// promptCategoryWith runs the category TUI with a header showing previous selections.
func promptCategoryWith(header string) (string, error) {
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
		header:       header,
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

// promptScaffoldDirsWith runs the scaffold TUI with a header showing previous selections.
func promptScaffoldDirsWith(pattern *skillPattern, header string) (bool, error) {
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
		header:       header,
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
