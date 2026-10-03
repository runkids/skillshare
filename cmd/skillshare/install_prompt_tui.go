package main

import (
	"errors"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"skillshare/internal/install"
	"skillshare/internal/ui"
)

// dirPickerLevel is one folder level the picker has gone into.
type dirPickerLevel struct {
	prefix string
	skills []install.SkillInfo
}

// runDirPickerTUI asks, one folder level at a time, which folder of a large
// repo to install from. The first choice at each level is every skill below
// it. Esc goes up a level, and cancels at the top.
// Returns (selected skills, installAll flag, error); (nil, false, nil) means
// the user cancelled.
func runDirPickerTUI(skills []install.SkillInfo) ([]install.SkillInfo, bool, error) {
	stack := []dirPickerLevel{{skills: skills}}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		groups := groupSkillsByDirectory(cur.skills, cur.prefix)

		items := []checklistItemData{{label: "All " + countNoun(len(cur.skills), "skill"), desc: "everything below here"}}
		for _, g := range groups {
			items = append(items, checklistItemData{label: g.dir, desc: countNoun(len(g.skills), "skill")})
		}
		title := "Install from which folder?"
		if cur.prefix != "" {
			title = "Install from which folder in " + cur.prefix + "?"
		}
		v, err := ui.Select(title, checklistOptions(items), "0")
		if errors.Is(err, ui.ErrCancelled) {
			stack = stack[:len(stack)-1]
			continue
		}
		if err != nil {
			return nil, false, err
		}
		choice, err := strconv.Atoi(v)
		if err != nil {
			return nil, false, err
		}
		if choice == 0 {
			ui.Answered("folder", dirPickerLabel(cur.prefix))
			return cur.skills, true, nil
		}

		g := groups[choice-1]
		// "(root)" is a virtual group for root-level skills — always a leaf.
		if g.dir == "(root)" {
			ui.Answered("folder", dirPickerLabel(cur.prefix))
			return g.skills, false, nil
		}
		prefix := g.dir
		if cur.prefix != "" {
			prefix = cur.prefix + "/" + g.dir
		}
		// A folder with no subfolders is where the skills are picked one by one.
		sub := groupSkillsByDirectory(g.skills, prefix)
		if len(sub) == 1 && sub[0].dir == "(root)" {
			ui.Answered("folder", prefix)
			return g.skills, false, nil
		}
		stack = append(stack, dirPickerLevel{prefix: prefix, skills: g.skills})
	}
	return nil, false, nil
}

// dirPickerLabel names a folder level on the answer line.
func dirPickerLabel(prefix string) string {
	if prefix == "" {
		return "repo root"
	}
	return prefix
}

// ---------------------------------------------------------------------------
// Skill select TUI — multi-select with checkboxes
// ---------------------------------------------------------------------------

// skillSelectItem is a list item for the skill multi-select TUI.
// Title() returns plain text — no inline ANSI — so bubbles filter highlighting works correctly.
// runSkillSelectTUI asks which skills to install, inline. Skills are sorted by
// path so ones in the same folder sit together. (nil, nil) means cancelled.
func runSkillSelectTUI(skills []install.SkillInfo) ([]install.SkillInfo, error) {
	sorted := make([]install.SkillInfo, len(skills))
	copy(sorted, skills)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })

	items := make([]checklistItemData, len(sorted))
	for i, s := range sorted {
		var parts []string
		if s.Description != "" {
			parts = append(parts, s.Description)
		}
		if loc := skillSelectLocation(s.Path); loc != "root" {
			parts = append(parts, loc)
		}
		if s.License != "" {
			parts = append(parts, s.License)
		}
		desc := strings.Join(parts, " · ")
		items[i] = checklistItemData{label: s.Name, desc: desc}
	}
	indices, err := runChecklistTUI(checklistConfig{title: "Install which skills?", items: items, itemName: "skill"})
	if err != nil || indices == nil {
		return nil, err
	}
	selected := make([]install.SkillInfo, len(indices))
	for i, idx := range indices {
		selected[i] = sorted[idx]
	}
	return selected, nil
}

// skillSelectLocation is the folder a skill sits in, or "root".
func skillSelectLocation(path string) string {
	if dir := filepath.Dir(path); path != "." && dir != "." {
		return dir
	}
	return "root"
}
