package main

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"

	"skillshare/internal/mcp"
)

var errMCPCancelled = errors.New("MCP operation cancelled; no changes saved")

// The same opt-out and terminal rules as skills, plus stdin for input prompts.
func mcpInteractive(o mcpOptions) bool {
	return !o.json && shouldLaunchTUI(o.noTUI, nil) && term.IsTerminal(int(os.Stdin.Fd()))
}

type mcpPrompts interface {
	choose(checklistConfig) ([]int, error)
	text(title, initial string) (string, error)
	review(string, *mcp.Plan) error
}

type terminalMCPPrompts struct{}

func (terminalMCPPrompts) choose(c checklistConfig) ([]int, error) { return runChecklistTUI(c) }
func (terminalMCPPrompts) text(title, initial string) (string, error) {
	return promptMCPText(title, initial)
}

func chooseMCP(p mcpPrompts, c checklistConfig) ([]int, error) {
	selected, err := p.choose(c)
	if err != nil {
		return nil, err
	}
	if len(selected) == 0 {
		return nil, errMCPCancelled
	}
	return selected, nil
}

func mcpServerNames(source *mcp.Source) []string {
	names := make([]string, 0, len(source.Servers))
	for name := range source.Servers {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func chooseMCPServer(source *mcp.Source, name, title string, p mcpPrompts) (string, error) {
	if name != "" {
		if _, ok := source.Servers[name]; !ok {
			return "", fmt.Errorf("MCP server %q not found", name)
		}
		return name, nil
	}
	names := mcpServerNames(source)
	if len(names) == 0 {
		return "", fmt.Errorf("no MCP servers configured; run skillshare mcp add")
	}
	items := make([]checklistItemData, len(names))
	for i, name := range names {
		items[i] = checklistItemData{label: name, desc: mcpConnectionSummary(source.Servers[name])}
	}
	selected, err := chooseMCP(p, checklistConfig{title: title, header: "Source: " + source.Path, items: items, singleSelect: true, itemName: "server"})
	if err != nil {
		return "", err
	}
	return names[selected[0]], nil
}

func chooseMCPTargets(service *mcp.Service, servers []mcp.Server, initial []string, p mcpPrompts) ([]string, error) {
	items := mcpTargetItems(service, nil)
	available := items[:0]
	for _, item := range items {
		compatible := true
		for _, server := range servers {
			if item.label == "pi" && server.PiExtension == "" {
				server.PiExtension = "pi-mcp-adapter"
			}
			if _, err := mcp.Render(item.label, server); err != nil {
				compatible = false
				break
			}
		}
		if compatible {
			item.preSelected = slices.Contains(initial, item.label)
			available = append(available, item)
		}
	}
	if len(available) == 0 {
		return nil, fmt.Errorf("no compatible MCP clients in this scope")
	}
	selected, err := chooseMCP(p, checklistConfig{title: "Which Agents should receive these connections?", header: "Only clients compatible with every selected server are shown.", items: available, itemName: "target"})
	if err != nil {
		return nil, err
	}
	targets := make([]string, 0, len(selected))
	for _, i := range selected {
		targets = append(targets, available[i].label)
	}

	if slices.Contains(targets, "pi") {
		initial := ""
		for _, server := range servers {
			if server.PiExtension != "" {
				initial = server.PiExtension
				break
			}
		}
		extension, err := choosePiExtension(initial, p)
		if err != nil {
			return nil, err
		}
		for i := range servers {
			servers[i].PiExtension = extension
			if _, err := mcp.Render("pi", servers[i]); err != nil {
				return nil, err
			}
		}
	}
	return targets, nil
}

func reviewMCPMutations(service *mcp.Service, mutations []mcp.Mutation, o mcpOptions, prompts mcpPrompts) error {
	p, err := service.PreviewMutations(mutations)
	if err != nil {
		return err
	}
	if o.revision != "" && o.revision != p.Revision {
		return fmt.Errorf("MCP configuration changed since preview; preview again")
	}
	if err := printMCPPlan(p, o.json); err != nil {
		return err
	}
	if o.dryRun {
		if p.Blocked {
			return fmt.Errorf("MCP conflicts found; no files changed")
		}
		return nil
	}
	var summary strings.Builder
	for _, mutation := range mutations {
		action := "Save"
		if mutation.Remove {
			action = "Remove"
		}
		fmt.Fprintf(&summary, "%s source server: %s\n", action, mutation.Name)
	}
	if err := prompts.review(summary.String(), p); err != nil {
		return err
	}
	choices := []checklistItemData{{label: "Save only", desc: "Update the source; leave Agent files unchanged"}}
	if p.Blocked {
		for _, mutation := range mutations {
			if len(mutation.Resolutions) > 0 {
				return fmt.Errorf("resolve MCP conflicts before importing; no settings saved")
			}
		}
	} else {
		choices = append([]checklistItemData{{label: "Save and sync", desc: "Apply the previewed changes to Agent files"}}, choices...)
	}
	selected, err := chooseMCP(prompts, checklistConfig{title: "Review complete — save these MCP changes?", items: choices, singleSelect: true})
	if err != nil {
		return err
	}
	sync := !p.Blocked && selected[0] == 0
	start := time.Now()
	result, err := service.MutateBatch(mutations, p.Revision, sync)
	logMCPOp(service.ConfigPath, "mcp configure", start, err)
	if result != nil {
		if outputErr := printMCPResult(result, false); outputErr != nil {
			return outputErr
		}
	}
	return err
}

func mcpRemoveWizard(service *mcp.Service, o mcpOptions, prompts mcpPrompts) error {
	source, err := mcp.LoadSource(service.ConfigPath)
	if err != nil {
		return err
	}
	name, err := chooseMCPServer(source, o.name, "Remove which MCP server?", prompts)
	if err != nil {
		return err
	}
	if err := source.CheckUnchanged(); err != nil {
		return err
	}
	fmt.Printf("Remove %s from the source. Sync also removes its managed Agent entries.\n", name)
	return reviewMCPMutations(service, []mcp.Mutation{{Name: name, Remove: true}}, o, prompts)
}

func mcpBackupLabel(b mcp.BackupInfo) string {
	stamp, _, _ := strings.Cut(b.ID, "-")
	if nanos, err := strconv.ParseInt(stamp, 10, 64); err == nil {
		return time.Unix(0, nanos).Local().Format("2006-01-02 15:04:05") + " · " + b.ID
	}
	return b.ID
}

func runMCPRestore(service *mcp.Service, o mcpOptions, prompts mcpPrompts) error {
	interactive := mcpInteractive(o)
	if o.name == "" {
		if !interactive {
			return fmt.Errorf("usage: skillshare mcp restore <id> [--dry-run]; omit the ID in an interactive terminal to browse backups")
		}
		backups, err := service.Backups()
		if err != nil {
			return err
		}
		if len(backups) == 0 {
			return fmt.Errorf("no MCP backups for this configuration")
		}
		targets := []string{}
		for _, b := range backups {
			if !slices.Contains(targets, b.Target) {
				targets = append(targets, b.Target)
			}
		}
		slices.Sort(targets)
		items := make([]checklistItemData, len(targets))
		for i, target := range targets {
			items[i] = checklistItemData{label: target}
		}
		selected, err := chooseMCP(prompts, checklistConfig{title: "Restore backups for which Agent?", items: items, singleSelect: true})
		if err != nil {
			return err
		}
		versions := []mcp.BackupInfo{}
		items = nil
		for _, b := range backups {
			if b.Target == targets[selected[0]] {
				versions = append(versions, b)
				items = append(items, checklistItemData{label: mcpBackupLabel(b), desc: b.Path})
			}
		}
		selected, err = chooseMCP(prompts, checklistConfig{title: "Choose an MCP backup (newest first)", items: items, singleSelect: true})
		if err != nil {
			return err
		}
		o.name = versions[selected[0]].ID
	}
	p, err := service.PreviewRestore(o.name)
	if err != nil {
		return err
	}
	if o.revision != "" && o.revision != p.Revision {
		return fmt.Errorf("MCP configuration changed since preview; preview again")
	}
	if o.dryRun || interactive {
		if err := printMCPPlan(p, o.json); err != nil {
			return err
		}
	}
	if p.Blocked {
		return fmt.Errorf("MCP restore conflicts found; no files changed")
	}
	if o.dryRun {
		return nil
	}
	if interactive {
		if err := prompts.review("Restore backup: "+o.name, p); err != nil {
			return err
		}
		_, err := chooseMCP(prompts, checklistConfig{title: "Restore the previewed Agent entries?", header: "The source definition stays unchanged. Esc cancels.", items: []checklistItemData{{label: "Restore backup", desc: o.name}}, singleSelect: true})
		if err != nil {
			return err
		}
	}
	start := time.Now()
	result, err := service.Restore(o.name, p.Revision)
	logMCPOp(service.ConfigPath, "mcp restore", start, err)
	if result != nil {
		if outputErr := printMCPResult(result, o.json); outputErr != nil {
			return outputErr
		}
	}
	return err
}

func choosePiExtension(initial string, p mcpPrompts) (string, error) {
	packages := []string{"pi-mcp-adapter", "pi-mcp-extension"}
	items := []checklistItemData{
		{label: packages[0], desc: "On-demand tools; supports environment-backed headers", preSelected: initial == packages[0]},
		{label: packages[1], desc: "Direct tools; start with /mcp:start <server>; no header interpolation", preSelected: initial == packages[1]},
	}
	selected, err := chooseMCP(p, checklistConfig{title: "Which MCP extension do you use in Pi?", header: "Install ONE with pi install npm:<package>. Sync only writes config; it does not install or verify the extension. Docs: https://pi.dev/packages/pi-mcp-adapter and https://pi.dev/packages/pi-mcp-extension", items: items, singleSelect: true, itemName: "extension"})
	if err != nil {
		return "", err
	}
	return packages[selected[0]], nil
}
