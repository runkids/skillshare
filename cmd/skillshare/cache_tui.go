package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"skillshare/internal/cache"
	"skillshare/internal/config"
	"skillshare/internal/ui"
	"skillshare/internal/uidist"
	versioncheck "skillshare/internal/version"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// cacheEntry is a list.Item for the cache TUI, covering both discovery and UI items.
type cacheEntry struct {
	kind       string // "discovery" or "ui"
	label      string // display name, e.g. "discovery-a1b2c3.gob" or "v0.16.6"
	detail     string // RootDir or "(current version)"
	size       int64
	orphan     bool
	corrupt    bool
	path       string // absolute path for deletion
	entryCount int    // number of cached skills (discovery only)
}

func (e cacheEntry) Title() string {
	var parts []string
	if e.kind == "discovery" {
		parts = append(parts, tc.Magenta.Render("[discovery]")+"  "+e.label)
	} else {
		parts = append(parts, tc.Cyan.Render("[ui]")+"  "+e.label)
	}
	parts = append(parts, formatBytes(e.size))

	// Inline summary (replaces Description since showDesc=false)
	if e.corrupt {
		parts = append(parts, tc.Red.Render("corrupt"))
	} else if e.kind == "discovery" {
		parts = append(parts, fmt.Sprintf("%d skills", e.entryCount))
		if e.orphan {
			parts = append(parts, tc.Yellow.Render("orphan"))
		}
	} else {
		parts = append(parts, e.detail)
	}
	return strings.Join(parts, "  ")
}

func (e cacheEntry) Description() string { return "" }

func (e cacheEntry) FilterValue() string {
	return e.label + " " + e.detail
}

// cacheTUIModel is the bubbletea model for the interactive cache browser.
type cacheTUIModel struct {
	list       list.Model
	entries    []cacheEntry
	quitting   bool
	termWidth  int
	termHeight int

	// Filter
	filterText  string
	filterInput textinput.Model
	filtering   bool
	matchCount  int

	// Confirmation overlay
	confirming    bool
	confirmAction string // "delete" or "clean-all"
	confirmLabel  string
	confirmIdx    int // index of item to delete (-1 for clean-all)
}

func newCacheTUIModel(entries []cacheEntry) cacheTUIModel {
	delegate := list.NewDefaultDelegate()
	configureDelegate(&delegate, false)

	items := make([]list.Item, len(entries))
	for i, e := range entries {
		items[i] = e
	}

	l := list.New(items, delegate, 0, 0)
	l.Title = "Cache"
	l.Styles.Title = tc.ListTitle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)

	fi := textinput.New()
	fi.Prompt = "/ "
	fi.PromptStyle = tc.Filter
	fi.Cursor.Style = tc.Filter

	return cacheTUIModel{
		list:        l,
		entries:     entries,
		matchCount:  len(entries),
		filterInput: fi,
	}
}

func (m cacheTUIModel) Init() tea.Cmd { return nil }

func (m *cacheTUIModel) applyFilter() {
	term := strings.ToLower(m.filterText)
	if term == "" {
		items := make([]list.Item, len(m.entries))
		for i, e := range m.entries {
			items[i] = e
		}
		m.matchCount = len(m.entries)
		m.list.SetItems(items)
		m.list.ResetSelected()
		return
	}

	var matched []list.Item
	for _, e := range m.entries {
		if strings.Contains(strings.ToLower(e.FilterValue()), term) {
			matched = append(matched, e)
		}
	}
	m.matchCount = len(matched)
	m.list.SetItems(matched)
	m.list.ResetSelected()
}

func (m cacheTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		// Overhead below the list widget:
		//   short: \n\n(2) + filter(1) + help(1) + \n(1) = 5
		//   tall:  + detail panel(7 fixed) + \n(1) = 13
		below := 5
		if msg.Height >= 25 {
			below = 13
		}
		listH := max(msg.Height-below, 4)
		m.list.SetSize(msg.Width, listH)
		return m, nil

	case tea.KeyMsg:
		// Confirmation overlay
		if m.confirming {
			switch msg.String() {
			case "y", "Y", "enter":
				return m.executeConfirmedAction()
			case "n", "N", "esc", "q":
				m.confirming = false
				return m, nil
			}
			return m, nil
		}

		// Filter mode
		if m.filtering {
			switch msg.String() {
			case "esc":
				m.filtering = false
				m.filterText = ""
				m.filterInput.SetValue("")
				m.applyFilter()
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
				m.applyFilter()
			}
			return m, cmd
		}

		// Normal mode
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "/":
			m.filtering = true
			m.filterInput.Focus()
			return m, textinput.Blink
		case "D", "d":
			if item, ok := m.list.SelectedItem().(cacheEntry); ok {
				m.confirming = true
				m.confirmAction = "delete"
				m.confirmLabel = fmt.Sprintf("Delete %s (%s)?", item.label, formatBytes(item.size))
				m.confirmIdx = m.list.Index()
			}
			return m, nil
		case "C":
			if len(m.entries) > 0 {
				var total int64
				for _, e := range m.entries {
					total += e.size
				}
				m.confirming = true
				m.confirmAction = "clean-all"
				m.confirmLabel = fmt.Sprintf("Remove all %d cache items (%s)?", len(m.entries), formatBytes(total))
				m.confirmIdx = -1
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// executeConfirmedAction runs the confirmed delete or clean-all.
func (m cacheTUIModel) executeConfirmedAction() (tea.Model, tea.Cmd) {
	m.confirming = false

	switch m.confirmAction {
	case "delete":
		if m.confirmIdx >= 0 && m.confirmIdx < len(m.entries) {
			entry := m.entries[m.confirmIdx]
			var err error
			if entry.kind == "discovery" {
				err = cache.RemoveDiskCache(entry.path)
			} else {
				err = uidist.ClearVersion(entry.label)
			}
			if err == nil {
				// Remove from entries and refresh list
				m.entries = append(m.entries[:m.confirmIdx], m.entries[m.confirmIdx+1:]...)
				m.applyFilter()
			}
		}
	case "clean-all":
		cacheDir := config.CacheDir()
		cache.ClearAllDiskCaches(cacheDir)
		uidist.ClearCache()
		m.entries = nil
		m.applyFilter()
	}

	if len(m.entries) == 0 {
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

func (m cacheTUIModel) View() string {
	if m.quitting {
		return ""
	}

	if m.confirming {
		return fmt.Sprintf("\n  %s\n\n  Proceed? [Y/n] ", m.confirmLabel)
	}

	var b strings.Builder
	b.WriteString(m.list.View())
	b.WriteString("\n\n")

	// Filter / status bar
	b.WriteString(m.renderFilterBar())

	// Detail panel — only when terminal is tall enough to avoid squeezing the list
	if m.termHeight >= 25 {
		if item, ok := m.list.SelectedItem().(cacheEntry); ok {
			b.WriteString(m.renderDetailPanel(item))
		}
		b.WriteString("\n")
	}

	// Help
	help := "↑↓ navigate  / filter  D delete  C clean all  q quit"
	b.WriteString(tc.Help.Render(help))
	b.WriteString("\n")

	return b.String()
}

func (m cacheTUIModel) renderFilterBar() string {
	return renderTUIFilterBar(
		m.filterInput.View(), m.filtering, m.filterText,
		m.matchCount, len(m.entries), 0,
		"items", renderPageInfoFromPaginator(m.list.Paginator),
	)
}

func (m cacheTUIModel) renderDetailPanel(e cacheEntry) string {
	const panelLines = 7 // fixed height: separator(1) + 6 data rows

	var b strings.Builder
	b.WriteString(tc.Separator.Render("  ─────────────────────────────────────────"))
	b.WriteString("\n")

	rows := 1 // separator counted
	row := func(label, value string) {
		b.WriteString("  ")
		b.WriteString(tc.Label.Render(label))
		b.WriteString(tc.Value.Render(value))
		b.WriteString("\n")
		rows++
	}

	row("Type:", e.kind)
	row("Path:", e.path)
	row("Size:", formatBytes(e.size))

	if e.kind == "discovery" {
		if e.corrupt {
			row("Status:", tc.Red.Render("corrupt"))
		} else {
			row("Skills:", fmt.Sprintf("%d", e.entryCount))
			row("Source:", e.detail)
			if e.orphan {
				row("Status:", tc.Yellow.Render("orphan (source directory missing)"))
			} else {
				row("Status:", tc.Green.Render("valid"))
			}
		}
	} else {
		row("Version:", e.label)
		row("Info:", e.detail)
	}

	// Pad to fixed height so the list above gets a stable size.
	for rows < panelLines {
		b.WriteString("\n")
		rows++
	}

	return b.String()
}

// cacheRunTUI builds entries and launches the interactive TUI.
func cacheRunTUI() error {
	cacheDir := config.CacheDir()
	items := cache.ListDiskCaches(cacheDir)
	uiVersions := listUIVersions(cacheDir)

	if len(items) == 0 && len(uiVersions) == 0 {
		ui.Info("No cache files found")
		return nil
	}

	currentVer := versioncheck.Version

	var entries []cacheEntry
	for _, item := range items {
		e := cacheEntry{
			kind:       "discovery",
			label:      filepath.Base(item.Path),
			size:       item.Size,
			orphan:     item.Orphan,
			corrupt:    item.Error != "",
			path:       item.Path,
			entryCount: item.EntryCount,
			detail:     item.RootDir,
		}
		entries = append(entries, e)
	}
	for _, v := range uiVersions {
		detail := "cached UI dist"
		if v.name == "v"+currentVer || v.name == currentVer {
			detail = "(current version)"
		}
		entries = append(entries, cacheEntry{
			kind:   "ui",
			label:  v.name,
			detail: detail,
			size:   v.size,
			path:   v.path,
		})
	}

	model := newCacheTUIModel(entries)
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	if m, ok := finalModel.(cacheTUIModel); ok && m.quitting && len(m.entries) == 0 {
		ui.Success("All cache files removed")
	}
	return nil
}
