package main

import (
	"fmt"

	"skillshare/internal/config"
	"skillshare/internal/ui"
)

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

func modeHintCommand(targetName string, projectMode bool) string {
	if targetName == "" {
		return ""
	}
	if projectMode {
		return fmt.Sprintf("skillshare target %s --mode copy -p && skillshare sync -p", targetName)
	}
	return fmt.Sprintf("skillshare target %s --mode copy && skillshare sync", targetName)
}

func printSymlinkCompatHint(targets map[string]config.TargetConfig, defaultMode string, projectMode bool) {
	// Find any target that uses a non-copy mode (merge or symlink).
	var exampleTarget string
	for name, target := range targets {
		mode := effectiveSyncMode(target.Mode, defaultMode)
		if mode != "copy" {
			exampleTarget = name
			break
		}
	}
	if exampleTarget == "" {
		return
	}

	fmt.Println()
	ui.Info("Symlink compatibility: some tools cannot read symlinked skills.")
	ui.Info("  If a tool can't discover your skills, switch it to copy mode:")
	cmd := modeHintCommand(exampleTarget, projectMode)
	fmt.Printf("  %s%s%s\n", ui.Muted, cmd, ui.Reset)
	fmt.Println()
}
