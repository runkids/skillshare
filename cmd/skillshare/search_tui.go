package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"skillshare/internal/search"
)

// ---------------------------------------------------------------------------
// Search select TUI — multi-select with checkboxes
// ---------------------------------------------------------------------------

// searchSelectOutcome represents the outcome of the search select TUI.
type searchSelectOutcome int

const (
	searchSelectNone        searchSelectOutcome = iota // esc / cancel
	searchSelectInstall                                // enter with selection
	searchSelectSearchAgain                            // s key
)

// searchSelectResult holds the TUI result: selected items and whether to search again.
type searchSelectResult struct {
	selected    []search.SearchResult
	searchAgain bool
}

// searchSelectItem is a list item for the search multi-select TUI.
// Title() returns plain text — no inline ANSI — so bubbles filter highlighting works correctly.
type searchSelectItem struct {
	idx      int
	result   search.SearchResult
	isHub    bool
	selected bool
}

func (i searchSelectItem) Title() string {
	check := "[ ]"
	if i.selected {
		check = "[x]"
	}
	title := check + " " + i.result.Name
	if i.isHub {
		badge := formatRiskBadgePlain(i.result.RiskLabel)
		if badge != "" {
			title += badge
		}
	} else {
		stars := search.FormatStars(i.result.Stars)
		title += " ★ " + stars
	}
	return title
}

func (i searchSelectItem) Description() string {
	var parts []string
	parts = append(parts, i.result.Source)
	if len(i.result.Tags) > 0 {
		tags := make([]string, len(i.result.Tags))
		for j, tag := range i.result.Tags {
			tags[j] = "#" + tag
		}
		parts = append(parts, strings.Join(tags, " "))
	}
	return strings.Join(parts, "  ")
}

func (i searchSelectItem) FilterValue() string {
	parts := []string{i.result.Name}
	if i.result.Description != "" {
		parts = append(parts, i.result.Description)
	}
	for _, tag := range i.result.Tags {
		parts = append(parts, tag)
	}
	return strings.Join(parts, " ")
}

// searchSelectModel is the bubbletea model for search multi-select.
type searchSelectModel struct {
	list      list.Model
	results   []search.SearchResult
	isHub     bool
	selected  map[int]bool
	selCount  int
	total     int
	outcome   searchSelectOutcome
	quitting  bool
	termWidth int

	// Application-level filter (matches list_tui pattern)
	allItems    []searchSelectItem
	filterText  string
	filterInput textinput.Model
	filtering   bool
	matchCount  int
}

func newSearchSelectModel(results []search.SearchResult, isHub bool) searchSelectModel {
	sel := make(map[int]bool, len(results))
	items := makeSearchSelectItems(results, isHub, sel)

	// Keep typed allItems for filter
	allItems := make([]searchSelectItem, len(items))
	for i, item := range items {
		allItems[i] = item.(searchSelectItem)
	}

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	delegate.SetSpacing(0)
	delegate.Styles.NormalTitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).PaddingLeft(2)
	delegate.Styles.NormalDesc = lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).PaddingLeft(2)
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().Bold(true).
		Foreground(tuiBrandYellow).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(tuiBrandYellow).PaddingLeft(1)
	delegate.Styles.SelectedDesc = lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(tuiBrandYellow).PaddingLeft(1)

	l := list.New(items, delegate, 0, 0)
	l.Title = searchSelectTitle(0, len(results))
	l.Styles.Title = lipgloss.NewStyle().
		Bold(true).Foreground(lipgloss.Color("6"))
	l.SetShowStatusBar(false)    // custom status line
	l.SetFilteringEnabled(false) // application-level filter
	l.SetShowHelp(false)
	l.SetShowPagination(false) // page info in custom status line

	// Filter text input
	fi := textinput.New()
	fi.Prompt = "/ "
	fi.PromptStyle = tuiFilterStyle
	fi.Cursor.Style = tuiFilterStyle

	return searchSelectModel{
		list:        l,
		results:     results,
		isHub:       isHub,
		selected:    sel,
		total:       len(results),
		allItems:    allItems,
		matchCount:  len(allItems),
		filterInput: fi,
	}
}

func searchSelectTitle(n, total int) string {
	return fmt.Sprintf("Select skills to install (%d/%d selected)", n, total)
}

func makeSearchSelectItems(results []search.SearchResult, isHub bool, selected map[int]bool) []list.Item {
	items := make([]list.Item, len(results))
	for i, r := range results {
		items[i] = searchSelectItem{
			idx:      i,
			result:   r,
			isHub:    isHub,
			selected: selected[i],
		}
	}
	return items
}

func (m searchSelectModel) Init() tea.Cmd { return nil }

func (m searchSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.list.SetSize(msg.Width, msg.Height-14)
		return m, nil

	case tea.KeyMsg:
		// --- Filter mode: only handle filter input + esc/enter ---
		if m.filtering {
			switch msg.String() {
			case "esc":
				m.filtering = false
				m.filterText = ""
				m.filterInput.SetValue("")
				m.applySearchFilter()
				return m, nil
			case "enter":
				m.filtering = false
				return m, nil
			}
			var cmd tea.Cmd
			m.filterInput, cmd = m.filterInput.Update(msg)
			newVal := m.filterInput.Value()
			if newVal != m.filterText {
				m.filterText = newVal
				m.applySearchFilter()
			}
			return m, cmd
		}

		// --- Normal mode ---
		switch msg.String() {
		case " ": // space — toggle current item
			item, ok := m.list.SelectedItem().(searchSelectItem)
			if !ok {
				break
			}
			m.selected[item.idx] = !m.selected[item.idx]
			if m.selected[item.idx] {
				m.selCount++
			} else {
				m.selCount--
			}
			m.refreshItems()
			return m, nil

		case "a": // toggle all visible
			selectAll := m.selCount < m.total
			for i := 0; i < m.total; i++ {
				m.selected[i] = selectAll
			}
			if selectAll {
				m.selCount = m.total
			} else {
				m.selCount = 0
			}
			m.refreshItems()
			return m, nil

		case "enter": // confirm — install if any selected, else cancel
			if m.selCount == 0 {
				m.outcome = searchSelectNone
			} else {
				m.outcome = searchSelectInstall
			}
			m.quitting = true
			return m, tea.Quit

		case "s": // search again
			m.outcome = searchSelectSearchAgain
			m.quitting = true
			return m, tea.Quit

		case "/":
			m.filtering = true
			m.filterInput.Focus()
			return m, textinput.Blink

		case "q", "ctrl+c", "esc":
			m.outcome = searchSelectNone
			m.quitting = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *searchSelectModel) refreshItems() {
	cursor := m.list.Index()
	// Rebuild allItems with current checkbox state
	for i := range m.allItems {
		m.allItems[i].selected = m.selected[m.allItems[i].idx]
	}
	// Apply filter if active, otherwise show all
	if m.filterText != "" {
		m.applySearchFilter()
	} else {
		items := make([]list.Item, len(m.allItems))
		for i, item := range m.allItems {
			items[i] = item
		}
		m.list.SetItems(items)
	}
	if cursor < len(m.list.Items()) {
		m.list.Select(cursor)
	}
	m.list.Title = searchSelectTitle(m.selCount, m.total)
}

// applySearchFilter does a case-insensitive substring match over allItems,
// preserving checkbox state from m.selected.
func (m *searchSelectModel) applySearchFilter() {
	term := strings.ToLower(m.filterText)

	if term == "" {
		items := make([]list.Item, len(m.allItems))
		for i := range m.allItems {
			m.allItems[i].selected = m.selected[m.allItems[i].idx]
			items[i] = m.allItems[i]
		}
		m.matchCount = len(m.allItems)
		m.list.SetItems(items)
		m.list.ResetSelected()
		return
	}

	var matched []list.Item
	for _, item := range m.allItems {
		if strings.Contains(strings.ToLower(item.FilterValue()), term) {
			item.selected = m.selected[item.idx]
			matched = append(matched, item)
		}
	}
	m.matchCount = len(matched)
	m.list.SetItems(matched)
	m.list.ResetSelected()
}

func (m searchSelectModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder
	b.WriteString(m.list.View())
	b.WriteString("\n\n")

	// Filter bar (always visible)
	b.WriteString(m.renderSearchFilterBar())

	// Detail panel for selected item
	if item, ok := m.list.SelectedItem().(searchSelectItem); ok {
		b.WriteString(m.renderSearchDetailPanel(item.result))
	}

	help := "↑↓ navigate  ←→ page  space toggle  a all  enter install  s search again  / filter  esc cancel"
	b.WriteString(tuiHelpStyle.Render(help))
	b.WriteString("\n")

	return b.String()
}

// renderSearchFilterBar renders the status line for the search TUI.
func (m searchSelectModel) renderSearchFilterBar() string {
	return renderTUIFilterBar(
		m.filterInput.View(), m.filtering, m.filterText,
		m.matchCount, len(m.allItems), 0,
		"skills", renderPageInfoFromPaginator(m.list.Paginator),
	)
}

// renderSearchDetailPanel renders the detail section for the selected search result.
func (m searchSelectModel) renderSearchDetailPanel(r search.SearchResult) string {
	var b strings.Builder
	b.WriteString(tuiSeparatorStyle.Render("  ─────────────────────────────────────────"))
	b.WriteString("\n")

	row := func(label, value string) {
		b.WriteString("  ")
		b.WriteString(tuiDetailLabelStyle.Render(label))
		b.WriteString(tuiDetailValueStyle.Render(value))
		b.WriteString("\n")
	}

	// Description — word-wrapped
	if r.Description != "" {
		const labelOffset = 16
		maxWidth := m.termWidth - labelOffset
		if maxWidth < 40 {
			maxWidth = 40
		}
		lines := wordWrapLines(r.Description, maxWidth)
		const maxDescLines = 3
		truncated := len(lines) > maxDescLines
		if truncated {
			lines = lines[:maxDescLines]
			lines[maxDescLines-1] += "..."
		}
		row("Description:", lines[0])
		indent := strings.Repeat(" ", labelOffset)
		for _, line := range lines[1:] {
			b.WriteString(indent)
			b.WriteString(tuiDetailValueStyle.Render(line))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Source
	row("Source:", r.Source)

	// Stars (non-hub)
	if !m.isHub {
		row("Stars:", search.FormatStars(r.Stars))
	}

	// Risk (hub)
	if m.isHub && r.RiskLabel != "" {
		row("Risk:", r.RiskLabel)
	}

	// Tags
	if len(r.Tags) > 0 {
		tags := make([]string, len(r.Tags))
		for i, tag := range r.Tags {
			tags[i] = "#" + tag
		}
		row("Tags:", tuiTargetStyle.Render(strings.Join(tags, "  ")))
	}

	return b.String()
}

// runSearchSelectTUI starts the search multi-select TUI.
// Returns (searchSelectResult, error).
func runSearchSelectTUI(results []search.SearchResult, isHub bool) (searchSelectResult, error) {
	model := newSearchSelectModel(results, isHub)
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return searchSelectResult{}, err
	}

	m, ok := finalModel.(searchSelectModel)
	if !ok {
		return searchSelectResult{}, nil
	}

	switch m.outcome {
	case searchSelectSearchAgain:
		return searchSelectResult{searchAgain: true}, nil
	case searchSelectInstall:
		selected := make([]search.SearchResult, 0, m.selCount)
		for i, r := range m.results {
			if m.selected[i] {
				selected = append(selected, r)
			}
		}
		return searchSelectResult{selected: selected}, nil
	default:
		return searchSelectResult{}, nil
	}
}
