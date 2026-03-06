package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	xansi "github.com/charmbracelet/x/ansi"
)

// skillItem wraps skillEntry to implement bubbles/list.Item interface.
type skillItem struct {
	entry skillEntry
}

// listSkillDelegate renders a compact single-line browser row for the list TUI.
type listSkillDelegate struct{}

func (listSkillDelegate) Height() int  { return 1 }
func (listSkillDelegate) Spacing() int { return 0 }
func (listSkillDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd {
	return nil
}

func (listSkillDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	skill, ok := item.(skillItem)
	if !ok {
		return
	}

	width := m.Width()
	if width <= 0 {
		width = 40
	}

	selected := index == m.Index()
	line1 := skillTitleLine(skill.entry)

	prefixStyle := tc.ListRowPrefix
	bodyStyle := tc.ListRow
	if selected {
		prefixStyle = tc.ListRowPrefixSelected
		bodyStyle = tc.ListRowSelected
	}

	bodyWidth := width - lipgloss.Width(prefixStyle.Render("▌"))
	if bodyWidth < 10 {
		bodyWidth = 10
	}

	line1 = truncateANSI(line1, bodyWidth)

	fmt.Fprint(w, lipgloss.JoinHorizontal(lipgloss.Top, prefixStyle.Render("▌"), bodyStyle.Width(bodyWidth).Render(line1)))
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

// Title returns the skill name with a type badge for tests and non-custom render paths.
func (i skillItem) Title() string {
	title := baseSkillPath(i.entry)
	if i.entry.RepoName != "" {
		title += "  [tracked]"
	} else if i.entry.Source == "" {
		title += "  [local]"
	}
	return title
}

// Description returns a one-line summary for tests and non-custom render paths.
func (i skillItem) Description() string {
	return ""
}

func skillTitleLine(e skillEntry) string {
	title := colorSkillPath(baseSkillPath(e))
	if badge := skillTypeBadge(e); badge != "" {
		return title + "  " + badge
	}
	return title
}

func baseSkillPath(e skillEntry) string {
	if e.RelPath != "" && e.RelPath != e.Name {
		return e.RelPath
	}
	if e.RelPath != "" {
		return e.RelPath
	}
	return e.Name
}

func skillTypeBadge(e skillEntry) string {
	switch {
	case e.RepoName == "" && e.Source == "":
		return tc.BadgeLocal.Render("loc")
	default:
		return ""
	}
}

// colorSkillPath renders a skill path with progressive luminance:
// top-level group → cyan, sub-dirs → dark gray..light gray, skill name → bright white.
func colorSkillPath(path string) string {
	segments := strings.Split(path, "/")
	if len(segments) <= 1 {
		return tc.Emphasis.Render(path)
	}

	dirs := segments[:len(segments)-1]
	name := segments[len(segments)-1]

	const (
		grayStart = 241
		grayEnd   = 249
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

	sep := tc.Faint.Render(" / ")
	return strings.Join(parts, sep) + sep + tc.Emphasis.Render(name)
}

func truncateANSI(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	runes := []rune(xansi.Strip(s))
	if width <= 1 {
		return string(runes[:width])
	}
	if len(runes) > width-1 {
		runes = runes[:width-1]
	}
	return string(runes) + "…"
}

// toSkillItems converts a slice of skillEntry to skillItem slice.
func toSkillItems(entries []skillEntry) []skillItem {
	items := make([]skillItem, len(entries))
	for i, e := range entries {
		items[i] = skillItem{entry: e}
	}
	return items
}
