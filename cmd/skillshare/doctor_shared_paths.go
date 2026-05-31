package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/ui"
)

// checkSharedTargetPaths warns when two or more enabled targets resolve to the
// same filesystem path after tilde expansion.
//
// Catches the "shared root" class of duplicate-skill problems (issue #135):
// e.g., enabling both `universal` and `warp` writes the same skill twice to
// ~/.agents/skills, and any runtime that scans that directory sees duplicates.
// Pure metadata check — no runtime probing required.
func checkSharedTargetPaths(cfg *config.Config, result *doctorResult) {
	pathTargets := make(map[string][]string)
	for name, target := range cfg.Targets {
		raw := target.SkillsConfig().Path
		if raw == "" {
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

	details := make([]string, 0, len(collisions))
	suggestions := make([]string, 0, len(collisions))
	for _, c := range collisions {
		ui.Warning("Shared path %s ← %s", c.path, strings.Join(c.targets, ", "))
		details = append(details, fmt.Sprintf("%s ← %s", c.path, strings.Join(c.targets, ", ")))
		suggestion := fmt.Sprintf("Choose one authoritative target for %s; disable or reconfigure the others: %s", c.path, strings.Join(c.targets, ", "))
		fmt.Println(ui.DimText("    suggestion: " + suggestion))
		suggestions = append(suggestions, suggestion)
		result.addWarning()
	}

	msg := fmt.Sprintf("%d shared target path(s) — enabled targets writing to the same directory may produce duplicate skills in runtime pickers", len(collisions))
	result.addCheckWithSuggestions("shared_target_paths", checkWarning, msg, details, suggestions)
}

// checkCrossTargetDiscovery warns when an enabled target's runtime is
// documented (via also_scans metadata) to also read a path that another enabled
// target writes to. Catches overlaps that checkSharedTargetPaths misses:
// different primary paths but converging runtime discovery (e.g. Codex's
// ~/.codex/skills primary plus its ~/.agents/skills also_scans means it sees
// the universal target's content too).
func checkCrossTargetDiscovery(cfg *config.Config, result *doctorResult, isProject bool) {
	primaryByName := make(map[string]string, len(cfg.Targets))
	for name, target := range cfg.Targets {
		raw := target.SkillsConfig().Path
		if raw == "" {
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
		var alsoPaths []string
		if isProject {
			alsoPaths = config.AlsoScansProject(scanner)
		} else {
			alsoPaths = config.AlsoScansGlobal(scanner)
		}
		for _, p := range alsoPaths {
			resolved := filepath.Clean(p)
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
		suggestion := fmt.Sprintf("Choose one authoritative route for %s-visible skills; disable or reconfigure overlapping writer target(s): %s", so.scanner, strings.Join(writers, ", "))
		fmt.Println(ui.DimText("    suggestion: " + suggestion))
		suggestions = append(suggestions, suggestion)
		for _, p := range so.paths {
			fmt.Println(ui.DimText(fmt.Sprintf("    %s ← %s", p.sharedPath, strings.Join(p.writers, ", "))))
			details = append(details, fmt.Sprintf("%s (%s) also scans %s ← %s",
				so.scanner, so.scannerPath, p.sharedPath, strings.Join(p.writers, ", ")))
		}
		result.addWarning()
	}

	msg := fmt.Sprintf("%d target(s) overlap with other targets' content via cross-runtime discovery", len(scannerNames))
	result.addCheckWithSuggestions("cross_target_discovery", checkWarning, msg, details, suggestions)
}
