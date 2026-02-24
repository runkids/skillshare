package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)
	l.SetStatusBarItemName("entry", "entries")
	applyTUIFilterStyle(&l)

	return logTUIModel{
		list:      l,
		modeLabel: modeLabel,
	}
}

func (m logTUIModel) Init() tea.Cmd {
	return nil
}

func (m logTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height-20)
		return m, nil

	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m logTUIModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	b.WriteString(m.list.View())
	b.WriteString("\n")

	// Detail panel for selected item
	if item, ok := m.list.SelectedItem().(logItem); ok {
		b.WriteString(renderLogDetailPanel(item))
	}

	help := "↑↓ navigate  / filter  q quit"
	b.WriteString(tuiHelpStyle.Render(help))
	b.WriteString("\n")

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
	for _, p := range pairs {
		value := p.value
		if p.isList && len(p.listValues) > 0 {
			value = strings.Join(p.listValues, ", ")
		}
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
