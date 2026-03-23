package main

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"skillshare/internal/mcp"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// ─── List Item ───────────────────────────────────────────────────────

type mcpListItem struct {
	name      string
	transport string // command line or URL
	disabled  bool
	targets   string // "all" or "claude, cursor"
}

func (i mcpListItem) FilterValue() string { return i.name }
func (i mcpListItem) Title() string       { return i.name }
func (i mcpListItem) Description() string { return i.transport }

// mcpListDelegate renders a compact single-line row for each MCP server.
type mcpListDelegate struct{}

func (mcpListDelegate) Height() int                             { return 1 }
func (mcpListDelegate) Spacing() int                           { return 0 }
func (mcpListDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (mcpListDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	srv, ok := item.(mcpListItem)
	if !ok {
		return
	}
	width := m.Width()
	if width <= 0 {
		width = 40
	}
	selected := index == m.Index()

	var badge string
	if srv.disabled {
		badge = tc.Dim.Render(" (disabled)")
	} else {
		badge = tc.Green.Render(" ✓")
	}

	line := srv.name + badge
	renderPrefixRow(w, line, width, selected)
}

// ─── Model ───────────────────────────────────────────────────────────

type mcpListTUIModel struct {
	list          list.Model
	allItems      []mcpListItem
	mcpConfigPath string
	modeLabel     string
	quitting      bool
	termWidth     int
	termHeight    int

	// Filter
	filtering   bool
	filterText  string
	filterInput textinput.Model
	matchCount  int

	// Confirm overlay for delete
	confirming    bool
	confirmTarget string // name of server to delete

	// Action feedback
	lastActionMsg string
}

func newMCPListTUIModel(items []mcpListItem, mcpConfigPath, modeLabel string) mcpListTUIModel {
	delegate := mcpListDelegate{}

	l := list.New(mcpToListItems(items), delegate, 0, 0)
	l.Title = fmt.Sprintf("MCP Servers (%s)", modeLabel)
	l.Styles.Title = tc.ListTitle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)

	fi := textinput.New()
	fi.Prompt = "/ "
	fi.PromptStyle = tc.Filter
	fi.Cursor.Style = tc.Filter
	fi.Placeholder = "filter by name"

	return mcpListTUIModel{
		list:          l,
		allItems:      items,
		matchCount:    len(items),
		mcpConfigPath: mcpConfigPath,
		modeLabel:     modeLabel,
		filterInput:   fi,
	}
}

// ─── Init ────────────────────────────────────────────────────────────

func (m mcpListTUIModel) Init() tea.Cmd {
	return nil
}

// ─── Update ──────────────────────────────────────────────────────────

func (m mcpListTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		m.syncMCPListSize()
		return m, nil

	case tea.KeyMsg:
		// Confirm overlay
		if m.confirming {
			return m.handleMCPConfirmKey(msg)
		}

		// Filter mode
		if m.filtering {
			switch msg.String() {
			case "esc":
				m.filtering = false
				m.filterText = ""
				m.filterInput.SetValue("")
				m.applyMCPFilter()
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
				m.applyMCPFilter()
			}
			return m, cmd
		}

		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "/":
			m.filtering = true
			m.filterInput.Focus()
			m.lastActionMsg = ""
			return m, textinput.Blink
		case "d":
			return m.toggleMCPDisabled()
		case "x":
			return m.enterMCPConfirm()
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// ─── View ────────────────────────────────────────────────────────────

func (m mcpListTUIModel) View() string {
	if m.quitting {
		return ""
	}
	if m.confirming {
		return m.renderMCPConfirmOverlay()
	}
	return m.viewMCPList()
}

func (m mcpListTUIModel) viewMCPList() string {
	var b strings.Builder

	b.WriteString(m.list.View())
	b.WriteString("\n\n")

	// Detail line for selected item
	if item, ok := m.list.SelectedItem().(mcpListItem); ok {
		b.WriteString(m.renderMCPDetail(item))
		b.WriteString("\n")
	}

	b.WriteString(m.renderMCPFilterBar())

	if m.lastActionMsg != "" {
		b.WriteString(renderMCPActionMsg(m.lastActionMsg))
		b.WriteString("\n")
	}

	b.WriteString(m.renderMCPHelp())
	b.WriteString("\n")

	return b.String()
}

// ─── Detail ──────────────────────────────────────────────────────────

func (m mcpListTUIModel) renderMCPDetail(item mcpListItem) string {
	var b strings.Builder

	label := tc.Label.Render("Transport")
	b.WriteString(label + tc.Dim.Render(item.transport) + "\n")

	label = tc.Label.Render("Targets")
	b.WriteString(label + tc.Dim.Render(item.targets) + "\n")

	label = tc.Label.Render("Status")
	if item.disabled {
		b.WriteString(label + tc.Yellow.Render("disabled") + "\n")
	} else {
		b.WriteString(label + tc.Green.Render("enabled") + "\n")
	}

	return b.String()
}

// ─── Filter ──────────────────────────────────────────────────────────

func (m mcpListTUIModel) renderMCPFilterBar() string {
	return renderTUIFilterBar(
		m.filterInput.View(), m.filtering, m.filterText,
		m.matchCount, len(m.allItems), 0,
		"servers", renderPageInfoFromPaginator(m.list.Paginator),
	)
}

func (m mcpListTUIModel) renderMCPHelp() string {
	helpText := "↑↓ navigate  / filter  d toggle  x remove  q quit"
	if m.filtering {
		helpText = "Enter lock  Esc clear"
	}
	return tc.Help.Render(helpText)
}

func renderMCPActionMsg(msg string) string {
	if strings.HasPrefix(msg, "✓") {
		return tc.Green.Render(msg)
	}
	if strings.HasPrefix(msg, "✗") {
		return tc.Red.Render(msg)
	}
	return tc.Yellow.Render(msg)
}

// ─── Layout ──────────────────────────────────────────────────────────

func (m *mcpListTUIModel) syncMCPListSize() {
	listHeight := max(m.termHeight-10, 6)
	m.list.SetSize(m.termWidth, listHeight)
}

// ─── Actions ─────────────────────────────────────────────────────────

func (m mcpListTUIModel) toggleMCPDisabled() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(mcpListItem)
	if !ok {
		return m, nil
	}

	cfg, err := mcp.LoadMCPConfig(m.mcpConfigPath)
	if err != nil {
		m.lastActionMsg = "✗ " + err.Error()
		return m, nil
	}

	srv, exists := cfg.Servers[item.name]
	if !exists {
		m.lastActionMsg = "✗ server not found"
		return m, nil
	}

	srv.Disabled = !srv.Disabled
	cfg.Servers[item.name] = srv

	if err := cfg.Save(m.mcpConfigPath); err != nil {
		m.lastActionMsg = "✗ " + err.Error()
		return m, nil
	}

	if srv.Disabled {
		m.lastActionMsg = fmt.Sprintf("✓ Disabled %q", item.name)
	} else {
		m.lastActionMsg = fmt.Sprintf("✓ Enabled %q", item.name)
	}

	m.reloadMCPItems(cfg)
	return m, nil
}

func (m mcpListTUIModel) enterMCPConfirm() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(mcpListItem)
	if !ok {
		return m, nil
	}
	m.confirming = true
	m.confirmTarget = item.name
	m.lastActionMsg = ""
	return m, nil
}

func (m mcpListTUIModel) handleMCPConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		m.confirming = false
		return m.removeMCPServer()
	case "n", "N", "esc", "q":
		m.confirming = false
		m.confirmTarget = ""
		return m, nil
	}
	return m, nil
}

func (m mcpListTUIModel) renderMCPConfirmOverlay() string {
	body := fmt.Sprintf("Remove MCP server %q?\nThis only removes the config entry.", m.confirmTarget)
	return fmt.Sprintf("\n%s\n\n%s\n\nProceed? [Y/n] ",
		tc.Title.Render("Remove Server"), body)
}

func (m mcpListTUIModel) removeMCPServer() (tea.Model, tea.Cmd) {
	cfg, err := mcp.LoadMCPConfig(m.mcpConfigPath)
	if err != nil {
		m.lastActionMsg = "✗ " + err.Error()
		return m, nil
	}

	name := m.confirmTarget
	if _, exists := cfg.Servers[name]; !exists {
		m.lastActionMsg = "✗ server not found"
		m.confirmTarget = ""
		return m, nil
	}

	delete(cfg.Servers, name)

	if err := cfg.Save(m.mcpConfigPath); err != nil {
		m.lastActionMsg = "✗ " + err.Error()
		return m, nil
	}

	m.lastActionMsg = fmt.Sprintf("✓ Removed %q", name)
	m.confirmTarget = ""

	// If no more servers, quit
	if len(cfg.Servers) == 0 {
		m.quitting = true
		return m, tea.Quit
	}

	m.reloadMCPItems(cfg)
	return m, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────

func (m *mcpListTUIModel) reloadMCPItems(cfg *mcp.MCPConfig) {
	items := buildMCPListItems(cfg)
	m.allItems = items
	m.applyMCPFilter()
}

func (m *mcpListTUIModel) applyMCPFilter() {
	if m.filterText == "" {
		m.matchCount = len(m.allItems)
		m.list.SetItems(mcpToListItems(m.allItems))
		m.list.ResetSelected()
		return
	}

	q := strings.ToLower(m.filterText)
	var matched []list.Item
	for _, item := range m.allItems {
		if strings.Contains(strings.ToLower(item.name), q) {
			matched = append(matched, item)
		}
	}
	m.matchCount = len(matched)
	m.list.SetItems(matched)
	m.list.ResetSelected()
}

func mcpToListItems(items []mcpListItem) []list.Item {
	result := make([]list.Item, len(items))
	for i, item := range items {
		result[i] = item
	}
	return result
}

func buildMCPListItems(cfg *mcp.MCPConfig) []mcpListItem {
	names := make([]string, 0, len(cfg.Servers))
	for name := range cfg.Servers {
		names = append(names, name)
	}
	sort.Strings(names)

	items := make([]mcpListItem, 0, len(names))
	for _, name := range names {
		srv := cfg.Servers[name]

		transport := srv.Command
		if srv.IsRemote() {
			transport = srv.URL
		}

		var targets string
		if len(srv.Targets) == 0 {
			targets = "all"
		} else {
			targets = strings.Join(srv.Targets, ", ")
		}

		items = append(items, mcpListItem{
			name:      name,
			transport: transport,
			disabled:  srv.Disabled,
			targets:   targets,
		})
	}
	return items
}

// ─── Entry Point ─────────────────────────────────────────────────────

// runMCPListTUI launches the interactive MCP server list TUI.
func runMCPListTUI(mcpConfigPath string, cfg *mcp.MCPConfig, modeLabel string) error {
	items := buildMCPListItems(cfg)
	model := newMCPListTUIModel(items, mcpConfigPath, modeLabel)

	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
