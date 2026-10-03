package main

import (
	"fmt"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/theme"
	"skillshare/internal/ui"
)

// printSyncOverlapHint warns when enabled targets have overlapping skill paths
// (same primary path or cross-runtime discovery via also_scans). Targets that
// share a folder with different settings undo each other's sync, so they get
// a specific line and the command to keep one; any other overlap keeps the
// brief line pointing at `doctor`.
func printSyncOverlapHint(targets map[string]config.TargetConfig, isProject, jsonOutput bool) {
	if jsonOutput {
		return
	}
	explained := map[string]bool{}
	for _, c := range config.SkillsFolderConflicts(targets) {
		ui.Warning("%s sync skills to %s with different filters, so each sync undoes the other",
			joinAnd(c.Targets), shortenPath(c.Path))
		for _, name := range c.Stop {
			fmt.Println(ui.DimText("  keep one: " + skillsOffCommand(name, isProject)))
		}
		for _, name := range c.Targets {
			explained[name] = true
		}
	}
	rest := 0
	for _, name := range config.DetectPathOverlap(targets, isProject) {
		if !explained[name] {
			rest++
		}
	}
	if rest > 0 {
		ui.Warning("%s share skill folders %s %s", plural(rest, "target"), theme.Dim().Render("— see"), theme.Accent().Render("skillshare doctor"))
	}
}

// skillsOffCommand is the command that stops syncing skills to target name.
func skillsOffCommand(name string, isProject bool) string {
	cmd := "skillshare target " + name + " --skills=false"
	if isProject {
		cmd += " -p"
	}
	return cmd
}

// joinAnd joins names as "a and b" or "a, b and c".
func joinAnd(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}
