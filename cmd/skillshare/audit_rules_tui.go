package main

import (
	"fmt"
	"os"
	"strings"

	"skillshare/internal/audit"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Severity tab definitions ──

type sevTab struct {
	label string
	sev   string // "" = ALL, "DISABLED" = disabled rules
}

var sevTabs = []sevTab{
	{"ALL", ""},
	{"CRIT", "CRITICAL"},
	{"HIGH", "HIGH"},
	{"MED", "MEDIUM"},
	{"LOW", "LOW"},
	{"INFO", "INFO"},
	{"OFF", "DISABLED"},
}

// ── Item types for the flat accordion list ──

// arHeaderItem represents a pattern group header (expandable).
type arHeaderItem struct {
	group    audit.PatternGroup
	expanded bool
}

func (i arHeaderItem) Title() string {
	arrow := "▸"
	if i.expanded {
		arrow = "▾"
	}
	countStr := fmt.Sprintf("%d", i.group.Total)
	extra := ""
	if i.group.Disabled > 0 {
		extra = tc.Yellow.Render(fmt.Sprintf(" %d off", i.group.Disabled))
	}
	return fmt.Sprintf("%s %s  %s%s", arrow, i.group.Pattern, tc.Dim.Render(countStr), extra)
}

func (i arHeaderItem) Description() string { return "" }

func (i arHeaderItem) FilterValue() string {
	return i.group.Pattern + " " + i.group.MaxSeverity
}

// arRuleItem represents a single rule under an expanded pattern.
type arRuleItem struct {
	rule    audit.CompiledRule
	display string // suffix after stripping pattern prefix (for compact list display)
}

func (i arRuleItem) Title() string {
	dot := tcSevStyle(i.rule.Severity).Render("●")
	if !i.rule.Enabled {
		return fmt.Sprintf("  %s %s  %s", dot, i.rule.ID, tc.Red.Render("off"))
	}
	return fmt.Sprintf("  %s %s", dot, i.rule.ID)
}

func (i arRuleItem) Description() string { return "" }

func (i arRuleItem) FilterValue() string {
	return i.rule.ID + " " + i.display + " " + i.rule.Message + " " + i.rule.Severity
}

// ── Model ──

type arModel struct {
	allRules  []audit.CompiledRule
	mode      runMode
	rulesPath string // cached audit-rules.yaml path (cwd is constant during TUI)

	list     list.Model
	expanded map[string]bool // pattern → expanded

	// Cached computed state — recomputed only in reloadRules/rebuildItems.
	patterns  []audit.PatternGroup
	sevCounts []int // one per sevTab

	sevTab       int // index into sevTabs
	detailScroll int

	pickingSeverity bool
	pendingReset    bool

	width, height int
	filterInput   textinput.Model
	filterText    string
	filtering     bool
	flashMsg      string
	flashTicks    int
	quitting      bool
}

// severityOptions are the valid severity levels with their shortcut keys.
var severityOptions = []struct {
	key string
	sev string
}{
	{"1", "CRITICAL"},
	{"2", "HIGH"},
	{"3", "MEDIUM"},
	{"4", "LOW"},
	{"5", "INFO"},
}

// arListWidth returns the left panel width for horizontal layout (40%).
func arListWidth(termWidth int) int {
	w := termWidth * 45 / 100
	if w < 35 {
		w = 35
	}
	if w > 65 {
		w = 65
	}
	return w
}

// arDetailWidth returns the right detail panel width.
func arDetailWidth(termWidth int) int {
	w := termWidth - arListWidth(termWidth) - 3 // 3 = border column
	if w < 30 {
		w = 30
	}
	return w
}

// useSplit returns true if horizontal split layout should be used.
func (m *arModel) useSplit() bool {
	return m.width >= tuiMinSplitWidth
}

// newARModel creates the initial model with flat accordion list.
func newARModel(rules []audit.CompiledRule, mode runMode) arModel {
	cwd, _ := os.Getwd()

	m := arModel{
		allRules:  rules,
		mode:      mode,
		rulesPath: auditRulesPathForMode(mode, cwd),
		expanded:  make(map[string]bool),
	}

	// Filter text input
	fi := textinput.New()
	fi.Prompt = "/ "
	fi.PromptStyle = tc.Filter
	fi.Cursor.Style = tc.Filter
	m.filterInput = fi

	// Create the list model once — rebuildItems will populate via SetItems.
	delegate := list.NewDefaultDelegate()
	configureDelegate(&delegate, false)
	m.list = list.New(nil, delegate, 0, 0)
	m.list.Styles.Title = tc.ListTitle
	m.list.SetShowStatusBar(false)
	m.list.SetFilteringEnabled(false)
	m.list.SetShowHelp(false)
	m.list.SetShowPagination(false)

	// Compute initial cached state and populate list items.
	m.recomputeCache()
	m.rebuildItems()
	return m
}

// recomputeCache refreshes cached patterns and severity counts from allRules.
// Call after allRules changes (i.e. in reloadRules).
func (m *arModel) recomputeCache() {
	m.patterns = audit.PatternSummary(m.allRules)

	counts := make([]int, len(sevTabs))
	for _, r := range m.allRules {
		counts[0]++ // ALL
		if !r.Enabled {
			counts[6]++ // OFF
			continue
		}
		switch r.Severity {
		case "CRITICAL":
			counts[1]++
		case "HIGH":
			counts[2]++
		case "MEDIUM":
			counts[3]++
		case "LOW":
			counts[4]++
		case "INFO":
			counts[5]++
		}
	}
	m.sevCounts = counts
}

// rebuildItems reconstructs the flat list from current state.
// Called after every state change: toggle, severity, filter, tab, expand/collapse.
func (m *arModel) rebuildItems() {
	// Pre-index rules by pattern for O(P+R) instead of O(P*R).
	rulesByPattern := make(map[string][]audit.CompiledRule)
	for _, r := range m.allRules {
		rulesByPattern[r.Pattern] = append(rulesByPattern[r.Pattern], r)
	}

	filterTerm := strings.ToLower(m.filterText)
	activeSevTab := sevTabs[m.sevTab]

	// Save current cursor position
	var savedIdx int
	if m.list.Items() != nil {
		savedIdx = m.list.Index()
	}

	var items []list.Item

	for _, pg := range m.patterns {
		patternRules := rulesByPattern[pg.Pattern]

		// Filter by severity tab
		var matchedRules []audit.CompiledRule
		for _, r := range patternRules {
			if !m.ruleMatchesSevTab(r, activeSevTab) {
				continue
			}
			matchedRules = append(matchedRules, r)
		}

		// Skip pattern if no rules match the tab
		if len(matchedRules) == 0 {
			continue
		}

		// Apply text filter to rules
		var filteredRules []audit.CompiledRule
		for _, r := range matchedRules {
			filterVal := r.ID + " " + r.Message + " " + r.Severity
			if filterTerm != "" && !strings.Contains(strings.ToLower(filterVal), filterTerm) {
				continue
			}
			filteredRules = append(filteredRules, r)
		}

		// Also check if the pattern name itself matches the filter
		patternMatches := filterTerm == "" || strings.Contains(strings.ToLower(pg.Pattern+" "+pg.MaxSeverity), filterTerm)

		// Skip if no filtered rules and pattern doesn't match
		if len(filteredRules) == 0 && !patternMatches {
			continue
		}

		// If pattern matches but no individual rules match, use matchedRules (before text filter)
		rulesForDisplay := filteredRules
		if len(filteredRules) == 0 && patternMatches {
			rulesForDisplay = matchedRules
		}

		// Add header
		isExpanded := m.expanded[pg.Pattern]
		items = append(items, arHeaderItem{group: pg, expanded: isExpanded})

		// Add rules if expanded
		if isExpanded {
			prefix := pg.Pattern + "-"
			for _, r := range rulesForDisplay {
				display := r.ID
				if strings.HasPrefix(r.ID, prefix) {
					display = strings.TrimPrefix(r.ID, prefix)
				}
				items = append(items, arRuleItem{rule: r, display: display})
			}
		}
	}

	// Update list items in-place (preserves delegate, styles, etc.)
	m.list.SetItems(items)
	m.list.Title = fmt.Sprintf("Audit Rules — %d patterns, %d rules", len(m.patterns), len(m.allRules))

	if m.width > 0 {
		if m.useSplit() {
			m.list.SetSize(arListWidth(m.width), m.listHeight())
		} else {
			m.list.SetSize(m.width, m.listHeight())
		}
	}

	// Restore cursor position
	if savedIdx > 0 && savedIdx < len(items) {
		m.list.Select(savedIdx)
	}
}

// ruleMatchesSevTab returns true if a rule matches the current severity tab filter.
func (m *arModel) ruleMatchesSevTab(r audit.CompiledRule, tab sevTab) bool {
	if tab.sev == "" {
		return true // ALL tab
	}
	if tab.sev == "DISABLED" {
		return !r.Enabled
	}
	return strings.EqualFold(r.Severity, tab.sev)
}

// listHeight returns the height for the list widget.
func (m *arModel) listHeight() int {
	// Reserve: 1 sev tab bar + 1 filter + 1 flash + 1 help + 2 gaps = ~6
	h := m.height - 6
	if h < 6 {
		h = 6
	}
	return h
}

func (m arModel) Init() tea.Cmd {
	return nil
}

func (m arModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.useSplit() {
			m.list.SetSize(arListWidth(msg.Width), m.listHeight())
		} else {
			m.list.SetSize(msg.Width, m.listHeight())
		}
		return m, nil

	case tea.KeyMsg:
		// Decrement flash on any keypress
		if m.flashTicks > 0 {
			m.flashTicks--
			if m.flashTicks == 0 {
				m.flashMsg = ""
			}
		}

		// --- Reset confirmation mode ---
		if m.pendingReset {
			if msg.String() == "R" {
				m.pendingReset = false
				m.resetAllRules()
				return m, nil
			}
			m.pendingReset = false
			m.flashMsg = ""
			m.flashTicks = 0
			// Fall through to normal key handling
		}

		// --- Severity picker mode ---
		if m.pickingSeverity {
			return m.updateSeverityPicker(msg)
		}

		// --- Filter mode ---
		if m.filtering {
			switch msg.String() {
			case "esc":
				m.filtering = false
				m.filterText = ""
				m.filterInput.SetValue("")
				m.rebuildItems()
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
				m.rebuildItems()
			}
			return m, cmd
		}

		// --- Normal mode keys ---
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "esc":
			m.quitting = true
			return m, tea.Quit
		case "/":
			m.filtering = true
			m.filterInput.Focus()
			return m, textinput.Blink
		case "tab":
			m.sevTab = (m.sevTab + 1) % len(sevTabs)
			m.rebuildItems()
			return m, nil
		case "shift+tab":
			m.sevTab = (m.sevTab - 1 + len(sevTabs)) % len(sevTabs)
			m.rebuildItems()
			return m, nil
		case "enter":
			m.toggleExpand()
			return m, nil
		case " ":
			m.toggleSelected()
			return m, nil
		case "s":
			m.pickingSeverity = true
			return m, nil
		case "R":
			m.pendingReset = true
			m.flashMsg = tc.Yellow.Render("Press R again to reset all rules to defaults")
			m.flashTicks = 5
			return m, nil
		case "ctrl+d":
			m.detailScroll += 5
			return m, nil
		case "ctrl+u":
			m.detailScroll -= 5
			if m.detailScroll < 0 {
				m.detailScroll = 0
			}
			return m, nil
		}

		// Track cursor movement to reset detail scroll
		prevIdx := m.list.Index()
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		if m.list.Index() != prevIdx {
			m.detailScroll = 0
		}
		return m, cmd
	}

	// Delegate non-key messages to the list
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// toggleExpand expands/collapses the selected pattern header.
func (m *arModel) toggleExpand() {
	item, ok := m.list.SelectedItem().(arHeaderItem)
	if !ok {
		return // on a rule item, do nothing
	}
	m.expanded[item.group.Pattern] = !m.expanded[item.group.Pattern]
	m.rebuildItems()
}

// toggleSelected toggles the selected item.
// Header → toggle all rules in pattern. Rule → toggle single rule.
func (m *arModel) toggleSelected() {
	switch item := m.list.SelectedItem().(type) {
	case arHeaderItem:
		m.togglePattern(item)
	case arRuleItem:
		m.toggleRule(item)
	}
}

// execMutation runs a mutation function, shows flash feedback, and reloads rules.
func (m *arModel) execMutation(fn func() error, successMsg string) {
	if err := fn(); err != nil {
		m.flashMsg = tc.Red.Render("✗ " + err.Error())
		m.flashTicks = 3
		return
	}
	m.flashMsg = tc.Green.Render("✓ " + successMsg)
	m.flashTicks = 3
	m.reloadRules()
	m.rebuildItems()
}

// togglePattern toggles all rules in the selected pattern.
func (m *arModel) togglePattern(item arHeaderItem) {
	allEnabled := item.group.Disabled == 0
	action := "Enabled"
	if allEnabled {
		action = "Disabled"
	}
	countStr := fmt.Sprintf("all %d rules", item.group.Total)
	if item.group.Total == 1 {
		countStr = "1 rule"
	}
	m.execMutation(
		func() error { return audit.TogglePattern(m.rulesPath, item.group.Pattern, !allEnabled) },
		fmt.Sprintf("%s %s (%s)", action, item.group.Pattern, countStr),
	)
}

// toggleRule toggles a single rule.
func (m *arModel) toggleRule(item arRuleItem) {
	newEnabled := !item.rule.Enabled
	action := "Enabled"
	if !newEnabled {
		action = "Disabled"
	}
	m.execMutation(
		func() error { return audit.ToggleRule(m.rulesPath, item.rule.ID, newEnabled) },
		action+" "+item.rule.ID,
	)
}

// updateSeverityPicker handles key input while the severity picker is active.
func (m arModel) updateSeverityPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.pickingSeverity = false
		return m, nil
	}

	for _, opt := range severityOptions {
		if msg.String() == opt.key {
			m.pickingSeverity = false
			switch item := m.list.SelectedItem().(type) {
			case arHeaderItem:
				m.setSeverityForPattern(item, opt.sev)
			case arRuleItem:
				m.setSeverityForRule(item, opt.sev)
			}
			return m, nil
		}
	}

	return m, nil
}

// setSeverityForRule sets the severity of a single rule.
func (m *arModel) setSeverityForRule(item arRuleItem, sev string) {
	m.execMutation(
		func() error { return audit.SetSeverity(m.rulesPath, item.rule.ID, sev) },
		fmt.Sprintf("%s → %s", item.rule.ID, sev),
	)
}

// setSeverityForPattern sets the severity for all rules in a pattern.
func (m *arModel) setSeverityForPattern(item arHeaderItem, sev string) {
	countStr := fmt.Sprintf("all %d rules", item.group.Total)
	if item.group.Total == 1 {
		countStr = "1 rule"
	}
	m.execMutation(
		func() error { return audit.SetPatternSeverity(m.rulesPath, item.group.Pattern, sev) },
		fmt.Sprintf("%s → %s (%s)", item.group.Pattern, sev, countStr),
	)
}

// resetAllRules deletes audit-rules.yaml, restoring all rules to built-in defaults.
func (m *arModel) resetAllRules() {
	m.execMutation(
		func() error { return audit.ResetRules(m.rulesPath) },
		"Reset all rules to built-in defaults",
	)
}

// reloadRules re-reads all rules from disk after a toggle mutation.
func (m *arModel) reloadRules() {
	audit.ResetGlobalCache()

	var rules []audit.CompiledRule
	var err error
	if m.mode == modeProject {
		cwd, _ := os.Getwd()
		rules, err = audit.ListRulesWithProject(cwd)
	} else {
		rules, err = audit.ListRules()
	}
	if err != nil {
		return // keep existing rules on error
	}
	m.allRules = rules
	m.recomputeCache()
}

// ── View ──

func (m arModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// Severity tab bar
	b.WriteString(m.renderSevTabBar())
	b.WriteString("\n\n")

	// Main content
	if m.useSplit() {
		b.WriteString(m.viewHorizontal())
	} else {
		b.WriteString(m.viewVertical())
	}

	return b.String()
}

// renderSevTabBar renders the severity tab bar at the top.
func (m arModel) renderSevTabBar() string {
	counts := m.sevCounts
	var parts []string

	activeStyle := lipgloss.NewStyle().Bold(true).Underline(true)
	inactiveStyle := tc.Dim

	for i, tab := range sevTabs {
		label := fmt.Sprintf("%s(%d)", tab.label, counts[i])
		if i == m.sevTab {
			// Color active tab by its severity
			switch tab.sev {
			case "CRITICAL":
				parts = append(parts, activeStyle.Inherit(tc.Critical).Render(label))
			case "HIGH":
				parts = append(parts, activeStyle.Inherit(tc.High).Render(label))
			case "MEDIUM":
				parts = append(parts, activeStyle.Inherit(tc.Medium).Render(label))
			case "LOW":
				parts = append(parts, activeStyle.Inherit(tc.Low).Render(label))
			case "INFO":
				parts = append(parts, activeStyle.Inherit(tc.Info).Render(label))
			case "DISABLED":
				parts = append(parts, activeStyle.Inherit(tc.Red).Render(label))
			default:
				parts = append(parts, activeStyle.Inherit(tc.Cyan).Render(label))
			}
		} else {
			parts = append(parts, inactiveStyle.Render(label))
		}
	}

	return "  " + strings.Join(parts, "  ")
}

// viewHorizontal renders the list + detail side by side.
func (m arModel) viewHorizontal() string {
	var b strings.Builder

	panelHeight := m.height - 6
	if panelHeight < 6 {
		panelHeight = 6
	}

	leftWidth := arListWidth(m.width)
	rightWidth := arDetailWidth(m.width)

	// Right panel: detail for selected item
	detailStr := m.renderSelectedDetail()
	if detailStr != "" {
		detailStr = applyDetailScroll(detailStr, m.detailScroll, panelHeight)
	}

	body := renderHorizontalSplit(m.list.View(), detailStr, leftWidth, rightWidth, panelHeight)
	b.WriteString(body)
	b.WriteString("\n\n")

	b.WriteString(m.renderFilterBar())
	b.WriteString(m.renderFlashAndHelp())

	return b.String()
}

// viewVertical renders the list with detail below (narrow terminal fallback).
func (m arModel) viewVertical() string {
	var b strings.Builder

	b.WriteString(m.list.View())
	b.WriteString("\n\n")
	b.WriteString(m.renderFilterBar())

	// Detail panel for selected item
	detail := m.renderSelectedDetail()
	if detail != "" {
		b.WriteString(detail)
	}

	b.WriteString(m.renderFlashAndHelp())

	return b.String()
}

// renderSelectedDetail renders detail for the currently selected item.
func (m arModel) renderSelectedDetail() string {
	switch item := m.list.SelectedItem().(type) {
	case arHeaderItem:
		return m.renderPatternDetail(item)
	case arRuleItem:
		return m.renderRuleDetail(item)
	}
	return ""
}

// renderPatternDetail renders the detail panel for a pattern header.
func (m arModel) renderPatternDetail(item arHeaderItem) string {
	var b strings.Builder
	pg := item.group

	row := func(label, value string) {
		b.WriteString(tc.Label.Render(label))
		b.WriteString(value)
		b.WriteString("\n")
	}

	// Header
	b.WriteString(tc.Title.Render(pg.Pattern))
	b.WriteString("\n")
	b.WriteString(tc.Dim.Render(strings.Repeat("─", 36)))
	b.WriteString("\n\n")

	row("Rules:", fmt.Sprintf("%d total", pg.Total))
	row("Max Sev:", tcSevStyle(pg.MaxSeverity).Render(pg.MaxSeverity))
	row("Enabled:", tc.Green.Render(fmt.Sprintf("%d", pg.Enabled)))
	if pg.Disabled > 0 {
		row("Disabled:", tc.Red.Render(fmt.Sprintf("%d", pg.Disabled)))
	} else {
		row("Disabled:", tc.Dim.Render("0"))
	}

	// Severity distribution
	b.WriteString("\n")
	b.WriteString(tc.Label.Render("Severity:"))
	b.WriteString("\n")
	sevCounts := make(map[string]int)
	for _, r := range m.allRules {
		if r.Pattern == pg.Pattern {
			sevCounts[r.Severity]++
		}
	}
	for _, sev := range []string{"CRITICAL", "HIGH", "MEDIUM", "LOW", "INFO"} {
		if count, ok := sevCounts[sev]; ok && count > 0 {
			b.WriteString("  ")
			b.WriteString(tcSevStyle(sev).Render(fmt.Sprintf("%-10s %d", sev, count)))
			b.WriteString("\n")
		}
	}

	return b.String()
}

// renderRuleDetail renders the detail panel for a single rule.
func (m arModel) renderRuleDetail(item arRuleItem) string {
	var b strings.Builder

	row := func(label, value string) {
		b.WriteString(tc.Label.Render(label))
		b.WriteString(value)
		b.WriteString("\n")
	}

	r := item.rule

	// Header
	b.WriteString(tc.Title.Render(item.rule.ID))
	b.WriteString("\n")
	b.WriteString(tc.Dim.Render(strings.Repeat("─", 36)))
	b.WriteString("\n\n")

	row("ID:", r.ID)
	row("Pattern:", tc.Emphasis.Render(r.Pattern))
	row("Severity:", tcSevStyle(r.Severity).Render(r.Severity))
	row("Message:", r.Message)
	row("Regex:", r.Regex)

	if r.Exclude != "" {
		row("Exclude:", r.Exclude)
	}

	statusStr := tc.Green.Render("enabled")
	if !r.Enabled {
		statusStr = tc.Red.Render("disabled")
	}
	sourceLabel := r.Source
	switch r.Source {
	case "global":
		sourceLabel = "global audit-rules.yaml"
	case "project":
		sourceLabel = "project audit-rules.yaml"
	case "builtin":
		sourceLabel = "built-in"
	}
	row("Status:", statusStr+" "+tc.Dim.Render("("+sourceLabel+")"))

	return b.String()
}

// renderFilterBar renders filter input or status.
func (m arModel) renderFilterBar() string {
	totalCount := len(m.list.Items())
	matchCount := totalCount

	return renderTUIFilterBar(
		m.filterInput.View(), m.filtering, m.filterText,
		matchCount, totalCount, 0,
		"items", "",
	)
}

// renderFlashAndHelp renders the flash message and help bar.
func (m arModel) renderFlashAndHelp() string {
	var b strings.Builder

	if m.flashMsg != "" {
		b.WriteString("  " + m.flashMsg + "\n")
	}

	if m.pickingSeverity {
		b.WriteString(m.renderSeverityPicker())
	} else {
		help := "↑↓ nav  Space toggle  Enter expand  Tab severity  s sev  R reset  / filter  q quit"
		if m.useSplit() {
			help += "  Ctrl+d/u scroll"
		}
		b.WriteString(tc.Help.Render(help))
	}
	b.WriteString("\n")

	return b.String()
}

// renderSeverityPicker renders the inline severity selection bar.
func (m arModel) renderSeverityPicker() string {
	var parts []string
	for _, opt := range severityOptions {
		badge := tcSevStyle(opt.sev).Render(opt.key + " " + opt.sev)
		parts = append(parts, badge)
	}
	return tc.Help.Render("Set severity: ") + strings.Join(parts, tc.Dim.Render("  ")) + tc.Help.Render("  Esc cancel")
}

// runAuditRulesTUI starts the bubbletea TUI for browsing/toggling audit rules.
func runAuditRulesTUI(rules []audit.CompiledRule, mode runMode) error {
	model := newARModel(rules, mode)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
