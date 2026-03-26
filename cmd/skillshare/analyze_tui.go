package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	ssync "skillshare/internal/sync"
)

// analyzeTargetGroup merges targets with identical skill sets into one entry.
type analyzeTargetGroup struct {
	entry analyzeTargetEntry // representative entry (skills, counts, tokens)
	names []string           // target names in this group (e.g. ["claude", "cursor"])
}

type analyzeTUIModel struct {
	list        list.Model
	allItems    []analyzeSkillItem
	filterText  string
	filterInput textinput.Model
	filtering   bool
	matchCount  int

	groups   []analyzeTargetGroup
	groupIdx int

	sortBy  string // "tokens" | "name"
	sortAsc bool

	thresholdHigh int
	thresholdLow  int

	detailScroll int

	termWidth   int
	termHeight  int
	quitting    bool
	loading     bool
	loadSpinner spinner.Model
	loadFn      func() analyzeLoadResult
	loadErr     error
	modeLabel   string
}

type analyzeDataLoadedMsg struct {
	result analyzeLoadResult
}

func newAnalyzeTUIModel(loadFn func() analyzeLoadResult, modeLabel string) analyzeTUIModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = tc.SpinnerStyle

	fi := textinput.New()
	fi.Prompt = "/ "
	fi.PromptStyle = tc.Filter
	fi.Cursor.Style = tc.Filter
	fi.Placeholder = "filter skills"

	return analyzeTUIModel{
		loading:     true,
		loadFn:      loadFn,
		loadSpinner: sp,
		modeLabel:   modeLabel,
		sortBy:      "tokens",
		filterInput: fi,
	}
}

func (m analyzeTUIModel) Init() tea.Cmd {
	if m.loading && m.loadFn != nil {
		fn := m.loadFn
		return tea.Batch(m.loadSpinner.Tick, func() tea.Msg {
			return analyzeDataLoadedMsg{result: fn()}
		})
	}
	return nil
}

// groupAnalyzeTargets merges targets with identical skill sets into groups.
// Targets with the same (SkillCount, AlwaysLoaded, OnDemandMax) share the same
// skills (no include/exclude filters differentiating them) and are grouped together.
func groupAnalyzeTargets(entries []analyzeTargetEntry) []analyzeTargetGroup {
	type key struct {
		skillCount int
		alwaysChar int
		onDemChar  int
	}
	order := []key{}
	groups := map[key]*analyzeTargetGroup{}
	for _, e := range entries {
		k := key{e.SkillCount, e.AlwaysLoaded.Chars, e.OnDemandMax.Chars}
		if g, ok := groups[k]; ok {
			g.names = append(g.names, e.Name)
		} else {
			order = append(order, k)
			groups[k] = &analyzeTargetGroup{
				entry: e,
				names: []string{e.Name},
			}
		}
	}
	result := make([]analyzeTargetGroup, 0, len(order))
	for _, k := range order {
		result = append(result, *groups[k])
	}
	return result
}

func (m *analyzeTUIModel) switchTarget() {
	if len(m.groups) == 0 {
		return
	}
	g := m.groups[m.groupIdx]
	maxTokens := 0
	for _, s := range g.entry.Skills {
		if s.DescriptionTokens > maxTokens {
			maxTokens = s.DescriptionTokens
		}
	}
	items := make([]analyzeSkillItem, len(g.entry.Skills))
	for i, s := range g.entry.Skills {
		items[i] = analyzeSkillItem{entry: s, maxTokens: maxTokens}
	}
	m.allItems = items
	m.recomputeThresholds()
	m.updateDelegate()
	m.applyFilter()
}

func (m *analyzeTUIModel) updateDelegate() {
	m.list.SetDelegate(analyzeSkillDelegate{
		thresholdLow:  m.thresholdLow,
		thresholdHigh: m.thresholdHigh,
	})
}

func (m *analyzeTUIModel) recomputeThresholds() {
	tokens := make([]int, len(m.allItems))
	for i, item := range m.allItems {
		tokens[i] = item.entry.DescriptionTokens
	}
	m.thresholdLow, m.thresholdHigh = computeThresholds(tokens)
}

func (m *analyzeTUIModel) applyFilter() {
	m.detailScroll = 0

	items := m.allItems
	if m.filterText != "" {
		lower := strings.ToLower(m.filterText)
		var filtered []analyzeSkillItem
		for _, item := range m.allItems {
			if strings.Contains(strings.ToLower(item.entry.Name), lower) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}

	m.sortItems(items)
	m.matchCount = len(items)

	listItems := make([]list.Item, len(items))
	for i, item := range items {
		listItems[i] = item
	}
	m.list.SetItems(listItems)
	m.list.ResetSelected()
}

func (m *analyzeTUIModel) sortItems(items []analyzeSkillItem) {
	switch m.sortBy {
	case "tokens":
		if m.sortAsc {
			sortAnalyzeItems(items, func(a, b analyzeSkillItem) bool {
				return a.entry.DescriptionTokens < b.entry.DescriptionTokens
			})
		} else {
			sortAnalyzeItems(items, func(a, b analyzeSkillItem) bool {
				return a.entry.DescriptionTokens > b.entry.DescriptionTokens
			})
		}
	case "name":
		if m.sortAsc {
			sortAnalyzeItems(items, func(a, b analyzeSkillItem) bool {
				return a.entry.Name < b.entry.Name
			})
		} else {
			sortAnalyzeItems(items, func(a, b analyzeSkillItem) bool {
				return a.entry.Name > b.entry.Name
			})
		}
	}
}

func sortAnalyzeItems(items []analyzeSkillItem, less func(a, b analyzeSkillItem) bool) {
	sort.Slice(items, func(i, j int) bool { return less(items[i], items[j]) })
}

func (m *analyzeTUIModel) cycleSort() {
	switch {
	case m.sortBy == "tokens" && !m.sortAsc:
		m.sortAsc = true // tokens ↑
	case m.sortBy == "tokens" && m.sortAsc:
		m.sortBy = "name"
		m.sortAsc = true // name A→Z
	case m.sortBy == "name" && m.sortAsc:
		m.sortAsc = false // name Z→A
	default:
		m.sortBy = "tokens"
		m.sortAsc = false // tokens ↓ (default)
	}
	m.applyFilter()
}

func (m analyzeTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		m.syncListSize()
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.loadSpinner, cmd = m.loadSpinner.Update(msg)
			return m, cmd
		}

	case analyzeDataLoadedMsg:
		m.loading = false
		m.loadFn = nil
		if msg.result.err != nil {
			m.loadErr = msg.result.err
			m.quitting = true
			return m, tea.Quit
		}
		if len(msg.result.targets) == 0 {
			m.quitting = true
			return m, tea.Quit
		}
		m.groups = groupAnalyzeTargets(msg.result.targets)
		m.groupIdx = 0
		delegate := analyzeSkillDelegate{}
		l := list.New(nil, delegate, 0, 0)
		l.Title = m.listTitle()
		l.Styles.Title = tc.ListTitle
		l.SetShowStatusBar(false)
		l.SetFilteringEnabled(false)
		l.SetShowHelp(false)
		l.SetShowPagination(false)
		m.list = l
		m.switchTarget()
		m.syncListSize()
		return m, nil

	case tea.MouseMsg:
		if listSplitActive(m.termWidth) && !m.loading {
			leftWidth := listPanelWidth(m.termWidth)
			if msg.X > leftWidth {
				switch msg.Button {
				case tea.MouseButtonWheelUp:
					if m.detailScroll > 0 {
						m.detailScroll--
					}
					return m, nil
				case tea.MouseButtonWheelDown:
					m.detailScroll++
					return m, nil
				}
			}
		}

	case tea.KeyMsg:
		if m.loading {
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				m.quitting = true
				return m, tea.Quit
			}
			return m, nil
		}

		if m.filtering {
			switch msg.String() {
			case "esc":
				m.filtering = false
				m.filterText = ""
				m.filterInput.SetValue("")
				m.applyFilter()
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
				m.applyFilter()
			}
			return m, cmd
		}

		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "tab":
			if len(m.groups) > 1 {
				m.groupIdx = (m.groupIdx + 1) % len(m.groups)
				m.switchTarget()
				m.list.Title = m.listTitle()
			}
			return m, nil
		case "shift+tab":
			if len(m.groups) > 1 {
				m.groupIdx = (m.groupIdx - 1 + len(m.groups)) % len(m.groups)
				m.switchTarget()
				m.list.Title = m.listTitle()
			}
			return m, nil
		case "s":
			m.cycleSort()
			return m, nil
		case "/":
			m.filtering = true
			m.filterInput.Focus()
			return m, textinput.Blink
		case "ctrl+d":
			m.detailScroll += 8
			return m, nil
		case "ctrl+u":
			m.detailScroll -= 8
			if m.detailScroll < 0 {
				m.detailScroll = 0
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	prevSelected := m.selectedKey()
	m.list, cmd = m.list.Update(msg)
	if m.selectedKey() != prevSelected {
		m.detailScroll = 0
	}
	return m, cmd
}

func (m analyzeTUIModel) selectedKey() string {
	item, ok := m.list.SelectedItem().(analyzeSkillItem)
	if !ok {
		return ""
	}
	return item.entry.Name
}

func (m *analyzeTUIModel) syncListSize() {
	if m.loading {
		return
	}
	if listSplitActive(m.termWidth) {
		panelHeight := m.termHeight - 9
		if panelHeight < 6 {
			panelHeight = 6
		}
		m.list.SetSize(listPanelWidth(m.termWidth), panelHeight)
		return
	}
	listHeight := m.termHeight - 20
	if listHeight < 6 {
		listHeight = 6
	}
	m.list.SetSize(m.termWidth, listHeight)
}

func (m analyzeTUIModel) listTitle() string {
	if len(m.groups) == 0 {
		return "Context Analysis"
	}
	g := m.groups[m.groupIdx]
	if len(g.names) == 1 {
		return g.names[0]
	}
	return fmt.Sprintf("%d targets", len(g.names))
}

func (m analyzeTUIModel) View() string {
	if m.quitting {
		return ""
	}
	if m.loading {
		return fmt.Sprintf("\n  %s Loading skills...\n", m.loadSpinner.View())
	}
	if listSplitActive(m.termWidth) {
		return m.viewSplit()
	}
	return m.viewVertical()
}

func (m analyzeTUIModel) viewSplit() string {
	var b strings.Builder

	panelHeight := m.termHeight - 9
	if panelHeight < 6 {
		panelHeight = 6
	}

	leftWidth := listPanelWidth(m.termWidth)
	rightWidth := listDetailPanelWidth(m.termWidth)

	var detailStr, scrollInfo string
	if item, ok := m.list.SelectedItem().(analyzeSkillItem); ok {
		header := m.renderDetailHeader(item.entry, rightWidth-1)
		bodyHeight := panelHeight - lipgloss.Height(header) - 2
		if bodyHeight < 4 {
			bodyHeight = 4
		}
		body, bodyScrollInfo := wrapAndScroll(m.renderDetailBody(item.entry, rightWidth-1), rightWidth-1, m.detailScroll, bodyHeight)
		scrollInfo = bodyScrollInfo
		detailStr = "\n" + header + "\n\n" + body
	}

	body := renderHorizontalSplit(m.list.View(), detailStr, leftWidth, rightWidth, panelHeight)
	b.WriteString(body)
	b.WriteString("\n\n")
	b.WriteString(m.renderFilterBar())
	b.WriteString(m.renderFooter(scrollInfo))

	return b.String()
}

func (m analyzeTUIModel) viewVertical() string {
	var b strings.Builder
	b.WriteString(m.list.View())
	b.WriteString("\n\n")
	b.WriteString(m.renderFilterBar())

	var scrollInfo string
	if item, ok := m.list.SelectedItem().(analyzeSkillItem); ok {
		detailHeight := m.termHeight - m.termHeight*2/5 - 10
		if detailHeight < 6 {
			detailHeight = 6
		}
		header := m.renderDetailHeader(item.entry, m.termWidth)
		bodyHeight := detailHeight - lipgloss.Height(header) - 1
		if bodyHeight < 4 {
			bodyHeight = 4
		}
		body, bodyScrollInfo := wrapAndScroll(m.renderDetailBody(item.entry, m.termWidth), m.termWidth, m.detailScroll, bodyHeight)
		scrollInfo = bodyScrollInfo
		b.WriteString(header)
		b.WriteString("\n\n")
		b.WriteString(body)
	}

	b.WriteString(m.renderFooter(scrollInfo))

	return b.String()
}

func (m analyzeTUIModel) renderFooter(scrollInfo string) string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(m.renderTargetBar())
	b.WriteString(m.renderStatsLine())
	b.WriteString("\n")
	help := m.helpText()
	help = appendScrollInfo(help, scrollInfo)
	b.WriteString(tc.Help.Render(help))
	b.WriteString("\n")
	return b.String()
}

func (m analyzeTUIModel) renderFilterBar() string {
	return renderTUIFilterBar(
		m.filterInput.View(), m.filtering, m.filterText,
		m.matchCount, len(m.allItems), 0,
		"skills", renderPageInfoFromPaginator(m.list.Paginator),
	)
}

func (m analyzeTUIModel) renderTargetBar() string {
	if len(m.groups) <= 1 {
		return ""
	}
	var parts []string
	for i, g := range m.groups {
		var label string
		if len(g.names) == 1 {
			label = fmt.Sprintf("%s (%d)", g.names[0], g.entry.SkillCount)
		} else {
			label = fmt.Sprintf("%d targets (%d)", len(g.names), g.entry.SkillCount)
		}
		if i == m.groupIdx {
			parts = append(parts, tc.Cyan.Render("► "+label))
		} else {
			parts = append(parts, tc.Dim.Render(label))
		}
	}
	return "  " + strings.Join(parts, tc.Dim.Render("  ·  ")) + "\n"
}

func (m analyzeTUIModel) renderStatsLine() string {
	if len(m.groups) == 0 {
		return ""
	}
	g := m.groups[m.groupIdx]
	return tc.Help.Render(fmt.Sprintf("%d skills  Always: %s tokens  On-demand: %s tokens  %s",
		g.entry.SkillCount,
		formatTokensStr(g.entry.AlwaysLoaded.Chars),
		formatTokensStr(g.entry.OnDemandMax.Chars),
		tc.Dim.Render("(1 token ≈ 4 chars)"),
	)) + "\n"
}

func (m analyzeTUIModel) renderDetailHeader(e analyzeSkillEntry, width int) string {
	title := tc.ListTitle.Render(e.Name)
	return lipgloss.NewStyle().Width(width).Render(title)
}

func (m analyzeTUIModel) renderDetailBody(e analyzeSkillEntry, width int) string {
	var b strings.Builder

	// Token breakdown
	g := m.groups[m.groupIdx]
	pct := 0.0
	if g.entry.AlwaysLoaded.Chars > 0 {
		pct = float64(e.DescriptionChars) / float64(g.entry.AlwaysLoaded.Chars) * 100
	}
	tokenRows := []string{
		renderFactRow("Desc tokens", fmt.Sprintf("%s  (%.0f%%)", formatTokensStr(e.DescriptionChars), pct)),
		renderFactRow("Body tokens", formatTokensStr(e.BodyChars)),
		renderFactRow("Total", formatTokensStr(e.DescriptionChars+e.BodyChars)),
	}
	b.WriteString(renderDetailSection("Tokens", strings.Join(tokenRows, "\n"), width))
	b.WriteString("\n\n")

	if len(e.LintIssues) > 0 {
		var qualityRows []string
		for _, issue := range e.LintIssues {
			var icon string
			if issue.Severity == ssync.LintError {
				icon = tc.Red.Render("✗")
			} else {
				icon = tc.Yellow.Render("⚠")
			}
			qualityRows = append(qualityRows, icon+" "+issue.Message)
		}
		b.WriteString(renderDetailSection("Quality", strings.Join(qualityRows, "\n"), width))
		b.WriteString("\n\n")
	}

	// Metadata
	var metaRows []string
	if e.relPath != "" {
		metaRows = append(metaRows, renderFactRow("Path", tc.Cyan.Render(e.relPath)))
	}
	if e.isTracked {
		metaRows = append(metaRows, renderFactRow("Tracked", tc.Green.Render("✓")))
	}
	if len(e.targetNames) > 0 {
		metaRows = append(metaRows, renderFactRow("Restricted to", strings.Join(e.targetNames, ", ")))
	}
	if len(metaRows) > 0 {
		b.WriteString(renderDetailSection("Details", strings.Join(metaRows, "\n"), width))
		b.WriteString("\n\n")
	}

	// Description preview
	if e.description != "" {
		maxWidth := width - 4
		if maxWidth < 32 {
			maxWidth = 32
		}
		lines := wordWrapLines(e.description, maxWidth)
		const maxLines = 6
		if len(lines) > maxLines {
			lines = lines[:maxLines]
			lines[len(lines)-1] += "..."
		}
		body := strings.Join(lines, "\n")
		b.WriteString(renderDetailSection("Description", tc.Value.Render(body), width))
	}

	return b.String()
}

func (m analyzeTUIModel) helpText() string {
	if m.filtering {
		return "Enter lock  Esc clear  q quit"
	}
	sortLabel := "tokens↓"
	switch {
	case m.sortBy == "tokens" && m.sortAsc:
		sortLabel = "tokens↑"
	case m.sortBy == "name" && m.sortAsc:
		sortLabel = "name↑"
	case m.sortBy == "name" && !m.sortAsc:
		sortLabel = "name↓"
	}
	help := fmt.Sprintf("↑↓ navigate  ←→ page  / filter  s sort(%s)  Ctrl+d/u detail", sortLabel)
	if len(m.groups) > 1 {
		help += "  Tab target"
	}
	help += "  q quit"
	return help
}

func runAnalyzeTUI(loadFn func() analyzeLoadResult, modeLabel string) error {
	model := newAnalyzeTUIModel(loadFn, modeLabel)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	finalModel, err := p.Run()
	if err != nil {
		return err
	}
	m, ok := finalModel.(analyzeTUIModel)
	if !ok {
		return nil
	}
	if m.loadErr != nil {
		return m.loadErr
	}
	return nil
}
