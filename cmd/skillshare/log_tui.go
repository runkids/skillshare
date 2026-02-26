package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"skillshare/internal/oplog"
	"skillshare/internal/ui"
)

// lc holds log-specific styles that don't belong in the shared tc palette.
var lc = struct {
	Bold  lipgloss.Style // counts, totals — bold without color
	Label lipgloss.Style // row labels — wider (22) than tc.Label (14)
}{
	Bold:  lipgloss.NewStyle().Bold(true),
	Label: lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Width(22),
}

// logLoadFn is a function that loads log items (runs in a goroutine inside the TUI).
type logLoadFn func() ([]logItem, error)

// logLoadedMsg is sent when the background load completes.
type logLoadedMsg struct {
	items []logItem
	err   error
}

// logDeletedMsg is sent when the background delete + reload completes.
type logDeletedMsg struct {
	items   []logItem
	deleted int
	err     error
}

// logTUIModel is the bubbletea model for the interactive log viewer.
type logTUIModel struct {
	list      list.Model
	modeLabel string // "global" or "project"
	quitting  bool

	// Async loading — spinner shown until data arrives
	loading     bool
	loadSpinner spinner.Model
	loadFn      logLoadFn
	loadErr     error

	// Application-level filter (matches list_tui pattern)
	allItems    []logItem
	filterText  string
	filterInput textinput.Model
	filtering   bool
	matchCount  int

	// Stats
	stats     logStats
	showStats bool

	// Detail panel scrolling
	detailScroll int
	termWidth    int
	termHeight   int

	// Delete selection
	selected       map[int]bool // key = allItems index; true = marked
	selCount       int
	configPath     string // needed for oplog.DeleteEntries
	confirmDelete  bool   // true = showing delete confirmation prompt
	deleting       bool   // true = delete in progress (spinner)
	lastDeletedMsg string // e.g. "Deleted 3 entries"
}

// newLogTUIModel creates a new TUI model.
// When loadFn is non-nil, items are loaded asynchronously (spinner shown).
// When loadFn is nil, items are used directly (pre-loaded).
func newLogTUIModel(loadFn logLoadFn, items []logItem, logLabel, modeLabel, configPath string) logTUIModel {
	var listItems []list.Item
	var allItems []logItem
	if loadFn == nil {
		listItems = make([]list.Item, len(items))
		for i, item := range items {
			listItems[i] = item
		}
		allItems = items
	}

	delegate := list.NewDefaultDelegate()
	configureDelegate(&delegate, false)

	l := list.New(listItems, delegate, 0, 0)
	l.Title = fmt.Sprintf("Log: %s (%s)", logLabel, modeLabel)
	l.Styles.Title = tc.ListTitle
	l.SetShowStatusBar(false)    // custom status line
	l.SetFilteringEnabled(false) // application-level filter
	l.SetShowHelp(false)
	l.SetShowPagination(false) // page info in custom status line

	// Loading spinner
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = tc.SpinnerStyle

	// Filter text input
	fi := textinput.New()
	fi.Prompt = "/ "
	fi.PromptStyle = tc.Filter
	fi.Cursor.Style = tc.Filter

	return logTUIModel{
		list:        l,
		modeLabel:   modeLabel,
		loading:     loadFn != nil,
		loadFn:      loadFn,
		loadSpinner: sp,
		allItems:    allItems,
		matchCount:  len(allItems),
		filterInput: fi,
		stats:       computeLogStatsFromItems(allItems),
		selected:    make(map[int]bool),
		configPath:  configPath,
	}
}

func (m logTUIModel) Init() tea.Cmd {
	if m.loading && m.loadFn != nil {
		return tea.Batch(m.loadSpinner.Tick, func() tea.Msg {
			items, err := m.loadFn()
			return logLoadedMsg{items: items, err: err}
		})
	}
	return nil
}

func (m logTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		// Horizontal layout: list takes left panel width; height = full minus overhead
		// Overhead: filter bar(1) + stats footer(1) + help bar(1) + newlines(2) = 5
		panelHeight := msg.Height - 5
		if panelHeight < 6 {
			panelHeight = 6
		}
		if m.termWidth >= 70 {
			m.list.SetSize(logListWidth(m.termWidth), panelHeight)
		} else {
			// Narrow fallback: vertical layout, list takes full width
			m.list.SetSize(msg.Width, panelHeight)
		}
		return m, nil

	case spinner.TickMsg:
		if m.loading || m.deleting {
			var cmd tea.Cmd
			m.loadSpinner, cmd = m.loadSpinner.Update(msg)
			return m, cmd
		}

	case logDeletedMsg:
		m.deleting = false
		if msg.err != nil {
			m.loadErr = msg.err
			m.quitting = true
			return m, tea.Quit
		}
		m.lastDeletedMsg = fmt.Sprintf("Deleted %d entries", msg.deleted)
		m.allItems = msg.items
		m.selected = make(map[int]bool)
		m.selCount = 0
		m.filterText = ""
		m.filterInput.SetValue("")
		m.matchCount = len(msg.items)
		m.stats = computeLogStatsFromItems(msg.items)
		listItems := make([]list.Item, len(msg.items))
		for i, item := range msg.items {
			listItems[i] = item
		}
		m.list.SetItems(listItems)
		m.list.ResetSelected()
		return m, nil

	case logLoadedMsg:
		m.loading = false
		// Keep loadFn for reload after delete (closure is lightweight)
		if msg.err != nil {
			m.loadErr = msg.err
			m.quitting = true
			return m, tea.Quit
		}
		m.allItems = msg.items
		m.matchCount = len(msg.items)
		m.stats = computeLogStatsFromItems(msg.items)
		listItems := make([]list.Item, len(msg.items))
		for i, item := range msg.items {
			listItems[i] = item
		}
		m.list.SetItems(listItems)
		return m, nil

	case tea.MouseMsg:
		if !m.loading && !m.showStats && m.termWidth >= 70 {
			leftWidth := logListWidth(m.termWidth)
			if msg.X > leftWidth {
				// Right panel: scroll detail with mouse wheel
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
		// Ignore keys while loading or deleting (except quit)
		if m.loading || m.deleting {
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				m.quitting = true
				return m, tea.Quit
			}
			return m, nil
		}

		// --- Confirm delete mode ---
		if m.confirmDelete {
			switch msg.String() {
			case "y":
				m.confirmDelete = false
				m.deleting = true
				return m, tea.Batch(m.loadSpinner.Tick, m.executeDelete())
			case "n", "esc":
				m.confirmDelete = false
				return m, nil
			}
			return m, nil
		}

		// --- Filter mode: route keys to filterInput ---
		if m.filtering {
			switch msg.String() {
			case "esc":
				m.filtering = false
				m.filterText = ""
				m.filterInput.SetValue("")
				m.applyLogFilter()
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
				m.applyLogFilter()
			}
			return m, cmd
		}

		// --- Normal mode ---
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "/":
			m.filtering = true
			m.filterInput.Focus()
			return m, textinput.Blink
		case "s":
			m.showStats = !m.showStats
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

		case " ": // space — toggle select current item for deletion
			idx := m.selectedAllItemsIndex()
			if idx < 0 {
				break
			}
			m.selected[idx] = !m.selected[idx]
			if m.selected[idx] {
				m.selCount++
			} else {
				delete(m.selected, idx)
				m.selCount--
			}
			m.allItems[idx].marked = m.selected[idx]
			m.lastDeletedMsg = "" // clear stale message
			m.rebuildListItems()
			return m, nil

		case "a": // toggle all/none (visible filtered items only)
			visibleIndices := m.visibleAllItemsIndices()
			selectAll := m.selCount < len(visibleIndices)

			// Clear all selections first, then re-select if toggling on
			for idx := range m.selected {
				if idx < len(m.allItems) {
					m.allItems[idx].marked = false
				}
			}
			m.selected = make(map[int]bool)
			m.selCount = 0

			if selectAll {
				for _, idx := range visibleIndices {
					m.selected[idx] = true
					m.allItems[idx].marked = true
					m.selCount++
				}
			}
			m.lastDeletedMsg = ""
			m.rebuildListItems()
			return m, nil

		case "d": // initiate delete of selected items
			if m.selCount == 0 {
				break
			}
			m.confirmDelete = true
			return m, nil
		}
	}

	prevIdx := m.list.Index()
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	if m.list.Index() != prevIdx {
		m.detailScroll = 0 // reset scroll when selection changes
	}
	return m, cmd
}

// applyLogFilter does a case-insensitive substring match over allItems.
func (m *logTUIModel) applyLogFilter() {
	term := strings.ToLower(m.filterText)

	if term == "" {
		all := make([]list.Item, len(m.allItems))
		for i, item := range m.allItems {
			all[i] = item
		}
		m.matchCount = len(m.allItems)
		m.list.SetItems(all)
		m.list.ResetSelected()
		m.stats = computeLogStatsFromItems(m.allItems)
		return
	}

	var matchedItems []logItem
	var matched []list.Item
	for _, item := range m.allItems {
		if strings.Contains(strings.ToLower(item.FilterValue()), term) {
			matchedItems = append(matchedItems, item)
			matched = append(matched, item)
		}
	}
	m.matchCount = len(matched)
	m.list.SetItems(matched)
	m.list.ResetSelected()
	m.stats = computeLogStatsFromItems(matchedItems)
}

// selectedAllItemsIndex returns the allItems index for the currently highlighted list item.
// Returns -1 if nothing is selected or the item can't be matched.
func (m *logTUIModel) selectedAllItemsIndex() int {
	sel, ok := m.list.SelectedItem().(logItem)
	if !ok {
		return -1
	}
	// Match by entry identity (pointer-free: use content)
	for i, item := range m.allItems {
		if item.entry.Timestamp == sel.entry.Timestamp &&
			item.entry.Command == sel.entry.Command &&
			item.entry.Status == sel.entry.Status &&
			item.entry.Duration == sel.entry.Duration &&
			item.source == sel.source {
			return i
		}
	}
	return -1
}

// visibleAllItemsIndices returns allItems indices for all currently visible list items.
// When a filter is active, only matching items are in the list.
func (m *logTUIModel) visibleAllItemsIndices() []int {
	listItems := m.list.Items()
	indices := make([]int, 0, len(listItems))
	for _, li := range listItems {
		item, ok := li.(logItem)
		if !ok {
			continue
		}
		for i, ai := range m.allItems {
			if ai.entry.Timestamp == item.entry.Timestamp &&
				ai.entry.Command == item.entry.Command &&
				ai.entry.Status == item.entry.Status &&
				ai.entry.Duration == item.entry.Duration &&
				ai.source == item.source {
				indices = append(indices, i)
				break
			}
		}
	}
	return indices
}

// rebuildListItems reconstructs list.Items from allItems (preserving marked state).
// Re-applies active filter if one exists.
func (m *logTUIModel) rebuildListItems() {
	curIdx := m.list.Index()
	if m.filterText != "" {
		m.applyLogFilter()
	} else {
		items := make([]list.Item, len(m.allItems))
		for i, item := range m.allItems {
			items[i] = item
		}
		m.list.SetItems(items)
		m.matchCount = len(items)
	}
	// Restore cursor position
	if curIdx < len(m.list.Items()) {
		m.list.Select(curIdx)
	}
}

// executeDelete performs the actual deletion in a background goroutine, then reloads.
func (m *logTUIModel) executeDelete() tea.Cmd {
	// Collect entries to delete, grouped by source file
	var opsMatches, auditMatches []oplog.Entry
	for idx, marked := range m.selected {
		if !marked || idx >= len(m.allItems) {
			continue
		}
		item := m.allItems[idx]
		switch item.source {
		case "audit":
			auditMatches = append(auditMatches, item.entry)
		default:
			opsMatches = append(opsMatches, item.entry)
		}
	}

	configPath := m.configPath
	loadFn := m.loadFn
	return func() tea.Msg {
		totalDeleted := 0

		if len(opsMatches) > 0 {
			n, err := oplog.DeleteEntries(configPath, oplog.OpsFile, opsMatches)
			if err != nil {
				return logDeletedMsg{err: err}
			}
			totalDeleted += n
		}
		if len(auditMatches) > 0 {
			n, err := oplog.DeleteEntries(configPath, oplog.AuditFile, auditMatches)
			if err != nil {
				return logDeletedMsg{err: err}
			}
			totalDeleted += n
		}

		// Reload items using the same loadFn if available
		if loadFn != nil {
			items, err := loadFn()
			if err != nil {
				return logDeletedMsg{err: err}
			}
			return logDeletedMsg{items: items, deleted: totalDeleted}
		}

		// Fallback: re-read both log files
		opsEntries, err := oplog.Read(configPath, oplog.OpsFile, 0)
		if err != nil {
			return logDeletedMsg{err: err}
		}
		auditEntries, err := oplog.Read(configPath, oplog.AuditFile, 0)
		if err != nil {
			return logDeletedMsg{err: err}
		}
		items := append(toLogItems(opsEntries, "operations"), toLogItems(auditEntries, "audit")...)
		sort.Slice(items, func(i, j int) bool {
			return items[i].entry.Timestamp > items[j].entry.Timestamp
		})
		return logDeletedMsg{items: items, deleted: totalDeleted}
	}
}

func (m logTUIModel) View() string {
	if m.quitting {
		return ""
	}

	// Loading / deleting state — spinner + message
	if m.loading {
		return fmt.Sprintf("\n  %s Loading log entries...\n", m.loadSpinner.View())
	}
	if m.deleting {
		return fmt.Sprintf("\n  %s Deleting entries...\n", m.loadSpinner.View())
	}

	var b strings.Builder

	// Stats overlay — full screen, unchanged
	if m.showStats {
		b.WriteString("\n")
		b.WriteString(m.renderStatsPanel())
		b.WriteString("\n")

		help := "s back to list  q quit"
		b.WriteString(tc.Help.Render(help))
		b.WriteString("\n")
		return b.String()
	}

	// Narrow terminal (<70 cols): vertical fallback
	if m.termWidth < 70 {
		return m.viewVertical()
	}

	// ── Horizontal split layout ──
	// Filter bar (full width, top)
	b.WriteString(m.renderLogFilterBar())

	// Panel height: terminal minus filter(1) + stats footer(1) + help(1) + newlines(2)
	panelHeight := m.termHeight - 5
	if panelHeight < 6 {
		panelHeight = 6
	}

	leftWidth := logListWidth(m.termWidth)
	rightWidth := logDetailPanelWidth(m.termWidth)

	// Left panel: list
	leftPanel := lipgloss.NewStyle().
		Width(leftWidth).MaxWidth(leftWidth).
		Height(panelHeight).MaxHeight(panelHeight).
		Render(m.list.View())

	// Border column
	borderStyle := tc.Border.
		Height(panelHeight).MaxHeight(panelHeight)
	borderCol := strings.Repeat("│\n", panelHeight)
	borderPanel := borderStyle.Render(strings.TrimRight(borderCol, "\n"))

	// Right panel: detail for selected item
	var detailStr string
	if item, ok := m.list.SelectedItem().(logItem); ok {
		detailContent := renderLogDetailPanel(item)
		detailStr = m.applyDetailScroll(detailContent, panelHeight)
	}
	rightPanel := lipgloss.NewStyle().
		Width(rightWidth).MaxWidth(rightWidth).
		Height(panelHeight).MaxHeight(panelHeight).
		PaddingLeft(1).
		Render(detailStr)

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, borderPanel, rightPanel)
	b.WriteString(body)
	b.WriteString("\n")

	// Stats footer
	b.WriteString(m.renderStatsFooter())

	b.WriteString(tc.Help.Render(m.logHelpBar()))
	b.WriteString("\n")

	return b.String()
}

// viewVertical renders the original vertical layout for narrow terminals.
func (m logTUIModel) viewVertical() string {
	var b strings.Builder

	b.WriteString(m.list.View())
	b.WriteString("\n\n")

	b.WriteString(m.renderLogFilterBar())

	if item, ok := m.list.SelectedItem().(logItem); ok {
		detailContent := renderLogDetailPanel(item)
		// Vertical: detail takes remaining space below the list
		detailHeight := m.termHeight - m.termHeight*2/5 - 7
		b.WriteString(m.applyDetailScroll(detailContent, detailHeight))
	}

	b.WriteString(m.renderStatsFooter())

	b.WriteString(tc.Help.Render(m.logHelpBar()))
	b.WriteString("\n")

	return b.String()
}

// logHelpBar returns the context-sensitive help text for the bottom bar.
func (m logTUIModel) logHelpBar() string {
	if m.confirmDelete {
		return fmt.Sprintf("Delete %d entries? y confirm  n cancel", m.selCount)
	}

	var parts []string
	parts = append(parts, "↑↓ navigate  ←→ page  / filter")

	if m.selCount > 0 {
		parts = append(parts, fmt.Sprintf("d delete(%d)  space toggle  a all", m.selCount))
	} else {
		parts = append(parts, "space select  a all")
	}

	parts = append(parts, "s stats  q quit")

	help := strings.Join(parts, "  ")

	if m.lastDeletedMsg != "" {
		help = tc.Green.Render(m.lastDeletedMsg) + "  " + help
	}

	return help
}

// renderLogFilterBar renders the status line for the log TUI.
func (m logTUIModel) renderLogFilterBar() string {
	return renderTUIFilterBar(
		m.filterInput.View(), m.filtering, m.filterText,
		m.matchCount, len(m.allItems), 0,
		"entries", renderPageInfoFromPaginator(m.list.Paginator),
	)
}

// logListWidth returns the left panel width for horizontal layout.
// 40% of terminal, clamped to [30, 60].
func logListWidth(termWidth int) int {
	w := termWidth * 2 / 5
	if w < 30 {
		w = 30
	}
	if w > 60 {
		w = 60
	}
	return w
}

// logDetailPanelWidth returns the right detail panel width.
// termWidth minus list width minus border/gap (3 chars).
func logDetailPanelWidth(termWidth int) int {
	w := termWidth - logListWidth(termWidth) - 3
	if w < 30 {
		w = 30
	}
	return w
}

// applyDetailScroll wraps a detail panel string with height limit and scroll offset.
// viewHeight is the maximum number of visible lines for the detail area.
func (m logTUIModel) applyDetailScroll(content string, viewHeight int) string {
	lines := strings.Split(content, "\n")

	maxDetailLines := viewHeight
	if maxDetailLines < 5 {
		maxDetailLines = 5
	}

	totalLines := len(lines)
	if totalLines <= maxDetailLines {
		return content // fits without scrolling
	}

	// Clamp scroll offset
	maxScroll := totalLines - maxDetailLines
	offset := m.detailScroll
	if offset > maxScroll {
		offset = maxScroll
	}

	visible := lines[offset:]
	if len(visible) > maxDetailLines {
		visible = visible[:maxDetailLines]
	}

	var b strings.Builder
	for _, line := range visible {
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Scroll indicator
	if offset > 0 || offset < maxScroll {
		indicator := fmt.Sprintf("  ── Ctrl+d/u scroll (%d/%d) ──", offset+1, maxScroll+1)
		b.WriteString(tc.Dim.Render(indicator))
		b.WriteString("\n")
	}

	return b.String()
}

// renderLogDetailPanel renders structured details for the selected log entry.
func renderLogDetailPanel(item logItem) string {
	var b strings.Builder

	row := func(label, value string) {
		b.WriteString(lc.Label.Render(label))
		b.WriteString(tc.Value.Render(value))
		b.WriteString("\n")
	}

	e := item.entry

	// Full timestamp
	row("Timestamp:", e.Timestamp)

	// Command — cyan to match CLI palette
	row("Command:", tc.Cyan.Render(strings.ToUpper(e.Command)))

	// Status with color
	statusDisplay := e.Status
	switch e.Status {
	case "ok":
		statusDisplay = tc.Green.Render(e.Status)
	case "error", "blocked":
		statusDisplay = tc.Red.Render(e.Status)
	case "partial":
		statusDisplay = tc.Yellow.Render(e.Status)
	}
	row("Status:", statusDisplay)

	// Duration
	if dur := formatLogDuration(e.Duration); dur != "" {
		row("Duration:", dur)
	}

	// Source log file
	if item.source != "" {
		row("Source:", item.source)
	}

	// Message
	if e.Message != "" {
		row("Message:", e.Message)
	}

	// Structured args via formatLogDetailPairs — colorize semantic values
	pairs := formatLogDetailPairs(e)
	const maxBulletItems = 100 // right panel has dedicated space + scroll

	for _, p := range pairs {
		// List fields: render as multi-line bullet list for readability
		if p.isList && len(p.listValues) > 0 {
			b.WriteString(lc.Label.Render(p.key + ":"))
			b.WriteString("\n")
			show := p.listValues
			remaining := 0
			if len(show) > maxBulletItems {
				remaining = len(show) - maxBulletItems
				show = show[:maxBulletItems]
			}
			for _, v := range show {
				b.WriteString("    - " + tc.Value.Render(v) + "\n")
			}
			if remaining > 0 {
				summary := fmt.Sprintf("    ... and %d more", remaining)
				b.WriteString(tc.Dim.Render(summary) + "\n")
			}
			continue
		}

		value := p.value
		if value == "" {
			continue
		}

		// Colorize only severity/status fields to avoid visual noise
		switch {
		case strings.Contains(p.key, "failed") || strings.Contains(p.key, "scan-errors"):
			value = tc.Red.Render(value)
		case strings.Contains(p.key, "warning"):
			value = tc.Yellow.Render(value)
		case p.key == "risk":
			value = colorizeRiskValue(value)
		case p.key == "threshold":
			value = colorizeThreshold(value)
		case strings.HasPrefix(p.key, "severity"):
			value = colorizeSeverityBreakdown(value)
		}

		row(p.key+":", value)
	}

	return b.String()
}

// severityStyles maps the 5 severity levels (c/h/m/l/i) to lipgloss styles.
var severityStyles = []lipgloss.Style{
	tc.Critical, tc.High, tc.Medium, tc.Low, tc.Info,
}

// colorizeSeverityBreakdown colors each number in "0/0/1/0/0" to match audit summary.
func colorizeSeverityBreakdown(value string) string {
	parts := strings.Split(value, "/")
	if len(parts) != 5 {
		return value
	}
	for i, p := range parts {
		parts[i] = severityStyles[i].Render(p)
	}
	sep := tc.Dim.Render("/")
	return strings.Join(parts, sep)
}

// colorizeThreshold applies color based on audit threshold level.
func colorizeThreshold(value string) string {
	if ui.SeverityColorID(value) == "" {
		return tc.Green.Render(value)
	}
	return tcSevStyle(value).Render(value)
}

// colorizeRiskValue applies color based on the risk label embedded in the value string.
// e.g. "CRITICAL (85/100)" → red, "LOW (15/100)" → green
func colorizeRiskValue(value string) string {
	sev := strings.SplitN(strings.ToUpper(value), " ", 2)[0]
	sev = strings.TrimRight(sev, "(")
	if ui.SeverityColorID(sev) == "" {
		return tc.Green.Render(value)
	}
	return tcSevStyle(sev).Render(value)
}

// computeLogStatsFromItems converts logItems to oplog entries and computes stats.
func computeLogStatsFromItems(items []logItem) logStats {
	entries := make([]oplog.Entry, len(items))
	for i, item := range items {
		entries[i] = item.entry
	}
	return computeLogStats(entries)
}

// renderStatsFooter renders a compact stats line above the help bar.
func (m logTUIModel) renderStatsFooter() string {
	if m.stats.Total == 0 {
		return ""
	}

	rateStyle := statsSuccessRateColor(m.stats.SuccessRate)

	parts := []string{
		tc.Dim.Render(fmt.Sprintf("%d ops", m.stats.Total)),
		rateStyle.Render(fmt.Sprintf("✓ %.1f%%", m.stats.SuccessRate*100)),
	}

	if m.stats.LastOperation != nil {
		ts, err := time.Parse(time.RFC3339, m.stats.LastOperation.Timestamp)
		if err == nil {
			lastPart := tc.Dim.Render("last: ") +
				tc.Cyan.Render(m.stats.LastOperation.Command) +
				tc.Dim.Render(fmt.Sprintf(" %s ago", formatRelativeTime(time.Since(ts))))
			parts = append(parts, lastPart)
		}
	}

	sep := tc.Dim.Render(" | ")
	return "  " + strings.Join(parts, sep) + "\n"
}

// renderStatsPanel renders the full stats overlay panel.
func (m logTUIModel) renderStatsPanel() string {
	var b strings.Builder

	b.WriteString(tc.Title.Render("  Operation Log Summary"))
	b.WriteString("\n")
	b.WriteString(tc.Dim.Render("  " + strings.Repeat("─", 50)))
	b.WriteString("\n\n")

	if m.stats.Total == 0 {
		b.WriteString(tc.Dim.Render("  No entries"))
		b.WriteString("\n")
		return b.String()
	}

	// ── Overview row ──
	okTotal := 0
	for _, cs := range m.stats.ByCommand {
		okTotal += cs.OK
	}
	rateColor := statsSuccessRateColor(m.stats.SuccessRate)
	b.WriteString(fmt.Sprintf("  %s  %s\n\n",
		tc.Dim.Render("Total:"),
		lc.Bold.Render(fmt.Sprintf("%d", m.stats.Total)),
	))
	b.WriteString(fmt.Sprintf("  %s  %s %s\n\n",
		tc.Dim.Render("OK:"),
		rateColor.Render(fmt.Sprintf("%d/%d", okTotal, m.stats.Total)),
		tc.Dim.Render(fmt.Sprintf("(%.1f%%)", m.stats.SuccessRate*100)),
	))

	// ── Command breakdown with horizontal bars ──
	header := fmt.Sprintf("  %-12s  %-20s  %s", "Command", "", "OK")
	b.WriteString(tc.Dim.Render(header))
	b.WriteString("\n")
	b.WriteString(tc.Dim.Render("  " + strings.Repeat("─", 42)))
	b.WriteString("\n")

	type cmdEntry struct {
		name string
		cs   commandStats
	}
	var cmds []cmdEntry
	for name, cs := range m.stats.ByCommand {
		cmds = append(cmds, cmdEntry{name, cs})
	}
	sort.Slice(cmds, func(i, j int) bool { return cmds[i].cs.Total > cmds[j].cs.Total })

	maxCount := 0
	if len(cmds) > 0 {
		maxCount = cmds[0].cs.Total
	}

	const cmdBarWidth = 20
	for _, cmd := range cmds {
		// Proportional bar
		barLen := cmdBarWidth
		if maxCount > 0 {
			barLen = cmd.cs.Total * cmdBarWidth / maxCount
		}
		if barLen < 1 {
			barLen = 1
		}

		// Color the bar: green portion for OK, red for errors
		okBarLen := 0
		if cmd.cs.Total > 0 {
			okBarLen = cmd.cs.OK * barLen / cmd.cs.Total
		}
		errBarLen := barLen - okBarLen

		cmdBar := tc.Green.Render(strings.Repeat("▓", okBarLen))
		if errBarLen > 0 {
			cmdBar += tc.Red.Render(strings.Repeat("▓", errBarLen))
		}
		padding := strings.Repeat(" ", cmdBarWidth-barLen)

		// "✓6/9" format — ok out of total, self-explanatory
		okRatio := fmt.Sprintf("✓%d/%d", cmd.cs.OK, cmd.cs.Total)
		ratioColor := tc.Green
		if cmd.cs.OK < cmd.cs.Total {
			ratioColor = tc.Red
		}

		b.WriteString(fmt.Sprintf("  %s  %s%s  %s\n",
			tc.Dim.Render(fmt.Sprintf("%-12s", cmd.name)),
			cmdBar, padding, ratioColor.Render(okRatio)))
	}

	b.WriteString("\n")

	// ── Status distribution ──
	okTotal, errTotal, partialTotal, blockedTotal := 0, 0, 0, 0
	for _, cs := range m.stats.ByCommand {
		okTotal += cs.OK
		errTotal += cs.Error
		partialTotal += cs.Partial
		blockedTotal += cs.Blocked
	}

	b.WriteString(tc.Dim.Render("  " + strings.Repeat("─", 50)))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s %s",
		tc.Dim.Render("Status:"),
		tc.Green.Render(fmt.Sprintf("✓ %d ok", okTotal))))
	if errTotal > 0 {
		b.WriteString(fmt.Sprintf("  %s", tc.Red.Render(fmt.Sprintf("✗ %d error", errTotal))))
	}
	if partialTotal > 0 {
		b.WriteString(fmt.Sprintf("  %s", tc.Yellow.Render(fmt.Sprintf("◐ %d partial", partialTotal))))
	}
	if blockedTotal > 0 {
		b.WriteString(fmt.Sprintf("  %s", tc.Red.Render(fmt.Sprintf("⊘ %d blocked", blockedTotal))))
	}
	b.WriteString("\n")

	// ── Last operation ──
	if m.stats.LastOperation != nil {
		ts, err := time.Parse(time.RFC3339, m.stats.LastOperation.Timestamp)
		if err == nil {
			ago := formatRelativeTime(time.Since(ts))
			b.WriteString(fmt.Sprintf("  %s %s %s\n",
				tc.Dim.Render("Last op:"),
				tc.Cyan.Render(m.stats.LastOperation.Command),
				tc.Dim.Render(fmt.Sprintf("(%s ago)", ago))))
		}
	}

	return b.String()
}

// statsSuccessRateColor returns a lipgloss style based on the success rate.
func statsSuccessRateColor(rate float64) lipgloss.Style {
	switch {
	case rate >= 0.9:
		return tc.Green.Bold(true)
	case rate >= 0.7:
		return tc.Yellow.Bold(true)
	default:
		return tc.Red.Bold(true)
	}
}

// runLogTUI starts the bubbletea TUI for the log viewer (pre-loaded items).
func runLogTUI(items []logItem, logLabel, modeLabel, configPath string) error {
	if len(items) == 0 {
		fmt.Printf("No %s log entries\n", strings.ToLower(logLabel))
		return nil
	}

	model := newLogTUIModel(nil, items, logLabel, modeLabel, configPath)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}

// runLogTUIAsync starts the bubbletea TUI with async loading (spinner shown).
func runLogTUIAsync(loadFn logLoadFn, logLabel, modeLabel, configPath string) error {
	model := newLogTUIModel(loadFn, nil, logLabel, modeLabel, configPath)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	finalModel, err := p.Run()
	if err != nil {
		return err
	}
	if m, ok := finalModel.(logTUIModel); ok && m.loadErr != nil {
		return m.loadErr
	}
	return nil
}
