package main

import (
	"fmt"
	"sort"

	"skillshare/internal/config"

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
	cfg        *config.Config
	projCfg    *config.ProjectConfig
	cwd        string
	configPath string

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
	cwd, configPath string,
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
		configPath:  configPath,
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
		var items []targetTUIItem
		if m.projCfg != nil {
			projCfg, err := config.LoadProject(m.cwd)
			if err != nil {
				return targetListLoadedMsg{err: err}
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
				return targetListLoadedMsg{err: err}
			}
			for name, t := range cfg.Targets {
				items = append(items, targetTUIItem{name: name, target: t})
			}
		}
		sort.Slice(items, func(i, j int) bool {
			return items[i].name < items[j].name
		})
		return targetListLoadedMsg{items: items}
	}
}

// ---- Stub Update & View -----------------------------------------------------

func (m targetListTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m targetListTUIModel) View() string {
	if m.quitting {
		return ""
	}
	if m.loading {
		return fmt.Sprintf("\n  %s Loading targets...\n", m.loadSpinner.View())
	}
	return "target list TUI placeholder"
}

// ---- Runner -----------------------------------------------------------------

func runTargetListTUI(mode runMode, cwd string) error {
	var (
		cfg        *config.Config
		projCfg    *config.ProjectConfig
		configPath string
		modeLabel  string
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
		configPath = config.ProjectConfigPath(cwd)
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
		configPath = config.ConfigPath()
	}

	model := newTargetListTUIModel(modeLabel, cfg, projCfg, cwd, configPath)
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
