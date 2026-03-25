package main

import (
	"fmt"

	ssync "skillshare/internal/sync"
)

// cmdAnalyzeProject runs the analyze command in project mode.
func cmdAnalyzeProject(root string, opts *analyzeOptions) error {
	if !projectConfigExists(root) {
		return fmt.Errorf("no project config found in %s", root)
	}

	runtime, err := loadProjectRuntime(root)
	if err != nil {
		if opts.json {
			return writeJSONError(err)
		}
		return err
	}

	if opts.targetName == "" && !opts.json && shouldLaunchTUI(opts.noTUI, nil) {
		loadFn := func() analyzeLoadResult {
			discovered, err := ssync.DiscoverSourceSkillsForAnalyze(runtime.sourcePath)
			if err != nil {
				return analyzeLoadResult{err: err}
			}
			entries, err := buildAnalyzeEntries(discovered, runtime.targets, "", "")
			if err != nil {
				return analyzeLoadResult{err: err}
			}
			return analyzeLoadResult{targets: entries}
		}
		return runAnalyzeTUI(loadFn, "project")
	}

	return runAnalyzeCore(runtime.sourcePath, runtime.targets, "", opts)
}
