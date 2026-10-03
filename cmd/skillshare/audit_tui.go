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
	grouped bool   // shown under a group row, which already names the first segment
}

func (i auditItem) Title() string {
	path := i.result.SkillName
	if i.grouped && auditRepoKey(path) != "" {
		path = path[strings.Index(path, "/")+1:]
	}
	name := colorSkillPath(compactAuditPath(path))
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
		item.grouped = true
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
		renderPrefixRow(w, alignRow(v.Title(), v.severityTag(), width-rowIndent), width, index == m.Index())
	}
}

func (i auditItem) Description() string { return "" }

// severityTag is the row's right column: the worst finding's severity.
func (i auditItem) severityTag() string {
	sev := i.result.MaxSeverity()
	if sev == "" {
		return ""
	}
	return theme.SeverityStyle(sev).Render(strings.ToLower(sev))
}

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

	showKeys bool // ? swaps the detail panel for the full key list

	// enter opens the selected result's files at its findings
	browser  *fileBrowser
	findings []audit.Finding
	finding  int
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
	l.SetShowTitle(false)
	l.Styles.NoItems = l.Styles.NoItems.PaddingLeft(2)
	l.SetStatusBarItemName(initialTab.noun(), initialTab.noun())
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)

	fi := newTUIFilterInput("type to match a name, severity, rule or file")

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
		if m.browser != nil {
			m.browser.resize(msg.Width, msg.Height)
		}
		bodyHeight := max(msg.Height-frameChrome, 6)
		if m.termWidth >= tuiNarrowSplitWidth {
			m.list.SetSize(auditListWidth(m.termWidth), bodyHeight)
		} else {
			// Narrow: the list sits above the details
			m.list.SetSize(msg.Width, max(bodyHeight/2, 4))
		}
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
		if m.termWidth >= tuiNarrowSplitWidth {
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
			cmd := handleTUIFilterKey(msg, &m.filtering, &m.filterText, &m.filterInput, m.applyFilter)
			return m, cmd
		}
		if m.browser != nil {
			return m.browserKey(msg.String())
		}

		switch msg.String() {
		case "enter":
			if item, ok := m.list.SelectedItem().(auditItem); ok {
				m.openFiles(item)
			}
			return m, nil
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
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

// openFiles opens the result's files at its first finding, in the order
// the details list them.
func (m *auditTUIModel) openFiles(item auditItem) {
	r := item.result
	m.browser = newFileBrowser("audit", r.SkillName, r.ScanTarget, true, m.termWidth, m.termHeight)
	m.findings = sortedFindings(r.Findings)
	m.finding = 0
	if len(m.findings) > 0 {
		m.browser.openAt(m.findings[0].File, m.findings[0].Line)
	}
}

func (m auditTUIModel) browserKey(k string) (tea.Model, tea.Cmd) {
	switch k {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.browser = nil
	case "n", "N":
		if len(m.findings) > 0 {
			step := 1
			if k == "N" {
				step = len(m.findings) - 1
			}
			m.finding = (m.finding + step) % len(m.findings)
			f := m.findings[m.finding]
			m.browser.openAt(f.File, f.Line)
		}
	default:
		m.browser.key(k)
	}
	return m, nil
}

// findingNote says which finding is marked and what it is.
func (m auditTUIModel) findingNote() string {
	if len(m.findings) == 0 {
		return theme.Dim().Render("No findings")
	}
	f := m.findings[m.finding]
	return theme.Dim().Render(fmt.Sprintf("%d/%d ", m.finding+1, len(m.findings))) +
		theme.SeverityStyle(f.Severity).Render(strings.ToUpper(f.Severity)) + " " +
		theme.Primary().Bold(true).Render(f.Pattern) + theme.Dim().Render(" · "+f.Message)
}

func (m auditTUIModel) View() string {
	if m.quitting {
		return ""
	}
	if m.browser != nil {
		var hints []keyHint
		if len(m.findings) > 1 {
			hints = []keyHint{{"n/N", "next/previous finding"}}
		}
		return m.browser.view(m.findingNote(), hints)
	}
	bodyHeight := max(m.termHeight-frameChrome, 6)
	title := m.renderTitleLine()
	if m.termWidth < tuiNarrowSplitWidth {
		listHeight := max(bodyHeight/2, 4)
		detailHeight := max(bodyHeight-listHeight-1, 4)
		detail := lipgloss.NewStyle().Height(detailHeight).MaxHeight(detailHeight).PaddingLeft(1).
			Render(m.renderRight(m.termWidth-2, detailHeight))
		return title + "\n\n" + m.list.View() + "\n\n" + detail + "\n" + m.renderBottom()
	}
	leftWidth := auditListWidth(m.termWidth)
	rightWidth := auditDetailPanelWidth(m.termWidth)
	return title + "\n\n" +
		renderFrameSplit(m.list.View(), m.renderRight(rightWidth-2, bodyHeight), leftWidth, rightWidth, bodyHeight) + "\n" +
		m.renderBottom()
}

// renderTitleLine renders the scope, the scan counts and the Skills / Agents tabs.
func (m auditTUIModel) renderTitleLine() string {
	s := m.summary
	var facts []string
	if s.Mode != "" {
		facts = append(facts, s.Mode)
	}
	facts = append(facts, formatNumber(s.Scanned)+" scanned")
	if m.filterText != "" {
		facts = append(facts, formatNumber(m.matchCount)+" shown")
	}
	// Colored facts go last; a colored fact ends the dim run of facts.
	if s.Failed > 0 {
		facts = append(facts, theme.Danger().Render(formatNumber(s.Failed)+" failed"))
	}
	if s.Warning > 0 {
		facts = append(facts, theme.Warning().Render(countNoun(s.Warning, "warning")))
	}
	tabs := []frameTab{
		{fmt.Sprintf("Skills %d", m.tabCounts[0]), m.activeTab == auditTabSkills},
		{fmt.Sprintf("Agents %d", m.tabCounts[1]), m.activeTab == auditTabAgents},
	}
	return renderFrameTitle(m.termWidth, "audit", facts, tabs)
}

// renderRight renders the detail panel, or the key list while ? is on.
func (m auditTUIModel) renderRight(width, height int) string {
	if m.showKeys {
		return renderKeysPanel(auditKeyGroups)
	}
	item, ok := m.list.SelectedItem().(auditItem)
	if !ok {
		return ""
	}
	detail, _ := wrapAndScroll(m.renderDetailContent(item), width, m.detailScroll, height)
	return detail
}

// renderBottom renders the scan summary on the note line and the key line;
// the filter input takes over the key line in place.
func (m auditTUIModel) renderBottom() string {
	note := truncateANSI("  "+m.renderSummaryNote(), m.termWidth)
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
		other := "agents"
		if m.activeTab == auditTabAgents {
			other = "skills"
		}
		hints := []keyHint{{"↑↓", "move"}, filter, {"tab", other}, {"enter", "open files"}, {"ctrl+d/u", "scroll"}, {"?", "keys"}}
		line = renderKeyLine(m.termWidth, hints, framePosition(m.list.Index()+1-m.groupsAbove(), m.matchCount))
	}
	return note + "\n" + line
}

// groupsAbove counts the group rows at or above the cursor, so the position
// counts results only.
func (m auditTUIModel) groupsAbove() int {
	n := 0
	for i, it := range m.list.Items() {
		if i > m.list.Index() {
			break
		}
		if _, ok := it.(groupItem); ok {
			n++
		}
	}
	return n
}

// auditKeyGroups lists every key for the ? panel.
var auditKeyGroups = []keyGroup{
	{"Move", []keyHint{
		{"↑↓", "move"},
		{"←→", "page"},
		{"/", "filter by name, severity, rule, file or category"},
		{"tab", "switch between Skills and Agents"},
		{"ctrl+d/u", "scroll the details"},
		{"enter", "open the files at the findings; n/N moves between findings"},
		{"esc", "clear the filter, then quit"},
		{"q", "quit"},
	}},
}

// renderSummaryNote renders the severity breakdown, threat categories,
// auditability and policy of the whole scan on one line.
func (m auditTUIModel) renderSummaryNote() string {
	s := m.summary
	sevParts := []string{
		acSevCount(s.Critical, theme.Severity("critical")).Render(fmt.Sprintf("%d", s.Critical)),
		acSevCount(s.High, theme.Severity("high")).Render(fmt.Sprintf("%d", s.High)),
		acSevCount(s.Medium, theme.Severity("medium")).Render(fmt.Sprintf("%d", s.Medium)),
		acSevCount(s.Low, theme.Severity("low")).Render(fmt.Sprintf("%d", s.Low)),
		acSevCount(s.Info, theme.Severity("info")).Render(fmt.Sprintf("%d", s.Info)),
	}
	dot := theme.Dim().Render(" · ")
	parts := []string{theme.Dim().Render("c/h/m/l/i ") + strings.Join(sevParts, theme.Dim().Render("/"))}
	if threats := formatCategoryBreakdownTUI(s.ByCategory); threats != "" {
		parts = append(parts, threats)
	}
	parts = append(parts, theme.Dim().Render(fmt.Sprintf("%.0f%% auditable", s.AvgAnalyzability*100)))
	if s.PolicyProfile != "" {
		parts = append(parts, theme.Dim().Render("policy ")+tuiColorizeProfile(s.PolicyProfile))
	}
	return strings.Join(parts, dot)
}

// renderDetailContent renders the full detail panel for the selected audit item.
// Mirrors the summary box style with colorized severity breakdown and structured findings.
func (m auditTUIModel) renderDetailContent(item auditItem) string {
	var b strings.Builder

	r := item.result

	row := func(label, value string) {
		b.WriteString(theme.Dim().Width(11).Render(label))
		b.WriteString(value)
		b.WriteString("\n")
	}

	b.WriteString(theme.Primary().Bold(true).Render(r.SkillName))
	b.WriteString("\n\n")

	// ── Summary section ──

	// Risk — colorized by severity
	riskText := fmt.Sprintf("%s (%d/100)", strings.ToUpper(r.RiskLabel), r.RiskScore)
	riskStyle := theme.SeverityStyle(r.RiskLabel)
	if r.RiskLabel == "clean" {
		riskStyle = theme.Success()
	}
	row("Risk", riskStyle.Render(riskText))

	// Max severity — use severity color; NONE = green
	maxSev := r.MaxSeverity()
	if maxSev == "" {
		maxSev = "NONE"
	}
	maxSevStyle := theme.SeverityStyle(maxSev)
	if strings.ToUpper(maxSev) == "NONE" {
		maxSevStyle = theme.Success()
	}
	row("Max sev", maxSevStyle.Render(maxSev))

	// Block status
	if r.IsBlocked {
		row("Status", theme.Danger().Render("✗ BLOCKED"))
	} else if len(r.Findings) == 0 {
		row("Status", theme.Success().Render("✓ Clean"))
	} else {
		row("Status", theme.Warning().Render("! Has findings (not blocked)"))
	}

	// Auditable — analyzability percentage
	auditableText := fmt.Sprintf("%.0f%%", r.Analyzability*100)
	if r.Analyzability >= 0.70 {
		row("Auditable", theme.Success().Render(auditableText))
	} else if r.TotalBytes > 0 {
		row("Auditable", theme.Warning().Render(auditableText))
	} else {
		row("Auditable", theme.Dim().Render("—"))
	}

	// Commands — tier profile
	if !r.TierProfile.IsEmpty() {
		row("Commands", theme.Dim().Render(r.TierProfile.String()))
	}

	// Threshold
	if r.Threshold != "" {
		row("Threshold", theme.Dim().Render("severity >= ")+theme.SeverityStyle(r.Threshold).Render(strings.ToUpper(r.Threshold)))
	}

	// Policy
	if m.summary.PolicyProfile != "" {
		policyText := tuiColorizeProfile(m.summary.PolicyProfile) +
			theme.Dim().Render(" / dedupe:") + tuiColorizeDedupe(m.summary.PolicyDedupe) +
			theme.Dim().Render(" / analyzers:") + tuiColorizeAnalyzers(m.summary.PolicyAnalyzers)
		row("Policy", policyText)
	}

	// Scan time
	if item.elapsed > 0 {
		row("Scan time", theme.Dim().Render(fmt.Sprintf("%.1fs", item.elapsed.Seconds())))
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
		row("Severity", theme.Dim().Render("c/h/m/l/i = ")+sevLine)
		row("Total", theme.Primary().Render(fmt.Sprintf("%d", len(r.Findings)))+theme.Dim().Render(" finding(s)"))
	}

	b.WriteString("\n")

	// ── Findings detail ──
	if len(r.Findings) > 0 {
		b.WriteString(theme.Primary().Bold(true).Render("Findings"))
		b.WriteString("\n\n")

		for idx, f := range sortedFindings(r.Findings) {
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

// sortedFindings orders findings most severe first, as the details number them.
func sortedFindings(findings []audit.Finding) []audit.Finding {
	sorted := make([]audit.Finding, len(findings))
	copy(sorted, findings)
	sort.SliceStable(sorted, func(i, j int) bool {
		return audit.SeverityRank(sorted[i].Severity) < audit.SeverityRank(sorted[j].Severity)
	})
	return sorted
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
	return max(termWidth-auditListWidth(termWidth), 30)
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
