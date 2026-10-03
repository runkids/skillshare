package main

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"skillshare/internal/backup"
	"skillshare/internal/config"
	"skillshare/internal/resource"
	"skillshare/internal/sync"
	"skillshare/internal/theme"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
)

// agentSyncStats aggregates per-target agent sync results.
type agentSyncStats struct {
	linked, local, updated, pruned int
	failed                         []string // targets whose agent sync failed
}

// syncAgentsGlobal discovers agents and syncs them to all agent-capable targets.
// Returns total stats and any error.
func syncAgentsGlobal(cfg *config.Config, dryRun, force, jsonOutput bool, start time.Time) (agentSyncStats, error) {
	agentsSource := cfg.EffectiveAgentsSource()

	// Check agent source exists
	if _, err := os.Stat(agentsSource); err != nil {
		if os.IsNotExist(err) {
			if !jsonOutput {
				fmt.Println(theme.Dim().Render("No agents folder (" + utils.FoldHomePath(agentsSource) + ")"))
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
		ui.Section("Agents")
		if len(agents) == 0 {
			ui.Note("No agents in " + utils.FoldHomePath(agentsSource) + "; only removing ones synced before")
		}
	}

	// Backup agent targets before sync (non-dry-run only).
	if !dryRun && !jsonOutput {
		backupDir, agentTargets, backupErr := resolveGlobalAgentBackupContextFromCfg(cfg)
		if backupErr != nil {
			ui.Warning("Failed to resolve agent backup targets: %v", backupErr)
			agentTargets = nil
		} else {
			defer func() {
				if _, err := backup.CleanupInDir(backupDir, backup.RetentionConfig(cfg)); err != nil {
					ui.Warning("Failed to clean up old agent backups: %v", err)
				}
			}()
		}
		var names []string
		var dir string
		for _, at := range agentTargets {
			entryName := at.name + "-agents"
			bp, bErr := backup.CreateInDir(backupDir, entryName, at.agentPath)
			if bErr != nil {
				ui.Warning("Failed to backup %s: %v", entryName, bErr)
			} else if bp != "" {
				names = append(names, at.name)
				dir = filepath.Dir(bp)
			}
		}
		printBackupRow(names, dir, ui.RowWidth(slices.Collect(maps.Keys(cfg.Targets))...))
	}

	// Resolve agent-capable targets: user config agents sub-key + built-in defaults
	builtinAgents := config.DefaultAgentTargets()
	var targets []sync.AgentTarget
	for _, name := range slices.Sorted(maps.Keys(cfg.Targets)) {
		tc := cfg.Targets[name]
		targets = append(targets, sync.AgentTarget{
			Name:   name,
			Path:   resolveAgentTargetPath(tc, builtinAgents, name),
			Config: tc.AgentsConfig(),
		})
	}
	results := sync.RunAgentSync(targets, agents, sync.AgentRunOptions{
		Source: agentsSource,
		DryRun: dryRun,
		Force:  force,
		ResolveExtension: func(ext string) (*sync.ExtensionSpec, error) {
			return resolveExtension(ext, globalExtensionsDir())
		},
	})
	return renderAgentRun(results, len(agents), dryRun, jsonOutput, start)
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
	if builtin, ok := config.LookupGlobalAgentTarget(name); ok {
		return config.ExpandPath(builtin.Path)
	}
	return ""
}

// syncAgentsOnlyProject runs `sync -p agents`: like the global agent-only sync,
// a target with invalid settings fails alone, and the run is logged.
func syncAgentsOnlyProject(projectRoot string, dryRun, force, jsonOutput bool, start time.Time) error {
	projCfg, err := config.LoadProject(projectRoot)
	if err != nil {
		return fmt.Errorf("cannot load project config: %w", err)
	}
	warnings, invalid, validErr := config.ValidateProjectConfigForSync(projCfg, projectRoot)
	if validErr != nil {
		return validErr
	}
	invalidNames := slices.Sorted(maps.Keys(invalid))
	if !jsonOutput {
		for _, w := range warnings {
			ui.Warning("%s", w)
		}
		for _, name := range invalidNames {
			ui.Error("%s: %s", name, invalidConfigMessage(invalid[name]))
		}
	}
	agentFailed, agentErr := syncAgentsProject(projectRoot, invalid, dryRun, force, jsonOutput, start)
	if agentErr == nil && len(invalidNames) > 0 {
		agentErr = fmt.Errorf("some targets failed to sync")
	}
	logSyncOp(config.ProjectConfigPath(projectRoot), syncLogStats{
		Targets:       len(projCfg.Targets),
		FailedTargets: mergeFailedTargets(agentFailed, invalidNames),
		DryRun:        dryRun,
		Force:         force,
		ProjectScope:  true,
	}, start, agentErr)
	return agentErr
}

// syncAgentsProject syncs agents for project mode using .skillshare/agents/ as source
// and project-level target agent paths.
// Targets in skip are left out. Returns the targets that failed and any error.
func syncAgentsProject(projectRoot string, skip map[string]error, dryRun, force, jsonOutput bool, start time.Time) ([]string, error) {
	// Load project config first to resolve agents source path.
	projCfg, loadErr := config.LoadProject(projectRoot)
	if loadErr != nil {
		return nil, fmt.Errorf("cannot load project config: %w", loadErr)
	}

	agentsSource := projCfg.EffectiveAgentsSource(projectRoot)

	if _, err := os.Stat(agentsSource); err != nil {
		if os.IsNotExist(err) {
			if !jsonOutput {
				fmt.Println(theme.Dim().Render("No project agents folder (" + utils.FoldHomePath(agentsSource) + ")"))
			}
			return nil, nil
		}
		return nil, fmt.Errorf("cannot access project agents: %w", err)
	}

	allAgents, err := resource.AgentKind{}.Discover(agentsSource)
	if err != nil {
		return nil, fmt.Errorf("cannot discover project agents: %w", err)
	}
	agents := resource.ActiveAgents(allAgents)

	if !jsonOutput {
		ui.Section("Agents")
		if len(agents) == 0 {
			ui.Note("No project agents; only removing ones synced before")
		}
	}

	builtinAgents := config.ProjectAgentTargets()

	// Backup agent targets before sync (non-dry-run only).
	if !dryRun && !jsonOutput {
		backupDir := backup.ProjectBackupDir(projectRoot)
		defer func() {
			if _, err := backup.CleanupInDir(backupDir, backup.DefaultCleanupConfig()); err != nil {
				ui.Warning("Failed to clean up old project agent backups: %v", err)
			}
		}()
		var names, all []string
		var dir string
		for _, entry := range projCfg.Targets {
			all = append(all, entry.Name)
			agentPath := resolveProjectAgentTargetPath(entry, builtinAgents, projectRoot)
			if agentPath == "" || skip[entry.Name] != nil {
				continue
			}
			entryName := entry.Name + "-agents"
			bp, bErr := backup.CreateInDir(backupDir, entryName, agentPath)
			if bErr != nil {
				ui.Warning("Failed to backup %s: %v", entryName, bErr)
			} else if bp != "" {
				names = append(names, entry.Name)
				dir = filepath.Dir(bp)
			}
		}
		printBackupRow(names, dir, ui.RowWidth(all...))
	}

	var targets []sync.AgentTarget
	for _, entry := range projCfg.Targets {
		if skip[entry.Name] != nil {
			continue
		}
		targets = append(targets, sync.AgentTarget{
			Name:   entry.Name,
			Path:   resolveProjectAgentTargetPath(entry, builtinAgents, projectRoot),
			Config: entry.AgentsConfig(),
		})
	}
	results := sync.RunAgentSync(targets, agents, sync.AgentRunOptions{
		Source:      agentsSource,
		ProjectRoot: projectRoot,
		DryRun:      dryRun,
		Force:       force,
		ResolveExtension: func(ext string) (*sync.ExtensionSpec, error) {
			return resolveExtension(ext, projectExtensionsDir(projectRoot))
		},
	})
	stats, err := renderAgentRun(results, len(agents), dryRun, jsonOutput, start)
	return stats.failed, err
}

// renderAgentRun prints one row per target and the closing line. Shared by
// the global and project sync paths. Returns the totals and an error when
// any target failed.
func renderAgentRun(results []sync.AgentTargetResult, agentCount int, dryRun, jsonOutput bool, start time.Time) (agentSyncStats, error) {
	var totals agentSyncStats
	var syncErr error
	var skippedTargets []string
	var targetCount int

	var names []string
	for _, r := range results {
		names = append(names, r.Name)
	}
	width := ui.RowWidth(names...)

	for _, r := range results {
		if r.Path == "" {
			skippedTargets = append(skippedTargets, r.Name)
			continue
		}
		targetCount++

		if r.Err != nil {
			if !jsonOutput {
				ui.Row(ui.MarkFail, r.Name, r.Err.Error(), width)
			}
			totals.failed = append(totals.failed, r.Name)
			syncErr = fmt.Errorf("some agent targets failed to sync")
			continue
		}
		if r.SyncErr != nil {
			if !jsonOutput {
				ui.Row(ui.MarkFail, r.Name, "agent sync failed: "+r.SyncErr.Error(), width)
			}
			totals.failed = append(totals.failed, r.Name)
			syncErr = fmt.Errorf("some agent targets failed to sync")
		}
		if !r.Synced {
			continue
		}

		stats := agentSyncStats{
			linked:  len(r.Linked),
			local:   len(r.Skipped),
			updated: len(r.Updated),
			pruned:  len(r.Pruned),
		}
		if !jsonOutput {
			linked := "linked"
			if r.Mode == "copy" {
				linked = "copied"
			}
			ui.Row(ui.MarkOK, r.Name, syncCounts("up to date",
				countPart{stats.linked, linked}, countPart{stats.local, "local"},
				countPart{stats.updated, "updated"}, countPart{stats.pruned, "pruned"}), width)
			for _, w := range r.Warnings {
				fmt.Printf("  %s %s\n", theme.Warning().Render(ui.MarkWarn), w)
			}
		}
		totals.linked += stats.linked
		totals.local += stats.local
		totals.updated += stats.updated
		totals.pruned += stats.pruned
	}

	if !jsonOutput {
		if len(skippedTargets) > 0 {
			sort.Strings(skippedTargets)
			ui.Note(fmt.Sprintf("No agents folder: %s", strings.Join(skippedTargets, ", ")))
		}
		fmt.Println()
		switch {
		case len(totals.failed) > 0:
			ui.Done(ui.MarkFail, fmt.Sprintf("%d of %s failed", len(totals.failed), plural(targetCount, "target")), time.Since(start))
		case dryRun:
			ui.Done(ui.MarkOK, fmt.Sprintf("Would sync %s to %s", plural(agentCount, "agent"), plural(targetCount, "target")), time.Since(start))
			ui.DryRun()
		default:
			ui.Done(ui.MarkOK, fmt.Sprintf("Synced %s to %s", plural(agentCount, "agent"), plural(targetCount, "target")), time.Since(start))
		}
	}

	return totals, syncErr
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
