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

// applyTUIFilterStyle sets filter prompt, cursor, and input cursor to the shared style.
func applyTUIFilterStyle(l *list.Model) {
	l.Styles.FilterPrompt = tuiFilterStyle
	l.Styles.FilterCursor = tuiFilterStyle
	l.FilterInput.Cursor.Style = tuiFilterStyle
}

// listTUIModel is the bubbletea model for the interactive skill list.
type listTUIModel struct {
	list       list.Model
	totalCount int
	modeLabel  string // "global" or "project"
	sourcePath string
	targets    map[string]config.TargetConfig
	quitting   bool
	action     string // "audit", "update", "uninstall", or "" (normal quit)
	termWidth  int
}

// newListTUIModel creates a new TUI model from skill entries.
func newListTUIModel(skills []skillItem, totalCount int, modeLabel, sourcePath string, targets map[string]config.TargetConfig) listTUIModel {
	// Convert to list.Item slice
	items := make([]list.Item, len(skills))
	for i, s := range skills {
		items[i] = s
	}

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

	// Create list model
	l := list.New(items, delegate, 0, 0)
	l.Title = fmt.Sprintf("Installed skills (%s)", modeLabel)
	l.Styles.Title = lipgloss.NewStyle().
		Bold(true).Foreground(lipgloss.Color("6")) // cyan
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	applyTUIFilterStyle(&l)
	l.SetShowHelp(false) // We render our own help

	// Custom status bar showing count
	l.SetStatusBarItemName("skill", "skills")

	return listTUIModel{
		list:       l,
		totalCount: totalCount,
		modeLabel:  modeLabel,
		sourcePath: sourcePath,
		targets:    targets,
	}
}

func (m listTUIModel) Init() tea.Cmd {
	return nil
}

func (m listTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		// Reserve space for detail panel + help
		m.list.SetSize(msg.Width, msg.Height-16)
		return m, nil

	case tea.KeyMsg:
		// Don't intercept keys when filtering
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
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

	var b strings.Builder

	// List view
	b.WriteString(m.list.View())
	b.WriteString("\n")

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

	// Description from SKILL.md frontmatter — word-wrapped to terminal width
	skillDir := filepath.Join(m.sourcePath, e.RelPath)
	skillMD := filepath.Join(skillDir, "SKILL.md")
	if desc := utils.ParseFrontmatterField(skillMD, "description"); desc != "" {
		// 2 (left padding) + 14 (label width) = 16 chars before value
		const labelOffset = 16
		maxWidth := m.termWidth - labelOffset
		if maxWidth < 40 {
			maxWidth = 40
		}
		lines := wordWrapLines(desc, maxWidth)
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

	// License from SKILL.md frontmatter
	if license := utils.ParseFrontmatterField(skillMD, "license"); license != "" {
		row("License:", license)
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

	// Files in skill directory
	if files := listSkillFiles(skillDir); len(files) > 0 {
		row("Files:", strings.Join(files, "  "))
	}

	// Synced targets
	if synced := m.findSyncedTargets(e); len(synced) > 0 {
		row("Synced to:", tuiTargetStyle.Render(strings.Join(synced, ", ")))
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
// Returns (action, skillName, error). action is "" on normal quit (q/ctrl+c).
func runListTUI(skills []skillItem, totalCount int, modeLabel, sourcePath string, targets map[string]config.TargetConfig) (string, string, error) {
	model := newListTUIModel(skills, totalCount, modeLabel, sourcePath, targets)
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return "", "", err
	}

	m, ok := finalModel.(listTUIModel)
	if !ok || m.action == "" {
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
