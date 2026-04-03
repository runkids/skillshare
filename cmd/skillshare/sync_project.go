package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/skillignore"
	"skillshare/internal/sync"
	"skillshare/internal/trash"
	"skillshare/internal/ui"
)

func cmdSyncProject(root string, dryRun, force, jsonOutput bool) (syncLogStats, []syncTargetResult, *skillignore.IgnoreStats, error) {
	start := time.Now()
	stats := syncLogStats{
		DryRun:       dryRun,
		Force:        force,
		ProjectScope: true,
	}

	if !projectConfigExists(root) {
		if err := performProjectInit(root, projectInitOptions{}); err != nil {
			return stats, nil, nil, err
		}
	}

	runtime, err := loadProjectRuntime(root)
	if err != nil {
		return stats, nil, nil, err
	}
	stats.Targets = len(runtime.config.Targets)

	// Validate project config before sync
	warnings, validErr := config.ValidateProjectConfig(runtime.config, root)
	if validErr != nil {
		return stats, nil, nil, validErr
	}
	if !jsonOutput {
		for _, w := range warnings {
			ui.Warning("%s", w)
		}
	}

	// ValidateProjectConfig warns on missing source (may not exist yet after init).
	// Gate here as a hard error — sync cannot proceed without source skills.
	if _, err := os.Stat(runtime.sourcePath); os.IsNotExist(err) {
		return stats, nil, nil, fmt.Errorf("source directory does not exist: %s", runtime.sourcePath)
	}

	// Phase 1: Discovery
	var spinner *ui.Spinner
	if !jsonOutput {
		spinner = ui.StartSpinner("Discovering skills")
	}
	discoveredSkills, ignoreStats, discoverErr := sync.DiscoverSourceSkillsWithStats(runtime.sourcePath)
	if discoverErr != nil {
		if spinner != nil {
			spinner.Fail("Discovery failed")
		}
		return stats, nil, nil, discoverErr
	}
	if spinner != nil {
		spinner.Success(fmt.Sprintf("Discovered %d skills", len(discoveredSkills)))
		reportCollisions(discoveredSkills, runtime.targets)
	}

	// Phase 2: Per-target sync
	if !jsonOutput {
		ui.Header("Syncing skills (project)")
		if dryRun {
			ui.Warning("Dry run mode - no changes will be made")
		}
	}

	var entries []syncTargetEntry
	notFoundCount := 0
	for _, entry := range runtime.config.Targets {
		name := entry.Name
		target, ok := runtime.targets[name]
		if !ok {
			if !jsonOutput {
				ui.Error("%s: target not found", name)
			}
			notFoundCount++
			continue
		}
		mode := target.SkillsConfig().Mode
		if mode == "" {
			mode = "merge"
		}
		entries = append(entries, syncTargetEntry{name: name, target: target, mode: mode})
	}

	var results []syncTargetResult
	var failedTargets int
	if jsonOutput {
		results, failedTargets = runParallelSyncQuiet(entries, runtime.sourcePath, discoveredSkills, dryRun, force)
	} else {
		results, failedTargets = runParallelSync(entries, runtime.sourcePath, discoveredSkills, dryRun, force)
	}
	failedTargets += notFoundCount

	var totals syncModeStats
	for _, r := range results {
		totals.linked += r.stats.linked
		totals.local += r.stats.local
		totals.updated += r.stats.updated
		totals.pruned += r.stats.pruned
	}
	stats.Failed = failedTargets

	if !jsonOutput {
		// Phase 3: Summary
		ui.SyncSummary(ui.SyncStats{
			Targets:  len(runtime.config.Targets),
			Linked:   totals.linked,
			Local:    totals.local,
			Updated:  totals.updated,
			Pruned:   totals.pruned,
			Duration: time.Since(start),
		})

		// Show ignored skills from .skillignore
		printIgnoredSkills(ignoreStats)
	}

	// Reconcile registry and cleanup regardless of target failures.
	// Registry cleanup only depends on source disk state, not target sync results.
	if !dryRun {
		if n, _ := trash.Cleanup(trash.ProjectTrashDir(root), 0); n > 0 {
			if !jsonOutput {
				ui.Info("Cleaned up %d expired trash item(s)", n)
			}
		}

		// Prune stale registry entries using already-discovered skills (avoids second walk)
		live := make(map[string]bool, len(discoveredSkills))
		for _, s := range discoveredSkills {
			live[s.RelPath] = true
		}
		// Ignored skills still exist on disk — don't prune their registry entries
		if ignoreStats != nil {
			for _, p := range ignoreStats.IgnoredSkills {
				live[p] = true
			}
		}
		var pruneChanged bool
		runtime.registry.Skills, pruneChanged = config.PruneStaleSkills(runtime.registry.Skills, live, true)
		if pruneChanged {
			if err := runtime.registry.Save(filepath.Join(root, ".skillshare")); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to save registry after prune: %v\n", err)
			}
		}
	}

	if failedTargets > 0 {
		return stats, results, ignoreStats, fmt.Errorf("some targets failed to sync")
	}

	return stats, results, ignoreStats, nil
}

func projectTargetDisplayPath(entry config.ProjectTargetEntry) string {
	if p := entry.SkillsConfig().Path; p != "" {
		return p
	}
	if known, ok := config.LookupProjectTarget(entry.Name); ok {
		return known.Path
	}
	return ""
}
