package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/utils"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// tuiBrandYellow is the logo yellow used for active/selected item borders across all TUIs.
const tuiBrandYellow = lipgloss.Color("#D4D93C")

// Styles for the TUI list view.
// Colors match existing CLI output: Cyan (\033[36m) = "6", Gray (\033[90m) = "8".
var (
	tuiDetailLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("8")). // gray — matches ui.Gray
				Width(14)

	tuiDetailValueStyle = lipgloss.NewStyle() // default foreground — matches ui.Reset

	tuiHelpStyle = lipgloss.NewStyle().
			MarginLeft(2).
			Foreground(lipgloss.Color("8")) // gray

	tuiSeparatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("8"))

	tuiFileStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	tuiTargetStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("6")) // cyan

	tuiFilterStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("6")) // cyan — shared by list and log TUI
)

// maxListItems is the maximum number of items passed to bubbles/list.
// Keeps the widget fast — pagination + filter operate on at most this many items.
const maxListItems = 1000

// applyTUIFilterStyle sets filter prompt, cursor, and input cursor to the shared style.
func applyTUIFilterStyle(l *list.Model) {
	l.Styles.FilterPrompt = tuiFilterStyle
	l.Styles.FilterCursor = tuiFilterStyle
	l.FilterInput.Cursor.Style = tuiFilterStyle
}

// listLoadResult holds the result of async skill loading inside the TUI.
type listLoadResult struct {
	skills     []skillItem
	totalCount int
	err        error
}

// listLoadFn is a function that loads skills (runs in a goroutine inside the TUI).
type listLoadFn func() listLoadResult

// skillsLoadedMsg is sent when the background load completes.
type skillsLoadedMsg struct{ result listLoadResult }

// doLoadCmd returns a tea.Cmd that runs loadFn in a goroutine and sends skillsLoadedMsg.
func doLoadCmd(fn listLoadFn) tea.Cmd {
	return func() tea.Msg {
		return skillsLoadedMsg{result: fn()}
	}
}

// detailData caches the I/O-heavy fields of renderDetailPanel for a single skill.
type detailData struct {
	Description   string
	License       string
	Files         []string
	SyncedTargets []string
}

// listTUIModel is the bubbletea model for the interactive skill list.
type listTUIModel struct {
	list        list.Model
	totalCount  int
	modeLabel   string // "global" or "project"
	sourcePath  string
	targets     map[string]config.TargetConfig
	quitting    bool
	action      string // "audit", "update", "uninstall", or "" (normal quit)
	termWidth   int
	detailCache map[string]*detailData // key = RelPath; lazy-populated

	// Async loading — spinner shown until data arrives
	loading     bool
	loadSpinner spinner.Model
	loadFn      listLoadFn
	loadErr     error // non-nil if loading failed

	// Application-level filter — replaces bubbles/list built-in fuzzy filter
	// to avoid O(N*M) fuzzy scan on 100k+ items every keystroke.
	allItems    []skillItem     // full item set (kept in memory, never passed to list)
	filterText  string          // current filter string
	filterInput textinput.Model // managed filter text input
	filtering   bool            // true when filter input is focused
	matchCount  int             // total matches (may exceed maxListItems)
}

// newListTUIModel creates a new TUI model.
// When loadFn is non-nil, skills are loaded asynchronously inside the TUI (spinner shown).
// When loadFn is nil, skills/totalCount are used directly (pre-loaded).
func newListTUIModel(loadFn listLoadFn, skills []skillItem, totalCount int, modeLabel, sourcePath string, targets map[string]config.TargetConfig) listTUIModel {
	// Create delegate — NormalTitle/SelectedTitle have NO Foreground
	// so that Title()'s inline lipgloss colors (white name + colored badge) take effect.
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetHeight(1)
	delegate.SetSpacing(0)
	delegate.Styles.NormalTitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).PaddingLeft(2) // bright white
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().Bold(true).
		Foreground(tuiBrandYellow).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(tuiBrandYellow).PaddingLeft(1)

	// Build initial item set (empty if async loading)
	var items []list.Item
	var allItems []skillItem
	if loadFn == nil {
		items = make([]list.Item, len(skills))
		for i, s := range skills {
			items[i] = s
		}
		allItems = skills
	}

	// Create list model — built-in filter DISABLED; we manage our own.
	l := list.New(items, delegate, 0, 0)
	l.Title = fmt.Sprintf("Installed skills (%s)", modeLabel)
	l.Styles.Title = lipgloss.NewStyle().
		Bold(true).Foreground(lipgloss.Color("6")) // cyan
	l.SetShowStatusBar(false)    // we render our own status with real total count
	l.SetFilteringEnabled(false) // application-level filter replaces built-in
	l.SetShowHelp(false)         // we render our own help
	l.SetShowPagination(false)   // we render page info in our status line

	// Loading spinner
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("6")) // cyan

	// Filter text input
	fi := textinput.New()
	fi.Prompt = "/ "
	fi.PromptStyle = tuiFilterStyle
	fi.Cursor.Style = tuiFilterStyle

	return listTUIModel{
		list:        l,
		totalCount:  totalCount,
		modeLabel:   modeLabel,
		sourcePath:  sourcePath,
		targets:     targets,
		detailCache: make(map[string]*detailData),
		loading:     loadFn != nil,
		loadSpinner: sp,
		loadFn:      loadFn,
		allItems:    allItems,
		matchCount:  len(allItems),
		filterInput: fi,
	}
}

func (m listTUIModel) Init() tea.Cmd {
	if m.loading && m.loadFn != nil {
		return tea.Batch(m.loadSpinner.Tick, doLoadCmd(m.loadFn))
	}
	return nil
}

// applyFilter does a case-insensitive substring match over allItems.
// When filtering, results are capped at maxListItems to keep bubbles/list fast.
// When filter is empty, all items are restored (full pagination).
func (m *listTUIModel) applyFilter() {
	term := strings.ToLower(m.filterText)

	// No filter — restore full item set
	if term == "" {
		all := make([]list.Item, len(m.allItems))
		for i, item := range m.allItems {
			all[i] = item
		}
		m.matchCount = len(m.allItems)
		m.list.SetItems(all)
		m.list.ResetSelected()
		return
	}

	// Substring match, capped at maxListItems
	var matched []list.Item
	count := 0
	for _, item := range m.allItems {
		if strings.Contains(strings.ToLower(item.FilterValue()), term) {
			count++
			if len(matched) < maxListItems {
				matched = append(matched, item)
			}
		}
	}
	m.matchCount = count
	m.list.SetItems(matched)
	m.list.ResetSelected()
}

func (m listTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		// Reserve space for detail panel + filter bar + help
		m.list.SetSize(msg.Width, msg.Height-17)
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.loadSpinner, cmd = m.loadSpinner.Update(msg)
			return m, cmd
		}

	case skillsLoadedMsg:
		m.loading = false
		m.loadFn = nil // release closure for GC
		if msg.result.err != nil {
			m.loadErr = msg.result.err
			m.quitting = true
			return m, tea.Quit
		}
		m.allItems = msg.result.skills
		m.totalCount = msg.result.totalCount
		m.matchCount = len(msg.result.skills)
		// Populate list
		items := make([]list.Item, len(msg.result.skills))
		for i, s := range msg.result.skills {
			items[i] = s
		}
		m.list.SetItems(items)
		return m, nil

	case tea.KeyMsg:
		// Ignore keys while loading
		if m.loading {
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				m.quitting = true
				return m, tea.Quit
			}
			return m, nil
		}

		// --- Filter mode: route keys to filterInput ---
		if m.filtering {
			switch msg.String() {
			case "esc":
				m.filtering = false
				m.filterText = ""
				m.filterInput.SetValue("")
				m.applyFilter()
				return m, nil
			case "enter":
				// Lock in filter, return focus to list
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

		// --- Normal mode ---
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "/":
			m.filtering = true
			m.filterInput.Focus()
			return m, textinput.Blink
		case "A":
			return m.quitWithAction("audit")
		case "U":
			return m.quitWithAction("update")
		case "X":
			return m.quitWithAction("uninstall")
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// quitWithAction sets the action on the selected skill and exits the TUI.
func (m listTUIModel) quitWithAction(action string) (tea.Model, tea.Cmd) {
	if _, ok := m.list.SelectedItem().(skillItem); ok {
		m.action = action
	}
	m.quitting = true
	return m, tea.Quit
}

func (m listTUIModel) View() string {
	if m.quitting {
		return ""
	}

	// Loading state — spinner + message
	if m.loading {
		return fmt.Sprintf("\n  %s Loading skills...\n", m.loadSpinner.View())
	}

	var b strings.Builder

	// List view
	b.WriteString(m.list.View())
	b.WriteString("\n\n")

	// Filter bar (always visible — shows input when filtering, status when not)
	b.WriteString(m.renderFilterBar())

	// Detail panel for selected item
	if item, ok := m.list.SelectedItem().(skillItem); ok {
		b.WriteString(m.renderDetailPanel(item.entry))
	}

	// Help line
	help := "↑↓ navigate  / filter  A audit  U update  X uninstall  q quit"
	b.WriteString(tuiHelpStyle.Render(help))
	b.WriteString("\n")

	return b.String()
}

// renderFilterBar renders the status line (always visible) and filter input when active.
func (m listTUIModel) renderFilterBar() string {
	total := len(m.allItems)
	pageInfo := m.renderPageInfo()

	if m.filtering {
		if m.filterText == "" {
			// Just entered filter mode, no text yet — show total as normal
			status := fmt.Sprintf("  %s skills%s", formatNumber(total), pageInfo)
			return "  " + m.filterInput.View() + tuiHelpStyle.Render(status) + "\n"
		}
		// Active filter with text
		status := fmt.Sprintf("  %s/%s skills", formatNumber(m.matchCount), formatNumber(total))
		if m.matchCount > maxListItems {
			status += fmt.Sprintf(" (first %s shown)", formatNumber(maxListItems))
		}
		status += pageInfo
		return "  " + m.filterInput.View() + tuiHelpStyle.Render(status) + "\n"
	}
	if m.filterText != "" {
		// Filter applied but input not focused
		status := fmt.Sprintf("  filter: %s — %s/%s skills", m.filterText, formatNumber(m.matchCount), formatNumber(total))
		if m.matchCount > maxListItems {
			status += fmt.Sprintf(" (first %s shown)", formatNumber(maxListItems))
		}
		status += pageInfo
		return tuiHelpStyle.Render(status) + "\n"
	}
	// No filter — show real total
	return tuiHelpStyle.Render(fmt.Sprintf("  %s skills%s", formatNumber(total), pageInfo)) + "\n"
}

// renderPageInfo returns page indicator like " · Page 2 of 4,729" or "" if single page.
func (m listTUIModel) renderPageInfo() string {
	p := m.list.Paginator
	if p.TotalPages <= 1 {
		return ""
	}
	return fmt.Sprintf(" · Page %s of %s", formatNumber(p.Page+1), formatNumber(p.TotalPages))
}

// formatNumber formats an integer with thousand separators (e.g., 108749 → "108,749").
func formatNumber(n int) string {
	if n < 0 {
		return "-" + formatNumber(-n)
	}
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	remainder := len(s) % 3
	if remainder > 0 {
		b.WriteString(s[:remainder])
	}
	for i := remainder; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

// getDetailData returns cached detail data for a skill, populating the cache on first access.
func (m listTUIModel) getDetailData(e skillEntry) *detailData {
	key := e.RelPath
	if d, ok := m.detailCache[key]; ok {
		return d
	}

	skillDir := filepath.Join(m.sourcePath, e.RelPath)
	skillMD := filepath.Join(skillDir, "SKILL.md")

	// Single file open for both description and license
	fm := utils.ParseFrontmatterFields(skillMD, []string{"description", "license"})

	d := &detailData{
		Description:   fm["description"],
		License:       fm["license"],
		Files:         listSkillFiles(skillDir),
		SyncedTargets: m.findSyncedTargets(e),
	}
	m.detailCache[key] = d
	return d
}

// renderDetailPanel renders the detail section for the selected skill.
func (m listTUIModel) renderDetailPanel(e skillEntry) string {
	var b strings.Builder
	b.WriteString(tuiSeparatorStyle.Render("  ─────────────────────────────────────────"))
	b.WriteString("\n")

	row := func(label, value string) {
		b.WriteString("  ")
		b.WriteString(tuiDetailLabelStyle.Render(label))
		b.WriteString(tuiDetailValueStyle.Render(value))
		b.WriteString("\n")
	}

	d := m.getDetailData(e)
	skillDir := filepath.Join(m.sourcePath, e.RelPath)

	// Description — word-wrapped to terminal width
	if d.Description != "" {
		const labelOffset = 16
		maxWidth := m.termWidth - labelOffset
		if maxWidth < 40 {
			maxWidth = 40
		}
		lines := wordWrapLines(d.Description, maxWidth)
		const maxDescLines = 3
		truncated := len(lines) > maxDescLines
		if truncated {
			lines = lines[:maxDescLines]
			lines[maxDescLines-1] += "..."
		}
		row("Description:", lines[0])
		indent := strings.Repeat(" ", labelOffset)
		for _, line := range lines[1:] {
			b.WriteString(indent)
			b.WriteString(tuiDetailValueStyle.Render(line))
			b.WriteString("\n")
		}
		b.WriteString("\n") // blank line after multi-line description
	}

	// License
	if d.License != "" {
		row("License:", d.License)
	}

	// Disk path
	row("Path:", skillDir)

	// Source info
	if e.RepoName != "" {
		row("Tracked:", e.RepoName)
	}
	if e.Source != "" {
		row("Source:", e.Source)
	} else {
		row("Source:", "(local)")
	}
	if e.InstalledAt != "" {
		row("Installed:", e.InstalledAt)
	}

	// Files
	if len(d.Files) > 0 {
		row("Files:", strings.Join(d.Files, "  "))
	}

	// Synced targets
	if len(d.SyncedTargets) > 0 {
		row("Synced to:", tuiTargetStyle.Render(strings.Join(d.SyncedTargets, ", ")))
	}

	return b.String()
}

// listSkillFiles returns visible file names in the skill directory.
func listSkillFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".") {
			name := e.Name()
			if e.IsDir() {
				name += "/"
			}
			names = append(names, name)
		}
	}
	return names
}

// findSyncedTargets returns target names where this skill has a symlink.
func (m listTUIModel) findSyncedTargets(e skillEntry) []string {
	if m.targets == nil {
		return nil
	}
	flatName := e.Name
	if e.RelPath != "" {
		flatName = utils.PathToFlatName(e.RelPath)
	}

	var synced []string
	for name, tc := range m.targets {
		linkPath := filepath.Join(tc.Path, flatName)
		if utils.IsSymlinkOrJunction(linkPath) {
			synced = append(synced, name)
		}
	}
	sort.Strings(synced)
	return synced
}

// runListTUI starts the bubbletea TUI for the skill list.
// When loadFn is non-nil, data is loaded asynchronously inside the TUI (no blank screen).
// Returns (action, skillName, error). action is "" on normal quit (q/ctrl+c).
func runListTUI(loadFn listLoadFn, modeLabel, sourcePath string, targets map[string]config.TargetConfig) (string, string, error) {
	model := newListTUIModel(loadFn, nil, 0, modeLabel, sourcePath, targets)
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return "", "", err
	}

	m, ok := finalModel.(listTUIModel)
	if !ok || m.action == "" {
		if m.loadErr != nil {
			return "", "", m.loadErr
		}
		return "", "", nil
	}

	// Extract skill name from selected item
	var skillName string
	if item, ok := m.list.SelectedItem().(skillItem); ok {
		if item.entry.RelPath != "" {
			skillName = item.entry.RelPath
		} else {
			skillName = item.entry.Name
		}
	}
	return m.action, skillName, nil
}

// wordWrapLines splits text into lines that fit within maxWidth, breaking at word boundaries.
func wordWrapLines(text string, maxWidth int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}

	var lines []string
	cur := words[0]
	for _, w := range words[1:] {
		if len(cur)+1+len(w) > maxWidth {
			lines = append(lines, cur)
			cur = w
		} else {
			cur += " " + w
		}
	}
	lines = append(lines, cur)
	return lines
}
