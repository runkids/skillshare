package main

import (
	"fmt"
	"os"
	"strings"

	"skillshare/internal/audit"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// ── Level enum ──

type arLevel int

const (
	arLevelPatterns arLevel = iota
	arLevelRules
)

// ── Pattern item (Level 1) ──

type arPatternItem struct {
	group audit.PatternGroup
}

func (i arPatternItem) Title() string {
	status := tc.Green.Render("all enabled")
	if i.group.Disabled > 0 {
		status = tc.Yellow.Render(fmt.Sprintf("%d disabled", i.group.Disabled))
	}
	countStr := fmt.Sprintf("%d rules", i.group.Total)
	if i.group.Total == 1 {
		countStr = "1 rule"
	}
	sev := tcSevStyle(i.group.MaxSeverity).Render(i.group.MaxSeverity)
	return fmt.Sprintf("▸ %-26s %8s   %-16s max: %s", i.group.Pattern, countStr, status, sev)
}

func (i arPatternItem) Description() string { return "" }

func (i arPatternItem) FilterValue() string {
	return i.group.Pattern + " " + i.group.MaxSeverity
}

// ── Rule item (Level 2) ──

type arRuleItem struct {
	rule    audit.CompiledRule
	shortID string
}

func (i arRuleItem) Title() string {
	sev := tcSevStyle(i.rule.Severity).Render(fmt.Sprintf("%-10s", i.rule.Severity))
	status := tc.Green.Render("enabled")
	if !i.rule.Enabled {
		status = tc.Red.Render("disabled") + " ←"
	}
	return fmt.Sprintf("● %s %-34s %s", sev, i.shortID, status)
}

func (i arRuleItem) Description() string { return "" }

func (i arRuleItem) FilterValue() string {
	return i.shortID + " " + i.rule.ID + " " + i.rule.Message + " " + i.rule.Severity
}

// ── Model ──

type arModel struct {
	level    arLevel
	allRules []audit.CompiledRule
	mode     runMode

	// Level 1
	patternList  list.Model
	patternItems []arPatternItem

	// Level 2
	ruleList       list.Model
	ruleItems      []arRuleItem
	currentPattern string

	// Shared
	width, height int
	filterInput   textinput.Model
	filterText    string
	filtering     bool
	flashMsg      string
	flashTicks    int
	quitting      bool
}

// newARModel creates the initial model starting at Level 1 (pattern list).
func newARModel(rules []audit.CompiledRule, mode runMode) arModel {
	m := arModel{
		level:    arLevelPatterns,
		allRules: rules,
		mode:     mode,
	}

	// Filter text input
	fi := textinput.New()
	fi.Prompt = "/ "
	fi.PromptStyle = tc.Filter
	fi.Cursor.Style = tc.Filter
	m.filterInput = fi

	// Build pattern list
	m.buildPatternList()
	return m
}

// buildPatternList constructs the Level 1 list from current allRules.
func (m *arModel) buildPatternList() {
	patterns := audit.PatternSummary(m.allRules)
	m.patternItems = make([]arPatternItem, len(patterns))
	items := make([]list.Item, len(patterns))
	for i, pg := range patterns {
		m.patternItems[i] = arPatternItem{group: pg}
		items[i] = m.patternItems[i]
	}

	totalRules := len(m.allRules)
	delegate := list.NewDefaultDelegate()
	configureDelegate(&delegate, false)

	l := list.New(items, delegate, 0, 0)
	l.Title = fmt.Sprintf("Audit Rules — %d patterns, %d rules", len(patterns), totalRules)
	l.Styles.Title = tc.ListTitle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)

	if m.width > 0 {
		l.SetSize(m.width, m.listHeight())
	}

	m.patternList = l
}

// buildRuleList constructs the Level 2 list for a specific pattern.
func (m *arModel) buildRuleList(pattern string) {
	m.currentPattern = pattern
	var ruleItems []arRuleItem
	prefix := pattern + "-"
	for _, r := range m.allRules {
		if r.Pattern != pattern {
			continue
		}
		shortID := r.ID
		if strings.HasPrefix(r.ID, prefix) {
			shortID = strings.TrimPrefix(r.ID, prefix)
		}
		ruleItems = append(ruleItems, arRuleItem{rule: r, shortID: shortID})
	}
	m.ruleItems = ruleItems

	items := make([]list.Item, len(ruleItems))
	for i, ri := range ruleItems {
		items[i] = ri
	}

	delegate := list.NewDefaultDelegate()
	configureDelegate(&delegate, false)

	l := list.New(items, delegate, 0, 0)
	l.Title = fmt.Sprintf("%s — %d rules", pattern, len(ruleItems))
	l.Styles.Title = tc.ListTitle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)

	if m.width > 0 {
		l.SetSize(m.width, m.listHeight())
	}

	m.ruleList = l
}

// listHeight returns the height for the list widget, reserving space for
// detail panel, filter bar, and help bar.
func (m *arModel) listHeight() int {
	if m.level == arLevelRules {
		// Reserve ~12 lines for detail + filter + help + flash
		h := m.height - 14
		if h < 6 {
			h = 6
		}
		return h
	}
	// Level 1: reserve ~4 lines for filter + help + flash
	h := m.height - 4
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
		m.patternList.SetSize(msg.Width, m.listHeight())
		if m.level == arLevelRules {
			m.ruleList.SetSize(msg.Width, m.listHeight())
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

		// --- Filter mode ---
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

		// --- Level-specific keys ---
		switch m.level {
		case arLevelPatterns:
			return m.updatePatternLevel(msg)
		case arLevelRules:
			return m.updateRuleLevel(msg)
		}
	}

	// Delegate to the active list
	var cmd tea.Cmd
	switch m.level {
	case arLevelPatterns:
		m.patternList, cmd = m.patternList.Update(msg)
	case arLevelRules:
		m.ruleList, cmd = m.ruleList.Update(msg)
	}
	return m, cmd
}

func (m arModel) updatePatternLevel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
	case "enter":
		m.drillDown()
		return m, nil
	case "d":
		m.togglePattern()
		return m, nil
	}

	var cmd tea.Cmd
	m.patternList, cmd = m.patternList.Update(msg)
	return m, cmd
}

func (m arModel) updateRuleLevel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.goBack()
		return m, nil
	case "/":
		m.filtering = true
		m.filterInput.Focus()
		return m, textinput.Blink
	case "d":
		m.toggleRule()
		return m, nil
	case "D":
		m.togglePatternFromRuleLevel()
		return m, nil
	}

	var cmd tea.Cmd
	m.ruleList, cmd = m.ruleList.Update(msg)
	return m, cmd
}

// drillDown enters Level 2 for the selected pattern.
func (m *arModel) drillDown() {
	item, ok := m.patternList.SelectedItem().(arPatternItem)
	if !ok {
		return
	}
	// Clear filter state
	m.filterText = ""
	m.filterInput.SetValue("")
	m.filtering = false

	m.level = arLevelRules
	m.buildRuleList(item.group.Pattern)
}

// goBack returns to Level 1.
func (m *arModel) goBack() {
	// Clear filter state
	m.filterText = ""
	m.filterInput.SetValue("")
	m.filtering = false

	m.level = arLevelPatterns
	m.buildPatternList()
}

// togglePattern toggles all rules in the selected pattern (Level 1).
func (m *arModel) togglePattern() {
	item, ok := m.patternList.SelectedItem().(arPatternItem)
	if !ok {
		return
	}

	allEnabled := item.group.Disabled == 0
	cwd, _ := os.Getwd()
	rulesPath := auditRulesPathForMode(m.mode, cwd)

	err := audit.TogglePattern(rulesPath, item.group.Pattern, allEnabled)
	if err != nil {
		m.flashMsg = tc.Red.Render("✗ " + err.Error())
		m.flashTicks = 3
		return
	}

	action := "Disabled"
	if allEnabled {
		action = "Enabled"
	}
	m.flashMsg = tc.Green.Render("✓ " + action + " " + item.group.Pattern)
	m.flashTicks = 3

	m.reloadRules()
	m.buildPatternList()
}

// toggleRule toggles a single rule (Level 2).
func (m *arModel) toggleRule() {
	item, ok := m.ruleList.SelectedItem().(arRuleItem)
	if !ok {
		return
	}

	newEnabled := !item.rule.Enabled
	cwd, _ := os.Getwd()
	rulesPath := auditRulesPathForMode(m.mode, cwd)

	err := audit.ToggleRule(rulesPath, item.rule.ID, newEnabled)
	if err != nil {
		m.flashMsg = tc.Red.Render("✗ " + err.Error())
		m.flashTicks = 3
		return
	}

	action := "Disabled"
	if newEnabled {
		action = "Enabled"
	}
	m.flashMsg = tc.Green.Render("✓ " + action + " " + item.shortID)
	m.flashTicks = 3

	curIdx := m.ruleList.Index()
	m.reloadRules()
	m.buildRuleList(m.currentPattern)
	if curIdx < len(m.ruleItems) {
		m.ruleList.Select(curIdx)
	}
}

// togglePatternFromRuleLevel toggles the entire current pattern (Level 2, D key).
func (m *arModel) togglePatternFromRuleLevel() {
	// Determine if all rules in current pattern are enabled
	allEnabled := true
	for _, ri := range m.ruleItems {
		if !ri.rule.Enabled {
			allEnabled = false
			break
		}
	}

	cwd, _ := os.Getwd()
	rulesPath := auditRulesPathForMode(m.mode, cwd)

	err := audit.TogglePattern(rulesPath, m.currentPattern, allEnabled)
	if err != nil {
		m.flashMsg = tc.Red.Render("✗ " + err.Error())
		m.flashTicks = 3
		return
	}

	action := "Disabled"
	if allEnabled {
		action = "Enabled"
	}
	m.flashMsg = tc.Green.Render("✓ " + action + " " + m.currentPattern)
	m.flashTicks = 3

	curIdx := m.ruleList.Index()
	m.reloadRules()
	m.buildRuleList(m.currentPattern)
	if curIdx < len(m.ruleItems) {
		m.ruleList.Select(curIdx)
	}
}

// reloadRules re-reads all rules from disk after a toggle mutation.
func (m *arModel) reloadRules() {
	audit.ResetGlobalCache()

	cwd, _ := os.Getwd()
	var rules []audit.CompiledRule
	var err error
	if m.mode == modeProject {
		rules, err = audit.ListRulesWithProject(cwd)
	} else {
		rules, err = audit.ListRules()
	}
	if err != nil {
		return // keep existing rules on error
	}
	m.allRules = rules
}

// applyFilter does a case-insensitive substring match and rebuilds the active list.
func (m *arModel) applyFilter() {
	term := strings.ToLower(m.filterText)

	switch m.level {
	case arLevelPatterns:
		if term == "" {
			items := make([]list.Item, len(m.patternItems))
			for i, pi := range m.patternItems {
				items[i] = pi
			}
			m.patternList.SetItems(items)
			m.patternList.ResetSelected()
			return
		}
		var matched []list.Item
		for _, pi := range m.patternItems {
			if strings.Contains(strings.ToLower(pi.FilterValue()), term) {
				matched = append(matched, pi)
			}
		}
		m.patternList.SetItems(matched)
		m.patternList.ResetSelected()

	case arLevelRules:
		if term == "" {
			items := make([]list.Item, len(m.ruleItems))
			for i, ri := range m.ruleItems {
				items[i] = ri
			}
			m.ruleList.SetItems(items)
			m.ruleList.ResetSelected()
			return
		}
		var matched []list.Item
		for _, ri := range m.ruleItems {
			if strings.Contains(strings.ToLower(ri.FilterValue()), term) {
				matched = append(matched, ri)
			}
		}
		m.ruleList.SetItems(matched)
		m.ruleList.ResetSelected()
	}
}

// ── View ──

func (m arModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	switch m.level {
	case arLevelPatterns:
		b.WriteString(m.patternList.View())
		b.WriteString("\n\n")
		b.WriteString(m.renderFilterBar())

		// Flash message
		if m.flashMsg != "" {
			b.WriteString("  " + m.flashMsg + "\n")
		}

		help := "↑↓ navigate  ←→ page  / filter  Enter drill  d toggle pattern  q quit"
		b.WriteString(tc.Help.Render(help))
		b.WriteString("\n")

	case arLevelRules:
		b.WriteString(m.ruleList.View())
		b.WriteString("\n\n")
		b.WriteString(m.renderFilterBar())

		// Detail panel for selected rule
		if item, ok := m.ruleList.SelectedItem().(arRuleItem); ok {
			b.WriteString(m.renderDetail(item))
		}

		// Flash message
		if m.flashMsg != "" {
			b.WriteString("  " + m.flashMsg + "\n")
		}

		help := "↑↓ navigate  ←→ page  / filter  d toggle rule  D toggle pattern  Esc back  q quit"
		b.WriteString(tc.Help.Render(help))
		b.WriteString("\n")
	}

	return b.String()
}

// renderFilterBar renders filter input or status.
func (m arModel) renderFilterBar() string {
	var noun string
	var matchCount, totalCount int

	switch m.level {
	case arLevelPatterns:
		noun = "patterns"
		totalCount = len(m.patternItems)
		if m.filterText != "" {
			matchCount = len(m.patternList.Items())
		} else {
			matchCount = totalCount
		}
	case arLevelRules:
		noun = "rules"
		totalCount = len(m.ruleItems)
		if m.filterText != "" {
			matchCount = len(m.ruleList.Items())
		} else {
			matchCount = totalCount
		}
	}

	return renderTUIFilterBar(
		m.filterInput.View(), m.filtering, m.filterText,
		matchCount, totalCount, 0,
		noun, "",
	)
}

// renderDetail renders the detail panel for a selected rule in Level 2.
func (m arModel) renderDetail(item arRuleItem) string {
	var b strings.Builder
	b.WriteString(tc.Separator.Render("  ─────────────────────────────────────────"))
	b.WriteString("\n")

	row := func(label, value string) {
		b.WriteString("  ")
		b.WriteString(tc.Label.Render(label))
		b.WriteString(tc.Value.Render(value))
		b.WriteString("\n")
	}

	r := item.rule

	row("ID:", r.ID)
	row("Severity:", tcSevStyle(r.Severity).Render(r.Severity))
	row("Message:", r.Message)

	// Regex — truncate at 80 chars
	regex := r.Regex
	if len(regex) > 80 {
		regex = regex[:77] + "..."
	}
	row("Regex:", tc.Dim.Render(regex))

	// Exclude
	if r.Exclude != "" {
		exclude := r.Exclude
		if len(exclude) > 80 {
			exclude = exclude[:77] + "..."
		}
		row("Exclude:", tc.Dim.Render(exclude))
	}

	// Status + source
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

// runAuditRulesTUI starts the bubbletea TUI for browsing/toggling audit rules.
func runAuditRulesTUI(rules []audit.CompiledRule, mode runMode) error {
	model := newARModel(rules, mode)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
