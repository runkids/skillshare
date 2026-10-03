package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"skillshare/internal/oplog"
	"skillshare/internal/theme"
	"skillshare/internal/ui"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

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
	logLabel  string // "Operations" or "Audit"
	modeLabel string // "global" or "project"
	quitting  bool

	// Async loading — spinner shown until data arrives
	loading     bool
	loadSpinner spinner.Model
	loadFn      logLoadFn
	loadErr     error
	emptyResult bool // true when async load returned zero entries

	// Application-level filter (matches list_tui pattern)
	allItems    []logItem
	filterText  string
	filterInput textinput.Model
	filtering   bool
	matchCount  int

	// Stats
	stats     logStats
	showStats bool // the Stats tab is open
	showKeys  bool // ? swaps the detail panel for the full key list

	// Detail panel scrolling
	detailScroll int
	termWidth    int
	termHeight   int

	// Delete selection
	selected       map[int]bool // key = allItems index; true = marked
	selCount       int
	configPath     string    // needed for oplog.DeleteEntries
	confirmDelete  bool      // true = asking on the key line
	deleteItems    []logItem // what the pending delete removes
	deleting       bool      // true = delete in progress (spinner)
	lastDeletedMsg string    // e.g. "✓ Deleted 3 entries"
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

	l := list.New(listItems, logDelegate{}, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)    // counts are on the title line
	l.SetFilteringEnabled(false) // application-level filter
	l.SetShowHelp(false)
	l.SetShowPagination(false) // page info in custom status line

	// Loading spinner
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = theme.Accent()

	// Filter text input
	fi := newTUIFilterInput("type to match a command, status or detail")

	return logTUIModel{
		list:        l,
		logLabel:    logLabel,
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
		bodyHeight := max(msg.Height-frameChrome, 6)
		if m.termWidth >= tuiNarrowSplitWidth {
			m.list.SetSize(logListWidth(m.termWidth), bodyHeight)
		} else {
			// Narrow: the list sits above the details
			m.list.SetSize(msg.Width, max(bodyHeight/2, 4))
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
		m.lastDeletedMsg = "  " + theme.Success().Render("✓") + " Deleted " + countNoun(msg.deleted, "entry")
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
		if len(msg.items) == 0 {
			m.emptyResult = true
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
		if !m.loading && !m.showStats && m.termWidth >= tuiNarrowSplitWidth {
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
			case "y", "Y", "enter":
				m.confirmDelete = false
				m.deleting = true
				return m, tea.Batch(m.loadSpinner.Tick, m.executeDelete())
			case "n", "N", "esc", "q":
				m.confirmDelete = false
				m.deleteItems = nil
				return m, nil
			}
			return m, nil
		}

		// --- Filter mode: route keys to filterInput ---
		if m.filtering {
			cmd := handleTUIFilterKey(msg, &m.filtering, &m.filterText, &m.filterInput, m.applyLogFilter)
			return m, cmd
		}

		// --- Normal mode ---
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "esc":
			switch {
			case m.showKeys:
				m.showKeys = false
			case m.showStats:
				m.showStats = false
			case m.filterText != "":
				m.filterText = ""
				m.filterInput.SetValue("")
				m.applyLogFilter()
			default:
				m.quitting = true
				return m, tea.Quit
			}
			return m, nil
		case "?":
			m.showKeys = !m.showKeys
			return m, nil
		case "tab", "shift+tab":
			m.showStats = !m.showStats
			return m, nil
		}
		if m.showStats {
			return m, nil // the Stats tab has no list to act on
		}
		switch msg.String() {
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

		case "d": // delete the selection, or the entry under the cursor
			m.deleteItems = m.deleteTargets()
			if len(m.deleteItems) == 0 {
				break
			}
			m.confirmDelete = true
			m.lastDeletedMsg = ""
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

// deleteTargets is what d removes: the selection, or the entry under the
// cursor when nothing is selected.
func (m *logTUIModel) deleteTargets() []logItem {
	var items []logItem
	for i, item := range m.allItems {
		if m.selected[i] {
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		if item, ok := m.list.SelectedItem().(logItem); ok {
			items = append(items, item)
		}
	}
	return items
}

// executeDelete performs the actual deletion in a background goroutine, then reloads.
func (m *logTUIModel) executeDelete() tea.Cmd {
	// Collect entries to delete, grouped by source file
	var opsMatches, auditMatches []oplog.Entry
	for _, item := range m.deleteItems {
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
	title := m.renderTitleLine()
	if m.loading {
		return title + "\n\n" + renderBusyLine(m.termWidth, m.loadSpinner.View(), "Loading log entries…")
	}
	bodyHeight := max(m.termHeight-frameChrome, 6)
	if m.showStats {
		right := m.renderStatsPanel()
		if m.showKeys {
			right = renderKeysPanel(logKeyGroups)
		}
		body := lipgloss.NewStyle().Height(bodyHeight).MaxHeight(bodyHeight).PaddingLeft(1).Render(right)
		return title + "\n\n" + body + "\n" + m.renderBottom()
	}
	if m.termWidth < tuiNarrowSplitWidth {
		listHeight := max(bodyHeight/2, 4)
		detailHeight := max(bodyHeight-listHeight-1, 4)
		detail := lipgloss.NewStyle().Height(detailHeight).MaxHeight(detailHeight).PaddingLeft(1).
			Render(m.renderRight(m.termWidth-2, detailHeight))
		return title + "\n\n" + m.list.View() + "\n\n" + detail + "\n" + m.renderBottom()
	}
	leftWidth := logListWidth(m.termWidth)
	rightWidth := logDetailPanelWidth(m.termWidth)
	return title + "\n\n" +
		renderFrameSplit(m.list.View(), m.renderRight(rightWidth-2, bodyHeight), leftWidth, rightWidth, bodyHeight) + "\n" +
		m.renderBottom()
}

// renderTitleLine renders the scope, the entry count, the success rate and
// the Entries / Stats tabs.
func (m logTUIModel) renderTitleLine() string {
	facts := []string{m.modeLabel}
	if m.logLabel == "Audit" {
		facts = append(facts, "audit log")
	}
	if !m.loading {
		count := countNoun(len(m.allItems), "entry")
		if m.filterText != "" {
			count = formatNumber(m.matchCount) + " of " + count
		}
		facts = append(facts, count)
	}
	if m.stats.Total > 0 && !m.loading {
		// Last, because a colored fact ends the dim run of facts.
		facts = append(facts, statsSuccessRateColor(m.stats.SuccessRate).Render(fmt.Sprintf("%.0f%% ok", m.stats.SuccessRate*100)))
	}
	return renderFrameTitle(m.termWidth, "log", facts, []frameTab{{"Entries", !m.showStats}, {"Stats", m.showStats}})
}

// renderRight renders the detail panel, or the key list while ? is on.
func (m logTUIModel) renderRight(width, height int) string {
	if m.showKeys {
		return renderKeysPanel(logKeyGroups)
	}
	item, ok := m.list.SelectedItem().(logItem)
	if !ok {
		return ""
	}
	detail, _ := wrapAndScroll(renderLogDetailPanel(item), width, m.detailScroll, height)
	return detail
}

// renderBottom renders the note line and the key line. The delete
// confirmation, the filter input and a running delete take over the key line.
func (m logTUIModel) renderBottom() string {
	note := m.lastDeletedMsg
	var line string
	switch {
	case m.deleting:
		line = renderBusyLine(m.termWidth, m.loadSpinner.View(), "Deleting "+countNoun(len(m.deleteItems), "entry")+"…")
	case m.confirmDelete:
		line = renderConfirmLine(m.termWidth, "Delete "+countNoun(len(m.deleteItems), "entry")+"?", true, "")
	case m.filtering:
		line = renderFilterLine(m.termWidth, m.filterInput.View(), m.matchCount)
	case m.showKeys:
		line = renderKeyLine(m.termWidth, []keyHint{{"?/esc", "close"}}, "")
	case m.showStats:
		line = renderKeyLine(m.termWidth, []keyHint{{"tab", "entries"}, {"esc", "back"}, {"?", "keys"}}, "")
	default:
		filter := keyHint{"/", "filter"}
		if m.filterText != "" {
			filter = keyHint{"esc", "clear filter"}
		}
		hints := []keyHint{{"↑↓", "move"}, {"space", "select"}, filter, {"d", "delete"}, {"tab", "stats"}, {"?", "keys"}}
		right := framePosition(m.list.Index()+1, m.matchCount)
		if m.selCount > 0 {
			right = formatNumber(m.selCount) + " selected"
		}
		line = renderKeyLine(m.termWidth, hints, right)
	}
	return note + "\n" + line
}

// logKeyGroups lists every key for the ? panel.
var logKeyGroups = []keyGroup{
	{"Move", []keyHint{
		{"↑↓", "move"},
		{"←→", "page"},
		{"/", "filter"},
		{"ctrl+d/u", "scroll the details"},
		{"tab", "switch between Entries and Stats"},
		{"esc", "clear the filter, then quit"},
		{"q", "quit"},
	}},
	{"Select", []keyHint{
		{"space", "select"},
		{"a", "select all, or none"},
	}},
	{"Actions", []keyHint{
		{"d", "delete the selection, or the entry under the cursor"},
	}},
}

// logListWidth returns the left panel width for horizontal layout.
// 25% of terminal, clamped to [30, 45] — aligned with audit TUI proportions.
func logListWidth(termWidth int) int {
	w := termWidth / 4
	if w < 30 {
		w = 30
	}
	if w > 45 {
		w = 45
	}
	return w
}

// logDetailPanelWidth returns the right detail panel width.
func logDetailPanelWidth(termWidth int) int {
	return max(termWidth-logListWidth(termWidth), 30)
}

// renderLogDetailPanel renders structured details for the selected log entry.
func renderLogDetailPanel(item logItem) string {
	var b strings.Builder
	e := item.entry
	pairs := formatLogDetailPairs(e)

	labelWidth := 9
	for _, p := range pairs {
		labelWidth = max(labelWidth, min(lipgloss.Width(p.key), 20))
	}
	row := func(label, value string) {
		b.WriteString(theme.Dim().Width(labelWidth + 1).Render(label))
		b.WriteString(value)
		b.WriteString("\n")
	}

	b.WriteString(theme.Primary().Bold(true).Render(e.Command))
	b.WriteString("\n\n")

	row("Status", ui.StyledMark(logStatusMark(e.Status))+" "+e.Status)
	when := e.Timestamp
	if ts, err := time.Parse(time.RFC3339, e.Timestamp); err == nil {
		when = ts.Local().Format("2006-01-02 15:04:05") + theme.Dim().Render(" · "+timeAgo(ts))
	}
	row("When", when)
	if dur := formatLogDuration(e.Duration); dur != "" {
		row("Took", dur)
	}
	if item.source == "audit" {
		row("Log", "audit")
	}
	if e.Message != "" {
		row("Message", e.Message)
	}

	const maxBulletItems = 100 // right panel has dedicated space + scroll

	for _, p := range pairs {
		// List fields: render as multi-line bullet list for readability
		if p.isList && len(p.listValues) > 0 {
			b.WriteString(theme.Dim().Render(p.key))
			b.WriteString("\n")
			show := p.listValues
			remaining := 0
			if len(show) > maxBulletItems {
				remaining = len(show) - maxBulletItems
				show = show[:maxBulletItems]
			}
			for _, v := range show {
				b.WriteString("  - " + v + "\n")
			}
			if remaining > 0 {
				summary := fmt.Sprintf("  ... and %d more", remaining)
				b.WriteString(theme.Dim().Render(summary) + "\n")
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
			value = theme.Danger().Render(value)
		case strings.Contains(p.key, "warning"):
			value = theme.Warning().Render(value)
		case p.key == "risk":
			value = colorizeRiskValue(value)
		case p.key == "threshold":
			value = colorizeThreshold(value)
		case strings.HasPrefix(p.key, "severity"):
			value = colorizeSeverityBreakdown(value)
		}

		row(p.key, value)
	}

	return b.String()
}

// severityStyles maps the 5 severity levels (c/h/m/l/i) to lipgloss styles.
var severityStyles = []lipgloss.Style{
	theme.Severity("critical"), theme.Severity("high"), theme.Severity("medium"), theme.Severity("low"), theme.Severity("info"),
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
	sep := theme.Dim().Render("/")
	return strings.Join(parts, sep)
}

// colorizeThreshold applies color based on audit threshold level.
func colorizeThreshold(value string) string {
	if ui.SeverityColorID(value) == "" {
		return theme.Success().Render(value)
	}
	return theme.SeverityStyle(value).Render(value)
}

// colorizeRiskValue applies color based on the risk label embedded in the value string.
// e.g. "CRITICAL (85/100)" → red, "LOW (15/100)" → green
func colorizeRiskValue(value string) string {
	sev := strings.SplitN(strings.ToUpper(value), " ", 2)[0]
	sev = strings.TrimRight(sev, "(")
	if ui.SeverityColorID(sev) == "" {
		return theme.Success().Render(value)
	}
	return theme.SeverityStyle(sev).Render(value)
}

// computeLogStatsFromItems converts logItems to oplog entries and computes stats.
func computeLogStatsFromItems(items []logItem) logStats {
	entries := make([]oplog.Entry, len(items))
	for i, item := range items {
		entries[i] = item.entry
	}
	return computeLogStats(entries)
}

// renderStatsPanel renders the full stats overlay panel.
func (m logTUIModel) renderStatsPanel() string {
	var b strings.Builder

	if m.stats.Total == 0 {
		b.WriteString(theme.Dim().Render("  No entries"))
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
		theme.Dim().Render("Total:"),
		lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("%d", m.stats.Total)),
	))
	b.WriteString(fmt.Sprintf("  %s  %s %s\n\n",
		theme.Dim().Render("OK:"),
		rateColor.Render(fmt.Sprintf("%d/%d", okTotal, m.stats.Total)),
		theme.Dim().Render(fmt.Sprintf("(%.1f%%)", m.stats.SuccessRate*100)),
	))

	// ── Command breakdown with horizontal bars ──
	nameWidth := 12
	for name := range m.stats.ByCommand {
		nameWidth = max(nameWidth, lipgloss.Width(name))
	}
	header := fmt.Sprintf("  %-*s  %-20s  %s", nameWidth, "Command", "", "OK")
	b.WriteString(theme.Dim().Render(header))
	b.WriteString("\n")
	b.WriteString(theme.Dim().Render("  " + strings.Repeat("─", nameWidth+30)))
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

		cmdBar := theme.Success().Render(strings.Repeat("▓", okBarLen))
		if errBarLen > 0 {
			cmdBar += theme.Danger().Render(strings.Repeat("▓", errBarLen))
		}
		padding := strings.Repeat(" ", cmdBarWidth-barLen)

		// "✓6/9" format — ok out of total, self-explanatory
		okRatio := fmt.Sprintf("✓%d/%d", cmd.cs.OK, cmd.cs.Total)
		ratioColor := theme.Success()
		if cmd.cs.OK < cmd.cs.Total {
			ratioColor = theme.Danger()
		}

		b.WriteString(fmt.Sprintf("  %s  %s%s  %s\n",
			theme.Dim().Render(fmt.Sprintf("%-*s", nameWidth, cmd.name)),
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

	b.WriteString(theme.Dim().Render("  " + strings.Repeat("─", 50)))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s %s",
		theme.Dim().Render("Status:"),
		theme.Success().Render(fmt.Sprintf("✓ %d ok", okTotal))))
	if errTotal > 0 {
		b.WriteString(fmt.Sprintf("  %s", theme.Danger().Render(fmt.Sprintf("✗ %d error", errTotal))))
	}
	if partialTotal > 0 {
		b.WriteString(fmt.Sprintf("  %s", theme.Warning().Render(fmt.Sprintf("◐ %d partial", partialTotal))))
	}
	if blockedTotal > 0 {
		b.WriteString(fmt.Sprintf("  %s", theme.Danger().Render(fmt.Sprintf("⊘ %d blocked", blockedTotal))))
	}
	b.WriteString("\n")

	// ── Last operation ──
	if m.stats.LastOperation != nil {
		ts, err := time.Parse(time.RFC3339, m.stats.LastOperation.Timestamp)
		if err == nil {
			ago := timeAgo(ts)
			b.WriteString(fmt.Sprintf("  %s %s %s\n",
				theme.Dim().Render("Last op:"),
				theme.Accent().Render(m.stats.LastOperation.Command),
				theme.Dim().Render("("+ago+")")))
		}
	}

	return b.String()
}

// statsSuccessRateColor returns a lipgloss style based on the success rate.
func statsSuccessRateColor(rate float64) lipgloss.Style {
	switch {
	case rate >= 0.9:
		return theme.Success().Bold(true)
	case rate >= 0.7:
		return theme.Warning().Bold(true)
	default:
		return theme.Danger().Bold(true)
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
	if m, ok := finalModel.(logTUIModel); ok {
		if m.loadErr != nil {
			return m.loadErr
		}
		if m.emptyResult {
			fmt.Printf("No %s log entries\n", strings.ToLower(logLabel))
			return nil
		}
	}
	return nil
}
