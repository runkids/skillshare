package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"skillshare/internal/install"
	"skillshare/internal/theme"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// dirPickerItemKind distinguishes directory entries from the "Install all" action.
type dirPickerItemKind int

const (
	dirPickerItemDir        dirPickerItemKind = iota // subdirectory
	dirPickerItemInstallAll                          // "Install all N skills"
)

// dirPickerItem is a list item for the directory picker TUI.
type dirPickerItem struct {
	kind       dirPickerItemKind
	dirName    string
	skillCount int
	skills     []install.SkillInfo
}

func (i dirPickerItem) Title() string {
	if i.kind == dirPickerItemInstallAll {
		return fmt.Sprintf("Install all %d skills", i.skillCount)
	}
	return i.dirName
}

func (i dirPickerItem) Description() string {
	if i.kind == dirPickerItemInstallAll {
		return "select all skills at this level"
	}
	return fmt.Sprintf("%d skills", i.skillCount)
}

func (i dirPickerItem) FilterValue() string {
	if i.kind == dirPickerItemInstallAll {
		return "install all"
	}
	return i.dirName
}

// dirPickerLevel represents one level in the navigation stack.
type dirPickerLevel struct {
	prefix string
	skills []install.SkillInfo
}

// dirPickerModel is the bubbletea model for the directory picker TUI.
type dirPickerModel struct {
	list       list.Model
	allSkills  []install.SkillInfo
	stack      []dirPickerLevel    // stack[0] = root
	result     []install.SkillInfo // selected skills (nil = cancelled)
	installAll bool                // true when user chose "Install all"
	quitting   bool

	// Application-level filter (matches list/search/log TUI pattern)
	allItems    []list.Item
	filterText  string
	filterInput textinput.Model
	filtering   bool
	matchCount  int
}

// newDirPickerModel creates a new directory picker TUI model.
func newDirPickerModel(skills []install.SkillInfo) dirPickerModel {
	root := dirPickerLevel{prefix: "", skills: skills}

	m := dirPickerModel{
		allSkills: skills,
		stack:     []dirPickerLevel{root},
	}

	items := m.buildItems(skills, "")

	l := list.New(items, newPrefixDelegate(true), 0, 0)
	l.Title = "Select directory"
	l.Styles.Title = theme.Title()
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)

	// Filter text input
	fi := newTUIFilterInput("")

	m.list = l
	m.allItems = items
	m.matchCount = len(items)
	m.filterInput = fi
	return m
}

// buildItems creates list items from skills grouped by directory at the given prefix.
func (m dirPickerModel) buildItems(skills []install.SkillInfo, prefix string) []list.Item {
	groups := groupSkillsByDirectory(skills, prefix)

	items := make([]list.Item, 0, len(groups)+1)
	// "Install all" is always first
	items = append(items, dirPickerItem{
		kind:       dirPickerItemInstallAll,
		skillCount: len(skills),
		skills:     skills,
	})

	for _, g := range groups {
		items = append(items, dirPickerItem{
			kind:       dirPickerItemDir,
			dirName:    g.dir,
			skillCount: len(g.skills),
			skills:     g.skills,
		})
	}

	return items
}

// currentLevel returns the top of the navigation stack.
func (m dirPickerModel) currentLevel() dirPickerLevel {
	return m.stack[len(m.stack)-1]
}

func (m dirPickerModel) Init() tea.Cmd {
	return nil
}

func (m dirPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height-4)
		return m, nil

	case tea.KeyMsg:
		// --- Filter mode: only handle filter input + esc/enter ---
		if m.filtering {
			cmd := handleTUIFilterKey(msg, &m.filtering, &m.filterText, &m.filterInput, m.applyDirFilter)
			return m, cmd
		}

		// --- Normal mode ---
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			item, ok := m.list.SelectedItem().(dirPickerItem)
			if !ok {
				break
			}

			switch item.kind {
			case dirPickerItemInstallAll:
				m.result = item.skills
				m.installAll = true
				m.quitting = true
				return m, tea.Quit

			case dirPickerItemDir:
				// "(root)" is a virtual group for root-level skills — always a leaf.
				if item.dirName == "(root)" {
					m.result = item.skills
					m.quitting = true
					return m, tea.Quit
				}

				cur := m.currentLevel()
				newPrefix := item.dirName
				if cur.prefix != "" {
					newPrefix = cur.prefix + "/" + item.dirName
				}

				// Leaf directory: no further subdirectories — exit for MultiSelect.
				groups := groupSkillsByDirectory(item.skills, newPrefix)
				isLeaf := len(groups) == 1 && groups[0].dir == "(root)"
				if isLeaf {
					m.result = item.skills
					m.quitting = true
					return m, tea.Quit
				}

				// Has subdirectories — drill in
				m.stack = append(m.stack, dirPickerLevel{
					prefix: newPrefix,
					skills: item.skills,
				})
				m.rebuildList()
				return m, nil
			}

		case "/":
			m.filtering = true
			m.filterInput.Focus()
			return m, textinput.Blink

		case "backspace", "esc":
			if len(m.stack) > 1 {
				m.stack = m.stack[:len(m.stack)-1]
				m.rebuildList()
				return m, nil
			}
			// At root: esc/backspace cancels
			m.quitting = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// rebuildList updates the list items and title for the current stack level.
func (m *dirPickerModel) rebuildList() {
	cur := m.currentLevel()
	items := m.buildItems(cur.skills, cur.prefix)
	m.allItems = items
	m.filterText = ""
	m.filterInput.SetValue("")
	m.filtering = false
	m.matchCount = len(items)
	m.list.SetItems(items)
	m.list.ResetSelected()

	if cur.prefix != "" {
		m.list.Title = fmt.Sprintf("Select directory: %s (%d skills)", cur.prefix, len(cur.skills))
	} else {
		m.list.Title = fmt.Sprintf("Select directory (%d skills)", len(cur.skills))
	}
}

// applyDirFilter does a case-insensitive substring match over allItems.
func (m *dirPickerModel) applyDirFilter() {
	term := strings.ToLower(m.filterText)

	if term == "" {
		m.matchCount = len(m.allItems)
		m.list.SetItems(m.allItems)
		m.list.ResetSelected()
		return
	}

	var matched []list.Item
	for _, item := range m.allItems {
		if strings.Contains(strings.ToLower(item.(dirPickerItem).FilterValue()), term) {
			matched = append(matched, item)
		}
	}
	m.matchCount = len(matched)
	m.list.SetItems(matched)
	m.list.ResetSelected()
}

func (m dirPickerModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	b.WriteString(m.list.View())
	b.WriteString("\n\n")

	// Filter bar (always visible)
	b.WriteString(renderTUIFilterBar(
		m.filterInput.View(), m.filtering, m.filterText,
		m.matchCount, len(m.allItems), 0,
		"items", renderPageInfoFromPaginator(m.list.Paginator),
	))

	// Help line — hide "backspace back" at root level
	var help string
	if len(m.stack) > 1 {
		help = "↑↓ navigate  enter select  backspace back  / filter  q quit"
	} else {
		help = "↑↓ navigate  enter select  / filter  q quit"
	}
	b.WriteString(theme.Dim().MarginLeft(2).Render(help))
	b.WriteString("\n")

	return b.String()
}

// runDirPickerTUI starts the directory picker TUI.
// Returns (selected skills, installAll flag, error).
// (nil, false, nil) means the user cancelled.
func runDirPickerTUI(skills []install.SkillInfo) ([]install.SkillInfo, bool, error) {
	model := newDirPickerModel(skills)
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, false, err
	}

	m, ok := finalModel.(dirPickerModel)
	if !ok {
		return nil, false, nil
	}

	return m.result, m.installAll, nil
}

// ---------------------------------------------------------------------------
// Skill select TUI — multi-select with checkboxes
// ---------------------------------------------------------------------------

// skillSelectItem is a list item for the skill multi-select TUI.
// Title() returns plain text — no inline ANSI — so bubbles filter highlighting works correctly.
// runSkillSelectTUI asks which skills to install, inline. Skills are sorted by
// path so ones in the same folder sit together. (nil, nil) means cancelled.
func runSkillSelectTUI(skills []install.SkillInfo) ([]install.SkillInfo, error) {
	sorted := make([]install.SkillInfo, len(skills))
	copy(sorted, skills)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })

	items := make([]checklistItemData, len(sorted))
	for i, s := range sorted {
		var parts []string
		if s.Description != "" {
			parts = append(parts, s.Description)
		}
		if loc := skillSelectLocation(s.Path); loc != "root" {
			parts = append(parts, loc)
		}
		if s.License != "" {
			parts = append(parts, s.License)
		}
		desc := strings.Join(parts, " · ")
		items[i] = checklistItemData{label: s.Name, desc: desc}
	}
	indices, err := runChecklistTUI(checklistConfig{title: "Install which skills?", items: items, itemName: "skill"})
	if err != nil || indices == nil {
		return nil, err
	}
	selected := make([]install.SkillInfo, len(indices))
	for i, idx := range indices {
		selected[i] = sorted[idx]
	}
	return selected, nil
}

// skillSelectLocation is the folder a skill sits in, or "root".
func skillSelectLocation(path string) string {
	if dir := filepath.Dir(path); path != "." && dir != "." {
		return dir
	}
	return "root"
}
