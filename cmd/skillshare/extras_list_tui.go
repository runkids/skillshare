package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/sync"
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
	targetCursor    int
	targetAction    string

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
	l.Title = fmt.Sprintf("Extras (%s)", modeLabel)
	l.Styles.Title = tc.ListTitle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = tc.SpinnerStyle

	fi := textinput.New()
	fi.Prompt = "/ "
	fi.PromptStyle = tc.Filter
	fi.Cursor.Style = tc.Filter
	fi.Placeholder = "filter by name"

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

		if m.filtering {
			switch msg.String() {
			case "esc":
				m.filtering = false
				m.filterText = ""
				m.filterInput.SetValue("")
				m.applyExtrasFilter()
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
				m.applyExtrasFilter()
			}
			return m, cmd
		}

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
			m.lastActionMsg = ""
			return m, textinput.Blink
		case "enter":
			if item, ok := m.list.SelectedItem().(extraTUIItem); ok {
				m.loadExtrasContent(item.entry)
				m.showContent = true
			}
			return m, nil
		case "N":
			m.wantsNew = true
			return m, tea.Quit
		case "X":
			return m.enterExtrasConfirm("remove")
		case "S":
			return m.enterTargetMenu("sync")
		case "C":
			return m.enterTargetMenu("collect")
		case "M":
			return m.enterTargetMenu("mode")
		case "F":
			return m.enterTargetMenu("flatten")
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
	if m.confirming {
		return m.renderConfirmOverlay()
	}
	if m.showTargetMenu {
		return m.renderTargetMenu()
	}
	if m.showModePicker {
		return m.renderModePicker()
	}
	if extrasSplitActive(m.termWidth) {
		return m.viewExtrasSplit()
	}
	return m.viewExtrasVertical()
}

// ─── Layout ──────────────────────────────────────────────────────────

func extrasSplitActive(termWidth int) bool {
	return termWidth >= tuiMinSplitWidth
}

func extrasPanelWidth(termWidth int) int {
	width := termWidth * 28 / 100
	return max(min(width, 36), 22)
}

func extrasDetailPanelWidth(termWidth int) int {
	return max(termWidth-extrasPanelWidth(termWidth)-1, 28)
}

func (m *extrasListTUIModel) syncExtrasListSize() {
	if extrasSplitActive(m.termWidth) {
		panelHeight := max(m.termHeight-5, 6)
		m.list.SetSize(extrasPanelWidth(m.termWidth), panelHeight)
		return
	}
	listHeight := max(m.termHeight-20, 6)
	m.list.SetSize(m.termWidth, listHeight)
}

func (m extrasListTUIModel) viewExtrasSplit() string {
	var b strings.Builder

	panelHeight := max(m.termHeight-5, 6)
	leftWidth := extrasPanelWidth(m.termWidth)
	rightWidth := extrasDetailPanelWidth(m.termWidth)

	var detailStr, scrollInfo string
	if item, ok := m.list.SelectedItem().(extraTUIItem); ok {
		detail := m.renderExtrasDetail(item.entry)
		bodyHeight := max(panelHeight-1, 4)
		detailStr, scrollInfo = wrapAndScroll(detail, rightWidth-1, m.detailScroll, bodyHeight)
		detailStr = "\n" + detailStr
	}

	body := renderHorizontalSplit(m.list.View(), detailStr, leftWidth, rightWidth, panelHeight)
	b.WriteString(body)
	b.WriteString("\n\n")
	b.WriteString(m.renderExtrasFilterBar())
	if m.lastActionMsg != "" {
		b.WriteString(renderExtrasActionMsg(m.lastActionMsg))
		b.WriteString("\n")
	}
	b.WriteString(m.renderExtrasHelp(scrollInfo))
	b.WriteString("\n")

	return b.String()
}

func (m extrasListTUIModel) viewExtrasVertical() string {
	var b strings.Builder

	b.WriteString(m.list.View())
	b.WriteString("\n\n")
	b.WriteString(m.renderExtrasFilterBar())

	var scrollInfo string
	if item, ok := m.list.SelectedItem().(extraTUIItem); ok {
		detailHeight := max(m.termHeight-m.termHeight*2/5-8, 6)
		detail := m.renderExtrasDetail(item.entry)
		body, bodyScrollInfo := wrapAndScroll(detail, m.termWidth, m.detailScroll, detailHeight)
		scrollInfo = bodyScrollInfo
		b.WriteString(body)
	}

	if m.lastActionMsg != "" {
		b.WriteString("\n")
		b.WriteString(renderExtrasActionMsg(m.lastActionMsg))
	}
	b.WriteString("\n")
	b.WriteString(m.renderExtrasHelp(scrollInfo))
	b.WriteString("\n")

	return b.String()
}

// ─── Detail Panel ────────────────────────────────────────────────────

func (m extrasListTUIModel) renderExtrasDetail(e extrasListEntry) string {
	var b strings.Builder

	b.WriteString(tc.Title.Render(e.Name))
	b.WriteString("\n\n")

	label := tc.Label.Render("Source")
	if e.SourceExists {
		b.WriteString(label + shortenPath(e.SourceDir) + "\n")
	} else {
		b.WriteString(label + tc.Dim.Render("not found") + "\n")
	}

	label = tc.Label.Render("Files")
	if e.SourceExists {
		b.WriteString(label + fmt.Sprintf("%d", e.FileCount) + "\n")
	} else {
		b.WriteString(label + tc.Dim.Render("—") + "\n")
	}

	b.WriteString("\n")
	b.WriteString(tc.Title.Render("Targets"))
	b.WriteString("\n")

	if len(e.Targets) == 0 {
		b.WriteString(tc.Dim.Render("  No targets configured") + "\n")
	} else {
		hasDrift := false
		for _, t := range e.Targets {
			var icon string
			var style lipgloss.Style
			switch t.Status {
			case "synced":
				icon = "✓"
				style = tc.Green
			case "drift":
				icon = "△"
				style = tc.Yellow
				hasDrift = true
			case "not synced":
				icon = "✗"
				style = tc.Red
				hasDrift = true
			default:
				icon = "-"
				style = tc.Dim
			}
			statusText := ""
			if t.Status != "synced" {
				statusText = "  " + t.Status
			}
			modeLabel := t.Mode
			if t.Flatten {
				modeLabel += ", flatten"
			}
			fmt.Fprintf(&b, "  %s %s (%s)%s\n",
				style.Render(icon), shortenPath(t.Path), modeLabel, tc.Dim.Render(statusText))
		}
		if hasDrift {
			b.WriteString("\n" + tc.Yellow.Render("hint:") + " press S to sync, or use --force to overwrite conflicts\n")
		}
	}

	if e.SourceExists && e.FileCount > 0 {
		b.WriteString("\n")
		b.WriteString(tc.Title.Render("Files"))
		b.WriteString("\n")
		files := discoverExtraFileNames(e.SourceDir)
		maxShow := 10
		for i, f := range files {
			if i >= maxShow {
				b.WriteString(tc.Dim.Render(fmt.Sprintf("  … and %d more", len(files)-maxShow)) + "\n")
				break
			}
			prefix := "├── "
			if i == len(files)-1 || i == maxShow-1 {
				prefix = "└── "
			}
			b.WriteString(tc.Dim.Render("  "+prefix) + f + "\n")
		}
	}

	return b.String()
}

func discoverExtraFileNames(sourceDir string) []string {
	files, err := sync.DiscoverExtraFiles(sourceDir)
	if err != nil {
		return nil
	}
	return files
}

// ─── Filter ──────────────────────────────────────────────────────────

func (m extrasListTUIModel) renderExtrasFilterBar() string {
	return renderTUIFilterBar(
		m.filterInput.View(), m.filtering, m.filterText,
		m.matchCount, len(m.allItems), 0,
		"extras", renderPageInfoFromPaginator(m.list.Paginator),
	)
}

func (m extrasListTUIModel) renderExtrasHelp(scrollInfo string) string {
	helpText := "↑↓ navigate  / filter  Enter view  N new  X remove  S sync  C collect  M mode  F flatten  q quit"
	if m.filtering {
		helpText = "Enter lock  Esc clear  q quit"
	}
	return tc.Help.Render(appendScrollInfo(helpText, scrollInfo))
}

func renderExtrasActionMsg(msg string) string {
	if strings.HasPrefix(msg, "✓") {
		return tc.Green.Render(msg)
	}
	if strings.HasPrefix(msg, "✗") {
		return tc.Red.Render(msg)
	}
	return tc.Yellow.Render(msg)
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
		extrasSource = m.cfg.ExtrasSource
	}
	entries := buildExtrasListEntries(extras, extrasSource, m.sourceFunc)
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
	case "y", "Y", "enter":
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

func (m extrasListTUIModel) renderConfirmOverlay() string {
	var title, body string

	switch m.confirmAction {
	case "remove":
		title = "Remove"
		body = fmt.Sprintf("Remove extra %q?\nThis only removes config.\nRun sync to clean up orphaned links.", m.confirmExtra)
	case "sync":
		title = "Sync"
		target := "all targets"
		if m.confirmTarget != "" {
			target = shortenPath(m.confirmTarget)
		}
		body = fmt.Sprintf("Sync %q to %s?", m.confirmExtra, target)
	case "collect":
		title = "Collect"
		target := "all targets"
		if m.confirmTarget != "" {
			target = shortenPath(m.confirmTarget)
		}
		body = fmt.Sprintf("Collect from %s into %q?", target, m.confirmExtra)
	}

	return fmt.Sprintf("\n%s\n\n%s\n\nProceed? [Y/n] ",
		tc.Title.Render(title), body)
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
	// Single target: skip menu for sync/collect/mode/flatten
	if len(item.entry.Targets) == 1 {
		if action == "mode" {
			return m.openModePicker(item.entry.Name, item.entry.Targets[0])
		}
		if action == "flatten" {
			return m, m.doFlattenToggle(item.entry.Name, item.entry.Targets[0])
		}
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
	m.targetCursor = 0
	m.lastActionMsg = ""
	return m, nil
}

func (m extrasListTUIModel) handleTargetMenuKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// "mode" and "flatten" have no "All targets" row
	totalItems := len(m.targetMenuItems) + 1
	if m.targetAction == "mode" || m.targetAction == "flatten" {
		totalItems = len(m.targetMenuItems)
	}

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

		// Mode/flatten: no "All targets", go directly to picker/toggle
		if m.targetAction == "mode" {
			t := m.targetMenuItems[m.targetCursor]
			return m.openModePicker(item.entry.Name, t)
		}
		if m.targetAction == "flatten" {
			t := m.targetMenuItems[m.targetCursor]
			return m, m.doFlattenToggle(item.entry.Name, t)
		}

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

	title := "Sync targets"
	switch m.targetAction {
	case "collect":
		title = "Collect from"
	case "mode":
		title = "Change mode"
	case "flatten":
		title = "Toggle flatten"
	}

	fmt.Fprintf(&b, "\n%s\n\n", tc.Title.Render(title))

	if m.targetAction == "mode" || m.targetAction == "flatten" {
		// No "All targets" for mode — list targets directly
		for i, t := range m.targetMenuItems {
			prefix := "  "
			if i == m.targetCursor {
				prefix = tc.Cyan.Render(">") + " "
			}
			fmt.Fprintf(&b, "%s%s  (%s)\n", prefix, shortenPath(t.Path), t.Mode)
		}
	} else {
		for i := 0; i <= len(m.targetMenuItems); i++ {
			prefix := "  "
			if i == m.targetCursor {
				prefix = tc.Cyan.Render(">") + " "
			}
			if i == 0 {
				fmt.Fprintf(&b, "%s%s\n", prefix, "All targets")
			} else {
				t := m.targetMenuItems[i-1]
				fmt.Fprintf(&b, "%s%s  (%s)\n", prefix, shortenPath(t.Path), t.Mode)
			}
		}
	}

	fmt.Fprintf(&b, "\n%s\n", tc.Help.Render("↑↓ select  Enter confirm  Esc cancel"))

	return b.String()
}

// ─── Mode Picker ─────────────────────────────────────────────────────

var extrasSyncModes = config.ExtraSyncModes

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

	fmt.Fprintf(&b, "\n%s\n", tc.Title.Render("Change mode"))
	fmt.Fprintf(&b, "%s  %s\n\n", tc.Dim.Render("Extra:"), m.modePickerExtra)
	fmt.Fprintf(&b, "%s  %s\n\n", tc.Dim.Render("Target:"), shortenPath(m.modePickerTarget))

	for i, mode := range extrasSyncModes {
		cursor := "  "
		if i == m.modeCursor {
			cursor = tc.Cyan.Render(">") + " "
		}
		var desc string
		switch mode {
		case "merge":
			desc = " (per-file symlinks)"
		case "copy":
			desc = " (file copies)"
		case "symlink":
			desc = " (directory symlink)"
		}
		if i == m.modeCursor {
			fmt.Fprintf(&b, "%s%s%s\n", cursor, tc.Cyan.Render(mode), tc.Dim.Render(desc))
		} else {
			fmt.Fprintf(&b, "%s%s%s\n", cursor, mode, tc.Dim.Render(desc))
		}
	}

	fmt.Fprintf(&b, "\n%s\n", tc.Help.Render("↑↓ select  Enter confirm  Esc cancel"))
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
	synced := 0
	for _, t := range targets {
		mode := sync.EffectiveMode(t.Mode)
		resolved := config.ExpandPath(t.Path)
		_, err := sync.SyncExtra(sourceDir, resolved, mode, false, false, t.Flatten, m.projectRoot())
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

	sourceDir := m.sourceFunc(*extra)
	collected := 0
	for _, t := range targets {
		resolved := config.ExpandPath(t.Path)
		result, err := sync.CollectExtraFiles(sourceDir, resolved, false, t.Flatten, m.projectRoot())
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

	rawText := strings.TrimSpace(string(data))
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
	sw := sidebarWidth(m.termWidth)
	return max(m.termWidth-sw-5-1, 40)
}

func (m *extrasListTUIModel) extrasContentViewHeight() int {
	return max(m.termHeight-7, 5)
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
	sw := sidebarWidth(m.termWidth)
	inSidebar := msg.X < sw+3

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
	var b strings.Builder

	extraName := m.contentExtraKey
	fileName := ""
	if len(m.treeNodes) > 0 && m.treeCursor < len(m.treeNodes) {
		fileName = m.treeNodes[m.treeCursor].relPath
	}

	b.WriteString("\n")
	b.WriteString(tc.Title.Render(fmt.Sprintf("  %s", extraName)))
	if fileName != "" {
		b.WriteString(tc.Dim.Render(fmt.Sprintf("  ─  %s", fileName)))
	}
	b.WriteString("\n\n")

	sw := sidebarWidth(m.termWidth)
	panelW := max(m.termWidth-sw-5, 20)
	contentHeight := m.extrasContentViewHeight()

	sidebarStr := m.renderExtrasSidebarStr(sw, contentHeight)
	contentStr, scrollInfo := m.renderExtrasContentPanelStr(contentHeight)

	leftPanel := lipgloss.NewStyle().
		Width(sw).MaxWidth(sw).
		Height(contentHeight).MaxHeight(contentHeight).
		PaddingLeft(1).
		Render(sidebarStr)

	borderStyle := tc.Border.Height(contentHeight).MaxHeight(contentHeight)
	borderCol := strings.Repeat("│\n", contentHeight)
	borderPanel := borderStyle.Render(strings.TrimRight(borderCol, "\n"))

	rightPanel := lipgloss.NewStyle().
		Width(panelW).MaxWidth(panelW).
		Height(contentHeight).MaxHeight(contentHeight).
		PaddingLeft(1).
		Render(contentStr)

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, borderPanel, rightPanel)
	b.WriteString(body)
	b.WriteString("\n\n")

	help := "j/k browse  l/Enter expand  h collapse  Ctrl+d/u scroll  g/G top/bottom  Esc back  q quit"
	if scrollInfo != "" {
		help += "  " + scrollInfo
	}
	b.WriteString(tc.Help.Render(help))
	b.WriteString("\n")

	return b.String()
}

func (m extrasListTUIModel) renderExtrasSidebarStr(width, height int) string {
	if len(m.treeNodes) == 0 {
		return "(no files)"
	}

	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(tc.BrandYellow)
	dirStyle := tc.Cyan
	fileStyle := lipgloss.NewStyle()

	total := len(m.treeNodes)
	start := min(m.treeScroll, total-height)
	start = max(start, 0)
	end := min(start+height, total)

	var lines []string
	for i := start; i < end; i++ {
		n := m.treeNodes[i]
		indent := strings.Repeat("  ", n.depth)

		var prefix string
		if n.isDir {
			if n.expanded {
				prefix = "▾ "
			} else {
				prefix = "▸ "
			}
		} else {
			prefix = "  "
		}

		name := n.name
		if n.isDir {
			name += "/"
		}

		label := indent + prefix + name
		maxLabel := max(width-2, 5)
		if len(label) > maxLabel {
			label = label[:maxLabel-3] + "..."
		}

		if i == m.treeCursor {
			lines = append(lines, selectedStyle.Render(label))
		} else if n.isDir {
			lines = append(lines, dirStyle.Render(label))
		} else {
			lines = append(lines, fileStyle.Render(label))
		}
	}

	if total > height {
		lines = append(lines, tc.Dim.Render(fmt.Sprintf(" (%d/%d)", m.treeCursor+1, total)))
	}

	return strings.Join(lines, "\n")
}

func (m extrasListTUIModel) renderExtrasContentPanelStr(height int) (string, string) {
	lines := strings.Split(m.contentText, "\n")
	totalLines := len(lines)

	if totalLines <= height {
		return strings.Join(lines, "\n"), ""
	}

	maxScroll := totalLines - height
	offset := min(m.contentScroll, maxScroll)

	visible := lines[offset : offset+height]
	result := make([]string, height)
	copy(result, visible)

	scrollInfo := fmt.Sprintf("(%d/%d)", offset+1, maxScroll+1)
	return strings.Join(result, "\n"), scrollInfo
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
			ui.Info("No extras configured.")
			ui.Info("Run 'skillshare extras init <name> --target <path>' to add one.")
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
		if err := cmdExtrasInitTUI(mode, cwd); err != nil {
			return err
		}
	}
}
