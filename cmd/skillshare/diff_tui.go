package main

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"skillshare/internal/theme"

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

// --- List items ---

type diffTargetItem struct {
	result targetDiffResult
}

// diffExtraItem wraps an extraDiffResult for the bubbletea list.
type diffExtraItem struct {
	result extraDiffResult
}

// row returns the row's text and its right column.
func (i diffExtraItem) row() (string, string) {
	r := i.result
	switch {
	case r.errMsg != "":
		return theme.Danger().Render("✗") + " " + r.extraName, theme.Danger().Render("error")
	case r.synced:
		return theme.Success().Render("✓") + " " + r.extraName, theme.Dim().Render("in sync")
	}
	return theme.Warning().Render("!") + " " + r.extraName, theme.Dim().Render(countNoun(len(r.items), "difference"))
}

func (i diffExtraItem) FilterValue() string { return i.result.extraName }

// diffSeparatorItem is a non-selectable visual separator / group header.
type diffSeparatorItem struct {
	label string
	count int  // 0 = no count displayed
	space bool // true = empty spacer row
}

func (s diffSeparatorItem) FilterValue() string { return "" }

// diffItemDelegate renders targets and extras as one-line rows and
// separators as group rows.
type diffItemDelegate struct{}

func (diffItemDelegate) Height() int                             { return 1 }
func (diffItemDelegate) Spacing() int                            { return 0 }
func (diffItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d diffItemDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	var left, right string
	switch v := item.(type) {
	case diffSeparatorItem:
		if !v.space {
			renderGroupRow(w, groupItem{label: v.label}, m.Width())
		}
		return
	case diffTargetItem:
		left, right = v.row()
	case diffExtraItem:
		left, right = v.row()
	default:
		return
	}
	renderPrefixRow(w, alignRow(left, right, m.Width()-rowIndent), m.Width(), index == m.Index())
}

// skipDiffSeparator advances the list selection past diffSeparatorItem entries.
func skipDiffSeparator(l *list.Model, direction int) {
	items := l.Items()
	idx := l.Index()
	n := len(items)
	for idx >= 0 && idx < n {
		if _, isSep := items[idx].(diffSeparatorItem); !isSep {
			break
		}
		idx += direction
	}
	if idx >= 0 && idx < n {
		l.Select(idx)
	}
}

// row returns the row's text and its right column: what differs.
func (i diffTargetItem) row() (string, string) {
	r := i.result
	switch {
	case r.errMsg != "":
		return theme.Danger().Render("✗") + " " + r.name, theme.Danger().Render("error")
	case r.synced:
		return theme.Success().Render("✓") + " " + r.name, theme.Dim().Render("in sync")
	}
	var parts []string
	if r.syncCount > 0 {
		parts = append(parts, fmt.Sprintf("%d to sync", r.syncCount))
	}
	if r.localCount > 0 {
		parts = append(parts, fmt.Sprintf("%d local", r.localCount))
	}
	desc := "differs"
	if len(parts) > 0 {
		desc = strings.Join(parts, " · ")
	}
	return theme.Warning().Render("!") + " " + r.name, theme.Dim().Render(desc)
}

func (i diffTargetItem) FilterValue() string { return i.result.name }

// --- Model ---

type diffTUIModel struct {
	quitting   bool
	termWidth  int
	termHeight int

	// Data — sorted: error → diff → synced
	allItems  []targetDiffResult
	allExtras []extraDiffResult

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

	showKeys bool // ? swaps the detail panel for the full key list
}

func newDiffTUIModel(results []targetDiffResult, extrasSlice ...[]extraDiffResult) diffTUIModel {
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

	var extras []extraDiffResult
	if len(extrasSlice) > 0 {
		extras = extrasSlice[0]
	}

	// Build list items with group headers
	var listItems []list.Item
	listItems = append(listItems, diffSeparatorItem{label: "Targets", count: len(sorted)})
	for _, r := range sorted {
		listItems = append(listItems, diffTargetItem{result: r})
	}
	// Append extras with spacer + separator
	if len(extras) > 0 {
		listItems = append(listItems, diffSeparatorItem{space: true})
		listItems = append(listItems, diffSeparatorItem{label: "Extras", count: len(extras)})
		for _, r := range extras {
			listItems = append(listItems, diffExtraItem{result: r})
		}
	}

	tl := list.New(listItems, diffItemDelegate{}, 0, 0)
	tl.SetShowTitle(false)
	tl.SetShowStatusBar(false)
	tl.SetFilteringEnabled(false)
	tl.SetShowHelp(false)
	tl.SetShowPagination(false)
	skipDiffSeparator(&tl, 1)

	fi := newTUIFilterInput("type to match a target or extra")

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = theme.Accent()

	return diffTUIModel{
		allItems:    sorted,
		allExtras:   extras,
		targetList:  tl,
		matchCount:  len(listItems),
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
		if m.termWidth < diffMinSplitWidth {
			lw = m.termWidth
		}
		m.targetList.SetSize(lw, m.diffPanelHeight())
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
		cmd := handleTUIFilterKey(msg, &m.filtering, &m.filterText, &m.filterInput, m.applyDiffFilter)
		return m, cmd
	}

	// Normal keys
	switch key {
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
			m.applyDiffFilter()
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

	// Skip separator items
	if m.targetList.Index() != prevIdx {
		direction := 1
		if m.targetList.Index() < prevIdx {
			direction = -1
		}
		skipDiffSeparator(&m.targetList, direction)

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
		var items []list.Item
		items = append(items, diffSeparatorItem{label: "Targets", count: len(m.allItems)})
		for _, r := range m.allItems {
			items = append(items, diffTargetItem{result: r})
		}
		if len(m.allExtras) > 0 {
			items = append(items, diffSeparatorItem{space: true})
			items = append(items, diffSeparatorItem{label: "Extras", count: len(m.allExtras)})
			for _, r := range m.allExtras {
				items = append(items, diffExtraItem{result: r})
			}
		}
		m.matchCount = len(items)
		m.targetList.SetItems(items)
		m.targetList.ResetSelected()
		skipDiffSeparator(&m.targetList, 1)
		m.cachedItems = nil // invalidate cache
		return
	}
	var matched []list.Item
	for _, r := range m.allItems {
		if strings.Contains(strings.ToLower(r.name), needle) {
			matched = append(matched, diffTargetItem{result: r})
		}
	}
	for _, r := range m.allExtras {
		if strings.Contains(strings.ToLower(r.extraName), needle) {
			matched = append(matched, diffExtraItem{result: r})
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
	return max(termWidth-diffListWidth(termWidth), 30)
}

func (m diffTUIModel) diffPanelHeight() int {
	bodyHeight := max(m.termHeight-frameChrome, 6)
	if m.termWidth >= diffMinSplitWidth {
		return bodyHeight
	}
	return max(bodyHeight/2, 4) // narrow: the details sit below the list
}

// --- Views ---

func (m diffTUIModel) View() string {
	if m.quitting {
		return ""
	}
	bodyHeight := max(m.termHeight-frameChrome, 6)
	title := m.renderTitleLine()
	if m.termWidth < diffMinSplitWidth {
		detailHeight := max(bodyHeight-m.diffPanelHeight()-1, 4)
		detail := lipgloss.NewStyle().Height(detailHeight).MaxHeight(detailHeight).PaddingLeft(1).
			Render(m.renderRight(m.termWidth-2, detailHeight))
		return title + "\n\n" + m.targetList.View() + "\n\n" + detail + "\n" + m.renderBottom()
	}
	leftWidth := diffListWidth(m.termWidth)
	rightWidth := diffDetailWidth(m.termWidth)
	return title + "\n\n" +
		renderFrameSplit(m.targetList.View(), m.renderRight(rightWidth-2, bodyHeight), leftWidth, rightWidth, bodyHeight) + "\n" +
		m.renderBottom()
}

// renderTitleLine renders the target and extra counts and how many differ.
func (m diffTUIModel) renderTitleLine() string {
	var errN, diffN, syncN int
	for _, r := range m.allItems {
		switch {
		case r.errMsg != "":
			errN++
		case !r.synced:
			diffN++
		default:
			syncN++
		}
	}
	facts := []string{countNoun(len(m.allItems), "target")}
	if len(m.allExtras) > 0 {
		facts = append(facts, countNoun(len(m.allExtras), "extra"))
	}
	if syncN > 0 {
		facts = append(facts, formatNumber(syncN)+" in sync")
	}
	// Colored facts go last; a colored fact ends the dim run of facts.
	if diffN > 0 {
		facts = append(facts, theme.Warning().Render(formatNumber(diffN)+" differ"))
	}
	if errN > 0 {
		facts = append(facts, theme.Danger().Render(countNoun(errN, "error")))
	}
	return renderFrameTitle(m.termWidth, "diff", facts, nil)
}

// renderRight renders the detail panel, or the key list while ? is on.
func (m diffTUIModel) renderRight(width, height int) string {
	if m.showKeys {
		return renderKeysPanel(diffKeyGroups)
	}
	detail, _ := wrapAndScroll(m.buildDiffDetail(), width, m.detailScroll, height)
	return detail
}

// renderBottom renders the note line and the key line; the filter input
// takes over the key line in place.
func (m diffTUIModel) renderBottom() string {
	var line string
	switch {
	case m.filtering:
		line = renderFilterLine(m.termWidth, m.filterInput.View(), m.matchCount)
	case m.showKeys:
		line = renderKeyLine(m.termWidth, []keyHint{{"?/esc", "close"}}, "")
	default:
		filter := keyHint{"/", "filter"}
		if m.filterText != "" {
			filter = keyHint{"esc", "clear filter"}
		}
		enter := keyHint{"enter", "show files"}
		if m.expandedSkill != "" {
			enter = keyHint{"enter", "hide files"}
		}
		hints := []keyHint{{"↑↓", "move"}, filter, enter, {"ctrl+d/u", "scroll"}, {"?", "keys"}}
		line = renderKeyLine(m.termWidth, hints, "")
	}
	return "\n" + line
}

// diffKeyGroups lists every key for the ? panel.
var diffKeyGroups = []keyGroup{
	{"Move", []keyHint{
		{"↑↓", "move"},
		{"←→", "page"},
		{"/", "filter"},
		{"enter", "show or hide the file-level diff"},
		{"ctrl+d/u", "scroll the details"},
		{"esc", "clear the filter, then quit"},
		{"q", "quit"},
	}},
}

// --- Detail renderer ---

func (m diffTUIModel) buildExtraDetail(selected diffExtraItem) string {
	r := selected.result
	var b strings.Builder

	row := func(label, value string) {
		b.WriteString(theme.Dim().Width(10).Render(label))
		b.WriteString(value)
		b.WriteString("\n")
	}

	b.WriteString(theme.Primary().Bold(true).Render(r.extraName))
	b.WriteString("\n\n")
	row("Target", shortenPath(r.targetPath))
	row("Mode", r.mode)
	b.WriteString("\n")

	if r.errMsg != "" {
		b.WriteString(theme.Danger().Render("✗") + " " + r.errMsg)
		b.WriteString("\n")
		return b.String()
	}
	if r.synced {
		b.WriteString(theme.Success().Render("✓") + " In sync")
		b.WriteString("\n")
		return b.String()
	}

	b.WriteString(theme.Primary().Bold(true).Render(countNoun(len(r.items), "difference")))
	b.WriteString("\n")
	hasLocal := false
	for _, item := range r.items {
		var prefix string
		var style lipgloss.Style
		switch item.action {
		case "add":
			prefix, style = "+ ", theme.Success()
		case "remove":
			prefix, style = "- ", theme.Danger()
		case "modify":
			prefix, style = "~ ", theme.Accent()
			if item.reason == "not a symlink (local file)" {
				hasLocal = true
			}
		default:
			prefix, style = "  ", theme.Dim()
		}
		b.WriteString(style.Render(prefix+item.file) + "  " + theme.Dim().Render(item.reason))
		b.WriteString("\n")
	}

	// Next Steps
	b.WriteString("\n")
	b.WriteString(theme.Primary().Bold(true).Render("Next"))
	b.WriteString("\n")
	b.WriteString("skillshare sync extras")
	b.WriteString("\n")
	if hasLocal {
		b.WriteString("skillshare extras collect " + r.extraName)
		b.WriteString("\n")
	}

	return b.String()
}

func (m diffTUIModel) buildDiffDetail() string {
	selectedItem := m.targetList.SelectedItem()
	switch selected := selectedItem.(type) {
	case diffExtraItem:
		return m.buildExtraDetail(selected)
	case diffSeparatorItem:
		return ""
	case diffTargetItem:
		// handled below
	default:
		return ""
	}
	item := selectedItem.(diffTargetItem)
	r := item.result

	var b strings.Builder

	row := func(label, value string) {
		b.WriteString(theme.Dim().Width(10).Render(label))
		b.WriteString(value)
		b.WriteString("\n")
	}

	b.WriteString(theme.Primary().Bold(true).Render(r.name))
	b.WriteString("\n\n")
	row("Mode", r.mode)
	if len(r.include) > 0 {
		row("Include", strings.Join(r.include, ", "))
	}
	if len(r.exclude) > 0 {
		row("Exclude", strings.Join(r.exclude, ", "))
	}
	if !r.srcMtime.IsZero() {
		row("Source", "changed "+r.srcMtime.Format("2006-01-02 15:04"))
	}
	if !r.dstMtime.IsZero() {
		row("Target", "changed "+r.dstMtime.Format("2006-01-02 15:04"))
	}

	b.WriteString("\n")

	// Error
	if r.errMsg != "" {
		b.WriteString(theme.Danger().Render("✗") + " " + r.errMsg)
		b.WriteString("\n")
		return b.String()
	}

	// Fully synced
	if r.synced {
		b.WriteString(theme.Success().Render("✓") + " In sync")
		b.WriteString("\n")
		return b.String()
	}

	// Loading spinner
	if m.loading {
		b.WriteString(m.loadSpinner.View() + " Loading the diff…\n")
		return b.String()
	}

	// Build agent name set for [A] badge rendering
	agentNames := make(map[string]bool, len(m.cachedItems))
	for _, item := range m.cachedItems {
		if item.kind == "agent" {
			agentNames[item.name] = true
		}
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
			kindStyle = theme.Success()
		case "modified":
			kindStyle = theme.Accent()
		case "override":
			kindStyle = theme.Warning()
		case "orphan":
			kindStyle = theme.Danger()
		case "local":
			kindStyle = theme.Dim()
		case "warn":
			kindStyle = theme.Danger()
		default:
			kindStyle = theme.Dim()
		}

		header := fmt.Sprintf("%s %d %s", cat.label, n, skillWord)
		b.WriteString(kindStyle.Render(header))
		b.WriteString("\n")

		if cat.expand {
			for _, name := range cat.names {
				if agentNames[name] {
					b.WriteString("  " + name + theme.Dim().Render("  agent"))
				} else {
					b.WriteString("  " + name)
				}
				b.WriteString("\n")
			}
		}
	}

	// File list + diff content (only shown after Enter toggle)
	if m.expandedSkill != "" {
		if len(m.expandedFiles) > 0 {
			b.WriteString("\n")
			b.WriteString(theme.Primary().Bold(true).Render(m.expandedSkill + " files"))
			b.WriteString("\n")
			for _, f := range m.expandedFiles {
				var icon string
				var style lipgloss.Style
				switch f.Action {
				case "add":
					icon, style = "+", theme.Success()
				case "delete":
					icon, style = "-", theme.Danger()
				case "modify":
					icon, style = "~", theme.Accent()
				default:
					icon, style = "?", theme.Dim()
				}
				b.WriteString(style.Render(icon) + " " + f.RelPath)
				b.WriteString("\n")
			}
		}

		// Unified diff content
		if m.expandedDiff != "" {
			b.WriteString("\n")
			b.WriteString(theme.Primary().Bold(true).Render(m.expandedSkill + " diff"))
			b.WriteString("\n")
			for _, line := range strings.Split(strings.TrimRight(m.expandedDiff, "\n"), "\n") {
				switch {
				case strings.HasPrefix(line, "+ "):
					b.WriteString(theme.Success().Render(line))
				case strings.HasPrefix(line, "- "):
					b.WriteString(theme.Danger().Render(line))
				case strings.HasPrefix(line, "--- "):
					b.WriteString(theme.Accent().Render(line))
				default:
					b.WriteString(theme.Dim().Render(line))
				}
				b.WriteString("\n")
			}
		}
		if len(m.expandedFiles) == 0 && m.expandedDiff == "" {
			b.WriteString("\n")
			b.WriteString(theme.Dim().Render("(No file-level diff available)"))
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
		b.WriteString(theme.Primary().Bold(true).Render("Next"))
		b.WriteString("\n")
		seen := map[string]bool{}
		for _, h := range hints {
			if seen[h] {
				continue
			}
			seen[h] = true
			b.WriteString("skillshare " + h)
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

func runDiffTUI(results []targetDiffResult, extrasResults ...[]extraDiffResult) error {
	model := newDiffTUIModel(results, extrasResults...)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
