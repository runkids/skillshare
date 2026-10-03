package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"skillshare/internal/config"
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
func folderConflictSuggestion(c config.SkillsFolderConflict, isProject bool) string {
	cmds := make([]string, 0, len(c.Stop))
	for _, name := range c.Stop {
		cmds = append(cmds, "`"+skillsOffCommand(name, isProject)+"`")
	}
	return fmt.Sprintf("Keep %s syncing skills to %s and stop the rest with %s.", c.Keep, c.Path, strings.Join(cmds, ", "))
}

func crossTargetDiscoverySuggestion(scanner string, writers []string, isProject bool) string {
	// Point at the scanner first: removing it only affects that runtime, while
	// removing a writer also hides skills from every other tool reading its path.
	return fmt.Sprintf("Choose one authoritative route for %s-visible skills; %s already reads the path written by %s, so start by removing the %s target (preview with `%s`). Removing %s instead also affects other tools that read the same path.",
		scanner, scanner, strings.Join(writers, ", "), scanner, targetRemoveDryRunCommand(isProject), strings.Join(writers, ", "))
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
	for _, c := range config.SkillsFolderConflicts(cfg.Targets) {
		conflicts[c.Path] = c
	}

	details := make([]string, 0, len(collisions))
	suggestions := make([]string, 0, len(collisions))
	for _, c := range collisions {
		detail := fmt.Sprintf("%s ← %s", c.path, strings.Join(c.targets, ", "))
		suggestion := sharedTargetPathsSuggestion(c.path, c.targets, isProject)
		if conflict, ok := conflicts[c.path]; ok {
			detail += " (different filters, so they undo each other on every sync)"
			suggestion = folderConflictSuggestion(conflict, isProject)
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
// the universal target's content too).
func checkCrossTargetDiscovery(cfg *config.Config, result *doctorResult, isProject bool) {
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

	type pathOverlap struct {
		sharedPath string
		writers    []string
	}
	type scannerOverlap struct {
		scanner     string
		scannerPath string
		paths       []pathOverlap
	}
	overlapsByScanner := make(map[string]*scannerOverlap)

	for scanner := range primaryByName {
		for _, p := range config.RuntimeScanPaths(scanner, isProject) {
			resolved := filepath.Clean(p)
			if resolved == primaryByName[scanner] {
				// The scanner's own write path — checkSharedTargetPaths covers it.
				continue
			}
			writers, ok := writersByPath[resolved]
			if !ok {
				continue
			}
			var others []string
			for _, w := range writers {
				if w != scanner {
					others = append(others, w)
				}
			}
			if len(others) == 0 {
				continue
			}
			sort.Strings(others)
			so, exists := overlapsByScanner[scanner]
			if !exists {
				so = &scannerOverlap{scanner: scanner, scannerPath: primaryByName[scanner]}
				overlapsByScanner[scanner] = so
			}
			so.paths = append(so.paths, pathOverlap{sharedPath: resolved, writers: others})
		}
	}

	if len(overlapsByScanner) == 0 {
		result.addCheck("cross_target_discovery", checkPass, "No cross-target discovery overlap", nil)
		return
	}

	// Stable per-scanner order.
	scannerNames := make([]string, 0, len(overlapsByScanner))
	for name := range overlapsByScanner {
		scannerNames = append(scannerNames, name)
	}
	sort.Strings(scannerNames)

	var details []string
	var suggestions []string
	for _, name := range scannerNames {
		so := overlapsByScanner[name]
		sort.Slice(so.paths, func(i, j int) bool { return so.paths[i].sharedPath < so.paths[j].sharedPath })

		// Union of all writers for the summary line.
		writerSet := map[string]struct{}{}
		for _, p := range so.paths {
			for _, w := range p.writers {
				writerSet[w] = struct{}{}
			}
		}
		writers := make([]string, 0, len(writerSet))
		for w := range writerSet {
			writers = append(writers, w)
		}
		sort.Strings(writers)

		ui.Warning("%s will see content from: %s", so.scanner, strings.Join(writers, ", "))
		for _, p := range so.paths {
			ui.Note(fmt.Sprintf("%s ← %s", shortenPath(p.sharedPath), strings.Join(p.writers, ", ")))
			details = append(details, fmt.Sprintf("%s (%s) also scans %s ← %s",
				so.scanner, so.scannerPath, p.sharedPath, strings.Join(p.writers, ", ")))
		}
		suggestion := crossTargetDiscoverySuggestion(so.scanner, writers, isProject)
		ui.Note("suggestion: " + suggestion)
		suggestions = append(suggestions, suggestion)
		result.addWarning()
	}

	msg := fmt.Sprintf("%d target(s) overlap with other targets' content via cross-runtime discovery", len(scannerNames))
	result.addCheckWithSuggestions("cross_target_discovery", checkWarning, msg, details, suggestions)
}
