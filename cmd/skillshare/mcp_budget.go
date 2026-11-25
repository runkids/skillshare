package main

import (
	"fmt"
	"slices"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/mcp"
	"skillshare/internal/ui"
)

// mcpToolBudget is the warn_mcp_tool_tokens threshold of the config the service reads.
func mcpToolBudget(service *mcp.Service) int {
	if service.ProjectRoot != "" {
		if cfg, err := config.LoadProject(service.ProjectRoot); err == nil {
			return cfg.ContextBudget.MCPToolThreshold()
		}
		return config.DefaultWarnMCPToolTokens
	}
	if cfg, err := config.Load(); err == nil {
		return cfg.ContextBudget.MCPToolThreshold()
	}
	return config.DefaultWarnMCPToolTokens
}

// mcpToolCostsJSON is the MCP part of `analyze --json`.
type mcpToolCostsJSON struct {
	Budget int                 `json:"budget"`
	Groups []mcp.ToolCostGroup `json:"groups"`
}

// mcpGroupLabel names a target, and the mcp.projects root when the group is not global.
func mcpGroupLabel(g mcp.ToolCostGroup) string {
	if g.Root == "" {
		return g.Target
	}
	return g.Target + " (" + shortenPath(g.Root) + ")"
}

// loadMCPToolCosts adds up the tool definitions of the MCP config scope ("-g" or "-p") reads.
// It returns nil when no target receives a server.
func loadMCPToolCosts(scope string) (*mcp.ToolCostReport, error) {
	service, _, err := mcpContext([]string{scope})
	if err != nil {
		return nil, err
	}
	report, err := service.ToolCosts()
	if err != nil || len(report.Groups) == 0 {
		return nil, err
	}
	return report, nil
}

// printServiceMCPToolCosts prints what the service's MCP targets' tool definitions cost.
// Until a live check has counted a server there is nothing to print, so it plans nothing.
func printServiceMCPToolCosts(service *mcp.Service) {
	if !service.HasToolCosts() {
		return
	}
	if costs, err := service.ToolCosts(); err == nil {
		printMCPToolCosts(costs, mcpToolBudget(service))
	}
}

// printMCPToolCosts summarizes what each MCP target's tool definitions cost and warns about
// every one over budget. Servers no live check has counted are named, not counted as zero.
// It prints nothing, and reports false, until a live check has counted a server.
func printMCPToolCosts(report *mcp.ToolCostReport, budget int) bool {
	if !slices.ContainsFunc(report.Groups, func(g mcp.ToolCostGroup) bool { return g.Measured > 0 }) {
		return false
	}
	var parts []string
	uncounted := map[string]bool{}
	for _, g := range report.Groups {
		for _, server := range g.Servers {
			if server.Checked == nil {
				uncounted[server.Project+"\x00"+server.Name] = true
			}
		}
		if g.Measured > 0 {
			parts = append(parts, fmt.Sprintf("%s ~%s", mcpGroupLabel(g), formatTokenK(g.Tokens)))
		}
	}
	if len(parts) > 0 {
		// Tool policies and Agents that load tools on demand can only lower these numbers.
		ui.Note("MCP tool definitions (upper bound)  " + strings.Join(parts, " · "))
	}
	for _, g := range report.Groups {
		if budget <= 0 || g.Tokens <= budget {
			continue
		}
		ui.Warning("%s: MCP tool definitions are ~%s tokens (budget: %s)", mcpGroupLabel(g), formatTokenComma(g.Tokens), formatTokenComma(budget))
		var top []string
		for _, server := range g.Servers {
			if server.Checked != nil && len(top) < 3 {
				top = append(top, fmt.Sprintf("%s ~%s", server.Name, formatTokenComma(server.Tokens)))
			}
		}
		fmt.Printf("   Largest: %s\n", strings.Join(top, ", "))
	}
	if len(uncounted) > 0 {
		ui.Note(fmt.Sprintf("%s not counted yet; run skillshare mcp check --live", plural(len(uncounted), "MCP server")))
	}
	return true
}
