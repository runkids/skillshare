package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
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
func collectExtrasDiff(extras []config.ExtraConfig, sourceResolver func(config.ExtraConfig) string) []extraDiffResult {
	var results []extraDiffResult

	for _, extra := range extras {
		sourceDir := sourceResolver(extra)

		if extra.File != "" {
			results = append(results, collectExtraFileDiff(extra, sourceDir)...)
			continue
		}

		files, err := sync.DiscoverExtraFiles(sourceDir)
		if err != nil {
			// Source doesn't exist — report for each target
			for _, t := range extra.Targets {
				results = append(results, extraDiffResult{
					extraName:  extra.Name,
					targetPath: t.Path,
					mode:       sync.ExtraTargetMode(t.Mode, extra.File != ""),
					errMsg:     "source directory not found",
				})
			}
			continue
		}

		for _, t := range extra.Targets {
			mode := sync.ExtraTargetMode(t.Mode, extra.File != "")
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
					if utils.IsLinkMode(targetFile, tInfo.Mode()) {
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

// collectExtraFileDiff reports each target of a single-file extra as one item.
func collectExtraFileDiff(extra config.ExtraConfig, sourceDir string) []extraDiffResult {
	var results []extraDiffResult
	for _, t := range extra.Targets {
		f := sync.NewExtraFile(sourceDir, extra.File, t.Path, t.As, t.Mode)
		r := extraDiffResult{extraName: extra.Name, targetPath: t.Path, mode: f.Mode}
		switch status := sync.ExtraFileStatus(f); status {
		case "synced":
			r.synced = true
		case "no source":
			r.errMsg = "source file not found"
		case "not synced":
			r.items = append(r.items, extraDiffItem{action: "add", file: filepath.Base(f.Target), reason: "missing in target"})
		default:
			r.items = append(r.items, extraDiffItem{action: "modify", file: filepath.Base(f.Target), reason: status})
		}
		results = append(results, r)
	}
	return results
}

// renderExtrasDiffPlain prints one row per extra and target, with the
// differing files below it, and returns how many are out of sync.
func renderExtrasDiffPlain(results []extraDiffResult) int {
	labels := make([]string, len(results))
	for i, r := range results {
		labels[i] = r.extraName
	}
	width := ui.RowWidth(labels...)
	need := 0
	for _, r := range results {
		dest := shortenPath(r.targetPath)
		switch {
		case r.errMsg != "":
			need++
			ui.Row(ui.MarkFail, r.extraName, dest+ui.DimText(" · "+r.errMsg), width)
		case r.synced:
			ui.Row(ui.MarkOK, r.extraName, dest+ui.DimText(" · in sync"), width)
		default:
			need++
			ui.Row(ui.MarkWarn, r.extraName, dest+ui.DimText(" · "+plural(len(r.items), "difference")+" · "+r.mode), width)
			for _, item := range r.items {
				sign := map[string]string{"add": "+", "remove": "-", "modify": "~"}[item.action]
				ui.Note(strings.Repeat(" ", width+2) + sign + " " + item.file + "  " + item.reason)
			}
		}
	}
	return need
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
