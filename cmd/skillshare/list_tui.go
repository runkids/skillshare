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
	listTabSkills listTab = iota // show skills only
	listTabAgents                // show agents only
)

// listStatusFilter pre-filters items by enabled/disabled state. It is set
// by `list --status`; inside the TUI, type status: in the filter instead.
type listStatusFilter int

const (
	statusFilterAll      listStatusFilter = iota // show enabled + disabled
	statusFilterEnabled                          // show only enabled
	statusFilterDisabled                         // show only disabled
)

// String returns the CLI representation of the status filter.
func (s listStatusFilter) String() string {
	switch s {
	case statusFilterEnabled:
		return "enabled"
	case statusFilterDisabled:
		return "disabled"
	default:
		return "all"
	}
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
	// ModelInvocationOff mirrors disable-model-invocation: the skill stays installed and
	// user-invocable, but the model no longer loads it on its own.
	ModelInvocationOff bool
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

	// Tab filter — pre-filters allItems by kind (Skills / Agents)
	activeTab   listTab     // currently selected tab
	tabPinned   bool        // the tab came from a kind flag; never switch it on load
	tabCounts   [2]int      // cached counts: [skills, agents]
	tabFiltered []skillItem // cached result of tab + status filter; set by applyFilter()

	// Status filter — pre-filters by enabled/disabled state (from list --status)
	statusFilter listStatusFilter

	showKeys bool // ? swaps the detail panel for the full key list

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
	confirmAction string // "audit", "update", "uninstall", or confirmModelInvocation
	confirmNote   string // consequence shown above the confirmation line
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
	initTab := listTabSkills
	if initialKind == kindAgents {
		initTab = listTabAgents
	}
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
	l.SetShowTitle(false)                              // the frame title line replaces it
	l.Styles.NoItems = l.Styles.NoItems.PaddingLeft(3) // align with the rows
	l.SetShowStatusBar(false)                          // the frame title shows the counts
	l.SetFilteringEnabled(false)                       // application-level filter replaces built-in
	l.SetShowHelp(false)                               // we render our own help
	l.SetShowPagination(false)                         // we render page info in our status line

	// Loading spinner
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = theme.Accent()

	// Filter text input
	fi := newTUIFilterInput("type to match, or t: g: r: status:")

	m := listTUIModel{
		list:             l,
		totalCount:       totalCount,
		modeLabel:        modeLabel,
		sourcePath:       sourcePath,
		agentsSourcePath: agentsSourcePath,
		targets:          targets,
		activeTab:        initTab,
		tabPinned:        initialKind != kindAll,
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
	return m
}

// recomputeTabCounts updates the cached per-tab counts from allItems. Without
// a kind flag, an install with only agents opens on the Agents tab.
func (m *listTUIModel) recomputeTabCounts() {
	var skills, agents int
	for _, item := range m.allItems {
		if item.entry.Kind == "agent" {
			agents++
		} else {
			skills++
		}
	}
	m.tabCounts = [2]int{skills, agents}
	if !m.tabPinned && skills == 0 && agents > 0 {
		m.activeTab = listTabAgents
	}
}

// tabFilteredItems returns the subset of allItems matching the active tab.
func (m *listTUIModel) tabFilteredItems() []skillItem {
	wantAgent := m.activeTab == listTabAgents
	filtered := make([]skillItem, 0, m.tabCounts[m.activeTab])
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

// noun returns the display noun for the tab, singular for one item.
func (t listTab) noun(n int) string {
	noun := "skill"
	if t == listTabAgents {
		noun = "agent"
	}
	if n != 1 {
		noun += "s"
	}
	return noun
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
				if m.confirmAction == confirmModelInvocation {
					m.clearConfirm()
					return m.toggleModelInvocation()
				}
				return m.quitWithAction(m.confirmAction)
			case "n", "N", "esc", "q":
				m.clearConfirm()
				return m, nil
			}
			return m, nil
		}

		// --- Filter mode: route keys to filterInput ---
		if m.filtering {
			cmd := handleTUIFilterKey(msg, &m.filtering, &m.filterText, &m.filterInput, m.applyFilter)
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
			case m.filterText != "":
				m.filterText = ""
				m.filterInput.SetValue("")
				m.applyFilter()
				skipGroupItem(&m.list, 1)
			default:
				m.quitting = true
				return m, tea.Quit
			}
			return m, nil
		case "?":
			m.showKeys = !m.showKeys
			return m, nil
		case "tab", "shift+tab":
			m.activeTab = 1 - m.activeTab
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
		case "enter":
			if item, ok := m.list.SelectedItem().(skillItem); ok {
				loadContentForSkill(&m, item.entry)
				m.showContent = true
			}
			return m, nil
		case "!":
			return m.enterConfirm("audit")
		case "u":
			return m.enterConfirm("update")
		case "d":
			return m.enterConfirm("uninstall")
		case "t":
			return m.toggleDisabled()
		case "m":
			return m.pressModelInvocation()
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
const (
	modelInvocationKey     = "disable-model-invocation"
	confirmModelInvocation = "model-invocation"
)

func (m *listTUIModel) clearConfirm() {
	m.confirming = false
	m.confirmAction = ""
	m.confirmSkill = ""
	m.confirmKind = ""
	m.confirmNote = ""
}

// pressModelInvocation handles m. Unlike t, which only touches the ignore file beside the
// skill, this edits SKILL.md itself — so when upstream owns that file it says what that costs
// before writing. Turning the flag back off restores the file and needs no warning.
func (m listTUIModel) pressModelInvocation() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(skillItem)
	if !ok || item.entry.Kind == "agent" { // agent harnesses do not read this key
		return m, nil
	}
	e := item.entry
	if m.getDetailData(e).ModelInvocationOff || (e.RepoName == "" && e.Source == "") {
		return m.toggleModelInvocation()
	}

	m.confirming = true
	m.confirmAction = confirmModelInvocation
	m.confirmSkill = e.RelPath
	m.confirmKind = e.Kind
	if e.RepoName != "" {
		m.confirmNote = "This edits SKILL.md in a tracked repo. 'skillshare update' skips the repo until you press m again."
	} else {
		m.confirmNote = "This edits an installed SKILL.md. The next 'skillshare update' reinstalls the skill and drops the change."
	}
	return m, nil
}

// toggleModelInvocation flips disable-model-invocation in the selected skill's SKILL.md.
func (m listTUIModel) toggleModelInvocation() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(skillItem)
	if !ok {
		return m, nil
	}
	skillMD := filepath.Join(m.sourcePath, item.entry.RelPath, "SKILL.md")
	if _, err := utils.ToggleFrontmatterFlag(skillMD, modelInvocationKey); err != nil {
		return m, nil
	}
	delete(m.detailCache, item.entry.RelPath) // the chip re-reads the file
	return m, nil
}

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
		return fmt.Sprintf("\n  %s Loading %s...\n", m.loadSpinner.View(), m.activeTab.noun(2))
	}

	// Content viewer — dual-pane
	if m.showContent {
		return renderContentOverlay(m)
	}

	if listSplitActive(m.termWidth) {
		return m.viewSplit()
	}
	return m.viewVertical()
}

// renderTitleLine renders the frame title: scope, counts and the tabs.
func (m listTUIModel) renderTitleLine() string {
	shown := len(m.tabFiltered)
	count := fmt.Sprintf("%s %s", formatNumber(shown), m.activeTab.noun(shown))
	if m.filterText != "" {
		count = fmt.Sprintf("%s of %s", formatNumber(m.matchCount), count)
	}
	facts := []string{m.modeLabel, count}
	if m.statusFilter != statusFilterAll {
		facts = append(facts, m.statusFilter.String()+" only")
	}
	if m.matchCount > maxListItems {
		facts = append(facts, "first "+formatNumber(maxListItems)+" shown")
	}
	tabs := []frameTab{
		{fmt.Sprintf("Skills %d", m.tabCounts[listTabSkills]), m.activeTab == listTabSkills},
		{fmt.Sprintf("Agents %d", m.tabCounts[listTabAgents]), m.activeTab == listTabAgents},
	}
	return renderFrameTitle(m.termWidth, "list", facts, tabs)
}

// renderBottom renders the note line and the key line. The filter input and
// confirmations take over the key line in place.
func (m listTUIModel) renderBottom() string {
	note := ""
	var line string
	switch {
	case m.confirming:
		note = theme.Dim().Render("  " + m.confirmNote)
		line = m.renderConfirm()
	case m.filtering:
		line = renderFilterLine(m.termWidth, m.filterInput.View(), m.matchCount)
	default:
		other := "agents"
		if m.activeTab == listTabAgents {
			other = "skills"
		}
		filter := keyHint{"/", "filter"}
		if m.filterText != "" {
			filter = keyHint{"esc", "clear filter"}
		}
		hints := []keyHint{{"↑↓", "move"}, filter, {"tab", other}, {"enter", "open"}, {"u", "update"}, {"d", "uninstall"}, {"?", "keys"}}
		if m.showKeys {
			hints = []keyHint{{"?/esc", "close"}}
		}
		line = renderKeyLine(m.termWidth, hints, m.position())
	}
	return note + "\n" + line
}

// renderConfirm asks about the pending action on the key line, with the
// command it will run at the end.
func (m listTUIModel) renderConfirm() string {
	if m.confirmAction == confirmModelInvocation {
		return renderConfirmLine(m.termWidth, "Make "+m.confirmSkill+" manual only?", false, "")
	}
	flag := "-g"
	if m.modeLabel == "project" {
		flag = "-p"
	}
	kindArg := ""
	if m.confirmKind == "agent" {
		kindArg = "agents "
	}
	cmd := fmt.Sprintf("skillshare %s %s%s %s", m.confirmAction, kindArg, flag, m.confirmSkill)
	verb := strings.ToUpper(m.confirmAction[:1]) + m.confirmAction[1:]
	return renderConfirmLine(m.termWidth, verb+" "+m.confirmSkill+"?", m.confirmAction == "uninstall", cmd)
}

// position renders "3/8": the selected row among the matches.
func (m listTUIModel) position() string {
	items := m.list.Items()
	n := 0
	for i := 0; i <= m.list.Index() && i < len(items); i++ {
		if _, ok := items[i].(skillItem); ok {
			n++
		}
	}
	return framePosition(n, m.matchCount)
}

// listKeyGroups lists every key for the ? panel.
func (m listTUIModel) listKeyGroups() []keyGroup {
	other := "agents"
	open := "open SKILL.md"
	if m.activeTab == listTabAgents {
		other, open = "skills", "open the agent file"
	}
	actions := []keyHint{{"u", "update"}, {"d", "uninstall"}, {"t", "enable / disable"}}
	if m.activeTab == listTabSkills {
		actions = append(actions, keyHint{"m", "manual only (the model stops loading it)"})
	}
	actions = append(actions, keyHint{"!", "audit"})
	return []keyGroup{
		{"Move", []keyHint{
			{"↑↓", "move"},
			{"←→", "page"},
			{"/", "filter   type to match, or t: g: r: status:"},
			{"tab", "switch to " + other},
			{"enter", open},
			{"ctrl+d/u", "scroll the details"},
			{"esc", "clear the filter, then quit"},
			{"q", "quit"},
		}},
		{"Actions", actions},
	}
}

func (m *listTUIModel) syncListSize() {
	bodyHeight := max(m.termHeight-frameChrome, 6)
	if listSplitActive(m.termWidth) {
		m.list.SetSize(listPanelWidth(m.termWidth), bodyHeight)
		return
	}
	m.list.SetSize(m.termWidth, max(bodyHeight/2, 4))
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

// listDetailPanelWidth is the room left for the detail panel, including the
// gap that renderFrameSplit puts before it.
func listDetailPanelWidth(termWidth int) int {
	width := termWidth - listPanelWidth(termWidth)
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
	bodyHeight := max(m.termHeight-frameChrome, 6)
	leftWidth := listPanelWidth(m.termWidth)
	rightWidth := listDetailPanelWidth(m.termWidth)
	right := m.renderRight(rightWidth-2, bodyHeight)
	return m.renderTitleLine() + "\n\n" +
		renderFrameSplit(m.list.View(), right, leftWidth, rightWidth, bodyHeight) + "\n" +
		m.renderBottom()
}

// renderRight renders the detail panel, or the key list while ? is on. The
// name stays put while the rest scrolls.
func (m listTUIModel) renderRight(width, height int) string {
	if m.showKeys {
		return renderKeysPanel(m.listKeyGroups())
	}
	item, ok := m.list.SelectedItem().(skillItem)
	if !ok {
		return ""
	}
	d := m.getDetailData(item.entry)
	header := renderDetailHeader(item.entry, d, width)
	body, _ := wrapAndScroll(m.renderDetailBody(item.entry, d, width), width, m.detailScroll, max(height-lipgloss.Height(header)-1, 4))
	return header + "\n\n" + body
}

func (m listTUIModel) viewVertical() string {
	bodyHeight := max(m.termHeight-frameChrome, 6)
	listHeight := max(bodyHeight/2, 4)
	detailHeight := max(bodyHeight-listHeight-1, 4)
	detail := lipgloss.NewStyle().Height(detailHeight).MaxHeight(detailHeight).
		Render(m.renderRight(m.termWidth-2, detailHeight))
	return m.renderTitleLine() + "\n\n" + m.list.View() + "\n\n" +
		lipgloss.NewStyle().PaddingLeft(1).Render(detail) + "\n" + m.renderBottom()
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
	fm := utils.ParseFrontmatterFields(skillMD, []string{"description", "license", modelInvocationKey})

	d := &detailData{
		Description:        fm["description"],
		License:            fm["license"],
		Files:              listSkillFiles(skillDir),
		SyncedTargets:      m.findSyncedTargets(e),
		ModelInvocationOff: strings.EqualFold(fm[modelInvocationKey], "true"),
	}
	m.detailCache[key] = d
	return d
}

// renderDetailBody renders the scrollable part of the detail panel: the
// description, the facts and the targets the skill is synced to.
func (m listTUIModel) renderDetailBody(e skillEntry, d *detailData, width int) string {
	var blocks []string
	if d.Description != "" {
		blocks = append(blocks, strings.Join(wordWrapLines(d.Description, max(width, 20)), "\n"))
	}

	var facts []string
	fact := func(label, value string) {
		facts = append(facts, theme.Dim().Render(fmt.Sprintf("%-9s ", label))+value)
	}
	switch {
	case e.Source != "":
		fact("Source", e.Source)
	case e.RepoName != "":
		repo := e.RepoName
		if e.Branch != "" {
			repo += theme.Dim().Render(" · ") + e.Branch
		}
		fact("Repo", repo)
	default:
		fact("Source", "local")
	}
	if e.InstalledAt != "" {
		fact("Installed", e.InstalledAt)
	}
	if d.License != "" {
		fact("License", d.License)
	}
	dir := m.sourcePath
	if e.Kind == "agent" {
		dir = m.agentsSourcePath
	}
	fact("Path", shortenPath(filepath.Join(dir, e.RelPath)))
	if len(d.Files) > 0 {
		fact("Files", strings.Join(d.Files, theme.Dim().Render(" · ")))
	}
	blocks = append(blocks, strings.Join(facts, "\n"))

	if len(d.SyncedTargets) > 0 {
		targets := make([]string, len(d.SyncedTargets))
		for i, t := range d.SyncedTargets {
			targets[i] = theme.Success().Render("✓") + " " + t
		}
		blocks = append(blocks, theme.Primary().Bold(true).Render("Targets")+"\n"+strings.Join(targets, "   "))
	}
	return strings.Join(blocks, "\n\n")
}

// renderDetailHeader renders the fixed top of the detail panel: the path,
// and what is switched off, if anything.
func renderDetailHeader(e skillEntry, d *detailData, width int) string {
	header := colorSkillPathBold(baseSkillPath(e))
	var off []string
	if e.Disabled {
		off = append(off, "disabled")
	}
	if d.ModelInvocationOff {
		off = append(off, "manual only")
	}
	if len(off) > 0 {
		header += "\n" + theme.Warning().Render(strings.Join(off, " · "))
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(header)
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
// initialStatus comes from list --status and stays for the whole session.
// Returns (action, skillName, skillKind, error). action is "" on normal quit (q/ctrl+c).
func runListTUI(loadFn listLoadFn, modeLabel, sourcePath, agentsSourcePath string, targets map[string]config.TargetConfig, initialKind resourceKindFilter, initialStatus listStatusFilter) (string, string, string, error) {
	model := newListTUIModel(loadFn, nil, 0, modeLabel, sourcePath, agentsSourcePath, targets, initialKind)
	model.statusFilter = initialStatus
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
