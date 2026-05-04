package main

import (
	"fmt"
	gosync "sync"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
)

func cmdDiffProject(root, targetName string, kind resourceKindFilter, opts diffRenderOpts, start time.Time) error {
	if kind == kindAgents {
		return diffProjectAgents(root, targetName, opts, start)
	}
	if !projectConfigExists(root) {
		if err := performProjectInit(root, projectInitOptions{}); err != nil {
			return err
		}
	}

	runtime, err := loadProjectRuntime(root)
	if err != nil {
		return err
	}

	var spinner *ui.Spinner
	if !opts.jsonOutput {
		spinner = ui.StartSpinner("Discovering skills")
	}
	discovered, err := sync.DiscoverSourceSkills(runtime.sourcePath)
	if err != nil {
		if spinner != nil {
			spinner.Fail("Discovery failed")
		}
		return fmt.Errorf("failed to discover skills: %w", err)
	}
	if spinner != nil {
		spinner.Success(fmt.Sprintf("Discovered %d skills", len(discovered)))
	}

	targets := make([]config.ProjectTargetEntry, len(runtime.config.Targets))
	copy(targets, runtime.config.Targets)

	if targetName != "" {
		found := false
		for _, entry := range runtime.config.Targets {
			if entry.Name == targetName {
				targets = []config.ProjectTargetEntry{entry}
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("target '%s' not found", targetName)
		}
	}

	// Pre-filter skills per target and compute total for progress bar
	type resolvedTarget struct {
		name     string
		target   config.TargetConfig
		mode     string
		filtered []sync.DiscoveredSkill
	}
	var resolved []resolvedTarget
	totalSkills := 0
	hasCopyMode := false
	for _, entry := range targets {
		target, ok := runtime.targets[entry.Name]
		if !ok {
			return fmt.Errorf("target '%s' not resolved", entry.Name)
		}
		sc := target.SkillsConfig()
		filtered, err := sync.FilterSkills(discovered, sc.Include, sc.Exclude)
		if err != nil {
			return fmt.Errorf("target %s has invalid include/exclude config: %w", entry.Name, err)
		}
		filtered = sync.FilterSkillsByTarget(filtered, entry.Name)
		mode := sc.Mode
		if mode == "" {
			mode = "merge"
		}
		resolved = append(resolved, resolvedTarget{entry.Name, target, mode, filtered})
		totalSkills += len(filtered)
		if mode == "copy" {
			hasCopyMode = true
		}
	}

	names := make([]string, len(resolved))
	for i, rt := range resolved {
		names[i] = rt.name
	}
	var progress *diffProgress
	if !opts.jsonOutput {
		progress = newDiffProgress(names, totalSkills, hasCopyMode)
	}

	results := make([]targetDiffResult, len(resolved))
	sem := make(chan struct{}, 8)
	var wg gosync.WaitGroup
	for i, rt := range resolved {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, rt resolvedTarget) {
			defer wg.Done()
			defer func() { <-sem }()
			progress.startTarget(rt.name)
			r := collectTargetDiff(rt.name, rt.target, runtime.sourcePath, rt.mode, rt.filtered, progress)
			progress.doneTarget(rt.name, r)
			results[idx] = r
		}(i, rt)
	}
	wg.Wait()

	progress.stop()

	// Extras diff (always included when extras are configured)
	var extrasResults []extraDiffResult
	if len(runtime.config.Extras) > 0 {
		extrasResults = collectExtrasDiff(runtime.config.Extras, func(extra config.ExtraConfig) string {
			return config.ExtrasSourceDirProject(root, extra.Name)
		})
	}

	// Merge agent diffs into skill results so they appear together
	results = mergeAgentDiffsProject(root, results, targetName)

	if opts.jsonOutput {
		return diffOutputJSONWithExtras(results, extrasResults, collectPluginDiff(config.PluginsSourceDirProject(root), root), collectHookDiff(config.HooksSourceDirProject(root), root), start)
	}
	if shouldLaunchTUI(opts.noTUI, nil) && len(results) > 0 {
		return runDiffTUI(results, extrasResults)
	}
	renderGroupedDiffs(results, opts)
	if len(extrasResults) > 0 {
		renderExtrasDiffPlain(extrasResults)
	}
	return nil
}
