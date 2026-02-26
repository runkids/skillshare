package main

import (
	"fmt"
	"strings"
)

// skillItem wraps skillEntry to implement bubbles/list.Item interface.
type skillItem struct {
	entry skillEntry
}

// FilterValue returns the searchable text for bubbletea's built-in fuzzy filter.
// Includes name, path, and source so users can filter by any field.
func (i skillItem) FilterValue() string {
	parts := []string{i.entry.Name}
	if i.entry.RelPath != "" && i.entry.RelPath != i.entry.Name {
		parts = append(parts, i.entry.RelPath)
	}
	if i.entry.Source != "" {
		parts = append(parts, i.entry.Source)
	}
	return strings.Join(parts, " ")
}

// Title returns the skill name with a type badge for the list delegate.
// Inline colors: name → bright white, badge: tracked → green, local → gray.
func (i skillItem) Title() string {
	nameStr := i.entry.Name
	if i.entry.RelPath != "" && i.entry.RelPath != i.entry.Name {
		nameStr = i.entry.RelPath
	}
	title := tc.Emphasis.Render(nameStr)

	if i.entry.RepoName != "" {
		title += "  " + tc.Green.Render("[tracked]")
	} else if i.entry.Source == "" {
		title += "  " + tc.Dim.Render("[local]")
	}
	return title
}

// Description returns a one-line summary for the list delegate.
// Local skills return "" since the [local] badge in Title() already conveys this.
func (i skillItem) Description() string {
	if i.entry.RepoName != "" {
		return fmt.Sprintf("tracked: %s", i.entry.RepoName)
	}
	if i.entry.Source != "" {
		return abbreviateSource(i.entry.Source)
	}
	return ""
}

// toSkillItems converts a slice of skillEntry to skillItem slice.
func toSkillItems(entries []skillEntry) []skillItem {
	items := make([]skillItem, len(entries))
	for i, e := range entries {
		items[i] = skillItem{entry: e}
	}
	return items
}
