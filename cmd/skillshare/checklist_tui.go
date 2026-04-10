package main

import (
	"fmt"
	"strings"

	"skillshare/internal/theme"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// checklistItemData holds the input data for a single checklist item.
type checklistItemData struct {
	label       string
	desc        string
	preSelected bool
}

// checklistConfig configures the checklist TUI.
type checklistConfig struct {
	title        string
	header       string // optional: rendered above the list (e.g. wizard breadcrumbs)
	items        []checklistItemData
	singleSelect bool   // true = radio behaviour (only one can be selected)
	itemName     string // status bar name (e.g. "target", "agent")
}

// checklistItem is a bubbles list.Item for the checklist TUI.
type checklistItem struct {
	idx          int
	label        string
	desc         string
	selected     bool
	singleSelect bool
}

func (i checklistItem) Title() string {
	var indicator string
	if i.singleSelect {
		if i.selected {
			indicator = "●"
		} else {
			indicator = "○"
		}
	} else {
		if i.selected {
			indicator = "[x]"
		} else {
			indicator = "[ ]"
		}
	}
	return indicator + " " + i.label
}

func (i checklistItem) Description() string { return i.desc }

func (i checklistItem) FilterValue() string {
	if i.desc != "" {
		return i.label + " " + i.desc
	}
	return i.label
}

// checklistModel is the bubbletea model for the generic checklist TUI.
type checklistModel struct {
	list         list.Model
	items        []checklistItemData
	selected     map[int]bool
	selCount     int
	total        int
	singleSelect bool
	title        string
	header       string // rendered above the list
	headerLines  int    // number of lines in header (for height calculation)
	result       []int  // selected indices; nil = cancelled
	quitting     bool
}

func newChecklistModel(cfg checklistConfig) checklistModel {
	sel := make(map[int]bool, len(cfg.items))
	selCount := 0
	selectedIdx := -1
	for i, item := range cfg.items {
		if item.preSelected {
			if cfg.singleSelect {
				if selectedIdx == -1 {
					sel[i] = true
					selCount = 1
					selectedIdx = i
				}
				continue
			}
			sel[i] = true
			selCount++
		}
	}
	if cfg.singleSelect && len(cfg.items) > 0 && selectedIdx == -1 {
		sel[0] = true
		selCount = 1
		selectedIdx = 0
	}

	hasDesc := false
	for _, item := range cfg.items {
		if item.desc != "" {
			hasDesc = true
			break
		}
	}

	listItems := makeChecklistItems(cfg.items, sel, cfg.singleSelect)

	l := list.New(listItems, newPrefixDelegate(hasDesc), 0, 0)
	l.Styles.Title = theme.Title()
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)
	if cfg.singleSelect && selectedIdx >= 0 {
		l.Select(selectedIdx)
	}

	itemName := cfg.itemName
	if itemName == "" {
		itemName = "item"
	}
	l.SetStatusBarItemName(itemName, itemName+"s")
	applyTUIFilterStyle(&l)

	headerLines := 0
	if cfg.header != "" {
		headerLines = strings.Count(cfg.header, "\n") + 1
	}

	m := checklistModel{
		list:         l,
		items:        cfg.items,
		selected:     sel,
		selCount:     selCount,
		total:        len(cfg.items),
		singleSelect: cfg.singleSelect,
		title:        cfg.title,
		header:       cfg.header,
		headerLines:  headerLines,
	}
	m.updateTitle()
	return m
}

func (m *checklistModel) updateTitle() {
	if m.singleSelect {
		m.list.Title = m.title
	} else {
		m.list.Title = fmt.Sprintf("%s (%d/%d selected)", m.title, m.selCount, m.total)
	}
}

func makeChecklistItems(items []checklistItemData, sel map[int]bool, singleSelect bool) []list.Item {
	out := make([]list.Item, len(items))
	for i, item := range items {
		out[i] = checklistItem{
			idx:          i,
			label:        item.label,
			desc:         item.desc,
			selected:     sel[i],
			singleSelect: singleSelect,
		}
	}
	return out
}

func (m checklistModel) Init() tea.Cmd { return nil }

func (m checklistModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height-4)
		return m, nil

	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch msg.String() {
		case " ": // toggle
			item, ok := m.list.SelectedItem().(checklistItem)
			if !ok {
				break
			}
			if m.singleSelect {
				// Radio: select the focused item and keep exactly one choice active.
				m.selectSingle(item.idx)
			} else {
				m.selected[item.idx] = !m.selected[item.idx]
				if m.selected[item.idx] {
					m.selCount++
				} else {
					m.selCount--
				}
			}
			m.refreshItems()
			return m, nil

		case "a": // toggle all (multi-select only)
			if m.singleSelect {
				break
			}
			selectAll := m.selCount < m.total
			for i := 0; i < m.total; i++ {
				m.selected[i] = selectAll
			}
			if selectAll {
				m.selCount = m.total
			} else {
				m.selCount = 0
			}
			m.refreshItems()
			return m, nil

		case "enter":
			if m.singleSelect {
				item, ok := m.list.SelectedItem().(checklistItem)
				if !ok {
					break
				}
				m.selectSingle(item.idx)
				m.result = []int{item.idx}
			} else {
				m.result = m.collectSelected()
			}
			m.quitting = true
			return m, tea.Quit

		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	if m.singleSelect && m.list.FilterState() != list.Filtering && len(m.items) > 0 {
		m.selectSingle(m.list.Index())
	}
	return m, cmd
}

func (m *checklistModel) selectSingle(idx int) {
	if !m.singleSelect || idx < 0 || idx >= len(m.items) {
		return
	}
	for k := range m.selected {
		delete(m.selected, k)
	}
	m.selected[idx] = true
	m.selCount = 1
	m.refreshItems()
}

func (m *checklistModel) refreshItems() {
	cursor := m.list.Index()
	items := makeChecklistItems(m.items, m.selected, m.singleSelect)
	m.list.SetItems(items)
	m.list.Select(cursor)
	m.updateTitle()
}

func (m checklistModel) collectSelected() []int {
	var indices []int
	for i := 0; i < m.total; i++ {
		if m.selected[i] {
			indices = append(indices, i)
		}
	}
	return indices
}

func (m checklistModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder
	b.WriteString(m.list.View())
	b.WriteString("\n")

	var help string
	if m.singleSelect {
		help = "↑↓ navigate  enter select  / filter  esc cancel"
	} else {
		help = "↑↓ navigate  space toggle  a all  enter confirm  / filter  esc cancel"
	}
	if m.header != "" {
		help += "  │  " + m.header
	}
	b.WriteString(theme.Dim().MarginLeft(2).Render(help))
	b.WriteString("\n")

	return b.String()
}

// runChecklistTUI runs the checklist TUI and returns the selected indices.
// Returns nil if the user cancelled (esc / ctrl+c / q).
func runChecklistTUI(cfg checklistConfig) ([]int, error) {
	model := newChecklistModel(cfg)
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	m, ok := finalModel.(checklistModel)
	if !ok {
		return nil, nil
	}

	return m.result, nil
}
