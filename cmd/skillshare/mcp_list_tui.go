package main

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"skillshare/internal/mcp"
	"skillshare/internal/theme"
)

type mcpListItem struct {
	name, description, detail string
}

func (i mcpListItem) Title() string       { return i.name }
func (i mcpListItem) Description() string { return i.description }
func (i mcpListItem) FilterValue() string { return i.name + " " + i.description }

// Lists never expose arguments or credential values, including URL queries.
func mcpConnectionSummary(server mcp.Server) string {
	if server.Command != "" {
		return "stdio · " + server.Command
	}
	u, err := url.Parse(server.URL)
	if err != nil {
		return "http"
	}
	u.User, u.RawQuery, u.Fragment = nil, "", ""
	return "http · " + u.String()
}

func mcpListItems(source *mcp.Source, plan *mcp.Plan) []list.Item {
	items := []list.Item{}
	for _, name := range mcpServerNames(source) {
		server := source.Servers[name]
		targets := server.Targets
		if targets == nil {
			targets = source.Targets
		}
		status := []string{}
		if plan != nil {
			for _, change := range plan.Changes {
				if change.Name == name {
					status = append(status, change.Target+": "+change.Action)
				}
			}
		}
		if len(status) == 0 {
			status = []string{"not synchronized / no preview"}
		}
		description := mcpConnectionSummary(server) + " · " + strings.Join(targets, ", ") + " · " + strings.Join(status, "; ")
		var detail strings.Builder
		fmt.Fprintf(&detail, "%s\n\n%s\nArguments: %d (values hidden)\nTargets: %s\n\nSync status\n%s\n", name, mcpConnectionSummary(server), len(server.Args), strings.Join(targets, ", "), strings.Join(status, "\n"))
		for _, group := range []struct {
			title  string
			values map[string]mcp.Value
		}{{"Environment", server.Env}, {"Headers", server.Headers}} {
			keys := make([]string, 0, len(group.values))
			for key := range group.values {
				keys = append(keys, key)
			}
			slices.Sort(keys)
			if len(keys) > 0 {
				fmt.Fprintf(&detail, "\n%s (values hidden)\n%s\n", group.title, strings.Join(keys, "\n"))
			}
		}
		if server.BearerToken != nil {
			detail.WriteString("\nBearer token: environment reference (value hidden)\n")
		}
		fmt.Fprintf(&detail, "\nSource: %s", source.Path)
		items = append(items, mcpListItem{name: name, description: description, detail: detail.String()})
	}
	return items
}

type mcpListModel struct {
	list                 list.Model
	viewport             viewport.Model
	width, height        int
	showDetail           bool
	action, name, notice string
}

func newMCPListModel(source *mcp.Source, plan *mcp.Plan, scope, notice string) mcpListModel {
	l := list.New(mcpListItems(source, plan), newPrefixDelegate(true), 80, 18)
	l.Title = "MCP connections (" + scope + ")"
	l.Styles.Title = theme.Title()
	l.SetShowHelp(false)
	l.SetStatusBarItemName("server", "servers")
	applyTUIFilterStyle(&l)
	return mcpListModel{list: l, viewport: viewport.New(76, 18), width: 80, height: 24, notice: notice}
}

func (m mcpListModel) Init() tea.Cmd { return nil }

func (m mcpListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.list.SetSize(msg.Width, max(5, msg.Height-5))
		m.viewport.Width, m.viewport.Height = max(10, msg.Width-4), max(3, msg.Height-5)
		if item, ok := m.list.SelectedItem().(mcpListItem); ok {
			m.viewport.SetContent(hardWrapContent(item.detail, m.viewport.Width))
		}
		return m, nil
	case tea.KeyMsg:
		if m.showDetail {
			if msg.String() == "esc" || msg.String() == "enter" {
				m.showDetail = false
				return m, nil
			}
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
		if m.list.FilterState() == list.Filtering {
			break
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.list.FilterState() == list.FilterApplied {
				m.list.ResetFilter()
				return m, nil
			}
			return m, tea.Quit
		case "enter":
			if item, ok := m.list.SelectedItem().(mcpListItem); ok {
				m.showDetail = true
				m.viewport.SetContent(hardWrapContent(item.detail, m.viewport.Width))
				m.viewport.GotoTop()
			}
			return m, nil
		case "a", "A":
			m.action = "add"
		case "i", "I":
			m.action = "import"
		case "e", "E":
			m.action = "edit"
		case "x", "X":
			m.action = "remove"
		case "s":
			m.action = "sync"
		case "b":
			m.action = "restore"
		case "r":
			m.action = "refresh"
		}
		if m.action != "" {
			if item, ok := m.list.SelectedItem().(mcpListItem); ok {
				m.name = item.name
			}
			if (m.action == "edit" || m.action == "remove") && m.name == "" {
				m.action = ""
				return m, nil
			}
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m mcpListModel) View() string {
	if m.showDetail {
		return "\n" + m.viewport.View() + "\n" + formatHelpBar("↑↓ scroll  Esc back  q quit")
	}
	notice := m.notice
	if len(m.list.Items()) == 0 {
		notice = "No MCP servers. Press a to add or i to import."
	}
	return m.list.View() + "\n" + theme.Dim().Render(hardWrapContent(notice, max(10, m.width-4))) + "\n" + formatHelpBar("/ search  Enter details  a add  i import  e edit  x remove") + "\n" + formatHelpBar("s sync  b backups  r refresh  q quit")
}

func runMCPManager(service *mcp.Service) error {
	notice := ""
	for {
		source, err := mcp.LoadSource(service.ConfigPath)
		if err != nil {
			return err
		}
		plan, previewErr := service.Preview()
		if previewErr != nil {
			notice = previewErr.Error()
		}
		scope := "global"
		if service.ProjectRoot != "" {
			scope = "project"
		}
		model := newMCPListModel(source, plan, scope, notice)
		result, err := tea.NewProgram(model, tea.WithAltScreen()).Run()
		if err != nil {
			return err
		}
		m := result.(mcpListModel)
		if m.action == "" {
			return nil
		}
		prompts := terminalMCPPrompts{}
		switch m.action {
		case "add":
			err = runMCPAdd(service, mcpOptions{})
		case "import":
			err = runMCPImport(service, mcpOptions{})
		case "edit":
			err = runMCPEdit(service, mcpOptions{name: m.name})
		case "remove":
			err = mcpRemoveWizard(service, mcpOptions{name: m.name}, prompts)
		case "restore":
			err = runMCPRestore(service, mcpOptions{}, prompts)
		case "sync":
			err = mcpSyncWizard(service, prompts)
		}
		notice = ""
		if errors.Is(err, errMCPCancelled) {
			notice = "Cancelled; no draft changes saved."
		} else if err != nil {
			notice = err.Error()
		}
	}
}

func mcpSyncWizard(service *mcp.Service, prompts mcpPrompts) error {
	p, err := service.Preview()
	if err != nil {
		return err
	}
	if err := printMCPPlan(p, false); err != nil {
		return err
	}
	if p.Blocked {
		return fmt.Errorf("MCP conflicts found; import the affected entries before syncing")
	}
	pending := false
	for _, change := range p.Changes {
		pending = pending || change.Action != "unchanged"
	}
	if !pending {
		return nil
	}
	if err := prompts.review("Sync managed Agent entries", p); err != nil {
		return err
	}
	_, err = chooseMCP(prompts, checklistConfig{title: "Apply these MCP changes?", items: []checklistItemData{{label: "Sync now", desc: "Only managed Agent entries will be changed; Esc cancels"}}, singleSelect: true})
	if err != nil {
		return err
	}
	start := time.Now()
	result, err := service.Apply(p.Revision)
	logMCPOp(service.ConfigPath, "sync mcp", start, err)
	if result != nil {
		if outputErr := printMCPResult(result, false); outputErr != nil {
			return outputErr
		}
	}
	return err
}
