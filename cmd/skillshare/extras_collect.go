package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
)

func cmdExtrasCollect(args []string) error {
	start := time.Now()

	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	cwd, _ := os.Getwd()
	if mode == modeAuto {
		if projectConfigExists(cwd) {
			mode = modeProject
		} else {
			mode = modeGlobal
		}
	}

	applyModeLabel(mode)

	// Parse flags
	var name string
	var fromPath string
	dryRun := false
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--from":
			if i+1 >= len(rest) {
				return fmt.Errorf("--from requires a path argument")
			}
			i++
			fromPath = rest[i]
		case "--dry-run":
			dryRun = true
		case "--help", "-h":
			printExtrasCollectHelp()
			return nil
		default:
			if name == "" {
				name = rest[i]
			} else {
				return fmt.Errorf("unexpected argument: %s", rest[i])
			}
		}
	}

	if name == "" {
		return fmt.Errorf("extras name is required: skillshare extras collect <name> --from <target-path>")
	}

	if mode == modeProject {
		return extrasCollectProject(cwd, name, fromPath, dryRun, start)
	}
	return extrasCollectGlobal(name, fromPath, dryRun, start)
}

func extrasCollectGlobal(name, fromPath string, dryRun bool, start time.Time) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	extra, targetPath, flatten, err := resolveCollectExtra(cfg.Extras, name, fromPath)
	if err != nil {
		return err
	}

	targetPath = config.ExpandPath(targetPath)
	sourceDir := config.ResolveExtrasSourceDir(*extra, cfg.ExtrasSource, cfg.Source)
	return runCollect(sourceDir, targetPath, extra.Name, dryRun, flatten, "global", config.ConfigPath(), start, "")
}

func extrasCollectProject(cwd, name, fromPath string, dryRun bool, start time.Time) error {
	projCfg, err := config.LoadProject(cwd)
	if err != nil {
		return err
	}

	extra, targetPath, flatten, err := resolveCollectExtra(projCfg.Extras, name, fromPath)
	if err != nil {
		return err
	}

	// Expand ~ and resolve relative target path
	targetPath = config.ExpandPath(targetPath)
	if !filepath.IsAbs(targetPath) {
		targetPath = filepath.Join(cwd, targetPath)
	}

	sourceDir := config.ExtrasSourceDirProject(cwd, extra.Name)
	return runCollect(sourceDir, targetPath, extra.Name, dryRun, flatten, "project", config.ProjectConfigPath(cwd), start, cwd)
}

// resolveCollectExtra finds the extra by name and determines target path and flatten flag.
func resolveCollectExtra(extras []config.ExtraConfig, name, fromPath string) (*config.ExtraConfig, string, bool, error) {
	var found *config.ExtraConfig
	for i, e := range extras {
		if e.Name == name {
			found = &extras[i]
			break
		}
	}
	if found == nil {
		return nil, "", false, fmt.Errorf("extra %q not found in config", name)
	}

	targetPath := fromPath
	flatten := false
	if targetPath == "" {
		if len(found.Targets) == 1 {
			targetPath = found.Targets[0].Path
			flatten = found.Targets[0].Flatten
		} else {
			return nil, "", false, fmt.Errorf("multiple targets configured for %q; use --from <path> to specify which target to collect from", name)
		}
	} else {
		// Find flatten value for the matching target
		for _, t := range found.Targets {
			if t.Path == targetPath {
				flatten = t.Flatten
				break
			}
		}
	}

	return found, targetPath, flatten, nil
}

func runCollect(sourceDir, targetPath, name string, dryRun, flatten bool, scope, cfgPath string, start time.Time, projectRoot string) error {
	if dryRun {
		ui.Warning("Dry run mode - no changes will be made")
	}

	result, err := sync.CollectExtraFiles(sourceDir, targetPath, dryRun, flatten, projectRoot)
	if err != nil {
		return err
	}

	if result.Collected > 0 {
		verb := "collected"
		if dryRun {
			verb = "would collect"
		}
		ui.Success("%d files %s from %s to extras/%s/", result.Collected, verb, shortenPath(targetPath), name)
	} else {
		ui.Info("No local files to collect from %s", shortenPath(targetPath))
	}

	if result.Skipped > 0 {
		ui.Info("%d files skipped (already synced or exist in source)", result.Skipped)
	}

	for _, e := range result.Errors {
		ui.Warning("  %s", e)
	}

	// Oplog
	status := "ok"
	if len(result.Errors) > 0 {
		status = "partial"
	}
	e := oplog.NewEntry("extras-collect", status, time.Since(start))
	e.Args = map[string]any{
		"name":      name,
		"scope":     scope,
		"collected": result.Collected,
		"skipped":   result.Skipped,
		"errors":    len(result.Errors),
		"dry_run":   dryRun,
	}
	oplog.WriteWithLimit(cfgPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck

	return nil
}

func printExtrasCollectHelp() {
	fmt.Println(`Usage: skillshare extras collect <name> [options]

Collect local files from a target back into the extras source directory.
Files are copied to source and replaced with symlinks in the target.

Arguments:
  name                Name of the extra to collect for

Options:
  --from <path>       Target directory to collect from (required if multiple targets)
  --dry-run           Show what would be collected without making changes
  --project, -p       Use project mode (.skillshare/)
  --global, -g        Use global mode (~/.config/skillshare/)
  --help, -h          Show this help

Examples:
  skillshare extras collect rules
  skillshare extras collect rules --from ~/.claude/rules --dry-run
  skillshare extras collect prompts -p`)
}
