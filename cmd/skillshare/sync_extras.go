package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
)

type syncExtrasJSONOutput struct {
	Extras   []syncExtrasJSONEntry `json:"extras"`
	Duration string                `json:"duration"`
}

type syncExtrasJSONEntry struct {
	Name    string                 `json:"name"`
	Targets []syncExtrasJSONTarget `json:"targets"`
}

type syncExtrasJSONTarget struct {
	Path    string `json:"path"`
	Mode    string `json:"mode"`
	Synced  int    `json:"synced"`
	Skipped int    `json:"skipped"`
	Pruned  int    `json:"pruned"`
	Error   string `json:"error,omitempty"`
}

func cmdSyncExtras(args []string) error {
	start := time.Now()

	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	dryRun, force, jsonOutput := parseSyncFlags(rest)

	cwd, _ := os.Getwd()
	if mode == modeAuto {
		if projectConfigExists(cwd) {
			mode = modeProject
		} else {
			mode = modeGlobal
		}
	}

	applyModeLabel(mode)

	if mode == modeProject {
		return cmdSyncExtrasProject(cwd, dryRun, force, jsonOutput, start)
	}
	return cmdSyncExtrasGlobal(dryRun, force, jsonOutput, start)
}

func cmdSyncExtrasGlobal(dryRun, force, jsonOutput bool, start time.Time) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if len(cfg.Extras) == 0 {
		if jsonOutput {
			return writeJSON(&syncExtrasJSONOutput{Extras: []syncExtrasJSONEntry{}, Duration: formatDuration(start)})
		}
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

	configDir := filepath.Dir(cfg.Source)

	// Auto-migrate legacy extras directories (flat → extras/<name>/)
	if warnings := config.MigrateExtrasDir(configDir, cfg.Extras); len(warnings) > 0 {
		for _, w := range warnings {
			ui.Warning(w)
		}
	}

	if dryRun && !jsonOutput {
		ui.Warning("Dry run mode - no changes will be made")
	}

	var totalSynced, totalSkipped, totalPruned, totalErrors int
	var jsonEntries []syncExtrasJSONEntry

	for _, extra := range cfg.Extras {
		extraSource := config.ExtrasSourceDir(cfg.Source, extra.Name)

		if !jsonOutput {
			ui.Header(capitalize(extra.Name))
		}

		// Check if source directory exists
		if _, statErr := os.Stat(extraSource); os.IsNotExist(statErr) {
			if !jsonOutput {
				ui.Info("Source directory does not exist: %s", shortenPath(extraSource))
				ui.Info("Create it to start syncing %s", extra.Name)
			}
			if jsonOutput {
				jsonEntries = append(jsonEntries, syncExtrasJSONEntry{Name: extra.Name, Targets: []syncExtrasJSONTarget{}})
			}
			continue
		}

		jsonEntry := syncExtrasJSONEntry{Name: extra.Name}

		for _, target := range extra.Targets {
			mode := target.Mode
			if mode == "" {
				mode = "merge"
			}
			result, syncErr := sync.SyncExtra(extraSource, target.Path, mode, dryRun, force)
			shortTarget := shortenPath(target.Path)

			jsonTarget := syncExtrasJSONTarget{
				Path: target.Path,
				Mode: mode,
			}

			if syncErr != nil {
				if !jsonOutput {
					ui.Warning("  %s: %v", shortTarget, syncErr)
				}
				jsonTarget.Error = syncErr.Error()
				jsonEntry.Targets = append(jsonEntry.Targets, jsonTarget)
				totalErrors++
				continue
			}

			totalSynced += result.Synced
			totalSkipped += result.Skipped
			totalPruned += result.Pruned
			totalErrors += len(result.Errors)

			jsonTarget.Synced = result.Synced
			jsonTarget.Skipped = result.Skipped
			jsonTarget.Pruned = result.Pruned
			if len(result.Errors) > 0 {
				jsonTarget.Error = strings.Join(result.Errors, "; ")
			}
			jsonEntry.Targets = append(jsonEntry.Targets, jsonTarget)

			if !jsonOutput {
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

		jsonEntries = append(jsonEntries, jsonEntry)
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

	if jsonOutput {
		output := syncExtrasJSONOutput{
			Extras:   jsonEntries,
			Duration: formatDuration(start),
		}
		return writeJSON(&output)
	}

	if totalErrors > 0 {
		return fmt.Errorf("%d extras sync error(s)", totalErrors)
	}
	return nil
}

func cmdSyncExtrasProject(cwd string, dryRun, force, jsonOutput bool, start time.Time) error {
	projCfg, err := config.LoadProject(cwd)
	if err != nil {
		return err
	}

	if len(projCfg.Extras) == 0 {
		if jsonOutput {
			return writeJSON(&syncExtrasJSONOutput{Extras: []syncExtrasJSONEntry{}, Duration: formatDuration(start)})
		}
		ui.Info("No extras configured in project.")
		ui.Info("Run 'skillshare extras init <name> --target <path> -p' to add one.")
		return nil
	}

	if dryRun && !jsonOutput {
		ui.Warning("Dry run mode - no changes will be made")
	}

	var totalSynced, totalSkipped, totalPruned, totalErrors int
	var jsonEntries []syncExtrasJSONEntry

	for _, extra := range projCfg.Extras {
		extraSource := config.ExtrasSourceDirProject(cwd, extra.Name)

		if !jsonOutput {
			ui.Header(capitalize(extra.Name))
		}

		if _, statErr := os.Stat(extraSource); os.IsNotExist(statErr) {
			if !jsonOutput {
				ui.Info("Source directory does not exist: %s", extraSource)
				ui.Info("Create it to start syncing %s", extra.Name)
			}
			if jsonOutput {
				jsonEntries = append(jsonEntries, syncExtrasJSONEntry{Name: extra.Name, Targets: []syncExtrasJSONTarget{}})
			}
			continue
		}

		jsonEntry := syncExtrasJSONEntry{Name: extra.Name}

		for _, target := range extra.Targets {
			mode := target.Mode
			if mode == "" {
				mode = "merge"
			}

			// Resolve relative paths against project root
			targetPath := target.Path
			if !filepath.IsAbs(targetPath) {
				targetPath = filepath.Join(cwd, targetPath)
			}

			result, syncErr := sync.SyncExtra(extraSource, targetPath, mode, dryRun, force)
			shortTarget := shortenPath(targetPath)

			jsonTarget := syncExtrasJSONTarget{
				Path: targetPath,
				Mode: mode,
			}

			if syncErr != nil {
				if !jsonOutput {
					ui.Warning("  %s: %v", shortTarget, syncErr)
				}
				jsonTarget.Error = syncErr.Error()
				jsonEntry.Targets = append(jsonEntry.Targets, jsonTarget)
				totalErrors++
				continue
			}

			totalSynced += result.Synced
			totalSkipped += result.Skipped
			totalPruned += result.Pruned
			totalErrors += len(result.Errors)

			jsonTarget.Synced = result.Synced
			jsonTarget.Skipped = result.Skipped
			jsonTarget.Pruned = result.Pruned
			if len(result.Errors) > 0 {
				jsonTarget.Error = strings.Join(result.Errors, "; ")
			}
			jsonEntry.Targets = append(jsonEntry.Targets, jsonTarget)

			if !jsonOutput {
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

		jsonEntries = append(jsonEntries, jsonEntry)
	}

	status := "ok"
	if totalErrors > 0 {
		status = "partial"
	}
	e := oplog.NewEntry("sync-extras", status, time.Since(start))
	e.Args = map[string]any{
		"extras_count": len(projCfg.Extras),
		"synced":       totalSynced,
		"skipped":      totalSkipped,
		"pruned":       totalPruned,
		"errors":       totalErrors,
		"dry_run":      dryRun,
		"force":        force,
		"scope":        "project",
	}
	oplog.WriteWithLimit(config.ProjectConfigPath(cwd), oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck

	if jsonOutput {
		output := syncExtrasJSONOutput{
			Extras:   jsonEntries,
			Duration: formatDuration(start),
		}
		return writeJSON(&output)
	}

	if totalErrors > 0 {
		return fmt.Errorf("%d extras sync error(s)", totalErrors)
	}
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

// cachedHome caches the home directory for shortenPath.
var cachedHome = func() string {
	h, _ := os.UserHomeDir()
	return h
}()

// shortenPath replaces the home directory prefix with ~.
func shortenPath(p string) string {
	if cachedHome != "" && strings.HasPrefix(p, cachedHome) {
		return "~" + p[len(cachedHome):]
	}
	return p
}
