package main

import (
	"fmt"
	"os"
	"path/filepath"

	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
)

func cmdDiff(args []string) error {
	var targetName string
	for i := 0; i < len(args); i++ {
		if args[i] == "--target" || args[i] == "-t" {
			if i+1 < len(args) {
				targetName = args[i+1]
				i++
			}
		} else {
			targetName = args[i]
		}
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Use same discovery as sync to get proper skill names (including tracked repo skills)
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
		sourceSkills := make(map[string]bool, len(filtered))
		for _, skill := range filtered {
			sourceSkills[skill.FlatName] = true
		}
		showTargetDiff(name, target, cfg.Source, sourceSkills)
	}

	return nil
}

func showTargetDiff(name string, target config.TargetConfig, source string, sourceSkills map[string]bool) {
	ui.Header(name)

	// Check if target is a symlink (symlink mode)
	_, err := os.Lstat(target.Path)
	if err != nil {
		ui.Warning("Cannot access target: %v", err)
		return
	}

	if utils.IsSymlinkOrJunction(target.Path) {
		showSymlinkDiff(target.Path, source)
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
