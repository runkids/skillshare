package main

import (
	"errors"
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
	mode = resolveAutoMode(mode, cwd)

	applyModeLabel(mode)

	// Parse flags
	var name string
	var fromPath string
	dryRun := false
	force := false
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
		case "--force", "-f":
			force = true
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
		return extrasCollectProject(cwd, name, fromPath, dryRun, force, start)
	}
	return extrasCollectGlobal(name, fromPath, dryRun, force, start)
}

func extrasCollectGlobal(name, fromPath string, dryRun, force bool, start time.Time) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	extra, target, err := resolveCollectExtra(cfg.Extras, name, fromPath, modeGlobal, "")
	if err != nil {
		return err
	}

	targetPath := config.ExpandPath(target.Path)
	if target.Extension != "" {
		ui.Warning("Skipping %s — managed by extension %q (one-way; collect not supported)", shortenPath(targetPath), target.Extension)
		return nil
	}

	sourceDir := config.ResolveExtrasSourceDir(*extra, cfg.EffectiveExtrasSource(), cfg.EffectiveSkillsSource())
	return runCollect(sourceDir, targetPath, extra.Name, target.Mode, dryRun, force, target.Flatten, "global", config.ConfigPath(), start, "")
}

func extrasCollectProject(cwd, name, fromPath string, dryRun, force bool, start time.Time) error {
	projCfg, err := config.LoadProject(cwd)
	if err != nil {
		return err
	}

	extra, target, err := resolveCollectExtra(projCfg.Extras, name, fromPath, modeProject, cwd)
	if err != nil {
		return err
	}

	// Expand ~ and resolve relative target path
	expandedPath := config.ExpandPath(target.Path)
	if !filepath.IsAbs(expandedPath) {
		expandedPath = filepath.Join(cwd, expandedPath)
	}

	if target.Extension != "" {
		ui.Warning("Skipping %s — managed by extension %q (one-way; collect not supported)", shortenPath(expandedPath), target.Extension)
		return nil
	}

	sourceDir := config.ResolveExtrasSourceDirProject(*extra, projCfg.EffectiveExtrasSource(cwd), cwd)
	return runCollect(sourceDir, expandedPath, extra.Name, target.Mode, dryRun, force, target.Flatten, "project", config.ProjectConfigPath(cwd), start, cwd)
}

// errSingleFileCollect: collect walks a whole target directory, which for a
// single-file extra would pull unrelated files (e.g. all of ~/.claude) into
// the source.
var errSingleFileCollect = errors.New("collect is not supported for single-file extras")

// resolveCollectExtra finds the extra by name and the target to collect from.
// With --from, an unconfigured path yields a target with default settings.
func resolveCollectExtra(extras []config.ExtraConfig, name, fromPath string, mode runMode, cwd string) (*config.ExtraConfig, config.ExtraTargetConfig, error) {
	var found *config.ExtraConfig
	for i, e := range extras {
		if e.Name == name {
			found = &extras[i]
			break
		}
	}
	if found == nil {
		return nil, config.ExtraTargetConfig{}, fmt.Errorf("extra %q not found in config", name)
	}
	if found.File != "" {
		return nil, config.ExtraTargetConfig{}, errSingleFileCollect
	}

	if fromPath == "" {
		if len(found.Targets) != 1 {
			return nil, config.ExtraTargetConfig{}, fmt.Errorf("multiple targets configured for %q; use --from <path> to specify which target to collect from", name)
		}
		return found, found.Targets[0], nil
	}
	for _, t := range found.Targets {
		if extraTargetPathMatches(mode, cwd, t.Path, fromPath) {
			return found, t, nil
		}
	}
	return found, config.ExtraTargetConfig{Path: fromPath}, nil
}

func runCollect(sourceDir, targetPath, name, mode string, dryRun, force, flatten bool, scope, cfgPath string, start time.Time, projectRoot string) error {
	result, err := sync.CollectExtraFiles(sourceDir, targetPath, mode, dryRun, force, flatten, projectRoot)
	if err != nil {
		return err
	}

	for _, e := range result.Errors {
		ui.Warning("%s", e)
	}

	switch {
	case result.Collected > 0 && dryRun:
		ui.Done(ui.MarkNone, fmt.Sprintf("Would collect %s from %s into extras/%s/", plural(result.Collected, "file"), shortenPath(targetPath), name), 0)
	case result.Collected > 0:
		ui.Done(ui.MarkOK, fmt.Sprintf("Collected %s from %s into extras/%s/", plural(result.Collected, "file"), shortenPath(targetPath), name), time.Since(start))
	default:
		ui.Done(ui.MarkNone, "No local files to collect from "+shortenPath(targetPath), 0)
	}

	if result.Skipped > 0 {
		ui.Note(fmt.Sprintf("%s skipped (already synced or in the source)", plural(result.Skipped, "file")))
	}
	if dryRun {
		fmt.Println()
		ui.DryRun()
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
		"force":     force,
	}
	oplog.WriteWithLimit(cfgPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck

	return nil
}

func printExtrasCollectHelp() {
	fmt.Println(`Usage: skillshare extras collect <name> [options]

Collect local files from a target back into the extras source directory.
Files are copied to source and replaced with symlinks in the target
(copy-mode targets keep their files).

Arguments:
  name                Name of the extra to collect for

Options:
  --from <path>       Target directory to collect from (required if multiple targets)
  --force, -f         Overwrite files that already exist in source
  --dry-run           Show what would be collected without making changes
  --project, -p       Use project mode (.skillshare/)
  --global, -g        Use global mode (~/.config/skillshare/)
  --help, -h          Show this help

Examples:
  skillshare extras collect rules
  skillshare extras collect rules --from ~/.claude/rules --dry-run
  skillshare extras collect rules --force
  skillshare extras collect prompts -p`)
}
