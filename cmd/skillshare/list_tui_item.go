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

// Title returns the skill name for the list delegate.
// When the skill is nested (e.g. "frontend/react-helper"), show the full
// relative path so the group context is visible in the flat list.
func (i skillItem) Title() string {
	if i.entry.RelPath != "" && i.entry.RelPath != i.entry.Name {
		return i.entry.RelPath
	}
	return i.entry.Name
}

// Description returns a one-line summary for the list delegate.
func (i skillItem) Description() string {
	if i.entry.RepoName != "" {
		return fmt.Sprintf("tracked: %s", i.entry.RepoName)
	}
	if i.entry.Source != "" {
		return abbreviateSource(i.entry.Source)
	}
	return "local"
}

// toSkillItems converts a slice of skillEntry to skillItem slice.
func toSkillItems(entries []skillEntry) []skillItem {
	items := make([]skillItem, len(entries))
	for i, e := range entries {
		items[i] = skillItem{entry: e}
	}
	return items
}
