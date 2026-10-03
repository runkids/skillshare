package main

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"skillshare/internal/mcp"
	"skillshare/internal/theme"
)

type mcpListItem struct {
	name, description, kind, detail string
}

func (i mcpListItem) FilterValue() string { return i.name + " " + i.description }

// Lists never expose arguments or credential values, including URL queries.
func mcpConnectionSummary(server mcp.Server) string {
	if server.Disabled {
		return "off in this project"
	}
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
			// A switch goes only where an Agent has one, which is what sync will write.
			if server.Disabled {
				targets = mcp.SwitchTargets(server, source.Targets, nil)
			}
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
		description := mcpConnectionSummary(server) + " · " + mcpTargetSummary(targets) + " · " + strings.Join(status, "; ")
		kind := "http"
		switch {
		case server.Disabled:
			kind = "off"
		case server.Command != "":
			kind = "stdio"
		}

		var detail strings.Builder
		row := func(label, value string) {
			detail.WriteString(theme.Dim().Render(fmt.Sprintf("%-10s ", label)) + value + "\n")
		}
		detail.WriteString(theme.Primary().Bold(true).Render(name) + "\n\n")
		row("Connects", mcpConnectionSummary(server))
		row("Arguments", fmt.Sprintf("%d", len(server.Args))+theme.Dim().Render(" · values hidden"))
		row("Targets", mcpTargetSummary(targets))
		row("Source", shortenPath(source.Path))
		detail.WriteString("\n" + theme.Primary().Bold(true).Render("Sync") + "\n" + strings.Join(status, "\n") + "\n")
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
				detail.WriteString("\n" + theme.Primary().Bold(true).Render(group.title) + theme.Dim().Render(" · values hidden") + "\n" + strings.Join(keys, "\n") + "\n")
			}
		}
		if server.BearerToken != nil {
			detail.WriteString("\n" + theme.Dim().Render("Bearer token from an environment variable · value hidden") + "\n")
		}
		items = append(items, mcpListItem{name: name, description: description, kind: kind, detail: detail.String()})
	}
	return items
}

// mcpDelegate renders "name    http" rows.
type mcpDelegate struct{}

func (mcpDelegate) Height() int                             { return 1 }
func (mcpDelegate) Spacing() int                            { return 0 }
func (mcpDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (mcpDelegate) Render(w io.Writer, m list.Model, index int, li list.Item) {
	item, ok := li.(mcpListItem)
	if !ok {
		return
	}
	renderPrefixRow(w, alignRow(item.name, theme.Dim().Render(item.kind), m.Width()-rowIndent), m.Width(), index == m.Index())
}

type mcpListModel struct {
	list          list.Model
	allItems      []list.Item
	filterInput   textinput.Model
	filterText    string
	filtering     bool
	width, height int
	detailScroll  int
	showKeys      bool
	scope         string
	// action and name tell runMCPManager what to run after the TUI quits.
	action, name, notice string
}

func newMCPListModel(source *mcp.Source, plan *mcp.Plan, scope, notice string) mcpListModel {
	items := mcpListItems(source, plan)
	l := list.New(items, mcpDelegate{}, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	m := mcpListModel{list: l, allItems: items, filterInput: newTUIFilterInput("type to match a name, URL or target"), scope: scope, notice: notice}
	m.resize(120, 30)
	return m
}

// resize sizes the list for the terminal.
func (m *mcpListModel) resize(width, height int) {
	m.width, m.height = width, height
	bodyHeight := max(height-frameChrome, 6)
	if width < tuiMinSplitWidth {
		m.list.SetSize(width, max(bodyHeight/2, 4))
		return
	}
	m.list.SetSize(listPanelWidth(width), bodyHeight)
}

// applyFilter keeps the servers whose name, connection or targets match.
func (m *mcpListModel) applyFilter() {
	needle := strings.ToLower(m.filterText)
	var matched []list.Item
	for _, it := range m.allItems {
		if needle == "" || strings.Contains(strings.ToLower(it.FilterValue()), needle) {
			matched = append(matched, it)
		}
	}
	m.list.SetItems(matched)
	m.list.ResetSelected()
	m.detailScroll = 0
}

func (m mcpListModel) Init() tea.Cmd { return nil }

func (m mcpListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
		return m, nil
	case tea.KeyMsg:
		if m.filtering {
			return m, handleTUIFilterKey(msg, &m.filtering, &m.filterText, &m.filterInput, m.applyFilter)
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc":
			switch {
			case m.showKeys:
				m.showKeys = false
			case m.filterText != "":
				m.filterText = ""
				m.filterInput.SetValue("")
				m.applyFilter()
			default:
				return m, tea.Quit
			}
			return m, nil
		case "?":
			m.showKeys = !m.showKeys
			return m, nil
		case "/":
			m.filtering = true
			m.filterInput.Focus()
			return m, textinput.Blink
		case "ctrl+d":
			m.detailScroll += 5
			return m, nil
		case "ctrl+u":
			m.detailScroll = max(m.detailScroll-5, 0)
			return m, nil
		}
		m.action = map[string]string{"n": "add", "i": "import", "e": "edit", "d": "remove", "s": "sync", "r": "restore"}[msg.String()]
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
	prev := m.list.Index()
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	if m.list.Index() != prev {
		m.detailScroll = 0
	}
	return m, cmd
}

func (m mcpListModel) View() string {
	bodyHeight := max(m.height-frameChrome, 6)
	count := countNoun(len(m.allItems), "server")
	if m.filterText != "" {
		count = formatNumber(len(m.list.Items())) + " of " + count
	}
	title := renderFrameTitle(m.width, "mcp", []string{m.scope, count}, nil)
	if m.width < tuiMinSplitWidth {
		detailHeight := max(bodyHeight-m.list.Height()-1, 4)
		detail := lipgloss.NewStyle().Height(detailHeight).MaxHeight(detailHeight).PaddingLeft(1).
			Render(m.renderRight(m.width-2, detailHeight))
		return title + "\n\n" + m.list.View() + "\n\n" + detail + "\n" + m.renderBottom()
	}
	leftWidth := listPanelWidth(m.width)
	rightWidth := m.width - leftWidth
	return title + "\n\n" +
		renderFrameSplit(m.list.View(), m.renderRight(rightWidth-2, bodyHeight), leftWidth, rightWidth, bodyHeight) + "\n" +
		m.renderBottom()
}

// renderRight renders the selected server, or the key list while ? is on.
func (m mcpListModel) renderRight(width, height int) string {
	if m.showKeys {
		return renderKeysPanel(mcpKeyGroups)
	}
	if len(m.allItems) == 0 {
		return theme.Dim().Render("No MCP servers yet. Press n to add one, or i to import\nthe servers your Agents already have.")
	}
	item, ok := m.list.SelectedItem().(mcpListItem)
	if !ok {
		return ""
	}
	detail, _ := wrapAndScroll(item.detail, width, m.detailScroll, height)
	return detail
}

// renderBottom renders the last action's result on the note line and the
// key line; the filter input takes over the key line in place.
func (m mcpListModel) renderBottom() string {
	note := ""
	if m.notice != "" {
		note = theme.Dim().Render(truncateANSI("  "+m.notice, m.width))
	}
	var line string
	switch {
	case m.filtering:
		line = renderFilterLine(m.width, m.filterInput.View(), len(m.list.Items()))
	case m.showKeys:
		line = renderKeyLine(m.width, []keyHint{{"?/esc", "close"}}, "")
	default:
		hints := []keyHint{{"↑↓", "move"}, {"n", "add"}, {"i", "import"}, {"e", "edit"}, {"d", "remove"}, {"s", "sync"}, {"?", "keys"}}
		if len(m.allItems) == 0 {
			hints = []keyHint{{"n", "add"}, {"i", "import"}, {"r", "restore"}, {"q", "quit"}}
		}
		line = renderKeyLine(m.width, hints, framePosition(m.list.Index()+1, len(m.list.Items())))
	}
	return note + "\n" + line
}

// mcpKeyGroups lists every key for the ? panel.
var mcpKeyGroups = []keyGroup{
	{"Move", []keyHint{
		{"↑↓", "move"},
		{"/", "filter"},
		{"ctrl+d/u", "scroll the details"},
		{"esc", "clear the filter, then quit"},
		{"q", "quit"},
	}},
	{"Servers", []keyHint{
		{"n", "add a server"},
		{"i", "import servers from your Agents"},
		{"e", "edit the server"},
		{"d", "remove the server"},
	}},
	{"Agents", []keyHint{
		{"s", "sync to your Agents"},
		{"r", "restore an Agent file from a backup"},
	}},
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
	fmt.Println() // apart from the plan printed above
	_, err = chooseMCP(prompts, checklistConfig{title: "Apply these MCP changes?", items: []checklistItemData{{label: "Sync now", desc: "Only managed Agent entries will be changed"}}, singleSelect: true})
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

// mcpTargetSummary names the receiving Agents, or says that a server is kept in Skillshare only.
func mcpTargetSummary(targets []string) string {
	if len(targets) == 0 {
		return "no targets"
	}
	return strings.Join(targets, ", ")
}
