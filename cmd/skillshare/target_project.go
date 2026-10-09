package main

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/sync"
	"skillshare/internal/targetsummary"
	"skillshare/internal/theme"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
	"skillshare/internal/validate"
)

func targetAddProject(args []string, root string) error {
	args, noSkills := stripNoSkillsFlag(args)
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

	if err := ensureProjectConfig(root); err != nil {
		return err
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
		ok, err := ui.ConfirmAction("Continue anyway?", false)
		if err != nil {
			return err
		}
		if !ok {
			ui.Cancelled("added")
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
	if noSkills {
		entry.EnsureSkills().SetEnabled(false)
	}
	if mode := config.NewTargetSkillsMode(cfg.TargetNaming, ""); mode != "" {
		entry.EnsureSkills().Mode = mode // a project target defaults to merge
	}

	cfg.Targets = append(cfg.Targets, entry)
	if err := cfg.Save(root); err != nil {
		return err
	}

	if !noSkills {
		if err := os.MkdirAll(absPath, 0755); err != nil {
			return fmt.Errorf("failed to create target directory: %w", err)
		}
	}

	reportTargetAdded(name, path, noSkills)
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

	if err := ensureProjectConfig(root); err != nil {
		return err
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
		if target, ok := targets[name]; ok && target.SkillsConfig().IsEnabled() {
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

	ui.Done(ui.MarkOK, "Removed "+plural(len(toRemove), "target"), 0)
	ui.Next("skillshare sync", "update target links")
	return nil
}

func targetRemoveProjectDryRun(toRemove []string, targets map[string]config.TargetConfig, sourcePath string) error {
	width := ui.RowWidth(toRemove...)
	for _, name := range toRemove {
		target, ok := targets[name]
		if !ok {
			ui.Row(ui.MarkNone, name, "would remove from config · target missing", width)
			continue
		}

		sc := target.SkillsConfig()
		if !sc.IsEnabled() {
			ui.Row(ui.MarkNone, name, "would remove from config · skills off, folder left as is", width)
			continue
		}
		info, err := os.Lstat(sc.Path)
		if err != nil {
			if os.IsNotExist(err) {
				ui.Row(ui.MarkNone, name, "would remove from config · folder not found", width)
				continue
			}
			ui.Row(ui.MarkWarn, name, err.Error(), width)
			continue
		}

		if info.IsDir() {
			ui.Row(ui.MarkNone, name, "would remove skill symlinks and remove from config", width)
		} else {
			ui.Row(ui.MarkNone, name, "would remove from config", width)
		}
	}

	fmt.Println()
	ui.DryRun()
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
	if err := ensureProjectConfig(root); err != nil {
		return err
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

	printTargetListPlain(items)

	return nil
}

func targetInfoProject(name string, args []string, root string) error {
	if err := ensureProjectConfig(root); err != nil {
		return err
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
	if err := checkTargetFlagCombination(settings, filterOpts); err != nil {
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
		printTargetFilterChanges(name, changes)

		e := oplog.NewEntry("target", statusFromErr(nil), time.Since(start))
		e.Args = map[string]any{
			"action":  "filter",
			"name":    name,
			"changes": changes,
		}
		oplog.WriteWithLimit(config.ProjectConfigPath(root), oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck
		return nil
	}

	if settings.Skills != nil {
		return setTargetSkillsProject(cfg, targetIdx, *settings.Skills, settings.DryRun, root)
	}

	if settings.changesTarget() {
		return updateTargetSettingsProject(cfg, targetIdx, settings, root)
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
		displayMode = "merge" + ui.DimText(" (default)")
		mode = "merge"
	}

	namingDisplay := config.EffectiveTargetNaming(sc.TargetNaming)
	if sc.TargetNaming == "" {
		namingDisplay += ui.DimText(" (default)")
	}

	var statusLine string
	resolvedSC := target.SkillsConfig()
	switch {
	case !resolvedSC.IsEnabled():
		statusLine = skillsOffSummary
	case mode == "symlink":
		statusLine = sync.CheckStatus(resolvedSC.Path, sourcePath).String()
	case mode == "copy":
		status, managed, local := sync.CheckStatusCopy(resolvedSC.Path)
		statusLine = statusWithCounts(status, managed, "managed", local)
	default:
		status, linked, local := sync.CheckStatusMerge(resolvedSC.Path, sourcePath)
		statusLine = statusWithCounts(status, linked, "linked", local)
	}

	fmt.Println(theme.Primary().Bold(true).Render(name))
	printTargetSkillsSection(projectTargetDisplayPath(targetEntry), displayMode, namingDisplay, statusLine, sc.Include, sc.Exclude)
	printTargetAgentSection(agentSummary)

	return nil
}

func updateTargetSettingsProject(cfg *config.ProjectConfig, idx int, settings parsedTargetSettingFlags, root string) error {
	entry := &cfg.Targets[idx]
	sc := entry.SkillsConfig()
	u := targetSettingsUpdate{name: entry.Name, oldMode: cmp.Or(sc.Mode, "merge"), oldNaming: sc.TargetNaming,
		ensureSkills: entry.EnsureSkills, ensureAgents: entry.EnsureAgents, save: func() error { return cfg.Save(root) }}
	if settings.AgentMode != "" {
		agentBuilder, err := targetsummary.NewProjectBuilder(cfg.EffectiveAgentsSource(root), root)
		if err != nil {
			return err
		}
		if u.agent, err = agentBuilder.ProjectTarget(*entry); err != nil {
			return err
		}
	}
	return u.apply(settings)
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
