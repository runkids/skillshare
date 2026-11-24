package main

import (
	"maps"
	"slices"
	"strings"

	"skillshare/internal/mcp"
	"skillshare/internal/ui"
)

// printMCPList prints mcp list: the pending plan, then servers no sync writes anywhere.
func printMCPList(p *mcp.Plan, source *mcp.Source, shared []mcpSharedServer) error {
	// The plan lists what a sync would touch, which leaves out a server no Agent receives.
	parked := mcpServersWithoutTargets(source, p)
	// An empty plan says no server is configured, which is untrue while one waits here.
	if len(parked) == 0 && len(shared) == 0 || len(p.Changes) > 0 {
		if err := printMCPPlan(p, false); err != nil {
			return err
		}
	} else {
		printMCPSource(p.SourcePath)
	}
	names := make([]string, len(parked))
	for i, server := range parked {
		names[i] = server[0]
	}
	for _, server := range shared {
		names = append(names, server.Name)
	}
	width := ui.RowWidth(names...)
	for _, server := range parked {
		ui.Row(ui.MarkNone, server[0], "kept  "+ui.DimText(server[1]), width)
	}
	// A shared server loads only in the projects that use it; name them, or say none do.
	for _, server := range shared {
		users := "not used by any project"
		if len(server.Projects) > 0 {
			users = strings.Join(server.Projects, ", ")
		}
		ui.Row(ui.MarkNone, server.Name, "shared  "+ui.DimText(users), width)
	}
	return nil
}

// mcpSharedServer is a server set to global: false and the mcp.projects roots that use it.
type mcpSharedServer struct {
	Name     string   `json:"name"`
	Projects []string `json:"projects"`
}

func mcpSharedServers(source *mcp.Source) []mcpSharedServer {
	var out []mcpSharedServer
	for _, name := range slices.Sorted(maps.Keys(source.Servers)) {
		if source.Servers[name].Shared() {
			out = append(out, mcpSharedServer{Name: name, Projects: append([]string{}, source.Users(name)...)})
		}
	}
	return out
}
