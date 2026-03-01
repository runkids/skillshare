package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Diff TUI — interactive diff browser: left panel target list, right panel
// detail showing sync/local differences. Browse-only (no mutating actions).
// ---------------------------------------------------------------------------

// Minimum terminal width for horizontal split; below this use vertical layout.
const diffMinSplitWidth = 80

// --- List item ---

type diffTargetItem struct {
	result targetDiffResult
}

func (i diffTargetItem) Title() string {
	r := i.result
	if r.errMsg != "" {
		return fmt.Sprintf("%s %s", tc.Red.Render("✗"), r.name)
	}
	if r.synced {
		return fmt.Sprintf("%s %s", tc.Green.Render("✓"), r.name)
	}
	return fmt.Sprintf("%s %s", tc.Yellow.Render("!"), r.name)
}

func (i diffTargetItem) Description() string {
	r := i.result
	if r.errMsg != "" {
		return "error"
	}
	if r.synced {
		return "synced"
	}
	total := r.syncCount + r.localCount
	return fmt.Sprintf("%d diff(s)", total)
}

func (i diffTargetItem) FilterValue() string { return i.result.name }

// --- Model ---

type diffTUIModel struct {
	quitting   bool
	termWidth  int
	termHeight int

	// Data — sorted: error → diff → synced
	results  []targetDiffResult
	allItems []targetDiffResult

	// Target list
	targetList list.Model

	// Filter
	filterText  string
	filterInput textinput.Model
	filtering   bool
	matchCount  int

	// Detail scroll (right panel)
	detailScroll int
}

func newDiffTUIModel(results []targetDiffResult) diffTUIModel {
	// Sort: error first, then diffs, then synced
	sorted := make([]targetDiffResult, len(results))
	copy(sorted, results)
	sort.Slice(sorted, func(i, j int) bool {
		ri, rj := sorted[i], sorted[j]
		oi, oj := diffSortOrder(ri), diffSortOrder(rj)
		if oi != oj {
			return oi < oj
		}
		return ri.name < rj.name
	})

	listItems := make([]list.Item, len(sorted))
	for i, r := range sorted {
		listItems[i] = diffTargetItem{result: r}
	}

	delegate := list.NewDefaultDelegate()
	configureDelegate(&delegate, true)

	tl := list.New(listItems, delegate, 0, 0)
	tl.Title = fmt.Sprintf("Diff — %d target(s)", len(sorted))
	tl.Styles.Title = tc.ListTitle
	tl.SetShowStatusBar(false)
	tl.SetFilteringEnabled(false)
	tl.SetShowHelp(false)
	tl.SetShowPagination(false)

	fi := textinput.New()
	fi.Prompt = "/ "
	fi.PromptStyle = tc.Filter
	fi.Cursor.Style = tc.Filter

	return diffTUIModel{
		results:     sorted,
		allItems:    sorted,
		targetList:  tl,
		matchCount:  len(sorted),
		filterInput: fi,
	}
}

// diffSortOrder returns 0 for error, 1 for diffs, 2 for synced.
func diffSortOrder(r targetDiffResult) int {
	if r.errMsg != "" {
		return 0
	}
	if !r.synced {
		return 1
	}
	return 2
}

func (m diffTUIModel) Init() tea.Cmd { return nil }

func (m diffTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		lw := diffListWidth(m.termWidth)
		h := m.diffPanelHeight()
		m.targetList.SetSize(lw, h)
		return m, nil

	case tea.KeyMsg:
		return m.handleDiffKey(msg)
	}

	var cmd tea.Cmd
	m.targetList, cmd = m.targetList.Update(msg)
	return m, cmd
}

func (m diffTUIModel) handleDiffKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Filter mode
	if m.filtering {
		switch key {
		case "esc":
			m.filtering = false
			m.filterText = ""
			m.filterInput.SetValue("")
			m.applyDiffFilter()
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
			m.applyDiffFilter()
		}
		return m, cmd
	}

	// Normal keys
	switch key {
	case "q", "esc", "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "/":
		m.filtering = true
		m.filterInput.Focus()
		return m, textinput.Blink

	// Detail scroll
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

	// Reset detail scroll on list navigation
	prevIdx := m.targetList.Index()

	var cmd tea.Cmd
	m.targetList, cmd = m.targetList.Update(msg)

	if m.targetList.Index() != prevIdx {
		m.detailScroll = 0
	}

	return m, cmd
}

// --- Filter ---

func (m *diffTUIModel) applyDiffFilter() {
	needle := strings.ToLower(m.filterText)
	if needle == "" {
		items := make([]list.Item, len(m.allItems))
		for i, r := range m.allItems {
			items[i] = diffTargetItem{result: r}
		}
		m.matchCount = len(m.allItems)
		m.targetList.SetItems(items)
		m.targetList.ResetSelected()
		return
	}
	var matched []list.Item
	for _, r := range m.allItems {
		if strings.Contains(strings.ToLower(r.name), needle) {
			matched = append(matched, diffTargetItem{result: r})
		}
	}
	m.matchCount = len(matched)
	m.targetList.SetItems(matched)
	m.targetList.ResetSelected()
}

// --- Layout helpers ---

func diffListWidth(_ int) int { return 40 }

func diffDetailWidth(termWidth int) int {
	return max(termWidth-diffListWidth(termWidth)-3, 30)
}

func (m diffTUIModel) diffPanelHeight() int {
	return max(m.termHeight-4, 10)
}

// --- Views ---

func (m diffTUIModel) View() string {
	if m.quitting {
		return ""
	}

	if m.termWidth >= diffMinSplitWidth {
		return m.viewDiffHorizontal()
	}
	return m.viewDiffVertical()
}

func (m diffTUIModel) viewDiffHorizontal() string {
	var b strings.Builder

	panelHeight := m.diffPanelHeight()
	leftWidth := diffListWidth(m.termWidth)
	rightWidth := diffDetailWidth(m.termWidth)

	// Left panel: list
	leftPanel := lipgloss.NewStyle().
		Width(leftWidth).MaxWidth(leftWidth).
		Height(panelHeight).MaxHeight(panelHeight).
		Render(m.targetList.View())

	// Border column
	borderStyle := tc.Border.
		Height(panelHeight).MaxHeight(panelHeight)
	borderCol := strings.Repeat("│\n", panelHeight)
	borderPanel := borderStyle.Render(strings.TrimRight(borderCol, "\n"))

	// Right panel: detail
	detailContent := m.buildDiffDetail()
	detailStr := m.applyDiffDetailScroll(detailContent, panelHeight)

	rightPanel := lipgloss.NewStyle().
		Width(rightWidth).MaxWidth(rightWidth).
		Height(panelHeight).MaxHeight(panelHeight).
		PaddingLeft(1).
		Render(detailStr)

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, borderPanel, rightPanel)
	b.WriteString(body)
	b.WriteString("\n")

	// Filter bar
	b.WriteString(m.renderDiffFilterBar())

	// Help
	b.WriteString(tc.Help.Render("↑↓ navigate  / filter  Ctrl+d/u scroll  q quit"))
	b.WriteString("\n")

	return b.String()
}

func (m diffTUIModel) viewDiffVertical() string {
	var b strings.Builder

	b.WriteString(m.targetList.View())
	b.WriteString("\n")

	b.WriteString(m.renderDiffFilterBar())

	// Detail below list
	detailContent := m.buildDiffDetail()
	detailHeight := max(m.termHeight/3, 6)
	b.WriteString(m.applyDiffDetailScroll(detailContent, detailHeight))
	b.WriteString("\n")

	b.WriteString(tc.Help.Render("↑↓ navigate  / filter  Ctrl+d/u scroll  q quit"))
	b.WriteString("\n")

	return b.String()
}

func (m diffTUIModel) renderDiffFilterBar() string {
	pag := renderPageInfoFromPaginator(m.targetList.Paginator)
	return renderTUIFilterBar(
		m.filterInput.View(), m.filtering, m.filterText,
		m.matchCount, len(m.allItems), 0, "targets", pag,
	)
}

// --- Detail renderer ---

func (m diffTUIModel) buildDiffDetail() string {
	item, ok := m.targetList.SelectedItem().(diffTargetItem)
	if !ok {
		return ""
	}
	r := item.result

	var b strings.Builder

	row := func(label, value string) {
		b.WriteString(tc.Label.Render(label))
		b.WriteString(tc.Value.Render(value))
		b.WriteString("\n")
	}

	row("Target:  ", r.name)
	row("Mode:    ", r.mode)
	if len(r.include) > 0 {
		row("Include: ", strings.Join(r.include, ", "))
	}
	if len(r.exclude) > 0 {
		row("Exclude: ", strings.Join(r.exclude, ", "))
	}

	b.WriteString("\n")

	// Error
	if r.errMsg != "" {
		b.WriteString(tc.Red.Render("  " + r.errMsg))
		b.WriteString("\n")
		return b.String()
	}

	// Fully synced
	if r.synced {
		b.WriteString(tc.Green.Render("  ✓ Fully synced"))
		b.WriteString("\n")
		return b.String()
	}

	// Categorized diff items
	items := make([]copyDiffEntry, len(r.items))
	copy(items, r.items)
	sort.Slice(items, func(i, j int) bool {
		return items[i].name < items[j].name
	})

	cats := categorizeItems(items)
	for _, cat := range cats {
		n := len(cat.names)
		skillWord := "skills"
		if n == 1 {
			skillWord = "skill"
		}

		var kindStyle lipgloss.Style
		switch cat.kind {
		case "sync":
			kindStyle = tc.Cyan
		case "force":
			kindStyle = tc.Yellow
		case "collect":
			kindStyle = tc.Green
		case "warn":
			kindStyle = tc.Red
		default:
			kindStyle = tc.Dim
		}

		header := fmt.Sprintf("  %s %d %s:", cat.label, n, skillWord)
		b.WriteString(kindStyle.Render(header))
		b.WriteString("\n")

		if cat.expand {
			for _, name := range cat.names {
				b.WriteString(tc.Dim.Render("    " + name))
				b.WriteString("\n")
			}
		}
	}

	// Hints
	var hints []string
	for _, cat := range cats {
		switch cat.kind {
		case "sync":
			hints = append(hints, "sync")
		case "force":
			hints = append(hints, "sync --force")
		case "collect":
			hints = append(hints, "collect")
		}
	}
	if len(hints) > 0 {
		b.WriteString("\n")
		b.WriteString(tc.Separator.Render("── Actions ───────────────────────────"))
		b.WriteString("\n")
		seen := map[string]bool{}
		for _, h := range hints {
			if seen[h] {
				continue
			}
			seen[h] = true
			b.WriteString(tc.Dim.Render(fmt.Sprintf("  skillshare %s", h)))
			b.WriteString("\n")
		}
	}

	return b.String()
}

// applyDiffDetailScroll applies vertical scrolling to detail content.
func (m diffTUIModel) applyDiffDetailScroll(content string, viewHeight int) string {
	lines := strings.Split(content, "\n")

	if len(lines) <= viewHeight {
		return content
	}

	maxScroll := len(lines) - viewHeight
	offset := min(m.detailScroll, maxScroll)

	end := min(offset+viewHeight, len(lines))
	visible := lines[offset:end]

	var b strings.Builder
	for _, line := range visible {
		b.WriteString(line)
		b.WriteString("\n")
	}

	if offset > 0 || offset < maxScroll {
		indicator := fmt.Sprintf("── Ctrl+d/u scroll (%d/%d) ──", offset+1, maxScroll+1)
		b.WriteString(tc.Dim.Render(indicator))
		b.WriteString("\n")
	}

	return b.String()
}

// --- Entry point ---

func runDiffTUI(results []targetDiffResult) error {
	model := newDiffTUIModel(results)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
