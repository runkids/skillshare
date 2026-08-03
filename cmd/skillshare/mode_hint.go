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

// symlinkIncompatTargets lists targets whose runtimes are known to skip
// symlinked skill directories. Order doubles as the hint's example priority,
// which also keeps the output deterministic (ranging a map would not).
var symlinkIncompatTargets = []string{"cursor", "antigravity", "copilot", "opencode"}

func printSymlinkCompatHint(targets map[string]config.TargetConfig, defaultMode string, projectMode bool) {
	// Pick the highest-priority known-incompatible target still on a non-copy mode.
	var exampleTarget string
	for _, name := range symlinkIncompatTargets {
		target, ok := targets[name]
		if !ok {
			continue
		}
		if effectiveSyncMode(target.SkillsConfig().Mode, defaultMode) != "copy" {
			exampleTarget = name
			break
		}
	}
	if exampleTarget == "" {
		return
	}

	fmt.Println()
	ui.Info("Symlink compatibility: some tools cannot read symlinked skills.")
	ui.Info("If a tool can't discover your skills, switch it to copy mode:")
	cmd := modeHintCommand(exampleTarget, projectMode)
	fmt.Printf("  %s%s%s\n", ui.Muted, cmd, ui.Reset)
}
