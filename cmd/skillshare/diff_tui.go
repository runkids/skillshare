package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// diffExpandMsg is sent when async diff computation completes.
type diffExpandMsg struct {
	skill string
	diff  string
	files []fileDiffEntry
}

// ---------------------------------------------------------------------------
// Diff TUI — interactive diff browser: left panel target list, right panel
// detail showing sync/local differences. Browse-only (no mutating actions).
// ---------------------------------------------------------------------------

// diffMinSplitWidth is the minimum terminal width for horizontal split.
const diffMinSplitWidth = tuiMinSplitWidth

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
	var parts []string
	if r.syncCount > 0 {
		parts = append(parts, fmt.Sprintf("%d sync", r.syncCount))
	}
	if r.localCount > 0 {
		parts = append(parts, fmt.Sprintf("%d local", r.localCount))
	}
	if len(parts) == 0 {
		return "0 diff(s)"
	}
	return strings.Join(parts, ", ")
}

func (i diffTargetItem) FilterValue() string { return i.result.name }

// --- Model ---

type diffTUIModel struct {
	quitting   bool
	termWidth  int
	termHeight int

	// Data — sorted: error → diff → synced
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

	// Expand state — file-level diff for a specific skill
	expandedSkill string // skill name currently expanded
	expandedDiff  string // cached unified diff text
	expandedFiles []fileDiffEntry

	// Async loading — spinner shown while computing file diffs
	loading     bool
	loadSpinner spinner.Model

	// Cached detail data — recomputed only on selection change
	cachedIdx   int
	cachedItems []copyDiffEntry
	cachedCats  []actionCategory
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
	var errN, diffN, syncN int
	for _, r := range sorted {
		switch {
		case r.errMsg != "":
			errN++
		case !r.synced:
			diffN++
		default:
			syncN++
		}
	}
	var titleParts []string
	if errN > 0 {
		titleParts = append(titleParts, fmt.Sprintf("%d err", errN))
	}
	if diffN > 0 {
		titleParts = append(titleParts, fmt.Sprintf("%d diff", diffN))
	}
	if syncN > 0 {
		titleParts = append(titleParts, fmt.Sprintf("%d ok", syncN))
	}
	tl.Title = fmt.Sprintf("Diff — %s", strings.Join(titleParts, ", "))
	tl.Styles.Title = tc.ListTitle
	tl.SetShowStatusBar(false)
	tl.SetFilteringEnabled(false)
	tl.SetShowHelp(false)
	tl.SetShowPagination(false)

	fi := textinput.New()
	fi.Prompt = "/ "
	fi.PromptStyle = tc.Filter
	fi.Cursor.Style = tc.Filter

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = tc.SpinnerStyle

	return diffTUIModel{
		allItems:    sorted,
		targetList:  tl,
		matchCount:  len(sorted),
		filterInput: fi,
		loadSpinner: sp,
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

// refreshDetailCache recomputes sorted items and categories for the selected target.
func (m *diffTUIModel) refreshDetailCache() {
	idx := m.targetList.Index()
	if idx == m.cachedIdx && m.cachedItems != nil {
		return
	}
	m.cachedIdx = idx
	item, ok := m.targetList.SelectedItem().(diffTargetItem)
	if !ok || item.result.synced || item.result.errMsg != "" {
		m.cachedItems = nil
		m.cachedCats = nil
		return
	}
	items := make([]copyDiffEntry, len(item.result.items))
	copy(items, item.result.items)
	sort.Slice(items, func(i, j int) bool {
		return items[i].name < items[j].name
	})
	m.cachedItems = items
	m.cachedCats = categorizeItems(items)
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
		m.refreshDetailCache()
		return m, nil

	case tea.KeyMsg:
		return m.handleDiffKey(msg)

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.loadSpinner, cmd = m.loadSpinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case diffExpandMsg:
		// Discard stale result if user navigated away (loading reset on nav)
		if !m.loading {
			return m, nil
		}
		m.loading = false
		m.expandedSkill = msg.skill
		m.expandedDiff = msg.diff
		m.expandedFiles = msg.files
		return m, nil
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

	case "enter":
		if m.loading {
			return m, nil // ignore while loading
		}
		item, ok := m.targetList.SelectedItem().(diffTargetItem)
		if !ok {
			return m, nil
		}
		r := item.result
		if r.synced || r.errMsg != "" {
			return m, nil
		}
		// Toggle expand
		if m.expandedSkill != "" {
			m.expandedSkill = ""
			m.expandedDiff = ""
			m.expandedFiles = nil
			m.detailScroll = 0
			return m, nil
		}
		// Find first expandable skill (any with srcDir or dstDir)
		for i := range r.items {
			entry := r.items[i]
			if entry.srcDir != "" || entry.dstDir != "" {
				m.loading = true
				m.expandedSkill = ""
				m.expandedDiff = ""
				m.expandedFiles = nil
				m.detailScroll = 0
				return m, tea.Batch(m.loadSpinner.Tick, expandDiffCmd(&entry))
			}
		}
		return m, nil

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
		m.expandedSkill = ""
		m.expandedDiff = ""
		m.expandedFiles = nil
		m.loading = false // cancel in-flight diff if navigating away
		m.refreshDetailCache()
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
		m.cachedItems = nil // invalidate cache
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
	m.cachedItems = nil // invalidate cache
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

	// Detail
	detailContent := m.buildDiffDetail()
	detailStr := applyDetailScroll(detailContent, m.detailScroll, panelHeight)

	body := renderHorizontalSplit(m.targetList.View(), detailStr, leftWidth, rightWidth, panelHeight)
	b.WriteString(body)
	b.WriteString("\n")

	// Filter bar
	b.WriteString(m.renderDiffFilterBar())

	// Help
	b.WriteString(tc.Help.Render("↑↓ navigate  / filter  Enter expand  Ctrl+d/u scroll  q quit"))
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
	b.WriteString(applyDetailScroll(detailContent, m.detailScroll, detailHeight))
	b.WriteString("\n")

	b.WriteString(tc.Help.Render("↑↓ navigate  / filter  Enter expand  Ctrl+d/u scroll  q quit"))
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
	if !r.srcMtime.IsZero() {
		row("Source:   ", r.srcMtime.Format("2006-01-02 15:04"))
	}
	if !r.dstMtime.IsZero() {
		row("Target:  ", r.dstMtime.Format("2006-01-02 15:04"))
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

	// Loading spinner
	if m.loading {
		b.WriteString(fmt.Sprintf("  %s Loading diff...\n", m.loadSpinner.View()))
		return b.String()
	}

	// Use cached sorted categories (refreshed on selection change)
	cats := m.cachedCats
	for _, cat := range cats {
		n := len(cat.names)
		skillWord := "skills"
		if n == 1 {
			skillWord = "skill"
		}

		var kindStyle lipgloss.Style
		switch cat.kind {
		case "new", "restore":
			kindStyle = tc.Green
		case "modified":
			kindStyle = tc.Cyan
		case "override":
			kindStyle = tc.Yellow
		case "orphan":
			kindStyle = tc.Red
		case "local":
			kindStyle = tc.Dim
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

	// File list + diff content (only shown after Enter toggle)
	if m.expandedSkill != "" {
		if len(m.expandedFiles) > 0 {
			b.WriteString("\n")
			b.WriteString(tc.Separator.Render(fmt.Sprintf("── %s files ──", m.expandedSkill)))
			b.WriteString("\n")
			for _, f := range m.expandedFiles {
				var icon string
				var style lipgloss.Style
				switch f.Action {
				case "add":
					icon, style = "+", tc.Green
				case "delete":
					icon, style = "-", tc.Red
				case "modify":
					icon, style = "~", tc.Cyan
				default:
					icon, style = "?", tc.Dim
				}
				b.WriteString(style.Render(fmt.Sprintf("  %s %s", icon, f.RelPath)))
				b.WriteString("\n")
			}
		}

		// Unified diff content
		if m.expandedDiff != "" {
			b.WriteString("\n")
			b.WriteString(tc.Separator.Render(fmt.Sprintf("── %s diff ──", m.expandedSkill)))
			b.WriteString("\n")
			for _, line := range strings.Split(strings.TrimRight(m.expandedDiff, "\n"), "\n") {
				switch {
				case strings.HasPrefix(line, "+ "):
					b.WriteString(tc.Green.Render(line))
				case strings.HasPrefix(line, "- "):
					b.WriteString(tc.Red.Render(line))
				case strings.HasPrefix(line, "--- "):
					b.WriteString(tc.Cyan.Render(line))
				default:
					b.WriteString(tc.Dim.Render(line))
				}
				b.WriteString("\n")
			}
		}
		if len(m.expandedFiles) == 0 && m.expandedDiff == "" {
			b.WriteString("\n")
			b.WriteString(tc.Dim.Render("  (No file-level diff available)"))
			b.WriteString("\n")
		}
	}

	// Next Steps
	var hints []string
	for _, cat := range cats {
		switch cat.kind {
		case "new", "modified", "restore", "orphan":
			hints = append(hints, "sync")
		case "override":
			hints = append(hints, "sync --force")
		case "local":
			hints = append(hints, "collect")
		}
	}
	if len(hints) > 0 {
		b.WriteString("\n")
		b.WriteString(tc.Title.Render("── Next Steps ──"))
		b.WriteString("\n")
		seen := map[string]bool{}
		for _, h := range hints {
			if seen[h] {
				continue
			}
			seen[h] = true
			b.WriteString(tc.Cyan.Render(fmt.Sprintf("  → skillshare %s", h)))
			b.WriteString("\n")
		}
	}

	return b.String()
}

// --- Async diff ---

// expandDiffCmd returns a tea.Cmd that computes file diffs in a goroutine.
// For items with srcDir, also generates parallel unified diffs for modified files.
// For items with only dstDir (local only), populates file list without diff.
func expandDiffCmd(entry *copyDiffEntry) tea.Cmd {
	return func() tea.Msg {
		entry.ensureFiles()

		// For remove/local-only items without srcDir, just return file list (no diff)
		if entry.srcDir == "" {
			return diffExpandMsg{skill: entry.name, files: entry.files}
		}

		// Collect modified files for diff
		var modFiles []fileDiffEntry
		for _, f := range entry.files {
			if f.Action == "modify" {
				modFiles = append(modFiles, f)
			}
		}

		// Parallel diff computation
		results := make([]string, len(modFiles))
		var wg sync.WaitGroup
		for i, f := range modFiles {
			wg.Add(1)
			go func(idx int, fe fileDiffEntry) {
				defer wg.Done()
				src := filepath.Join(entry.srcDir, fe.RelPath)
				dst := filepath.Join(entry.dstDir, fe.RelPath)
				results[idx] = generateUnifiedDiff(src, dst)
			}(i, f)
		}
		wg.Wait()

		// Combine results preserving order
		var buf strings.Builder
		for i, f := range modFiles {
			if results[i] != "" {
				buf.WriteString(fmt.Sprintf("--- %s\n", f.RelPath))
				buf.WriteString(results[i])
			}
		}

		return diffExpandMsg{
			skill: entry.name,
			diff:  buf.String(),
			files: entry.files,
		}
	}
}

// --- Entry point ---

func runDiffTUI(results []targetDiffResult) error {
	model := newDiffTUIModel(results)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
