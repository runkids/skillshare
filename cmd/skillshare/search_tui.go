package main

import (
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"skillshare/internal/search"
	"skillshare/internal/theme"
)

// ---------------------------------------------------------------------------
// Search select TUI — multi-select with checkboxes
// ---------------------------------------------------------------------------

// searchSelectOutcome represents the outcome of the search select TUI.
type searchSelectOutcome int

const (
	searchSelectNone        searchSelectOutcome = iota // q / cancel
	searchSelectInstall                                // enter
	searchSelectSearchAgain                            // esc: back to the keyword
)

// searchSelectResult holds the TUI result: selected items and whether to search again.
type searchSelectResult struct {
	selected    []search.SearchResult
	searchAgain bool
}

// searchSelectItem is a list item for the search multi-select TUI.
type searchSelectItem struct {
	idx      int
	result   search.SearchResult
	isHub    bool
	selected bool
}

// meta is what the row shows at the right: the audit risk for an index,
// the stars for GitHub.
func (i searchSelectItem) meta() string {
	if i.isHub {
		if i.result.RiskLabel == "" {
			return ""
		}
		return theme.RiskLabelStyle(i.result.RiskLabel).Render(i.result.RiskLabel)
	}
	return theme.Dim().Render("★ " + search.FormatStars(i.result.Stars))
}

func (i searchSelectItem) FilterValue() string {
	parts := []string{i.result.Name}
	if i.result.Description != "" {
		parts = append(parts, i.result.Description)
	}
	for _, tag := range i.result.Tags {
		parts = append(parts, tag)
	}
	if i.isHub && i.result.RiskLabel != "" {
		parts = append(parts, i.result.RiskLabel)
	}
	return strings.Join(parts, " ")
}

// searchDelegate renders "○ name" with the stars or risk at the right.
type searchDelegate struct{}

func (searchDelegate) Height() int                             { return 1 }
func (searchDelegate) Spacing() int                            { return 0 }
func (searchDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (searchDelegate) Render(w io.Writer, m list.Model, index int, li list.Item) {
	item, ok := li.(searchSelectItem)
	if !ok {
		return
	}
	mark := theme.Dim().Render("○")
	if item.selected {
		mark = theme.Accent().Render("◉")
	}
	renderPrefixRow(w, alignRow(mark+" "+item.result.Name, item.meta(), m.Width()-rowIndent), m.Width(), index == m.Index())
}

// searchSelectModel is the bubbletea model for search multi-select.
type searchSelectModel struct {
	list                  list.Model
	results               []search.SearchResult
	query                 string
	isHub                 bool
	selected              map[int]bool
	selCount              int
	total                 int
	outcome               searchSelectOutcome
	quitting              bool
	termWidth, termHeight int
	detailScroll          int
	showKeys              bool

	// Application-level filter (matches list_tui pattern)
	allItems    []searchSelectItem
	filterText  string
	filterInput textinput.Model
	filtering   bool
	matchCount  int
}

func newSearchSelectModel(results []search.SearchResult, query string, isHub bool) searchSelectModel {
	sel := make(map[int]bool, len(results))
	items := makeSearchSelectItems(results, isHub, sel)

	// Keep typed allItems for filter
	allItems := make([]searchSelectItem, len(items))
	for i, item := range items {
		allItems[i] = item.(searchSelectItem)
	}

	l := list.New(items, searchDelegate{}, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)    // the title line has the counts
	l.SetFilteringEnabled(false) // application-level filter
	l.SetShowHelp(false)
	l.SetShowPagination(false)

	m := searchSelectModel{
		list:        l,
		results:     results,
		query:       query,
		isHub:       isHub,
		selected:    sel,
		total:       len(results),
		allItems:    allItems,
		matchCount:  len(allItems),
		filterInput: newTUIFilterInput("type to match a name, description or tag"),
	}
	m.resize(120, 30)
	return m
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

// resize sizes the list for the terminal.
func (m *searchSelectModel) resize(width, height int) {
	m.termWidth, m.termHeight = width, height
	bodyHeight := max(height-frameChrome, 6)
	if width < tuiMinSplitWidth {
		m.list.SetSize(width, max(bodyHeight/2, 4))
		return
	}
	m.list.SetSize(listPanelWidth(width), bodyHeight)
}

func (m searchSelectModel) Init() tea.Cmd { return nil }

func (m searchSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		// --- Filter mode: only handle filter input + esc/enter ---
		if m.filtering {
			cmd := handleTUIFilterKey(msg, &m.filtering, &m.filterText, &m.filterInput, m.applySearchFilter)
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

		case "a": // toggle all
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

		case "enter": // install the selection, or the row under the cursor
			if m.selCount == 0 {
				item, ok := m.list.SelectedItem().(searchSelectItem)
				if !ok {
					return m, nil
				}
				m.selected[item.idx] = true
				m.selCount = 1
			}
			m.outcome = searchSelectInstall
			m.quitting = true
			return m, tea.Quit

		case "?":
			m.showKeys = !m.showKeys
			return m, nil

		case "/":
			m.filtering = true
			m.filterInput.Focus()
			return m, textinput.Blink

		case "ctrl+d":
			m.detailScroll += 5
			return m, nil

		case "ctrl+u":
			m.detailScroll = max(m.detailScroll-5, 0)
			return m, nil

		case "esc":
			switch {
			case m.showKeys:
				m.showKeys = false
			case m.filterText != "":
				m.filterText = ""
				m.filterInput.SetValue("")
				m.applySearchFilter()
			default:
				m.outcome = searchSelectSearchAgain
				m.quitting = true
				return m, tea.Quit
			}
			return m, nil

		case "q", "ctrl+c":
			m.outcome = searchSelectNone
			m.quitting = true
			return m, tea.Quit
		}
	}

	prev := m.list.Index()
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	if m.list.Index() != prev {
		m.detailScroll = 0
	}
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
}

// applySearchFilter does a case-insensitive substring match over allItems,
// preserving checkbox state from m.selected.
func (m *searchSelectModel) applySearchFilter() {
	term := strings.ToLower(m.filterText)

	var matched []list.Item
	for _, item := range m.allItems {
		if term == "" || strings.Contains(strings.ToLower(item.FilterValue()), term) {
			item.selected = m.selected[item.idx]
			matched = append(matched, item)
		}
	}
	m.matchCount = len(matched)
	m.list.SetItems(matched)
	m.list.ResetSelected()
	m.detailScroll = 0
}

func (m searchSelectModel) View() string {
	if m.quitting {
		return ""
	}
	bodyHeight := max(m.termHeight-frameChrome, 6)
	title := m.renderTitleLine()
	if m.termWidth < tuiMinSplitWidth {
		detailHeight := max(bodyHeight-m.list.Height()-1, 4)
		detail := lipgloss.NewStyle().Height(detailHeight).MaxHeight(detailHeight).PaddingLeft(1).
			Render(m.renderRight(m.termWidth-2, detailHeight))
		return title + "\n\n" + m.list.View() + "\n\n" + detail + "\n" + m.renderBottom()
	}
	leftWidth := listPanelWidth(m.termWidth)
	rightWidth := m.termWidth - leftWidth
	return title + "\n\n" +
		renderFrameSplit(m.list.View(), m.renderRight(rightWidth-2, bodyHeight), leftWidth, rightWidth, bodyHeight) + "\n" +
		m.renderBottom()
}

// renderTitleLine renders the keyword, where it searched, and the counts.
func (m searchSelectModel) renderTitleLine() string {
	var facts []string
	if m.query != "" {
		facts = append(facts, `"`+m.query+`"`)
	}
	where := "GitHub"
	if m.isHub {
		where = "index"
	}
	count := countNoun(m.total, "result")
	if m.filterText != "" {
		count = formatNumber(m.matchCount) + " of " + count
	}
	facts = append(facts, where, count)
	if m.selCount > 0 {
		facts = append(facts, theme.Accent().Render(formatNumber(m.selCount)+" selected"))
	}
	return renderFrameTitle(m.termWidth, "search", facts, nil)
}

// renderRight renders the selected result, or the key list while ? is on.
func (m searchSelectModel) renderRight(width, height int) string {
	if m.showKeys {
		return renderKeysPanel(searchKeyGroups)
	}
	item, ok := m.list.SelectedItem().(searchSelectItem)
	if !ok {
		return ""
	}
	detail, _ := wrapAndScroll(m.renderSearchDetailPanel(item.result), width, m.detailScroll, height)
	return detail
}

// renderBottom renders the key line; the filter input takes it over in place.
func (m searchSelectModel) renderBottom() string {
	var line string
	switch {
	case m.filtering:
		line = renderFilterLine(m.termWidth, m.filterInput.View(), m.matchCount)
	case m.showKeys:
		line = renderKeyLine(m.termWidth, []keyHint{{"?/esc", "close"}}, "")
	default:
		back := keyHint{"esc", "new search"}
		if m.filterText != "" {
			back = keyHint{"esc", "clear filter"}
		}
		install := keyHint{"enter", "install"}
		if m.selCount > 0 {
			install.desc = "install " + formatNumber(m.selCount)
		}
		hints := []keyHint{{"↑↓", "move"}, {"space", "select"}, install, {"/", "filter"}, back, {"?", "keys"}}
		line = renderKeyLine(m.termWidth, hints, framePosition(m.list.Index()+1, m.matchCount))
	}
	return "\n" + line
}

// searchKeyGroups lists every key for the ? panel.
var searchKeyGroups = []keyGroup{
	{"Move", []keyHint{
		{"↑↓", "move"},
		{"←→", "page"},
		{"/", "filter the results"},
		{"ctrl+d/u", "scroll the details"},
		{"esc", "clear the filter, then search again"},
		{"q", "quit"},
	}},
	{"Install", []keyHint{
		{"space", "select"},
		{"a", "select all, or none"},
		{"enter", "install the selection, or the row under the cursor"},
	}},
}

// renderSearchDetailPanel renders the detail section for the selected search result.
func (m searchSelectModel) renderSearchDetailPanel(r search.SearchResult) string {
	var b strings.Builder
	row := func(label, value string) {
		b.WriteString(theme.Dim().Width(8).Render(label))
		b.WriteString(value)
		b.WriteString("\n")
	}

	b.WriteString(theme.Primary().Bold(true).Render(r.Name))
	b.WriteString("\n\n")
	if r.Description != "" {
		b.WriteString(r.Description)
		b.WriteString("\n\n")
	}

	row("Source", shortenPath(r.Source))
	if !m.isHub {
		row("Stars", search.FormatStars(r.Stars))
	}
	if m.isHub && r.RiskLabel != "" {
		row("Risk", theme.RiskLabelStyle(r.RiskLabel).Render(r.RiskLabel))
	}
	if len(r.Tags) > 0 {
		tags := make([]string, len(r.Tags))
		for i, tag := range r.Tags {
			tags[i] = "#" + tag
		}
		row("Tags", theme.Accent().Render(strings.Join(tags, "  ")))
	}
	return b.String()
}

// runSearchSelectTUI starts the search multi-select TUI.
// Returns (searchSelectResult, error).
func runSearchSelectTUI(results []search.SearchResult, query string, isHub bool) (searchSelectResult, error) {
	model := newSearchSelectModel(results, query, isHub)
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
