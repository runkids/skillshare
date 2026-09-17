package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"skillshare/internal/mcp"
)

type mcpReviewModel struct {
	viewport viewport.Model
	content  string
	accepted bool
}

func (m mcpReviewModel) Init() tea.Cmd { return nil }

func (m mcpReviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewport.Width, m.viewport.Height = max(10, msg.Width-4), max(3, msg.Height-4)
		m.viewport.SetContent(hardWrapContent(m.content, m.viewport.Width))
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			m.accepted = true
			return m, tea.Quit
		case "esc", "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m mcpReviewModel) View() string {
	return "MCP change preview\n\n" + m.viewport.View() + "\n" + formatHelpBar("↑↓ scroll  Enter continue  Esc cancel")
}

func (terminalMCPPrompts) review(summary string, plan *mcp.Plan) error {
	var content strings.Builder
	fmt.Fprintf(&content, "%s\n\nSource: %s\n", summary, plan.SourcePath)
	if plan.Blocked {
		content.WriteString("\nConflicts found. Agent files cannot be synced.\n")
	}
	for _, change := range plan.Changes {
		fmt.Fprintf(&content, "\n%s · %s · %s\n  %s\n", change.Target, change.Name, change.Action, change.Path)
		if change.Message != "" {
			fmt.Fprintf(&content, "  %s\n", change.Message)
		}
	}
	m := mcpReviewModel{viewport: viewport.New(76, 20), content: content.String()}
	m.viewport.SetContent(hardWrapContent(m.content, m.viewport.Width))
	result, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return err
	}
	if !result.(mcpReviewModel).accepted {
		return errMCPCancelled
	}
	return nil
}
