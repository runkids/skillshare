package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/oplog"
	"skillshare/internal/resource"
	"skillshare/internal/trash"
	"skillshare/internal/ui"
)

// cmdUninstallAgents removes agents from the source directory by moving them to agent trash.
func cmdUninstallAgents(agentsDir string, opts *uninstallOptions, cfgPath string, start time.Time) error {
	if _, err := os.Stat(agentsDir); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("agents source directory does not exist: %s", agentsDir)
		}
		return fmt.Errorf("cannot access agents source: %w", err)
	}

	// Discover all agents for resolution
	discovered, discErr := resource.AgentKind{}.Discover(agentsDir)
	if discErr != nil {
		return fmt.Errorf("failed to discover agents: %w", discErr)
	}

	// Resolve targets
	var targets []resource.DiscoveredResource
	if opts.all {
		targets = discovered
		if len(targets) == 0 {
			ui.Info("No agents found")
			return nil
		}
	} else {
		for _, input := range opts.skillNames {
			found := false
			for _, d := range discovered {
				if d.Name == input || d.FlatName == input || d.RelPath == input || strings.TrimSuffix(d.RelPath, ".md") == input {
					targets = append(targets, d)
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("agent %q not found in %s", input, agentsDir)
			}
		}
	}

	if len(targets) == 0 {
		return fmt.Errorf("specify agent name(s) or --all")
	}

	// Confirmation (unless --force or --json)
	if !opts.force && !opts.jsonOutput {
		ui.Warning("Uninstalling %d agent(s)", len(targets))
		const maxDisplay = 20
		if len(targets) <= maxDisplay {
			for _, t := range targets {
				fmt.Printf("  - %s\n", t.Name)
			}
		} else {
			for _, t := range targets[:maxDisplay] {
				fmt.Printf("  - %s\n", t.Name)
			}
			fmt.Printf("  ... and %d more\n", len(targets)-maxDisplay)
		}
		fmt.Println()
		fmt.Print("Continue? [y/N] ")
		var input string
		fmt.Scanln(&input)
		input = strings.TrimSpace(strings.ToLower(input))
		if input != "y" && input != "yes" {
			ui.Info("Cancelled")
			return nil
		}
	}

	trashBase := trash.AgentTrashDir()
	var removed []string
	var failed []string

	for _, t := range targets {
		agentFile := filepath.Join(agentsDir, t.RelPath)
		metaName := strings.TrimSuffix(filepath.Base(t.RelPath), ".md")
		metaFile := filepath.Join(filepath.Dir(agentFile), metaName+".skillshare-meta.json")

		if opts.dryRun {
			ui.Info("[dry-run] Would remove agent: %s", t.Name)
			removed = append(removed, t.Name)
			continue
		}

		_, err := trash.MoveAgentToTrash(agentFile, metaFile, t.Name, trashBase)
		if err != nil {
			ui.Error("Failed to remove %s: %v", t.Name, err)
			failed = append(failed, t.Name)
			continue
		}

		ui.Success("Removed agent: %s", t.Name)
		removed = append(removed, t.Name)
	}

	// JSON output
	if opts.jsonOutput {
		output := struct {
			Removed  []string `json:"removed"`
			Failed   []string `json:"failed"`
			DryRun   bool     `json:"dry_run"`
			Duration string   `json:"duration"`
		}{
			Removed:  removed,
			Failed:   failed,
			DryRun:   opts.dryRun,
			Duration: formatDuration(start),
		}
		var jsonErr error
		if len(failed) > 0 {
			jsonErr = fmt.Errorf("%d agent(s) failed to uninstall", len(failed))
		}
		return writeJSONResult(&output, jsonErr)
	}

	// Summary
	if !opts.dryRun {
		fmt.Println()
		ui.Info("%d agent(s) removed, %d failed", len(removed), len(failed))
		if len(removed) > 0 {
			ui.Info("Run 'skillshare sync agents' to update targets")
		}
	}

	// Oplog
	logUninstallAgentOp(cfgPath, removed, len(removed), len(failed), opts.dryRun, start)

	if len(failed) > 0 {
		return fmt.Errorf("%d agent(s) failed to uninstall", len(failed))
	}
	return nil
}

func logUninstallAgentOp(cfgPath string, names []string, removed, failed int, dryRun bool, start time.Time) {
	status := "ok"
	if failed > 0 && removed > 0 {
		status = "partial"
	} else if failed > 0 {
		status = "error"
	}
	e := oplog.NewEntry("uninstall", status, time.Since(start))
	e.Args = map[string]any{
		"resource_kind": "agent",
		"names":         names,
		"removed":       removed,
		"failed":        failed,
		"dry_run":       dryRun,
	}
	oplog.WriteWithLimit(cfgPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck
}
