package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/theme"
	"skillshare/internal/ui"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── Messages ────────────────────────────────────────────────────────

type extrasLoadedMsg struct {
	items []extrasListEntry
	err   error
}

type extrasActionDoneMsg struct {
	msg string
	err error
}

// ─── Model ───────────────────────────────────────────────────────────

type extrasListTUIModel struct {
	list       list.Model
	allItems   []extrasListEntry
	modeLabel  string
	quitting   bool
	wantsNew   bool
	termWidth  int
	termHeight int

	// Config context
	cfg        *config.Config
	projCfg    *config.ProjectConfig
	cwd        string
	configPath string
	sourceFunc func(extra config.ExtraConfig) string

	// Async loading
	loading     bool
	loadSpinner spinner.Model
	loadFn      func() ([]extrasListEntry, error)
	loadErr     error
	emptyResult bool

	// Filter
	filterText  string
	filterInput textinput.Model
	filtering   bool
	matchCount  int

	// Detail panel
	detailScroll int

	// Content viewer
	showContent     bool
	contentScroll   int
	contentText     string
	contentExtraKey string
	treeAllNodes    []treeNode
	treeNodes       []treeNode
	treeCursor      int
	treeScroll      int

	// Confirm overlay
	confirming    bool
	confirmAction string
	confirmExtra  string
	confirmTarget string

	// Target sub-menu
	showTargetMenu  bool
	targetMenuItems []extrasTargetInfo
	targetMenuFile  string // single-file extra: its file, for target display paths
	targetCursor    int
	targetAction    string

	// e menu: the mode and flatten setting of every target
	showEditMenu   bool
	editMenuCursor int

	showKeys bool // ? swaps the detail panel for the full key list

	// Mode picker
	showModePicker   bool
	modePickerTarget string // target path being edited
	modePickerExtra  string
	modeCursor       int

	// Action feedback
	lastActionMsg string
}

func newExtrasListTUIModel(
	loadFn func() ([]extrasListEntry, error),
	modeLabel string,
	cfg *config.Config,
	projCfg *config.ProjectConfig,
	cwd, configPath string,
	sourceFunc func(extra config.ExtraConfig) string,
) extrasListTUIModel {
	delegate := extrasListDelegate{}

	l := list.New(nil, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = theme.Accent()

	fi := newTUIFilterInput("filter by name")

	return extrasListTUIModel{
		list:        l,
		modeLabel:   modeLabel,
		cfg:         cfg,
		projCfg:     projCfg,
		cwd:         cwd,
		configPath:  configPath,
		sourceFunc:  sourceFunc,
		loading:     true,
		loadSpinner: sp,
		loadFn:      loadFn,
		filterInput: fi,
	}
}

// ─── Init ────────────────────────────────────────────────────────────

func (m extrasListTUIModel) Init() tea.Cmd {
	if m.loading && m.loadFn != nil {
		fn := m.loadFn
		return tea.Batch(m.loadSpinner.Tick, func() tea.Msg {
			items, err := fn()
			return extrasLoadedMsg{items: items, err: err}
		})
	}
	return nil
}

// ─── Update ──────────────────────────────────────────────────────────

func (m extrasListTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		m.syncExtrasListSize()
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.loadSpinner, cmd = m.loadSpinner.Update(msg)
			return m, cmd
		}

	case extrasLoadedMsg:
		m.loading = false
		m.loadFn = nil
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
		m.list.SetItems(extrasToListItems(msg.items))
		return m, nil

	case extrasActionDoneMsg:
		if msg.err != nil {
			m.lastActionMsg = "✗ " + msg.err.Error()
		} else {
			m.lastActionMsg = msg.msg
		}
		m.reloadExtras()
		return m, nil

	case tea.MouseMsg:
		if m.showContent && !m.loading {
			return m.handleExtrasContentMouse(msg)
		}
		if extrasSplitActive(m.termWidth) && !m.loading {
			leftWidth := extrasPanelWidth(m.termWidth)
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
		if m.loading {
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				m.quitting = true
				return m, tea.Quit
			}
			return m, nil
		}

		if m.showContent {
			return m.handleExtrasContentKey(msg)
		}

		if m.confirming {
			return m.handleConfirmKey(msg)
		}

		if m.showTargetMenu {
			return m.handleTargetMenuKey(msg)
		}

		if m.showModePicker {
			return m.handleModePickerKey(msg)
		}

		if m.showEditMenu {
			return m.handleEditMenuKey(msg)
		}

		if m.filtering {
			cmd := handleTUIFilterKey(msg, &m.filtering, &m.filterText, &m.filterInput, m.applyExtrasFilter)
			return m, cmd
		}

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
				m.applyExtrasFilter()
			default:
				m.quitting = true
				return m, tea.Quit
			}
			return m, nil
		case "?":
			m.showKeys = !m.showKeys
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
			m.lastActionMsg = ""
			return m, textinput.Blink
		case "enter":
			if item, ok := m.list.SelectedItem().(extraTUIItem); ok {
				m.loadExtrasContent(item.entry)
				m.showContent = true
			}
			return m, nil
		case "n":
			m.wantsNew = true
			return m, tea.Quit
		case "d":
			return m.enterExtrasConfirm("remove")
		case "s":
			return m.enterTargetMenu("sync")
		case "c":
			return m.enterTargetMenu("collect")
		case "e":
			if item, ok := m.list.SelectedItem().(extraTUIItem); ok {
				if len(item.entry.Targets) == 0 {
					m.lastActionMsg = "✗ No targets configured"
					return m, nil
				}
				m.showEditMenu = true
				m.showKeys = false
				m.editMenuCursor = 0
				m.lastActionMsg = ""
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	prevSelected := extrasSelectedKey(m.list.SelectedItem())
	m.list, cmd = m.list.Update(msg)
	if extrasSelectedKey(m.list.SelectedItem()) != prevSelected {
		m.detailScroll = 0
		m.lastActionMsg = ""
	}
	return m, cmd
}

// ─── View ────────────────────────────────────────────────────────────

func (m extrasListTUIModel) View() string {
	if m.quitting {
		return ""
	}
	if m.loading {
		return fmt.Sprintf("\n  %s Loading extras...\n", m.loadSpinner.View())
	}
	if m.showContent {
		return m.renderExtrasContentOverlay()
	}
	bodyHeight := max(m.termHeight-frameChrome, 6)
	count := countNoun(len(m.allItems), "extra")
	if m.filterText != "" {
		count = formatNumber(m.matchCount) + " of " + count
	}
	title := renderFrameTitle(m.termWidth, "extras", []string{m.modeLabel, count}, nil)
	if !extrasSplitActive(m.termWidth) {
		detailHeight := max(bodyHeight-m.list.Height()-1, 4)
		detail := lipgloss.NewStyle().Height(detailHeight).MaxHeight(detailHeight).PaddingLeft(1).
			Render(m.renderRight(m.termWidth-2, detailHeight))
		return title + "\n\n" + m.list.View() + "\n\n" + detail + "\n" + m.renderBottom()
	}
	leftWidth := extrasPanelWidth(m.termWidth)
	rightWidth := m.termWidth - leftWidth
	return title + "\n\n" +
		renderFrameSplit(m.list.View(), m.renderRight(rightWidth-2, bodyHeight), leftWidth, rightWidth, bodyHeight) + "\n" +
		m.renderBottom()
}

// renderRight renders the detail panel, or in its place the key list, the
// e menu, the target menu or the mode picker.
func (m extrasListTUIModel) renderRight(width, height int) string {
	item, ok := m.list.SelectedItem().(extraTUIItem)
	switch {
	case m.showKeys:
		return renderKeysPanel(extrasKeyGroups)
	case !ok:
		return ""
	case m.showEditMenu:
		return m.renderEditMenu(item.entry)
	case m.showTargetMenu:
		return m.renderTargetMenu()
	case m.showModePicker:
		return m.renderModePicker()
	}
	detail, _ := wrapAndScroll(m.renderExtrasDetail(item.entry), width, m.detailScroll, height)
	return detail
}

// renderBottom renders the last action's result on the note line and the
// key line. Confirmations, the filter input and the open menu's keys take
// over the key line in place.
func (m extrasListTUIModel) renderBottom() string {
	note := ""
	if m.lastActionMsg != "" {
		note = "  " + renderExtrasActionMsg(m.lastActionMsg)
	}
	var line string
	switch {
	case m.confirming:
		question, what, danger := m.confirmText()
		note = theme.Dim().Render("  " + what)
		line = renderConfirmLine(m.termWidth, question, danger, "")
	case m.filtering:
		line = renderFilterLine(m.termWidth, m.filterInput.View(), m.matchCount)
	case m.showEditMenu || m.showTargetMenu || m.showModePicker:
		line = renderKeyLine(m.termWidth, []keyHint{{"↑↓", "move"}, {"enter", "choose"}, {"esc", "back"}}, "")
	case m.showKeys:
		line = renderKeyLine(m.termWidth, []keyHint{{"?/esc", "close"}}, "")
	default:
		filter := keyHint{"/", "filter"}
		if m.filterText != "" {
			filter = keyHint{"esc", "clear filter"}
		}
		hints := []keyHint{{"↑↓", "move"}, filter, {"enter", "files"}, {"s", "sync"}, {"c", "collect"}, {"e", "edit"}, {"?", "keys"}}
		line = renderKeyLine(m.termWidth, hints, framePosition(m.list.Index()+1, m.matchCount))
	}
	return note + "\n" + line
}

// extrasKeyGroups lists every key for the ? panel.
var extrasKeyGroups = []keyGroup{
	{"Move", []keyHint{
		{"↑↓", "move"},
		{"/", "filter by name"},
		{"enter", "browse the files"},
		{"ctrl+d/u", "scroll the details"},
		{"esc", "clear the filter, then quit"},
		{"q", "quit"},
	}},
	{"Extras", []keyHint{
		{"n", "new extra"},
		{"d", "remove the extra"},
		{"s", "sync to its targets"},
		{"c", "collect from a target"},
		{"e", "change a target's mode or flatten"},
	}},
}

// ─── Layout ──────────────────────────────────────────────────────────

func extrasSplitActive(termWidth int) bool {
	return termWidth >= tuiMinSplitWidth
}

func extrasPanelWidth(termWidth int) int {
	width := termWidth * 28 / 100
	return max(min(width, 36), 22)
}

func (m *extrasListTUIModel) syncExtrasListSize() {
	bodyHeight := max(m.termHeight-frameChrome, 6)
	if extrasSplitActive(m.termWidth) {
		m.list.SetSize(extrasPanelWidth(m.termWidth), bodyHeight)
		return
	}
	m.list.SetSize(m.termWidth, max(bodyHeight/2, 4))
}

// ─── Detail Panel ────────────────────────────────────────────────────

// extrasTargetDisplayPath returns the path shown for a target: the directory,
// or for a single-file extra (file set) the file it writes.
func extrasTargetDisplayPath(file string, t extrasTargetInfo) string {
	if file == "" {
		return t.Path
	}
	return singleFileTargetPath(config.ExtraConfig{File: file}, config.ExtraTargetConfig{Path: t.Path, As: t.As})
}

func (m extrasListTUIModel) renderExtrasDetail(e extrasListEntry) string {
	var b strings.Builder

	b.WriteString(theme.Primary().Bold(true).Render(e.Name))
	b.WriteString("\n\n")

	label := theme.Dim().Width(8).Render("Source")
	if e.File != "" {
		b.WriteString(label + shortenPath(filepath.Join(e.SourceDir, e.File)))
		if !e.SourceExists {
			b.WriteString(theme.Dim().Render(" · not found"))
		}
		b.WriteString("\n")
	} else if e.SourceExists {
		b.WriteString(label + shortenPath(e.SourceDir) + "\n")
	} else {
		b.WriteString(label + theme.Dim().Render("not found") + "\n")
	}

	label = theme.Dim().Width(8).Render("Files")
	if e.SourceExists {
		b.WriteString(label + fmt.Sprintf("%d", e.FileCount) + "\n")
	} else {
		b.WriteString(label + theme.Dim().Render("—") + "\n")
	}

	b.WriteString("\n")
	b.WriteString(theme.Primary().Bold(true).Render("Targets"))
	b.WriteString("\n")

	if len(e.Targets) == 0 {
		b.WriteString(theme.Dim().Render("  No targets configured") + "\n")
	} else {
		hasDrift := false
		for _, t := range e.Targets {
			var icon string
			var style lipgloss.Style
			switch t.Status {
			case "synced":
				icon = "✓"
				style = theme.Success()
			case "drift", "modified", "invalid mode":
				icon = "△"
				style = theme.Warning()
				hasDrift = true
			case "not synced":
				icon = "✗"
				style = theme.Danger()
				hasDrift = true
			default:
				icon = "-"
				style = theme.Dim()
			}
			statusText := ""
			if t.Status != "synced" {
				statusText = "  " + t.Status
			}
			modeLabel := t.Mode
			if t.Extension != "" {
				modeLabel = "extension: " + t.Extension
			} else if t.Flatten {
				modeLabel += ", flatten"
			}
			fmt.Fprintf(&b, "  %s %s (%s)%s\n",
				style.Render(icon), shortenPath(extrasTargetDisplayPath(e.File, t)), modeLabel, theme.Dim().Render(statusText))
		}
		if hasDrift {
			b.WriteString("\n" + theme.Warning().Render("hint:") + " press s to sync, or use --force to overwrite conflicts\n")
		}
	}

	if e.SourceExists && e.FileCount > 0 {
		b.WriteString("\n")
		b.WriteString(theme.Primary().Bold(true).Render("Files"))
		b.WriteString("\n")
		files := discoverExtraFileNames(e.SourceDir, e.File)
		maxShow := 10
		for i, f := range files {
			if i >= maxShow {
				b.WriteString(theme.Dim().Render(fmt.Sprintf("  … and %d more", len(files)-maxShow)) + "\n")
				break
			}
			prefix := "├── "
			if i == len(files)-1 || i == maxShow-1 {
				prefix = "└── "
			}
			b.WriteString(theme.Dim().Render("  "+prefix) + f + "\n")
		}
	}

	return b.String()
}

func discoverExtraFileNames(sourceDir, file string) []string {
	files, err := sync.DiscoverExtraSource(sourceDir, file)
	if err != nil {
		return nil
	}
	return files
}

// ─── Filter ──────────────────────────────────────────────────────────

func renderExtrasActionMsg(msg string) string {
	if strings.HasPrefix(msg, "✓") {
		return theme.Success().Render(msg)
	}
	if strings.HasPrefix(msg, "✗") {
		return theme.Danger().Render(msg)
	}
	return theme.Warning().Render(msg)
}

// ─── Helpers ─────────────────────────────────────────────────────────

func extrasToListItems(entries []extrasListEntry) []list.Item {
	items := make([]list.Item, len(entries))
	for i, e := range entries {
		items[i] = extraTUIItem{entry: e}
	}
	return items
}

func extrasSelectedKey(item list.Item) string {
	extra, ok := item.(extraTUIItem)
	if !ok {
		return ""
	}
	return extra.entry.Name
}

func (m *extrasListTUIModel) applyExtrasFilter() {
	m.detailScroll = 0

	if m.filterText == "" {
		m.matchCount = len(m.allItems)
		m.list.SetItems(extrasToListItems(m.allItems))
		m.list.ResetSelected()
		return
	}

	q := strings.ToLower(m.filterText)
	var matched []list.Item
	for _, item := range m.allItems {
		if strings.Contains(strings.ToLower(item.Name), q) {
			matched = append(matched, extraTUIItem{entry: item})
		}
	}
	m.matchCount = len(matched)
	m.list.SetItems(matched)
	m.list.ResetSelected()
}

func (m *extrasListTUIModel) reloadExtras() {
	var extras []config.ExtraConfig
	if m.projCfg != nil {
		projCfg, err := config.LoadProject(m.cwd)
		if err == nil {
			m.projCfg = projCfg
			extras = projCfg.Extras
		}
	} else if m.cfg != nil {
		cfg, err := config.Load()
		if err == nil {
			m.cfg = cfg
			extras = cfg.Extras
		}
	}

	extrasSource := ""
	if m.cfg != nil {
		extrasSource = m.cfg.EffectiveExtrasSource()
	}
	extensionsDir := globalExtensionsDir()
	if m.projCfg != nil {
		extensionsDir = projectExtensionsDir(m.cwd)
	}
	root := ""
	if m.projCfg != nil {
		root = m.cwd
	}
	entries := buildExtrasListEntries(extras, extrasSource, extensionsDir, m.sourceFunc, root)
	m.allItems = entries
	m.applyExtrasFilter()
}

// ─── Confirm Overlay ─────────────────────────────────────────────────

func (m extrasListTUIModel) enterExtrasConfirm(action string) (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(extraTUIItem)
	if !ok {
		return m, nil
	}
	m.confirming = true
	m.confirmAction = action
	m.confirmExtra = item.entry.Name
	m.lastActionMsg = ""
	return m, nil
}

func (m extrasListTUIModel) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.confirming = false
		return m, m.executeAction()
	case "n", "N", "esc", "q":
		m.confirming = false
		m.confirmAction = ""
		m.confirmExtra = ""
		m.confirmTarget = ""
		return m, nil
	}
	return m, nil
}

// confirmTargetLabel names the confirmed target: "all targets", its directory,
// or for a single-file extra the file it writes. confirmTarget itself stays the
// configured path, which executeAction matches targets by.
func (m extrasListTUIModel) confirmTargetLabel(entry extrasListEntry) string {
	if m.confirmTarget == "" {
		return "all targets"
	}
	for _, t := range entry.Targets {
		if t.Path == m.confirmTarget {
			return shortenPath(extrasTargetDisplayPath(entry.File, t))
		}
	}
	return shortenPath(m.confirmTarget)
}

// confirmText returns the question for the key line, what the action does
// for the note line, and whether it is destructive.
func (m extrasListTUIModel) confirmText() (question, what string, danger bool) {
	var entry extrasListEntry
	if item, ok := m.list.SelectedItem().(extraTUIItem); ok {
		entry = item.entry
	}
	switch m.confirmAction {
	case "remove":
		if entry.File != "" {
			return "Remove extra " + m.confirmExtra + "?", "Its target files are restored to what was there before", true
		}
		return "Remove extra " + m.confirmExtra + "?", "Only removes it from the config; run sync to clean up the links it left", true
	case "collect":
		return "Collect into " + m.confirmExtra + "?", "Copies new files from " + m.confirmTargetLabel(entry) + " into the source", false
	}
	return "Sync " + m.confirmExtra + "?", "Writes the source to " + m.confirmTargetLabel(entry), false
}

// ─── Target Sub-Menu ─────────────────────────────────────────────────

func (m extrasListTUIModel) enterTargetMenu(action string) (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(extraTUIItem)
	if !ok {
		return m, nil
	}
	if len(item.entry.Targets) == 0 {
		m.lastActionMsg = "✗ No targets configured"
		return m, nil
	}
	// Single target: skip the menu
	if len(item.entry.Targets) == 1 {
		m.confirmExtra = item.entry.Name
		m.confirmAction = action
		m.confirmTarget = item.entry.Targets[0].Path
		m.confirming = true
		m.lastActionMsg = ""
		return m, nil
	}
	m.showTargetMenu = true
	m.targetAction = action
	m.targetMenuItems = item.entry.Targets
	m.targetMenuFile = item.entry.File
	m.targetCursor = 0
	m.lastActionMsg = ""
	return m, nil
}

func (m extrasListTUIModel) handleTargetMenuKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	totalItems := len(m.targetMenuItems) + 1 // "All targets" first

	switch msg.String() {
	case "q", "esc":
		m.showTargetMenu = false
		m.targetMenuItems = nil
		return m, nil
	case "up", "k":
		if m.targetCursor > 0 {
			m.targetCursor--
		}
		return m, nil
	case "down", "j":
		if m.targetCursor < totalItems-1 {
			m.targetCursor++
		}
		return m, nil
	case "enter":
		item, ok := m.list.SelectedItem().(extraTUIItem)
		if !ok {
			m.showTargetMenu = false
			return m, nil
		}
		m.showTargetMenu = false
		m.confirmExtra = item.entry.Name
		m.confirmAction = m.targetAction
		if m.targetCursor == 0 {
			m.confirmTarget = ""
		} else {
			m.confirmTarget = m.targetMenuItems[m.targetCursor-1].Path
		}
		m.confirming = true
		return m, nil
	}
	return m, nil
}

func (m extrasListTUIModel) renderTargetMenu() string {
	var b strings.Builder
	title := "Sync " + m.confirmExtraName() + " to"
	if m.targetAction == "collect" {
		title = "Collect into " + m.confirmExtraName() + " from"
	}
	b.WriteString(theme.Primary().Bold(true).Render(title) + "\n\n")
	b.WriteString(renderPickerRow("All targets", "", m.targetCursor == 0))
	for i, t := range m.targetMenuItems {
		b.WriteString(renderPickerRow(shortenPath(extrasTargetDisplayPath(m.targetMenuFile, t)), t.Mode, m.targetCursor == i+1))
	}
	return b.String()
}

// confirmExtraName is the selected extra's name, for menu titles.
func (m extrasListTUIModel) confirmExtraName() string {
	return extrasSelectedKey(m.list.SelectedItem())
}

// extrasEditOption is one row of the e menu: the mode or flatten setting of
// one target.
type extrasEditOption struct {
	action string // "mode" or "flatten"
	target extrasTargetInfo
}

func extrasEditOptions(e extrasListEntry) []extrasEditOption {
	var options []extrasEditOption
	for _, t := range e.Targets {
		options = append(options, extrasEditOption{"mode", t}, extrasEditOption{"flatten", t})
	}
	return options
}

func (m extrasListTUIModel) handleEditMenuKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(extraTUIItem)
	if !ok {
		m.showEditMenu = false
		return m, nil
	}
	options := extrasEditOptions(item.entry)
	switch msg.String() {
	case "q", "esc":
		m.showEditMenu = false
	case "up", "k":
		m.editMenuCursor = max(m.editMenuCursor-1, 0)
	case "down", "j":
		m.editMenuCursor = min(m.editMenuCursor+1, len(options)-1)
	case "enter":
		m.showEditMenu = false
		option := options[m.editMenuCursor]
		if option.action == "mode" {
			return m.openModePicker(item.entry.Name, option.target)
		}
		return m, m.doFlattenToggle(item.entry.Name, option.target)
	}
	return m, nil
}

// renderEditMenu renders the e menu: per target, its mode and flatten.
func (m extrasListTUIModel) renderEditMenu(e extrasListEntry) string {
	var b strings.Builder
	b.WriteString(theme.Primary().Bold(true).Render("Edit "+e.Name) + "\n")
	for i, option := range extrasEditOptions(e) {
		if option.action == "mode" {
			b.WriteString("\n" + theme.Dim().Render(shortenPath(extrasTargetDisplayPath(e.File, option.target))) + "\n")
		}
		current := sync.EffectiveMode(option.target.Mode)
		if option.action == "flatten" {
			current = "off"
			if option.target.Flatten {
				current = "on"
			}
		}
		b.WriteString(renderPickerRow(fmt.Sprintf("%-8s", option.action), current, i == m.editMenuCursor))
	}
	return b.String()
}

// ─── Mode Picker ─────────────────────────────────────────────────────

var extrasSyncModes = config.ValidSyncModes // directory modes; import is set in config for single-file extras

func (m extrasListTUIModel) openModePicker(extraName string, t extrasTargetInfo) (tea.Model, tea.Cmd) {
	m.showModePicker = true
	m.modePickerExtra = extraName
	m.modePickerTarget = t.Path
	m.modeCursor = 0
	current := sync.EffectiveMode(t.Mode)
	for i, mode := range extrasSyncModes {
		if mode == current {
			m.modeCursor = i
			break
		}
	}
	m.lastActionMsg = ""
	return m, nil
}

func (m extrasListTUIModel) handleModePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.showModePicker = false
		return m, nil
	case "up", "k":
		if m.modeCursor > 0 {
			m.modeCursor--
		}
		return m, nil
	case "down", "j":
		if m.modeCursor < len(extrasSyncModes)-1 {
			m.modeCursor++
		}
		return m, nil
	case "enter":
		m.showModePicker = false
		newMode := extrasSyncModes[m.modeCursor]
		name := m.modePickerExtra
		target := m.modePickerTarget
		return m, func() tea.Msg {
			msg, err := m.doSetMode(name, target, newMode)
			return extrasActionDoneMsg{msg: msg, err: err}
		}
	}
	return m, nil
}

func (m extrasListTUIModel) renderModePicker() string {
	var b strings.Builder
	b.WriteString(theme.Primary().Bold(true).Render("Mode") + theme.Dim().Render(" · "+m.modePickerExtra+" → "+shortenPath(m.modePickerTarget)) + "\n\n")
	for i, mode := range extrasSyncModes {
		desc := map[string]string{"merge": "per-file symlinks", "copy": "file copies", "symlink": "directory symlink"}[mode]
		b.WriteString(renderPickerRow(mode, desc, i == m.modeCursor))
	}
	return b.String()
}

func (m extrasListTUIModel) doSetMode(name, targetPath, newMode string) (string, error) {
	if m.projCfg != nil {
		projCfg, err := config.LoadProject(m.cwd)
		if err != nil {
			return "", err
		}
		if err := applyExtraTarget(projCfg.Extras, name, targetPath, func(t *config.ExtraTargetConfig) { t.Mode = newMode }); err != nil {
			return "", err
		}
		if err := projCfg.Save(m.cwd); err != nil {
			return "", err
		}
	} else {
		cfg, err := config.Load()
		if err != nil {
			return "", err
		}
		if err := applyExtraTarget(cfg.Extras, name, targetPath, func(t *config.ExtraTargetConfig) { t.Mode = newMode }); err != nil {
			return "", err
		}
		if err := cfg.Save(); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("✓ Set %s → %s to %s", name, shortenPath(targetPath), newMode), nil
}

// ─── Action Execution ────────────────────────────────────────────────

func (m extrasListTUIModel) executeAction() tea.Cmd {
	action := m.confirmAction
	name := m.confirmExtra
	target := m.confirmTarget

	return func() tea.Msg {
		var msg string
		var err error

		switch action {
		case "remove":
			msg, err = m.doRemove(name)
		case "sync":
			msg, err = m.doSync(name, target)
		case "collect":
			msg, err = m.doCollect(name, target)
		}

		return extrasActionDoneMsg{msg: msg, err: err}
	}
}

func (m extrasListTUIModel) doRemove(name string) (string, error) {
	var sourceDir string
	var err error

	// Load fresh config to avoid mutating the TUI model's copy in a background goroutine.
	if m.projCfg != nil {
		projCfg, loadErr := config.LoadProject(m.cwd)
		if loadErr != nil {
			return "", loadErr
		}
		sourceDir, err = removeExtraFromProjectConfig(projCfg, m.cwd, name)
	} else {
		cfg, loadErr := config.Load()
		if loadErr != nil {
			return "", loadErr
		}
		sourceDir, err = removeExtraFromGlobalConfig(cfg, name)
	}
	if err != nil {
		return "", err
	}
	cleanEmptyExtrasDirQuiet(sourceDir)
	return fmt.Sprintf("✓ Removed %q", name), nil
}

// resolveExtraWithTargets finds the extra config by name and filters targets.
func (m extrasListTUIModel) resolveExtraWithTargets(name, targetPath string) (*config.ExtraConfig, []config.ExtraTargetConfig, error) {
	var extras []config.ExtraConfig
	if m.projCfg != nil {
		extras = m.projCfg.Extras
	} else {
		extras = m.cfg.Extras
	}

	var extra *config.ExtraConfig
	for i, e := range extras {
		if e.Name == name {
			extra = &extras[i]
			break
		}
	}
	if extra == nil {
		return nil, nil, fmt.Errorf("extra %q not found", name)
	}

	if targetPath == "" {
		return extra, extra.Targets, nil
	}
	for _, t := range extra.Targets {
		if t.Path == targetPath {
			return extra, []config.ExtraTargetConfig{t}, nil
		}
	}
	return extra, extra.Targets, nil
}

func (m extrasListTUIModel) projectRoot() string {
	if m.projCfg != nil {
		return m.cwd
	}
	return ""
}

func (m extrasListTUIModel) doSync(name, targetPath string) (string, error) {
	extra, targets, err := m.resolveExtraWithTargets(name, targetPath)
	if err != nil {
		return "", err
	}

	sourceDir := m.sourceFunc(*extra)
	projectRoot := m.projectRoot()
	synced := 0
	for _, t := range targets {
		mode := sync.EffectiveMode(t.Mode)
		resolved := config.ExpandPath(t.Path)
		// Resolve the per-target transform extension so the TUI sync applies
		// it instead of copying files verbatim.
		var spec *sync.ExtensionSpec
		if t.Extension != "" {
			effMode, modeErr := validateExtensionMode(t.Mode)
			if modeErr != nil {
				return "", fmt.Errorf("sync %s: %w", t.Path, modeErr)
			}
			mode = effMode
			extDir := globalExtensionsDir()
			if projectRoot != "" {
				extDir = projectExtensionsDir(projectRoot)
			}
			var specErr error
			spec, specErr = resolveExtension(t.Extension, extDir)
			if specErr != nil {
				return "", fmt.Errorf("sync %s: %w", t.Path, specErr)
			}
		}
		_, err := sync.SyncExtraTarget(*extra, t, sourceDir, resolved, mode, false, false, projectRoot, spec)
		if err != nil {
			return "", fmt.Errorf("sync %s: %w", t.Path, err)
		}
		synced++
	}
	return fmt.Sprintf("✓ Synced %q to %d target(s)", name, synced), nil
}

func (m extrasListTUIModel) doCollect(name, targetPath string) (string, error) {
	extra, targets, err := m.resolveExtraWithTargets(name, targetPath)
	if err != nil {
		return "", err
	}

	if extra.File != "" {
		return "", errSingleFileCollect
	}
	sourceDir := m.sourceFunc(*extra)
	collected := 0
	for _, t := range targets {
		resolved := config.ExpandPath(t.Path)
		result, err := sync.CollectExtraFiles(sourceDir, resolved, t.Mode, false, false, t.Flatten, m.projectRoot())
		if err != nil {
			return "", fmt.Errorf("collect from %s: %w", t.Path, err)
		}
		collected += result.Collected
	}

	return fmt.Sprintf("✓ Collected %d file(s) into %q", collected, name), nil
}

func (m extrasListTUIModel) doFlattenToggle(name string, t extrasTargetInfo) tea.Cmd {
	targetPath := t.Path
	newFlatten := !t.Flatten

	return func() tea.Msg {
		if err := config.ValidateExtraFlatten(newFlatten, t.Mode); err != nil {
			return extrasActionDoneMsg{msg: "", err: err}
		}

		if m.projCfg != nil {
			projCfg, loadErr := config.LoadProject(m.cwd)
			if loadErr != nil {
				return extrasActionDoneMsg{err: loadErr}
			}
			if err := applyExtraTarget(projCfg.Extras, name, targetPath, func(et *config.ExtraTargetConfig) { et.Flatten = newFlatten }); err != nil {
				return extrasActionDoneMsg{err: err}
			}
			if err := projCfg.Save(m.cwd); err != nil {
				return extrasActionDoneMsg{err: err}
			}
		} else {
			cfg, loadErr := config.Load()
			if loadErr != nil {
				return extrasActionDoneMsg{err: loadErr}
			}
			if err := applyExtraTarget(cfg.Extras, name, targetPath, func(et *config.ExtraTargetConfig) { et.Flatten = newFlatten }); err != nil {
				return extrasActionDoneMsg{err: err}
			}
			if err := cfg.Save(); err != nil {
				return extrasActionDoneMsg{err: err}
			}
		}

		label := "enabled"
		if !newFlatten {
			label = "disabled"
		}
		return extrasActionDoneMsg{msg: fmt.Sprintf("✓ Flatten %s for %s → %s", label, name, shortenPath(targetPath))}
	}
}

// ─── Content Viewer ──────────────────────────────────────────────────

func (m *extrasListTUIModel) loadExtrasContent(e extrasListEntry) {
	m.contentExtraKey = e.Name
	m.contentScroll = 0
	m.treeCursor = 0
	m.treeScroll = 0

	m.treeAllNodes = buildTreeNodes(e.SourceDir)
	m.treeNodes = buildVisibleNodes(m.treeAllNodes)

	if len(m.treeNodes) == 0 {
		m.contentText = "(no files)"
		return
	}

	m.autoPreviewExtrasFile()
}

func (m *extrasListTUIModel) autoPreviewExtrasFile() {
	if len(m.treeNodes) == 0 || m.treeCursor >= len(m.treeNodes) {
		return
	}
	if !m.treeNodes[m.treeCursor].isDir {
		m.loadExtrasContentFile()
	}
}

func (m *extrasListTUIModel) loadExtrasContentFile() {
	m.contentScroll = 0

	if len(m.treeNodes) == 0 || m.treeCursor >= len(m.treeNodes) {
		m.contentText = "(no files)"
		return
	}

	node := m.treeNodes[m.treeCursor]
	if node.isDir {
		m.contentText = fmt.Sprintf("(directory: %s)", node.name)
		return
	}

	sourceDir := ""
	if item, ok := m.list.SelectedItem().(extraTUIItem); ok {
		sourceDir = item.entry.SourceDir
	}
	if sourceDir == "" {
		m.contentText = "(no source)"
		return
	}

	filePath := filepath.Join(sourceDir, node.relPath)
	data, err := os.ReadFile(filePath)
	if err != nil {
		m.contentText = fmt.Sprintf("(error reading file: %v)", err)
		return
	}

	rawText := printableText(strings.TrimSpace(string(data)))
	if rawText == "" {
		m.contentText = "(empty)"
		return
	}

	w := m.extrasContentPanelWidth()
	if strings.HasSuffix(strings.ToLower(node.name), ".md") {
		m.contentText = hardWrapContent(renderMarkdown(rawText, w), w)
		return
	}
	m.contentText = hardWrapContent(rawText, w)
}

func (m *extrasListTUIModel) extrasContentPanelWidth() int {
	return fileViewerTextWidth(m.termWidth, false)
}

func (m *extrasListTUIModel) extrasContentViewHeight() int {
	return fileViewerHeight(m.termHeight)
}

func (m *extrasListTUIModel) extrasContentMaxScroll() int {
	lines := strings.Split(m.contentText, "\n")
	return max(len(lines)-m.extrasContentViewHeight(), 0)
}

func (m *extrasListTUIModel) ensureExtrasTreeCursorVisible() {
	contentHeight := m.extrasContentViewHeight()
	if m.treeCursor < m.treeScroll {
		m.treeScroll = m.treeCursor
	} else if m.treeCursor >= m.treeScroll+contentHeight {
		m.treeScroll = m.treeCursor - contentHeight + 1
	}
}

func (m extrasListTUIModel) handleExtrasContentKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.showContent = false
		return m, nil
	case "j", "down":
		if m.treeCursor < len(m.treeNodes)-1 {
			m.treeCursor++
			m.ensureExtrasTreeCursorVisible()
			m.autoPreviewExtrasFile()
		}
		return m, nil
	case "k", "up":
		if m.treeCursor > 0 {
			m.treeCursor--
			m.ensureExtrasTreeCursorVisible()
			m.autoPreviewExtrasFile()
		}
		return m, nil
	case "l", "right", "enter":
		if len(m.treeNodes) > 0 && m.treeCursor < len(m.treeNodes) {
			if m.treeNodes[m.treeCursor].isDir {
				m.toggleExtrasTreeDir()
			}
		}
		return m, nil
	case "h", "left":
		m.collapseOrParentExtras()
		return m, nil
	case "ctrl+d":
		half := m.extrasContentViewHeight() / 2
		m.contentScroll = min(m.contentScroll+half, m.extrasContentMaxScroll())
		return m, nil
	case "ctrl+u":
		half := m.extrasContentViewHeight() / 2
		m.contentScroll = max(m.contentScroll-half, 0)
		return m, nil
	case "G":
		m.contentScroll = m.extrasContentMaxScroll()
		return m, nil
	case "g":
		m.contentScroll = 0
		return m, nil
	}
	return m, nil
}

func (m extrasListTUIModel) handleExtrasContentMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	inSidebar := msg.X < sidebarWidth(m.termWidth)

	switch {
	case msg.Button == tea.MouseButtonWheelUp:
		if inSidebar {
			if m.treeCursor > 0 {
				m.treeCursor--
				m.ensureExtrasTreeCursorVisible()
				m.autoPreviewExtrasFile()
			}
		} else if m.contentScroll > 0 {
			m.contentScroll--
		}
	case msg.Button == tea.MouseButtonWheelDown:
		if inSidebar {
			if m.treeCursor < len(m.treeNodes)-1 {
				m.treeCursor++
				m.ensureExtrasTreeCursorVisible()
				m.autoPreviewExtrasFile()
			}
		} else {
			m.contentScroll = min(m.contentScroll+1, m.extrasContentMaxScroll())
		}
	case msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress:
		if inSidebar {
			row := msg.Y - 2
			idx := m.treeScroll + row
			if idx >= 0 && idx < len(m.treeNodes) {
				m.treeCursor = idx
				if m.treeNodes[idx].isDir {
					m.toggleExtrasTreeDir()
				} else {
					m.loadExtrasContentFile()
				}
			}
		}
	}
	return m, nil
}

func (m *extrasListTUIModel) toggleExtrasTreeDir() {
	if len(m.treeNodes) == 0 || m.treeCursor >= len(m.treeNodes) {
		return
	}
	node := m.treeNodes[m.treeCursor]
	if !node.isDir {
		return
	}
	for i := range m.treeAllNodes {
		if m.treeAllNodes[i].relPath == node.relPath {
			m.treeAllNodes[i].expanded = !m.treeAllNodes[i].expanded
			break
		}
	}
	m.treeNodes = buildVisibleNodes(m.treeAllNodes)
	if m.treeCursor >= len(m.treeNodes) {
		m.treeCursor = len(m.treeNodes) - 1
	}
}

func (m *extrasListTUIModel) collapseOrParentExtras() {
	if len(m.treeNodes) == 0 || m.treeCursor >= len(m.treeNodes) {
		return
	}
	node := m.treeNodes[m.treeCursor]
	if node.isDir && node.expanded {
		m.toggleExtrasTreeDir()
		return
	}
	if node.depth > 0 {
		for i := m.treeCursor - 1; i >= 0; i-- {
			if m.treeNodes[i].isDir && m.treeNodes[i].depth == node.depth-1 {
				m.treeCursor = i
				m.ensureExtrasTreeCursorVisible()
				return
			}
		}
	}
}

func (m extrasListTUIModel) renderExtrasContentOverlay() string {
	return renderFileViewer(m.termWidth, m.termHeight, fileViewer{
		command: "extras", name: m.contentExtraKey,
		nodes: m.treeNodes, cursor: m.treeCursor, scroll: m.treeScroll,
		content: m.contentText, contentScroll: m.contentScroll,
	})
}

// ─── Runner ──────────────────────────────────────────────────────────

func runExtrasListTUI(
	loadFn func() ([]extrasListEntry, error),
	modeLabel string,
	cfg *config.Config,
	projCfg *config.ProjectConfig,
	cwd, configPath string,
	sourceFunc func(extra config.ExtraConfig) string,
) error {
	for {
		model := newExtrasListTUIModel(loadFn, modeLabel, cfg, projCfg, cwd, configPath, sourceFunc)
		p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
		finalModel, err := p.Run()
		if err != nil {
			return err
		}

		m, ok := finalModel.(extrasListTUIModel)
		if !ok {
			return nil
		}
		if m.loadErr != nil {
			return m.loadErr
		}
		if m.emptyResult {
			ui.Done(ui.MarkNone, "No extras configured", 0)
			ui.Next("skillshare extras init <name> --target <path>", "add one")
			return nil
		}

		if !m.wantsNew {
			return nil
		}

		// Launch init wizard, then loop back to list TUI
		mode := modeGlobal
		if projCfg != nil {
			mode = modeProject
		}
		if err := cmdExtrasInitPrompt(mode, cwd); err != nil {
			return err
		}
	}
}
