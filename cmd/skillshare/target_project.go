package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
	"skillshare/internal/validate"
)

func targetAddProject(args []string, root string) error {
	if len(args) < 1 || len(args) > 2 {
		return fmt.Errorf("usage: skillshare target add <name> [path]")
	}

	name := args[0]
	path := ""
	if len(args) == 2 {
		path = args[1]
	}

	if err := validate.TargetName(name); err != nil {
		return fmt.Errorf("invalid target name: %w", err)
	}

	if !projectConfigExists(root) {
		if err := performProjectInit(root, projectInitOptions{}); err != nil {
			return err
		}
	}

	projectTargets := config.ProjectTargets()
	knownPath := ""
	if known, ok := projectTargets[name]; ok {
		knownPath = filepath.ToSlash(known.Path)
	}

	if path == "" {
		if knownPath == "" {
			return fmt.Errorf("usage: skillshare target add <name> <path>")
		}
		path = knownPath
	}

	if utils.HasTildePrefix(path) {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("cannot expand path: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}

	path = filepath.ToSlash(path)

	if err := validate.Path(path); err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	absPath := path
	if !filepath.IsAbs(path) {
		absPath = filepath.Join(root, filepath.FromSlash(path))
	}

	if !validate.IsLikelySkillsPath(absPath) {
		ui.Warning("Path doesn't appear to be a skills directory")
		fmt.Print("  Continue anyway? [y/N]: ")
		var input string
		fmt.Scanln(&input)
		input = strings.ToLower(strings.TrimSpace(input))
		if input != "y" && input != "yes" {
			ui.Info("Cancelled")
			return nil
		}
	}

	cfg, err := config.LoadProject(root)
	if err != nil {
		return err
	}

	for _, entry := range cfg.Targets {
		if entry.Name == name {
			return fmt.Errorf("target '%s' already exists", name)
		}
	}

	entry := config.ProjectTargetEntry{Name: name}
	if pathProvidedRequiresStorage(path, knownPath) {
		entry.Path = path
	}

	cfg.Targets = append(cfg.Targets, entry)
	if err := cfg.Save(root); err != nil {
		return err
	}

	if err := os.MkdirAll(absPath, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	ui.Success("Added target: %s -> %s", name, path)
	ui.Info("Run 'skillshare sync' to sync skills to this target")
	return nil
}

func pathProvidedRequiresStorage(path, knownPath string) bool {
	if path == "" {
		return false
	}
	if knownPath == "" {
		return true
	}
	normalized := strings.TrimSuffix(filepath.ToSlash(path), "/")
	known := strings.TrimSuffix(knownPath, "/")
	return normalized != known
}

func targetRemoveProject(args []string, root string) error {
	opts, err := parseTargetRemoveArgs(args)
	if err != nil {
		return err
	}

	if !projectConfigExists(root) {
		if err := performProjectInit(root, projectInitOptions{}); err != nil {
			return err
		}
	}

	cfg, err := config.LoadProject(root)
	if err != nil {
		return err
	}

	toRemove, err := resolveProjectTargetsToRemove(cfg, opts)
	if err != nil {
		return err
	}

	targets, err := config.ResolveProjectTargets(root, cfg)
	if err != nil {
		return err
	}

	sourcePath := filepath.Join(root, ".skillshare", "skills")

	if opts.dryRun {
		return targetRemoveProjectDryRun(toRemove, targets, sourcePath)
	}

	for _, name := range toRemove {
		if target, ok := targets[name]; ok {
			mode := target.Mode
			if mode == "" {
				mode = "merge"
			}
			if mode == "symlink" {
				if err := unlinkSymlinkMode(target.Path, sourcePath); err != nil {
					ui.Warning("%s: %v", name, err)
				}
			} else {
				if err := unlinkMergeModeSafe(target.Path, sourcePath); err != nil {
					ui.Warning("%s: %v", name, err)
				}
			}
		}
	}

	cfg.Targets = filterProjectTargets(cfg.Targets, toRemove)
	if err := cfg.Save(root); err != nil {
		return err
	}

	for _, name := range toRemove {
		ui.Success("Removed target: %s", name)
	}
	ui.Info("Run 'skillshare sync' to update target links")
	return nil
}

func targetRemoveProjectDryRun(toRemove []string, targets map[string]config.TargetConfig, sourcePath string) error {
	ui.Warning("Dry run mode - no changes will be made")
	ui.Header("Unlinking targets (project)")
	for _, name := range toRemove {
		target, ok := targets[name]
		if !ok {
			ui.Info("%s: would remove from config (target missing)", name)
			continue
		}

		info, err := os.Lstat(target.Path)
		if err != nil {
			if os.IsNotExist(err) {
				ui.Info("%s: would remove from config (path missing)", name)
				continue
			}
			ui.Warning("%s: %v", name, err)
			continue
		}

		if info.IsDir() {
			ui.Info("%s: would remove skill symlinks", name)
		}
		ui.Info("%s: would remove from config", name)
	}

	return nil
}

func resolveProjectTargetsToRemove(cfg *config.ProjectConfig, opts *targetRemoveOptions) ([]string, error) {
	if opts.removeAll {
		var toRemove []string
		for _, entry := range cfg.Targets {
			toRemove = append(toRemove, entry.Name)
		}
		return toRemove, nil
	}

	for _, entry := range cfg.Targets {
		if entry.Name == opts.name {
			return []string{opts.name}, nil
		}
	}
	return nil, fmt.Errorf("target '%s' not found", opts.name)
}

func filterProjectTargets(targets []config.ProjectTargetEntry, remove []string) []config.ProjectTargetEntry {
	if len(remove) == 0 {
		return targets
	}

	removeSet := map[string]bool{}
	for _, name := range remove {
		removeSet[name] = true
	}

	filtered := make([]config.ProjectTargetEntry, 0, len(targets))
	for _, target := range targets {
		if !removeSet[target.Name] {
			filtered = append(filtered, target)
		}
	}
	return filtered
}

func targetListProject(root string) error {
	if !projectConfigExists(root) {
		if err := performProjectInit(root, projectInitOptions{}); err != nil {
			return err
		}
	}

	cfg, err := config.LoadProject(root)
	if err != nil {
		return err
	}

	targets := make([]config.ProjectTargetEntry, len(cfg.Targets))
	copy(targets, cfg.Targets)
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].Name < targets[j].Name
	})

	ui.Header("Configured Targets (project)")
	for _, entry := range targets {
		displayPath := projectTargetDisplayPath(entry)
		mode := entry.Mode
		if mode == "" {
			mode = "merge"
		}
		fmt.Printf("  %-12s %s (%s)\n", entry.Name, displayPath, mode)
	}

	return nil
}

func targetInfoProject(name string, args []string, root string) error {
	if !projectConfigExists(root) {
		if err := performProjectInit(root, projectInitOptions{}); err != nil {
			return err
		}
	}

	// Parse filter flags first, pass remaining to mode parsing
	filterOpts, remaining, err := parseFilterFlags(args)
	if err != nil {
		return err
	}

	var newMode string
	for i := 0; i < len(remaining); i++ {
		switch remaining[i] {
		case "--mode", "-m":
			if i+1 >= len(remaining) {
				return fmt.Errorf("--mode requires a value (merge or symlink)")
			}
			newMode = remaining[i+1]
			i++
		}
	}

	cfg, err := config.LoadProject(root)
	if err != nil {
		return err
	}

	var targetIdx int = -1
	for i := range cfg.Targets {
		if cfg.Targets[i].Name == name {
			targetIdx = i
			break
		}
	}

	if targetIdx < 0 {
		return fmt.Errorf("target '%s' not found. Use 'skillshare target list' to see available targets", name)
	}

	// Apply filter updates if any
	if filterOpts.hasUpdates() {
		start := time.Now()
		entry := &cfg.Targets[targetIdx]
		changes, fErr := applyFilterUpdates(&entry.Include, &entry.Exclude, filterOpts)
		if fErr != nil {
			return fErr
		}
		if err := cfg.Save(root); err != nil {
			return err
		}
		for _, change := range changes {
			ui.Success("%s: %s", name, change)
		}
		if len(changes) > 0 {
			ui.Info("Run 'skillshare sync' to apply filter changes")
		}

		e := oplog.NewEntry("target", statusFromErr(nil), time.Since(start))
		e.Args = map[string]any{
			"action":  "filter",
			"name":    name,
			"changes": changes,
		}
		oplog.Write(config.ProjectConfigPath(root), oplog.OpsFile, e) //nolint:errcheck
		return nil
	}

	if newMode != "" {
		return updateTargetModeProject(cfg, targetIdx, newMode, root)
	}

	targets, err := config.ResolveProjectTargets(root, cfg)
	if err != nil {
		return err
	}

	target, ok := targets[name]
	if !ok {
		return fmt.Errorf("target '%s' not resolved", name)
	}

	targetEntry := cfg.Targets[targetIdx]
	sourcePath := filepath.Join(root, ".skillshare", "skills")

	mode := targetEntry.Mode
	displayMode := mode
	if mode == "" {
		displayMode = "merge (default)"
		mode = "merge"
	}

	ui.Header(fmt.Sprintf("Target: %s", name))
	fmt.Printf("  Path:    %s\n", projectTargetDisplayPath(targetEntry))
	fmt.Printf("  Mode:    %s\n", displayMode)

	if mode == "symlink" {
		status := sync.CheckStatus(target.Path, sourcePath)
		fmt.Printf("  Status:  %s\n", status)
	} else {
		status, linked, local := sync.CheckStatusMerge(target.Path, sourcePath)
		fmt.Printf("  Status:  %s (%d shared, %d local)\n", status, linked, local)
	}

	fmt.Printf("  Include: %s\n", formatFilterList(targetEntry.Include))
	fmt.Printf("  Exclude: %s\n", formatFilterList(targetEntry.Exclude))

	return nil
}

func updateTargetModeProject(cfg *config.ProjectConfig, idx int, newMode string, root string) error {
	if newMode != "merge" && newMode != "symlink" {
		return fmt.Errorf("invalid mode '%s'. Use 'merge' or 'symlink'", newMode)
	}

	entry := &cfg.Targets[idx]
	oldMode := entry.Mode
	if oldMode == "" {
		oldMode = "merge"
	}

	entry.Mode = newMode
	if err := cfg.Save(root); err != nil {
		return err
	}

	ui.Success("Changed %s mode: %s -> %s", entry.Name, oldMode, newMode)
	ui.Info("Run 'skillshare sync' to apply the new mode")
	return nil
}

func unlinkMergeModeSafe(targetPath, sourcePath string) error {
	info, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}
	return unlinkMergeMode(targetPath, sourcePath)
}
