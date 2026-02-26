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
	"skillshare/internal/utils"
)

func cmdDiff(args []string) error {
	start := time.Now()

	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}

	if mode == modeAuto {
		if projectConfigExists(cwd) {
			mode = modeProject
		} else {
			mode = modeGlobal
		}
	}

	applyModeLabel(mode)

	scope := "global"
	cfgPath := config.ConfigPath()
	if mode == modeProject {
		scope = "project"
		cfgPath = config.ProjectConfigPath(cwd)
	}

	var targetName string
	for i := 0; i < len(rest); i++ {
		if rest[i] == "--target" || rest[i] == "-t" {
			if i+1 < len(rest) {
				targetName = rest[i+1]
				i++
			}
		} else {
			targetName = rest[i]
		}
	}

	var cmdErr error
	if mode == modeProject {
		cmdErr = cmdDiffProject(cwd, targetName)
	} else {
		cmdErr = cmdDiffGlobal(targetName)
	}
	logDiffOp(cfgPath, targetName, scope, 0, start, cmdErr)
	return cmdErr
}

func logDiffOp(cfgPath string, targetName, scope string, targetsShown int, start time.Time, cmdErr error) {
	e := oplog.NewEntry("diff", statusFromErr(cmdErr), time.Since(start))
	a := map[string]any{"scope": scope}
	if targetName != "" {
		a["target"] = targetName
	}
	if targetsShown > 0 {
		a["targets_shown"] = targetsShown
	}
	e.Args = a
	if cmdErr != nil {
		e.Message = cmdErr.Error()
	}
	oplog.WriteWithLimit(cfgPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck
}

func cmdDiffGlobal(targetName string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	discovered, err := sync.DiscoverSourceSkills(cfg.Source)
	if err != nil {
		return fmt.Errorf("failed to discover skills: %w", err)
	}

	targets := cfg.Targets
	if targetName != "" {
		if t, exists := cfg.Targets[targetName]; exists {
			targets = map[string]config.TargetConfig{targetName: t}
		} else {
			return fmt.Errorf("target '%s' not found", targetName)
		}
	}

	for name, target := range targets {
		filtered, err := sync.FilterSkills(discovered, target.Include, target.Exclude)
		if err != nil {
			return fmt.Errorf("target %s has invalid include/exclude config: %w", name, err)
		}
		mode := target.Mode
		if mode == "" {
			mode = cfg.Mode
			if mode == "" {
				mode = "merge"
			}
		}
		showTargetDiff(name, target, cfg.Source, mode, filtered)
	}

	return nil
}

func showTargetDiff(name string, target config.TargetConfig, source, mode string, filtered []sync.DiscoveredSkill) {
	ui.Header(name)

	if len(target.Include) > 0 {
		ui.Info("  include: %s", strings.Join(target.Include, ", "))
	}
	if len(target.Exclude) > 0 {
		ui.Info("  exclude: %s", strings.Join(target.Exclude, ", "))
	}

	// Check if target is a symlink (symlink mode)
	_, err := os.Lstat(target.Path)
	if err != nil {
		ui.Warning("Cannot access target: %v", err)
		return
	}

	sourceSkills := make(map[string]bool, len(filtered))
	for _, skill := range filtered {
		sourceSkills[skill.FlatName] = true
	}

	if utils.IsSymlinkOrJunction(target.Path) {
		showSymlinkDiff(target.Path, source)
		return
	}

	if mode == "copy" {
		manifest, _ := sync.ReadManifest(target.Path)
		showCopyDiff(name, target.Path, filtered, sourceSkills, manifest)
		return
	}

	// Merge mode - check individual skills
	showMergeDiff(name, target.Path, source, sourceSkills)
}

func showSymlinkDiff(targetPath, source string) {
	absLink, err := utils.ResolveLinkTarget(targetPath)
	if err != nil {
		ui.Warning("Unable to resolve symlink target: %v", err)
		return
	}
	absSource, _ := filepath.Abs(source)
	if utils.PathsEqual(absLink, absSource) {
		ui.Success("Fully synced (symlink mode)")
	} else {
		ui.Warning("Symlink points to different location: %s", absLink)
	}
}

func showCopyDiff(targetName, targetPath string, filtered []sync.DiscoveredSkill, sourceSkills map[string]bool, manifest *sync.Manifest) {
	var syncCount, localCount int

	// Check each source skill
	for _, skill := range filtered {
		oldChecksum, isManaged := manifest.Managed[skill.FlatName]
		targetSkillPath := filepath.Join(targetPath, skill.FlatName)
		if !isManaged {
			// Not in manifest — missing or local entry
			if info, err := os.Stat(targetSkillPath); err == nil {
				if info.IsDir() {
					ui.DiffItem("modify", skill.FlatName, "local copy (sync --force to replace)")
				} else {
					ui.DiffItem("modify", skill.FlatName, "target entry is not a directory")
				}
			} else if os.IsNotExist(err) {
				ui.DiffItem("add", skill.FlatName, "missing")
			} else {
				ui.DiffItem("modify", skill.FlatName, "cannot access target entry")
			}
			syncCount++
			continue
		}
		// Managed — verify target directory still exists
		targetInfo, err := os.Stat(targetSkillPath)
		if os.IsNotExist(err) {
			ui.DiffItem("add", skill.FlatName, "missing (deleted from target)")
			syncCount++
			continue
		}
		if err != nil {
			ui.DiffItem("modify", skill.FlatName, "cannot access target entry")
			syncCount++
			continue
		}
		if !targetInfo.IsDir() {
			ui.DiffItem("modify", skill.FlatName, "target entry is not a directory")
			syncCount++
			continue
		}
		// Compare checksums to detect content drift
		srcChecksum, err := sync.DirChecksum(skill.SourcePath)
		if err != nil {
			ui.DiffItem("modify", skill.FlatName, "cannot compute checksum")
			syncCount++
			continue
		}
		if srcChecksum != oldChecksum {
			ui.DiffItem("modify", skill.FlatName, "content changed")
			syncCount++
		}
	}

	// Managed copies no longer in source (orphans)
	for name := range manifest.Managed {
		if !sourceSkills[name] {
			ui.DiffItem("remove", name, "orphan (will be pruned)")
			syncCount++
		}
	}

	// Local directories not in source and not managed
	entries, _ := os.ReadDir(targetPath)
	for _, e := range entries {
		if utils.IsHidden(e.Name()) || !e.IsDir() {
			continue
		}
		if sourceSkills[e.Name()] {
			continue
		}
		if _, isManaged := manifest.Managed[e.Name()]; isManaged {
			continue
		}
		ui.DiffItem("remove", e.Name(), "local only")
		localCount++
	}

	if syncCount == 0 && localCount == 0 {
		ui.Success("Fully synced")
	} else {
		fmt.Println()
		if syncCount > 0 {
			ui.Info("Run 'sync' to copy missing, 'sync --force' to replace local copies")
		}
		if localCount > 0 {
			ui.Info("Run 'collect %s' to import local-only skills to source", targetName)
		}
	}
}

func showMergeDiff(targetName, targetPath, source string, sourceSkills map[string]bool) {
	targetSkills := make(map[string]bool)
	targetSymlinks := make(map[string]bool)
	entries, err := os.ReadDir(targetPath)
	if err != nil {
		ui.Warning("Cannot read target: %v", err)
		return
	}

	for _, e := range entries {
		if utils.IsHidden(e.Name()) {
			continue
		}
		skillPath := filepath.Join(targetPath, e.Name())
		if utils.IsSymlinkOrJunction(skillPath) {
			targetSymlinks[e.Name()] = true
		}
		targetSkills[e.Name()] = true
	}

	// Compare and count
	var syncCount, localCount int

	// Skills only in source (not synced)
	for skill := range sourceSkills {
		if !targetSkills[skill] {
			ui.DiffItem("add", skill, "missing")
			syncCount++
		} else if !targetSymlinks[skill] {
			ui.DiffItem("modify", skill, "local copy (sync --force to replace)")
			syncCount++
		}
	}

	// Skills only in target (local only)
	for skill := range targetSkills {
		if !sourceSkills[skill] && !targetSymlinks[skill] {
			ui.DiffItem("remove", skill, "local only")
			localCount++
		}
	}

	// Show action hints
	if syncCount == 0 && localCount == 0 {
		ui.Success("Fully synced")
	} else {
		fmt.Println()
		if syncCount > 0 {
			ui.Info("Run 'sync' to add missing, 'sync --force' to replace local copies")
		}
		if localCount > 0 {
			ui.Info("Run 'collect %s' to import local-only skills to source", targetName)
		}
	}
}
