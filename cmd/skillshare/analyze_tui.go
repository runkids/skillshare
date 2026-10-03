package main

import (
	"fmt"
	"sort"
	"strings"

	"skillshare/internal/theme"

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
	list               list.Model
	allItems           []analyzeSkillItem
	filterText         string
	filterInput        textinput.Model
	filtering          bool
	matchCount         int
	filteredDescTokens int
	filteredBodyTokens int

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

	initialFilter string

	showKeys bool // ? swaps the detail panel for the full key list

	browser *fileBrowser // enter opens the selected skill's files
	lint    string       // the opened skill's lint issues, shown above the keys
}

type analyzeDataLoadedMsg struct {
	result analyzeLoadResult
}

func newAnalyzeTUIModel(loadFn func() analyzeLoadResult, modeLabel string, initialFilter string) analyzeTUIModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = theme.Accent()

	fi := newTUIFilterInput("type to match a skill")

	return analyzeTUIModel{
		loading:       true,
		loadFn:        loadFn,
		loadSpinner:   sp,
		modeLabel:     modeLabel,
		sortBy:        "tokens",
		filterInput:   fi,
		initialFilter: initialFilter,
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
			if skillMatchesFilter(item.entry, lower) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}

	m.sortItems(items)
	m.matchCount = len(items)

	// Cache filtered sums for renderStatsLine (avoids per-render iteration
	// and ensures correctness across paginated views).
	var descTokens, bodyTokens int
	for _, item := range items {
		descTokens += item.entry.DescriptionTokens
		bodyTokens += item.entry.BodyTokens
	}
	m.filteredDescTokens = descTokens
	m.filteredBodyTokens = bodyTokens

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
		if m.browser != nil {
			m.browser.resize(msg.Width, msg.Height)
		}
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
		l.SetShowTitle(false)
		l.SetShowStatusBar(false)
		l.SetFilteringEnabled(false)
		l.SetShowHelp(false)
		l.SetShowPagination(false)
		m.list = l
		m.switchTarget()
		if m.initialFilter != "" {
			m.filterText = m.initialFilter
			m.filterInput.SetValue(m.initialFilter)
			m.applyFilter()
			m.initialFilter = ""
		}
		m.syncListSize()
		return m, nil

	case tea.MouseMsg:
		if m.browser != nil {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				m.browser.wheel(-1)
			case tea.MouseButtonWheelDown:
				m.browser.wheel(1)
			}
			return m, nil
		}
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
			cmd := handleTUIFilterKey(msg, &m.filtering, &m.filterText, &m.filterInput, m.applyFilter)
			return m, cmd
		}
		if m.browser != nil {
			switch msg.String() {
			case "q", "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			case "esc":
				m.browser = nil
			default:
				m.browser.key(msg.String())
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			if item, ok := m.list.SelectedItem().(analyzeSkillItem); ok {
				m.browser = newFileBrowser("analyze", item.entry.Name, item.entry.path, true, m.termWidth, m.termHeight)
				m.lint = lintNote(item.entry.LintIssues)
			}
			return m, nil
		case "esc":
			switch {
			case m.showKeys:
				m.showKeys = false
			case m.filterText != "":
				m.filterText = ""
				m.filterInput.SetValue("")
				m.applyFilter()
			default:
				m.quitting = true
				return m, tea.Quit
			}
			return m, nil
		case "?":
			m.showKeys = !m.showKeys
			return m, nil
		case "tab":
			if len(m.groups) > 1 {
				m.groupIdx = (m.groupIdx + 1) % len(m.groups)
				m.switchTarget()
			}
			return m, nil
		case "shift+tab":
			if len(m.groups) > 1 {
				m.groupIdx = (m.groupIdx - 1 + len(m.groups)) % len(m.groups)
				m.switchTarget()
			}
			return m, nil
		case "o":
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
	bodyHeight := max(m.termHeight-frameChrome, 6)
	if listSplitActive(m.termWidth) {
		m.list.SetSize(listPanelWidth(m.termWidth), bodyHeight)
		return
	}
	m.list.SetSize(m.termWidth, max(bodyHeight/2, 4))
}

// groupLabel names a target group on its tab: "claude", or "claude +2"
// when several targets load the same skills.
func groupLabel(g analyzeTargetGroup) string {
	if len(g.names) == 1 {
		return g.names[0]
	}
	return fmt.Sprintf("%s +%d", g.names[0], len(g.names)-1)
}

// lintNote names the first lint issue and how many more there are.
func lintNote(issues []ssync.LintIssue) string {
	if len(issues) == 0 {
		return ""
	}
	icon := lintIcon(issues)
	note := icon + theme.Dim().Render(issues[0].Message)
	if len(issues) > 1 {
		note += theme.Dim().Render(fmt.Sprintf(" · %d more in the details", len(issues)-1))
	}
	return note
}

func (m analyzeTUIModel) View() string {
	if m.quitting {
		return ""
	}
	if m.browser != nil {
		return m.browser.view(m.lint, nil)
	}
	title := m.renderTitleLine()
	if m.loading {
		return title + "\n\n" + renderBusyLine(m.termWidth, m.loadSpinner.View(), "Loading skills…")
	}
	bodyHeight := max(m.termHeight-frameChrome, 6)
	if !listSplitActive(m.termWidth) {
		listHeight := max(bodyHeight/2, 4)
		detailHeight := max(bodyHeight-listHeight-1, 4)
		detail := lipgloss.NewStyle().Height(detailHeight).MaxHeight(detailHeight).PaddingLeft(1).
			Render(m.renderRight(m.termWidth-2, detailHeight))
		return title + "\n\n" + m.list.View() + "\n\n" + detail + "\n" + m.renderBottom()
	}
	leftWidth := listPanelWidth(m.termWidth)
	rightWidth := listDetailPanelWidth(m.termWidth)
	return title + "\n\n" +
		renderFrameSplit(m.list.View(), m.renderRight(rightWidth-2, bodyHeight), leftWidth, rightWidth, bodyHeight) + "\n" +
		m.renderBottom()
}

// renderTitleLine renders the scope, the skill count, the always-loaded and
// on-demand tokens, and one tab per target group.
func (m analyzeTUIModel) renderTitleLine() string {
	facts := []string{m.modeLabel}
	if m.loading || len(m.groups) == 0 {
		return renderFrameTitle(m.termWidth, "analyze", facts, nil)
	}
	count := countNoun(len(m.allItems), "skill")
	if m.filterText != "" {
		count = formatNumber(m.matchCount) + " of " + count
	}
	facts = append(facts, count,
		formatTokensStr(m.filteredDescTokens)+" always",
		formatTokensStr(m.filteredBodyTokens)+" on demand")
	var tabs []frameTab
	if len(m.groups) > 1 {
		for i, g := range m.groups {
			tabs = append(tabs, frameTab{groupLabel(g), i == m.groupIdx})
		}
	} else {
		facts = append(facts, groupLabel(m.groups[0]))
	}
	return renderFrameTitle(m.termWidth, "analyze", facts, tabs)
}

// renderRight renders the detail panel, or the key list while ? is on.
func (m analyzeTUIModel) renderRight(width, height int) string {
	if m.showKeys {
		return renderKeysPanel(analyzeKeyGroups)
	}
	item, ok := m.list.SelectedItem().(analyzeSkillItem)
	if !ok {
		return ""
	}
	header := theme.Primary().Bold(true).Render(item.entry.Name)
	body, _ := wrapAndScroll(m.renderDetailBody(item.entry, width), width, m.detailScroll, max(height-2, 4))
	return header + "\n\n" + body
}

// renderBottom renders the note line and the key line; the filter input
// takes over the key line in place.
func (m analyzeTUIModel) renderBottom() string {
	note := "1 token ≈ 4 ASCII chars or 1 CJK char"
	if len(m.groups) > 0 {
		if g := m.groups[m.groupIdx]; len(g.names) > 1 {
			note = strings.Join(g.names, ", ") + " load the same skills · " + note
		}
	}
	note = theme.Dim().Render(truncateANSI("  "+note, m.termWidth))
	var line string
	switch {
	case m.filtering:
		line = renderFilterLine(m.termWidth, m.filterInput.View(), m.matchCount)
	case m.showKeys:
		line = renderKeyLine(m.termWidth, []keyHint{{"?/esc", "close"}}, "")
	default:
		filter := keyHint{"/", "filter"}
		if m.filterText != "" {
			filter = keyHint{"esc", "clear filter"}
		}
		hints := []keyHint{{"↑↓", "move"}, filter, {"o", "sort: " + m.sortLabel()}}
		if len(m.groups) > 1 {
			hints = append(hints, keyHint{"tab", "next target"})
		}
		hints = append(hints, keyHint{"enter", "open files"}, keyHint{"ctrl+d/u", "scroll"}, keyHint{"?", "keys"})
		line = renderKeyLine(m.termWidth, hints, framePosition(m.list.Index()+1, m.matchCount))
	}
	return note + "\n" + line
}

// sortLabel names the current sort order, e.g. "tokens ↓".
func (m analyzeTUIModel) sortLabel() string {
	arrow := "↓"
	if m.sortAsc {
		arrow = "↑"
	}
	return m.sortBy + " " + arrow
}

// analyzeKeyGroups lists every key for the ? panel.
var analyzeKeyGroups = []keyGroup{
	{"Move", []keyHint{
		{"↑↓", "move"},
		{"←→", "page"},
		{"/", "filter"},
		{"tab", "next target"},
		{"ctrl+d/u", "scroll the details"},
		{"esc", "clear the filter, then quit"},
		{"q", "quit"},
	}},
	{"View", []keyHint{
		{"o", "sort by tokens or name, up or down"},
		{"enter", "open the skill's files"},
	}},
}

// renderDetailBody renders the description, the token counts, lint issues
// and where the skill lives.
func (m analyzeTUIModel) renderDetailBody(e analyzeSkillEntry, width int) string {
	var blocks []string
	if e.description != "" {
		lines := wordWrapLines(e.description, max(width, 20))
		const maxLines = 6
		if len(lines) > maxLines {
			lines = lines[:maxLines]
			lines[len(lines)-1] += "…"
		}
		blocks = append(blocks, strings.Join(lines, "\n"))
	}

	var facts []string
	fact := func(label, value string) {
		facts = append(facts, theme.Dim().Render(fmt.Sprintf("%-9s ", label))+value)
	}
	g := m.groups[m.groupIdx]
	always := formatTokensStr(e.DescriptionTokens)
	if g.entry.AlwaysLoaded.EstimatedTokens > 0 {
		pct := float64(e.DescriptionTokens) / float64(g.entry.AlwaysLoaded.EstimatedTokens) * 100
		always += theme.Dim().Render(fmt.Sprintf(" · %.0f%% of always-loaded", pct))
	}
	fact("Always", always)
	fact("On demand", formatTokensStr(e.BodyTokens))
	fact("Total", formatTokensStr(e.DescriptionTokens+e.BodyTokens))
	if e.relPath != "" {
		fact("Path", e.relPath)
	}
	if e.isTracked {
		fact("Source", "tracked")
	}
	if len(e.targetNames) > 0 {
		fact("Only on", strings.Join(e.targetNames, ", "))
	}
	blocks = append(blocks, strings.Join(facts, "\n"))

	if len(e.LintIssues) > 0 {
		rows := []string{theme.Primary().Bold(true).Render("Quality")}
		for _, issue := range e.LintIssues {
			icon := theme.Warning().Render("!")
			if issue.Severity == ssync.LintError {
				icon = theme.Danger().Render("✗")
			}
			rows = append(rows, icon+" "+issue.Message)
		}
		blocks = append(blocks, strings.Join(rows, "\n"))
	}
	return strings.Join(blocks, "\n\n")
}

func runAnalyzeTUI(loadFn func() analyzeLoadResult, modeLabel string, initialFilter string) error {
	model := newAnalyzeTUIModel(loadFn, modeLabel, initialFilter)
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
