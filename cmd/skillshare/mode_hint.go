package main

import (
	"fmt"

	"skillshare/internal/config"
	"skillshare/internal/ui"
)

var preferredModeHintTargets = []string{"cursor", "antigravity", "copilot", "opencode"}

func effectiveSyncMode(targetMode, defaultMode string) string {
	mode := normalizeSyncMode(targetMode)
	if mode == "" {
		mode = normalizeSyncMode(defaultMode)
	}
	if mode == "" {
		mode = "merge"
	}
	return mode
}

func modeHintExampleTarget(targets map[string]config.TargetConfig, defaultMode string) string {
	if len(targets) == 0 {
		return ""
	}

	// Keep hints quiet for simple setups (e.g. claude/codex only) and only
	// suggest copy mode for targets that more commonly need compatibility tuning.
	for _, preferred := range preferredModeHintTargets {
		target, ok := targets[preferred]
		if !ok {
			continue
		}
		mode := effectiveSyncMode(target.Mode, defaultMode)
		if mode == "merge" || mode == "symlink" {
			return preferred
		}
	}
	return ""
}

func modeHintCommand(targetName string, projectMode bool) string {
	if targetName == "" {
		return ""
	}
	if projectMode {
		return fmt.Sprintf("skillshare target %s --mode copy -p && skillshare sync -p", targetName)
	}
	return fmt.Sprintf("skillshare target %s --mode copy && skillshare sync", targetName)
}

func printPerTargetModeHint(targets map[string]config.TargetConfig, defaultMode string, projectMode bool) {
	example := modeHintExampleTarget(targets, defaultMode)
	if example == "" {
		return
	}

	// Keep this hint subtle and visually separated from sync/doctor results.
	fmt.Println()
	fmt.Printf("%sHint:%s tune sync mode per target when needed\n", ui.Muted, ui.Reset)
	fmt.Printf("%s  %s%s\n", ui.Muted, modeHintCommand(example, projectMode), ui.Reset)
	fmt.Println()
}
