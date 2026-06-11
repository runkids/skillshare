package main

import (
	"fmt"
	"os"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
)

// cmdExtrasRemoveTarget handles `skillshare extras <name> --remove-target <path>
// [--prune]`. It removes one target from an existing extra. By default only the
// config entry is removed (synced files are left on disk); --prune additionally
// deletes the skillshare-managed files under that target.
func cmdExtrasRemoveTarget(args []string) error {
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

	var name, rmPath string
	var prune bool
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--remove-target":
			if i+1 >= len(rest) {
				return fmt.Errorf("--remove-target requires a path argument")
			}
			i++
			rmPath = rest[i]
		case "--prune":
			prune = true
		case "--help", "-h":
			printExtrasRemoveTargetHelp()
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
		return fmt.Errorf("extras name is required: skillshare extras <name> --remove-target <path>")
	}
	if rmPath == "" {
		return fmt.Errorf("--remove-target requires a path argument")
	}

	var extras []config.ExtraConfig
	var configPath string
	var saveFn func() error
	var sourceDirForExtra func(config.ExtraConfig) string
	var extensionsDir string
	if mode == modeProject {
		projCfg, loadErr := config.LoadProject(cwd)
		if loadErr != nil {
			return loadErr
		}
		extras = projCfg.Extras
		configPath = config.ProjectConfigPath(cwd)
		saveFn = func() error { return projCfg.Save(cwd) }
		projectExtrasSource := projCfg.EffectiveExtrasSource(cwd)
		sourceDirForExtra = func(extra config.ExtraConfig) string {
			return config.ExtrasSourceDirProject(projectExtrasSource, extra.Name)
		}
		extensionsDir = projectExtensionsDir(cwd)
	} else {
		cfg, loadErr := config.Load()
		if loadErr != nil {
			return loadErr
		}
		extras = cfg.Extras
		configPath = config.ConfigPath()
		saveFn = cfg.Save
		extrasSource := cfg.EffectiveExtrasSource()
		skillsSource := cfg.EffectiveSkillsSource()
		sourceDirForExtra = func(extra config.ExtraConfig) string {
			return config.ResolveExtrasSourceDir(extra, extrasSource, skillsSource)
		}
		extensionsDir = globalExtensionsDir()
	}

	idx := -1
	for i := range extras {
		if extras[i].Name == name {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("extra %q not found", name)
	}

	tIdx := -1
	var targetMode string
	var targetExtension string
	var target config.ExtraTargetConfig
	for j, t := range extras[idx].Targets {
		if extraTargetPathMatches(mode, cwd, t.Path, rmPath) {
			tIdx = j
			targetMode = t.Mode
			targetExtension = t.Extension
			target = t
			break
		}
	}
	if tIdx == -1 {
		return fmt.Errorf("target %q not found in extra %q", rmPath, name)
	}
	if len(extras[idx].Targets) == 1 {
		return fmt.Errorf("%q is the last target of %q — use 'skillshare extras remove %s' to remove the whole extra", rmPath, name, name)
	}
	if prune && targetExtension != "" {
		effectiveMode, modeErr := validateExtensionMode(targetMode)
		if modeErr != nil {
			return fmt.Errorf("target %q: %w", rmPath, modeErr)
		}
		targetMode = effectiveMode
	}

	// Resolve the on-disk target path for optional pruning before mutating config.
	resolved := canonicalExtraTargetPath(mode, cwd, target.Path)

	var pruned int
	if prune {
		var managedFiles map[string]bool
		if targetMode == "copy" {
			var managedErr error
			managedFiles, managedErr = managedExtraTargetFiles(target, sourceDirForExtra(extras[idx]), extensionsDir)
			if managedErr != nil {
				return managedErr
			}
		}

		var errs []string
		pruned, errs = sync.PruneExtraTargetFiles(resolved, targetMode, managedFiles)
		if len(errs) > 0 {
			for _, msg := range errs {
				ui.Warning("%s", msg)
			}
			return fmt.Errorf("failed to prune target %s", shortenPath(rmPath))
		}
	}

	extras[idx].Targets = append(extras[idx].Targets[:tIdx], extras[idx].Targets[tIdx+1:]...)

	if err := saveFn(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	ui.Success("Removed target %s from %s", shortenPath(rmPath), name)

	if prune {
		ui.Info("Pruned %d file(s) from %s", pruned, shortenPath(rmPath))
	} else {
		ui.Info("Synced files left in place. Run 'skillshare sync extras%s' to clean up orphaned links, or re-run with --prune.", projectSuffix(mode))
	}

	e := oplog.NewEntry("extras-target", "ok", time.Since(start))
	e.Args = map[string]any{"name": name, "target": rmPath, "action": "remove", "prune": prune}
	oplog.WriteWithLimit(configPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck

	return nil
}

func managedExtraTargetFiles(target config.ExtraTargetConfig, sourceDir, extensionsDir string) (map[string]bool, error) {
	files, err := sync.DiscoverExtraFiles(sourceDir)
	if err != nil {
		return nil, err
	}

	outputExt := ""
	if target.Extension != "" {
		spec, err := resolveExtension(target.Extension, extensionsDir)
		if err != nil {
			return nil, err
		}
		if spec != nil {
			outputExt = spec.OutputExt
		}
	}

	seen := make(map[string]bool)
	managed := make(map[string]bool)
	for _, rel := range files {
		tgtRel, ok := sync.FlattenRel(rel, target.Flatten, seen)
		if !ok {
			continue
		}
		managed[sync.ApplyOutputExt(tgtRel, outputExt)] = true
	}
	return managed, nil
}

func printExtrasRemoveTargetHelp() {
	fmt.Println(`Usage: skillshare extras <name> --remove-target <path> [--prune]

Remove one target from an existing extra. By default only the config entry is
removed; synced files are left on disk. Use --prune to also delete the
skillshare-managed files under that target.

Options:
  --remove-target <path>  Target directory to remove (required)
  --prune                 Also delete skillshare-managed files under that target
  --project, -p           Use project mode (.skillshare/)
  --global, -g            Use global mode (~/.config/skillshare/)
  --help, -h              Show this help

Examples:
  skillshare extras rules --remove-target ~/.cursor/rules
  skillshare extras rules --remove-target ~/.cursor/rules --prune`)
}
