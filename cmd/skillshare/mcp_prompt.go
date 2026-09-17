package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

	"skillshare/internal/mcp"
)

type mcpInput struct {
	title     string
	input     textarea.Model
	cancelled bool
}

func (m mcpInput) Init() tea.Cmd { return textarea.Blink }
func (m mcpInput) View() string {
	return m.title + "\n" + m.input.View() + "\nCtrl+D: continue · Esc: cancel\n"
}
func (m mcpInput) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+d":
			return m, tea.Quit
		case "esc", "ctrl+c":
			m.cancelled = true
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func promptMCPText(title, initial string) (string, error) {
	input := textarea.New()
	input.SetWidth(76)
	input.SetHeight(6)
	input.CharLimit = 1024 * 1024
	input.SetValue(initial)
	input.Focus()
	result, err := tea.NewProgram(mcpInput{title: title, input: input}).Run()
	if err != nil {
		return "", err
	}
	m := result.(mcpInput)
	if m.cancelled {
		return "", errMCPCancelled
	}
	return strings.TrimSpace(m.input.Value()), nil
}

func mcpTargetItems(service *mcp.Service, server *mcp.Server) []checklistItemData {
	items := []checklistItemData{}
	paths := service.ClientPaths()
	for _, name := range mcp.Targets {
		if paths[name] == "" {
			continue
		}
		if server != nil {
			if _, err := mcp.Render(name, *server); err != nil {
				continue
			}
		}
		items = append(items, checklistItemData{label: name})
	}
	return items
}

func mcpAddWizard(service *mcp.Service, o mcpOptions) error {
	input, err := promptMCPText("Paste an MCP URL or server JSON (nothing will be executed)", o.url)
	if err != nil {
		return err
	}
	name := o.name
	if name == "" {
		name, err = promptMCPText("Choose a name for this MCP", "")
		if err != nil {
			return err
		}
	}
	candidate := mcp.Candidate{Name: name, Server: mcp.Server{URL: input}}
	if strings.HasPrefix(input, "{") {
		candidates, err := mcp.Import("", []byte(input), name)
		if err != nil {
			return err
		}
		if len(candidates) != 1 {
			return fmt.Errorf("paste one server at a time, or use mcp import --file to select a server")
		}
		candidate = candidates[0]
	}
	return mcpCandidateWizard(service, candidate, o)
}

func mcpCandidateWizard(service *mcp.Service, c mcp.Candidate, o mcpOptions) error {
	if len(c.Problems) > 0 {
		return fmt.Errorf("cannot import: %s", strings.Join(c.Problems, "; "))
	}
	for _, warning := range c.Warnings {
		fmt.Println(warning)
	}
	source, err := mcp.LoadSource(service.ConfigPath)
	if err != nil {
		return err
	}
	initial := o.targets
	if initial == nil {
		initial = source.Targets
	}
	prompts := terminalMCPPrompts{}
	c.Server.Targets, err = chooseMCPTargets(service, []mcp.Server{c.Server}, initial, prompts)
	if err != nil {
		return err
	}
	mutation, err := mcpImportMutation(service, c, c.Server.Targets, o)
	if err != nil {
		return err
	}
	if err := source.CheckUnchanged(); err != nil {
		return err
	}
	return reviewMCPMutations(service, []mcp.Mutation{mutation}, o, prompts)
}
