package main

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"skillshare/internal/mcp"
)

func runMCPAdd(service *mcp.Service, o mcpOptions) error {
	if o.name == "" || o.url == "" && len(o.command) == 0 {
		if !mcpInteractive(o) {
			return fmt.Errorf("provide a name and --url URL or -- command args; run without flags in a terminal for guided setup")
		}
		return mcpAddWizard(service, o)
	}
	server := mcp.Server{URL: o.url, Targets: o.targets}
	if len(o.command) > 0 {
		server.Command, server.Args = o.command[0], o.command[1:]
	}
	source, err := mcp.LoadSource(service.ConfigPath)
	if err != nil {
		return err
	}
	if _, exists := source.Servers[o.name]; exists && !o.replace {
		return fmt.Errorf("MCP %s already exists; use --replace to explicitly replace its source definition", o.name)
	}
	return finishMCPMutation(service, mcp.Mutation{Name: o.name, Server: &server}, o, time.Now())
}

func finishMCPMutation(service *mcp.Service, mutation mcp.Mutation, o mcpOptions, start time.Time) error {
	mutation.Replace = o.replace
	if o.dryRun {
		p, err := service.PreviewMutation(mutation)
		if err != nil {
			return err
		}
		if err := printMCPPlan(p, o.json); err != nil {
			return err
		}
		if p.Blocked {
			return fmt.Errorf("MCP conflicts found; no files changed")
		}
		return nil
	}
	result, err := service.Mutate(mutation, o.revision, o.sync)
	logMCPOp(service.ConfigPath, "mcp configure", start, err)
	if result != nil {
		if outputErr := printMCPResult(result, o.json); outputErr != nil {
			return outputErr
		}
	}
	return err
}

func runMCPImport(service *mcp.Service, o mcpOptions) error {
	if o.from == "" && o.file == "" {
		if !mcpInteractive(o) {
			return fmt.Errorf("import requires --from <client> or --file <path>")
		}
		items := mcpTargetItems(service, nil)
		selected, err := runChecklistTUI(checklistConfig{title: "Import MCP from which Agent?", singleSelect: true, items: items})
		if err != nil {
			return err
		}
		if len(selected) == 0 {
			return nil
		}
		o.from = items[selected[0]].label
	}
	var candidates []mcp.Candidate
	var err error
	if o.file != "" {
		data, readErr := os.ReadFile(o.file)
		if readErr != nil {
			return readErr
		}
		format := o.from
		if format == "" && strings.HasSuffix(o.file, ".toml") {
			format = "codex"
		}
		candidates, err = mcp.Import(format, data, o.name)
	} else {
		candidates, err = service.ImportClient(o.from)
	}
	if err != nil {
		return err
	}
	if o.name == "" {
		if !mcpInteractive(o) {
			return json.NewEncoder(os.Stdout).Encode(candidates)
		}
		return mcpBatchImportWizard(service, candidates, o, terminalMCPPrompts{})
	}
	for _, c := range candidates {
		if c.Name != o.name {
			continue
		}
		if len(c.Problems) > 0 {
			return fmt.Errorf("cannot import %s: %s", c.Name, strings.Join(c.Problems, "; "))
		}
		if len(c.Warnings) > 0 && !o.json {
			for _, w := range c.Warnings {
				fmt.Println(w)
			}
		}
		source, err := mcp.LoadSource(service.ConfigPath)
		if err != nil {
			return err
		}
		if _, exists := source.Servers[c.Name]; exists && !o.replace {
			return fmt.Errorf("MCP source entry exists; use --replace")
		}
		c.Server.Targets = o.targets
		selectedTargets := c.Server.Targets
		if selectedTargets == nil {
			selectedTargets = source.Targets
		}
		mutation, err := mcpImportMutation(service, c, selectedTargets, o)
		if err != nil {
			return err
		}
		return finishMCPMutation(service, mutation, o, time.Now())
	}
	return fmt.Errorf("MCP server %q not found in import", o.name)
}

// mcpImportMutation adopts the imported client's own entry when it already
// matches and rewrites it only with --replace: a converted entry (a literal
// token becoming an environment reference) would otherwise silently change an
// Agent that works today.
func mcpImportMutation(service *mcp.Service, c mcp.Candidate, targets []string, o mcpOptions) (mcp.Mutation, error) {
	mutation := mcp.Mutation{Name: c.Name, Server: &c.Server, Replace: o.replace}
	if c.From == "" || !slices.Contains(targets, c.From) {
		return mutation, nil
	}
	if o.replace {
		mutation.Resolutions = []mcp.Resolution{{Target: c.From, Name: c.Name, Action: "replace"}}
		return mutation, nil
	}
	mutation.Resolutions = []mcp.Resolution{{Target: c.From, Name: c.Name, Action: "adopt"}}
	p, err := service.PreviewMutation(mutation)
	if err != nil {
		return mutation, err
	}
	for _, change := range p.Changes {
		if change.Target == c.From && change.Name == c.Name && change.Action == "conflict" {
			return mutation, fmt.Errorf("%s already has %s with different settings, such as a literal token that import turned into an environment reference; set any reported variables, then rerun with --replace to rewrite it, or leave %s out of --target", c.From, c.Name, c.From)
		}
	}
	return mutation, nil
}
