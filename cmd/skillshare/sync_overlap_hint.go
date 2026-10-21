package main

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/theme"
	"skillshare/internal/ui"
)

// printSyncOverlapHint warns when enabled targets have overlapping skill paths
// (same primary path or cross-runtime discovery via also_scans). Targets that
// share a folder with different settings undo each other's sync, so they get
// a specific line and the command to keep one; any other overlap keeps the
// brief line pointing at `doctor`.
func printSyncOverlapHint(targets map[string]config.TargetConfig, projects map[string]config.ManagedProject, isProject, jsonOutput bool, defaultMode string, discovered []sync.DiscoveredSkill) {
	if jsonOutput {
		return
	}
	conflicts, rest := sync.TargetFolderConflicts(targets, isProject, defaultMode, sync.HarmlessOverlap(targets, defaultMode, discovered))
	printSyncFolderConflicts(conflicts, rest, isProject, targets, projects)
}

// printSyncFolderConflicts renders prepared diagnostics without rediscovering targets.
func printSyncFolderConflicts(conflicts []config.SkillsFolderConflict, rest int, isProject bool, targets map[string]config.TargetConfig, projects map[string]config.ManagedProject) {
	for _, c := range conflicts {
		ui.Warning("%s sync skills to %s with different settings, so each sync undoes the other", joinAnd(c.Targets), shortenPath(c.Path))
		for _, name := range c.Stop {
			fmt.Println(ui.DimText("  keep one: " + skillsOffCommand(name, isProject, targets, projects)))
		}
	}

	if rest > 0 {
		ui.Warning("%s share skill folders %s %s", plural(rest, "target"), theme.Dim().Render("— see"), theme.Accent().Render("skillshare doctor"))
	}
}

// skillsOffCommand is the command that stops syncing skills to target name.
// A project's skills setting covers all its targets, so a project target is
// stopped by dropping its tools from the project instead, unless it is the
// project's only target.
func skillsOffCommand(name string, isProject bool, targets map[string]config.TargetConfig, projects map[string]config.ManagedProject) string {
	if project, _, ok := config.SplitProjectTarget(name); ok && !isProject {
		cmd, ok := projectTargetOffCommand(name, targets, projects)
		if !ok {
			// Without the registration, list the project's targets to choose from.
			cmd = "skillshare project show "
			if strings.HasPrefix(project, "-") {
				cmd += "-- "
			}
			cmd += shellArg(project)
		}
		if runtime.GOOS == "windows" && strings.ContainsAny(cmd, "$`%!") {
			return "PowerShell: " + cmd
		}
		return cmd
	}
	cmd := "skillshare target " + name + " --skills=false"
	if isProject {
		cmd += " -p"
	}
	return cmd
}

// projectTargetOffCommand keeps the project's other targets, or turns skills
// off when name is the only one. Dropped tools also stop syncing agents; files
// already synced stay in place.
func projectTargetOffCommand(name string, targets map[string]config.TargetConfig, projects map[string]config.ManagedProject) (string, bool) {
	root := targets[name].ProjectRoot()
	if root == "" {
		return "", false
	}
	for key, entry := range projects {
		if filepath.Clean(config.ExpandPath(key)) != filepath.Clean(root) {
			continue
		}
		groups, err := entry.ToolGroups(key)
		if err != nil {
			return "", false
		}
		var keep []string
		found := false
		for _, g := range groups {
			if config.ProjectTargetName(entry.ProjectName(key), g.Target()) == name {
				found = true
				continue
			}
			keep = append(keep, g.Tools...)
		}
		if !found {
			return "", false
		}
		if len(keep) == 0 {
			return "skillshare project edit " + shellArg(root) + " --no-skills", true
		}
		return "skillshare project edit " + shellArg(root) + " --target " + shellArg(strings.Join(keep, ",")), true
	}
	return "", false
}

// shellArg leaves simple arguments bare, including non-leading tildes. Unix
// shells use literal single quotes; ordinary Windows arguments use double
// quotes for cmd.exe too. Windows expansion characters require PowerShell's
// literal quotes and a shell label at the command's call site.
func shellArg(s string) string {
	safe := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-.~/:@+="
	if runtime.GOOS == "windows" {
		safe += `\`
	}
	if s != "" && !strings.HasPrefix(s, "~") && strings.Trim(s, safe) == "" {
		return s
	}
	if runtime.GOOS == "windows" {
		if strings.ContainsAny(s, "$`%!") {
			return "'" + strings.ReplaceAll(s, "'", "''") + "'"
		}
		return `"` + s + `"`
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// joinAnd joins names as "a and b" or "a, b and c".
func joinAnd(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}
