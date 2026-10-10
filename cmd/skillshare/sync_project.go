package main

import (
	"fmt"
	"os"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/skillignore"
	"skillshare/internal/sync"
	"skillshare/internal/trash"
	"skillshare/internal/ui"
)

// The returned map holds the targets skipped for invalid settings.
func cmdSyncProject(root string, dryRun, force, jsonOutput, quiet bool) (syncLogStats, []syncTargetResult, *skillignore.IgnoreStats, *contextCostJSON, map[string]error, []string, error) {
	start := time.Now()
	stats := syncLogStats{
		DryRun:       dryRun,
		Force:        force,
		ProjectScope: true,
	}

	if err := ensureProjectConfig(root); err != nil {
		return stats, nil, nil, nil, nil, nil, err
	}

	cfg, err := config.LoadProject(root)
	if err != nil {
		return stats, nil, nil, nil, nil, nil, err
	}
	stats.Targets = len(cfg.Targets)

	// Validate project config before resolving targets. A target with invalid
	// settings, even one without a path, fails alone; the other targets still sync.
	warnings, invalid, validErr := config.ValidateProjectConfigForSync(cfg, root)
	if validErr != nil {
		return stats, nil, nil, nil, nil, nil, validErr
	}
	runtime, err := newProjectRuntime(root, cfg, invalid)
	if err != nil {
		return stats, nil, nil, nil, nil, nil, err
	}
	if !jsonOutput {
		for _, w := range warnings {
			ui.Warning("%s", w)
		}
	}

	// ValidateProjectConfig warns on missing source (may not exist yet after init).
	// Gate here as a hard error — sync cannot proceed without source skills.
	if _, err := os.Stat(runtime.sourcePath); os.IsNotExist(err) {
		return stats, nil, nil, nil, nil, nil, fmt.Errorf("source directory does not exist: %s", runtime.sourcePath)
	}

	// A skill moved by hand keeps its install record; sync never prunes one.
	var metaWarnings []string
	if !dryRun {
		if rErr := config.AdoptMovedProjectSkills(root, cfg, runtime.skillsStore, runtime.sourcePath); rErr != nil {
			metaWarnings = append(metaWarnings, fmt.Sprintf("install metadata not updated: %v", rErr))
		}
	}

	// Phase 1: Discovery
	var spinner *ui.Spinner
	if !jsonOutput {
		spinner = ui.StartSpinner("Discovering skills")
	}
	walk := runtime.skillsWalk()
	discoveredSkills, ignoreStats, discoverErr := sync.DiscoverSourceSkillsWithStatsAndContext(runtime.sourcePath, walk)
	if discoverErr != nil {
		if spinner != nil {
			spinner.Fail("Discovery failed")
		}
		return stats, nil, nil, nil, nil, nil, discoverErr
	}
	if spinner != nil {
		spinner.Stop()
		reportCollisions(discoveredSkills, runtime.targets)
	}
	sourceIncomplete := walk.Follow.Incomplete()
	linkWarnings := append(metaWarnings, sync.SourceLinkWarnings(walk, sourceIncomplete)...)
	if !jsonOutput {
		for _, w := range linkWarnings {
			ui.Warning("%s", w)
		}
	}

	var entries []syncTargetEntry
	var notFound []string
	for _, entry := range runtime.config.Targets {
		name := entry.Name
		target, ok := runtime.targets[name]
		if !ok && invalid[name] == nil {
			if !jsonOutput {
				ui.Row(ui.MarkFail, name, "target not found", ui.RowWidth(name))
			}
			notFound = append(notFound, name)
			continue
		}
		mode := target.SkillsConfig().Mode
		if mode == "" {
			mode = "merge"
		}
		entries = append(entries, syncTargetEntry{name: name, target: target, mode: mode, configErr: invalid[name], sourceIncomplete: sourceIncomplete})
	}

	var results []syncTargetResult
	ignorePatterns := sync.EffectiveFileIgnorePatterns(runtime.config.Ignore)
	if jsonOutput {
		results, _ = runParallelSyncQuiet(entries, runtime.sourcePath, discoveredSkills, ignorePatterns, dryRun, force, root)
	} else {
		results, _ = runParallelSync(entries, runtime.sourcePath, discoveredSkills, ignorePatterns, dryRun, force, root)
	}

	movedTargets := sync.MovedProjectTargets(runtime.config, runtime.targets)
	for _, msg := range sync.CleanMovedProjectDirs(root, runtime.sourcePath, movedTargets, dryRun) {
		if !jsonOutput {
			ui.Note(msg)
		}
	}

	var totals syncModeStats
	for _, r := range results {
		totals.linked += r.stats.linked
		totals.local += r.stats.local
		totals.updated += r.stats.updated
		totals.pruned += r.stats.pruned
	}
	stats.FailedTargets = mergeFailedTargets(failedSkillTargets(results), notFound)

	if !jsonOutput {
		// Phase 3: Summary
		printSyncDone(len(discoveredSkills), len(runtime.config.Targets), len(stats.FailedTargets), dryRun, time.Since(start))

		// Show ignored skills from .skillignore
		printIgnoredSkills(ignoreStats)

		// One-liner path-overlap hint — points users at `doctor` for details.
		printSyncOverlapHint(runtime.targets, true, jsonOutput, "", discoveredSkills)
	}

	// Compute context cost once — used by both text summary and JSON output
	analyzeEntries, analyzeErr := buildAnalyzeEntries(discoveredSkills, runtime.targets, "", runtime.sourcePath, "")

	var ctxCost *contextCostJSON
	if analyzeErr == nil && len(analyzeEntries) > 0 {
		ctxCost = buildContextCostJSON(analyzeEntries, runtime.config.ContextBudget)

		if !jsonOutput && !quiet {
			printTokenSummary(analyzeEntries)
			if violations := checkBudget(analyzeEntries, runtime.config.ContextBudget); len(violations) > 0 {
				printBudgetWarning(violations, true)
			}
		}
	}

	// Reconcile registry and cleanup regardless of target failures.
	// Registry cleanup only depends on source disk state, not target sync results.
	if !dryRun {
		if n, _ := trash.Cleanup(trash.ProjectTrashDir(root), 0); n > 0 {
			if !jsonOutput {
				ui.Note(fmt.Sprintf("Removed %s older than 7 days from trash", plural(n, "item")))
			}
		}
	}

	if len(stats.FailedTargets) > 0 {
		return stats, results, ignoreStats, ctxCost, invalid, linkWarnings, fmt.Errorf("some targets failed to sync")
	}

	return stats, results, ignoreStats, ctxCost, invalid, linkWarnings, nil
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
