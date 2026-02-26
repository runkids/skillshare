package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"skillshare/internal/audit"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ac holds audit-specific styles that don't belong in the shared tc palette.
var ac = struct {
	File     lipgloss.Style // file:line locations — cyan
	Snippet  lipgloss.Style // code snippet highlight
	ItemName lipgloss.Style // list item name — light gray
}{
	File:     lipgloss.NewStyle().Foreground(lipgloss.Color("6")),
	Snippet:  lipgloss.NewStyle().Foreground(lipgloss.Color("179")),
	ItemName: lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
}

// acSevCount returns severity color for non-zero counts, dim for zero.
func acSevCount(count int, style lipgloss.Style) lipgloss.Style {
	if count == 0 {
		return tc.Dim
	}
	return style
}

// auditItem implements list.Item for audit TUI.
type auditItem struct {
	result  *audit.Result
	elapsed time.Duration
}

func (i auditItem) Title() string {
	name := colorSkillPath(i.result.SkillName)
	if len(i.result.Findings) == 0 {
		return tc.Green.Render("✓") + " " + name
	}
	if i.result.IsBlocked {
		return tc.Red.Render("✗") + " " + name
	}
	return tc.Yellow.Render("!") + " " + name
}

func (i auditItem) Description() string { return "" }
func (i auditItem) FilterValue() string { return i.result.SkillName }

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
}

func newAuditTUIModel(results []*audit.Result, scanOutputs []audit.ScanOutput, summary auditRunSummary) auditTUIModel {
	// Build items sorted: by severity (findings first), then by name.
	items := make([]auditItem, 0, len(results))
	for idx, r := range results {
		var elapsed time.Duration
		if idx < len(scanOutputs) {
			elapsed = scanOutputs[idx].Elapsed
		}
		items = append(items, auditItem{result: r, elapsed: elapsed})
	}
	sort.Slice(items, func(i, j int) bool {
		ri, rj := items[i].result, items[j].result
		// Skills with findings come first.
		hasI, hasJ := len(ri.Findings) > 0, len(rj.Findings) > 0
		if hasI != hasJ {
			return hasI
		}
		if hasI && hasJ {
			// Higher severity (lower rank) first.
			rankI := audit.SeverityRank(ri.MaxSeverity())
			rankJ := audit.SeverityRank(rj.MaxSeverity())
			if rankI != rankJ {
				return rankI < rankJ
			}
			// Higher risk score first.
			if ri.RiskScore != rj.RiskScore {
				return ri.RiskScore > rj.RiskScore
			}
		}
		return ri.SkillName < rj.SkillName
	})

	// Cap items for list widget performance.
	allItems := items
	displayItems := items
	if len(displayItems) > maxListItems {
		displayItems = displayItems[:maxListItems]
	}

	listItems := make([]list.Item, len(displayItems))
	for i, item := range displayItems {
		listItems[i] = item
	}

	delegate := list.NewDefaultDelegate()
	configureDelegate(&delegate, false)

	l := list.New(listItems, delegate, 0, 0)
	l.Title = fmt.Sprintf("Audit results (%d scanned)", summary.Scanned)
	l.Styles.Title = tc.ListTitle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)

	fi := textinput.New()
	fi.Prompt = "/ "
	fi.PromptStyle = tc.Filter
	fi.Cursor.Style = tc.Filter

	return auditTUIModel{
		list:        l,
		allItems:    allItems,
		matchCount:  len(allItems),
		filterInput: fi,
		summary:     summary,
	}
}

func (m auditTUIModel) Init() tea.Cmd { return nil }

func (m *auditTUIModel) applyFilter() {
	term := strings.ToLower(m.filterText)

	if term == "" {
		all := make([]list.Item, min(len(m.allItems), maxListItems))
		for i := range all {
			all[i] = m.allItems[i]
		}
		m.matchCount = len(m.allItems)
		m.list.SetItems(all)
		m.list.ResetSelected()
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
		// Overhead: filter(1) + summary footer(1) + help(1) + newlines(3) = 6
		panelHeight := msg.Height - 6
		if panelHeight < 6 {
			panelHeight = 6
		}
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

	// Panel height (matches WindowSizeMsg overhead)
	panelHeight := m.termHeight - 6
	if panelHeight < 6 {
		panelHeight = 6
	}

	leftWidth := auditListWidth(m.termWidth)
	rightWidth := auditDetailPanelWidth(m.termWidth)

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
	if item, ok := m.list.SelectedItem().(auditItem); ok {
		detailContent := m.renderDetailContent(item)
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

	// Filter bar (below panels, matching list TUI layout)
	b.WriteString(m.renderFilterBar())

	// Summary footer
	b.WriteString(m.renderSummaryFooter())

	// Help line
	b.WriteString(tc.Help.Render("↑↓ navigate  ←→ page  / filter  Ctrl+d/u scroll detail  q quit"))
	b.WriteString("\n")

	return b.String()
}

// viewVertical renders the original vertical layout for narrow terminals.
func (m auditTUIModel) viewVertical() string {
	var b strings.Builder

	b.WriteString(m.list.View())
	b.WriteString("\n\n")

	b.WriteString(m.renderFilterBar())

	if item, ok := m.list.SelectedItem().(auditItem); ok {
		detailContent := m.renderDetailContent(item)
		detailHeight := m.termHeight - m.termHeight*2/5 - 7
		b.WriteString(m.applyDetailScroll(detailContent, detailHeight))
	}

	b.WriteString(m.renderSummaryFooter())

	b.WriteString(tc.Help.Render("↑↓ navigate  ←→ page  / filter  Ctrl+d/u scroll  q quit"))
	b.WriteString("\n")

	return b.String()
}

func (m auditTUIModel) renderFilterBar() string {
	return renderTUIFilterBar(
		m.filterInput.View(), m.filtering, m.filterText,
		m.matchCount, len(m.allItems), maxListItems,
		"results", m.renderPageInfo(),
	)
}

func (m auditTUIModel) renderPageInfo() string {
	return renderPageInfoFromPaginator(m.list.Paginator)
}

// renderSummaryFooter renders the compact summary line above the help bar.
func (m auditTUIModel) renderSummaryFooter() string {
	s := m.summary

	parts := []string{
		tc.Dim.Render(fmt.Sprintf("Scanned: %s", formatNumber(s.Scanned))),
		tc.Green.Render(fmt.Sprintf("Passed: %s", formatNumber(s.Passed))),
	}
	if s.Warning > 0 {
		parts = append(parts, tc.Yellow.Render(fmt.Sprintf("Warning: %s", formatNumber(s.Warning))))
	} else {
		parts = append(parts, tc.Dim.Render(fmt.Sprintf("Warning: %s", formatNumber(s.Warning))))
	}
	if s.Failed > 0 {
		parts = append(parts, tc.Red.Render(fmt.Sprintf("Failed: %s", formatNumber(s.Failed))))
	} else {
		parts = append(parts, tc.Dim.Render(fmt.Sprintf("Failed: %s", formatNumber(s.Failed))))
	}

	sevParts := []string{
		acSevCount(s.Critical, tc.Critical).Render(fmt.Sprintf("%d", s.Critical)),
		acSevCount(s.High, tc.High).Render(fmt.Sprintf("%d", s.High)),
		acSevCount(s.Medium, tc.Medium).Render(fmt.Sprintf("%d", s.Medium)),
		acSevCount(s.Low, tc.Low).Render(fmt.Sprintf("%d", s.Low)),
		acSevCount(s.Info, tc.Info).Render(fmt.Sprintf("%d", s.Info)),
	}
	sep := tc.Dim.Render("/")
	parts = append(parts, tc.Dim.Render("c/h/m/l/i = ")+strings.Join(sevParts, sep))

	return "  " + strings.Join(parts, tc.Dim.Render(" | ")) + "\n"
}

// renderDetailContent renders the full detail panel for the selected audit item.
// Mirrors the summary box style with colorized severity breakdown and structured findings.
func (m auditTUIModel) renderDetailContent(item auditItem) string {
	var b strings.Builder

	r := item.result

	row := func(label, value string) {
		b.WriteString(tc.Label.Render(label))
		b.WriteString(value)
		b.WriteString("\n")
	}

	// ── Header ──
	b.WriteString(tc.Title.Render(r.SkillName))
	b.WriteString("\n")
	b.WriteString(tc.Dim.Render(strings.Repeat("─", 40)))
	b.WriteString("\n\n")

	// ── Summary section ──

	// Risk — colorized by severity
	riskText := fmt.Sprintf("%s (%d/100)", strings.ToUpper(r.RiskLabel), r.RiskScore)
	riskStyle := tcSevStyle(r.RiskLabel)
	if r.RiskLabel == "clean" {
		riskStyle = tc.Green
	}
	row("Risk:", riskStyle.Render(riskText))

	// Max severity — use severity color; NONE = green
	maxSev := r.MaxSeverity()
	if maxSev == "" {
		maxSev = "NONE"
	}
	maxSevStyle := tcSevStyle(maxSev)
	if strings.ToUpper(maxSev) == "NONE" {
		maxSevStyle = tc.Green
	}
	row("Max sev:", maxSevStyle.Render(maxSev))

	// Block status
	if r.IsBlocked {
		row("Status:", tc.Red.Render("✗ BLOCKED"))
	} else if len(r.Findings) == 0 {
		row("Status:", tc.Green.Render("✓ Clean"))
	} else {
		row("Status:", tc.Yellow.Render("! Has findings (not blocked)"))
	}

	// Threshold
	if r.Threshold != "" {
		row("Threshold:", tc.Dim.Render("severity >= ")+tc.Emphasis.Render(strings.ToUpper(r.Threshold)))
	}

	// Scan time
	if item.elapsed > 0 {
		row("Scan time:", tc.Dim.Render(fmt.Sprintf("%.1fs", item.elapsed.Seconds())))
	}

	// Severity breakdown — only non-zero counts are colorized; zeros are dim
	if len(r.Findings) > 0 {
		counts := map[string]int{}
		for _, f := range r.Findings {
			counts[f.Severity]++
		}
		sep := tc.Dim.Render("/")
		sevLine := acSevCount(counts["CRITICAL"], tc.Critical).Render(fmt.Sprintf("%d", counts["CRITICAL"])) + sep +
			acSevCount(counts["HIGH"], tc.High).Render(fmt.Sprintf("%d", counts["HIGH"])) + sep +
			acSevCount(counts["MEDIUM"], tc.Medium).Render(fmt.Sprintf("%d", counts["MEDIUM"])) + sep +
			acSevCount(counts["LOW"], tc.Low).Render(fmt.Sprintf("%d", counts["LOW"])) + sep +
			acSevCount(counts["INFO"], tc.Info).Render(fmt.Sprintf("%d", counts["INFO"]))
		row("Severity:", tc.Dim.Render("c/h/m/l/i = ")+sevLine)
		row("Total:", tc.Emphasis.Render(fmt.Sprintf("%d", len(r.Findings)))+tc.Dim.Render(" finding(s)"))
	}

	b.WriteString("\n")

	// ── Findings detail ──
	if len(r.Findings) > 0 {
		b.WriteString(tc.Title.Render("Findings"))
		b.WriteString("\n")
		b.WriteString(tc.Dim.Render(strings.Repeat("─", 40)))
		b.WriteString("\n\n")

		sorted := make([]audit.Finding, len(r.Findings))
		copy(sorted, r.Findings)
		sort.Slice(sorted, func(i, j int) bool {
			return audit.SeverityRank(sorted[i].Severity) < audit.SeverityRank(sorted[j].Severity)
		})

		for idx, f := range sorted {
			// [N] SEVERITY  pattern
			sevBadge := tcSevStyle(f.Severity).Render(strings.ToUpper(f.Severity))
			header := tc.Dim.Render(fmt.Sprintf("[%d] ", idx+1))
			patternText := tc.Emphasis.Bold(true).Render(f.Pattern)
			b.WriteString(header + sevBadge + "  " + patternText + "\n")

			// Message
			b.WriteString(tc.Dim.Render("    "))
			b.WriteString(tc.Dim.Render(f.Message))
			b.WriteString("\n")

			// Location: file:line
			loc := fmt.Sprintf("%s:%d", f.File, f.Line)
			b.WriteString(tc.Dim.Render("    "))
			b.WriteString(ac.File.Render(loc))
			b.WriteString("\n")

			// Snippet — with │ gutter
			if f.Snippet != "" {
				gutter := tc.Dim.Render("    │ ")
				b.WriteString(gutter)
				b.WriteString(ac.Snippet.Render(f.Snippet))
				b.WriteString("\n")
			}

			b.WriteString("\n")
		}
	}

	return b.String()
}

// applyDetailScroll wraps a detail panel string with height limit and scroll offset.
func (m auditTUIModel) applyDetailScroll(content string, viewHeight int) string {
	lines := strings.Split(content, "\n")

	maxDetailLines := viewHeight
	if maxDetailLines < 5 {
		maxDetailLines = 5
	}

	totalLines := len(lines)
	if totalLines <= maxDetailLines {
		return content
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
		indicator := fmt.Sprintf("── Ctrl+d/u scroll (%d/%d) ──", offset+1, maxScroll+1)
		b.WriteString(tc.Dim.Render(indicator))
		b.WriteString("\n")
	}

	return b.String()
}

// auditListWidth returns the left panel width for horizontal layout.
// 25% of terminal, clamped to [25, 45].
func auditListWidth(termWidth int) int {
	w := termWidth / 4
	if w < 25 {
		w = 25
	}
	if w > 45 {
		w = 45
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

// runAuditTUI starts the bubbletea TUI for audit results.
func runAuditTUI(results []*audit.Result, scanOutputs []audit.ScanOutput, summary auditRunSummary) error {
	model := newAuditTUIModel(results, scanOutputs, summary)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}
