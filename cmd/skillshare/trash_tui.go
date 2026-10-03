package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"skillshare/internal/theme"
	"skillshare/internal/trash"
)

// ---------------------------------------------------------------------------
// Trash TUI — interactive multi-select with restore / delete / empty
// Left-right split layout: list on left, detail panel on right.
// ---------------------------------------------------------------------------

// trashItem is a list item for the trash TUI: one row with a checkbox.
type trashItem struct {
	entry    trash.TrashEntry
	idx      int  // index in allItems (stable identity)
	selected bool // checkbox state
}

func (i trashItem) Title() string { return i.entry.Name }

func (i trashItem) Description() string { return "" }

// trashDelegate renders "○ name" with the kind, size and age at the right.
type trashDelegate struct{}

func (trashDelegate) Height() int                             { return 1 }
func (trashDelegate) Spacing() int                            { return 0 }
func (trashDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (trashDelegate) Render(w io.Writer, m list.Model, index int, li list.Item) {
	item, ok := li.(trashItem)
	if !ok {
		return
	}
	mark := theme.Dim().Render("○")
	if item.selected {
		mark = theme.Accent().Render("◉")
	}
	meta := formatBytes(item.entry.Size) + " · " + timeAgo(item.entry.Date)
	if item.entry.Kind == "agent" {
		meta = "agent · " + meta
	}
	width := m.Width()
	renderPrefixRow(w, alignRow(mark+" "+item.entry.Name, theme.Dim().Render(meta), width-rowIndent), width, index == m.Index())
}
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
	list           list.Model
	modeLabel      string // "global" or "project"
	skillTrashBase string // for reload after operations
	agentTrashBase string // for reload after operations
	destDir        string // skill restore destination
	agentDestDir   string // agent restore destination
	cfgPath        string
	quitting       bool
	termWidth      int
	termHeight     int

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

	// Confirmation on the key line
	confirming     bool
	confirmAction  string             // "restore", "delete", "empty"
	confirmEntries []trash.TrashEntry // what the confirmed action works on

	// Operation spinner
	operating      bool
	operatingLabel string
	opSpinner      spinner.Model

	// Feedback
	lastOpMsg string // result of the last operation, shown above the key line

	// Detail scroll for right panel
	detailScroll int

	showKeys bool // ? swaps the detail panel for the full key list

	browser *fileBrowser // enter opens the selected item's files
}

func newTrashTUIModel(items []trash.TrashEntry, skillTrashBase, agentTrashBase, destDir, agentDestDir, cfgPath, modeLabel string) trashTUIModel {
	allItems := make([]trashItem, len(items))
	listItems := make([]list.Item, len(items))
	for i, entry := range items {
		ti := trashItem{entry: entry, idx: i}
		allItems[i] = ti
		listItems[i] = ti
	}

	l := list.New(listItems, trashDelegate{}, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)

	// Spinner for operations
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = theme.Accent()

	// Filter text input
	fi := newTUIFilterInput("type to match a name")

	return trashTUIModel{
		list:           l,
		modeLabel:      modeLabel,
		skillTrashBase: skillTrashBase,
		agentTrashBase: agentTrashBase,
		destDir:        destDir,
		agentDestDir:   agentDestDir,
		cfgPath:        cfgPath,
		allItems:       allItems,
		matchCount:     len(allItems),
		filterInput:    fi,
		selected:       make(map[int]bool),
		opSpinner:      sp,
	}
}

// ---------------------------------------------------------------------------
// Panel width helpers
// ---------------------------------------------------------------------------

func trashSplitActive(termWidth int) bool {
	return termWidth >= tuiMinSplitWidth
}

func trashListWidth(termWidth int) int {
	w := termWidth * 36 / 100
	if w < 30 {
		w = 30
	}
	if w > 46 {
		w = 46
	}
	return w
}

func trashDetailPanelWidth(termWidth int) int {
	return max(termWidth-trashListWidth(termWidth), 28)
}

func (m *trashTUIModel) syncTrashListSize() {
	bodyHeight := max(m.termHeight-frameChrome, 6)
	if trashSplitActive(m.termWidth) {
		m.list.SetSize(trashListWidth(m.termWidth), bodyHeight)
		return
	}
	m.list.SetSize(m.termWidth, max(bodyHeight/2, 4))
}

// ---------------------------------------------------------------------------
// Init / Update
// ---------------------------------------------------------------------------

func (m trashTUIModel) Init() tea.Cmd { return nil }

func (m trashTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		m.syncTrashListSize()
		if m.browser != nil {
			m.browser.resize(msg.Width, msg.Height)
		}
		return m, nil

	case tea.MouseMsg:
		if m.browser != nil {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				m.browser.wheel(-1)
			case tea.MouseButtonWheelDown:
				m.browser.wheel(1)
			}
			return m, nil
		}
		if trashSplitActive(m.termWidth) && !m.operating && !m.confirming {
			leftWidth := trashListWidth(m.termWidth)
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

	case spinner.TickMsg:
		if m.operating {
			var cmd tea.Cmd
			m.opSpinner, cmd = m.opSpinner.Update(msg)
			return m, cmd
		}

	case trashOpDoneMsg:
		m.operating = false
		verb := map[string]string{"restore": "Restored", "delete": "Deleted", "empty": "Deleted"}[msg.action]
		var parts []string
		if msg.count > 0 {
			parts = append(parts, theme.Success().Render("✓")+" "+verb+" "+countNoun(msg.count, "item"))
		}
		if msg.err != nil {
			parts = append(parts, theme.Danger().Render("✗")+" "+msg.err.Error())
		}
		m.lastOpMsg = "  " + strings.Join(parts, "  ")
		m.rebuildFromEntries(msg.reloadedItems)
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

		// --- Confirmation on the key line ---
		if m.confirming {
			switch msg.String() {
			case "y", "Y", "enter":
				m.confirming = false
				return m.startOperation()
			case "n", "N", "esc", "q":
				m.confirming = false
				m.confirmAction = ""
				m.confirmEntries = nil
				return m, nil
			}
			return m, nil
		}

		// --- Filter mode ---
		if m.filtering {
			cmd := handleTUIFilterKey(msg, &m.filtering, &m.filterText, &m.filterInput, m.applyTrashFilter)
			return m, cmd
		}

		if m.browser != nil {
			switch msg.String() {
			case "q", "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			case "esc":
				m.browser = nil
			default:
				m.browser.key(msg.String())
			}
			return m, nil
		}

		// --- Normal mode ---
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			if item, ok := m.list.SelectedItem().(trashItem); ok {
				m.browser = newFileBrowser("trash", item.entry.Name, item.entry.Path, false, m.termWidth, m.termHeight)
			}
			return m, nil
		case "esc":
			switch {
			case m.showKeys:
				m.showKeys = false
			case m.filterText != "":
				m.filterText = ""
				m.filterInput.SetValue("")
				m.applyTrashFilter()
			default:
				m.quitting = true
				return m, tea.Quit
			}
			return m, nil
		case "?":
			m.showKeys = !m.showKeys
			return m, nil
		case "/":
			m.filtering = true
			m.filterInput.Focus()
			m.lastOpMsg = ""
			return m, textinput.Blink

		case "ctrl+d":
			m.detailScroll += 5
			return m, nil
		case "ctrl+u":
			m.detailScroll -= 5
			if m.detailScroll < 0 {
				m.detailScroll = 0
			}
			return m, nil

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

		case "r", "d": // restore / delete the selection, or the row under the cursor
			entries := m.actionEntries()
			if len(entries) == 0 {
				break
			}
			m.confirmAction = map[string]string{"r": "restore", "d": "delete"}[msg.String()]
			m.confirmEntries = entries
			m.confirming = true
			m.lastOpMsg = ""
			return m, nil

		case "D": // empty all (ignores selection)
			if len(m.allItems) == 0 {
				break
			}
			m.confirmEntries = nil
			for _, item := range m.allItems {
				m.confirmEntries = append(m.confirmEntries, item.entry)
			}
			m.confirmAction = "empty"
			m.confirming = true
			m.lastOpMsg = ""
			return m, nil
		}
	}

	prevIdx := m.list.Index()
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	if m.list.Index() != prevIdx {
		m.detailScroll = 0
	}
	return m, cmd
}

// ---------------------------------------------------------------------------
// Filter & selection helpers
// ---------------------------------------------------------------------------

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

// actionEntries is what r and d work on: the selection, or the row under
// the cursor when nothing is selected.
func (m *trashTUIModel) actionEntries() []trash.TrashEntry {
	if m.selCount > 0 {
		return m.selectedEntries()
	}
	if item, ok := m.list.SelectedItem().(trashItem); ok {
		return []trash.TrashEntry{item.entry}
	}
	return nil
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

// ---------------------------------------------------------------------------
// Async operations
// ---------------------------------------------------------------------------

// startOperation begins the async operation (restore/delete/empty).
func (m trashTUIModel) startOperation() (tea.Model, tea.Cmd) {
	action := m.confirmAction
	entries := m.confirmEntries
	m.operating = true
	verb := map[string]string{"restore": "Restoring", "delete": "Deleting", "empty": "Deleting"}[action]
	m.operatingLabel = verb + " " + countNoun(len(entries), "item") + "…"
	m.confirmAction = ""
	m.confirmEntries = nil

	// Capture values for goroutine
	destDir := m.destDir
	agentDestDir := m.agentDestDir
	cfgPath := m.cfgPath
	skillTrashBase := m.skillTrashBase
	agentTrashBase := m.agentTrashBase

	cmd := func() tea.Msg {
		start := time.Now()
		count := 0
		var errMsgs []string

		switch action {
		case "restore":
			for _, entry := range entries {
				e := entry // copy for closure
				var restoreErr error
				if e.Kind == "agent" {
					restoreErr = trash.RestoreAgent(&e, agentDestDir)
				} else {
					restoreErr = trash.Restore(&e, destDir)
				}
				if restoreErr != nil {
					errMsgs = append(errMsgs, fmt.Sprintf("%s: %s", entry.Name, restoreErr))
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

		// Reload items from disk — merge skill + agent trash
		var reloaded []trash.TrashEntry
		for _, e := range trash.List(skillTrashBase) {
			e.Kind = "skill"
			reloaded = append(reloaded, e)
		}
		for _, e := range trash.List(agentTrashBase) {
			e.Kind = "agent"
			reloaded = append(reloaded, e)
		}
		sort.Slice(reloaded, func(i, j int) bool {
			return reloaded[i].Date.After(reloaded[j].Date)
		})
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
	m.detailScroll = 0
}

// ---------------------------------------------------------------------------
// View — split dispatch
// ---------------------------------------------------------------------------

func (m trashTUIModel) View() string {
	if m.quitting {
		return ""
	}
	if m.browser != nil {
		return m.browser.view("", nil)
	}
	bodyHeight := max(m.termHeight-frameChrome, 6)
	title := m.renderTitleLine()
	if !trashSplitActive(m.termWidth) {
		listHeight := max(bodyHeight/2, 4)
		detailHeight := max(bodyHeight-listHeight-1, 4)
		detail := lipgloss.NewStyle().Height(detailHeight).MaxHeight(detailHeight).PaddingLeft(1).
			Render(m.renderRight(m.termWidth-2, detailHeight))
		return title + "\n\n" + m.list.View() + "\n\n" + detail + "\n" + m.renderBottom()
	}
	leftWidth := trashListWidth(m.termWidth)
	rightWidth := trashDetailPanelWidth(m.termWidth)
	return title + "\n\n" +
		renderFrameSplit(m.list.View(), m.renderRight(rightWidth-2, bodyHeight), leftWidth, rightWidth, bodyHeight) + "\n" +
		m.renderBottom()
}

// renderTitleLine renders the scope, the item count and the total size.
func (m trashTUIModel) renderTitleLine() string {
	var total int64
	for _, item := range m.allItems {
		total += item.entry.Size
	}
	count := countNoun(len(m.allItems), "item")
	if m.filterText != "" {
		count = formatNumber(m.matchCount) + " of " + count
	}
	return renderFrameTitle(m.termWidth, "trash", []string{m.modeLabel, count, formatBytes(total)}, nil)
}

// renderRight renders the detail panel, or the key list while ? is on.
func (m trashTUIModel) renderRight(width, height int) string {
	if m.showKeys {
		return renderKeysPanel(trashKeyGroups)
	}
	item, ok := m.list.SelectedItem().(trashItem)
	if !ok {
		return ""
	}
	detail, _ := wrapAndScroll(m.renderTrashDetailPanel(item.entry, width), width, m.detailScroll, height)
	return detail
}

// renderBottom renders the note line and the key line. Confirmations, the
// filter input and running operations take over the key line in place.
func (m trashTUIModel) renderBottom() string {
	note := m.lastOpMsg
	var line string
	switch {
	case m.operating:
		line = renderBusyLine(m.termWidth, m.opSpinner.View(), m.operatingLabel)
	case m.confirming:
		note = theme.Dim().Render("  " + m.confirmNote())
		line = m.renderConfirm()
	case m.filtering:
		line = renderFilterLine(m.termWidth, m.filterInput.View(), m.matchCount)
	default:
		filter := keyHint{"/", "filter"}
		if m.filterText != "" {
			filter = keyHint{"esc", "clear filter"}
		}
		hints := []keyHint{{"↑↓", "move"}, {"space", "select"}, filter, {"enter", "open files"}, {"r", "restore"}, {"d", "delete"}, {"?", "keys"}}
		if m.showKeys {
			hints = []keyHint{{"?/esc", "close"}}
		}
		right := framePosition(m.list.Index()+1, m.matchCount)
		if m.selCount > 0 {
			right = formatNumber(m.selCount) + " selected"
		}
		line = renderKeyLine(m.termWidth, hints, right)
	}
	return note + "\n" + line
}

// renderConfirm asks about the pending action on the key line.
func (m trashTUIModel) renderConfirm() string {
	n := countNoun(len(m.confirmEntries), "item")
	switch m.confirmAction {
	case "restore":
		return renderConfirmLine(m.termWidth, "Restore "+n+"?", false, "")
	case "delete":
		return renderConfirmLine(m.termWidth, "Delete "+n+" for good?", true, "")
	default:
		return renderConfirmLine(m.termWidth, "Empty the trash? All "+n+" are deleted for good.", true, "")
	}
}

// confirmNote names what the pending action works on and, for a restore,
// where it goes.
func (m trashTUIModel) confirmNote() string {
	names := make([]string, 0, 5)
	for i, e := range m.confirmEntries {
		if i == 5 {
			names = append(names, fmt.Sprintf("+%d more", len(m.confirmEntries)-5))
			break
		}
		names = append(names, e.Name)
	}
	note := strings.Join(names, ", ")
	if m.confirmAction == "restore" {
		note += " → " + m.restoreDestination()
	}
	return note
}

// restoreDestination says where the pending restore puts things.
func (m trashTUIModel) restoreDestination() string {
	var hasSkills, hasAgents bool
	for _, e := range m.confirmEntries {
		if e.Kind == "agent" {
			hasAgents = true
		} else {
			hasSkills = true
		}
	}
	switch {
	case hasSkills && hasAgents:
		return "skills to " + shortenPath(m.destDir) + ", agents to " + shortenPath(m.agentDestDir)
	case hasAgents:
		return shortenPath(m.agentDestDir)
	default:
		return shortenPath(m.destDir)
	}
}

// trashKeyGroups lists every key for the ? panel.
var trashKeyGroups = []keyGroup{
	{"Move", []keyHint{
		{"↑↓", "move"},
		{"←→", "page"},
		{"/", "filter by name"},
		{"ctrl+d/u", "scroll the details"},
		{"enter", "open the files"},
		{"esc", "clear the filter, then quit"},
		{"q", "quit"},
	}},
	{"Select", []keyHint{
		{"space", "select"},
		{"a", "select all, or none"},
	}},
	{"Actions", []keyHint{
		{"r", "restore the selection, or the row under the cursor"},
		{"d", "delete for good"},
		{"D", "empty the trash"},
	}},
}

// ---------------------------------------------------------------------------
// Rendering helpers
// ---------------------------------------------------------------------------

// renderTrashDetailPanel renders the facts and a preview of the selected entry.
func (m trashTUIModel) renderTrashDetailPanel(entry trash.TrashEntry, width int) string {
	var b strings.Builder
	b.WriteString(theme.Primary().Bold(true).Render(entry.Name))
	b.WriteString("\n\n")
	row := func(label, value string) {
		b.WriteString(theme.Dim().Render(fmt.Sprintf("%-9s ", label)) + value + "\n")
	}
	kind := "skill"
	if entry.Kind == "agent" {
		kind = "agent"
	}
	row("Kind", kind)
	row("Trashed", entry.Date.Format("2006-01-02 15:04")+theme.Dim().Render(" · "+timeAgo(entry.Date)))
	row("Size", formatBytes(entry.Size))
	row("Path", shortenPath(entry.Path))

	// Content preview — SKILL.md for skills, agent .md file for agents
	var previewFile, previewTitle string
	if entry.Kind == "agent" {
		if entries, readErr := os.ReadDir(entry.Path); readErr == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
					previewFile = filepath.Join(entry.Path, e.Name())
					previewTitle = e.Name()
					break
				}
			}
		}
	} else {
		previewFile = filepath.Join(entry.Path, "SKILL.md")
		previewTitle = "SKILL.md"
	}
	if previewFile != "" {
		if data, err := os.ReadFile(previewFile); err == nil {
			lines := strings.SplitN(printableText(string(data)), "\n", 16)
			if len(lines) > 15 {
				lines = lines[:15]
			}
			if preview := strings.TrimRight(strings.Join(lines, "\n"), "\n"); preview != "" {
				b.WriteString("\n" + theme.Primary().Bold(true).Render(previewTitle) + "\n")
				for _, line := range strings.Split(preview, "\n") {
					b.WriteString(theme.Dim().Render(line) + "\n")
				}
			}
		}
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Entry point
// ---------------------------------------------------------------------------

// runTrashTUI starts the bubbletea TUI for the trash viewer.
func runTrashTUI(items []trash.TrashEntry, skillTrashBase, agentTrashBase, destDir, agentDestDir, cfgPath, modeLabel string) error {
	model := newTrashTUIModel(items, skillTrashBase, agentTrashBase, destDir, agentDestDir, cfgPath, modeLabel)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}
