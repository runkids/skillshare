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

// groupItem is a non-selectable visual separator in the skill list.
type groupItem struct {
	label string // display name (repo name without "_" prefix, or "local")
	count int    // number of skills in this group
}

func (g groupItem) FilterValue() string { return "" }
func (g groupItem) Title() string       { return g.label }
func (g groupItem) Description() string { return "" }

// listSkillDelegate renders a compact single-line browser row for the list TUI.
type listSkillDelegate struct{}

func (listSkillDelegate) Height() int  { return 1 }
func (listSkillDelegate) Spacing() int { return 0 }
func (listSkillDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd {
	return nil
}

func (listSkillDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	width := m.Width()
	if width <= 0 {
		width = 40
	}

	switch v := item.(type) {
	case groupItem:
		renderGroupRow(w, v, width)
	case skillItem:
		selected := index == m.Index()
		renderSkillRow(w, v, width, selected)
	}
}

func renderGroupRow(w io.Writer, g groupItem, width int) {
	label := g.label
	if g.count > 0 {
		label += fmt.Sprintf(" (%d)", g.count)
	}
	label = tc.Dim.Render(label)

	lineWidth := width - lipgloss.Width(label) - 3 // "─ " prefix + " "
	if lineWidth < 2 {
		lineWidth = 2
	}
	line := strings.Repeat("─", lineWidth)

	fmt.Fprint(w, tc.Dim.Render("─ ")+label+" "+tc.Dim.Render(line))
}

func renderSkillRow(w io.Writer, skill skillItem, width int, selected bool) {
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
	// Subtract padding so text + padding fits within bodyWidth.
	textWidth := bodyWidth - bodyStyle.GetPaddingLeft() - bodyStyle.GetPaddingRight()
	if textWidth < 8 {
		textWidth = 8
	}

	line1 = truncateANSI(line1, textWidth)

	fmt.Fprint(w, lipgloss.JoinHorizontal(lipgloss.Top, prefixStyle.Render("▌"), bodyStyle.Width(bodyWidth).MaxWidth(bodyWidth).Render(line1)))
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
	if badge := skillTypeBadge(i.entry); badge != "" {
		title += "  " + badge
	}
	return title
}

// Description returns a one-line summary for tests and non-custom render paths.
func (i skillItem) Description() string {
	return ""
}

func skillTitleLine(e skillEntry) string {
	title := colorSkillPath(compactSkillPath(e))
	if badge := skillTypeBadge(e); badge != "" {
		return title + "  " + badge
	}
	return title
}

// compactSkillPath returns a short display path for list rows.
// For tracked repos, strips the repo prefix (first segment) then shows
// at most 2 trailing segments. The full path is in the detail panel header.
func compactSkillPath(e skillEntry) string {
	full := baseSkillPath(e)
	segments := strings.Split(full, "/")

	// Tracked repos: first segment is the repo dir (e.g. "_runkids-my-skills").
	if e.RepoName != "" && len(segments) > 1 {
		segments = segments[1:]
	}

	if len(segments) > 2 {
		segments = segments[len(segments)-2:]
	}
	return strings.Join(segments, "/")
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
	if e.RepoName == "" && e.Source == "" {
		return tc.BadgeLocal.Render("local")
	}
	return ""
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

	sep := tc.Faint.Render("/")
	return strings.Join(parts, sep) + sep + tc.Emphasis.Render(name)
}

// colorSkillPathBold is like colorSkillPath but renders the skill name in bold
// for extra prominence in the detail panel header.
func colorSkillPathBold(path string) string {
	segments := strings.Split(path, "/")
	boldName := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	if len(segments) <= 1 {
		return boldName.Render(path)
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

	sep := tc.Faint.Render("/")
	return strings.Join(parts, sep) + sep + boldName.Render(name)
}

func truncateANSI(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	return xansi.Truncate(s, width, "…")
}

// toSkillItems converts a slice of skillEntry to skillItem slice.
func toSkillItems(entries []skillEntry) []skillItem {
	items := make([]skillItem, len(entries))
	for i, e := range entries {
		items[i] = skillItem{entry: e}
	}
	return items
}

// buildGroupedItems inserts groupItem separators before each repo/local group.
// Skills must be sorted by RelPath (tracked repos with "_" prefix sort first).
// If all skills belong to a single group (e.g. all standalone), no separators are added.
func buildGroupedItems(skills []skillItem) []list.Item {
	// Check if there are multiple groups.
	hasTracked := false
	hasStandalone := false
	for _, s := range skills {
		if s.entry.RepoName != "" {
			hasTracked = true
		} else {
			hasStandalone = true
		}
		if hasTracked && hasStandalone {
			break
		}
	}
	multiGroup := hasTracked && hasStandalone || countRepoGroups(skills) > 1

	if !multiGroup {
		items := make([]list.Item, len(skills))
		for i, s := range skills {
			items[i] = s
		}
		return items
	}

	var items []list.Item
	var currentGroup string
	groupCount := 0

	flush := func() {
		if groupCount > 0 {
			// Patch the count into the last group header
			for i := len(items) - 1 - groupCount; i >= 0; i-- {
				if g, ok := items[i].(groupItem); ok {
					g.count = groupCount
					items[i] = g
					break
				}
			}
		}
	}

	for _, s := range skills {
		key := s.entry.RepoName // "" for local
		if key != currentGroup {
			flush()
			label := "standalone"
			if key != "" {
				label = strings.TrimPrefix(key, "_")
			}
			items = append(items, groupItem{label: label})
			currentGroup = key
			groupCount = 0
		}
		items = append(items, s)
		groupCount++
	}
	flush()
	return items
}

// countRepoGroups returns the number of distinct repo groups (by RepoName).
func countRepoGroups(skills []skillItem) int {
	seen := map[string]bool{}
	for _, s := range skills {
		seen[s.entry.RepoName] = true
	}
	return len(seen)
}

// skipGroupItem advances the list selection past groupItem separators.
// direction: +1 for down, -1 for up.
func skipGroupItem(l *list.Model, direction int) {
	items := l.Items()
	idx := l.Index()
	n := len(items)
	for {
		if idx < 0 || idx >= n {
			break
		}
		if _, isGroup := items[idx].(groupItem); !isGroup {
			break
		}
		idx += direction
	}
	// Clamp
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	l.Select(idx)
}
