package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"skillshare/internal/audit"
	"skillshare/internal/theme"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type auditTab int

const (
	auditTabSkills auditTab = iota
	auditTabAgents
)

func (t auditTab) noun() string {
	if t == auditTabAgents {
		return "agents"
	}
	return "skills"
}

// acSevCount returns severity color for non-zero counts, dim for zero.
func acSevCount(count int, style lipgloss.Style) lipgloss.Style {
	if count == 0 {
		return theme.Dim()
	}
	return style
}

// auditItem implements list.Item for audit TUI.
type auditItem struct {
	result  *audit.Result
	elapsed time.Duration
	kind    string // "skill" or "agent"
}

func (i auditItem) Title() string {
	name := colorSkillPath(compactAuditPath(i.result.SkillName))
	if len(i.result.Findings) == 0 {
		return theme.Success().Render("✓") + " " + name
	}
	if i.result.IsBlocked {
		return theme.Danger().Render("✗") + " " + name
	}
	return theme.Warning().Render("!") + " " + name
}

// compactAuditPath strips tracked repo prefix (first segment starting with "_")
// and keeps at most the last 2 segments.
func compactAuditPath(name string) string {
	segments := strings.Split(name, "/")
	if strings.HasPrefix(segments[0], "_") {
		if len(segments) > 1 {
			segments = segments[1:]
		} else {
			// Repo-root skill: "_repo-name" → "repo-name"
			segments[0] = strings.TrimPrefix(segments[0], "_")
		}
	}
	if len(segments) > 2 {
		segments = segments[len(segments)-2:]
	}
	return strings.Join(segments, "/")
}

// auditRepoKey extracts the grouping key from a skill/agent name.
// For tracked repos: "_repo-name/skill" → "_repo-name"
// For nested agents: "demo/code-reviewer.md" → "demo"
// For flat names: "my-skill" → "" (standalone)
func auditRepoKey(name string) string {
	segments := strings.Split(name, "/")
	if len(segments) > 1 {
		return segments[0]
	}
	return ""
}

// buildGroupedAuditItems inserts groupItem separators.
// If all items belong to a single group, no separators are added.
func buildGroupedAuditItems(items []auditItem) []list.Item {
	// Check if there are multiple groups.
	groups := map[string]bool{}
	for _, item := range items {
		groups[auditRepoKey(item.result.SkillName)] = true
		if len(groups) > 1 {
			break
		}
	}

	if len(groups) <= 1 {
		result := make([]list.Item, len(items))
		for i, item := range items {
			result[i] = item
		}
		return result
	}

	var result []list.Item
	var currentGroup string
	groupCount := 0

	flush := func() {
		if groupCount > 0 {
			for i := len(result) - 1 - groupCount; i >= 0; i-- {
				if g, ok := result[i].(groupItem); ok {
					g.count = groupCount
					result[i] = g
					break
				}
			}
		}
	}

	for _, item := range items {
		key := auditRepoKey(item.result.SkillName)
		if key != currentGroup {
			flush()
			label := "standalone"
			if key != "" {
				label = strings.TrimPrefix(key, "_")
			}
			result = append(result, groupItem{label: label})
			currentGroup = key
			groupCount = 0
		}
		result = append(result, item)
		groupCount++
	}
	flush()
	return result
}

// auditDelegate renders a compact single-line row for the audit TUI.
type auditDelegate struct{}

func (auditDelegate) Height() int  { return 1 }
func (auditDelegate) Spacing() int { return 0 }
func (auditDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd {
	return nil
}

func (auditDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	width := m.Width()
	if width <= 0 {
		width = 40
	}

	switch v := item.(type) {
	case groupItem:
		renderGroupRow(w, v, width)
	case auditItem:
		renderPrefixRow(w, v.Title(), width, index == m.Index())
	}
}

func (i auditItem) Description() string { return "" }

func (i auditItem) FilterValue() string {
	// Searchable: skill name, risk label, status, max severity, finding patterns, finding files.
	r := i.result
	status := "clean"
	if r.IsBlocked {
		status = "blocked"
	} else if len(r.Findings) > 0 {
		status = "warning"
	}
	parts := []string{r.SkillName, r.RiskLabel, status, r.MaxSeverity()}

	seen := map[string]bool{}
	for _, f := range r.Findings {
		if !seen[f.Pattern] {
			parts = append(parts, f.Pattern)
			seen[f.Pattern] = true
		}
		if !seen[f.File] {
			parts = append(parts, f.File)
			seen[f.File] = true
		}
		if f.RuleID != "" && !seen[f.RuleID] {
			parts = append(parts, f.RuleID)
			seen[f.RuleID] = true
		}
		if f.Analyzer != "" && !seen[f.Analyzer] {
			parts = append(parts, f.Analyzer)
			seen[f.Analyzer] = true
		}
		if f.Category != "" && !seen[f.Category] {
			parts = append(parts, f.Category)
			seen[f.Category] = true
		}
	}
	return strings.Join(parts, " ")
}

// auditTUIModel is the bubbletea model for interactive audit results.
type auditTUIModel struct {
	list     list.Model
	quitting bool

	allItems    []auditItem
	filterText  string
	filterInput textinput.Model
	filtering   bool
	matchCount  int

	// Detail panel scrolling
	detailScroll int
	termWidth    int
	termHeight   int

	summary auditRunSummary

	// Tab switching (skills ↔ agents)
	activeTab    auditTab
	skillItems   []auditItem
	agentItems   []auditItem
	skillSummary auditRunSummary
	agentSummary auditRunSummary
	tabCounts    [2]int // [skills, agents]
}

func sortAuditItems(items []auditItem) {
	sort.Slice(items, func(i, j int) bool {
		ri, rj := items[i].result, items[j].result
		ki, kj := auditRepoKey(ri.SkillName), auditRepoKey(rj.SkillName)
		if ki != kj {
			if ki != "" && kj != "" {
				return ki < kj
			}
			return ki != ""
		}
		hasI, hasJ := len(ri.Findings) > 0, len(rj.Findings) > 0
		if hasI != hasJ {
			return hasI
		}
		if hasI && hasJ {
			rankI := audit.SeverityRank(ri.MaxSeverity())
			rankJ := audit.SeverityRank(rj.MaxSeverity())
			if rankI != rankJ {
				return rankI < rankJ
			}
			if ri.RiskScore != rj.RiskScore {
				return ri.RiskScore > rj.RiskScore
			}
		}
		return ri.SkillName < rj.SkillName
	})
}

func newAuditTUIModel(
	skillResults []*audit.Result, skillOutputs []audit.ScanOutput, skillSummary auditRunSummary,
	agentResults []*audit.Result, agentOutputs []audit.ScanOutput, agentSummary auditRunSummary,
	initialTab auditTab,
) auditTUIModel {
	buildItems := func(results []*audit.Result, outputs []audit.ScanOutput, kind string) []auditItem {
		items := make([]auditItem, 0, len(results))
		for idx, r := range results {
			var elapsed time.Duration
			if idx < len(outputs) {
				elapsed = outputs[idx].Elapsed
			}
			items = append(items, auditItem{result: r, elapsed: elapsed, kind: kind})
		}
		sortAuditItems(items)
		return items
	}

	skillItems := buildItems(skillResults, skillOutputs, "skill")
	agentItems := buildItems(agentResults, agentOutputs, "agent")

	var activeItems []auditItem
	var activeSummary auditRunSummary
	if initialTab == auditTabAgents {
		activeItems = agentItems
		activeSummary = agentSummary
	} else {
		activeItems = skillItems
		activeSummary = skillSummary
	}

	displayItems := activeItems
	if len(displayItems) > maxListItems {
		displayItems = displayItems[:maxListItems]
	}
	listItems := buildGroupedAuditItems(displayItems)

	l := list.New(listItems, auditDelegate{}, 0, 0)
	l.Title = fmt.Sprintf("Audit results (%d scanned)", activeSummary.Scanned)
	l.Styles.Title = theme.Title()
	l.Styles.NoItems = l.Styles.NoItems.PaddingLeft(2)
	l.SetStatusBarItemName(initialTab.noun(), initialTab.noun())
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)

	fi := textinput.New()
	fi.Prompt = "/ "
	fi.PromptStyle = theme.Accent()
	fi.Cursor.Style = theme.Accent()

	m := auditTUIModel{
		list:         l,
		allItems:     activeItems,
		matchCount:   len(activeItems),
		filterInput:  fi,
		summary:      activeSummary,
		activeTab:    initialTab,
		skillItems:   skillItems,
		agentItems:   agentItems,
		skillSummary: skillSummary,
		agentSummary: agentSummary,
		tabCounts:    [2]int{len(skillItems), len(agentItems)},
	}
	skipGroupItem(&m.list, 1)
	return m
}

func (m *auditTUIModel) switchTab() {
	if m.activeTab == auditTabAgents {
		m.allItems = m.agentItems
		m.summary = m.agentSummary
	} else {
		m.allItems = m.skillItems
		m.summary = m.skillSummary
	}
	m.filterText = ""
	m.filterInput.SetValue("")
	m.detailScroll = 0
	m.applyFilter()
	m.list.Title = fmt.Sprintf("Audit results (%d scanned)", m.summary.Scanned)
	m.list.SetStatusBarItemName(m.activeTab.noun(), m.activeTab.noun())
	skipGroupItem(&m.list, 1)
}

func (m auditTUIModel) Init() tea.Cmd { return nil }

func (m *auditTUIModel) applyFilter() {
	term := strings.ToLower(m.filterText)

	if term == "" {
		displayItems := m.allItems
		if len(displayItems) > maxListItems {
			displayItems = displayItems[:maxListItems]
		}
		m.matchCount = len(m.allItems)
		m.list.SetItems(buildGroupedAuditItems(displayItems))
		m.list.ResetSelected()
		skipGroupItem(&m.list, 1)
		return
	}

	var matched []list.Item
	count := 0
	for _, item := range m.allItems {
		if strings.Contains(strings.ToLower(item.FilterValue()), term) {
			count++
			if len(matched) < maxListItems {
				matched = append(matched, item)
			}
		}
	}
	m.matchCount = count
	m.list.SetItems(matched)
	m.list.ResetSelected()
}

func (m auditTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		panelHeight := m.auditPanelHeight()
		if m.termWidth >= 70 {
			m.list.SetSize(auditListWidth(m.termWidth), panelHeight)
		} else {
			m.list.SetSize(msg.Width, panelHeight)
		}
		return m, nil

	case tea.MouseMsg:
		if m.termWidth >= 70 {
			leftWidth := auditListWidth(m.termWidth)
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
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "/":
			m.filtering = true
			m.filterInput.Focus()
			return m, textinput.Blink
		case "ctrl+d":
			m.detailScroll += 5
			return m, nil
		case "ctrl+u":
			m.detailScroll -= 5
			if m.detailScroll < 0 {
				m.detailScroll = 0
			}
			return m, nil
		case "tab":
			m.activeTab = (m.activeTab + 1) % 2
			m.switchTab()
			return m, nil
		case "shift+tab":
			m.activeTab = (m.activeTab - 1 + 2) % 2
			m.switchTab()
			return m, nil
		}
	}

	prevIdx := m.list.Index()
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	// Auto-skip group separator items
	if _, isGroup := m.list.SelectedItem().(groupItem); isGroup {
		dir := 1
		if m.list.Index() < prevIdx {
			dir = -1
		}
		skipGroupItem(&m.list, dir)
	}

	if m.list.Index() != prevIdx {
		m.detailScroll = 0 // reset scroll when selection changes
	}
	return m, cmd
}

func (m auditTUIModel) View() string {
	if m.quitting {
		return ""
	}

	// Narrow terminal (<70 cols): vertical fallback
	if m.termWidth < 70 {
		return m.viewVertical()
	}

	// ── Horizontal split layout ──
	var b strings.Builder

	b.WriteString(m.renderTabBar())
	b.WriteString("\n")

	panelHeight := m.auditPanelHeight()

	leftWidth := auditListWidth(m.termWidth)
	rightWidth := auditDetailPanelWidth(m.termWidth)

	// Left panel: list
	leftPanel := lipgloss.NewStyle().
		Width(leftWidth).MaxWidth(leftWidth).
		Height(panelHeight).MaxHeight(panelHeight).
		Render(m.list.View())

	// Border column
	borderStyle := theme.Dim().
		Height(panelHeight).MaxHeight(panelHeight)
	borderCol := strings.Repeat("│\n", panelHeight)
	borderPanel := borderStyle.Render(strings.TrimRight(borderCol, "\n"))

	// Right panel: detail for selected item
	var detailStr, scrollInfo string
	if item, ok := m.list.SelectedItem().(auditItem); ok {
		detailStr, scrollInfo = wrapAndScroll(m.renderDetailContent(item), rightWidth-1, m.detailScroll, panelHeight)
	}
	rightPanel := lipgloss.NewStyle().
		Width(rightWidth).MaxWidth(rightWidth).
		Height(panelHeight).MaxHeight(panelHeight).
		PaddingLeft(1).
		Render(detailStr)

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, borderPanel, rightPanel)
	b.WriteString(body)
	b.WriteString("\n\n")

	// Filter bar (below panels)
	b.WriteString(m.renderFilterBar())

	// Summary footer
	b.WriteString(m.renderSummaryFooter())

	// Help line
	b.WriteString(theme.Dim().MarginLeft(2).Render(appendScrollInfo("Tab skills/agents  ↑↓ navigate  ←→ page  / filter  Ctrl+d/u scroll detail  q quit", scrollInfo)))

	return b.String()
}

// viewVertical renders the original vertical layout for narrow terminals.
func (m auditTUIModel) viewVertical() string {
	var b strings.Builder

	b.WriteString(m.renderTabBar())
	b.WriteString("\n")

	b.WriteString(m.list.View())
	b.WriteString("\n\n")

	b.WriteString(m.renderFilterBar())

	var scrollInfo string
	if item, ok := m.list.SelectedItem().(auditItem); ok {
		detailHeight := m.termHeight - m.termHeight*2/5 - 8
		var detailStr string
		detailStr, scrollInfo = wrapAndScroll(m.renderDetailContent(item), m.termWidth, m.detailScroll, detailHeight)
		b.WriteString(detailStr)
	}

	b.WriteString(m.renderSummaryFooter())

	b.WriteString(theme.Dim().MarginLeft(2).Render(appendScrollInfo("Tab skills/agents  ↑↓ navigate  ←→ page  / filter  Ctrl+d/u scroll  q quit", scrollInfo)))
	b.WriteString("\n")

	return b.String()
}

func (m auditTUIModel) renderTabBar() string {
	type tab struct {
		label string
		tab   auditTab
		count int
	}
	tabs := []tab{
		{"Skills", auditTabSkills, m.tabCounts[0]},
		{"Agents", auditTabAgents, m.tabCounts[1]},
	}

	activeStyle := lipgloss.NewStyle().Bold(true).Underline(true)
	inactiveStyle := theme.Dim()

	var parts []string
	for _, t := range tabs {
		label := fmt.Sprintf("%s(%d)", t.label, t.count)
		if t.tab == m.activeTab {
			parts = append(parts, activeStyle.Inherit(theme.Accent()).Render(label))
		} else {
			parts = append(parts, inactiveStyle.Render(label))
		}
	}
	return "  " + strings.Join(parts, "  ")
}

func (m auditTUIModel) renderFilterBar() string {
	return renderTUIFilterBar(
		m.filterInput.View(), m.filtering, m.filterText,
		m.matchCount, len(m.allItems), maxListItems,
		m.activeTab.noun(), m.renderPageInfo(),
	)
}

func (m auditTUIModel) renderPageInfo() string {
	return renderPageInfoFromPaginator(m.list.Paginator)
}

// renderSummaryFooter renders the compact summary above the help bar.
// Line 1: scan counts + severity breakdown.
// Line 2 (if any): category threat breakdown.
func (m auditTUIModel) renderSummaryFooter() string {
	s := m.summary
	pipe := theme.Dim().Render(" | ")

	// ── Line 1: counts + severity ──
	// Label is always dim; value uses semantic color + bold for non-zero emphasis.
	parts := []string{
		theme.Dim().Render("Scanned: ") + theme.Primary().Render(formatNumber(s.Scanned)),
		theme.Dim().Render("Passed: ") + theme.Dim().Render(formatNumber(s.Passed)),
	}
	if s.Warning > 0 {
		parts = append(parts, theme.Dim().Render("Warning: ")+theme.Warning().Bold(true).Render(formatNumber(s.Warning)))
	} else {
		parts = append(parts, theme.Dim().Render("Warning: ")+theme.Dim().Render(formatNumber(s.Warning)))
	}
	if s.Failed > 0 {
		parts = append(parts, theme.Dim().Render("Failed: ")+theme.Danger().Bold(true).Render(formatNumber(s.Failed)))
	} else {
		parts = append(parts, theme.Dim().Render("Failed: ")+theme.Dim().Render(formatNumber(s.Failed)))
	}

	sevParts := []string{
		acSevCount(s.Critical, theme.Severity("critical")).Render(fmt.Sprintf("%d", s.Critical)),
		acSevCount(s.High, theme.Severity("high")).Render(fmt.Sprintf("%d", s.High)),
		acSevCount(s.Medium, theme.Severity("medium")).Render(fmt.Sprintf("%d", s.Medium)),
		acSevCount(s.Low, theme.Severity("low")).Render(fmt.Sprintf("%d", s.Low)),
		acSevCount(s.Info, theme.Severity("info")).Render(fmt.Sprintf("%d", s.Info)),
	}
	sep := theme.Dim().Render("/")
	parts = append(parts, theme.Dim().Render("c/h/m/l/i = ")+strings.Join(sevParts, sep))

	parts = append(parts, theme.Dim().Render(fmt.Sprintf("Auditable: %.0f%% avg", s.AvgAnalyzability*100)))
	if s.PolicyProfile != "" {
		parts = append(parts, theme.Dim().Render("Policy: ")+tuiColorizeProfile(s.PolicyProfile))
	}

	var b strings.Builder
	b.WriteString("  ")
	b.WriteString(strings.Join(parts, pipe))
	b.WriteString("\n")

	// ── Line 2: category breakdown with semantic colors ──
	if threatsLine := formatCategoryBreakdownTUI(s.ByCategory); threatsLine != "" {
		b.WriteString("  ")
		b.WriteString(theme.Dim().Render("Threats: "))
		b.WriteString(threatsLine)
		b.WriteString("\n")
	}

	return b.String()
}

// renderDetailContent renders the full detail panel for the selected audit item.
// Mirrors the summary box style with colorized severity breakdown and structured findings.
func (m auditTUIModel) renderDetailContent(item auditItem) string {
	var b strings.Builder

	r := item.result

	row := func(label, value string) {
		b.WriteString(theme.Dim().Width(14).Render(label))
		b.WriteString(value)
		b.WriteString("\n")
	}

	// ── Header ──
	b.WriteString(theme.Title().Render(r.SkillName))
	b.WriteString("\n")
	b.WriteString(theme.Dim().Render(strings.Repeat("─", 40)))
	b.WriteString("\n\n")

	// ── Summary section ──

	// Risk — colorized by severity
	riskText := fmt.Sprintf("%s (%d/100)", strings.ToUpper(r.RiskLabel), r.RiskScore)
	riskStyle := theme.SeverityStyle(r.RiskLabel)
	if r.RiskLabel == "clean" {
		riskStyle = theme.Success()
	}
	row("Risk:", riskStyle.Render(riskText))

	// Max severity — use severity color; NONE = green
	maxSev := r.MaxSeverity()
	if maxSev == "" {
		maxSev = "NONE"
	}
	maxSevStyle := theme.SeverityStyle(maxSev)
	if strings.ToUpper(maxSev) == "NONE" {
		maxSevStyle = theme.Success()
	}
	row("Max sev:", maxSevStyle.Render(maxSev))

	// Block status
	if r.IsBlocked {
		row("Status:", theme.Danger().Render("✗ BLOCKED"))
	} else if len(r.Findings) == 0 {
		row("Status:", theme.Success().Render("✓ Clean"))
	} else {
		row("Status:", theme.Warning().Render("! Has findings (not blocked)"))
	}

	// Auditable — analyzability percentage
	auditableText := fmt.Sprintf("%.0f%%", r.Analyzability*100)
	if r.Analyzability >= 0.70 {
		row("Auditable:", theme.Success().Render(auditableText))
	} else if r.TotalBytes > 0 {
		row("Auditable:", theme.Warning().Render(auditableText))
	} else {
		row("Auditable:", theme.Dim().Render("—"))
	}

	// Commands — tier profile
	if !r.TierProfile.IsEmpty() {
		row("Commands:", theme.Dim().Render(r.TierProfile.String()))
	}

	// Threshold
	if r.Threshold != "" {
		row("Threshold:", theme.Dim().Render("severity >= ")+theme.SeverityStyle(r.Threshold).Render(strings.ToUpper(r.Threshold)))
	}

	// Policy
	if m.summary.PolicyProfile != "" {
		policyText := tuiColorizeProfile(m.summary.PolicyProfile) +
			theme.Dim().Render(" / dedupe:") + tuiColorizeDedupe(m.summary.PolicyDedupe) +
			theme.Dim().Render(" / analyzers:") + tuiColorizeAnalyzers(m.summary.PolicyAnalyzers)
		row("Policy:", policyText)
	}

	// Scan time
	if item.elapsed > 0 {
		row("Scan time:", theme.Dim().Render(fmt.Sprintf("%.1fs", item.elapsed.Seconds())))
	}

	// Severity breakdown — only non-zero counts are colorized; zeros are dim
	if len(r.Findings) > 0 {
		counts := map[string]int{}
		for _, f := range r.Findings {
			counts[f.Severity]++
		}
		sep := theme.Dim().Render("/")
		sevLine := acSevCount(counts["CRITICAL"], theme.Severity("critical")).Render(fmt.Sprintf("%d", counts["CRITICAL"])) + sep +
			acSevCount(counts["HIGH"], theme.Severity("high")).Render(fmt.Sprintf("%d", counts["HIGH"])) + sep +
			acSevCount(counts["MEDIUM"], theme.Severity("medium")).Render(fmt.Sprintf("%d", counts["MEDIUM"])) + sep +
			acSevCount(counts["LOW"], theme.Severity("low")).Render(fmt.Sprintf("%d", counts["LOW"])) + sep +
			acSevCount(counts["INFO"], theme.Severity("info")).Render(fmt.Sprintf("%d", counts["INFO"]))
		row("Severity:", theme.Dim().Render("c/h/m/l/i = ")+sevLine)
		row("Total:", theme.Primary().Render(fmt.Sprintf("%d", len(r.Findings)))+theme.Dim().Render(" finding(s)"))
	}

	b.WriteString("\n")

	// ── Findings detail ──
	if len(r.Findings) > 0 {
		b.WriteString(theme.Title().Render("Findings"))
		b.WriteString("\n")
		b.WriteString(theme.Dim().Render(strings.Repeat("─", 40)))
		b.WriteString("\n\n")

		sorted := make([]audit.Finding, len(r.Findings))
		copy(sorted, r.Findings)
		sort.Slice(sorted, func(i, j int) bool {
			return audit.SeverityRank(sorted[i].Severity) < audit.SeverityRank(sorted[j].Severity)
		})

		for idx, f := range sorted {
			// [N] SEVERITY  pattern
			sevBadge := theme.SeverityStyle(f.Severity).Render(strings.ToUpper(f.Severity))
			header := theme.Dim().Render(fmt.Sprintf("[%d] ", idx+1))
			patternText := theme.Primary().Bold(true).Render(f.Pattern)
			b.WriteString(header + sevBadge + "  " + patternText + "\n")

			// Message
			b.WriteString(theme.Dim().Render("    "))
			b.WriteString(theme.Dim().Render(f.Message))
			b.WriteString("\n")

			// Metadata: ruleID / analyzer / category
			if meta := findingMetaTUI(f); meta != "" {
				b.WriteString(theme.Dim().Render("    "))
				b.WriteString(theme.Accent().Render(meta))
				b.WriteString("\n")
			}

			// Location: file:line
			loc := fmt.Sprintf("%s:%d", f.File, f.Line)
			b.WriteString(theme.Dim().Render("    "))
			b.WriteString(theme.Accent().Render(loc))
			b.WriteString("\n")

			// Snippet — with │ gutter
			if f.Snippet != "" {
				gutter := theme.Dim().Render("    │ ")
				b.WriteString(gutter)
				b.WriteString(theme.Warning().Render(f.Snippet))
				b.WriteString("\n")
			}

			b.WriteString("\n")
		}
	}

	return b.String()
}

// auditFooterLines returns the number of lines the footer occupies below the panel.
// gap(2) + tab(1) + filter(1) + summary(1-2) + help(1) = 6 or 7
func (m auditTUIModel) auditFooterLines() int {
	n := 6 // gap(2) + tab(1) + filter + summary-line1 + help
	if len(m.summary.ByCategory) > 0 {
		n++ // summary-line2 (threats)
	}
	return n
}

// auditPanelHeight returns the panel height for both SetSize and View.
func (m auditTUIModel) auditPanelHeight() int {
	h := m.termHeight - m.auditFooterLines()
	if h < 6 {
		h = 6
	}
	return h
}

// auditListWidth returns the left panel width for horizontal layout.
// 36% of terminal, clamped to [30, 46].
func auditListWidth(termWidth int) int {
	w := termWidth * 36 / 100
	if w < 30 {
		w = 30
	}
	if w > 46 {
		w = 46
	}
	return w
}

// auditDetailPanelWidth returns the right detail panel width.
func auditDetailPanelWidth(termWidth int) int {
	w := termWidth - auditListWidth(termWidth) - 3
	if w < 30 {
		w = 30
	}
	return w
}

// ── TUI (lipgloss) color helpers for audit policy values ──
// Label logic is shared with CLI via policyProfileLabel/policyDedupeLabel/policyAnalyzersLabel.

// tuiColorizeProfile returns a lipgloss-styled UPPERCASE profile name.
// Only STRICT gets attention color; everything else is dim metadata.
func tuiColorizeProfile(profile string) string {
	label := policyProfileLabel(profile)
	if label == "STRICT" {
		return theme.Warning().Render(label)
	}
	return theme.Dim().Render(label)
}

// tuiColorizeDedupe returns a lipgloss-styled UPPERCASE dedupe mode.
func tuiColorizeDedupe(dedupe string) string {
	label := policyDedupeLabel(dedupe)
	if label == "LEGACY" {
		return theme.Warning().Render(label)
	}
	return theme.Dim().Render(label)
}

// tuiColorizeAnalyzers returns a lipgloss-styled UPPERCASE analyzer list.
func tuiColorizeAnalyzers(analyzers []string) string {
	return theme.Dim().Render(policyAnalyzersLabel(analyzers))
}

// findingMetaTUI builds a compact "ruleID / analyzer / category" string for TUI detail.
// Returns "" if no Phase 2 fields are set.
func findingMetaTUI(f audit.Finding) string {
	var parts []string
	if f.RuleID != "" {
		parts = append(parts, f.RuleID)
	}
	if f.Analyzer != "" {
		parts = append(parts, f.Analyzer)
	}
	if f.Category != "" {
		parts = append(parts, f.Category)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " / ")
}

// runAuditTUI starts the bubbletea TUI for audit results.
func runAuditTUI(
	skillResults []*audit.Result, skillOutputs []audit.ScanOutput, skillSummary auditRunSummary,
	agentResults []*audit.Result, agentOutputs []audit.ScanOutput, agentSummary auditRunSummary,
	initialTab auditTab,
) error {
	model := newAuditTUIModel(skillResults, skillOutputs, skillSummary, agentResults, agentOutputs, agentSummary, initialTab)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}
