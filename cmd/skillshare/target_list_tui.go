package main

import (
	"fmt"
	"sort"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/sync"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// ---- Messages ---------------------------------------------------------------

type targetListLoadedMsg struct {
	items []targetTUIItem
	err   error
}

type targetListActionDoneMsg struct {
	msg string
	err error
}

// ---- Model ------------------------------------------------------------------

type targetListTUIModel struct {
	list       list.Model
	allItems   []targetTUIItem
	modeLabel  string
	quitting   bool
	termWidth  int
	termHeight int

	// Config context (dual-mode)
	cfg     *config.Config
	projCfg *config.ProjectConfig
	cwd     string

	// Async loading
	loading     bool
	loadSpinner spinner.Model
	loadErr     error
	emptyResult bool

	// Filter
	filterText  string
	filterInput textinput.Model
	filtering   bool
	matchCount  int

	// Detail panel
	detailScroll int

	// Mode picker overlay
	showModePicker   bool
	modePickerTarget string // target name being edited
	modeCursor       int

	// Include/Exclude edit sub-panel
	editingFilter    bool   // true when in I/E edit mode
	editFilterType   string // "include" or "exclude"
	editFilterTarget string // target name being edited
	editPatterns     []string
	editCursor       int // selected pattern index
	editAdding       bool
	editInput        textinput.Model

	// Action feedback
	lastActionMsg string
}

func newTargetListTUIModel(
	modeLabel string,
	cfg *config.Config,
	projCfg *config.ProjectConfig,
	cwd string,
) targetListTUIModel {
	delegate := targetListDelegate{}

	l := list.New(nil, delegate, 0, 0)
	l.Title = fmt.Sprintf("Targets (%s)", modeLabel)
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

	ei := textinput.New()
	ei.Prompt = "> pattern: "
	ei.PromptStyle = tc.Cyan
	ei.Cursor.Style = tc.Cyan
	ei.Placeholder = "glob pattern"

	return targetListTUIModel{
		list:        l,
		modeLabel:   modeLabel,
		cfg:         cfg,
		projCfg:     projCfg,
		cwd:         cwd,
		loading:     true,
		loadSpinner: sp,
		filterInput: fi,
		editInput:   ei,
	}
}

func (m targetListTUIModel) Init() tea.Cmd {
	return tea.Batch(
		m.loadSpinner.Tick,
		m.loadTargets(),
	)
}

func (m targetListTUIModel) loadTargets() tea.Cmd {
	return func() tea.Msg {
		items, err := buildTargetTUIItems(m.projCfg != nil, m.cwd)
		return targetListLoadedMsg{items: items, err: err}
	}
}

func buildTargetTUIItems(isProject bool, cwd string) ([]targetTUIItem, error) {
	var items []targetTUIItem
	if isProject {
		projCfg, err := config.LoadProject(cwd)
		if err != nil {
			return nil, err
		}
		for _, entry := range projCfg.Targets {
			items = append(items, targetTUIItem{
				name: entry.Name,
				target: config.TargetConfig{
					Path:    projectTargetDisplayPath(entry),
					Mode:    entry.Mode,
					Include: entry.Include,
					Exclude: entry.Exclude,
				},
			})
		}
	} else {
		cfg, err := config.Load()
		if err != nil {
			return nil, err
		}
		for name, t := range cfg.Targets {
			items = append(items, targetTUIItem{name: name, target: t})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].name < items[j].name
	})
	return items, nil
}

// ---- Update -----------------------------------------------------------------

func (m targetListTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		m.syncTargetListSize()
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.loadSpinner, cmd = m.loadSpinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case targetListLoadedMsg:
		m.loading = false
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
		items := make([]list.Item, len(msg.items))
		for i, it := range msg.items {
			items[i] = it
		}
		m.list.SetItems(items)
		m.matchCount = len(msg.items)
		m.syncTargetListSize()
		return m, nil

	case targetListActionDoneMsg:
		if msg.err != nil {
			m.lastActionMsg = "✗ " + msg.err.Error()
		} else {
			m.lastActionMsg = msg.msg
		}
		m.reloadTargetItems()
		return m, nil

	case tea.KeyMsg:
		if m.loading {
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				m.quitting = true
				return m, tea.Quit
			}
			return m, nil
		}
		if m.showModePicker {
			return m.handleModePickerKey(msg)
		}
		if m.editingFilter {
			return m.handleFilterEditKey(msg)
		}
		if m.filtering {
			return m.handleFilterInputKey(msg)
		}
		return m.handleNormalKey(msg)

	case tea.MouseMsg:
		if targetSplitActive(m.termWidth) && !m.loading {
			pw := targetPanelWidth(m.termWidth)
			if msg.X > pw {
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
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m targetListTUIModel) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "ctrl+d":
		m.detailScroll += 8
		return m, nil
	case "ctrl+u":
		if m.detailScroll >= 8 {
			m.detailScroll -= 8
		} else {
			m.detailScroll = 0
		}
		return m, nil
	case "/":
		m.filtering = true
		m.filterInput.Focus()
		m.lastActionMsg = ""
		return m, textinput.Blink
	case "M":
		if item, ok := m.list.SelectedItem().(targetTUIItem); ok {
			return m.openModePicker(item.name, item.target)
		}
		return m, nil
	case "I":
		if item, ok := m.list.SelectedItem().(targetTUIItem); ok {
			return m.openFilterEdit(item.name, "include", item.target.Include)
		}
		return m, nil
	case "E":
		if item, ok := m.list.SelectedItem().(targetTUIItem); ok {
			return m.openFilterEdit(item.name, "exclude", item.target.Exclude)
		}
		return m, nil
	}

	prevName := targetSelectedName(m.list.SelectedItem())
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	if targetSelectedName(m.list.SelectedItem()) != prevName {
		m.detailScroll = 0
		m.lastActionMsg = ""
	}
	return m, cmd
}

func targetSelectedName(item list.Item) string {
	if ti, ok := item.(targetTUIItem); ok {
		return ti.name
	}
	return ""
}

func (m targetListTUIModel) handleFilterInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.filtering = false
		m.filterText = ""
		m.filterInput.SetValue("")
		m.applyTargetFilter()
		return m, nil
	case "enter":
		m.filtering = false
		m.filterInput.Blur()
		m.filterText = m.filterInput.Value()
		m.applyTargetFilter()
		return m, nil
	}
	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	m.filterText = m.filterInput.Value()
	m.applyTargetFilter()
	return m, cmd
}

func (m *targetListTUIModel) applyTargetFilter() {
	query := strings.ToLower(m.filterText)
	var filtered []list.Item
	for _, it := range m.allItems {
		if query == "" || strings.Contains(strings.ToLower(it.name), query) {
			filtered = append(filtered, it)
		}
	}
	m.list.SetItems(filtered)
	m.list.ResetSelected()
	m.matchCount = len(filtered)
	m.detailScroll = 0
}

func (m *targetListTUIModel) reloadTargetItems() {
	items, err := buildTargetTUIItems(m.projCfg != nil, m.cwd)
	if err == nil {
		m.allItems = items
		m.applyTargetFilter()
	}
}

// ─── Mode Picker ─────────────────────────────────────────────────────

var targetSyncModes = config.ExtraSyncModes // ["merge", "copy", "symlink"]

func (m targetListTUIModel) openModePicker(name string, target config.TargetConfig) (tea.Model, tea.Cmd) {
	m.showModePicker = true
	m.modePickerTarget = name
	m.modeCursor = 0
	current := sync.EffectiveMode(target.Mode)
	for i, mode := range targetSyncModes {
		if mode == current {
			m.modeCursor = i
			break
		}
	}
	m.lastActionMsg = ""
	return m, nil
}

func (m targetListTUIModel) handleModePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		if m.modeCursor < len(targetSyncModes)-1 {
			m.modeCursor++
		}
		return m, nil
	case "enter":
		m.showModePicker = false
		newMode := targetSyncModes[m.modeCursor]
		name := m.modePickerTarget
		return m, func() tea.Msg {
			msg, err := m.doSetTargetMode(name, newMode)
			return targetListActionDoneMsg{msg: msg, err: err}
		}
	}
	return m, nil
}

func (m targetListTUIModel) doSetTargetMode(name, newMode string) (string, error) {
	if m.projCfg != nil {
		projCfg, err := config.LoadProject(m.cwd)
		if err != nil {
			return "", err
		}
		for i, entry := range projCfg.Targets {
			if entry.Name == name {
				projCfg.Targets[i].Mode = newMode
				break
			}
		}
		if err := projCfg.Save(m.cwd); err != nil {
			return "", err
		}
	} else {
		cfg, err := config.Load()
		if err != nil {
			return "", err
		}
		t := cfg.Targets[name]
		t.Mode = newMode
		cfg.Targets[name] = t
		if err := cfg.Save(); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("✓ Set %s mode to %s", name, newMode), nil
}

// ─── Include/Exclude Edit Sub-Panel ──────────────────────────────────

func (m targetListTUIModel) openFilterEdit(name, filterType string, patterns []string) (tea.Model, tea.Cmd) {
	m.editingFilter = true
	m.editFilterType = filterType
	m.editFilterTarget = name
	m.editPatterns = make([]string, len(patterns))
	copy(m.editPatterns, patterns)
	m.editCursor = 0
	m.editAdding = false
	m.editInput.Reset()
	m.lastActionMsg = ""
	return m, nil
}

func (m targetListTUIModel) handleFilterEditKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.editAdding {
		return m.handleFilterEditAddKey(msg)
	}

	switch msg.String() {
	case "esc":
		m.editingFilter = false
		return m, nil
	case "up", "k":
		if m.editCursor > 0 {
			m.editCursor--
		}
		return m, nil
	case "down", "j":
		if m.editCursor < len(m.editPatterns)-1 {
			m.editCursor++
		}
		return m, nil
	case "a":
		m.editAdding = true
		m.editInput.Reset()
		m.editInput.Focus()
		return m, nil
	case "d":
		if len(m.editPatterns) > 0 && m.editCursor < len(m.editPatterns) {
			pattern := m.editPatterns[m.editCursor]
			m.editPatterns = append(m.editPatterns[:m.editCursor], m.editPatterns[m.editCursor+1:]...)
			if m.editCursor >= len(m.editPatterns) && m.editCursor > 0 {
				m.editCursor--
			}
			name := m.editFilterTarget
			filterType := m.editFilterType
			return m, func() tea.Msg {
				msg, err := m.doRemovePattern(name, filterType, pattern)
				return targetListActionDoneMsg{msg: msg, err: err}
			}
		}
		return m, nil
	}
	return m, nil
}

func (m targetListTUIModel) handleFilterEditAddKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.editAdding = false
		m.editInput.Blur()
		return m, nil
	case "enter":
		pattern := strings.TrimSpace(m.editInput.Value())
		m.editAdding = false
		m.editInput.Blur()
		if pattern == "" {
			return m, nil
		}
		m.editPatterns = append(m.editPatterns, pattern)
		m.editCursor = len(m.editPatterns) - 1
		name := m.editFilterTarget
		filterType := m.editFilterType
		return m, func() tea.Msg {
			msg, err := m.doAddPattern(name, filterType, pattern)
			return targetListActionDoneMsg{msg: msg, err: err}
		}
	}
	var cmd tea.Cmd
	m.editInput, cmd = m.editInput.Update(msg)
	return m, cmd
}

func (m targetListTUIModel) doAddPattern(name, filterType, pattern string) (string, error) {
	if m.projCfg != nil {
		projCfg, err := config.LoadProject(m.cwd)
		if err != nil {
			return "", err
		}
		for i, entry := range projCfg.Targets {
			if entry.Name == name {
				if filterType == "include" {
					projCfg.Targets[i].Include = append(projCfg.Targets[i].Include, pattern)
				} else {
					projCfg.Targets[i].Exclude = append(projCfg.Targets[i].Exclude, pattern)
				}
				break
			}
		}
		if err := projCfg.Save(m.cwd); err != nil {
			return "", err
		}
	} else {
		cfg, err := config.Load()
		if err != nil {
			return "", err
		}
		t := cfg.Targets[name]
		if filterType == "include" {
			t.Include = append(t.Include, pattern)
		} else {
			t.Exclude = append(t.Exclude, pattern)
		}
		cfg.Targets[name] = t
		if err := cfg.Save(); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("✓ Added %s pattern: %s", filterType, pattern), nil
}

func (m targetListTUIModel) doRemovePattern(name, filterType, pattern string) (string, error) {
	removeFromSlice := func(slice []string, val string) []string {
		var result []string
		for _, s := range slice {
			if s != val {
				result = append(result, s)
			}
		}
		return result
	}

	if m.projCfg != nil {
		projCfg, err := config.LoadProject(m.cwd)
		if err != nil {
			return "", err
		}
		for i, entry := range projCfg.Targets {
			if entry.Name == name {
				if filterType == "include" {
					projCfg.Targets[i].Include = removeFromSlice(entry.Include, pattern)
				} else {
					projCfg.Targets[i].Exclude = removeFromSlice(entry.Exclude, pattern)
				}
				break
			}
		}
		if err := projCfg.Save(m.cwd); err != nil {
			return "", err
		}
	} else {
		cfg, err := config.Load()
		if err != nil {
			return "", err
		}
		t := cfg.Targets[name]
		if filterType == "include" {
			t.Include = removeFromSlice(t.Include, pattern)
		} else {
			t.Exclude = removeFromSlice(t.Exclude, pattern)
		}
		cfg.Targets[name] = t
		if err := cfg.Save(); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("✓ Removed %s pattern: %s", filterType, pattern), nil
}

// ---- View -------------------------------------------------------------------

func (m targetListTUIModel) View() string {
	if m.quitting {
		return ""
	}
	if m.loading {
		return fmt.Sprintf("\n  %s Loading targets...\n", m.loadSpinner.View())
	}
	if m.showModePicker {
		return m.renderModePicker()
	}
	if targetSplitActive(m.termWidth) {
		return m.viewTargetSplit()
	}
	return m.viewTargetVertical()
}

func (m targetListTUIModel) viewTargetSplit() string {
	var b strings.Builder

	panelHeight := max(m.termHeight-5, 6)
	leftWidth := targetPanelWidth(m.termWidth)
	rightWidth := targetDetailPanelWidth(m.termWidth)

	var detailStr, scrollInfo string
	if item, ok := m.list.SelectedItem().(targetTUIItem); ok {
		if m.editingFilter {
			detailStr = "\n" + m.renderFilterEditPanel()
		} else {
			detail := m.renderTargetDetail(item)
			bodyHeight := max(panelHeight-1, 4)
			detailStr, scrollInfo = wrapAndScroll(detail, rightWidth-1, m.detailScroll, bodyHeight)
			detailStr = "\n" + detailStr
		}
	}

	body := renderHorizontalSplit(m.list.View(), detailStr, leftWidth, rightWidth, panelHeight)
	b.WriteString(body)
	b.WriteString("\n\n")
	b.WriteString(m.renderTargetFilterBar())
	if m.lastActionMsg != "" {
		b.WriteString(renderTargetActionMsg(m.lastActionMsg))
		b.WriteString("\n")
	}
	b.WriteString(m.renderTargetHelp(scrollInfo))
	b.WriteString("\n")

	return b.String()
}

func (m targetListTUIModel) viewTargetVertical() string {
	var b strings.Builder

	b.WriteString(m.list.View())
	b.WriteString("\n\n")
	b.WriteString(m.renderTargetFilterBar())

	var scrollInfo string
	if item, ok := m.list.SelectedItem().(targetTUIItem); ok {
		if m.editingFilter {
			b.WriteString(m.renderFilterEditPanel())
		} else {
			detailHeight := max(m.termHeight-m.termHeight*2/5-8, 6)
			detail := m.renderTargetDetail(item)
			body, bodyScrollInfo := wrapAndScroll(detail, m.termWidth, m.detailScroll, detailHeight)
			scrollInfo = bodyScrollInfo
			b.WriteString(body)
		}
	}

	if m.lastActionMsg != "" {
		b.WriteString("\n")
		b.WriteString(renderTargetActionMsg(m.lastActionMsg))
	}
	b.WriteString("\n")
	b.WriteString(m.renderTargetHelp(scrollInfo))
	b.WriteString("\n")

	return b.String()
}

func renderTargetActionMsg(msg string) string {
	if strings.HasPrefix(msg, "✓") {
		return tc.Green.Render(msg)
	}
	if strings.HasPrefix(msg, "✗") {
		return tc.Red.Render(msg)
	}
	return tc.Yellow.Render(msg)
}

func (m targetListTUIModel) renderTargetHelp(scrollInfo string) string {
	helpText := "↑↓ navigate  Ctrl+d/u scroll  / filter  M mode  I include  E exclude  q quit"
	if m.filtering {
		helpText = "Enter lock  Esc clear  q quit"
	}
	return tc.Help.Render(appendScrollInfo(helpText, scrollInfo))
}

func (m targetListTUIModel) renderTargetFilterBar() string {
	return renderTUIFilterBar(
		m.filterInput.View(),
		m.filtering,
		m.filterText,
		m.matchCount,
		len(m.allItems),
		0,
		"targets",
		renderPageInfoFromPaginator(m.list.Paginator),
	)
}

// ---- Layout -----------------------------------------------------------------

func targetSplitActive(termWidth int) bool {
	return termWidth >= tuiMinSplitWidth
}

func targetPanelWidth(termWidth int) int {
	w := termWidth * 36 / 100
	return max(min(w, 40), 26)
}

func targetDetailPanelWidth(termWidth int) int {
	return max(termWidth-targetPanelWidth(termWidth)-1, 28)
}

func (m *targetListTUIModel) syncTargetListSize() {
	if targetSplitActive(m.termWidth) {
		pw := targetPanelWidth(m.termWidth)
		ph := max(m.termHeight-5, 6)
		m.list.SetSize(pw, ph)
	} else {
		lh := max(m.termHeight-20, 6)
		m.list.SetSize(m.termWidth, lh)
	}
}

// ---- Detail panel -----------------------------------------------------------

func (m targetListTUIModel) renderTargetDetail(item targetTUIItem) string {
	var b strings.Builder

	fmt.Fprintf(&b, "%s\n\n", tc.Title.Render(item.name))

	fmt.Fprintf(&b, "%s  %s\n", tc.Dim.Render("Path:"), shortenPath(item.target.Path))
	fmt.Fprintf(&b, "%s  %s\n", tc.Dim.Render("Mode:"), sync.EffectiveMode(item.target.Mode))

	if len(item.target.Include) > 0 {
		fmt.Fprintf(&b, "\n%s\n", tc.Dim.Render("Include:"))
		for _, p := range item.target.Include {
			fmt.Fprintf(&b, "  %s\n", p)
		}
	}
	if len(item.target.Exclude) > 0 {
		fmt.Fprintf(&b, "\n%s\n", tc.Dim.Render("Exclude:"))
		for _, p := range item.target.Exclude {
			fmt.Fprintf(&b, "  %s\n", p)
		}
	}

	if len(item.target.Include) == 0 && len(item.target.Exclude) == 0 {
		fmt.Fprintf(&b, "\n%s\n", tc.Dim.Render("No include/exclude filters"))
	}

	return b.String()
}

// ---- Overlay renders --------------------------------------------------------

func (m targetListTUIModel) renderModePicker() string {
	var b strings.Builder

	fmt.Fprintf(&b, "\n%s\n", tc.Title.Render("Change mode"))
	fmt.Fprintf(&b, "%s  %s\n\n", tc.Dim.Render("Target:"), m.modePickerTarget)

	for i, mode := range targetSyncModes {
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

func (m targetListTUIModel) renderFilterEditPanel() string {
	var b strings.Builder

	title := capitalize(m.editFilterType)
	fmt.Fprintf(&b, "%s %s\n", tc.Title.Render(title+" patterns"), tc.Dim.Render("("+m.editFilterTarget+")"))
	fmt.Fprintln(&b)

	if len(m.editPatterns) == 0 {
		fmt.Fprintf(&b, "  %s\n", tc.Dim.Render("(empty)"))
	} else {
		for i, p := range m.editPatterns {
			if i == m.editCursor {
				fmt.Fprintf(&b, "  %s %s\n", tc.Cyan.Render(">"), tc.Cyan.Render(p))
			} else {
				fmt.Fprintf(&b, "    %s\n", p)
			}
		}
	}

	if m.editAdding {
		fmt.Fprintln(&b)
		fmt.Fprintf(&b, "  %s\n", m.editInput.View())
	}

	fmt.Fprintln(&b)
	if m.editAdding {
		fmt.Fprintf(&b, "%s\n", tc.Help.Render("Enter confirm  Esc cancel"))
	} else {
		fmt.Fprintf(&b, "%s\n", tc.Help.Render("a add  d delete  esc back"))
	}
	return b.String()
}

// ---- Runner -----------------------------------------------------------------

func runTargetListTUI(mode runMode, cwd string) error {
	var (
		cfg       *config.Config
		projCfg   *config.ProjectConfig
		modeLabel string
	)

	if mode == modeProject {
		modeLabel = "project"
		pc, err := config.LoadProject(cwd)
		if err != nil {
			return err
		}
		if len(pc.Targets) == 0 {
			return targetListProject(cwd)
		}
		projCfg = pc
	} else {
		modeLabel = "global"
		c, err := config.Load()
		if err != nil {
			return err
		}
		if len(c.Targets) == 0 {
			return targetList(false)
		}
		cfg = c
	}

	model := newTargetListTUIModel(modeLabel, cfg, projCfg, cwd)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	m, ok := finalModel.(targetListTUIModel)
	if !ok {
		return nil
	}
	if m.loadErr != nil {
		return m.loadErr
	}
	if m.emptyResult {
		if mode == modeProject {
			return targetListProject(cwd)
		}
		return targetList(false)
	}
	return nil
}
