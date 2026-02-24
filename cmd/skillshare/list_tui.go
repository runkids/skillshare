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
)

// listTUIModel is the bubbletea model for the interactive skill list.
type listTUIModel struct {
	list       list.Model
	totalCount int
	modeLabel  string // "global" or "project"
	sourcePath string
	targets    map[string]config.TargetConfig
	quitting   bool
	action     string // "audit", "update", "uninstall", or "" (normal quit)
}

// newListTUIModel creates a new TUI model from skill entries.
func newListTUIModel(skills []skillItem, totalCount int, modeLabel, sourcePath string, targets map[string]config.TargetConfig) listTUIModel {
	// Convert to list.Item slice
	items := make([]list.Item, len(skills))
	for i, s := range skills {
		items[i] = s
	}

	// Create delegate with colors matching existing CLI output
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	delegate.Styles.NormalTitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("6")).PaddingLeft(2) // cyan — matches ui.Cyan
	delegate.Styles.NormalDesc = lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).PaddingLeft(2) // gray — matches ui.Gray
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("6")).Bold(true).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color("6")).PaddingLeft(1)
	delegate.Styles.SelectedDesc = lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color("6")).PaddingLeft(1)

	// Create list model
	l := list.New(items, delegate, 0, 0)
	l.Title = fmt.Sprintf("Installed skills (%s)", modeLabel)
	l.Styles.Title = lipgloss.NewStyle().
		Bold(true).Foreground(lipgloss.Color("6")) // cyan
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
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
		// Reserve space for detail panel + help
		m.list.SetSize(msg.Width, msg.Height-12)
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

	// Description from SKILL.md frontmatter
	skillDir := filepath.Join(m.sourcePath, e.RelPath)
	skillMD := filepath.Join(skillDir, "SKILL.md")
	if desc := utils.ParseFrontmatterField(skillMD, "description"); desc != "" {
		// Truncate long descriptions to one line
		if len(desc) > 80 {
			desc = desc[:77] + "..."
		}
		row("Description:", desc)
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
