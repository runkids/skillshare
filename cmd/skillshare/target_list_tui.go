package main

import (
	"cmp"
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
	"github.com/charmbracelet/lipgloss"
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

	// e opens one menu of every setting the target has.
	showEditMenu   bool
	editMenuCursor int

	showKeys bool // ? swaps the detail panel for the full key list

	// Exit-with-action (for destructive ops dispatched after TUI exit)
	action string // "remove" or "" (normal quit)

	// Action feedback
	lastActionMsg string
}

// targetEditOption is one row of the e menu: a setting of the skills or the
// agents side, with the reason when it does nothing.
type targetEditOption struct {
	scope    string // "skills" or "agents"
	action   string // "mode", "naming", "include", "exclude"
	disabled string
}

func (o targetEditOption) label() string {
	return capitalize(o.scope) + " " + o.action
}

func newTargetListTUIModel(
	modeLabel string,
	cfg *config.Config,
	projCfg *config.ProjectConfig,
	cwd string,
) targetListTUIModel {
	delegate := targetListDelegate{}

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

	ei := textinput.New()
	ei.Prompt = "pattern "
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
			skillSync, skillSyncText := targetSkillSyncSummary(resolved, projCfg.EffectiveSkillsSource(cwd))
			items = append(items, targetTUIItem{
				name:          entry.Name,
				target:        resolved,
				displayPath:   projectTargetDisplayPath(entry),
				skillSync:     skillSync,
				skillSyncText: skillSyncText,
				agentConfig:   config.ResourceTargetConfig{Mode: agentSummaryMode(agentSummary), Include: agentSummaryInclude(agentSummary), Exclude: agentSummaryExclude(agentSummary)},
				agentSummary:  agentSummary,
				namingErr:     resolved.NamingModeConfigError(""),
			})
		}
	} else {
		cfg, err := config.LoadWithoutProjects()
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
			skillSync, skillSyncText := targetSkillSyncSummary(t, cfg.EffectiveSkillsSource())
			items = append(items, targetTUIItem{
				name:          name,
				target:        t,
				displayPath:   t.SkillsConfig().Path,
				skillSync:     skillSync,
				skillSyncText: skillSyncText,
				agentConfig:   config.ResourceTargetConfig{Mode: agentSummaryMode(agentSummary), Include: agentSummaryInclude(agentSummary), Exclude: agentSummaryExclude(agentSummary)},
				agentSummary:  agentSummary,
				namingErr:     t.NamingModeConfigError(cfg.Mode),
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
		if m.showEditMenu {
			return m.handleEditMenuKey(msg)
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
	case "esc":
		switch {
		case m.showKeys:
			m.showKeys = false
		case m.filterText != "":
			m.filterText = ""
			m.filterInput.SetValue("")
			m.applyTargetFilter()
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
		m.detailScroll = max(m.detailScroll-8, 0)
		return m, nil
	case "/":
		m.filtering = true
		m.filterInput.Focus()
		m.lastActionMsg = ""
		return m, textinput.Blink
	case "e":
		if item, ok := m.list.SelectedItem().(targetTUIItem); ok {
			options := targetEditOptions(item)
			m.showEditMenu = true
			m.showKeys = false
			m.editMenuCursor = moveEditMenuCursor(options, -1, 1)
			m.lastActionMsg = ""
		}
		return m, nil
	case "d":
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
	case "y", "Y":
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

var targetSyncModes = config.ValidSyncModes // ["merge", "symlink", "copy"]

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
				if scope == "skills" {
					if err := config.TargetNamingModeError(projCfg.Targets[i].SkillsConfig().TargetNaming, newMode); err != nil {
						return "", err
					}
				}
				break
			}
		}
		if err := projCfg.Save(m.cwd); err != nil {
			return "", err
		}
	} else {
		cfg, err := config.LoadWithoutProjects()
		if err != nil {
			return "", err
		}
		t := cfg.Targets[name]
		targetCfg := scopeSetterGlobal(&t, scope)
		targetCfg.Mode = newMode
		if scope == "skills" {
			if err := config.TargetNamingModeError(t.SkillsConfig().TargetNaming, cmp.Or(newMode, cfg.Mode)); err != nil {
				return "", err
			}
		}
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
				if err := config.TargetNamingModeError(newNaming, projCfg.Targets[i].SkillsConfig().Mode); err != nil {
					return "", err
				}
				break
			}
		}
		if err := projCfg.Save(m.cwd); err != nil {
			return "", err
		}
	} else {
		cfg, err := config.LoadWithoutProjects()
		if err != nil {
			return "", err
		}
		t := cfg.Targets[name]
		t.EnsureSkills().TargetNaming = newNaming
		if err := config.TargetNamingModeError(newNaming, cmp.Or(t.SkillsConfig().Mode, cfg.Mode)); err != nil {
			return "", err
		}
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
	case "n":
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
		cfg, err := config.LoadWithoutProjects()
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
		cfg, err := config.LoadWithoutProjects()
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
	bodyHeight := max(m.termHeight-frameChrome, 6)
	count := countNoun(len(m.allItems), "target")
	if m.filterText != "" {
		count = formatNumber(m.matchCount) + " of " + count
	}
	title := renderFrameTitle(m.termWidth, "target", []string{m.modeLabel, count}, nil)
	if !targetSplitActive(m.termWidth) {
		detailHeight := max(bodyHeight-m.list.Height()-1, 4)
		detail := lipgloss.NewStyle().Height(detailHeight).MaxHeight(detailHeight).PaddingLeft(1).
			Render(m.renderRight(m.termWidth-2, detailHeight))
		return title + "\n\n" + m.list.View() + "\n\n" + detail + "\n" + m.renderBottom()
	}
	leftWidth := targetPanelWidth(m.termWidth)
	rightWidth := m.termWidth - leftWidth
	return title + "\n\n" +
		renderFrameSplit(m.list.View(), m.renderRight(rightWidth-2, bodyHeight), leftWidth, rightWidth, bodyHeight) + "\n" +
		m.renderBottom()
}

// renderRight renders the detail panel, or in its place the key list, the
// e menu, a picker or the pattern editor.
func (m targetListTUIModel) renderRight(width, height int) string {
	item, ok := m.list.SelectedItem().(targetTUIItem)
	switch {
	case m.showKeys:
		return renderKeysPanel(targetKeyGroups)
	case !ok:
		return ""
	case m.showEditMenu:
		return m.renderEditMenu(item)
	case m.showModePicker:
		return m.renderModePicker()
	case m.showNamingPicker:
		return m.renderNamingPicker()
	case m.editingFilter:
		return m.renderFilterEditPanel()
	}
	detail, _ := wrapAndScroll(m.renderTargetDetail(item), width, m.detailScroll, height)
	return detail
}

// renderBottom renders the last action's result on the note line and the
// key line. The remove confirmation, the filter input and the open menu's
// keys take over the key line in place.
func (m targetListTUIModel) renderBottom() string {
	note := ""
	if m.lastActionMsg != "" {
		note = "  " + renderTargetActionMsg(m.lastActionMsg)
	}
	var line string
	switch {
	case m.confirming:
		flag, what := "-g", "Backs it up, turns its synced skills into regular folders, and drops it from the config"
		if m.projCfg != nil {
			flag, what = "-p", "Turns its synced skills into regular folders and drops it from the config"
		}
		note = theme.Dim().Render("  " + what)
		line = renderConfirmLine(m.termWidth, "Remove target "+m.confirmTarget+"?", true, "skillshare target remove "+flag+" "+m.confirmTarget)
	case m.filtering:
		line = renderFilterLine(m.termWidth, m.filterInput.View(), m.matchCount)
	case m.editingFilter && m.editAdding:
		line = renderKeyLine(m.termWidth, []keyHint{{"enter", "add"}, {"esc", "cancel"}}, "")
	case m.editingFilter:
		line = renderKeyLine(m.termWidth, []keyHint{{"↑↓", "move"}, {"n", "add"}, {"d", "delete"}, {"esc", "back"}}, "")
	case m.showEditMenu || m.showModePicker || m.showNamingPicker:
		line = renderKeyLine(m.termWidth, []keyHint{{"↑↓", "move"}, {"enter", "choose"}, {"esc", "back"}}, "")
	case m.showKeys:
		line = renderKeyLine(m.termWidth, []keyHint{{"?/esc", "close"}}, "")
	default:
		filter := keyHint{"/", "filter"}
		if m.filterText != "" {
			filter = keyHint{"esc", "clear filter"}
		}
		hints := []keyHint{{"↑↓", "move"}, filter, {"e", "edit"}, {"d", "remove"}, {"?", "keys"}}
		line = renderKeyLine(m.termWidth, hints, framePosition(m.list.Index()+1, m.matchCount))
	}
	return note + "\n" + line
}

// targetKeyGroups lists every key for the ? panel.
var targetKeyGroups = []keyGroup{
	{"Move", []keyHint{
		{"↑↓", "move"},
		{"/", "filter by name"},
		{"ctrl+d/u", "scroll the details"},
		{"esc", "clear the filter, then quit"},
		{"q", "quit"},
	}},
	{"Target", []keyHint{
		{"e", "change mode, naming, include or exclude"},
		{"d", "remove the target"},
	}},
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

// ---- Layout -----------------------------------------------------------------

func targetSplitActive(termWidth int) bool {
	return termWidth >= tuiMinSplitWidth
}

func targetPanelWidth(termWidth int) int {
	w := termWidth * 36 / 100
	return max(min(w, 40), 26)
}

func (m *targetListTUIModel) syncTargetListSize() {
	bodyHeight := max(m.termHeight-frameChrome, 6)
	if targetSplitActive(m.termWidth) {
		m.list.SetSize(targetPanelWidth(m.termWidth), bodyHeight)
		return
	}
	m.list.SetSize(m.termWidth, max(bodyHeight/2, 4))
}

// ---- Detail panel -----------------------------------------------------------

func (m targetListTUIModel) renderTargetDetail(item targetTUIItem) string {
	var b strings.Builder
	row := func(label, value string) {
		b.WriteString(theme.Dim().Render(fmt.Sprintf("%-9s", label)) + value + "\n")
	}
	heading := func(text string) {
		b.WriteString("\n" + theme.Primary().Bold(true).Render(text) + "\n")
	}

	b.WriteString(theme.Primary().Bold(true).Render(item.name) + "\n")

	sc := item.target.SkillsConfig()
	displayPath := item.displayPath
	if displayPath == "" {
		displayPath = sc.Path
	}
	heading("Skills")
	row("Path", shortenPath(displayPath))
	if sc.IsEnabled() {
		row("Mode", sync.EffectiveMode(sc.Mode))
		row("Naming", config.EffectiveTargetNaming(sc.TargetNaming))
		if item.namingErr != nil {
			row("Warning", theme.Warning().Render(item.namingErr.Error()))
		}
	}
	row("Sync", item.skillSync)
	// Filters are kept for turning skills back on but do nothing while off.
	if sc.IsEnabled() && len(sc.Include) > 0 {
		row("Include", strings.Join(sc.Include, ", "))
	}
	if sc.IsEnabled() && len(sc.Exclude) > 0 {
		row("Exclude", strings.Join(sc.Exclude, ", "))
	}

	if item.agentSummary != nil {
		agentPath := item.agentSummary.DisplayPath
		if agentPath == "" {
			agentPath = item.agentSummary.Path
		}
		heading("Agents")
		row("Path", shortenPath(agentPath))
		row("Mode", item.agentSummary.Mode)
		row("Sync", formatTargetAgentSyncSummary(item.agentSummary))
		switch {
		case item.agentSummary.Mode == "symlink":
			if len(item.agentSummary.Include) > 0 || len(item.agentSummary.Exclude) > 0 {
				row("Filters", theme.Dim().Render("ignored in symlink mode"))
			}
		default:
			if len(item.agentSummary.Include) > 0 {
				row("Include", strings.Join(item.agentSummary.Include, ", "))
			}
			if len(item.agentSummary.Exclude) > 0 {
				row("Exclude", strings.Join(item.agentSummary.Exclude, ", "))
			}
		}
	}
	return b.String()
}

// skillsOffSummary is the skills sync summary of a target with skills off.
const skillsOffSummary = "skills off (not synced)"

// targetSkillSyncSummary returns the skills sync summary twice: as the TUI
// and JSON show it, and as the plain list shows it, with zero counts left out.
func targetSkillSyncSummary(target config.TargetConfig, sourcePath string) (summary, text string) {
	sc := target.SkillsConfig()
	if !sc.IsEnabled() {
		return skillsOffSummary, skillsOffSummary
	}
	return buildTargetSkillSyncSummary(sc.Path, sourcePath, sc.Mode)
}

func buildTargetSkillSyncSummary(targetPath, sourcePath, mode string) (summary, text string) {
	var status fmt.Stringer
	var synced, local int
	label := "managed"
	switch sync.EffectiveMode(mode) {
	case "copy":
		status, synced, local = sync.CheckStatusCopy(targetPath)
	case "merge":
		status, synced, local = sync.CheckStatusMerge(targetPath, sourcePath)
		label = "shared"
	default:
		s := sync.CheckStatus(targetPath, sourcePath).String()
		return s, s
	}
	summary = fmt.Sprintf("%s (%d %s, %d local)", status, synced, label, local)
	text = status.String()
	if counts := joinAgentCounts(synced, label, local); counts != "" {
		text += " · " + counts
	}
	return summary, text
}

// ---- Overlay renders --------------------------------------------------------

func (m targetListTUIModel) renderModePicker() string {
	var b strings.Builder
	b.WriteString(theme.Primary().Bold(true).Render(capitalize(m.modePickerScope)+" mode") + theme.Dim().Render(" · "+m.modePickerTarget) + "\n\n")
	for i, mode := range targetSyncModes {
		desc := map[string]string{"merge": "per-file symlinks", "copy": "file copies", "symlink": "directory symlink"}[mode]
		b.WriteString(renderPickerRow(mode, desc, i == m.modeCursor))
	}
	return b.String()
}

// renderEditMenu renders the e menu: every setting the target has.
func (m targetListTUIModel) renderEditMenu(item targetTUIItem) string {
	var b strings.Builder
	b.WriteString(theme.Primary().Bold(true).Render("Edit "+item.name) + "\n\n")
	for i, option := range targetEditOptions(item) {
		label := option.label()
		switch {
		case option.disabled != "":
			b.WriteString("  " + theme.Dim().Render(label+" · "+option.disabled) + "\n")
		case i == m.editMenuCursor:
			b.WriteString(theme.Accent().Render("› "+label) + "\n")
		default:
			b.WriteString("  " + label + "\n")
		}
	}
	return b.String()
}

func (m targetListTUIModel) renderNamingPicker() string {
	var b strings.Builder
	b.WriteString(theme.Primary().Bold(true).Render("Skills naming") + theme.Dim().Render(" · "+m.namingPickerTarget) + "\n\n")
	for i, naming := range config.ValidTargetNamings {
		desc := map[string]string{"flat": "flattened __ names", "standard": "SKILL.md name", "prefixed": "<repo>-<name>, copy mode only"}[naming]
		b.WriteString(renderPickerRow(naming, desc, i == m.namingCursor))
	}
	return b.String()
}

func (m targetListTUIModel) renderFilterEditPanel() string {
	var b strings.Builder
	b.WriteString(theme.Primary().Bold(true).Render(capitalize(m.editFilterScope)+" "+m.editFilterType) + theme.Dim().Render(" · "+m.editFilterTarget) + "\n\n")
	if len(m.editPatterns) == 0 {
		b.WriteString(theme.Dim().Render("  No patterns yet. Press n to add one.") + "\n")
	}
	for i, p := range m.editPatterns {
		if i == m.editCursor && !m.editAdding {
			b.WriteString(theme.Accent().Render("› "+p) + "\n")
		} else {
			b.WriteString("  " + p + "\n")
		}
	}
	if m.editAdding {
		b.WriteString("\n  " + m.editInput.View() + "\n")
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
		c, err := config.LoadWithoutProjects()
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

// targetEditOptions lists the settings e can change: mode, naming and the
// filters of the skills side, and mode and filters of the agents side when
// the target has agents.
func targetEditOptions(item targetTUIItem) []targetEditOption {
	options := []targetEditOption{
		{scope: "skills", action: "mode"},
		{scope: "skills", action: "naming"},
		{scope: "skills", action: "include"},
		{scope: "skills", action: "exclude"},
	}
	if item.agentSummary == nil {
		return options
	}
	options = append(options, targetEditOption{scope: "agents", action: "mode"})
	for _, action := range []string{"include", "exclude"} {
		option := targetEditOption{scope: "agents", action: action}
		if item.agentSummary.Mode == "symlink" {
			option.disabled = "ignored in symlink mode"
		}
		options = append(options, option)
	}
	return options
}

func (m targetListTUIModel) handleEditMenuKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(targetTUIItem)
	if !ok {
		m.showEditMenu = false
		return m, nil
	}
	options := targetEditOptions(item)

	switch msg.String() {
	case "q", "esc":
		m.showEditMenu = false
		return m, nil
	case "up", "k":
		m.editMenuCursor = moveEditMenuCursor(options, m.editMenuCursor, -1)
		return m, nil
	case "down", "j":
		m.editMenuCursor = moveEditMenuCursor(options, m.editMenuCursor, 1)
		return m, nil
	case "enter":
		m.showEditMenu = false
		option := options[m.editMenuCursor]
		cfg := itemConfigForScope(item, option.scope)
		switch option.action {
		case "mode":
			return m.openModePickerForScope(item.name, cfg, option.scope)
		case "naming":
			return m.openNamingPicker(item.name, item.target)
		default:
			patterns := cfg.Include
			if option.action == "exclude" {
				patterns = cfg.Exclude
			}
			return m.openFilterEditForScope(item.name, option.scope, option.action, patterns)
		}
	}
	return m, nil
}

// moveEditMenuCursor moves from current by delta, skipping options that do
// nothing; it stays put at either end. A current of -1 finds the first
// usable option.
func moveEditMenuCursor(options []targetEditOption, current, delta int) int {
	for next := current + delta; next >= 0 && next < len(options); next += delta {
		if options[next].disabled == "" {
			return next
		}
	}
	return max(current, 0)
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
