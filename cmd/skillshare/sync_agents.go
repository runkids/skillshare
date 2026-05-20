package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"skillshare/internal/backup"
	"skillshare/internal/config"
	"skillshare/internal/resource"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
)

// agentSyncStats aggregates per-target agent sync results.
type agentSyncStats struct {
	linked, local, updated, pruned int
}

// syncAgentsGlobal discovers agents and syncs them to all agent-capable targets.
// Returns total stats and any error.
func syncAgentsGlobal(cfg *config.Config, dryRun, force, jsonOutput bool, start time.Time) (agentSyncStats, error) {
	agentsSource := cfg.EffectiveAgentsSource()

	// Check agent source exists
	if _, err := os.Stat(agentsSource); err != nil {
		if os.IsNotExist(err) {
			if !jsonOutput {
				ui.Info("No agents source directory (%s)", agentsSource)
			}
			return agentSyncStats{}, nil
		}
		return agentSyncStats{}, fmt.Errorf("cannot access agents source: %w", err)
	}

	// Discover agents (excludes disabled from sync)
	allAgents, err := resource.AgentKind{}.Discover(agentsSource)
	if err != nil {
		return agentSyncStats{}, fmt.Errorf("cannot discover agents: %w", err)
	}
	agents := resource.ActiveAgents(allAgents)

	if !jsonOutput {
		ui.Header("Syncing agents")
		if len(agents) == 0 {
			ui.Info("No agents found in %s; pruning synced target entries only", agentsSource)
		}
		if dryRun {
			ui.Warning("Dry run mode - no changes will be made")
		}
	}

	// Backup agent targets before sync (non-dry-run only).
	if !dryRun && !jsonOutput {
		backupDir, agentTargets, _ := resolveGlobalAgentBackupContextFromCfg(cfg)
		backedUp := false
		for _, at := range agentTargets {
			entryName := at.name + "-agents"
			bp, bErr := backup.CreateInDir(backupDir, entryName, at.agentPath)
			if bErr != nil {
				ui.Warning("Failed to backup %s: %v", entryName, bErr)
			} else if bp != "" {
				if !backedUp {
					ui.Header("Backing up")
					backedUp = true
				}
				ui.Success("%s -> %s", entryName, bp)
			}
		}
	}

	// Resolve agent-capable targets: user config agents sub-key + built-in defaults
	builtinAgents := config.DefaultAgentTargets()
	var totals agentSyncStats
	var syncErr error
	var skippedTargets []string
	var targetCount int

	for name := range cfg.Targets {
		agentPath := resolveAgentTargetPath(cfg.Targets[name], builtinAgents, name)
		if agentPath == "" {
			skippedTargets = append(skippedTargets, name)
			continue
		}
		targetCount++

		tc := cfg.Targets[name]
		ac := tc.AgentsConfig()
		filtered, filterErr := sync.FilterAgents(agents, ac.Include, ac.Exclude)
		if filterErr != nil {
			if !jsonOutput {
				ui.Error("%s: invalid agent filter: %v", name, filterErr)
			}
			syncErr = fmt.Errorf("some agent targets failed to sync")
			continue
		}
		stats, targetErr := syncAgentTarget(name, agentPath, ac.Mode, filtered, agentsSource, dryRun, force, jsonOutput)
		if targetErr != nil {
			syncErr = fmt.Errorf("some agent targets failed to sync")
		}
		totals.linked += stats.linked
		totals.local += stats.local
		totals.updated += stats.updated
		totals.pruned += stats.pruned
	}

	if !jsonOutput {
		ui.AgentSyncSummary(ui.AgentSyncStats{
			Targets:  targetCount,
			Linked:   totals.linked,
			Local:    totals.local,
			Updated:  totals.updated,
			Pruned:   totals.pruned,
			Duration: time.Since(start),
		})
		if len(skippedTargets) > 0 {
			sort.Strings(skippedTargets)
			ui.Warning("%d target(s) skipped for agents (no agents path): %s",
				len(skippedTargets), strings.Join(skippedTargets, ", "))
		}
	}

	return totals, syncErr
}

// resolveAgentTargetPath returns the effective agent path for a target,
// checking user config first, then built-in defaults. Returns "" if none.
func resolveAgentTargetPath(tc config.TargetConfig, builtinAgents map[string]config.TargetConfig, name string) string {
	if ac := tc.AgentsConfig(); ac.Path != "" {
		return config.ExpandPath(ac.Path)
	}
	if builtin, ok := builtinAgents[name]; ok {
		return config.ExpandPath(builtin.Path)
	}
	return ""
}

// syncAgentsProject syncs agents for project mode using .skillshare/agents/ as source
// and project-level target agent paths.
func syncAgentsProject(projectRoot string, dryRun, force, jsonOutput bool, start time.Time) error {
	// Load project config first to resolve agents source path.
	projCfg, loadErr := config.LoadProject(projectRoot)
	if loadErr != nil {
		return fmt.Errorf("cannot load project config: %w", loadErr)
	}

	agentsSource := projCfg.EffectiveAgentsSource(projectRoot)

	if _, err := os.Stat(agentsSource); err != nil {
		if os.IsNotExist(err) {
			if !jsonOutput {
				ui.Info("No project agents directory (%s)", agentsSource)
			}
			return nil
		}
		return fmt.Errorf("cannot access project agents: %w", err)
	}

	allAgents, err := resource.AgentKind{}.Discover(agentsSource)
	if err != nil {
		return fmt.Errorf("cannot discover project agents: %w", err)
	}
	agents := resource.ActiveAgents(allAgents)

	if !jsonOutput {
		ui.Header("Syncing agents (project)")
		if len(agents) == 0 {
			ui.Info("No project agents found; pruning synced target entries only")
		}
		if dryRun {
			ui.Warning("Dry run mode - no changes will be made")
		}
	}

	builtinAgents := config.ProjectAgentTargets()

	// Backup agent targets before sync (non-dry-run only).
	if !dryRun && !jsonOutput {
		backupDir := filepath.Join(projectRoot, ".skillshare", "backups")
		backedUp := false
		for _, entry := range projCfg.Targets {
			agentPath := resolveProjectAgentTargetPath(entry, builtinAgents, projectRoot)
			if agentPath == "" {
				continue
			}
			entryName := entry.Name + "-agents"
			bp, bErr := backup.CreateInDir(backupDir, entryName, agentPath)
			if bErr != nil {
				ui.Warning("Failed to backup %s: %v", entryName, bErr)
			} else if bp != "" {
				if !backedUp {
					ui.Header("Backing up")
					backedUp = true
				}
				ui.Success("%s -> %s", entryName, bp)
			}
		}
	}

	var totals agentSyncStats
	var syncErr error
	var skippedTargets []string
	var targetCount int

	for _, entry := range projCfg.Targets {
		agentPath := resolveProjectAgentTargetPath(entry, builtinAgents, projectRoot)
		if agentPath == "" {
			skippedTargets = append(skippedTargets, entry.Name)
			continue
		}
		targetCount++

		ac := entry.AgentsConfig()
		filtered, filterErr := sync.FilterAgents(agents, ac.Include, ac.Exclude)
		if filterErr != nil {
			if !jsonOutput {
				ui.Error("%s: invalid agent filter: %v", entry.Name, filterErr)
			}
			syncErr = fmt.Errorf("some agent targets failed to sync")
			continue
		}
		stats, targetErr := syncAgentTarget(entry.Name, agentPath, ac.Mode, filtered, agentsSource, dryRun, force, jsonOutput)
		if targetErr != nil {
			syncErr = fmt.Errorf("some agent targets failed to sync")
		}
		totals.linked += stats.linked
		totals.local += stats.local
		totals.updated += stats.updated
		totals.pruned += stats.pruned
	}

	if !jsonOutput {
		ui.AgentSyncSummary(ui.AgentSyncStats{
			Targets:  targetCount,
			Linked:   totals.linked,
			Local:    totals.local,
			Updated:  totals.updated,
			Pruned:   totals.pruned,
			Duration: time.Since(start),
		})
		if len(skippedTargets) > 0 {
			sort.Strings(skippedTargets)
			ui.Warning("%d target(s) skipped for agents (no agents path): %s",
				len(skippedTargets), strings.Join(skippedTargets, ", "))
		}
	}

	return syncErr
}

// syncAgentTarget syncs agents to a single target directory.
// Shared by both global and project sync paths.
func syncAgentTarget(name, agentPath, modeOverride string, agents []resource.DiscoveredResource, agentsSource string, dryRun, force, jsonOutput bool) (agentSyncStats, error) {
	mode := modeOverride
	if mode == "" {
		mode = "merge"
	}

	result, err := sync.SyncAgents(agents, agentsSource, agentPath, mode, dryRun, force)
	if err != nil {
		if !jsonOutput {
			ui.Error("%s: agent sync failed: %v", name, err)
		}
		return agentSyncStats{}, err
	}

	var pruned []string
	switch mode {
	case "copy":
		pruned, _ = sync.PruneOrphanAgentCopies(agentPath, agents, dryRun)
	case "merge":
		pruned, _ = sync.PruneOrphanAgentLinks(agentPath, agents, dryRun)
	}

	stats := agentSyncStats{
		linked:  len(result.Linked),
		local:   len(result.Skipped),
		updated: len(result.Updated),
		pruned:  len(pruned),
	}

	if !jsonOutput {
		reportAgentSyncResult(name, mode, stats, dryRun)
	}

	return stats, nil
}

// reportAgentSyncResult prints per-target agent sync status.
func reportAgentSyncResult(name, mode string, stats agentSyncStats, dryRun bool) {
	if stats.linked > 0 || stats.updated > 0 || stats.pruned > 0 {
		ui.Success("%s: agents %s (%d linked, %d local, %d updated, %d pruned)",
			name, mode, stats.linked, stats.local, stats.updated, stats.pruned)
	} else if stats.local > 0 {
		ui.Success("%s: agents %s (%d local preserved)", name, mode, stats.local)
	} else {
		ui.Success("%s: agents %s (up to date)", name, mode)
	}
}

// collectAgentTargetPathsGlobal returns the set of resolved agent target paths
// for all targets in the global config. Returns nil when agents source does not
// exist or contains no agent files (meaning no real agent sync would happen).
func collectAgentTargetPathsGlobal(cfg *config.Config) map[string]bool {
	agentsSource := cfg.EffectiveAgentsSource()
	if _, err := os.Stat(agentsSource); err != nil {
		return nil
	}
	agents, err := resource.AgentKind{}.Discover(agentsSource)
	if err != nil || len(agents) == 0 {
		return nil
	}

	builtinAgents := config.DefaultAgentTargets()
	paths := make(map[string]bool)
	for name := range cfg.Targets {
		agentPath := resolveAgentTargetPath(cfg.Targets[name], builtinAgents, name)
		if agentPath != "" {
			paths[filepath.Clean(agentPath)] = true
		}
	}
	if len(paths) == 0 {
		return nil
	}
	return paths
}

// collectAgentTargetPathsProject returns the set of resolved agent target paths
// for all targets in the project config. Returns nil when no agents exist.
func collectAgentTargetPathsProject(projectRoot string) map[string]bool {
	projCfg, err := config.LoadProject(projectRoot)
	if err != nil {
		return nil
	}

	agentsSource := projCfg.EffectiveAgentsSource(projectRoot)
	if _, err := os.Stat(agentsSource); err != nil {
		return nil
	}
	agents, err := resource.AgentKind{}.Discover(agentsSource)
	if err != nil || len(agents) == 0 {
		return nil
	}

	builtinAgents := config.ProjectAgentTargets()
	paths := make(map[string]bool)
	for _, entry := range projCfg.Targets {
		agentPath := resolveProjectAgentTargetPath(entry, builtinAgents, projectRoot)
		if agentPath != "" {
			paths[filepath.Clean(agentPath)] = true
		}
	}
	if len(paths) == 0 {
		return nil
	}
	return paths
}
