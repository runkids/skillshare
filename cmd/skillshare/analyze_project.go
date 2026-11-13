package main

import (
	"fmt"

	"skillshare/internal/config"
	ssync "skillshare/internal/sync"
)

// cmdAnalyzeProject runs the analyze command in project mode.
func cmdAnalyzeProject(root string, opts *analyzeOptions) error {
	if !projectConfigExists(root) {
		return fmt.Errorf("no project config found in %s", root)
	}

	cfg, err := config.LoadProjectReadOnly(root)
	if err != nil { return err }
	if err := cfg.ActiveProfileError(); err != nil { return err }
	runtime, err := loadProjectRuntime(root)
	if err != nil {
		if opts.json {
			return writeJSONError(err)
		}
		return err
	}

	if opts.targetName == "" && !opts.json && shouldLaunchTUI(opts.noTUI, nil) {
		loadFn := func() analyzeLoadResult {
			discovered, err := ssync.DiscoverSourceSkillsForAnalyze(runtime.sourcePath, runtime.skillsWalk())
			if err != nil {
				return analyzeLoadResult{err: err}
			}
			entries, err := buildAnalyzeEntries(discovered, runtime.targets, "", runtime.sourcePath, "")
			if err != nil {
				return analyzeLoadResult{err: err}
			}
			return analyzeLoadResult{targets: entries}
		}
		return runAnalyzeTUI(loadFn, "project", opts.filter)
	}

	return runAnalyzeCore(runtime.sourcePath, runtime.skillsWalk(), runtime.targets, "", runtime.config.ContextBudget, opts)
}
