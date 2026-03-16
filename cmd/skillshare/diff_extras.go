package main

import (
	"fmt"
	"os"
	"path/filepath"

	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
)

// extraDiffResult holds diff for one extra → one target.
type extraDiffResult struct {
	extraName  string
	targetPath string
	mode       string
	synced     bool
	errMsg     string
	items      []extraDiffItem
}

type extraDiffItem struct {
	action string // "add", "remove", "modify"
	file   string // relative path
	reason string
}

type extraDiffJSONEntry struct {
	Name   string              `json:"name"`
	Target string              `json:"target"`
	Mode   string              `json:"mode"`
	Synced bool                `json:"synced"`
	Error  string              `json:"error,omitempty"`
	Items  []extraDiffJSONItem `json:"items"`
}

type extraDiffJSONItem struct {
	Action string `json:"action"`
	File   string `json:"file"`
	Reason string `json:"reason"`
}

// collectExtrasDiff computes diff for all configured extras.
func collectExtrasDiff(extras []config.ExtraConfig, sourceResolver func(string) string) []extraDiffResult {
	var results []extraDiffResult

	for _, extra := range extras {
		sourceDir := sourceResolver(extra.Name)

		files, err := sync.DiscoverExtraFiles(sourceDir)
		if err != nil {
			// Source doesn't exist — report for each target
			for _, t := range extra.Targets {
				results = append(results, extraDiffResult{
					extraName:  extra.Name,
					targetPath: t.Path,
					mode:       sync.EffectiveMode(t.Mode),
					errMsg:     "source directory not found",
				})
			}
			continue
		}

		for _, t := range extra.Targets {
			mode := sync.EffectiveMode(t.Mode)
			r := extraDiffResult{
				extraName:  extra.Name,
				targetPath: t.Path,
				mode:       mode,
			}

			if _, statErr := os.Stat(t.Path); os.IsNotExist(statErr) {
				// Target doesn't exist — all files need to be added
				for _, f := range files {
					r.items = append(r.items, extraDiffItem{
						action: "add",
						file:   f,
						reason: "target directory missing",
					})
				}
				results = append(results, r)
				continue
			}

			// Compare source files with target
			allSynced := true
			for _, rel := range files {
				sourceFile := filepath.Join(sourceDir, rel)
				targetFile := filepath.Join(t.Path, rel)

				tInfo, statErr := os.Lstat(targetFile)
				if statErr != nil {
					r.items = append(r.items, extraDiffItem{
						action: "add",
						file:   rel,
						reason: "missing in target",
					})
					allSynced = false
					continue
				}

				switch mode {
				case "merge", "symlink":
					if tInfo.Mode()&os.ModeSymlink != 0 {
						link, _ := os.Readlink(targetFile)
						if link != sourceFile {
							r.items = append(r.items, extraDiffItem{
								action: "modify",
								file:   rel,
								reason: fmt.Sprintf("symlink points to %s", link),
							})
							allSynced = false
						}
					} else {
						r.items = append(r.items, extraDiffItem{
							action: "modify",
							file:   rel,
							reason: "not a symlink (local file)",
						})
						allSynced = false
					}
				case "copy":
					if !tInfo.Mode().IsRegular() {
						r.items = append(r.items, extraDiffItem{
							action: "modify",
							file:   rel,
							reason: "not a regular file",
						})
						allSynced = false
					}
				}
			}

			r.synced = allSynced
			results = append(results, r)
		}
	}

	return results
}

// renderExtrasDiffPlain renders extras diff in plain text.
func renderExtrasDiffPlain(results []extraDiffResult) {
	fmt.Println()
	ui.Header("Extras")

	for _, r := range results {
		if r.errMsg != "" {
			ui.Warning("  %s → %s: %s", r.extraName, shortenPath(r.targetPath), r.errMsg)
			continue
		}

		if r.synced {
			ui.Success("  %s → %s: synced (%s)", r.extraName, shortenPath(r.targetPath), r.mode)
			continue
		}

		ui.Warning("  %s → %s: %d difference(s) (%s)", r.extraName, shortenPath(r.targetPath), len(r.items), r.mode)
		for _, item := range r.items {
			switch item.action {
			case "add":
				fmt.Printf("    + %s  %s\n", item.file, item.reason)
			case "remove":
				fmt.Printf("    - %s  %s\n", item.file, item.reason)
			case "modify":
				fmt.Printf("    ~ %s  %s\n", item.file, item.reason)
			}
		}
	}
}

// extrasDiffToJSON converts internal results to JSON-friendly structs.
func extrasDiffToJSON(results []extraDiffResult) []extraDiffJSONEntry {
	var entries []extraDiffJSONEntry
	for _, r := range results {
		entry := extraDiffJSONEntry{
			Name:   r.extraName,
			Target: r.targetPath,
			Mode:   r.mode,
			Synced: r.synced,
			Error:  r.errMsg,
		}
		for _, item := range r.items {
			entry.Items = append(entry.Items, extraDiffJSONItem{
				Action: item.action,
				File:   item.file,
				Reason: item.reason,
			})
		}
		entries = append(entries, entry)
	}
	return entries
}
