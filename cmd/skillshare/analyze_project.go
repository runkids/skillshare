package main

import "fmt"

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

	return runAnalyzeCore(runtime.sourcePath, runtime.targets, "", opts)
}
