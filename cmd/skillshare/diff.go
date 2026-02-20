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

func cmdDiff(args []string) error {
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

	if mode == modeProject {
		return cmdDiffProject(cwd, targetName)
	}
	return cmdDiffGlobal(targetName)
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

	// Build source path lookup for checksum comparison
	sourcePathMap := make(map[string]string, len(filtered))
	for _, skill := range filtered {
		sourcePathMap[skill.FlatName] = skill.SourcePath
	}

	// Check each source skill
	for _, skill := range filtered {
		oldChecksum, isManaged := manifest.Managed[skill.FlatName]
		if !isManaged {
			// Not in manifest — missing or local copy
			if _, err := os.Stat(filepath.Join(targetPath, skill.FlatName)); err == nil {
				ui.DiffItem("modify", skill.FlatName, "local copy (sync --force to replace)")
			} else {
				ui.DiffItem("add", skill.FlatName, "missing")
			}
			syncCount++
			continue
		}
		// Managed — compare checksums to detect content drift
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
			ui.Info("Run 'pull %s' to import local-only skills to source", targetName)
		}
	}
}
