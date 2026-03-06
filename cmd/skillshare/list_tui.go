package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/utils"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/paginator"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// maxListItems is the maximum number of items passed to bubbles/list.
// Keeps the widget fast — pagination + filter operate on at most this many items.
const maxListItems = 1000

// applyTUIFilterStyle sets filter prompt, cursor, and input cursor to the shared style.
func applyTUIFilterStyle(l *list.Model) {
	l.Styles.FilterPrompt = tc.Filter
	l.Styles.FilterCursor = tc.Filter
	l.FilterInput.Cursor.Style = tc.Filter
}

// listLoadResult holds the result of async skill loading inside the TUI.
type listLoadResult struct {
	skills     []skillItem
	totalCount int
	err        error
}

// listLoadFn is a function that loads skills (runs in a goroutine inside the TUI).
type listLoadFn func() listLoadResult

// skillsLoadedMsg is sent when the background load completes.
type skillsLoadedMsg struct{ result listLoadResult }

// doLoadCmd returns a tea.Cmd that runs loadFn in a goroutine and sends skillsLoadedMsg.
func doLoadCmd(fn listLoadFn) tea.Cmd {
	return func() tea.Msg {
		return skillsLoadedMsg{result: fn()}
	}
}

// detailData caches the I/O-heavy fields of renderDetailPanel for a single skill.
type detailData struct {
	Description   string
	License       string
	Files         []string
	SyncedTargets []string
}

// listTUIModel is the bubbletea model for the interactive skill list.
type listTUIModel struct {
	list        list.Model
	totalCount  int
	modeLabel   string // "global" or "project"
	sourcePath  string
	targets     map[string]config.TargetConfig
	quitting    bool
	action      string // "audit", "update", "uninstall", or "" (normal quit)
	termWidth   int
	detailCache map[string]*detailData // key = RelPath; lazy-populated

	// Async loading — spinner shown until data arrives
	loading     bool
	loadSpinner spinner.Model
	loadFn      listLoadFn
	loadErr     error // non-nil if loading failed
	emptyResult bool  // true when async load returned zero skills

	// Application-level filter — replaces bubbles/list built-in fuzzy filter
	// to avoid O(N*M) fuzzy scan on 100k+ items every keystroke.
	allItems     []skillItem     // full item set (kept in memory, never passed to list)
	filterText   string          // current filter string
	filterInput  textinput.Model // managed filter text input
	filtering    bool            // true when filter input is focused
	matchCount   int             // total matches (may exceed maxListItems)
	detailScroll int

	// In-TUI confirmation overlay
	confirming    bool   // true when confirmation overlay is shown
	confirmAction string // "audit", "update", "uninstall"
	confirmSkill  string // skill name for confirmation display

	// Content viewer overlay — dual-pane: left tree + right content
	showContent     bool
	contentScroll   int
	contentText     string // current file content (rendered)
	contentSkillKey string // RelPath of skill being viewed
	termHeight      int
	treeAllNodes    []treeNode // complete flat tree (includes collapsed children)
	treeNodes       []treeNode // visible nodes (collapsed children hidden)
	treeCursor      int        // selected index in treeNodes
	treeScroll      int        // scroll offset for sidebar
}

// newListTUIModel creates a new TUI model.
// When loadFn is non-nil, skills are loaded asynchronously inside the TUI (spinner shown).
// When loadFn is nil, skills/totalCount are used directly (pre-loaded).
func newListTUIModel(loadFn listLoadFn, skills []skillItem, totalCount int, modeLabel, sourcePath string, targets map[string]config.TargetConfig) listTUIModel {
	delegate := listSkillDelegate{}

	// Build initial item set (empty if async loading)
	var items []list.Item
	var allItems []skillItem
	if loadFn == nil {
		items = buildGroupedItems(skills)
		allItems = skills
	}

	// Create list model — built-in filter DISABLED; we manage our own.
	l := list.New(items, delegate, 0, 0)
	l.Title = fmt.Sprintf("Installed skills (%s)", modeLabel)
	l.Styles.Title = tc.ListTitle
	l.SetShowStatusBar(false)    // we render our own status with real total count
	l.SetFilteringEnabled(false) // application-level filter replaces built-in
	l.SetShowHelp(false)         // we render our own help
	l.SetShowPagination(false)   // we render page info in our status line

	// Loading spinner
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = tc.SpinnerStyle

	// Filter text input
	fi := textinput.New()
	fi.Prompt = "/ "
	fi.PromptStyle = tc.Filter
	fi.Cursor.Style = tc.Filter
	fi.Placeholder = "filter or t:tracked g:group r:repo"

	m := listTUIModel{
		list:        l,
		totalCount:  totalCount,
		modeLabel:   modeLabel,
		sourcePath:  sourcePath,
		targets:     targets,
		detailCache: make(map[string]*detailData),
		loading:     loadFn != nil,
		loadSpinner: sp,
		loadFn:      loadFn,
		allItems:    allItems,
		matchCount:  len(allItems),
		filterInput: fi,
	}
	// Skip initial group header (index 0)
	if loadFn == nil {
		skipGroupItem(&m.list, 1)
	}
	return m
}

func (m listTUIModel) Init() tea.Cmd {
	if m.loading && m.loadFn != nil {
		return tea.Batch(m.loadSpinner.Tick, doLoadCmd(m.loadFn))
	}
	return nil
}

// applyFilter parses tag syntax (t:type g:group r:repo) and free text,
// then matches all non-empty conditions with AND logic.
// Results are capped at maxListItems to keep bubbles/list fast.
// When filter is empty, all items are restored (full pagination).
func (m *listTUIModel) applyFilter() {
	m.detailScroll = 0

	// No filter — restore full item set with group separators
	if m.filterText == "" {
		m.matchCount = len(m.allItems)
		m.list.SetItems(buildGroupedItems(m.allItems))
		m.list.ResetSelected()
		return
	}

	q := parseFilterQuery(m.filterText)

	// Structured match, capped at maxListItems
	var matched []list.Item
	count := 0
	for _, item := range m.allItems {
		if matchSkillItem(item, q) {
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

func (m listTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		m.syncListSize()
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.loadSpinner, cmd = m.loadSpinner.Update(msg)
			return m, cmd
		}

	case skillsLoadedMsg:
		m.loading = false
		m.loadFn = nil // release closure for GC
		if msg.result.err != nil {
			m.loadErr = msg.result.err
			m.quitting = true
			return m, tea.Quit
		}
		if msg.result.totalCount == 0 {
			m.emptyResult = true
			m.quitting = true
			return m, tea.Quit
		}
		m.allItems = msg.result.skills
		m.totalCount = msg.result.totalCount
		m.matchCount = len(msg.result.skills)
		// Populate list with group separators
		m.list.SetItems(buildGroupedItems(msg.result.skills))
		skipGroupItem(&m.list, 1)
		return m, nil

	case tea.MouseMsg:
		if m.showContent && !m.loading {
			return m.handleContentMouse(msg)
		}
		if listSplitActive(m.termWidth) && !m.loading {
			leftWidth := listPanelWidth(m.termWidth)
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
		// Ignore keys while loading
		if m.loading {
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				m.quitting = true
				return m, tea.Quit
			}
			return m, nil
		}

		// --- Content viewer: dual-pane (keyboard always controls left tree) ---
		if m.showContent {
			return m.handleContentKey(msg)
		}

		// --- Confirmation overlay ---
		if m.confirming {
			switch msg.String() {
			case "y", "Y", "enter":
				return m.quitWithAction(m.confirmAction)
			case "n", "N", "esc", "q":
				m.confirming = false
				m.confirmAction = ""
				m.confirmSkill = ""
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
				m.applyFilter()
				return m, nil
			case "enter":
				// Lock in filter, return focus to list
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

		// --- Normal mode ---
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "ctrl+d":
			m.detailScroll += 8
			return m, nil
		case "ctrl+u":
			m.detailScroll -= 8
			if m.detailScroll < 0 {
				m.detailScroll = 0
			}
			return m, nil
		case "/":
			m.filtering = true
			m.filterInput.Focus()
			return m, textinput.Blink
		case "enter", "D":
			if item, ok := m.list.SelectedItem().(skillItem); ok {
				loadContentForSkill(&m, item.entry)
				m.showContent = true
			}
			return m, nil
		case "A":
			return m.enterConfirm("audit")
		case "U":
			return m.enterConfirm("update")
		case "X":
			return m.enterConfirm("uninstall")
		}
	}

	var cmd tea.Cmd
	prevIdx := m.list.Index()
	prevSelected := selectedSkillKey(m.list.SelectedItem())
	m.list, cmd = m.list.Update(msg)

	// Auto-skip group separator items
	if _, isGroup := m.list.SelectedItem().(groupItem); isGroup {
		dir := 1
		if m.list.Index() < prevIdx {
			dir = -1
		}
		skipGroupItem(&m.list, dir)
	}

	if selectedSkillKey(m.list.SelectedItem()) != prevSelected {
		m.detailScroll = 0
	}
	return m, cmd
}

// enterConfirm shows the confirmation overlay for the given action.
func (m listTUIModel) enterConfirm(action string) (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(skillItem)
	if !ok {
		return m, nil
	}
	name := item.entry.RelPath
	if name == "" {
		name = item.entry.Name
	}
	m.confirming = true
	m.confirmAction = action
	m.confirmSkill = name
	return m, nil
}

// quitWithAction sets the action on the selected skill and exits the TUI.
func (m listTUIModel) quitWithAction(action string) (tea.Model, tea.Cmd) {
	if _, ok := m.list.SelectedItem().(skillItem); ok {
		m.action = action
	}
	m.quitting = true
	return m, tea.Quit
}

func (m listTUIModel) View() string {
	if m.quitting {
		return ""
	}

	// Loading state — spinner + message
	if m.loading {
		return fmt.Sprintf("\n  %s Loading skills...\n", m.loadSpinner.View())
	}

	// Content viewer — dual-pane
	if m.showContent {
		return renderContentOverlay(m)
	}

	// Confirmation overlay
	if m.confirming {
		flag := "-g"
		if m.modeLabel == "project" {
			flag = "-p"
		}
		cmd := fmt.Sprintf("skillshare %s %s %s", m.confirmAction, flag, m.confirmSkill)
		if m.confirmAction == "uninstall" {
			return fmt.Sprintf("\n  %s\n\n  → %s\n\n  Proceed? [Y/n] ",
				tc.Red.Render("Uninstall "+m.confirmSkill+"?"), cmd)
		}
		return fmt.Sprintf("\n  → %s\n\n  Proceed? [Y/n] ", cmd)
	}

	if listSplitActive(m.termWidth) {
		return m.viewSplit()
	}
	return m.viewVertical()
}

// renderFilterBar renders the status line for the list TUI.
func (m listTUIModel) renderFilterBar() string {
	return renderTUIFilterBar(
		m.filterInput.View(), m.filtering, m.filterText,
		m.matchCount, len(m.allItems), maxListItems,
		"skills", m.renderPageInfo(),
	)
}

func (m *listTUIModel) syncListSize() {
	if listSplitActive(m.termWidth) {
		panelHeight := m.termHeight - 5
		if panelHeight < 6 {
			panelHeight = 6
		}
		m.list.SetSize(listPanelWidth(m.termWidth), panelHeight)
		return
	}

	listHeight := m.termHeight - 20
	if listHeight < 6 {
		listHeight = 6
	}
	m.list.SetSize(m.termWidth, listHeight)
}

func listSplitActive(termWidth int) bool {
	return termWidth >= tuiMinSplitWidth
}

func listPanelWidth(termWidth int) int {
	width := termWidth * 36 / 100
	if width < 30 {
		width = 30
	}
	if width > 46 {
		width = 46
	}
	return width
}

func listDetailPanelWidth(termWidth int) int {
	width := termWidth - listPanelWidth(termWidth) - 1
	if width < 28 {
		width = 28
	}
	return width
}

func selectedSkillKey(item list.Item) string {
	skill, ok := item.(skillItem)
	if !ok {
		return ""
	}
	if skill.entry.RelPath != "" {
		return skill.entry.RelPath
	}
	return skill.entry.Name
}

func (m listTUIModel) viewSplit() string {
	var b strings.Builder

	panelHeight := m.termHeight - 5
	if panelHeight < 6 {
		panelHeight = 6
	}

	leftWidth := listPanelWidth(m.termWidth)
	rightWidth := listDetailPanelWidth(m.termWidth)

	var detailStr, scrollInfo string
	if item, ok := m.list.SelectedItem().(skillItem); ok {
		detailData := m.getDetailData(item.entry)
		header := renderDetailHeader(item.entry, detailData, rightWidth-1)
		// Leading "\n" pushes the detail content below the list title line
		// so the skill name is visible (not hidden on the same row as the title).
		bodyHeight := panelHeight - lipgloss.Height(header) - 2
		if bodyHeight < 4 {
			bodyHeight = 4
		}
		body, bodyScrollInfo := wrapAndScroll(m.renderDetailBody(item.entry, detailData, rightWidth-1), rightWidth-1, m.detailScroll, bodyHeight)
		scrollInfo = bodyScrollInfo
		detailStr = "\n" + header + "\n\n" + body
	}

	body := renderHorizontalSplit(m.list.View(), detailStr, leftWidth, rightWidth, panelHeight)
	b.WriteString(body)
	b.WriteString("\n\n")
	b.WriteString(m.renderFilterBar())
	b.WriteString(m.renderSummaryFooter())
	b.WriteString("\n")
	helpText := "↑↓ navigate  ←→ page  / filter  Ctrl+d/u detail  Enter view  A audit  U update  X uninstall  q quit"
	if m.filtering {
		helpText = "t:type g:group r:repo  Enter lock  Esc clear  q quit"
	}
	help := appendScrollInfo(helpText, scrollInfo)
	b.WriteString(tc.Help.Render(help))
	b.WriteString("\n")

	return b.String()
}

func (m listTUIModel) viewVertical() string {
	var b strings.Builder

	b.WriteString(m.list.View())
	b.WriteString("\n\n")
	b.WriteString(m.renderFilterBar())

	var scrollInfo string
	if item, ok := m.list.SelectedItem().(skillItem); ok {
		detailHeight := m.termHeight - m.termHeight*2/5 - 8
		if detailHeight < 6 {
			detailHeight = 6
		}
		detailData := m.getDetailData(item.entry)
		header := renderDetailHeader(item.entry, detailData, m.termWidth)
		bodyHeight := detailHeight - lipgloss.Height(header) - 1
		if bodyHeight < 4 {
			bodyHeight = 4
		}
		body, bodyScrollInfo := wrapAndScroll(m.renderDetailBody(item.entry, detailData, m.termWidth), m.termWidth, m.detailScroll, bodyHeight)
		scrollInfo = bodyScrollInfo
		b.WriteString(header)
		b.WriteString("\n\n")
		b.WriteString(body)
	}

	b.WriteString(m.renderSummaryFooter())
	b.WriteString("\n")
	helpText := "↑↓ navigate  ←→ page  / filter  Ctrl+d/u detail  Enter view  A audit  U update  X uninstall  q quit"
	if m.filtering {
		helpText = "t:type g:group r:repo  Enter lock  Esc clear  q quit"
	}
	help := appendScrollInfo(helpText, scrollInfo)
	b.WriteString(tc.Help.Render(help))
	b.WriteString("\n")

	return b.String()
}

func (m listTUIModel) renderSummaryFooter() string {
	localCount := 0
	trackedCount := 0
	remoteCount := 0
	for _, item := range m.allItems {
		switch {
		case item.entry.RepoName != "":
			trackedCount++
		case item.entry.Source != "":
			remoteCount++
		default:
			localCount++
		}
	}

	parts := []string{
		tc.Emphasis.Render(formatNumber(m.matchCount)) + tc.Dim.Render("/") + tc.Dim.Render(formatNumber(len(m.allItems))) + tc.Dim.Render(" visible"),
		tc.Cyan.Render(formatNumber(localCount)) + tc.Dim.Render(" local"),
		tc.Green.Render(formatNumber(trackedCount)) + tc.Dim.Render(" tracked"),
		tc.Yellow.Render(formatNumber(remoteCount)) + tc.Dim.Render(" remote"),
	}
	return tc.Help.Render(strings.Join(parts, tc.Dim.Render(" | "))) + "\n"
}

// renderTUIFilterBar renders a unified filter + status line shared by all TUIs.
// inputView is filterInput.View(). maxShown is the item cap (0 = no cap).
func renderTUIFilterBar(inputView string, filtering bool, filterText string, matchCount, totalCount, maxShown int, noun, pageInfo string) string {
	if filtering {
		if filterText == "" {
			status := fmt.Sprintf("  %s %s%s", formatNumber(totalCount), noun, pageInfo)
			return "  " + inputView + tc.Help.Render(status) + "\n"
		}
		status := fmt.Sprintf("  %s/%s %s", formatNumber(matchCount), formatNumber(totalCount), noun)
		if maxShown > 0 && matchCount > maxShown {
			status += fmt.Sprintf(" (first %s shown)", formatNumber(maxShown))
		}
		status += pageInfo
		return "  " + inputView + tc.Help.Render(status) + "\n"
	}
	if filterText != "" {
		status := fmt.Sprintf("filter: %s — %s/%s %s", filterText, formatNumber(matchCount), formatNumber(totalCount), noun)
		if maxShown > 0 && matchCount > maxShown {
			status += fmt.Sprintf(" (first %s shown)", formatNumber(maxShown))
		}
		status += pageInfo
		return tc.Help.Render(status) + "\n"
	}
	return tc.Help.Render(fmt.Sprintf("%s %s%s", formatNumber(totalCount), noun, pageInfo)) + "\n"
}

// renderPageInfo returns page indicator like " · Page 2 of 4,729" or "" if single page.
func (m listTUIModel) renderPageInfo() string {
	return renderPageInfoFromPaginator(m.list.Paginator)
}

// renderPageInfoFromPaginator returns " · Page 2 of 4,729" or "" if single page.
// Shared by list, log, and search TUIs.
func renderPageInfoFromPaginator(p paginator.Model) string {
	if p.TotalPages <= 1 {
		return ""
	}
	return fmt.Sprintf(" · Page %s of %s", formatNumber(p.Page+1), formatNumber(p.TotalPages))
}

// formatNumber formats an integer with thousand separators (e.g., 108749 → "108,749").
func formatNumber(n int) string {
	if n < 0 {
		return "-" + formatNumber(-n)
	}
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	remainder := len(s) % 3
	if remainder > 0 {
		b.WriteString(s[:remainder])
	}
	for i := remainder; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

// getDetailData returns cached detail data for a skill, populating the cache on first access.
func (m listTUIModel) getDetailData(e skillEntry) *detailData {
	key := e.RelPath
	if d, ok := m.detailCache[key]; ok {
		return d
	}

	skillDir := filepath.Join(m.sourcePath, e.RelPath)
	skillMD := filepath.Join(skillDir, "SKILL.md")

	// Single file open for both description and license
	fm := utils.ParseFrontmatterFields(skillMD, []string{"description", "license"})

	d := &detailData{
		Description:   fm["description"],
		License:       fm["license"],
		Files:         listSkillFiles(skillDir),
		SyncedTargets: m.findSyncedTargets(e),
	}
	m.detailCache[key] = d
	return d
}

// renderDetailGroup renders a titled section with indented rows (no border box).
func renderDetailGroup(title string, rows []string, _ int) string {
	// Filter out empty rows
	var filtered []string
	for _, r := range rows {
		if r != "" {
			filtered = append(filtered, r)
		}
	}
	if len(filtered) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(tc.Title.Render(title))
	b.WriteString("\n")
	for _, r := range filtered {
		b.WriteString("  ")
		b.WriteString(r)
		b.WriteString("\n")
	}
	return b.String()
}

func renderDetailCard(title string, body string, width int) string {
	style := lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Left).
		Padding(0, 0)
	if title == "" {
		return style.Render(body)
	}
	return style.Render(tc.Title.Render(title) + "\n" + body)
}

func renderDetailSection(title string, body string, width int) string {
	style := lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Left).
		Padding(0, 0)
	return style.Render(tc.Title.Render(title) + "\n" + body)
}

// renderDetailBody renders the scrollable detail body for the selected skill.
func (m listTUIModel) renderDetailBody(e skillEntry, d *detailData, width int) string {
	var b strings.Builder
	cardWidth := width
	if cardWidth < 38 {
		cardWidth = 38
	}

	// Description
	if d.Description != "" {
		maxWidth := cardWidth - 4
		if maxWidth < 32 {
			maxWidth = 32
		}
		lines := wordWrapLines(d.Description, maxWidth)
		const maxOverviewLines = 4
		if len(lines) > maxOverviewLines {
			lines = lines[:maxOverviewLines]
			lines[len(lines)-1] += "..."
		}
		body := strings.Join(renderDetailParagraph(lines), "\n")
		b.WriteString(renderDetailSection("Description", body, cardWidth))
		b.WriteString("\n\n")
	}

	// Source section (Installed date and Synced-to targets are in the header)
	var sourceRows []string
	if e.Source != "" {
		sourceRows = append(sourceRows, renderFactRow("Source", tc.Cyan.Render(e.Source)))
	} else if e.RepoName != "" {
		sourceRows = append(sourceRows, renderFactRow("Repo", e.RepoName))
	}
	if d.License != "" {
		sourceRows = append(sourceRows, renderFactRow("License", tc.Green.Bold(true).Render(d.License)))
	}
	if len(d.Files) > 0 {
		fileLabel := fmt.Sprintf("Files (%d)", len(d.Files))
		sourceRows = append(sourceRows, renderFactRow(fileLabel, tc.Cyan.Render(strings.Join(d.Files, " · "))))
	}
	if len(d.SyncedTargets) > 0 {
		sourceRows = append(sourceRows, renderFactRow("Synced to", tc.Cyan.Render(strings.Join(d.SyncedTargets, ", "))))
	}
	if len(sourceRows) > 0 {
		b.WriteString(renderDetailSection("Details", strings.Join(sourceRows, "\n"), cardWidth))
	}

	return b.String()
}

func renderDetailHeader(e skillEntry, d *detailData, width int) string {
	// Line 1: Skill path — bold name for prominence in the detail panel
	path := baseSkillPath(e)
	title := colorSkillPathBold(path)

	var body strings.Builder
	body.WriteString(title)

	// Line 2: Compact metadata — status · date · targets on one line
	var metaParts []string
	metaParts = append(metaParts, detailStatusBits(e))
	if e.InstalledAt != "" {
		metaParts = append(metaParts, tc.Dim.Render(e.InstalledAt))
	}
	if len(d.SyncedTargets) > 0 {
		metaParts = append(metaParts, tc.Cyan.Render(fmt.Sprintf("%d target(s)", len(d.SyncedTargets))))
	}
	body.WriteString("\n\n")
	body.WriteString(strings.Join(metaParts, tc.Dim.Render("  ·  ")))

	return renderDetailCard("", body.String(), width)
}

func renderDetailParagraph(lines []string) []string {
	rendered := make([]string, 0, len(lines))
	for _, line := range lines {
		rendered = append(rendered, tc.Value.Render(line))
	}
	return rendered
}

func detailStatusBits(e skillEntry) string {
	var bits []string
	switch {
	case e.RepoName != "":
		bits = append(bits, tc.Green.Render("tracked"))
	case e.Source != "":
		bits = append(bits, tc.Yellow.Render("remote"))
	default:
		bits = append(bits, tc.Dim.Render("local"))
	}
	return strings.Join(bits, "  ")
}

func renderFactRow(label, value string) string {
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("247")).Width(12)
	return labelStyle.Render(label+":") + " " + tc.Value.Render(value)
}

// listSkillFiles returns visible file names in the skill directory.
func listSkillFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".") {
			name := e.Name()
			if e.IsDir() {
				name += "/"
			}
			names = append(names, name)
		}
	}
	return names
}

// findSyncedTargets returns target names where this skill has a symlink.
func (m listTUIModel) findSyncedTargets(e skillEntry) []string {
	if m.targets == nil {
		return nil
	}
	flatName := e.Name
	if e.RelPath != "" {
		flatName = utils.PathToFlatName(e.RelPath)
	}

	var synced []string
	for name, tc := range m.targets {
		linkPath := filepath.Join(tc.Path, flatName)
		if utils.IsSymlinkOrJunction(linkPath) {
			synced = append(synced, name)
		}
	}
	sort.Strings(synced)
	return synced
}

// runListTUI starts the bubbletea TUI for the skill list.
// When loadFn is non-nil, data is loaded asynchronously inside the TUI (no blank screen).
// Returns (action, skillName, error). action is "" on normal quit (q/ctrl+c).
func runListTUI(loadFn listLoadFn, modeLabel, sourcePath string, targets map[string]config.TargetConfig) (string, string, error) {
	model := newListTUIModel(loadFn, nil, 0, modeLabel, sourcePath, targets)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	finalModel, err := p.Run()
	if err != nil {
		return "", "", err
	}

	m, ok := finalModel.(listTUIModel)
	if !ok || m.action == "" {
		if m.loadErr != nil {
			return "", "", m.loadErr
		}
		if m.emptyResult {
			return "empty", "", nil
		}
		return "", "", nil
	}

	// Extract skill name from selected item
	var skillName string
	if item, ok := m.list.SelectedItem().(skillItem); ok {
		if item.entry.RelPath != "" {
			skillName = item.entry.RelPath
		} else {
			skillName = item.entry.Name
		}
	}
	return m.action, skillName, nil
}

// wordWrapLines splits text into lines that fit within maxWidth, breaking at word boundaries.
func wordWrapLines(text string, maxWidth int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}

	var lines []string
	cur := words[0]
	for _, w := range words[1:] {
		if len(cur)+1+len(w) > maxWidth {
			lines = append(lines, cur)
			cur = w
		} else {
			cur += " " + w
		}
	}
	lines = append(lines, cur)
	return lines
}
