package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/skillignore"
	"skillshare/internal/theme"
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

// listTab represents the active tab filter in the list TUI.
type listTab int

const (
	listTabAll    listTab = iota // show all items (skills + agents)
	listTabSkills                // show skills only
	listTabAgents                // show agents only
)

// listStatusFilter pre-filters items by enabled/disabled state.
// Cycled with the `s` key: All → Enabled → Disabled → All.
type listStatusFilter int

const (
	statusFilterAll      listStatusFilter = iota // show enabled + disabled
	statusFilterEnabled                          // show only enabled
	statusFilterDisabled                         // show only disabled
)

// label returns the display name shown in the status chip.
func (s listStatusFilter) label() string {
	switch s {
	case statusFilterEnabled:
		return "Enabled"
	case statusFilterDisabled:
		return "Disabled"
	default:
		return "All"
	}
}

// applyTUIFilterStyle sets filter prompt, cursor, and input cursor to the shared style.
func applyTUIFilterStyle(l *list.Model) {
	l.Styles.FilterPrompt = theme.Accent()
	l.Styles.FilterCursor = theme.Accent()
	l.FilterInput.Cursor.Style = theme.Accent()
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
	list             list.Model
	totalCount       int
	modeLabel        string // "global" or "project"
	sourcePath       string
	agentsSourcePath string
	targets          map[string]config.TargetConfig
	quitting         bool
	action           string // "audit", "update", "uninstall", or "" (normal quit)
	termWidth        int
	detailCache      map[string]*detailData // key = RelPath; lazy-populated

	// Async loading — spinner shown until data arrives
	loading     bool
	loadSpinner spinner.Model
	loadFn      listLoadFn
	loadErr     error // non-nil if loading failed
	emptyResult bool  // true when async load returned zero skills

	// Tab filter — pre-filters allItems by kind (All / Skills / Agents)
	activeTab   listTab     // currently selected tab
	activeTabP  *listTab    // shared pointer for delegate to read current tab
	tabCounts   [3]int      // cached counts: [all, skills, agents]
	tabFiltered []skillItem // cached result of tab + status filter; set by applyFilter()

	// Status filter — pre-filters by enabled/disabled state (cycled with `s`)
	statusFilter listStatusFilter

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
	confirmKind   string // "skill" or "agent"

	// Content viewer overlay — dual-pane: left tree + right content
	showContent     bool
	contentScroll   int
	contentText     string // current file content (rendered)
	contentSkillKey string // RelPath of skill being viewed
	contentKind     string // "skill" or "agent" — set when entering content view
	termHeight      int
	treeAllNodes    []treeNode // complete flat tree (includes collapsed children)
	treeNodes       []treeNode // visible nodes (collapsed children hidden)
	treeCursor      int        // selected index in treeNodes
	treeScroll      int        // scroll offset for sidebar
}

// newListTUIModel creates a new TUI model.
// When loadFn is non-nil, skills are loaded asynchronously inside the TUI (spinner shown).
// When loadFn is nil, skills/totalCount are used directly (pre-loaded).
func newListTUIModel(loadFn listLoadFn, skills []skillItem, totalCount int, modeLabel, sourcePath, agentsSourcePath string, targets map[string]config.TargetConfig, initialKind resourceKindFilter) listTUIModel {
	// Map CLI kind filter to initial tab
	var initTab listTab
	switch initialKind {
	case kindAgents:
		initTab = listTabAgents
	case kindSkills:
		initTab = listTabSkills
	default:
		initTab = listTabAll
	}

	// Shared pointer lets the delegate read the current tab without re-creation.
	tabPtr := new(listTab)
	*tabPtr = initTab
	delegate := listSkillDelegate{activeTab: tabPtr}

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
	l.Styles.Title = theme.Title()
	l.Styles.NoItems = l.Styles.NoItems.PaddingLeft(2) // align with title
	l.SetShowStatusBar(false)                          // we render our own status with real total count
	l.SetFilteringEnabled(false)                       // application-level filter replaces built-in
	l.SetShowHelp(false)                               // we render our own help
	l.SetShowPagination(false)                         // we render page info in our status line

	// Loading spinner
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = theme.Accent()

	// Filter text input
	fi := textinput.New()
	fi.Prompt = "/ "
	fi.PromptStyle = theme.Accent()
	fi.Cursor.Style = theme.Accent()
	fi.Placeholder = "filter or t:type g:group r:repo k:kind"

	m := listTUIModel{
		list:             l,
		totalCount:       totalCount,
		modeLabel:        modeLabel,
		sourcePath:       sourcePath,
		agentsSourcePath: agentsSourcePath,
		targets:          targets,
		activeTab:        initTab,
		activeTabP:       tabPtr,
		detailCache:      make(map[string]*detailData),
		loading:          loadFn != nil,
		loadSpinner:      sp,
		loadFn:           loadFn,
		allItems:         allItems,
		matchCount:       len(allItems),
		filterInput:      fi,
	}
	if loadFn == nil {
		m.recomputeTabCounts()
		m.applyFilter() // applies tab + text filter
		skipGroupItem(&m.list, 1)
	}
	m.updateTitle()
	return m
}

// recomputeTabCounts updates the cached per-tab counts from allItems.
func (m *listTUIModel) recomputeTabCounts() {
	var skills, agents int
	for _, item := range m.allItems {
		if item.entry.Kind == "agent" {
			agents++
		} else {
			skills++
		}
	}
	m.tabCounts = [3]int{len(m.allItems), skills, agents}
}

// tabFilteredItems returns the subset of allItems matching the active tab.
// For listTabAll, items are reordered so skills come first then agents,
// keeping each kind's original order intact.
func (m *listTUIModel) tabFilteredItems() []skillItem {
	if m.activeTab == listTabAll {
		skills := make([]skillItem, 0, m.tabCounts[1])
		agents := make([]skillItem, 0, m.tabCounts[2])
		for _, item := range m.allItems {
			if item.entry.Kind == "agent" {
				agents = append(agents, item)
			} else {
				skills = append(skills, item)
			}
		}
		return append(skills, agents...)
	}
	wantAgent := m.activeTab == listTabAgents
	cap := m.tabCounts[1]
	if wantAgent {
		cap = m.tabCounts[2]
	}
	filtered := make([]skillItem, 0, cap)
	for _, item := range m.allItems {
		if (item.entry.Kind == "agent") == wantAgent {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// statusFilteredItems returns the subset of items matching the active status
// filter. Order is preserved. statusFilterAll returns items unchanged.
func (m *listTUIModel) statusFilteredItems(items []skillItem) []skillItem {
	if m.statusFilter == statusFilterAll {
		return items
	}
	wantDisabled := m.statusFilter == statusFilterDisabled
	filtered := make([]skillItem, 0, len(items))
	for _, item := range items {
		if item.entry.Disabled == wantDisabled {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// tabNoun returns the display noun for the active tab.
func (t listTab) noun() string {
	switch t {
	case listTabSkills:
		return "skills"
	case listTabAgents:
		return "agents"
	default:
		return "resources"
	}
}

// updateTitle sets the list title based on the active tab and mode.
func (m *listTUIModel) updateTitle() {
	m.list.Title = fmt.Sprintf("Installed %s (%s)", m.activeTab.noun(), m.modeLabel)
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
	m.tabFiltered = m.statusFilteredItems(m.tabFilteredItems())

	// No filter — restore tab-filtered item set with group separators
	if m.filterText == "" {
		m.matchCount = len(m.tabFiltered)
		m.list.SetItems(buildGroupedItems(m.tabFiltered))
		m.list.ResetSelected()
		return
	}

	q := parseFilterQuery(m.filterText)

	// Structured match, capped at maxListItems
	var matched []list.Item
	count := 0
	for _, item := range m.tabFiltered {
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
		m.recomputeTabCounts()
		m.applyFilter() // applies tab + text filter, sets matchCount
		skipGroupItem(&m.list, 1)
		m.updateTitle()
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
				m.confirmKind = ""
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
		case "tab":
			m.activeTab = (m.activeTab + 1) % 3
			*m.activeTabP = m.activeTab
			m.applyFilter()
			m.updateTitle()
			skipGroupItem(&m.list, 1)
			return m, nil
		case "shift+tab":
			m.activeTab = (m.activeTab - 1 + 3) % 3
			*m.activeTabP = m.activeTab
			m.applyFilter()
			m.updateTitle()
			skipGroupItem(&m.list, 1)
			return m, nil
		case "s":
			m.statusFilter = (m.statusFilter + 1) % 3
			m.applyFilter()
			skipGroupItem(&m.list, 1)
			return m, nil
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
		case "E":
			return m.toggleDisabled()
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
	m.confirmKind = item.entry.Kind
	return m, nil
}

// toggleDisabled toggles the selected skill's disabled state in-place
// by writing/removing its relPath in .skillignore. Stays in the TUI.
func (m listTUIModel) toggleDisabled() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(skillItem)
	if !ok {
		return m, nil
	}

	var ignorePath string
	if item.entry.Kind == "agent" {
		ignorePath = filepath.Join(m.agentsSourcePath, ".agentignore")
	} else {
		ignorePath = filepath.Join(m.sourcePath, ".skillignore")
	}
	pattern := item.entry.RelPath
	if pattern == "" {
		pattern = item.entry.Name
	}

	newDisabled := !item.entry.Disabled
	var writeErr error
	if newDisabled {
		_, writeErr = skillignore.AddPattern(ignorePath, pattern)
	} else {
		_, writeErr = skillignore.RemovePattern(ignorePath, pattern)
	}
	if writeErr != nil {
		return m, nil
	}

	// Toggle the flag in allItems
	for i := range m.allItems {
		if m.allItems[i].entry.RelPath == item.entry.RelPath {
			m.allItems[i].entry.Disabled = newDisabled
			break
		}
	}

	// Clear detail cache so the panel re-renders
	delete(m.detailCache, item.entry.RelPath)

	// When a status filter is active, the toggled item no longer matches it —
	// re-apply the filter so it leaves the view (selection resets).
	if m.statusFilter != statusFilterAll {
		m.applyFilter()
		skipGroupItem(&m.list, 1)
		return m, nil
	}

	// Otherwise patch the flag in place to preserve selection.
	items := m.list.Items()
	for i, li := range items {
		if si, ok := li.(skillItem); ok && si.entry.RelPath == item.entry.RelPath {
			si.entry.Disabled = newDisabled
			items[i] = si
			break
		}
	}
	m.list.SetItems(items)
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
		return fmt.Sprintf("\n  %s Loading %s...\n", m.loadSpinner.View(), m.activeTab.noun())
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
		kindArg := ""
		if m.confirmKind == "agent" {
			kindArg = "agents "
		}
		cmd := fmt.Sprintf("skillshare %s %s%s %s", m.confirmAction, kindArg, flag, m.confirmSkill)
		if m.confirmAction == "uninstall" {
			return fmt.Sprintf("\n  %s\n\n  → %s\n\n  Proceed? [Y/n] ",
				theme.Danger().Render("Uninstall "+m.confirmSkill+"?"), cmd)
		}
		return fmt.Sprintf("\n  → %s\n\n  Proceed? [Y/n] ", cmd)
	}

	if listSplitActive(m.termWidth) {
		return m.viewSplit()
	}
	return m.viewVertical()
}

// renderTabBar renders the kind tab bar (All / Skills / Agents).
func (m listTUIModel) renderTabBar() string {
	type tab struct {
		label string
		tab   listTab
		count int
	}
	tabs := []tab{
		{"All", listTabAll, m.tabCounts[0]},
		{"Skills", listTabSkills, m.tabCounts[1]},
		{"Agents", listTabAgents, m.tabCounts[2]},
	}

	activeStyle := lipgloss.NewStyle().Bold(true).Underline(true)
	inactiveStyle := theme.Dim()

	var parts []string
	for _, t := range tabs {
		label := fmt.Sprintf("%s(%d)", t.label, t.count)
		if t.tab == m.activeTab {
			parts = append(parts, activeStyle.Inherit(theme.Accent()).Render(label))
		} else {
			parts = append(parts, inactiveStyle.Render(label))
		}
	}
	return "  " + strings.Join(parts, "  ") + "     " + m.renderStatusChip()
}

// renderStatusChip renders the enabled/disabled status filter indicator shown
// next to the tab bar. Always visible so the `s` filter is discoverable.
func (m listTUIModel) renderStatusChip() string {
	label := "Status: " + m.statusFilter.label()
	switch m.statusFilter {
	case statusFilterEnabled:
		return theme.Success().Render(label)
	case statusFilterDisabled:
		return theme.Warning().Render(label)
	default:
		return theme.Dim().Render(label)
	}
}

// renderFilterBar renders the status line for the list TUI.
func (m listTUIModel) renderFilterBar() string {
	return renderTUIFilterBar(
		m.filterInput.View(), m.filtering, m.filterText,
		m.matchCount, len(m.tabFiltered), maxListItems,
		m.activeTab.noun(), m.renderPageInfo(),
	)
}

func (m *listTUIModel) syncListSize() {
	if listSplitActive(m.termWidth) {
		// tab(1) + gap(1) + panel + gap(1) + filter(1) + summary(1) + gap(1) + help(1) + trail(1) = 8 overhead
		panelHeight := m.termHeight - 8
		if panelHeight < 6 {
			panelHeight = 6
		}
		m.list.SetSize(listPanelWidth(m.termWidth), panelHeight)
		return
	}

	listHeight := m.termHeight - 22
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

	b.WriteString(m.renderTabBar())
	b.WriteString("\n\n")

	panelHeight := m.termHeight - 8
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
	helpText := "Tab skills/agents  s status  ↑↓ navigate  ←→ page  / filter  Ctrl+d/u detail  Enter view  A audit  U update  E enable/disable  X uninstall  q quit"
	if m.filtering {
		helpText = "t:type g:group r:repo k:kind  Enter lock  Esc clear  q quit"
	}
	help := appendScrollInfo(helpText, scrollInfo)
	b.WriteString(formatHelpBar(help))
	b.WriteString("\n")

	return b.String()
}

func (m listTUIModel) viewVertical() string {
	var b strings.Builder

	b.WriteString(m.renderTabBar())
	b.WriteString("\n\n")
	b.WriteString(m.list.View())
	b.WriteString("\n\n")
	b.WriteString(m.renderFilterBar())

	var scrollInfo string
	if item, ok := m.list.SelectedItem().(skillItem); ok {
		detailHeight := m.termHeight - m.termHeight*2/5 - 10 // -2 for tab bar
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
	helpText := "Tab skills/agents  s status  ↑↓ navigate  ←→ page  / filter  Ctrl+d/u detail  Enter view  A audit  U update  E enable/disable  X uninstall  q quit"
	if m.filtering {
		helpText = "t:type g:group r:repo k:kind  Enter lock  Esc clear  q quit"
	}
	help := appendScrollInfo(helpText, scrollInfo)
	b.WriteString(formatHelpBar(help))
	b.WriteString("\n")

	return b.String()
}

func (m listTUIModel) renderSummaryFooter() string {
	localCount := 0
	trackedCount := 0
	remoteCount := 0
	for _, item := range m.tabFiltered {
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
		theme.Primary().Render(formatNumber(m.matchCount)) + theme.Dim().Render("/") + theme.Dim().Render(formatNumber(len(m.tabFiltered))) + theme.Dim().Render(" visible"),
		theme.Accent().Render(formatNumber(localCount)) + theme.Dim().Render(" local"),
		theme.Success().Render(formatNumber(trackedCount)) + theme.Dim().Render(" tracked"),
		theme.Warning().Render(formatNumber(remoteCount)) + theme.Dim().Render(" remote"),
	}
	return theme.Dim().MarginLeft(2).Render(strings.Join(parts, theme.Dim().Render(" | "))) + "\n"
}

// renderTUIFilterBar renders a unified filter + status line shared by all TUIs.
// inputView is filterInput.View(). maxShown is the item cap (0 = no cap).
func renderTUIFilterBar(inputView string, filtering bool, filterText string, matchCount, totalCount, maxShown int, noun, pageInfo string) string {
	if filtering {
		if filterText == "" {
			status := fmt.Sprintf("  %s %s%s", formatNumber(totalCount), noun, pageInfo)
			return "  " + inputView + theme.Dim().MarginLeft(2).Render(status) + "\n"
		}
		status := fmt.Sprintf("  %s/%s %s", formatNumber(matchCount), formatNumber(totalCount), noun)
		if maxShown > 0 && matchCount > maxShown {
			status += fmt.Sprintf(" (first %s shown)", formatNumber(maxShown))
		}
		status += pageInfo
		return "  " + inputView + theme.Dim().MarginLeft(2).Render(status) + "\n"
	}
	if filterText != "" {
		status := fmt.Sprintf("filter: %s — %s/%s %s", filterText, formatNumber(matchCount), formatNumber(totalCount), noun)
		if maxShown > 0 && matchCount > maxShown {
			status += fmt.Sprintf(" (first %s shown)", formatNumber(maxShown))
		}
		status += pageInfo
		return theme.Dim().MarginLeft(2).Render(status) + "\n"
	}
	return theme.Dim().MarginLeft(2).Render(fmt.Sprintf("%s %s%s", formatNumber(totalCount), noun, pageInfo)) + "\n"
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

// getDetailData returns cached detail data for a skill or agent, populating the cache on first access.
func (m listTUIModel) getDetailData(e skillEntry) *detailData {
	key := e.RelPath
	if d, ok := m.detailCache[key]; ok {
		return d
	}

	if e.Kind == "agent" {
		// Agents are single .md files — read frontmatter from the file directly
		agentFile := filepath.Join(m.agentsSourcePath, e.RelPath)
		fm := utils.ParseFrontmatterFields(agentFile, []string{"description", "license"})
		d := &detailData{
			Description:   fm["description"],
			License:       fm["license"],
			Files:         []string{filepath.Base(e.RelPath)},
			SyncedTargets: m.findSyncedTargets(e),
		}
		m.detailCache[key] = d
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
	b.WriteString(theme.Title().Render(title))
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
	return style.Render(theme.Title().Render(title) + "\n" + body)
}

func renderDetailSection(title string, body string, width int) string {
	style := lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Left).
		Padding(0, 0)
	return style.Render(theme.Title().Render(title) + "\n" + body)
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
		sourceRows = append(sourceRows, renderFactRow("Source", theme.Accent().Render(e.Source)))
	} else if e.RepoName != "" {
		sourceRows = append(sourceRows, renderFactRow("Repo", e.RepoName))
	}
	if e.Branch != "" {
		sourceRows = append(sourceRows, renderFactRow("Branch", theme.Accent().Render(e.Branch)))
	}
	if d.License != "" {
		sourceRows = append(sourceRows, renderFactRow("License", theme.Success().Bold(true).Render(d.License)))
	}
	if len(d.Files) > 0 {
		fileLabel := fmt.Sprintf("Files (%d)", len(d.Files))
		sourceRows = append(sourceRows, renderFactRow(fileLabel, theme.Accent().Render(strings.Join(d.Files, " · "))))
	}
	if len(d.SyncedTargets) > 0 {
		sourceRows = append(sourceRows, renderFactRow("Synced to", theme.Accent().Render(strings.Join(d.SyncedTargets, ", "))))
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
		metaParts = append(metaParts, theme.Dim().Render(e.InstalledAt))
	}
	if len(d.SyncedTargets) > 0 {
		metaParts = append(metaParts, theme.Accent().Render(fmt.Sprintf("%d target(s)", len(d.SyncedTargets))))
	}
	body.WriteString("\n\n")
	body.WriteString(strings.Join(metaParts, theme.Dim().Render("  ·  ")))

	return renderDetailCard("", body.String(), width)
}

func renderDetailParagraph(lines []string) []string {
	rendered := make([]string, 0, len(lines))
	for _, line := range lines {
		rendered = append(rendered, lipgloss.NewStyle().Render(line))
	}
	return rendered
}

func detailStatusBits(e skillEntry) string {
	var bits []string

	// Kind label (Agent / Skill)
	if e.Kind == "agent" {
		bits = append(bits, theme.Accent().Render("Agent"))
	} else {
		bits = append(bits, theme.Accent().Render("Skill"))
	}

	switch {
	case e.RepoName != "":
		bits = append(bits, theme.Success().Render("tracked"))
	case e.Source != "":
		bits = append(bits, theme.Warning().Render("remote"))
	default:
		bits = append(bits, theme.Dim().Render("local"))
	}
	if e.Disabled {
		bits = append(bits, theme.Danger().Render("disabled"))
	}
	return strings.Join(bits, "  ")
}

func renderFactRow(label, value string) string {
	labelStyle := lipgloss.NewStyle().Faint(true).Width(12)
	return labelStyle.Render(label+":") + " " + lipgloss.NewStyle().Render(value)
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
		linkPath := filepath.Join(tc.SkillsConfig().Path, flatName)
		if utils.IsSymlinkOrJunction(linkPath) {
			synced = append(synced, name)
		}
	}
	sort.Strings(synced)
	return synced
}

// runListTUI starts the bubbletea TUI for the skill list.
// When loadFn is non-nil, data is loaded asynchronously inside the TUI (no blank screen).
// Returns (action, skillName, skillKind, error). action is "" on normal quit (q/ctrl+c).
func runListTUI(loadFn listLoadFn, modeLabel, sourcePath, agentsSourcePath string, targets map[string]config.TargetConfig, initialKind resourceKindFilter) (string, string, string, error) {
	model := newListTUIModel(loadFn, nil, 0, modeLabel, sourcePath, agentsSourcePath, targets, initialKind)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	finalModel, err := p.Run()
	if err != nil {
		return "", "", "", err
	}

	m, ok := finalModel.(listTUIModel)
	if !ok || m.action == "" {
		if m.loadErr != nil {
			return "", "", "", m.loadErr
		}
		if m.emptyResult {
			return "empty", "", "", nil
		}
		return "", "", "", nil
	}

	// Extract skill name and kind from selected item
	var skillName string
	var skillKind string
	if item, ok := m.list.SelectedItem().(skillItem); ok {
		if item.entry.RelPath != "" {
			skillName = item.entry.RelPath
		} else {
			skillName = item.entry.Name
		}
		skillKind = item.entry.Kind
	}
	return m.action, skillName, skillKind, nil
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
