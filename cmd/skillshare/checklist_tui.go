package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	result       []int // selected indices; nil = cancelled
	quitting     bool
}

func newChecklistModel(cfg checklistConfig) checklistModel {
	sel := make(map[int]bool, len(cfg.items))
	selCount := 0
	for i, item := range cfg.items {
		if item.preSelected {
			sel[i] = true
			selCount++
		}
	}

	hasDesc := false
	for _, item := range cfg.items {
		if item.desc != "" {
			hasDesc = true
			break
		}
	}

	listItems := makeChecklistItems(cfg.items, sel, cfg.singleSelect)

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = hasDesc
	if hasDesc {
		delegate.SetSpacing(0)
	} else {
		delegate.SetHeight(1)
		delegate.SetSpacing(0)
	}
	delegate.Styles.NormalTitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).PaddingLeft(2)
	delegate.Styles.NormalDesc = lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).PaddingLeft(2)
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().Bold(true).
		Foreground(tuiBrandYellow).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(tuiBrandYellow).PaddingLeft(1)
	delegate.Styles.SelectedDesc = lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(tuiBrandYellow).PaddingLeft(1)

	l := list.New(listItems, delegate, 0, 0)
	l.Styles.Title = lipgloss.NewStyle().
		Bold(true).Foreground(lipgloss.Color("6"))
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)

	itemName := cfg.itemName
	if itemName == "" {
		itemName = "item"
	}
	l.SetStatusBarItemName(itemName, itemName+"s")
	applyTUIFilterStyle(&l)

	m := checklistModel{
		list:         l,
		items:        cfg.items,
		selected:     sel,
		selCount:     selCount,
		total:        len(cfg.items),
		singleSelect: cfg.singleSelect,
		title:        cfg.title,
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
				// Radio: clear all, select this one
				wasSelected := m.selected[item.idx]
				for k := range m.selected {
					delete(m.selected, k)
				}
				if !wasSelected {
					m.selected[item.idx] = true
					m.selCount = 1
				} else {
					m.selCount = 0
				}
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
				// Single select: enter confirms current cursor item
				item, ok := m.list.SelectedItem().(checklistItem)
				if !ok {
					break
				}
				// If nothing was space-toggled, select the cursor item
				if m.selCount == 0 {
					m.result = []int{item.idx}
				} else {
					m.result = m.collectSelected()
				}
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
	return m, cmd
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
		help = "↑↓ navigate  space/enter select  / filter  esc cancel"
	} else {
		help = "↑↓ navigate  space toggle  a all  enter confirm  / filter  esc cancel"
	}
	b.WriteString(tuiHelpStyle.Render(help))
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
