package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"skillshare/internal/mcp"
)

func runMCPEdit(service *mcp.Service, o mcpOptions) error {
	source, err := mcp.LoadSource(service.ConfigPath)
	if err != nil {
		return err
	}
	interactive := mcpInteractive(o)
	if o.name == "" && !interactive {
		return fmt.Errorf("usage: skillshare mcp edit <name> --url URL, --target CLIENT or -- command args; omit options in a terminal for the editor")
	}
	prompts := terminalMCPPrompts{}
	name, err := chooseMCPServer(source, o.name, "Edit which MCP server?", prompts)
	if err != nil {
		return err
	}
	server := source.Servers[name]
	if o.url != "" && len(o.command) > 0 {
		return fmt.Errorf("choose either --url or -- command args")
	}
	if o.piExtension != "" || o.url != "" || len(o.command) > 0 || o.targets != nil {
		server = patchMCPServer(server, o)
		if err := source.CheckUnchanged(); err != nil {
			return err
		}
		o.replace = true
		return finishMCPMutation(service, mcp.Mutation{Name: name, Server: &server, Replace: true}, o, time.Now())
	}
	if !interactive {
		return fmt.Errorf("provide --url, --target or -- command args with --no-tui/--json; omit --no-tui for the editor")
	}
	server, err = editMCPDraft(service, name, server, source.Targets, prompts)
	if err != nil {
		return err
	}
	if err := source.CheckUnchanged(); err != nil {
		return err
	}
	return reviewMCPMutations(service, []mcp.Mutation{{Name: name, Server: &server, Replace: true}}, o, prompts)
}

func patchMCPServer(server mcp.Server, o mcpOptions) mcp.Server {
	if o.piExtension != "" {
		server.PiExtension = o.piExtension
	}
	if o.url != "" {
		server.Command = ""
		server.Args = nil
		server.Env = nil
		server.URL = o.url
		server.Transport = ""
	}
	if len(o.command) > 0 {
		server.URL = ""
		server.Headers = nil
		server.BearerToken = nil
		server.Command = o.command[0]
		server.Args = o.command[1:]
		server.Transport = ""
	}
	if o.targets != nil {
		server.Targets = slices.Clone(o.targets)
	}
	return server
}

func editMCPDraft(service *mcp.Service, name string, server mcp.Server, defaults []string, prompts mcpPrompts) (mcp.Server, error) {
	// Edits are a private draft; Esc anywhere leaves the source untouched.
	server.Env, server.Headers = maps.Clone(server.Env), maps.Clone(server.Headers)
	for {
		targets := server.Targets
		if targets == nil {
			targets = defaults
		}
		items := []checklistItemData{
			{label: "Connection", desc: mcpConnectionSummary(server)},
			{label: "Arguments", desc: "Local command arguments, one per line or a JSON array"},
			{label: "Environment variables", desc: fmt.Sprintf("%d variables (values hidden)", len(server.Env))},
			{label: "HTTP headers", desc: fmt.Sprintf("%d headers (values hidden)", len(server.Headers))},
			{label: "Bearer token", desc: "Environment variable name only"},
			{label: "Targets", desc: strings.Join(targets, ", ")},
			{label: "Review changes", desc: "Preview before saving"},
			{label: "Pi extension", desc: server.PiExtension},
		}
		selected, err := chooseMCP(prompts, checklistConfig{title: "Edit MCP: " + name, header: "Esc cancels this draft without saving.", items: items, singleSelect: true})
		if err != nil {
			return server, err
		}
		switch selected[0] {
		case 0:
			kind, err := chooseMCP(prompts, checklistConfig{title: "Connection type", items: []checklistItemData{{label: "Local command (stdio)", preSelected: server.Command != ""}, {label: "Remote URL (Streamable HTTP)", preSelected: server.URL != ""}}, singleSelect: true})
			if err != nil {
				return server, err
			}
			if kind[0] == 0 {
				command, err := prompts.text("Executable only (for example npx); edit arguments separately", server.Command)
				if err != nil {
					return server, err
				}
				if command == "" {
					continue
				}
				server.URL, server.Command, server.Transport = "", command, ""
				server.Headers, server.BearerToken = nil, nil
			} else {
				url, err := prompts.text("MCP URL (https://…/mcp)", server.URL)
				if err != nil {
					return server, err
				}
				if url == "" {
					continue
				}
				server = patchMCPServer(server, mcpOptions{url: url})
			}
		case 1:
			if server.Command == "" {
				fmt.Println("Arguments apply to local commands. Change the connection type first.")
				continue
			}
			initial, _ := json.Marshal(server.Args)
			if server.Args == nil {
				initial = []byte("[]")
			}
			text, err := prompts.text("Arguments: one per line, or a JSON array; empty clears all arguments", string(initial))
			if err != nil {
				return server, err
			}
			args, err := parseMCPArgumentInput(text)
			if err != nil {
				fmt.Println(err)
				continue
			}
			server.Args = args
		case 2:
			if server.Command == "" {
				fmt.Println("Environment variables apply to local commands.")
				continue
			}
			server.Env, err = editMCPValues("Environment variables", server.Env, prompts)
		case 3:
			if server.URL == "" {
				fmt.Println("HTTP headers apply to remote URLs.")
				continue
			}
			server.Headers, err = editMCPValues("HTTP headers", server.Headers, prompts)
		case 4:
			if server.URL == "" {
				fmt.Println("Bearer tokens apply to remote URLs.")
				continue
			}
			initial := ""
			if server.BearerToken != nil {
				initial = server.BearerToken.FromEnv
			}
			var name string
			name, err = prompts.text("Bearer token environment variable (for example MCP_TOKEN); empty removes it", initial)
			if name == "" {
				server.BearerToken = nil
			} else {
				server.BearerToken = &mcp.Value{FromEnv: name}
			}
		case 5:
			servers := []mcp.Server{server}
			server.Targets, err = chooseMCPTargets(service, servers, targets, prompts)
			server.PiExtension = servers[0].PiExtension
		case 7:
			server.PiExtension, err = choosePiExtension(server.PiExtension, prompts)
		case 6:
			if err := server.Validate(name); err != nil {
				fmt.Println(err)
				continue
			}
			return server, nil
		}
		if err != nil {
			return server, err
		}
	}
}

func parseMCPArgumentInput(text string) ([]string, error) {
	if text == "" {
		return nil, nil
	}
	if strings.HasPrefix(strings.TrimSpace(text), "[") {
		var args []string
		if err := json.Unmarshal([]byte(text), &args); err != nil {
			return nil, fmt.Errorf("arguments must be a JSON array of strings or one literal argument per line")
		}
		return args, nil
	}
	return strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n"), nil
}

func editMCPValues(title string, values map[string]mcp.Value, prompts mcpPrompts) (map[string]mcp.Value, error) {
	values = maps.Clone(values)
	if values == nil {
		values = map[string]mcp.Value{}
	}
	for {
		keys := make([]string, 0, len(values))
		for key := range values {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		items := []checklistItemData{{label: "Done"}, {label: "Add variable/header"}}
		for _, key := range keys {
			items = append(items, checklistItemData{label: key, desc: "Edit or remove (value hidden)"})
		}
		selected, err := chooseMCP(prompts, checklistConfig{title: title, items: items, singleSelect: true})
		if err != nil {
			return values, err
		}
		if selected[0] == 0 {
			return values, nil
		}
		key := ""
		if selected[0] >= 2 {
			key = keys[selected[0]-2]
		} else {
			key, err = prompts.text("Variable/header name", "")
			if err != nil {
				return values, err
			}
			if key == "" {
				continue
			}
			if _, exists := values[key]; exists {
				fmt.Println("That name already exists; select it to edit.")
				continue
			}
		}
		kind, err := chooseMCP(prompts, checklistConfig{title: key + ": value source", items: []checklistItemData{{label: "Environment reference", desc: "Recommended for credentials; only the variable name is saved"}, {label: "Literal value", desc: "For non-secret values only"}, {label: "Remove"}}, singleSelect: true})
		if err != nil {
			return values, err
		}
		if kind[0] == 2 {
			delete(values, key)
			continue
		}
		initial, label := values[key].FromEnv, "Environment variable name (not its secret value)"
		if kind[0] == 1 {
			initial, label = values[key].Literal, "Literal non-secret value"
		}
		text, err := prompts.text(label, initial)
		if err != nil {
			return values, err
		}
		if kind[0] == 0 {
			if text == "" {
				continue
			}
			values[key] = mcp.Value{FromEnv: text}
		} else {
			values[key] = mcp.Value{Literal: text}
		}
	}
}
