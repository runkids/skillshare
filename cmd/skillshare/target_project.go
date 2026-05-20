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
	"skillshare/internal/targetsummary"
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

	knownPath := ""
	if known, ok := config.LookupProjectTarget(name); ok {
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
		entry.Skills = &config.ResourceTargetConfig{Path: path}
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

	sourcePath := cfg.EffectiveSkillsSource(root)

	if opts.dryRun {
		return targetRemoveProjectDryRun(toRemove, targets, sourcePath)
	}

	for _, name := range toRemove {
		if target, ok := targets[name]; ok {
			sc := target.SkillsConfig()
			mode := sc.Mode
			if mode == "" {
				mode = "merge"
			}
			if mode == "symlink" {
				if err := unlinkSymlinkMode(sc.Path, sourcePath); err != nil {
					ui.Warning("%s: %v", name, err)
				}
			} else {
				// Remove copy-mode manifest if present
				sync.RemoveManifest(sc.Path)
				if err := unlinkMergeModeSafe(sc.Path, sourcePath); err != nil {
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

		sc := target.SkillsConfig()
		info, err := os.Lstat(sc.Path)
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
	return targetListProjectWithJSON(root, false)
}

func targetListProjectWithJSON(root string, jsonOutput bool) error {
	if !projectConfigExists(root) {
		if err := performProjectInit(root, projectInitOptions{}); err != nil {
			return err
		}
	}

	if jsonOutput {
		items, err := buildTargetTUIItems(true, root)
		if err != nil {
			return err
		}

		outputItems := make([]targetListJSONItem, 0, len(items))
		for _, item := range items {
			outputItems = append(outputItems, newTargetListJSONItem(item))
		}
		output := struct {
			Targets []targetListJSONItem `json:"targets"`
		}{Targets: outputItems}
		return writeJSON(&output)
	}

	items, err := buildTargetTUIItems(true, root)
	if err != nil {
		return err
	}

	ui.Header("Configured Targets (project)")
	printTargetListPlain(items)

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
	settings, err := parseTargetSettingFlags(remaining)
	if err != nil {
		return err
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
		var changes []string
		mutated := false

		if filterOpts.Skills.hasUpdates() {
			s := entry.EnsureSkills()
			skillChanges, fErr := applyFilterUpdates(&s.Include, &s.Exclude, filterOpts.Skills)
			if fErr != nil {
				return fErr
			}
			changes = append(changes, skillChanges...)
			mutated = true
		}

		if filterOpts.Agents.hasUpdates() {
			agentBuilder, buildErr := targetsummary.NewProjectBuilder(cfg.EffectiveAgentsSource(root), root)
			if buildErr != nil {
				return buildErr
			}
			agentSummary, buildErr := agentBuilder.ProjectTarget(*entry)
			if buildErr != nil {
				return buildErr
			}
			if agentSummary == nil {
				return fmt.Errorf("target '%s' does not have an agents path", name)
			}
			if agentSummary.Mode == "symlink" {
				return fmt.Errorf("target '%s' agent include/exclude filters are ignored in symlink mode; use --agent-mode merge or --agent-mode copy first", name)
			}

			ac := entry.AgentsConfig()
			include := append([]string(nil), ac.Include...)
			exclude := append([]string(nil), ac.Exclude...)
			agentChanges, fErr := applyFilterUpdates(&include, &exclude, filterOpts.Agents)
			if fErr != nil {
				return fErr
			}
			if len(agentChanges) > 0 {
				a := entry.EnsureAgents()
				a.Include = include
				a.Exclude = exclude
				mutated = true
			}
			changes = append(changes, scopeFilterChanges("agents", agentChanges)...)
		}

		if mutated {
			if err := cfg.Save(root); err != nil {
				return err
			}
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
		oplog.WriteWithLimit(config.ProjectConfigPath(root), oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck
		return nil
	}

	if settings.SkillMode != "" {
		return updateTargetModeProject(cfg, targetIdx, settings.SkillMode, root)
	}

	if settings.AgentMode != "" {
		return updateTargetAgentModeProject(cfg, targetIdx, settings.AgentMode, root)
	}

	if settings.Naming != "" {
		return updateTargetNamingProject(cfg, targetIdx, settings.Naming, root)
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
	sourcePath := cfg.EffectiveSkillsSource(root)
	agentBuilder, err := targetsummary.NewProjectBuilder(cfg.EffectiveAgentsSource(root), root)
	if err != nil {
		return err
	}
	agentSummary, err := agentBuilder.ProjectTarget(targetEntry)
	if err != nil {
		return err
	}

	sc := targetEntry.SkillsConfig()
	mode := sc.Mode
	displayMode := mode
	if mode == "" {
		displayMode = "merge (default)"
		mode = "merge"
	}

	namingDisplay := config.EffectiveTargetNaming(sc.TargetNaming)
	if sc.TargetNaming == "" {
		namingDisplay += " (default)"
	}

	ui.Header(fmt.Sprintf("Target: %s", name))
	fmt.Printf("  Path:    %s\n", projectTargetDisplayPath(targetEntry))
	fmt.Printf("  Mode:    %s\n", displayMode)
	fmt.Printf("  Naming:  %s\n", namingDisplay)

	resolvedSC := target.SkillsConfig()
	switch mode {
	case "symlink":
		status := sync.CheckStatus(resolvedSC.Path, sourcePath)
		fmt.Printf("  Status:  %s\n", status)
	case "copy":
		status, managed, local := sync.CheckStatusCopy(resolvedSC.Path)
		fmt.Printf("  Status:  %s (%d managed, %d local)\n", status, managed, local)
	default:
		status, linked, local := sync.CheckStatusMerge(resolvedSC.Path, sourcePath)
		fmt.Printf("  Status:  %s (%d shared, %d local)\n", status, linked, local)
	}

	fmt.Printf("  Include: %s\n", formatFilterList(sc.Include))
	fmt.Printf("  Exclude: %s\n", formatFilterList(sc.Exclude))
	printTargetAgentSection(agentSummary)

	return nil
}

func updateTargetModeProject(cfg *config.ProjectConfig, idx int, newMode string, root string) error {
	if newMode != "merge" && newMode != "symlink" && newMode != "copy" {
		return fmt.Errorf("invalid mode '%s'. Use 'merge', 'symlink', or 'copy'", newMode)
	}

	entry := &cfg.Targets[idx]
	oldMode := entry.SkillsConfig().Mode
	if oldMode == "" {
		oldMode = "merge"
	}

	entry.EnsureSkills().Mode = newMode
	if err := cfg.Save(root); err != nil {
		return err
	}

	ui.Success("Changed %s mode: %s -> %s", entry.Name, oldMode, newMode)
	ui.Info("Run 'skillshare sync' to apply the new mode")
	return nil
}

func updateTargetAgentModeProject(cfg *config.ProjectConfig, idx int, newMode string, root string) error {
	if newMode != "merge" && newMode != "symlink" && newMode != "copy" {
		return fmt.Errorf("invalid agent mode '%s'. Use 'merge', 'symlink', or 'copy'", newMode)
	}

	entry := &cfg.Targets[idx]
	agentBuilder, err := targetsummary.NewProjectBuilder(cfg.EffectiveAgentsSource(root), root)
	if err != nil {
		return err
	}
	agentSummary, err := agentBuilder.ProjectTarget(*entry)
	if err != nil {
		return err
	}
	if agentSummary == nil {
		return fmt.Errorf("target '%s' does not have an agents path", entry.Name)
	}

	oldMode := agentSummary.Mode
	entry.EnsureAgents().Mode = newMode
	if err := cfg.Save(root); err != nil {
		return err
	}

	ui.Success("Changed %s agent mode: %s -> %s", entry.Name, oldMode, newMode)
	if newMode == "symlink" && (len(agentSummary.Include) > 0 || len(agentSummary.Exclude) > 0) {
		ui.Warning("Agent include/exclude filters are ignored in symlink mode")
	}
	ui.Info("Run 'skillshare sync' to apply the new mode")
	return nil
}

func updateTargetNamingProject(cfg *config.ProjectConfig, idx int, newNaming string, root string) error {
	if !config.IsValidTargetNaming(newNaming) {
		return fmt.Errorf("invalid target naming '%s'. Use 'flat' or 'standard'", newNaming)
	}

	entry := &cfg.Targets[idx]
	oldNaming := config.EffectiveTargetNaming(entry.SkillsConfig().TargetNaming)

	entry.EnsureSkills().TargetNaming = newNaming
	if err := cfg.Save(root); err != nil {
		return err
	}

	ui.Success("Changed %s target naming: %s -> %s", entry.Name, oldNaming, newNaming)
	ui.Info("Run 'skillshare sync' to apply the new naming")
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
