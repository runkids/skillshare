package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"skillshare/internal/oplog"
	"skillshare/internal/ui"
)

// logDetailLabelStyle uses a wider width than the shared tuiDetailLabelStyle
// because log keys like "severity(c/h/m/l/i):" are longer than skill detail keys.
var logDetailLabelStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("8")).
	Width(22)

// logTUIModel is the bubbletea model for the interactive log viewer.
type logTUIModel struct {
	list      list.Model
	modeLabel string // "global" or "project"
	quitting  bool

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
	termHeight   int
}

// newLogTUIModel creates a new TUI model from log items.
func newLogTUIModel(items []logItem, logLabel, modeLabel string) logTUIModel {
	listItems := make([]list.Item, len(items))
	for i, item := range items {
		listItems[i] = item
	}

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	// Title: no foreground — let embedded lipgloss colors (command, status) show through
	delegate.Styles.NormalTitle = lipgloss.NewStyle().PaddingLeft(2)
	delegate.Styles.NormalDesc = lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).PaddingLeft(2)
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().
		Bold(true).Foreground(tuiBrandYellow).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(tuiBrandYellow).PaddingLeft(1)
	delegate.Styles.SelectedDesc = lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(tuiBrandYellow).PaddingLeft(1)

	l := list.New(listItems, delegate, 0, 0)
	l.Title = fmt.Sprintf("Log: %s (%s)", logLabel, modeLabel)
	l.Styles.Title = lipgloss.NewStyle().
		Bold(true).Foreground(lipgloss.Color("6"))
	l.SetShowStatusBar(false)    // custom status line
	l.SetFilteringEnabled(false) // application-level filter
	l.SetShowHelp(false)
	l.SetShowPagination(false) // page info in custom status line

	// Filter text input
	fi := textinput.New()
	fi.Prompt = "/ "
	fi.PromptStyle = tuiFilterStyle
	fi.Cursor.Style = tuiFilterStyle

	return logTUIModel{
		list:        l,
		modeLabel:   modeLabel,
		allItems:    items,
		matchCount:  len(items),
		filterInput: fi,
		stats:       computeLogStatsFromItems(items),
	}
}

func (m logTUIModel) Init() tea.Cmd {
	return nil
}

func (m logTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termHeight = msg.Height
		// Reserve bottom half for detail panel + filter + footer + help (~6 lines overhead)
		// Give list roughly 40% of terminal, min 6 lines
		listHeight := msg.Height * 2 / 5
		if listHeight < 6 {
			listHeight = 6
		}
		m.list.SetSize(msg.Width, listHeight)
		return m, nil

	case tea.KeyMsg:
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
		case "j":
			m.detailScroll++
			return m, nil
		case "k":
			if m.detailScroll > 0 {
				m.detailScroll--
			}
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

func (m logTUIModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	if m.showStats {
		b.WriteString("\n")
		b.WriteString(m.renderStatsPanel())
		b.WriteString("\n")

		help := "s back to list  q quit"
		b.WriteString(tuiHelpStyle.Render(help))
		b.WriteString("\n")
		return b.String()
	}

	b.WriteString(m.list.View())
	b.WriteString("\n\n")

	// Filter bar (always visible)
	b.WriteString(m.renderLogFilterBar())

	// Detail panel for selected item (scrollable, height-limited)
	if item, ok := m.list.SelectedItem().(logItem); ok {
		detailContent := renderLogDetailPanel(item)
		b.WriteString(m.scrollableDetail(detailContent))
	}

	// Stats footer
	b.WriteString(m.renderStatsFooter())

	help := "↑↓ navigate  ←→ page  / filter  s stats  q quit"
	b.WriteString(tuiHelpStyle.Render(help))
	b.WriteString("\n")

	return b.String()
}

// renderLogFilterBar renders the status line for the log TUI.
func (m logTUIModel) renderLogFilterBar() string {
	return renderTUIFilterBar(
		m.filterInput.View(), m.filtering, m.filterText,
		m.matchCount, len(m.allItems), 0,
		"entries", renderPageInfoFromPaginator(m.list.Paginator),
	)
}

// scrollableDetail wraps a detail panel string with height limit and scroll offset.
func (m logTUIModel) scrollableDetail(content string) string {
	lines := strings.Split(content, "\n")

	// List takes ~40% of terminal; remaining goes to detail.
	// Subtract overhead: filter bar(2) + stats footer(1) + help bar(1) + newlines(3) = 7
	listHeight := m.termHeight * 2 / 5
	if listHeight < 6 {
		listHeight = 6
	}
	maxDetailLines := m.termHeight - listHeight - 7
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
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	if offset > 0 || offset < maxScroll {
		indicator := fmt.Sprintf("  ── j/k scroll (%d/%d) ──", offset+1, maxScroll+1)
		b.WriteString(dimStyle.Render(indicator))
		b.WriteString("\n")
	}

	return b.String()
}

// renderLogDetailPanel renders structured details for the selected log entry.
func renderLogDetailPanel(item logItem) string {
	var b strings.Builder
	b.WriteString(tuiSeparatorStyle.Render("  ─────────────────────────────────────────"))
	b.WriteString("\n")

	row := func(label, value string) {
		b.WriteString("  ")
		b.WriteString(logDetailLabelStyle.Render(label))
		b.WriteString(tuiDetailValueStyle.Render(value))
		b.WriteString("\n")
	}

	e := item.entry

	cyanStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	greenStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	redStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	yellowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))

	// Full timestamp
	row("Timestamp:", e.Timestamp)

	// Command — cyan to match CLI palette
	row("Command:", cyanStyle.Render(strings.ToUpper(e.Command)))

	// Status with color
	statusDisplay := e.Status
	switch e.Status {
	case "ok":
		statusDisplay = greenStyle.Render(e.Status)
	case "error", "blocked":
		statusDisplay = redStyle.Render(e.Status)
	case "partial":
		statusDisplay = yellowStyle.Render(e.Status)
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
	const maxBulletItems = 5 // condense long lists to avoid flooding the panel

	for _, p := range pairs {
		// List fields: render as multi-line bullet list for readability
		if p.isList && len(p.listValues) > 0 {
			b.WriteString("  ")
			b.WriteString(logDetailLabelStyle.Render(p.key + ":"))
			b.WriteString("\n")
			show := p.listValues
			remaining := 0
			if len(show) > maxBulletItems {
				remaining = len(show) - maxBulletItems
				show = show[:maxBulletItems]
			}
			for _, v := range show {
				b.WriteString("      - " + tuiDetailValueStyle.Render(v) + "\n")
			}
			if remaining > 0 {
				summary := fmt.Sprintf("      ... and %d more", remaining)
				b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(summary) + "\n")
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
			value = redStyle.Render(value)
		case strings.Contains(p.key, "warning"):
			value = yellowStyle.Render(value)
		case p.key == "risk":
			value = colorizeRiskValue(value, redStyle, yellowStyle, greenStyle)
		case p.key == "threshold":
			value = colorizeThreshold(value, redStyle, yellowStyle, greenStyle)
		case strings.HasPrefix(p.key, "severity"):
			value = colorizeSeverityBreakdown(value)
		}

		row(p.key+":", value)
	}

	return b.String()
}

// severityStyles maps the 5 severity levels (c/h/m/l/i) to lipgloss styles
// using shared color IDs from ui.SeverityColorID.
var severityStyles = []lipgloss.Style{
	lipgloss.NewStyle().Foreground(lipgloss.Color(ui.SeverityIDCritical)),
	lipgloss.NewStyle().Foreground(lipgloss.Color(ui.SeverityIDHigh)),
	lipgloss.NewStyle().Foreground(lipgloss.Color(ui.SeverityIDMedium)),
	lipgloss.NewStyle().Foreground(lipgloss.Color(ui.SeverityIDLow)),
	lipgloss.NewStyle().Foreground(lipgloss.Color(ui.SeverityIDInfo)),
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
	sep := lipgloss.NewStyle().Foreground(lipgloss.Color(ui.SeverityIDLow)).Render("/")
	return strings.Join(parts, sep)
}

// severityLipglossStyle returns a lipgloss style for the given severity string.
func severityLipglossStyle(severity string) lipgloss.Style {
	id := ui.SeverityColorID(severity)
	if id == "" {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(id))
}

// colorizeThreshold applies color based on audit threshold level.
func colorizeThreshold(value string, _, _, green lipgloss.Style) string {
	style := severityLipglossStyle(value)
	if ui.SeverityColorID(value) == "" {
		return green.Render(value)
	}
	return style.Render(value)
}

// colorizeRiskValue applies color based on the risk label embedded in the value string.
// e.g. "CRITICAL (85/100)" → red, "LOW (15/100)" → green
func colorizeRiskValue(value string, _, _, green lipgloss.Style) string {
	// Extract the severity word (first token before space or paren)
	sev := strings.SplitN(strings.ToUpper(value), " ", 2)[0]
	sev = strings.TrimRight(sev, "(")
	style := severityLipglossStyle(sev)
	if ui.SeverityColorID(sev) == "" {
		return green.Render(value)
	}
	return style.Render(value)
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

	parts := []string{
		fmt.Sprintf("%d ops", m.stats.Total),
		fmt.Sprintf("✓ %.1f%%", m.stats.SuccessRate*100),
	}

	if m.stats.LastOperation != nil {
		ts, err := time.Parse(time.RFC3339, m.stats.LastOperation.Timestamp)
		if err == nil {
			parts = append(parts, fmt.Sprintf("last: %s %s ago",
				m.stats.LastOperation.Command, formatRelativeTime(time.Since(ts))))
		}
	}

	line := strings.Join(parts, " | ")
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	return "  " + style.Render(line) + "\n"
}

// renderStatsPanel renders the full stats overlay panel.
func (m logTUIModel) renderStatsPanel() string {
	var b strings.Builder

	cyanStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	greenStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	redStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	yellowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	boldStyle := lipgloss.NewStyle().Bold(true)

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	b.WriteString(titleStyle.Render("  Operation Log Summary"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  " + strings.Repeat("─", 50)))
	b.WriteString("\n\n")

	if m.stats.Total == 0 {
		b.WriteString(dimStyle.Render("  No entries"))
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
		dimStyle.Render("Total:"),
		boldStyle.Render(fmt.Sprintf("%d", m.stats.Total)),
	))
	b.WriteString(fmt.Sprintf("  %s  %s %s\n\n",
		dimStyle.Render("OK:"),
		rateColor.Render(fmt.Sprintf("%d/%d", okTotal, m.stats.Total)),
		dimStyle.Render(fmt.Sprintf("(%.1f%%)", m.stats.SuccessRate*100)),
	))


	// ── Command breakdown with horizontal bars ──
	header := fmt.Sprintf("  %-12s  %-20s  %s", "Command", "", "OK")
	b.WriteString(dimStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  " + strings.Repeat("─", 42)))
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

		cmdBar := greenStyle.Render(strings.Repeat("▓", okBarLen))
		if errBarLen > 0 {
			cmdBar += redStyle.Render(strings.Repeat("▓", errBarLen))
		}
		padding := strings.Repeat(" ", cmdBarWidth-barLen)

		// "✓6/9" format — ok out of total, self-explanatory
		okRatio := fmt.Sprintf("✓%d/%d", cmd.cs.OK, cmd.cs.Total)
		ratioColor := greenStyle
		if cmd.cs.OK < cmd.cs.Total {
			ratioColor = redStyle
		}
		if cmd.cs.OK == cmd.cs.Total {
			ratioColor = greenStyle
		}

		b.WriteString(fmt.Sprintf("  %s  %s%s  %s\n",
			dimStyle.Render(fmt.Sprintf("%-12s", cmd.name)),
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

	b.WriteString(dimStyle.Render("  " + strings.Repeat("─", 50)))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s %s",
		dimStyle.Render("Status:"),
		greenStyle.Render(fmt.Sprintf("✓ %d ok", okTotal))))
	if errTotal > 0 {
		b.WriteString(fmt.Sprintf("  %s", redStyle.Render(fmt.Sprintf("✗ %d error", errTotal))))
	}
	if partialTotal > 0 {
		b.WriteString(fmt.Sprintf("  %s", yellowStyle.Render(fmt.Sprintf("◐ %d partial", partialTotal))))
	}
	if blockedTotal > 0 {
		b.WriteString(fmt.Sprintf("  %s", redStyle.Render(fmt.Sprintf("⊘ %d blocked", blockedTotal))))
	}
	b.WriteString("\n")

	// ── Last operation ──
	if m.stats.LastOperation != nil {
		ts, err := time.Parse(time.RFC3339, m.stats.LastOperation.Timestamp)
		if err == nil {
			ago := formatRelativeTime(time.Since(ts))
			b.WriteString(fmt.Sprintf("  %s %s %s\n",
				dimStyle.Render("Last op:"),
				cyanStyle.Render(m.stats.LastOperation.Command),
				dimStyle.Render(fmt.Sprintf("(%s ago)", ago))))
		}
	}

	return b.String()
}

// statsSuccessRateColor returns a lipgloss style based on the success rate.
func statsSuccessRateColor(rate float64) lipgloss.Style {
	switch {
	case rate >= 0.9:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true) // green
	case rate >= 0.7:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true) // yellow
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true) // red
	}
}

// runLogTUI starts the bubbletea TUI for the log viewer.
func runLogTUI(items []logItem, logLabel, modeLabel string) error {
	if len(items) == 0 {
		fmt.Printf("No %s log entries\n", strings.ToLower(logLabel))
		return nil
	}

	model := newLogTUIModel(items, logLabel, modeLabel)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
