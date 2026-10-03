package main

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"skillshare/internal/mcp"
	"skillshare/internal/ui"
)

// promptMCPText asks for an MCP value inline. Values may hold secrets, so
// the answer is not echoed back.
func promptMCPText(title, initial string) (string, error) {
	v, err := ui.Text(title, initial)
	if errors.Is(err, ui.ErrCancelled) {
		return "", errMCPCancelled
	}
	return strings.TrimSpace(v), err
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
	// Accounts of an Agent follow, under the names the config gives them.
	if source, err := mcp.LoadSource(service.ConfigPath); err == nil {
		paths := service.AccountPaths(source.Accounts)
		for _, name := range slices.Sorted(maps.Keys(paths)) {
			if server != nil {
				if _, err := mcp.Render(source.Accounts[name].Agent, *server); err != nil {
					continue
				}
			}
			items = append(items, checklistItemData{label: name})
		}
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
		name, err = ui.Input("Choose a name for this MCP", "", "")
		if errors.Is(err, ui.ErrCancelled) {
			return errMCPCancelled
		}
		if err != nil {
			return err
		}
		name = strings.TrimSpace(name)
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
