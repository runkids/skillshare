package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"skillshare/internal/trash"
)

// ---------------------------------------------------------------------------
// Trash TUI — interactive multi-select with restore / delete / empty
// ---------------------------------------------------------------------------

// trashItem is a list item for the trash TUI. 1-line with checkbox.
type trashItem struct {
	entry    trash.TrashEntry
	idx      int  // index in allItems (stable identity)
	selected bool // checkbox state
}

func (i trashItem) Title() string {
	check := "[ ]"
	if i.selected {
		check = "[x]"
	}
	age := formatAge(time.Since(i.entry.Date))
	size := formatBytes(i.entry.Size)
	return fmt.Sprintf("%s %s  (%s, %s ago)", check, i.entry.Name, size, age)
}

func (i trashItem) Description() string { return "" }
func (i trashItem) FilterValue() string { return i.entry.Name }

// trashOpDoneMsg is sent when an async operation (restore/delete/empty) completes.
type trashOpDoneMsg struct {
	action        string // "restore", "delete", "empty"
	count         int
	err           error
	reloadedItems []trash.TrashEntry
}

// trashTUIModel is the bubbletea model for the interactive trash viewer.
type trashTUIModel struct {
	list      list.Model
	modeLabel string // "global" or "project"
	trashBase string
	destDir   string
	cfgPath   string
	quitting  bool
	termWidth int

	// All items (source of truth for filter + selection)
	allItems []trashItem

	// Application-level filter (matches list_tui pattern)
	filterText  string
	filterInput textinput.Model
	filtering   bool
	matchCount  int

	// Multi-select
	selected map[int]bool // key = idx; true = marked
	selCount int

	// Confirmation overlay
	confirming    bool
	confirmAction string   // "restore", "delete", "empty"
	confirmNames  []string // names for display

	// Operation spinner
	operating      bool
	operatingLabel string
	opSpinner      spinner.Model

	// Feedback
	lastOpMsg string // green/red message after operation

	// Cached detail panel — recomputed only on selection change
	cachedDetailIdx int
	cachedDetailStr string
}

func newTrashTUIModel(items []trash.TrashEntry, trashBase, destDir, cfgPath, modeLabel string) trashTUIModel {
	allItems := make([]trashItem, len(items))
	listItems := make([]list.Item, len(items))
	for i, entry := range items {
		ti := trashItem{entry: entry, idx: i}
		allItems[i] = ti
		listItems[i] = ti
	}

	delegate := list.NewDefaultDelegate()
	configureDelegate(&delegate, false)

	l := list.New(listItems, delegate, 0, 0)
	l.Title = trashTUITitle(modeLabel, len(items))
	l.Styles.Title = tc.ListTitle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)

	// Spinner for operations
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = tc.SpinnerStyle

	// Filter text input
	fi := textinput.New()
	fi.Prompt = "/ "
	fi.PromptStyle = tc.Filter
	fi.Cursor.Style = tc.Filter

	return trashTUIModel{
		list:        l,
		modeLabel:   modeLabel,
		trashBase:   trashBase,
		destDir:     destDir,
		cfgPath:     cfgPath,
		allItems:    allItems,
		matchCount:  len(allItems),
		filterInput: fi,
		selected:    make(map[int]bool),
		opSpinner:   sp,
	}
}

func trashTUITitle(modeLabel string, count int) string {
	return fmt.Sprintf("Trash (%s) — %d items", modeLabel, count)
}

func (m trashTUIModel) Init() tea.Cmd { return nil }

func (m trashTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.list.SetSize(msg.Width, msg.Height-14)
		m.refreshTrashDetailCache()
		return m, nil

	case spinner.TickMsg:
		if m.operating {
			var cmd tea.Cmd
			m.opSpinner, cmd = m.opSpinner.Update(msg)
			return m, cmd
		}

	case trashOpDoneMsg:
		m.operating = false
		verb := capitalize(msg.action) + "d"
		switch {
		case msg.err != nil && msg.count > 0:
			// Partial success: some succeeded, some failed
			m.lastOpMsg = tc.Green.Render(fmt.Sprintf("%s %d item(s)", verb, msg.count)) +
				"  " + tc.Red.Render(fmt.Sprintf("Failed: %s", msg.err))
		case msg.err != nil:
			m.lastOpMsg = tc.Red.Render(fmt.Sprintf("Error: %s", msg.err))
		default:
			m.lastOpMsg = tc.Green.Render(fmt.Sprintf("%s %d item(s)", verb, msg.count))
		}
		// Reload items (rebuildFromEntries invalidates detail cache)
		m.rebuildFromEntries(msg.reloadedItems)
		m.refreshTrashDetailCache()
		return m, nil

	case tea.KeyMsg:
		// Operating — only quit allowed
		if m.operating {
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				m.quitting = true
				return m, tea.Quit
			}
			return m, nil
		}

		// --- Confirmation overlay ---
		if m.confirming {
			switch msg.String() {
			case "y", "Y", "enter":
				m.confirming = false
				return m.startOperation()
			case "n", "N", "esc":
				m.confirming = false
				m.confirmAction = ""
				m.confirmNames = nil
				return m, nil
			}
			return m, nil
		}

		// --- Filter mode ---
		if m.filtering {
			switch msg.String() {
			case "esc":
				m.filtering = false
				m.filterText = ""
				m.filterInput.SetValue("")
				m.applyTrashFilter()
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
				m.applyTrashFilter()
			}
			return m, cmd
		}

		// --- Normal mode ---
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "/":
			m.filtering = true
			m.filterInput.Focus()
			m.lastOpMsg = ""
			return m, textinput.Blink

		case " ": // toggle select current item
			item, ok := m.list.SelectedItem().(trashItem)
			if !ok {
				break
			}
			m.selected[item.idx] = !m.selected[item.idx]
			if m.selected[item.idx] {
				m.selCount++
			} else {
				delete(m.selected, item.idx)
				m.selCount--
			}
			m.allItems[item.idx].selected = m.selected[item.idx]
			m.lastOpMsg = ""
			m.refreshListItems()
			return m, nil

		case "a": // toggle all visible
			visibleIndices := m.visibleIndices()
			selectAll := m.selCount < len(visibleIndices)

			// Clear all selections first
			for idx := range m.selected {
				if idx < len(m.allItems) {
					m.allItems[idx].selected = false
				}
			}
			m.selected = make(map[int]bool)
			m.selCount = 0

			if selectAll {
				for _, idx := range visibleIndices {
					m.selected[idx] = true
					m.allItems[idx].selected = true
					m.selCount++
				}
			}
			m.lastOpMsg = ""
			m.refreshListItems()
			return m, nil

		case "r": // restore selected
			if m.selCount == 0 {
				break
			}
			names := m.selectedNames()
			m.confirmAction = "restore"
			m.confirmNames = names
			m.confirming = true
			return m, nil

		case "d": // delete selected permanently
			if m.selCount == 0 {
				break
			}
			names := m.selectedNames()
			m.confirmAction = "delete"
			m.confirmNames = names
			m.confirming = true
			return m, nil

		case "D": // empty all (ignores selection)
			if len(m.allItems) == 0 {
				break
			}
			names := make([]string, len(m.allItems))
			for i, item := range m.allItems {
				names[i] = item.entry.Name
			}
			m.confirmAction = "empty"
			m.confirmNames = names
			m.confirming = true
			return m, nil
		}
	}

	prevIdx := m.list.Index()
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	if m.list.Index() != prevIdx {
		m.invalidateTrashDetailCache()
		m.refreshTrashDetailCache()
	}
	return m, cmd
}

// applyTrashFilter does a case-insensitive substring match over allItems.
func (m *trashTUIModel) applyTrashFilter() {
	term := strings.ToLower(m.filterText)

	if term == "" {
		items := make([]list.Item, len(m.allItems))
		for i, item := range m.allItems {
			items[i] = item
		}
		m.matchCount = len(m.allItems)
		m.list.SetItems(items)
		m.list.ResetSelected()
		return
	}

	var matched []list.Item
	for _, item := range m.allItems {
		if strings.Contains(strings.ToLower(item.FilterValue()), term) {
			matched = append(matched, item)
		}
	}
	m.matchCount = len(matched)
	m.list.SetItems(matched)
	m.list.ResetSelected()
	m.invalidateTrashDetailCache()
}

// refreshListItems rebuilds list items preserving cursor and checkbox state.
func (m *trashTUIModel) refreshListItems() {
	cursor := m.list.Index()
	for i := range m.allItems {
		m.allItems[i].selected = m.selected[m.allItems[i].idx]
	}
	if m.filterText != "" {
		m.applyTrashFilter()
	} else {
		items := make([]list.Item, len(m.allItems))
		for i, item := range m.allItems {
			items[i] = item
		}
		m.list.SetItems(items)
		m.matchCount = len(m.allItems)
	}
	if cursor < len(m.list.Items()) {
		m.list.Select(cursor)
	}
}

// visibleIndices returns allItems indices for all currently visible list items.
func (m *trashTUIModel) visibleIndices() []int {
	listItems := m.list.Items()
	indices := make([]int, 0, len(listItems))
	for _, li := range listItems {
		if item, ok := li.(trashItem); ok {
			indices = append(indices, item.idx)
		}
	}
	return indices
}

// selectedNames returns names of all selected items.
func (m *trashTUIModel) selectedNames() []string {
	var names []string
	for _, item := range m.allItems {
		if m.selected[item.idx] {
			names = append(names, item.entry.Name)
		}
	}
	return names
}

// selectedEntries returns trash entries for all selected items.
func (m *trashTUIModel) selectedEntries() []trash.TrashEntry {
	var entries []trash.TrashEntry
	for _, item := range m.allItems {
		if m.selected[item.idx] {
			entries = append(entries, item.entry)
		}
	}
	return entries
}

// startOperation begins the async operation (restore/delete/empty).
func (m trashTUIModel) startOperation() (tea.Model, tea.Cmd) {
	action := m.confirmAction
	m.operating = true
	m.operatingLabel = capitalize(action) + " in progress..."
	m.confirmAction = ""
	m.confirmNames = nil

	// Capture values for goroutine
	var entries []trash.TrashEntry
	if action == "empty" {
		for _, item := range m.allItems {
			entries = append(entries, item.entry)
		}
	} else {
		entries = m.selectedEntries()
	}
	trashBase := m.trashBase
	destDir := m.destDir
	cfgPath := m.cfgPath

	cmd := func() tea.Msg {
		start := time.Now()
		count := 0
		var errMsgs []string

		switch action {
		case "restore":
			for _, entry := range entries {
				e := entry // copy for closure
				if err := trash.Restore(&e, destDir); err != nil {
					errMsgs = append(errMsgs, fmt.Sprintf("%s: %s", entry.Name, err))
					continue // don't stop — process remaining items
				}
				count++
			}
		case "delete", "empty":
			for _, entry := range entries {
				if err := os.RemoveAll(entry.Path); err != nil {
					errMsgs = append(errMsgs, fmt.Sprintf("%s: %s", entry.Name, err))
					continue
				}
				count++
			}
		}

		// Build combined error (nil if all succeeded)
		var opErr error
		if len(errMsgs) > 0 {
			opErr = fmt.Errorf("%s", strings.Join(errMsgs, "; "))
		}

		// Log the operation
		logTrashOp(cfgPath, action, count, "", start, opErr)

		// Reload items from disk
		reloaded := trash.List(trashBase)
		return trashOpDoneMsg{
			action:        action,
			count:         count,
			err:           opErr,
			reloadedItems: reloaded,
		}
	}

	return m, tea.Batch(m.opSpinner.Tick, cmd)
}

// rebuildFromEntries replaces all items from freshly loaded trash entries.
func (m *trashTUIModel) rebuildFromEntries(entries []trash.TrashEntry) {
	m.allItems = make([]trashItem, len(entries))
	listItems := make([]list.Item, len(entries))
	for i, entry := range entries {
		ti := trashItem{entry: entry, idx: i}
		m.allItems[i] = ti
		listItems[i] = ti
	}
	m.selected = make(map[int]bool)
	m.selCount = 0
	m.filterText = ""
	m.filterInput.SetValue("")
	m.matchCount = len(entries)
	m.list.SetItems(listItems)
	m.list.ResetSelected()
	m.list.Title = trashTUITitle(m.modeLabel, len(entries))
	m.invalidateTrashDetailCache()
}

// refreshTrashDetailCache recomputes the detail panel only when the selection changes.
func (m *trashTUIModel) refreshTrashDetailCache() {
	idx := m.list.Index()
	if idx == m.cachedDetailIdx && m.cachedDetailStr != "" {
		return
	}
	m.cachedDetailIdx = idx
	if item, ok := m.list.SelectedItem().(trashItem); ok {
		m.cachedDetailStr = m.renderTrashDetailPanel(item.entry)
	} else {
		m.cachedDetailStr = ""
	}
}

func (m *trashTUIModel) invalidateTrashDetailCache() {
	m.cachedDetailStr = ""
	m.cachedDetailIdx = -1
}

func (m trashTUIModel) View() string {
	if m.quitting {
		return ""
	}

	// Operating state — spinner
	if m.operating {
		return fmt.Sprintf("\n  %s %s\n", m.opSpinner.View(), m.operatingLabel)
	}

	// Confirmation overlay
	if m.confirming {
		return m.viewConfirm()
	}

	var b strings.Builder

	// List view
	b.WriteString(m.list.View())
	b.WriteString("\n\n")

	// Filter bar
	b.WriteString(m.renderTrashFilterBar())

	// Detail panel for selected item (cached)
	b.WriteString(m.cachedDetailStr)
	b.WriteString("\n")

	// Feedback message + help
	help := m.trashHelpBar()
	if m.lastOpMsg != "" {
		b.WriteString("  ")
		b.WriteString(m.lastOpMsg)
		b.WriteString("\n")
	}
	b.WriteString(tc.Help.Render(help))
	b.WriteString("\n")

	return b.String()
}

// viewConfirm renders the confirmation overlay.
func (m trashTUIModel) viewConfirm() string {
	var b strings.Builder
	b.WriteString("\n")

	verb := m.confirmAction
	switch verb {
	case "restore":
		b.WriteString(fmt.Sprintf("  Restore %d item(s) to %s?\n\n", len(m.confirmNames), m.destDir))
	case "delete":
		b.WriteString("  ")
		b.WriteString(tc.Red.Render(fmt.Sprintf("Permanently delete %d item(s)?", len(m.confirmNames))))
		b.WriteString("\n\n")
	case "empty":
		b.WriteString("  ")
		b.WriteString(tc.Red.Render(fmt.Sprintf("Empty trash — permanently delete ALL %d item(s)?", len(m.confirmNames))))
		b.WriteString("\n\n")
	}

	// Show names (cap at 10)
	show := m.confirmNames
	if len(show) > 10 {
		show = show[:10]
	}
	for _, name := range show {
		b.WriteString(fmt.Sprintf("    %s\n", name))
	}
	if len(m.confirmNames) > 10 {
		b.WriteString(fmt.Sprintf("    ... and %d more\n", len(m.confirmNames)-10))
	}

	b.WriteString("\n  ")
	b.WriteString(tc.Help.Render("y confirm  n cancel"))
	b.WriteString("\n")

	return b.String()
}

// trashHelpBar returns the context-sensitive help text.
func (m trashTUIModel) trashHelpBar() string {
	var parts []string
	parts = append(parts, "↑↓ navigate  ←→ page  / filter")

	if m.selCount > 0 {
		parts = append(parts, fmt.Sprintf("r restore(%d)  d delete(%d)", m.selCount, m.selCount))
		parts = append(parts, "space toggle  a all")
	} else {
		parts = append(parts, "space select  a all")
	}

	parts = append(parts, "D empty  q quit")
	return strings.Join(parts, "  ")
}

// renderTrashFilterBar renders the status line for the trash TUI.
func (m trashTUIModel) renderTrashFilterBar() string {
	return renderTUIFilterBar(
		m.filterInput.View(), m.filtering, m.filterText,
		m.matchCount, len(m.allItems), 0,
		"items", renderPageInfoFromPaginator(m.list.Paginator),
	)
}

// renderTrashDetailPanel renders the detail section for the selected trash entry.
func (m trashTUIModel) renderTrashDetailPanel(entry trash.TrashEntry) string {
	var b strings.Builder
	b.WriteString(tc.Separator.Render("  ─────────────────────────────────────────"))
	b.WriteString("\n")

	row := func(label, value string) {
		b.WriteString("  ")
		b.WriteString(tc.Label.Render(label))
		b.WriteString(tc.Value.Render(value))
		b.WriteString("\n")
	}

	row("Name:", entry.Name)
	row("Trashed:", entry.Date.Format("2006-01-02 15:04:05"))
	row("Size:", formatBytes(entry.Size))
	row("Path:", entry.Path)

	// SKILL.md preview — read first 15 lines
	skillMD := filepath.Join(entry.Path, "SKILL.md")
	if data, err := os.ReadFile(skillMD); err == nil {
		lines := strings.SplitN(string(data), "\n", 16)
		if len(lines) > 15 {
			lines = lines[:15]
		}
		preview := strings.TrimRight(strings.Join(lines, "\n"), "\n")
		if preview != "" {
			b.WriteString("\n")
			b.WriteString(tc.Separator.Render("  ── SKILL.md ──────────────────────────────"))
			b.WriteString("\n")
			for _, line := range strings.Split(preview, "\n") {
				b.WriteString("  ")
				b.WriteString(tc.Help.Render(line))
				b.WriteString("\n")
			}
		}
	}

	return b.String()
}

// capitalize returns the string with the first letter uppercased.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// runTrashTUI starts the bubbletea TUI for the trash viewer.
func runTrashTUI(items []trash.TrashEntry, trashBase, destDir, cfgPath, modeLabel string) error {
	model := newTrashTUIModel(items, trashBase, destDir, cfgPath, modeLabel)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
