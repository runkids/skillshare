package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
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
// Color hierarchy: top-level group → cyan, sub-dirs → dim, separator → faint, skill name → bright white.
func (i skillItem) Title() string {
	nameStr := i.entry.Name
	if i.entry.RelPath != "" && i.entry.RelPath != i.entry.Name {
		nameStr = i.entry.RelPath
	}

	title := colorSkillPath(nameStr)

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

// colorSkillPath renders a skill path with progressive luminance:
// top-level group → cyan, sub-dirs → dark gray..light gray, "/" → faint, skill name → bright white.
func colorSkillPath(path string) string {
	segments := strings.Split(path, "/")
	if len(segments) <= 1 {
		return tc.Emphasis.Render(path)
	}

	sep := tc.Faint.Render("/")
	dirs := segments[:len(segments)-1]
	name := segments[len(segments)-1]

	const (
		grayStart = 241 // darkest sub-dir
		grayEnd   = 249 // lightest sub-dir (approaching white)
	)

	var parts []string
	for idx, dir := range dirs {
		if idx == 0 {
			parts = append(parts, tc.Cyan.Render(dir))
		} else {
			gray := grayStart
			if subCount := len(dirs) - 1; subCount > 1 {
				gray = grayStart + (idx-1)*(grayEnd-grayStart)/(subCount-1)
			}
			style := lipgloss.NewStyle().Foreground(lipgloss.Color(fmt.Sprintf("%d", gray)))
			parts = append(parts, style.Render(dir))
		}
	}

	return strings.Join(parts, sep) + sep + tc.Emphasis.Render(name)
}

// toSkillItems converts a slice of skillEntry to skillItem slice.
func toSkillItems(entries []skillEntry) []skillItem {
	items := make([]skillItem, len(entries))
	for i, e := range entries {
		items[i] = skillItem{entry: e}
	}
	return items
}
