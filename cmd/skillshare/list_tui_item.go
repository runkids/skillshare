package main

import (
	"fmt"
	"io"
	"strings"

	"skillshare/internal/theme"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	xansi "github.com/charmbracelet/x/ansi"
)

// skillItem wraps skillEntry to implement bubbles/list.Item interface.
type skillItem struct {
	entry   skillEntry
	grouped bool // shown under a group heading, which names the first path segment
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

func (d listSkillDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	width := m.Width()
	if width <= 0 {
		width = 40
	}

	switch v := item.(type) {
	case groupItem:
		renderGroupRow(w, v, width)
	case skillItem:
		renderPrefixRow(w, skillRowLine(v.entry, v.grouped, width-rowIndent), width, index == m.Index())
	}
}

// renderGroupRow renders a group name as a dim heading over its rows.
func renderGroupRow(w io.Writer, g groupItem, width int) {
	fmt.Fprint(w, truncateANSI(" "+theme.Dim().Render(g.label), width))
}

// rowIndent is the room before a row's text: a space, the "›" cursor on the
// selected row, and a space.
const rowIndent = 3

// renderPrefixRow renders a single-line list row, marking the selected one
// with "›" and a highlight. Shared by the full-screen TUIs.
func renderPrefixRow(w io.Writer, line string, width int, selected bool) {
	textWidth := max(width-rowIndent, 8)
	line = truncateANSI(line, textWidth)
	if !selected {
		fmt.Fprint(w, strings.Repeat(" ", rowIndent)+line)
		return
	}
	// Strip embedded ANSI so the highlight fills the whole row: compound
	// sequences contain resets that break the background in lipgloss.
	fmt.Fprint(w, " "+theme.Accent().Render("›")+" "+theme.SelectedRow().Width(textWidth).Render(xansi.Strip(line)))
}

// prefixItemDelegate is a generic list delegate that renders items with the "▌"
// prefix bar style. It works with any item implementing list.DefaultItem
// (Title() + Description()). Use newPrefixDelegate(showDesc) to create one.
type prefixItemDelegate struct {
	showDesc bool
}

func newPrefixDelegate(showDesc bool) prefixItemDelegate {
	return prefixItemDelegate{showDesc: showDesc}
}

func (d prefixItemDelegate) Height() int {
	if d.showDesc {
		return 2
	}
	return 1
}

func (d prefixItemDelegate) Spacing() int { return 0 }

func (d prefixItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d prefixItemDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	width := m.Width()
	if width <= 0 {
		width = 40
	}
	selected := index == m.Index()

	di, ok := item.(list.DefaultItem)
	if !ok {
		return
	}

	if d.showDesc {
		renderPrefixRowWithDesc(w, di.Title(), di.Description(), width, selected)
	} else {
		renderPrefixRow(w, di.Title(), width, selected)
	}
}

// renderPrefixRowWithDesc renders a 2-line list row: the title as in
// renderPrefixRow and the description under it, dim.
func renderPrefixRowWithDesc(w io.Writer, title, desc string, width int, selected bool) {
	textWidth := max(width-rowIndent, 8)
	renderPrefixRow(w, title, width, selected)
	fmt.Fprint(w, "\n"+strings.Repeat(" ", rowIndent)+theme.Dim().Render(truncateANSI(xansi.Strip(desc), textWidth)))
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

// skillRowLine renders a row's text: the short path, and how it was
// installed (or that it is disabled) aligned at the right edge of width.
func skillRowLine(e skillEntry, grouped bool, width int) string {
	tag := skillTypeCategory(e)
	name := colorSkillPath(compactSkillPath(e, grouped))
	if e.Disabled {
		tag = "disabled"
		name = theme.Dim().Render(compactSkillPath(e, grouped))
	}
	return alignRow(name, theme.Dim().Render(tag), width)
}

// compactSkillPath returns a short display path for list rows.
// Strips a tracked skill's repo dir, and the first segment of any row
// under a group heading, which shows it; then shows at most 2 trailing
// segments. The full path is in the detail panel.
func compactSkillPath(e skillEntry, grouped bool) string {
	full := baseSkillPath(e)
	segments := strings.Split(full, "/")
	if (grouped || e.RepoName != "") && len(segments) > 1 {
		segments = segments[1:]
	}

	if len(segments) > 2 {
		segments = segments[len(segments)-2:]
	}
	return strings.Join(segments, "/")
}

func baseSkillPath(e skillEntry) string {
	if e.RelPath != "" {
		return e.RelPath
	}
	return e.Name
}

func skillTypeBadge(e skillEntry) string {
	var badge string
	if e.RepoName == "" && e.Source == "" {
		badge = theme.Badge().Render("local")
	}
	if e.Disabled {
		disabled := theme.Badge().Faint(true).Render("disabled")
		if badge != "" {
			return badge + "  " + disabled
		}
		return disabled
	}
	return badge
}

// colorSkillPath renders a skill path with progressive luminance:
// top-level group → cyan, sub-dirs → dark gray..light gray, skill name → bright white.
func colorSkillPath(path string) string {
	segments := strings.Split(path, "/")
	if len(segments) <= 1 {
		return theme.Primary().Render(path)
	}

	dirs := segments[:len(segments)-1]
	name := segments[len(segments)-1]

	var parts []string
	for idx, dir := range dirs {
		if idx == 0 {
			parts = append(parts, theme.Accent().Render(dir))
		} else {
			parts = append(parts, theme.Dim().Render(dir))
		}
	}

	sep := theme.Dim().Render("/")
	return strings.Join(parts, sep) + sep + theme.Primary().Render(name)
}

// colorSkillPathBold is like colorSkillPath but renders the skill name in bold
// for extra prominence in the detail panel header.
func colorSkillPathBold(path string) string {
	segments := strings.Split(path, "/")
	boldName := theme.Primary().Bold(true)
	if len(segments) <= 1 {
		return boldName.Render(path)
	}

	dirs := segments[:len(segments)-1]
	name := segments[len(segments)-1]

	var parts []string
	for idx, dir := range dirs {
		if idx == 0 {
			parts = append(parts, theme.Accent().Render(dir))
		} else {
			parts = append(parts, theme.Dim().Render(dir))
		}
	}

	sep := theme.Dim().Render("/")
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

// buildGroupedItems inserts groupItem separators before each top-level group.
// Skills must be sorted by RelPath (tracked repos with "_" prefix sort first).
// Grouping follows skillTopGroup(): tracked entries group by their repo root;
// local nested entries group by their first path segment; flat locals fall
// into "standalone". When items contain mixed kinds (skills + agents), the
// kind is included in the key so they stay in separate blocks.
func buildGroupedItems(skills []skillItem) []list.Item {
	// Check if there are multiple groups.
	groups := map[string]bool{}
	hasMultiKinds := false
	for _, s := range skills {
		groups[s.entry.Kind+"\x00"+skillTopGroup(s.entry)] = true
		if !hasMultiKinds && len(skills) > 0 && s.entry.Kind != skills[0].entry.Kind {
			hasMultiKinds = true
		}
	}

	if len(groups) <= 1 {
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
		top := skillTopGroup(s.entry)
		key := s.entry.Kind + "\x00" + top
		if key != currentGroup {
			flush()
			label := "standalone"
			if top != "" {
				label = strings.TrimPrefix(top, "_")
			}
			// Prefix with kind when mixed to visually separate skills/agents
			if hasMultiKinds {
				kindPrefix := "Skills"
				if s.entry.Kind == "agent" {
					kindPrefix = "Agents"
				}
				label = kindPrefix + " · " + label
			}
			items = append(items, groupItem{label: label})
			currentGroup = key
			groupCount = 0
		}
		s.grouped = true
		items = append(items, s)
		groupCount++
	}
	flush()
	return items
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
