package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
)

func cmdSyncExtras(args []string) error {
	start := time.Now()

	dryRun, force := parseSyncFlags(args)

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if len(cfg.Extras) == 0 {
		ui.Info("No extras configured.")
		fmt.Println()
		ui.Info("Add extras to your config.yaml:")
		fmt.Println()
		fmt.Println("  extras:")
		fmt.Println("    - name: rules")
		fmt.Println("      targets:")
		fmt.Println("        - path: ~/.claude/rules")
		fmt.Println("        - path: ~/.cursor/rules")
		fmt.Println("          mode: copy")
		return nil
	}

	sourceRoot := filepath.Dir(cfg.Source) // ~/.config/skillshare/

	if dryRun {
		ui.Warning("Dry run mode - no changes will be made")
	}

	var totalSynced, totalSkipped, totalPruned, totalErrors int

	for _, extra := range cfg.Extras {
		extraSource := filepath.Join(sourceRoot, extra.Name)

		ui.Header(capitalizeFirst(extra.Name))

		// Check if source directory exists
		if _, statErr := os.Stat(extraSource); os.IsNotExist(statErr) {
			ui.Info("Source directory does not exist: %s", shortenPath(extraSource))
			ui.Info("Create it to start syncing %s", extra.Name)
			continue
		}

		for _, target := range extra.Targets {
			mode := target.Mode
			if mode == "" {
				mode = "merge"
			}

			result, syncErr := sync.SyncExtra(extraSource, target.Path, mode, dryRun, force)
			shortTarget := shortenPath(target.Path)

			if syncErr != nil {
				ui.Warning("  %s: %v", shortTarget, syncErr)
				totalErrors++
				continue
			}

			totalSynced += result.Synced
			totalSkipped += result.Skipped
			totalPruned += result.Pruned
			totalErrors += len(result.Errors)

			// Report result
			verb := syncVerb(mode)
			if result.Synced > 0 {
				parts := []string{fmt.Sprintf("%d files %s", result.Synced, verb)}
				if result.Pruned > 0 {
					parts = append(parts, fmt.Sprintf("%d pruned", result.Pruned))
				}
				ui.Success("  %s  %s (%s)", shortTarget, strings.Join(parts, ", "), mode)
			} else if result.Skipped > 0 {
				ui.Warning("  %s  %d files skipped (use --force to override)", shortTarget, result.Skipped)
			} else {
				ui.Success("  %s  up to date (%s)", shortTarget, mode)
			}

			for _, e := range result.Errors {
				ui.Warning("    %s", e)
			}
		}
	}

	// Oplog
	status := "ok"
	if totalErrors > 0 {
		status = "partial"
	}
	e := oplog.NewEntry("sync-extras", status, time.Since(start))
	e.Args = map[string]any{
		"extras_count": len(cfg.Extras),
		"synced":       totalSynced,
		"skipped":      totalSkipped,
		"pruned":       totalPruned,
		"errors":       totalErrors,
		"dry_run":      dryRun,
		"force":        force,
	}
	oplog.WriteWithLimit(config.ConfigPath(), oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck

	return nil
}

// syncVerb returns a user-facing verb for the given sync mode.
func syncVerb(mode string) string {
	switch mode {
	case "copy":
		return "copied"
	case "symlink":
		return "linked"
	default:
		return "synced"
	}
}

// shortenPath replaces the home directory prefix with ~.
func shortenPath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	if strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}

// capitalizeFirst returns s with the first Unicode character upper-cased.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}
