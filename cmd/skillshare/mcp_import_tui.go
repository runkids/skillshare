package main

import (
	"fmt"
	"strings"

	"skillshare/internal/mcp"
)

func mcpBatchImportWizard(service *mcp.Service, candidates []mcp.Candidate, o mcpOptions, prompts mcpPrompts) error {
	source, err := mcp.LoadSource(service.ConfigPath)
	if err != nil {
		return err
	}
	eligible := []mcp.Candidate{}
	items := []checklistItemData{}
	for _, candidate := range candidates {
		if len(candidate.Problems) > 0 {
			fmt.Printf("Skipped %s: %s\n", candidate.Name, strings.Join(candidate.Problems, "; "))
			continue
		}
		if _, exists := source.Servers[candidate.Name]; exists && !o.replace {
			fmt.Printf("Skipped %s: already in source (use --replace to replace it)\n", candidate.Name)
			continue
		}
		eligible = append(eligible, candidate)
		items = append(items, checklistItemData{label: candidate.Name, desc: mcpConnectionSummary(candidate.Server)})
	}
	if len(items) == 0 {
		return fmt.Errorf("no new importable MCP servers")
	}
	selected, err := chooseMCP(prompts, checklistConfig{title: "Choose MCP servers to import", header: "Space toggles · a selects all · Enter continues · Esc cancels", items: items, itemName: "server"})
	if err != nil {
		return err
	}
	servers := make([]mcp.Server, 0, len(selected))
	for _, i := range selected {
		servers = append(servers, eligible[i].Server)
	}
	initial := o.targets
	if initial == nil {
		initial = source.Targets
	}
	targets, err := chooseMCPTargets(service, servers, initial, prompts)
	if err != nil {
		return err
	}
	mutations := make([]mcp.Mutation, 0, len(selected))
	for _, i := range selected {
		candidate := eligible[i]
		for _, warning := range candidate.Warnings {
			fmt.Printf("%s: %s\n", candidate.Name, warning)
		}
		candidate.Server.Targets = targets
		mutation, err := mcpImportMutation(service, candidate, targets, o)
		if err != nil {
			return err
		}
		mutations = append(mutations, mutation)
	}
	if err := source.CheckUnchanged(); err != nil {
		return err
	}
	return reviewMCPMutations(service, mutations, o, prompts)
}
