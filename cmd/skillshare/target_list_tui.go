package main

import (
	"fmt"
	"sort"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/targetsummary"
	"skillshare/internal/theme"

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
	modePickerScope  string // "skills" or "agents"
	modeCursor       int

	// Naming picker overlay
	showNamingPicker   bool
	namingPickerTarget string
	namingCursor       int

	// Include/Exclude edit sub-panel
	editingFilter    bool   // true when in I/E edit mode
	editFilterType   string // "include" or "exclude"
	editFilterTarget string // target name being edited
	editFilterScope  string // "skills" or "agents"
	editPatterns     []string
	editCursor       int // selected pattern index
	editAdding       bool
	editInput        textinput.Model

	// Remove confirmation overlay
	confirming    bool
	confirmTarget string

	// Scope picker overlay for M/I/E when both skills and agents are available.
	showScopePicker   bool
	scopePickerTarget string
	scopePickerAction string // "mode", "include", "exclude"
	scopePickerCursor int

	// Exit-with-action (for destructive ops dispatched after TUI exit)
	action string // "remove" or "" (normal quit)

	// Action feedback
	lastActionMsg string
}

type targetScopeOption struct {
	scope    string
	enabled  bool
	disabled string
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
	l.Styles.Title = theme.Title()
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = theme.Accent()

	fi := textinput.New()
	fi.Prompt = "/ "
	fi.PromptStyle = theme.Accent()
	fi.Cursor.Style = theme.Accent()
	fi.Placeholder = "filter by name"

	ei := textinput.New()
	ei.Prompt = "> pattern: "
	ei.PromptStyle = theme.Accent()
	ei.Cursor.Style = theme.Accent()
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
		resolvedTargets, err := config.ResolveProjectTargets(cwd, projCfg)
		if err != nil {
			return nil, err
		}
		agentBuilder, err := targetsummary.NewProjectBuilder(projCfg.EffectiveAgentsSource(cwd), cwd)
		if err != nil {
			return nil, err
		}
		for _, entry := range projCfg.Targets {
			resolved, ok := resolvedTargets[entry.Name]
			if !ok {
				continue
			}
			agentSummary, err := agentBuilder.ProjectTarget(entry)
			if err != nil {
				return nil, err
			}
			items = append(items, targetTUIItem{
				name:         entry.Name,
				target:       resolved,
				displayPath:  projectTargetDisplayPath(entry),
				skillSync:    buildTargetSkillSyncSummary(resolved.SkillsConfig().Path, projCfg.EffectiveSkillsSource(cwd), resolved.SkillsConfig().Mode),
				agentConfig:  config.ResourceTargetConfig{Mode: agentSummaryMode(agentSummary), Include: agentSummaryInclude(agentSummary), Exclude: agentSummaryExclude(agentSummary)},
				agentSummary: agentSummary,
			})
		}
	} else {
		cfg, err := config.Load()
		if err != nil {
			return nil, err
		}
		agentBuilder, err := targetsummary.NewGlobalBuilder(cfg)
		if err != nil {
			return nil, err
		}
		for name, t := range cfg.Targets {
			agentSummary, err := agentBuilder.GlobalTarget(name, t)
			if err != nil {
				return nil, err
			}
			items = append(items, targetTUIItem{
				name:         name,
				target:       t,
				displayPath:  t.SkillsConfig().Path,
				skillSync:    buildTargetSkillSyncSummary(t.SkillsConfig().Path, cfg.Source, t.SkillsConfig().Mode),
				agentConfig:  config.ResourceTargetConfig{Mode: agentSummaryMode(agentSummary), Include: agentSummaryInclude(agentSummary), Exclude: agentSummaryExclude(agentSummary)},
				agentSummary: agentSummary,
			})
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
		if m.showScopePicker {
			return m.handleScopePickerKey(msg)
		}
		if m.showNamingPicker {
			return m.handleNamingPickerKey(msg)
		}
		if m.confirming {
			return m.handleConfirmKey(msg)
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
			if item.agentSummary != nil {
				return m.openScopePicker(item, "mode")
			}
			return m.openModePicker(item.name, item.target)
		}
		return m, nil
	case "N":
		if item, ok := m.list.SelectedItem().(targetTUIItem); ok {
			return m.openNamingPicker(item.name, item.target)
		}
		return m, nil
	case "I":
		if item, ok := m.list.SelectedItem().(targetTUIItem); ok {
			if item.agentSummary != nil {
				return m.openScopePicker(item, "include")
			}
			return m.openFilterEdit(item.name, "include", item.target.SkillsConfig().Include)
		}
		return m, nil
	case "E":
		if item, ok := m.list.SelectedItem().(targetTUIItem); ok {
			if item.agentSummary != nil {
				return m.openScopePicker(item, "exclude")
			}
			return m.openFilterEdit(item.name, "exclude", item.target.SkillsConfig().Exclude)
		}
		return m, nil
	case "R":
		if item, ok := m.list.SelectedItem().(targetTUIItem); ok {
			m.confirming = true
			m.confirmTarget = item.name
			m.lastActionMsg = ""
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

// ─── Remove Confirmation ─────────────────────────────────────────────

func (m targetListTUIModel) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		m.action = "remove"
		m.quitting = true
		return m, tea.Quit
	case "n", "N", "esc", "q":
		m.confirming = false
		m.confirmTarget = ""
		return m, nil
	}
	return m, nil
}

// ─── Mode Picker ─────────────────────────────────────────────────────

var targetSyncModes = config.ExtraSyncModes // ["merge", "copy", "symlink"]

func (m targetListTUIModel) openModePicker(name string, target config.TargetConfig) (tea.Model, tea.Cmd) {
	return m.openModePickerForScope(name, target.SkillsConfig(), "skills")
}

func (m targetListTUIModel) openModePickerForScope(name string, currentConfig config.ResourceTargetConfig, scope string) (tea.Model, tea.Cmd) {
	m.showModePicker = true
	m.modePickerTarget = name
	m.modePickerScope = scope
	m.modeCursor = 0
	current := sync.EffectiveMode(currentConfig.Mode)
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
		scope := m.modePickerScope
		return m, func() tea.Msg {
			msg, err := m.doSetTargetMode(name, scope, newMode)
			return targetListActionDoneMsg{msg: msg, err: err}
		}
	}
	return m, nil
}

func (m targetListTUIModel) doSetTargetMode(name, scope, newMode string) (string, error) {
	if m.projCfg != nil {
		projCfg, err := config.LoadProject(m.cwd)
		if err != nil {
			return "", err
		}
		for i, entry := range projCfg.Targets {
			if entry.Name == name {
				targetCfg := scopeSetterProject(&projCfg.Targets[i], scope)
				targetCfg.Mode = newMode
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
		targetCfg := scopeSetterGlobal(&t, scope)
		targetCfg.Mode = newMode
		cfg.Targets[name] = t
		if err := cfg.Save(); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("✓ Set %s %s mode to %s", name, scope, newMode), nil
}

// ─── Naming Picker ──────────────────────────────────────────────────

func (m targetListTUIModel) openNamingPicker(name string, target config.TargetConfig) (tea.Model, tea.Cmd) {
	m.showNamingPicker = true
	m.namingPickerTarget = name
	m.namingCursor = 0
	current := config.EffectiveTargetNaming(target.SkillsConfig().TargetNaming)
	for i, n := range config.ValidTargetNamings {
		if n == current {
			m.namingCursor = i
			break
		}
	}
	m.lastActionMsg = ""
	return m, nil
}

func (m targetListTUIModel) handleNamingPickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.showNamingPicker = false
		return m, nil
	case "up", "k":
		if m.namingCursor > 0 {
			m.namingCursor--
		}
		return m, nil
	case "down", "j":
		if m.namingCursor < len(config.ValidTargetNamings)-1 {
			m.namingCursor++
		}
		return m, nil
	case "enter":
		m.showNamingPicker = false
		newNaming := config.ValidTargetNamings[m.namingCursor]
		name := m.namingPickerTarget
		return m, func() tea.Msg {
			msg, err := m.doSetTargetNaming(name, newNaming)
			return targetListActionDoneMsg{msg: msg, err: err}
		}
	}
	return m, nil
}

func (m targetListTUIModel) doSetTargetNaming(name, newNaming string) (string, error) {
	if m.projCfg != nil {
		projCfg, err := config.LoadProject(m.cwd)
		if err != nil {
			return "", err
		}
		for i, entry := range projCfg.Targets {
			if entry.Name == name {
				projCfg.Targets[i].EnsureSkills().TargetNaming = newNaming
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
		t.EnsureSkills().TargetNaming = newNaming
		cfg.Targets[name] = t
		if err := cfg.Save(); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("✓ Set %s target naming to %s", name, newNaming), nil
}

// ─── Include/Exclude Edit Sub-Panel ──────────────────────────────────

func (m targetListTUIModel) openFilterEdit(name, filterType string, patterns []string) (tea.Model, tea.Cmd) {
	return m.openFilterEditForScope(name, "skills", filterType, patterns)
}

func (m targetListTUIModel) openFilterEditForScope(name, scope, filterType string, patterns []string) (tea.Model, tea.Cmd) {
	m.editingFilter = true
	m.editFilterType = filterType
	m.editFilterTarget = name
	m.editFilterScope = scope
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
			scope := m.editFilterScope
			filterType := m.editFilterType
			return m, func() tea.Msg {
				msg, err := m.doRemovePattern(name, scope, filterType, pattern)
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
		scope := m.editFilterScope
		filterType := m.editFilterType
		return m, func() tea.Msg {
			msg, err := m.doAddPattern(name, scope, filterType, pattern)
			return targetListActionDoneMsg{msg: msg, err: err}
		}
	}
	var cmd tea.Cmd
	m.editInput, cmd = m.editInput.Update(msg)
	return m, cmd
}

func (m targetListTUIModel) doAddPattern(name, scope, filterType, pattern string) (string, error) {
	if m.projCfg != nil {
		projCfg, err := config.LoadProject(m.cwd)
		if err != nil {
			return "", err
		}
		for i, entry := range projCfg.Targets {
			if entry.Name == name {
				targetCfg := scopeSetterProject(&projCfg.Targets[i], scope)
				if filterType == "include" {
					targetCfg.Include = append(targetCfg.Include, pattern)
				} else {
					targetCfg.Exclude = append(targetCfg.Exclude, pattern)
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
		targetCfg := scopeSetterGlobal(&t, scope)
		if filterType == "include" {
			targetCfg.Include = append(targetCfg.Include, pattern)
		} else {
			targetCfg.Exclude = append(targetCfg.Exclude, pattern)
		}
		cfg.Targets[name] = t
		if err := cfg.Save(); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("✓ Added %s %s pattern: %s", scope, filterType, pattern), nil
}

func (m targetListTUIModel) doRemovePattern(name, scope, filterType, pattern string) (string, error) {
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
				targetCfg := scopeSetterProject(&projCfg.Targets[i], scope)
				if filterType == "include" {
					targetCfg.Include = removeFromSlice(targetCfg.Include, pattern)
				} else {
					targetCfg.Exclude = removeFromSlice(targetCfg.Exclude, pattern)
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
		targetCfg := scopeSetterGlobal(&t, scope)
		if filterType == "include" {
			targetCfg.Include = removeFromSlice(targetCfg.Include, pattern)
		} else {
			targetCfg.Exclude = removeFromSlice(targetCfg.Exclude, pattern)
		}
		cfg.Targets[name] = t
		if err := cfg.Save(); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("✓ Removed %s %s pattern: %s", scope, filterType, pattern), nil
}

// ---- View -------------------------------------------------------------------

func (m targetListTUIModel) View() string {
	if m.quitting {
		return ""
	}
	if m.loading {
		return fmt.Sprintf("\n  %s Loading targets...\n", m.loadSpinner.View())
	}
	if m.confirming {
		return m.renderConfirmOverlay()
	}
	if m.showModePicker {
		return m.renderModePicker()
	}
	if m.showScopePicker {
		return m.renderScopePicker()
	}
	if m.showNamingPicker {
		return m.renderNamingPicker()
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
		return theme.Success().Render(msg)
	}
	if strings.HasPrefix(msg, "✗") {
		return theme.Danger().Render(msg)
	}
	return theme.Warning().Render(msg)
}

func (m targetListTUIModel) renderTargetHelp(scrollInfo string) string {
	helpText := "↑↓ navigate  / filter  Ctrl+d/u scroll  M mode(sk/ag)  N naming(sk)  I include(sk/ag)  E exclude(sk/ag)  R remove  q quit"
	if m.filtering {
		helpText = "Enter lock  Esc clear  q quit"
	}
	return theme.Dim().MarginLeft(2).Render(appendScrollInfo(helpText, scrollInfo))
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

	fmt.Fprintf(&b, "%s\n\n", theme.Title().Render(item.name))

	sc := item.target.SkillsConfig()
	displayPath := item.displayPath
	if displayPath == "" {
		displayPath = sc.Path
	}
	fmt.Fprintf(&b, "%s\n", theme.Dim().Render("Skills:"))
	fmt.Fprintf(&b, "%s  %s\n", theme.Dim().Render("Path:"), shortenPath(displayPath))
	fmt.Fprintf(&b, "%s  %s\n", theme.Dim().Render("Mode:"), sync.EffectiveMode(sc.Mode))
	fmt.Fprintf(&b, "%s  %s\n", theme.Dim().Render("Naming:"), config.EffectiveTargetNaming(sc.TargetNaming))
	fmt.Fprintf(&b, "%s  %s\n", theme.Dim().Render("Sync:"), item.skillSync)

	if len(sc.Include) > 0 {
		fmt.Fprintf(&b, "\n%s\n", theme.Dim().Render("Include:"))
		for _, p := range sc.Include {
			fmt.Fprintf(&b, "  %s\n", p)
		}
	}
	if len(sc.Exclude) > 0 {
		fmt.Fprintf(&b, "\n%s\n", theme.Dim().Render("Exclude:"))
		for _, p := range sc.Exclude {
			fmt.Fprintf(&b, "  %s\n", p)
		}
	}

	if len(sc.Include) == 0 && len(sc.Exclude) == 0 {
		fmt.Fprintf(&b, "\n%s\n", theme.Dim().Render("No include/exclude filters"))
	}

	if item.agentSummary != nil {
		agentPath := item.agentSummary.DisplayPath
		if agentPath == "" {
			agentPath = item.agentSummary.Path
		}

		fmt.Fprintf(&b, "\n%s\n", theme.Dim().Render("Agents:"))
		fmt.Fprintf(&b, "%s  %s\n", theme.Dim().Render("Path:"), shortenPath(agentPath))
		fmt.Fprintf(&b, "%s  %s\n", theme.Dim().Render("Mode:"), item.agentSummary.Mode)
		fmt.Fprintf(&b, "%s  %s\n", theme.Dim().Render("Sync:"), formatTargetAgentSyncSummary(item.agentSummary))

		if item.agentSummary.Mode == "symlink" {
			fmt.Fprintf(&b, "\n%s\n", theme.Dim().Render("Agent include/exclude filters ignored in symlink mode"))
		} else if len(item.agentSummary.Include) > 0 {
			fmt.Fprintf(&b, "\n%s\n", theme.Dim().Render("Agent Include:"))
			for _, p := range item.agentSummary.Include {
				fmt.Fprintf(&b, "  %s\n", p)
			}
		}
		if item.agentSummary.Mode != "symlink" && len(item.agentSummary.Exclude) > 0 {
			fmt.Fprintf(&b, "\n%s\n", theme.Dim().Render("Agent Exclude:"))
			for _, p := range item.agentSummary.Exclude {
				fmt.Fprintf(&b, "  %s\n", p)
			}
		}
		if item.agentSummary.Mode != "symlink" && len(item.agentSummary.Include) == 0 && len(item.agentSummary.Exclude) == 0 {
			fmt.Fprintf(&b, "\n%s\n", theme.Dim().Render("No agent include/exclude filters"))
		}
	}

	return b.String()
}

func buildTargetSkillSyncSummary(targetPath, sourcePath, mode string) string {
	switch sync.EffectiveMode(mode) {
	case "copy":
		status, managed, local := sync.CheckStatusCopy(targetPath)
		return fmt.Sprintf("%s (%d managed, %d local)", status, managed, local)
	case "merge":
		status, linked, local := sync.CheckStatusMerge(targetPath, sourcePath)
		return fmt.Sprintf("%s (%d shared, %d local)", status, linked, local)
	default:
		return sync.CheckStatus(targetPath, sourcePath).String()
	}
}

// ---- Overlay renders --------------------------------------------------------

func (m targetListTUIModel) renderConfirmOverlay() string {
	flag := "-g"
	if m.projCfg != nil {
		flag = "-p"
	}
	cmd := fmt.Sprintf("skillshare target remove %s %s", flag, m.confirmTarget)
	return fmt.Sprintf("\n  %s\n\n  → %s\n\n  Proceed? [Y/n] ",
		theme.Danger().Render("Remove target "+m.confirmTarget+"?"), cmd)
}

func (m targetListTUIModel) renderModePicker() string {
	var b strings.Builder

	fmt.Fprintf(&b, "\n%s\n", theme.Title().Render("Change "+m.modePickerScope+" mode"))
	fmt.Fprintf(&b, "%s  %s\n\n", theme.Dim().Render("Target:"), m.modePickerTarget)

	for i, mode := range targetSyncModes {
		cursor := "  "
		if i == m.modeCursor {
			cursor = theme.Accent().Render(">") + " "
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
			fmt.Fprintf(&b, "%s%s%s\n", cursor, theme.Accent().Render(mode), theme.Dim().Render(desc))
		} else {
			fmt.Fprintf(&b, "%s%s%s\n", cursor, mode, theme.Dim().Render(desc))
		}
	}

	fmt.Fprintf(&b, "\n%s\n", theme.Dim().MarginLeft(2).Render("↑↓ select  Enter confirm  Esc cancel"))
	return b.String()
}

func (m targetListTUIModel) renderScopePicker() string {
	var b strings.Builder
	item, ok := m.list.SelectedItem().(targetTUIItem)
	if !ok {
		return ""
	}
	options := targetScopeOptions(item, m.scopePickerAction)

	fmt.Fprintf(&b, "\n%s\n", theme.Title().Render("Choose resource"))
	fmt.Fprintf(&b, "%s  %s\n", theme.Dim().Render("Target:"), m.scopePickerTarget)
	fmt.Fprintf(&b, "%s  %s\n\n", theme.Dim().Render("Action:"), m.scopePickerAction)

	for i, option := range options {
		cursor := "  "
		if i == m.scopePickerCursor {
			cursor = theme.Accent().Render(">") + " "
		}
		label := capitalize(option.scope)
		if option.enabled {
			if i == m.scopePickerCursor {
				fmt.Fprintf(&b, "%s%s\n", cursor, theme.Accent().Render(label))
			} else {
				fmt.Fprintf(&b, "%s%s\n", cursor, label)
			}
			continue
		}
		fmt.Fprintf(&b, "%s%s%s\n", cursor, theme.Dim().Render(label), theme.Dim().Render(" ("+option.disabled+")"))
	}

	fmt.Fprintf(&b, "\n%s\n", theme.Dim().MarginLeft(2).Render("↑↓ select  Enter confirm  Esc cancel"))
	return b.String()
}

func (m targetListTUIModel) renderNamingPicker() string {
	var b strings.Builder

	fmt.Fprintf(&b, "\n%s\n", theme.Title().Render("Change target naming"))
	fmt.Fprintf(&b, "%s  %s\n\n", theme.Dim().Render("Target:"), m.namingPickerTarget)

	for i, naming := range config.ValidTargetNamings {
		cursor := "  "
		if i == m.namingCursor {
			cursor = theme.Accent().Render(">") + " "
		}
		var desc string
		switch naming {
		case "flat":
			desc = " (flattened __ names)"
		case "standard":
			desc = " (SKILL.md name)"
		}
		if i == m.namingCursor {
			fmt.Fprintf(&b, "%s%s%s\n", cursor, theme.Accent().Render(naming), theme.Dim().Render(desc))
		} else {
			fmt.Fprintf(&b, "%s%s%s\n", cursor, naming, theme.Dim().Render(desc))
		}
	}

	fmt.Fprintf(&b, "\n%s\n", theme.Dim().MarginLeft(2).Render("↑↓ select  Enter confirm  Esc cancel"))
	return b.String()
}

func (m targetListTUIModel) renderFilterEditPanel() string {
	var b strings.Builder

	title := capitalize(m.editFilterType)
	fmt.Fprintf(&b, "%s %s\n", theme.Title().Render(title+" "+m.editFilterScope+" patterns"), theme.Dim().Render("("+m.editFilterTarget+")"))
	fmt.Fprintln(&b)

	if len(m.editPatterns) == 0 {
		fmt.Fprintf(&b, "  %s\n", theme.Dim().Render("(empty)"))
	} else {
		for i, p := range m.editPatterns {
			if i == m.editCursor {
				fmt.Fprintf(&b, "  %s %s\n", theme.Accent().Render(">"), theme.Accent().Render(p))
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
		fmt.Fprintf(&b, "%s\n", theme.Dim().MarginLeft(2).Render("Enter confirm  Esc cancel"))
	} else {
		fmt.Fprintf(&b, "%s\n", theme.Dim().MarginLeft(2).Render("a add  d delete  esc back"))
	}
	return b.String()
}

// ---- Runner -----------------------------------------------------------------

func runTargetListTUI(mode runMode, cwd string) (string, string, error) {
	var (
		cfg       *config.Config
		projCfg   *config.ProjectConfig
		modeLabel string
	)

	if mode == modeProject {
		modeLabel = "project"
		pc, err := config.LoadProject(cwd)
		if err != nil {
			return "", "", err
		}
		if len(pc.Targets) == 0 {
			return "", "", targetListProject(cwd)
		}
		projCfg = pc
	} else {
		modeLabel = "global"
		c, err := config.Load()
		if err != nil {
			return "", "", err
		}
		if len(c.Targets) == 0 {
			return "", "", targetList(false)
		}
		cfg = c
	}

	model := newTargetListTUIModel(modeLabel, cfg, projCfg, cwd)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	finalModel, err := p.Run()
	if err != nil {
		return "", "", err
	}

	m, ok := finalModel.(targetListTUIModel)
	if !ok {
		return "", "", nil
	}
	if m.loadErr != nil {
		return "", "", m.loadErr
	}
	if m.emptyResult {
		if mode == modeProject {
			return "", "", targetListProject(cwd)
		}
		return "", "", targetList(false)
	}
	return m.action, m.confirmTarget, nil
}

func (m targetListTUIModel) openScopePicker(item targetTUIItem, action string) (tea.Model, tea.Cmd) {
	m.showScopePicker = true
	m.scopePickerTarget = item.name
	m.scopePickerAction = action
	m.scopePickerCursor = firstEnabledScopeOption(targetScopeOptions(item, action))
	m.lastActionMsg = ""
	return m, nil
}

func (m targetListTUIModel) handleScopePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(targetTUIItem)
	if !ok {
		m.showScopePicker = false
		return m, nil
	}
	options := targetScopeOptions(item, m.scopePickerAction)

	switch msg.String() {
	case "q", "esc":
		m.showScopePicker = false
		return m, nil
	case "up", "k":
		m.scopePickerCursor = moveScopePickerCursor(options, m.scopePickerCursor, -1)
		return m, nil
	case "down", "j":
		m.scopePickerCursor = moveScopePickerCursor(options, m.scopePickerCursor, 1)
		return m, nil
	case "enter":
		if len(options) == 0 || !options[m.scopePickerCursor].enabled {
			m.showScopePicker = false
			m.lastActionMsg = "✗ Agents include/exclude filters are ignored in symlink mode"
			return m, nil
		}
		m.showScopePicker = false
		scope := options[m.scopePickerCursor].scope
		switch m.scopePickerAction {
		case "mode":
			return m.openModePickerForScope(item.name, itemConfigForScope(item, scope), scope)
		case "include":
			return m.openFilterEditForScope(item.name, scope, "include", itemConfigForScope(item, scope).Include)
		case "exclude":
			return m.openFilterEditForScope(item.name, scope, "exclude", itemConfigForScope(item, scope).Exclude)
		}
		return m, nil
	}
	return m, nil
}

func targetScopeOptions(item targetTUIItem, action string) []targetScopeOption {
	options := []targetScopeOption{{scope: "skills", enabled: true}}
	if item.agentSummary == nil {
		return options
	}

	option := targetScopeOption{scope: "agents", enabled: true}
	if (action == "include" || action == "exclude") && item.agentSummary.Mode == "symlink" {
		option.enabled = false
		option.disabled = "ignored in symlink mode"
	}
	return append(options, option)
}

func firstEnabledScopeOption(options []targetScopeOption) int {
	for i, option := range options {
		if option.enabled {
			return i
		}
	}
	return 0
}

func moveScopePickerCursor(options []targetScopeOption, current, delta int) int {
	if len(options) == 0 {
		return 0
	}
	if current < 0 || current >= len(options) {
		current = firstEnabledScopeOption(options)
	}
	next := current
	for {
		candidate := next + delta
		if candidate < 0 || candidate >= len(options) {
			return current
		}
		next = candidate
		if options[next].enabled {
			return next
		}
	}
}

func itemConfigForScope(item targetTUIItem, scope string) config.ResourceTargetConfig {
	if scope == "agents" {
		return item.agentConfig
	}
	return item.target.SkillsConfig()
}

func scopeSetterGlobal(target *config.TargetConfig, scope string) *config.ResourceTargetConfig {
	if scope == "agents" {
		return target.EnsureAgents()
	}
	return target.EnsureSkills()
}

func scopeSetterProject(target *config.ProjectTargetEntry, scope string) *config.ResourceTargetConfig {
	if scope == "agents" {
		return target.EnsureAgents()
	}
	return target.EnsureSkills()
}

func agentSummaryMode(summary *targetsummary.AgentSummary) string {
	if summary == nil {
		return ""
	}
	return summary.Mode
}

func agentSummaryInclude(summary *targetsummary.AgentSummary) []string {
	if summary == nil {
		return nil
	}
	return append([]string(nil), summary.Include...)
}

func agentSummaryExclude(summary *targetsummary.AgentSummary) []string {
	if summary == nil {
		return nil
	}
	return append([]string(nil), summary.Exclude...)
}
