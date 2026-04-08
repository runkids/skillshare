package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/install"
	"skillshare/internal/oplog"
	"skillshare/internal/resource"
	"skillshare/internal/trash"
	"skillshare/internal/ui"
)

// cmdUninstallAgents removes agents from the source directory by moving them to agent trash.
func cmdUninstallAgents(agentsDir string, opts *uninstallOptions, cfgPath string, trashBase string, start time.Time) error {
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

		// Resolve --group targets
		if len(opts.groups) > 0 {
			groupFiltered, err := filterDiscoveredAgentsByGroups(discovered, opts.groups, agentsDir)
			if err != nil {
				return err
			}
			if len(groupFiltered) == 0 {
				return fmt.Errorf("no agents found in group(s): %s", strings.Join(opts.groups, ", "))
			}
			// Deduplicate against already-resolved name targets
			seen := make(map[string]bool, len(targets))
			for _, t := range targets {
				seen[t.RelPath] = true
			}
			for _, d := range groupFiltered {
				if !seen[d.RelPath] {
					targets = append(targets, d)
				}
			}
		}
	}

	if len(targets) == 0 {
		return fmt.Errorf("specify agent name(s), --group, or --all")
	}

	// Confirmation (unless --force or --json)
	if !opts.force && !opts.jsonOutput {
		ui.Warning("Uninstalling %d agent(s)", len(targets))
		const maxDisplay = 20
		display := targets
		if len(display) > maxDisplay {
			display = display[:maxDisplay]
		}
		for _, t := range display {
			fmt.Printf("  - %s\n", strings.TrimSuffix(t.RelPath, ".md"))
		}
		if len(targets) > maxDisplay {
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

	store, _ := install.LoadMetadata(agentsDir)
	var removed []string
	var failed []string

	for _, t := range targets {
		agentFile := filepath.Join(agentsDir, t.RelPath)

		displayName := strings.TrimSuffix(t.RelPath, ".md")
		if opts.dryRun {
			ui.Info("[dry-run] Would remove agent: %s", displayName)
			removed = append(removed, displayName)
			continue
		}

		// Trash the agent file (+ legacy sidecar if it still exists)
		metaName := strings.TrimSuffix(filepath.Base(t.RelPath), ".md")
		legacySidecar := filepath.Join(filepath.Dir(agentFile), metaName+".skillshare-meta.json")
		_, err := trash.MoveAgentToTrash(agentFile, legacySidecar, displayName, trashBase)
		if err != nil {
			ui.Error("Failed to remove %s: %v", displayName, err)
			failed = append(failed, displayName)
			continue
		}

		// Remove from centralized metadata store
		if store != nil {
			store.Remove(displayName)
		}

		ui.Success("Removed agent: %s", displayName)
		removed = append(removed, displayName)
	}

	// Save store after all removals
	if store != nil && len(removed) > 0 {
		store.Save(agentsDir) //nolint:errcheck
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
