package main

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
)

func targetRemoveDryRunCommand(isProject bool) string {
	modeFlag := "--global"
	if isProject {
		modeFlag = "--project"
	}
	return fmt.Sprintf("skillshare target remove <name> %s --dry-run", modeFlag)
}

func sharedTargetPathsSuggestion(path string, targets []string, isProject bool) string {
	return fmt.Sprintf("Choose one authoritative target for %s; preview removing duplicate targets with `%s` (currently: %s).",
		path, targetRemoveDryRunCommand(isProject), strings.Join(targets, ", "))
}

// folderConflictSuggestion says which targets to stop syncing skills for when
// targets share a folder with different settings.
func folderConflictSuggestion(c config.SkillsFolderConflict, cfg *config.Config, isProject bool) string {
	cmds := make([]string, 0, len(c.Stop))
	for _, name := range c.Stop {
		cmds = append(cmds, "`"+skillsOffCommand(name, isProject, cfg.Targets, cfg.Projects)+"`")
	}
	return fmt.Sprintf("Keep %s syncing skills to %s and stop the rest with %s.", c.Keep, c.Path, strings.Join(cmds, ", "))
}

func crossTargetDiscoverySuggestion(scanner string, writers []string, isProject bool) string {
	// Point at the scanner first: removing it only affects that runtime, while
	// removing a writer also hides skills from every other tool reading its path.
	return fmt.Sprintf("Choose one authoritative route for %s-visible skills; %s already reads the path written by %s, so start by removing the %s target (preview with `%s`). Removing %s instead also affects other tools that read the same path.",
		scanner, scanner, strings.Join(writers, ", "), scanner, targetRemoveDryRunCommand(isProject), strings.Join(writers, ", "))
}

// leakedSkillsSuggestion also names the most specific environment variable
// that makes the runtime skip every one of paths, when one does.
func leakedSkillsSuggestion(scanner, agent string, paths []string, isProject bool) string {
	s := fmt.Sprintf("These skills are not in %s's folder, but %s also reads other targets' folders. Sync them to %s too", scanner, scanner, scanner)
	lookup := scanner
	if agent != "" {
		lookup = agent
	}
	disabledBy := config.AlsoScansDisabledBy(lookup, isProject)
	for _, v := range disabledBy[paths[0]] {
		if !slices.ContainsFunc(paths[1:], func(p string) bool { return !slices.Contains(disabledBy[p], v) }) {
			s += ", or where " + scanner + " runs, set " + v + "=1"
			break
		}
	}
	return s + "."
}

// checkSharedTargetPaths warns when two or more enabled targets resolve to the
// same filesystem path after tilde expansion.
//
// Catches the "shared root" class of duplicate-skill problems (issue #135):
// e.g., enabling both `universal` and `warp` writes the same skill twice to
// ~/.agents/skills, and any runtime that scans that directory sees duplicates.
// Pure metadata check — no runtime probing required.
func checkSharedTargetPaths(cfg *config.Config, result *doctorResult, isProject bool) {
	pathTargets := make(map[string][]string)
	for name, target := range cfg.Targets {
		raw := target.SkillsConfig().Path
		// A target with skills off neither writes nor counts as a scanner.
		if raw == "" || !target.SkillsConfig().IsEnabled() {
			continue
		}
		resolved := filepath.Clean(config.ExpandPath(raw))
		pathTargets[resolved] = append(pathTargets[resolved], name)
	}

	type collision struct {
		path    string
		targets []string
	}
	var collisions []collision
	for p, names := range pathTargets {
		if len(names) < 2 {
			continue
		}
		sort.Strings(names)
		collisions = append(collisions, collision{path: p, targets: names})
	}

	if len(collisions) == 0 {
		result.addCheck("shared_target_paths", checkPass, "No shared target paths", nil)
		return
	}

	sort.Slice(collisions, func(i, j int) bool {
		return collisions[i].path < collisions[j].path
	})

	conflicts := make(map[string]config.SkillsFolderConflict)
	for _, c := range config.SkillsFolderConflicts(cfg.Targets, cfg.Mode) {
		conflicts[c.Path] = c
	}

	details := make([]string, 0, len(collisions))
	suggestions := make([]string, 0, len(collisions))
	for _, c := range collisions {
		detail := fmt.Sprintf("%s ← %s", c.path, strings.Join(c.targets, ", "))
		suggestion := sharedTargetPathsSuggestion(c.path, c.targets, isProject)
		if conflict, ok := conflicts[c.path]; ok {
			detail += " (different settings, so they undo each other on every sync)"
			suggestion = folderConflictSuggestion(conflict, cfg, isProject)
		}
		ui.Warning("Shared path %s", detail)
		details = append(details, detail)
		ui.Note("suggestion: " + suggestion)
		suggestions = append(suggestions, suggestion)
		result.addWarning()
	}

	msg := fmt.Sprintf("%d shared target path(s) — enabled targets writing to the same directory may produce duplicate skills in runtime pickers", len(collisions))
	result.addCheckWithSuggestions("shared_target_paths", checkWarning, msg, details, suggestions)
}

// checkCrossTargetDiscovery warns when an enabled target's runtime is
// documented (via its target metadata) to also read a path that another enabled
// target writes to. Catches overlaps that checkSharedTargetPaths misses:
// different primary paths but converging runtime discovery (e.g. Codex's
// ~/.codex/skills primary plus its ~/.agents/skills also_scans means it sees
// the universal target's content too). For a runtime that keeps one skill per
// name, only the skills missing from its own folder count.
func checkCrossTargetDiscovery(cfg *config.Config, result *doctorResult, isProject bool, discovered []sync.DiscoveredSkill) {
	primaryByName := make(map[string]string, len(cfg.Targets))
	for name, target := range cfg.Targets {
		raw := target.SkillsConfig().Path
		// A target with skills off neither writes nor counts as a scanner.
		if raw == "" || !target.SkillsConfig().IsEnabled() {
			continue
		}
		primaryByName[name] = filepath.Clean(config.ExpandPath(raw))
	}

	writersByPath := make(map[string][]string)
	for name, path := range primaryByName {
		writersByPath[path] = append(writersByPath[path], name)
	}

	// Say which written folders a runtime skips because of this environment:
	// the runtime may run with another one.
	var skipped []string
	for _, scanner := range slices.Sorted(maps.Keys(primaryByName)) {
		for _, s := range config.ScansTurnedOff(scanner, isProject) {
			if _, written := writersByPath[filepath.Clean(s.Path)]; !written {
				continue
			}
			note := fmt.Sprintf("%s skips %s: %s is set in this environment", scanner, shortenPath(s.Path), s.EnvVar)
			ui.Note(note)
			skipped = append(skipped, note)
		}
	}

	type pathOverlap struct {
		sharedPath string
		writers    []string
		leaked     []string
	}
	type scannerOverlap struct {
		scanner     string
		scannerPath string
		paths       []pathOverlap
		// unknown means leaked may miss skills the scanner should not load.
		unknown bool
	}
	overlapsByScanner := make(map[string]*scannerOverlap)

	for scanner := range primaryByName {
		for _, p := range config.TargetScanPaths(scanner, cfg.Targets[scanner], isProject) {
			resolved := filepath.Clean(p)
			if resolved == primaryByName[scanner] {
				// The scanner's own write path — checkSharedTargetPaths covers it.
				continue
			}
			writers, ok := writersByPath[resolved]
			if !ok {
				continue
			}
			var others, leaked []string
			unknown := false
			for _, w := range writers {
				if w == scanner {
					continue
				}
				l, ok := sync.LeakedSkills(scanner, w, cfg.Targets, cfg.Mode, discovered)
				if ok && len(l) == 0 {
					continue
				}
				unknown = unknown || !ok
				others = append(others, w)
				leaked = append(leaked, l...)
			}
			if len(others) == 0 {
				continue
			}
			sort.Strings(others)
			slices.Sort(leaked)
			so, exists := overlapsByScanner[scanner]
			if !exists {
				so = &scannerOverlap{scanner: scanner, scannerPath: primaryByName[scanner]}
				overlapsByScanner[scanner] = so
			}
			so.unknown = so.unknown || unknown
			so.paths = append(so.paths, pathOverlap{sharedPath: resolved, writers: others, leaked: slices.Compact(leaked)})
		}
	}

	if len(overlapsByScanner) == 0 {
		result.addCheck("cross_target_discovery", checkPass, "No cross-target discovery overlap", skipped)
		return
	}

	// Stable per-scanner order.
	scannerNames := make([]string, 0, len(overlapsByScanner))
	for name := range overlapsByScanner {
		scannerNames = append(scannerNames, name)
	}
	sort.Strings(scannerNames)

	details := skipped
	var suggestions []string
	for _, name := range scannerNames {
		so := overlapsByScanner[name]
		sort.Slice(so.paths, func(i, j int) bool { return so.paths[i].sharedPath < so.paths[j].sharedPath })

		// Union of writers, leaked skills and paths for the summary line.
		var writers, leaked, paths []string
		for _, p := range so.paths {
			writers = append(writers, p.writers...)
			leaked = append(leaked, p.leaked...)
			paths = append(paths, p.sharedPath)
		}
		slices.Sort(writers)
		writers = slices.Compact(writers)
		slices.Sort(leaked)
		leaked = slices.Compact(leaked)

		header := fmt.Sprintf("%s will see content from: %s", so.scanner, strings.Join(writers, ", "))
		suggestion := crossTargetDiscoverySuggestion(so.scanner, writers, isProject)
		if !so.unknown {
			header = fmt.Sprintf("%s loads %s missing from its own folder, from: %s", so.scanner, plural(len(leaked), "skill"), strings.Join(writers, ", "))
			suggestion = leakedSkillsSuggestion(so.scanner, cfg.Targets[so.scanner].Agent, paths, isProject)
		}
		ui.Warning("%s", header)
		for _, p := range so.paths {
			note := fmt.Sprintf("%s ← %s", shortenPath(p.sharedPath), strings.Join(p.writers, ", "))
			detail := fmt.Sprintf("%s (%s) also scans %s ← %s", so.scanner, so.scannerPath, p.sharedPath, strings.Join(p.writers, ", "))
			if !so.unknown {
				note += ": " + strings.Join(p.leaked, ", ")
				detail += " and loads skills missing from its own folder: " + strings.Join(p.leaked, ", ")
			}
			ui.Note(note)
			details = append(details, detail)
		}
		ui.Note("suggestion: " + suggestion)
		suggestions = append(suggestions, suggestion)
		result.addWarning()
	}

	msg := fmt.Sprintf("%d target(s) overlap with other targets' content via cross-runtime discovery", len(scannerNames))
	result.addCheckWithSuggestions("cross_target_discovery", checkWarning, msg, details, suggestions)
}
