package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"skillshare/internal/audit"
	"skillshare/internal/theme"

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
	{"All", ""},
	{"Crit", "CRITICAL"},
	{"High", "HIGH"},
	{"Med", "MEDIUM"},
	{"Low", "LOW"},
	{"Info", "INFO"},
	{"Off", "DISABLED"},
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
	return theme.Dim().Render(arrow) + " " + i.group.Pattern
}

// right is the row's right column: the rule count and how many are off.
func (i arHeaderItem) right() string {
	right := theme.Dim().Render(countNoun(i.group.Total, "rule"))
	if i.group.Disabled > 0 {
		right = theme.Warning().Render(fmt.Sprintf("%d off", i.group.Disabled)) + theme.Dim().Render(" · ") + right
	}
	return right
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
	return "  " + theme.SeverityStyle(i.rule.Severity).Render("●") + " " + i.display
}

// right is the row's right column: "off" for a disabled rule.
func (i arRuleItem) right() string {
	if !i.rule.Enabled {
		return theme.Warning().Render("off")
	}
	return ""
}

// arDelegate renders a pattern or rule row with its right column aligned.
type arDelegate struct{}

func (arDelegate) Height() int                             { return 1 }
func (arDelegate) Spacing() int                            { return 0 }
func (arDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (arDelegate) Render(w io.Writer, m list.Model, index int, li list.Item) {
	var left, right string
	switch v := li.(type) {
	case arHeaderItem:
		left, right = v.Title(), v.right()
	case arRuleItem:
		left, right = v.Title(), v.right()
	default:
		return
	}
	renderPrefixRow(w, alignRow(left, right, m.Width()-rowIndent), m.Width(), index == m.Index())
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

	pickingSeverity bool // e: the severity choices are on the key line
	pendingReset    bool // R: the reset confirmation is on the key line
	showKeys        bool // ? swaps the detail panel for the full key list

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
	return max(termWidth-arListWidth(termWidth), 30)
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
	fi := newTUIFilterInput("type to match a pattern, rule, message or severity")
	m.filterInput = fi

	// Create the list model once — rebuildItems will populate via SetItems.
	m.list = list.New(nil, arDelegate{}, 0, 0)
	m.list.SetShowTitle(false)
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
	bodyHeight := max(m.height-frameChrome, 6)
	if m.useSplit() {
		return bodyHeight
	}
	return max(bodyHeight/2, 4) // narrow: the details sit below the list
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

		// --- Reset confirmation on the key line ---
		if m.pendingReset {
			switch msg.String() {
			case "y", "Y", "enter":
				m.pendingReset = false
				m.resetAllRules()
			case "n", "N", "esc", "q":
				m.pendingReset = false
			}
			return m, nil
		}

		// --- Severity picker mode ---
		if m.pickingSeverity {
			return m.updateSeverityPicker(msg)
		}

		// --- Filter mode ---
		if m.filtering {
			cmd := handleTUIFilterKey(msg, &m.filtering, &m.filterText, &m.filterInput, m.rebuildItems)
			return m, cmd
		}

		// --- Normal mode keys ---
		switch msg.String() {
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
				m.rebuildItems()
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
		case "t", " ":
			m.toggleSelected()
			return m, nil
		case "e":
			if m.list.SelectedItem() != nil {
				m.pickingSeverity = true
				m.flashMsg = ""
			}
			return m, nil
		case "R":
			m.pendingReset = true
			m.flashMsg = ""
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
		m.flashMsg = theme.Danger().Render("✗ " + err.Error())
		m.flashTicks = 3
		return
	}
	m.flashMsg = theme.Success().Render("✓ " + successMsg)
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
	if msg.String() == "esc" || msg.String() == "q" {
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
	bodyHeight := max(m.height-frameChrome, 6)
	title := m.renderTitleLine()
	if !m.useSplit() {
		listHeight := m.listHeight()
		detailHeight := max(bodyHeight-listHeight-1, 4)
		detail := lipgloss.NewStyle().Height(detailHeight).MaxHeight(detailHeight).PaddingLeft(1).
			Render(m.renderRight(m.width-2, detailHeight))
		return title + "\n\n" + m.list.View() + "\n\n" + detail + "\n" + m.renderBottom()
	}
	leftWidth := arListWidth(m.width)
	rightWidth := arDetailWidth(m.width)
	return title + "\n\n" +
		renderFrameSplit(m.list.View(), m.renderRight(rightWidth-2, bodyHeight), leftWidth, rightWidth, bodyHeight) + "\n" +
		m.renderBottom()
}

// renderTitleLine renders the scope, the pattern and rule counts and the
// severity tabs.
func (m arModel) renderTitleLine() string {
	scope := "global"
	if m.mode == modeProject {
		scope = "project"
	}
	facts := []string{scope, countNoun(len(m.patterns), "pattern"), countNoun(len(m.allRules), "rule")}
	tabs := make([]frameTab, len(sevTabs))
	for i, t := range sevTabs {
		tabs[i] = frameTab{fmt.Sprintf("%s %d", t.label, m.sevCounts[i]), i == m.sevTab}
	}
	return renderFrameTitle(m.width, "audit rules", facts, tabs)
}

// renderRight renders the detail panel, or the key list while ? is on.
func (m arModel) renderRight(width, height int) string {
	if m.showKeys {
		return renderKeysPanel(arKeyGroups)
	}
	detail, _ := wrapAndScroll(m.renderSelectedDetail(), width, m.detailScroll, height)
	return detail
}

// renderBottom renders the note line and the key line. The severity
// choices, the reset confirmation and the filter input take over the key
// line in place.
func (m arModel) renderBottom() string {
	note := ""
	if m.flashMsg != "" {
		note = "  " + m.flashMsg
	}
	var line string
	switch {
	case m.pickingSeverity:
		line = m.renderSeverityPicker()
	case m.pendingReset:
		note = theme.Dim().Render("  Deletes " + shortenPath(m.rulesPath) + "; every rule goes back to its built-in default.")
		line = renderConfirmLine(m.width, "Reset all rules?", true, "")
	case m.filtering:
		line = renderFilterLine(m.width, m.filterInput.View(), len(m.list.Items()))
	case m.showKeys:
		line = renderKeyLine(m.width, []keyHint{{"?/esc", "close"}}, "")
	default:
		filter := keyHint{"/", "filter"}
		if m.filterText != "" {
			filter = keyHint{"esc", "clear filter"}
		}
		hints := []keyHint{{"↑↓", "move"}, {"enter", "expand"}, {"t", "on/off"}, {"e", "severity"}, filter, {"tab", "severity tab"}, {"?", "keys"}}
		line = renderKeyLine(m.width, hints, framePosition(m.list.Index()+1, len(m.list.Items())))
	}
	return note + "\n" + line
}

// arKeyGroups lists every key for the ? panel.
var arKeyGroups = []keyGroup{
	{"Move", []keyHint{
		{"↑↓", "move"},
		{"enter", "expand or collapse a pattern"},
		{"tab", "next severity tab"},
		{"/", "filter"},
		{"ctrl+d/u", "scroll the details"},
		{"esc", "clear the filter, then quit"},
		{"q", "quit"},
	}},
	{"Change", []keyHint{
		{"t", "turn the rule, or every rule in the pattern, on or off"},
		{"e", "change the severity"},
		{"R", "reset all rules to the built-in defaults"},
	}},
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
		b.WriteString(theme.Dim().Width(10).Render(label))
		b.WriteString(value)
		b.WriteString("\n")
	}

	b.WriteString(theme.Primary().Bold(true).Render(pg.Pattern))
	b.WriteString("\n\n")

	row("Rules", fmt.Sprintf("%d total", pg.Total))
	row("Max sev", theme.SeverityStyle(pg.MaxSeverity).Render(pg.MaxSeverity))
	row("Enabled", theme.Success().Render(fmt.Sprintf("%d", pg.Enabled)))
	if pg.Disabled > 0 {
		row("Disabled", theme.Danger().Render(fmt.Sprintf("%d", pg.Disabled)))
	} else {
		row("Disabled", theme.Dim().Render("0"))
	}

	// Severity distribution
	b.WriteString("\n")
	b.WriteString(theme.Primary().Bold(true).Render("Severity"))
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
			b.WriteString(theme.SeverityStyle(sev).Render(fmt.Sprintf("%-9s %d", strings.ToLower(sev), count)))
			b.WriteString("\n")
		}
	}

	return b.String()
}

// renderRuleDetail renders the detail panel for a single rule.
func (m arModel) renderRuleDetail(item arRuleItem) string {
	var b strings.Builder

	row := func(label, value string) {
		b.WriteString(theme.Dim().Width(10).Render(label))
		b.WriteString(value)
		b.WriteString("\n")
	}

	r := item.rule

	b.WriteString(theme.Primary().Bold(true).Render(item.rule.ID))
	b.WriteString("\n\n")

	row("Pattern", theme.Primary().Render(r.Pattern))
	row("Severity", theme.SeverityStyle(r.Severity).Render(r.Severity))
	row("Message", r.Message)
	row("Regex", r.Regex)

	if r.Exclude != "" {
		row("Exclude", r.Exclude)
	}

	statusStr := theme.Success().Render("enabled")
	if !r.Enabled {
		statusStr = theme.Danger().Render("disabled")
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
	row("Status", statusStr+" "+theme.Dim().Render("("+sourceLabel+")"))

	return b.String()
}

// renderSeverityPicker asks for the new severity on the key line.
func (m arModel) renderSeverityPicker() string {
	name := ""
	switch item := m.list.SelectedItem().(type) {
	case arHeaderItem:
		name = item.group.Pattern
	case arRuleItem:
		name = item.rule.ID
	}
	parts := make([]string, 0, len(severityOptions)+1)
	for _, opt := range severityOptions {
		parts = append(parts, theme.Primary().Render(opt.key)+" "+theme.SeverityStyle(opt.sev).Render(strings.ToLower(opt.sev)))
	}
	parts = append(parts, joinKeyHints([]keyHint{{"esc", "cancel"}}))
	return "  " + theme.Warning().Render("?") + " " + theme.Primary().Bold(true).Render("Severity for "+name+"?") + "  " +
		strings.Join(parts, theme.Dim().Render(" · "))
}

// runAuditRulesTUI starts the bubbletea TUI for browsing/toggling audit rules.
func runAuditRulesTUI(rules []audit.CompiledRule, mode runMode) error {
	model := newARModel(rules, mode)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
