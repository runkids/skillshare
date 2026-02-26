package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"skillshare/internal/install"
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

	delegate := list.NewDefaultDelegate()
	configureDelegate(&delegate, true)

	l := list.New(items, delegate, 0, 0)
	l.Title = "Select directory"
	l.Styles.Title = tc.ListTitle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)

	// Filter text input
	fi := textinput.New()
	fi.Prompt = "/ "
	fi.PromptStyle = tc.Filter
	fi.Cursor.Style = tc.Filter

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
			switch msg.String() {
			case "esc":
				m.filtering = false
				m.filterText = ""
				m.filterInput.SetValue("")
				m.applyDirFilter()
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
				m.applyDirFilter()
			}
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
	b.WriteString(tc.Help.Render(help))
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
type skillSelectItem struct {
	idx      int    // original index in sorted slice
	name     string // skill name
	license  string // license from SKILL.md (may be empty)
	loc      string // display location (directory or "root")
	selected bool
}

func (i skillSelectItem) Title() string {
	check := "[ ]"
	if i.selected {
		check = "[x]"
	}
	title := check + " " + i.name
	if i.license != "" {
		title += " (" + i.license + ")"
	}
	return title
}

func (i skillSelectItem) Description() string { return "" }
func (i skillSelectItem) FilterValue() string { return i.name }

// skillSelectModel is the bubbletea model for skill multi-select.
type skillSelectModel struct {
	list     list.Model
	skills   []install.SkillInfo // sorted input
	locs     []string            // pre-computed location strings
	selected map[int]bool        // index → checked
	selCount int                 // maintained counter
	total    int
	result   []install.SkillInfo // nil = cancelled
	quitting bool

	// Application-level filter (matches list/search/log TUI pattern)
	allItems    []skillSelectItem
	filterText  string
	filterInput textinput.Model
	filtering   bool
	matchCount  int
}

func newSkillSelectModel(skills []install.SkillInfo) skillSelectModel {
	// Sort by path so skills in the same directory cluster together.
	sorted := make([]install.SkillInfo, len(skills))
	copy(sorted, skills)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Path < sorted[j].Path
	})

	// Pre-compute location strings once.
	locs := make([]string, len(sorted))
	for i, s := range sorted {
		dir := filepath.Dir(s.Path)
		switch {
		case s.Path == ".", dir == ".":
			locs[i] = "root"
		default:
			locs[i] = dir
		}
	}

	sel := make(map[int]bool, len(sorted))
	items := makeSkillSelectItems(sorted, locs, sel)

	delegate := list.NewDefaultDelegate()
	configureDelegate(&delegate, false)

	// Keep typed allItems for filter
	allItems := make([]skillSelectItem, len(items))
	for i, item := range items {
		allItems[i] = item.(skillSelectItem)
	}

	l := list.New(items, delegate, 0, 0)
	l.Title = skillSelectTitle(0, len(sorted))
	l.Styles.Title = tc.ListTitle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)

	// Filter text input
	fi := textinput.New()
	fi.Prompt = "/ "
	fi.PromptStyle = tc.Filter
	fi.Cursor.Style = tc.Filter

	return skillSelectModel{
		list:        l,
		skills:      sorted,
		locs:        locs,
		selected:    sel,
		total:       len(sorted),
		allItems:    allItems,
		matchCount:  len(allItems),
		filterInput: fi,
	}
}

func skillSelectTitle(n, total int) string {
	return fmt.Sprintf("Select skills to install (%d/%d selected)", n, total)
}

// makeSkillSelectItems creates list items from sorted skills with pre-computed locs.
func makeSkillSelectItems(skills []install.SkillInfo, locs []string, selected map[int]bool) []list.Item {
	items := make([]list.Item, len(skills))
	for i, s := range skills {
		items[i] = skillSelectItem{
			idx:      i,
			name:     s.Name,
			license:  s.License,
			loc:      locs[i],
			selected: selected[i],
		}
	}
	return items
}

func (m skillSelectModel) Init() tea.Cmd { return nil }

func (m skillSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height-4)
		return m, nil

	case tea.KeyMsg:
		// --- Filter mode: only handle filter input + esc/enter ---
		if m.filtering {
			switch msg.String() {
			case "esc":
				m.filtering = false
				m.filterText = ""
				m.filterInput.SetValue("")
				m.applySkillFilter()
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
				m.applySkillFilter()
			}
			return m, cmd
		}

		// --- Normal mode ---
		switch msg.String() {
		case " ": // space — toggle current item
			item, ok := m.list.SelectedItem().(skillSelectItem)
			if !ok {
				break
			}
			m.selected[item.idx] = !m.selected[item.idx]
			if m.selected[item.idx] {
				m.selCount++
			} else {
				m.selCount--
			}
			m.refreshItems()
			return m, nil

		case "a": // toggle all
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

		case "enter": // confirm
			m.result = make([]install.SkillInfo, 0, m.selCount)
			for i, s := range m.skills {
				if m.selected[i] {
					m.result = append(m.result, s)
				}
			}
			m.quitting = true
			return m, tea.Quit

		case "/":
			m.filtering = true
			m.filterInput.Focus()
			return m, textinput.Blink

		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// refreshItems rebuilds list items to reflect current selection state.
func (m *skillSelectModel) refreshItems() {
	cursor := m.list.Index()
	// Rebuild allItems with current checkbox state
	items := makeSkillSelectItems(m.skills, m.locs, m.selected)
	m.allItems = make([]skillSelectItem, len(items))
	for i, item := range items {
		m.allItems[i] = item.(skillSelectItem)
	}
	// Apply filter if active, otherwise show all
	if m.filterText != "" {
		m.applySkillFilter()
	} else {
		m.matchCount = len(m.allItems)
		m.list.SetItems(items)
	}
	if cursor < len(m.list.Items()) {
		m.list.Select(cursor)
	}
	m.list.Title = skillSelectTitle(m.selCount, m.total)
}

// applySkillFilter does a case-insensitive substring match over allItems,
// preserving checkbox state from m.selected.
func (m *skillSelectModel) applySkillFilter() {
	term := strings.ToLower(m.filterText)

	if term == "" {
		items := make([]list.Item, len(m.allItems))
		for i := range m.allItems {
			m.allItems[i].selected = m.selected[m.allItems[i].idx]
			items[i] = m.allItems[i]
		}
		m.matchCount = len(m.allItems)
		m.list.SetItems(items)
		m.list.ResetSelected()
		return
	}

	var matched []list.Item
	for _, item := range m.allItems {
		if strings.Contains(strings.ToLower(item.FilterValue()), term) {
			item.selected = m.selected[item.idx]
			matched = append(matched, item)
		}
	}
	m.matchCount = len(matched)
	m.list.SetItems(matched)
	m.list.ResetSelected()
}

func (m skillSelectModel) View() string {
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
		"skills", renderPageInfoFromPaginator(m.list.Paginator),
	))

	help := "↑↓ navigate  space toggle  a all  enter confirm  / filter  esc cancel"
	b.WriteString(tc.Help.Render(help))
	b.WriteString("\n")

	return b.String()
}

// runSkillSelectTUI starts the skill multi-select TUI.
// Returns (selected, error). (nil, nil) means the user cancelled.
func runSkillSelectTUI(skills []install.SkillInfo) ([]install.SkillInfo, error) {
	model := newSkillSelectModel(skills)
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	m, ok := finalModel.(skillSelectModel)
	if !ok {
		return nil, nil
	}

	return m.result, nil
}
