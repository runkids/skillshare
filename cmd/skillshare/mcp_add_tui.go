package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"skillshare/internal/mcp"
	"skillshare/internal/ui"
)

type mcpAddPhase int

const (
	mcpPhaseNameInput    mcpAddPhase = iota // server name
	mcpPhaseTransport                       // stdio or remote (radio: up/down/enter)
	mcpPhaseCommandInput                    // command (if stdio)
	mcpPhaseURLInput                        // url (if remote)
	mcpPhaseArgsInput                       // args, space-separated (stdio only)
	mcpPhaseEnvInput                        // env KEY=VALUE (one per line, empty to finish)
	mcpPhaseConfirm                         // summary + y/n
)

type mcpAddTUIModel struct {
	phase    mcpAddPhase
	name     string
	isRemote bool
	command  string
	url      string
	argsStr  string   // space-separated
	envPairs []string // KEY=VALUE entries

	textInput textinput.Model
	currRadio int // 0=stdio, 1=remote

	done      bool
	cancelled bool
	err       error

	// Result
	mcpConfigPath string
}

func newMCPAddTUIModel(mcpConfigPath string) mcpAddTUIModel {
	ti := textinput.New()
	ti.Placeholder = "my-server"
	ti.Focus()
	ti.PromptStyle = tc.Cyan
	ti.Cursor.Style = tc.Cyan

	return mcpAddTUIModel{
		phase:         mcpPhaseNameInput,
		textInput:     ti,
		mcpConfigPath: mcpConfigPath,
	}
}

func (m mcpAddTUIModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m mcpAddTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.cancelled = true
			return m, tea.Quit
		case "esc":
			switch m.phase {
			case mcpPhaseNameInput:
				m.cancelled = true
				return m, tea.Quit
			case mcpPhaseTransport:
				m.phase = mcpPhaseNameInput
				m.textInput.SetValue(m.name)
				m.textInput.Placeholder = "my-server"
				m.textInput.Focus()
				return m, nil
			case mcpPhaseCommandInput:
				m.phase = mcpPhaseTransport
				return m, nil
			case mcpPhaseURLInput:
				m.phase = mcpPhaseTransport
				return m, nil
			case mcpPhaseArgsInput:
				m.phase = mcpPhaseCommandInput
				m.textInput.SetValue(m.command)
				m.textInput.Placeholder = "npx -y @some/mcp-server"
				m.textInput.Focus()
				return m, nil
			case mcpPhaseEnvInput:
				if m.isRemote {
					m.phase = mcpPhaseURLInput
					m.textInput.SetValue(m.url)
					m.textInput.Placeholder = "https://api.example.com/mcp"
					m.textInput.Focus()
				} else {
					m.phase = mcpPhaseArgsInput
					m.textInput.SetValue(m.argsStr)
					m.textInput.Placeholder = "arg1 arg2"
					m.textInput.Focus()
				}
				return m, nil
			case mcpPhaseConfirm:
				m.phase = mcpPhaseEnvInput
				m.textInput.SetValue("")
				m.textInput.Placeholder = "KEY=VALUE (empty to skip)"
				m.textInput.Focus()
				return m, nil
			}
		}

		switch m.phase {
		case mcpPhaseNameInput:
			switch msg.String() {
			case "enter":
				name := strings.TrimSpace(m.textInput.Value())
				if name == "" {
					return m, nil
				}
				m.name = name
				m.err = nil
				m.phase = mcpPhaseTransport
				m.textInput.Blur()
				return m, nil
			}

		case mcpPhaseTransport:
			switch msg.String() {
			case "up", "k":
				if m.currRadio > 0 {
					m.currRadio--
				}
				return m, nil
			case "down", "j":
				if m.currRadio < 1 {
					m.currRadio++
				}
				return m, nil
			case "enter", " ":
				m.isRemote = m.currRadio == 1
				if m.isRemote {
					m.phase = mcpPhaseURLInput
					m.textInput.SetValue("")
					m.textInput.Placeholder = "https://api.example.com/mcp"
					m.textInput.Focus()
				} else {
					m.phase = mcpPhaseCommandInput
					m.textInput.SetValue("")
					m.textInput.Placeholder = "npx -y @some/mcp-server"
					m.textInput.Focus()
				}
				return m, nil
			}
			return m, nil

		case mcpPhaseCommandInput:
			switch msg.String() {
			case "enter":
				cmd := strings.TrimSpace(m.textInput.Value())
				if cmd == "" {
					return m, nil
				}
				m.command = cmd
				m.err = nil
				m.phase = mcpPhaseArgsInput
				m.textInput.SetValue("")
				m.textInput.Placeholder = "arg1 arg2"
				return m, nil
			}

		case mcpPhaseURLInput:
			switch msg.String() {
			case "enter":
				url := strings.TrimSpace(m.textInput.Value())
				if url == "" {
					return m, nil
				}
				m.url = url
				m.err = nil
				m.phase = mcpPhaseEnvInput
				m.textInput.SetValue("")
				m.textInput.Placeholder = "KEY=VALUE (empty to skip)"
				return m, nil
			}

		case mcpPhaseArgsInput:
			switch msg.String() {
			case "enter":
				m.argsStr = strings.TrimSpace(m.textInput.Value())
				m.phase = mcpPhaseEnvInput
				m.textInput.SetValue("")
				m.textInput.Placeholder = "KEY=VALUE (empty to skip)"
				return m, nil
			}

		case mcpPhaseEnvInput:
			switch msg.String() {
			case "enter":
				val := strings.TrimSpace(m.textInput.Value())
				if val == "" {
					// Empty input = done adding env vars
					m.phase = mcpPhaseConfirm
					m.textInput.Blur()
					return m, nil
				}
				// Validate KEY=VALUE format
				if !strings.Contains(val, "=") {
					m.err = fmt.Errorf("invalid format %q: expected KEY=VALUE", val)
					return m, nil
				}
				m.envPairs = append(m.envPairs, val)
				m.err = nil
				m.textInput.SetValue("")
				return m, nil
			}

		case mcpPhaseConfirm:
			switch msg.String() {
			case "y", "Y", "enter":
				m.done = true
				return m, tea.Quit
			case "n", "N":
				m.cancelled = true
				return m, tea.Quit
			}
			return m, nil
		}
	}

	// Delegate to textinput for typing phases
	switch m.phase {
	case mcpPhaseNameInput, mcpPhaseCommandInput, mcpPhaseURLInput, mcpPhaseArgsInput, mcpPhaseEnvInput:
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m mcpAddTUIModel) View() string {
	var b strings.Builder

	b.WriteString(tc.Title.Render("MCP Add Server"))
	b.WriteString("\n\n")

	switch m.phase {
	case mcpPhaseNameInput:
		b.WriteString(tc.Cyan.Render("Server name: "))
		b.WriteString(m.textInput.View())
		if m.err != nil {
			b.WriteString("\n" + tc.Red.Render(m.err.Error()))
		}
		b.WriteString("\n\n")
		b.WriteString(tc.Help.Render("enter confirm  esc cancel"))

	case mcpPhaseTransport:
		b.WriteString(tc.Dim.Render(fmt.Sprintf("Name: %s", m.name)))
		b.WriteString("\n\n")
		b.WriteString(tc.Cyan.Render("Transport:"))
		b.WriteString("\n")
		transports := []struct {
			label string
			desc  string
		}{
			{"stdio", "(command-based, local process)"},
			{"remote", "(HTTP/SSE url)"},
		}
		for i, t := range transports {
			cursor := "  "
			if i == m.currRadio {
				cursor = "▸ "
			}
			if i == m.currRadio {
				b.WriteString(tc.Cyan.Render(cursor+t.label) + " " + tc.Dim.Render(t.desc))
			} else {
				b.WriteString(tc.Dim.Render(cursor + t.label + " " + t.desc))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(tc.Help.Render("↑↓/jk navigate  enter/space select  esc back"))

	case mcpPhaseCommandInput:
		b.WriteString(tc.Dim.Render(fmt.Sprintf("Name: %s  Transport: stdio", m.name)))
		b.WriteString("\n\n")
		b.WriteString(tc.Cyan.Render("Command: "))
		b.WriteString(m.textInput.View())
		if m.err != nil {
			b.WriteString("\n" + tc.Red.Render(m.err.Error()))
		}
		b.WriteString("\n\n")
		b.WriteString(tc.Help.Render("enter confirm  esc back"))

	case mcpPhaseURLInput:
		b.WriteString(tc.Dim.Render(fmt.Sprintf("Name: %s  Transport: remote", m.name)))
		b.WriteString("\n\n")
		b.WriteString(tc.Cyan.Render("URL: "))
		b.WriteString(m.textInput.View())
		if m.err != nil {
			b.WriteString("\n" + tc.Red.Render(m.err.Error()))
		}
		b.WriteString("\n\n")
		b.WriteString(tc.Help.Render("enter confirm  esc back"))

	case mcpPhaseArgsInput:
		b.WriteString(tc.Dim.Render(fmt.Sprintf("Name: %s  Command: %s", m.name, m.command)))
		b.WriteString("\n\n")
		b.WriteString(tc.Cyan.Render("Args (space-separated, optional): "))
		b.WriteString(m.textInput.View())
		b.WriteString("\n\n")
		b.WriteString(tc.Help.Render("enter confirm (or skip)  esc back"))

	case mcpPhaseEnvInput:
		transport := "stdio"
		if m.isRemote {
			transport = "remote"
		}
		b.WriteString(tc.Dim.Render(fmt.Sprintf("Name: %s  Transport: %s", m.name, transport)))
		if len(m.envPairs) > 0 {
			b.WriteString("\n")
			for _, pair := range m.envPairs {
				b.WriteString(tc.Dim.Render(fmt.Sprintf("  env: %s", pair)))
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
		b.WriteString(tc.Cyan.Render("Env var (KEY=VALUE, empty to finish): "))
		b.WriteString(m.textInput.View())
		if m.err != nil {
			b.WriteString("\n" + tc.Red.Render(m.err.Error()))
		}
		b.WriteString("\n\n")
		b.WriteString(tc.Help.Render("enter add/finish  esc back"))

	case mcpPhaseConfirm:
		b.WriteString(tc.Cyan.Render("Summary:"))
		b.WriteString("\n")
		fmt.Fprintf(&b, "  Name:      %s\n", m.name)
		transport := "stdio"
		if m.isRemote {
			transport = "remote"
			fmt.Fprintf(&b, "  URL:       %s\n", m.url)
		} else {
			fmt.Fprintf(&b, "  Command:   %s\n", m.command)
			if m.argsStr != "" {
				fmt.Fprintf(&b, "  Args:      %s\n", m.argsStr)
			}
		}
		fmt.Fprintf(&b, "  Transport: %s\n", transport)
		if len(m.envPairs) > 0 {
			for _, pair := range m.envPairs {
				fmt.Fprintf(&b, "  Env:       %s\n", pair)
			}
		}
		b.WriteString("\n")
		b.WriteString(tc.Cyan.Render("Add this server? (Y/n) "))
		b.WriteString("\n\n")
		b.WriteString(tc.Help.Render("y/enter confirm  n cancel"))
	}

	return b.String()
}

// runMCPAddTUI launches the interactive wizard for `mcp add`.
func runMCPAddTUI(mcpConfigPath string) error {
	m := newMCPAddTUIModel(mcpConfigPath)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	result, ok := finalModel.(mcpAddTUIModel)
	if !ok || result.cancelled || !result.done {
		return nil
	}

	// Load config and check for duplicates
	cfg, err := mcp.LoadMCPConfig(mcpConfigPath)
	if err != nil {
		return err
	}

	if _, exists := cfg.Servers[result.name]; exists {
		return fmt.Errorf("server %q already exists; remove it first with: skillshare mcp remove %s", result.name, result.name)
	}

	// Build MCPServer from collected values
	srv := mcp.MCPServer{}
	if result.isRemote {
		srv.URL = result.url
	} else {
		srv.Command = result.command
		if result.argsStr != "" {
			srv.Args = strings.Fields(result.argsStr)
		}
	}

	// Parse env K=V pairs
	if len(result.envPairs) > 0 {
		srv.Env = make(map[string]string)
		for _, pair := range result.envPairs {
			k, v, ok := strings.Cut(pair, "=")
			if !ok {
				return fmt.Errorf("invalid env value %q: expected KEY=VALUE", pair)
			}
			srv.Env[k] = v
		}
	}

	cfg.Servers[result.name] = srv

	if err := cfg.Validate(); err != nil {
		return err
	}

	if err := cfg.Save(mcpConfigPath); err != nil {
		return err
	}

	ui.Success("Added MCP server %q", result.name)
	fmt.Printf("%sRun 'skillshare sync mcp' to push to targets.%s\n", ui.Dim, ui.Reset)
	return nil
}
