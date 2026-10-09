package main

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/backup"
	"skillshare/internal/config"
	"skillshare/internal/hooks"
	"skillshare/internal/mcp"
	"skillshare/internal/oplog"
	"skillshare/internal/sync"
	"skillshare/internal/targetsummary"
	"skillshare/internal/theme"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
	"skillshare/internal/validate"
)

func cmdTarget(args []string) error {
	mode, rest, err := parseModeArgs(args, "--agent", "--config-dir", "--cli", "--add-include", "--add-exclude", "--remove-include", "--remove-exclude", "--add-agent-include", "--add-agent-exclude", "--remove-agent-include", "--remove-agent-exclude", "--mode", "-m", "--agent-mode", "--target-naming", "--skills")
	if err != nil {
		return err
	}

	if len(rest) < 1 {
		return fmt.Errorf("usage: skillshare target <add|remove|list|name> [options]")
	}
	if wantsHelp(rest) {
		printTargetHelp()
		return nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}

	mode = resolveAutoMode(mode, cwd)

	applyModeLabel(mode)

	subcmd := rest[0]
	subargs := rest[1:]

	switch subcmd {
	case "help", "--help", "-h":
		printTargetHelp()
		return nil
	case "add":
		if mode == modeProject {
			return targetAddProject(subargs, cwd)
		}
		return targetAdd(subargs)
	case "remove", "rm":
		if mode == modeProject {
			return targetRemoveProject(subargs, cwd)
		}
		return targetRemove(subargs)
	case "list", "ls":
		jsonOutput := hasFlag(subargs, "--json")
		noTUI := hasFlag(subargs, "--no-tui")
		if jsonOutput {
			if mode == modeProject {
				return targetListProjectWithJSON(cwd, true)
			}
			return targetList(true)
		}
		if shouldLaunchTUI(noTUI, nil) {
			action, targetName, tuiErr := runTargetListTUI(mode, cwd)
			if tuiErr != nil {
				return tuiErr
			}
			if action == "remove" && targetName != "" {
				if mode == modeProject {
					return targetRemoveProject([]string{targetName}, cwd)
				}
				return targetRemove([]string{targetName})
			}
			return nil
		}
		if mode == modeProject {
			return targetListProjectWithJSON(cwd, false)
		}
		return targetList(false)
	default:
		// Assume it's a target name - show info or modify settings
		if mode == modeProject {
			return targetInfoProject(subcmd, subargs, cwd)
		}
		return targetInfo(subcmd, subargs)
	}
}

func printTargetHelp() {
	printHelp("skillshare target <add|remove|list|name> [options]", "Manage target skill directories.",
		helpGroup{title: "Commands", rows: []helpRow{
			{"add <name> [path]", "Add a target (path optional for known project targets)"},
			{"add <name> --agent <agent> --config-dir <dir> [--cli <executable>]", ""},
			{"", "Add another account of an Agent: its second config directory.\n--cli runs its plugin commands with a compatible CLI instead"},
			{"add <name> ... --no-skills", ""},
			{"", "Add a target without syncing skills to it"},
			{"remove <name>", "Remove a target"},
			{"remove --all", "Remove all targets"},
			{"list", "List configured targets"},
			{"<name>", "Show target info or modify settings"},
		}},
		helpGroup{title: "Options", rows: []helpRow{
			{"--json", "Output list as JSON"},
			{"--no-tui", "Skip interactive TUI, show plain text list"},
			{"-p, --project", "Use project-level config in current directory"},
			{"-g, --global", "Use global config (~/.config/skillshare)"},
		}},
		helpGroup{title: "Target settings", rows: []helpRow{
			{"<name> --mode <mode>", "Set sync mode (merge, symlink, or copy)"},
			{"<name> --agent-mode <mode>", "Set agents sync mode (merge, symlink, or copy)"},
			{"<name> --target-naming <naming>", "Set target naming (flat, standard, or prefixed; prefixed needs copy mode)"},
			{"<name> --skills=false [--dry-run]", "Stop syncing skills; removes only links into the source"},
			{"<name> --skills=true", "Sync skills again (on the next 'skillshare sync')"},
			{"<name> --add-include <pattern>", "Add an include filter pattern"},
			{"<name> --add-exclude <pattern>", "Add an exclude filter pattern"},
			{"<name> --remove-include <pattern>", "Remove an include filter pattern"},
			{"<name> --remove-exclude <pattern>", "Remove an exclude filter pattern"},
			{"<name> --add-agent-include <pattern>", "Add an agent include filter pattern"},
			{"<name> --add-agent-exclude <pattern>", "Add an agent exclude filter pattern"},
			{"<name> --remove-agent-include <pattern>", "Remove an agent include filter pattern"},
			{"<name> --remove-agent-exclude <pattern>", "Remove an agent exclude filter pattern"},
		}},
		helpExamples(
			helpRow{"skillshare target add cursor", ""},
			helpRow{"skillshare target add my-ide .my-ide/skills", ""},
			helpRow{"skillshare target add claude-work --agent claude --config-dir ~/.claude-work", ""},
			helpRow{"skillshare target add omo --agent pi --config-dir ~/.omo/agent --cli omo", ""},
			helpRow{"skillshare target remove cursor", ""},
			helpRow{"skillshare target list", ""},
			helpRow{"skillshare target cursor", ""},
			helpRow{"skillshare target claude --agent-mode copy", ""},
			helpRow{"skillshare target claude --mode copy --target-naming prefixed", ""},
			helpRow{"skillshare target claude --add-include \"team-*\"", ""},
			helpRow{"skillshare target claude --add-agent-include \"team-*\"", ""},
			helpRow{"skillshare target claude --remove-include \"team-*\"", ""},
			helpRow{"skillshare target claude --add-exclude \"_legacy*\"", ""},
			helpRow{"skillshare target gemini --skills=false", ""},
		),
		helpGroup{title: "Project mode", examples: true, rows: []helpRow{
			{"skillshare target add claude -p", ""},
			{"skillshare target add gemini --no-skills -p", ""},
			{"skillshare target claude --add-include \"team-*\" -p", ""},
			{"skillshare target list -p", ""},
		}},
	)
}

func targetAdd(args []string) error {
	args, noSkills := stripNoSkillsFlag(args)
	if len(args) < 2 {
		return fmt.Errorf("usage: skillshare target add <name> <path>\n       skillshare target add <name> --agent <agent> --config-dir <dir>")
	}

	name := args[0]
	path := args[1]

	// Validate target name
	if err := validate.TargetName(name); err != nil {
		return fmt.Errorf("invalid target name: %w", err)
	}
	if strings.HasPrefix(path, "--") {
		return targetAddAgentConfigDir(name, args[1:], noSkills)
	}

	// Expand ~
	if utils.HasTildePrefix(path) {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("cannot expand path: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}

	// Validate target path and get warnings. With skills off nothing is
	// written there, so the folder need not exist yet.
	var warnings []string
	var err error
	if noSkills {
		err = validate.Path(path)
	} else {
		warnings, err = validate.TargetPath(path)
	}
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Show warnings to user
	for _, w := range warnings {
		ui.Warning("%s", w)
	}

	// If path doesn't look like a skills directory, ask for confirmation
	if !validate.IsLikelySkillsPath(path) {
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

	cfg, err := config.LoadWithoutProjects()
	if err != nil {
		return err
	}

	if _, exists := cfg.Targets[name]; exists {
		return fmt.Errorf("target '%s' already exists", name)
	}

	skills := &config.ResourceTargetConfig{Path: path, Mode: config.NewTargetSkillsMode(cfg.TargetNaming, cfg.Mode)}
	skills.SetEnabled(!noSkills)
	cfg.Targets[name] = config.TargetConfig{Skills: skills}
	if err := cfg.Save(); err != nil {
		return err
	}

	reportTargetAdded(name, path, noSkills, skills.Mode != "")
	return nil
}

// reportTargetAdded prints the result of target add. copyForced is set when the target
// was given copy mode because the naming it inherits needs it.
func reportTargetAdded(name, path string, noSkills, copyForced bool) {
	if copyForced {
		ui.Info("Target naming \"prefixed\" needs copy mode, so %s was added with mode copy", name)
	}
	if noSkills {
		ui.Done(ui.MarkOK, fmt.Sprintf("Added target %s -> %s (skills off)", name, shortenPath(path)), 0)
		ui.Next(fmt.Sprintf("skillshare target %s --skills=true", name), "sync skills to this target too")
		return
	}
	ui.Done(ui.MarkOK, fmt.Sprintf("Added target %s -> %s", name, shortenPath(path)), 0)
	ui.Next("skillshare sync", "sync skills to this target")
}

// targetAddAgentConfigDir adds another config directory of a built-in Agent, such as a
// second account. Its skills and agents paths follow the directory.
func targetAddAgentConfigDir(name string, args []string, noSkills bool) error {
	var agent, dir, cli string
	for i := 0; i < len(args); i++ {
		if i+1 == len(args) || (args[i] != "--agent" && args[i] != "--config-dir" && args[i] != "--cli") {
			return fmt.Errorf("usage: skillshare target add <name> --agent <agent> --config-dir <dir> [--cli <executable>]")
		}
		switch args[i] {
		case "--agent":
			agent = args[i+1]
		case "--config-dir":
			dir = args[i+1]
		default:
			cli = args[i+1]
		}
		i++
	}
	cfg, err := config.LoadWithoutProjects()
	if err != nil {
		return err
	}
	path, err := cfg.AddAgentConfigDirTarget(name, agent, dir, cli)
	if err != nil {
		return err
	}
	if noSkills {
		tc := cfg.Targets[name]
		tc.EnsureSkills().SetEnabled(false)
		cfg.Targets[name] = tc
	}
	if err := cfg.Save(); err != nil {
		return err
	}
	target := cfg.Targets[name]
	reportTargetAdded(name, path, noSkills, target.SkillsConfig().Mode != "")
	return nil
}

// targetRemoveOptions holds parsed options for target remove
type targetRemoveOptions struct {
	name      string
	removeAll bool
	dryRun    bool
}

// parseTargetRemoveArgs parses target remove arguments
func parseTargetRemoveArgs(args []string) (*targetRemoveOptions, error) {
	opts := &targetRemoveOptions{}

	for _, arg := range args {
		switch arg {
		case "--all", "-a":
			opts.removeAll = true
		case "--dry-run", "-n":
			opts.dryRun = true
		default:
			opts.name = arg
		}
	}

	if !opts.removeAll && opts.name == "" {
		return nil, fmt.Errorf("usage: skillshare target remove <name> or --all")
	}

	return opts, nil
}

// resolveTargetsToRemove determines which targets to remove
func resolveTargetsToRemove(cfg *config.Config, opts *targetRemoveOptions) ([]string, error) {
	if opts.removeAll {
		var toRemove []string
		for n := range cfg.Targets {
			toRemove = append(toRemove, n)
		}
		return toRemove, nil
	}

	if _, exists := cfg.Targets[opts.name]; !exists {
		return nil, fmt.Errorf("target '%s' not found", opts.name)
	}
	return []string{opts.name}, nil
}

// backupTargets creates backups for targets before removal
func backupTargets(cfg *config.Config, toRemove []string, width int) {
	for _, targetName := range toRemove {
		target := cfg.Targets[targetName]
		if !target.SkillsConfig().IsEnabled() {
			continue
		}
		backupPath, err := backup.Create(targetName, target.SkillsConfig().Path)
		if err != nil {
			ui.Row(ui.MarkWarn, targetName, "backup failed: "+err.Error(), width)
		} else if backupPath != "" {
			ui.Row(ui.MarkOK, targetName, "backed up to "+shortenPath(backupPath), width)
		}
	}

	// Also backup agent directories for targets being removed.
	backupDir, agentTargets, err := resolveGlobalAgentBackupContextFromCfg(cfg)
	if err != nil || len(agentTargets) == 0 {
		return
	}
	removeSet := make(map[string]struct{}, len(toRemove))
	for _, name := range toRemove {
		removeSet[name] = struct{}{}
	}
	for _, at := range agentTargets {
		if _, ok := removeSet[at.name]; !ok {
			continue
		}
		entryName := at.name + "-agents"
		bp, bErr := backup.CreateInDir(backupDir, entryName, at.agentPath)
		if bErr != nil {
			ui.Row(ui.MarkWarn, at.name, "agents backup failed: "+bErr.Error(), width)
		} else if bp != "" {
			ui.Row(ui.MarkOK, at.name, "agents backed up to "+shortenPath(bp), width)
		}
	}
}

// unlinkTarget unlinks a single target and says what it did, or "" when
// there was nothing to unlink.
func unlinkTarget(target config.TargetConfig, sourcePath string) (string, error) {
	sc := target.SkillsConfig()
	info, err := os.Lstat(sc.Path)
	if err != nil {
		return "", nil // Target doesn't exist, OK to remove from config
	}

	if utils.IsLinkMode(sc.Path, info.Mode()) {
		if err := unlinkSymlinkMode(sc.Path, sourcePath); err != nil {
			return "", err
		}
		return "unlinked and restored", nil
	} else if info.IsDir() {
		// Remove manifest if present (merge/copy mode)
		sync.RemoveManifest(sc.Path) //nolint:errcheck
		if err := unlinkMergeMode(sc.Path, sourcePath); err != nil {
			return "", err
		}
		return "skill symlinks removed", nil
	}

	return "", nil
}

func targetRemove(args []string) error {
	opts, err := parseTargetRemoveArgs(args)
	if err != nil {
		return err
	}

	cfg, err := config.LoadWithoutProjects()
	if err != nil {
		return err
	}

	toRemove, err := resolveTargetsToRemove(cfg, opts)
	if err != nil {
		return err
	}

	if opts.dryRun {
		return targetRemoveDryRun(cfg, toRemove)
	}

	width := ui.RowWidth(toRemove...)
	backupTargets(cfg, toRemove, width)

	leaving := make(map[string]bool, len(toRemove))
	for _, targetName := range toRemove {
		leaving[targetName] = true
	}
	var stillNamed []string
	removed := 0
	for _, targetName := range toRemove {
		target := cfg.Targets[targetName]
		// Another target writing the same folder (codex and universal) still owns its links.
		if !target.SkillsConfig().IsEnabled() {
			ui.Row(ui.MarkNone, targetName, "skills off, folder left as is", width)
		} else if keeper := config.SkillsPathKeptBy(cfg.Targets, targetName, leaving); keeper != "" {
			ui.Row(ui.MarkNone, targetName, fmt.Sprintf("skills kept, %s uses the same folder", keeper), width)
		} else if done, err := unlinkTarget(target, cfg.EffectiveSkillsSource()); err != nil {
			ui.Row(ui.MarkFail, targetName, err.Error(), width)
			continue
		} else if done != "" {
			ui.Row(ui.MarkOK, targetName, done, width)
		} else {
			ui.Row(ui.MarkNone, targetName, "nothing to unlink", width)
		}
		// Read the MCP config before it is saved without the target: afterwards its
		// own name no longer resolves, so the config no longer loads.
		if target.Agent != "" {
			if warning := mcp.ReferenceWarning(config.ConfigPath(), targetName); warning != "" {
				stillNamed = append(stillNamed, warning)
			}
			if warning := hooks.ReferenceWarning(config.ConfigPath(), targetName); warning != "" {
				stillNamed = append(stillNamed, warning)
			}
		}
		delete(cfg.Targets, targetName)
		removed++
	}

	if err := cfg.Save(); err != nil {
		return err
	}
	for _, warning := range stillNamed {
		ui.Warning("%s", warning)
	}
	if removed > 0 {
		fmt.Println()
		ui.Done(ui.MarkOK, "Removed "+plural(removed, "target"), 0)
	}
	return nil
}

func targetRemoveDryRun(cfg *config.Config, toRemove []string) error {
	width := ui.RowWidth(toRemove...)
	leaving := make(map[string]bool, len(toRemove))
	for _, targetName := range toRemove {
		leaving[targetName] = true
	}
	for _, targetName := range toRemove {
		target := cfg.Targets[targetName]
		if !target.SkillsConfig().IsEnabled() {
			ui.Row(ui.MarkNone, targetName, "would remove from config · skills off, folder left as is", width)
			continue
		}
		if keeper := config.SkillsPathKeptBy(cfg.Targets, targetName, leaving); keeper != "" {
			ui.Row(ui.MarkNone, targetName, fmt.Sprintf("would remove from config · skills kept, %s uses the same folder", keeper), width)
			continue
		}
		info, err := os.Lstat(target.SkillsConfig().Path)
		if err != nil {
			if os.IsNotExist(err) {
				ui.Row(ui.MarkNone, targetName, "would remove from config · folder not found", width)
				continue
			}
			ui.Row(ui.MarkWarn, targetName, err.Error(), width)
			continue
		}

		switch {
		case utils.IsLinkMode(target.SkillsConfig().Path, info.Mode()):
			ui.Row(ui.MarkNone, targetName, "would back up, unlink and restore contents, and remove from config", width)
		case info.IsDir():
			ui.Row(ui.MarkNone, targetName, "would back up, remove skill symlinks, and remove from config", width)
		default:
			ui.Row(ui.MarkNone, targetName, "would remove from config", width)
		}
	}

	fmt.Println()
	ui.DryRun()
	return nil
}

// unlinkSymlinkMode removes symlink and copies source contents back.
func unlinkSymlinkMode(targetPath, sourcePath string) error {
	// Remove the symlink
	if err := os.Remove(targetPath); err != nil {
		return fmt.Errorf("failed to remove symlink: %w", err)
	}

	// Copy source contents to target
	if err := copyDir(sourcePath, targetPath); err != nil {
		return fmt.Errorf("failed to copy skills: %w", err)
	}

	return nil
}

// unlinkMergeMode removes individual skill symlinks pointing to source and
// copies the skill contents back so the target retains real files.
func unlinkMergeMode(targetPath, sourcePath string) error {
	entries, err := os.ReadDir(targetPath)
	if err != nil {
		return err
	}

	absSource, err := filepath.Abs(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to resolve source path: %w", err)
	}
	absSourcePrefix := absSource + string(filepath.Separator)

	for _, entry := range entries {
		skillPath := filepath.Join(targetPath, entry.Name())

		if !utils.IsSymlinkOrJunction(skillPath) {
			continue // Not a symlink — preserve local skills
		}

		absLink, err := utils.ResolveLinkTarget(skillPath)
		if err != nil {
			continue // Can't resolve — skip
		}

		// Check if symlink points to anywhere under source directory
		if !utils.PathHasPrefix(absLink, absSourcePrefix) {
			continue // Not managed by skillshare — skip
		}

		// Remove symlink and copy the skill back if source still exists
		os.Remove(skillPath)
		if _, statErr := os.Stat(absLink); statErr == nil {
			if err := copyDir(absLink, skillPath); err != nil {
				return fmt.Errorf("failed to copy %s: %w", entry.Name(), err)
			}
		}
	}

	return nil
}

// targetListJSONItem is the JSON representation for a single target.
type targetListJSONItem struct {
	Name               string   `json:"name"`
	Path               string   `json:"path"`
	Mode               string   `json:"mode"`
	TargetNaming       string   `json:"targetNaming"`
	Sync               string   `json:"sync"`
	Include            []string `json:"include"`
	Exclude            []string `json:"exclude"`
	AgentPath          string   `json:"agentPath,omitempty"`
	AgentMode          string   `json:"agentMode,omitempty"`
	AgentSync          string   `json:"agentSync,omitempty"`
	AgentInclude       []string `json:"agentInclude,omitempty"`
	AgentExclude       []string `json:"agentExclude,omitempty"`
	AgentLinkedCount   *int     `json:"agentLinkedCount,omitempty"`
	AgentLocalCount    *int     `json:"agentLocalCount,omitempty"`
	AgentExpectedCount *int     `json:"agentExpectedCount,omitempty"`
	SkillsEnabled      bool     `json:"skillsEnabled"`
	Warning            string   `json:"warning,omitempty"`
}

func targetList(jsonOutput bool) error {
	cfg, err := config.LoadWithoutProjects()
	if err != nil {
		return err
	}

	if jsonOutput {
		return targetListJSON(cfg)
	}

	items, err := buildTargetTUIItems(false, "")
	if err != nil {
		return err
	}

	printTargetListPlain(items)

	return nil
}

func targetListJSON(cfg *config.Config) error {
	items, err := buildTargetTUIItems(false, "")
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

func targetInfo(name string, args []string) error {
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

	cfg, err := config.LoadWithoutProjects()
	if err != nil {
		return err
	}

	target, exists := cfg.Targets[name]
	if !exists {
		return fmt.Errorf("target '%s' not found. Use 'skillshare target list' to see available targets", name)
	}

	// Apply filter updates if any
	if filterOpts.hasUpdates() {
		start := time.Now()
		var changes []string
		mutated := false

		if filterOpts.Skills.hasUpdates() {
			s := target.EnsureSkills()
			skillChanges, fErr := applyFilterUpdates(&s.Include, &s.Exclude, filterOpts.Skills)
			if fErr != nil {
				return fErr
			}
			changes = append(changes, skillChanges...)
			mutated = true
		}

		if filterOpts.Agents.hasUpdates() {
			agentBuilder, buildErr := targetsummary.NewGlobalBuilder(cfg)
			if buildErr != nil {
				return buildErr
			}
			agentSummary, buildErr := agentBuilder.GlobalTarget(name, target)
			if buildErr != nil {
				return buildErr
			}
			if agentSummary == nil {
				return fmt.Errorf("target '%s' does not have an agents path", name)
			}
			if agentSummary.Mode == "symlink" {
				return fmt.Errorf("target '%s' agent include/exclude filters are ignored in symlink mode; use --agent-mode merge or --agent-mode copy first", name)
			}

			ac := target.AgentsConfig()
			include := append([]string(nil), ac.Include...)
			exclude := append([]string(nil), ac.Exclude...)
			agentChanges, fErr := applyFilterUpdates(&include, &exclude, filterOpts.Agents)
			if fErr != nil {
				return fErr
			}
			if len(agentChanges) > 0 {
				a := target.EnsureAgents()
				a.Include = include
				a.Exclude = exclude
				mutated = true
			}
			changes = append(changes, scopeFilterChanges("agents", agentChanges)...)
		}

		if mutated {
			cfg.Targets[name] = target
			if err := cfg.Save(); err != nil {
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
		oplog.WriteWithLimit(config.ConfigPath(), oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck
		return nil
	}

	if settings.Skills != nil {
		return setTargetSkillsGlobal(cfg, name, target, *settings.Skills, settings.DryRun)
	}

	if settings.changesTarget() {
		return updateTargetSettingsGlobal(cfg, name, target, settings)
	}

	// Show target info
	return showTargetInfo(cfg, name, target)
}

func updateTargetSettingsGlobal(cfg *config.Config, name string, target config.TargetConfig, settings parsedTargetSettingFlags) error {
	u := targetSettingsUpdate{name: name, ensureSkills: target.EnsureSkills, ensureAgents: target.EnsureAgents, save: func() error {
		cfg.Targets[name] = target
		return cfg.Save()
	}}
	sc := target.SkillsConfig()
	u.oldMode, u.oldNaming = cmp.Or(sc.Mode, cfg.Mode, "merge"), sc.TargetNaming
	if settings.AgentMode != "" {
		agentBuilder, err := targetsummary.NewGlobalBuilder(cfg)
		if err != nil {
			return err
		}
		if u.agent, err = agentBuilder.GlobalTarget(name, target); err != nil {
			return err
		}
	}
	return u.apply(settings)
}

// targetSettingsUpdate carries one target's current skills and agents settings
// and how to write them, for --mode, --agent-mode and --target-naming in
// global and project mode alike.
type targetSettingsUpdate struct {
	name               string
	oldMode, oldNaming string                      // effective skills values
	agent              *targetsummary.AgentSummary // set only when --agent-mode is given
	ensureSkills       func() *config.ResourceTargetConfig
	ensureAgents       func() *config.ResourceTargetConfig
	save               func() error
}

// apply checks every given setting as a whole, writes the ones that differ
// and saves once, so the config never holds half of the change. A setting the
// target already has is reported as unchanged and, when nothing else differs,
// the config is not rewritten.
func (u targetSettingsUpdate) apply(settings parsedTargetSettingFlags) error {
	if settings.SkillMode != "" && !config.IsValidSyncMode(settings.SkillMode) {
		return fmt.Errorf("invalid mode '%s'. Use 'merge', 'symlink', or 'copy'", settings.SkillMode)
	}
	if settings.Naming != "" && !config.IsValidTargetNaming(settings.Naming) {
		return fmt.Errorf("invalid target naming '%s'. Use 'flat', 'standard', or 'prefixed'", settings.Naming)
	}
	if settings.AgentMode != "" {
		if !config.IsValidSyncMode(settings.AgentMode) {
			return fmt.Errorf("invalid agent mode '%s'. Use 'merge', 'symlink', or 'copy'", settings.AgentMode)
		}
		if u.agent == nil {
			return fmt.Errorf("target '%s' does not have an agents path", u.name)
		}
	}

	// An agents-only change leaves the skills pair alone: a target with skills off may keep one it cannot sync.
	if settings.SkillMode != "" || settings.Naming != "" {
		mode, naming := cmp.Or(settings.SkillMode, u.oldMode), cmp.Or(settings.Naming, u.oldNaming)
		if err := config.TargetNamingModeError(naming, mode); err != nil {
			switch {
			case settings.Naming == "":
				return fmt.Errorf("%w; change the target naming first", err)
			case settings.SkillMode == "":
				return fmt.Errorf("%w; set --mode copy first", err)
			}
			return err
		}
	}

	type row struct {
		label, old, want string
		set              func(string)
	}
	rows := []row{
		{"mode", u.oldMode, settings.SkillMode, func(v string) { u.ensureSkills().Mode = v }},
		{"target naming", config.EffectiveTargetNaming(u.oldNaming), settings.Naming, func(v string) { u.ensureSkills().TargetNaming = v }},
	}
	if u.agent != nil {
		rows = append(rows, row{"agent mode", u.agent.Mode, settings.AgentMode, func(v string) { u.ensureAgents().Mode = v }})
	}
	changed := false
	var lines []string
	for _, r := range rows {
		switch {
		case r.want == "":
		case r.want == r.old:
			lines = append(lines, fmt.Sprintf("%s %s unchanged: %s", u.name, r.label, r.old))
		default:
			r.set(r.want)
			changed = true
			lines = append(lines, fmt.Sprintf("Changed %s %s: %s -> %s", u.name, r.label, r.old, r.want))
		}
	}
	if changed {
		if err := u.save(); err != nil {
			return err
		}
	}

	for _, line := range lines {
		ui.Done(ui.MarkOK, line, 0)
	}
	if settings.AgentMode == "symlink" && settings.AgentMode != u.agent.Mode && (len(u.agent.Include) > 0 || len(u.agent.Exclude) > 0) {
		ui.Warning("Agent include/exclude filters are ignored in symlink mode")
	}
	if changed {
		ui.Next("skillshare sync", "apply the new settings")
	}
	return nil
}

func showTargetInfo(cfg *config.Config, name string, target config.TargetConfig) error {
	sc := target.SkillsConfig()
	effectiveMode := sc.Mode
	if effectiveMode == "" {
		effectiveMode = cfg.Mode
		if effectiveMode == "" {
			effectiveMode = "merge"
		}
	}

	modeDisplay := effectiveMode
	if sc.Mode == "" {
		modeDisplay = effectiveMode + ui.DimText(" (default)")
	}

	var statusLine string
	switch {
	case !sc.IsEnabled():
		statusLine = skillsOffSummary
	case effectiveMode == "copy":
		status, managed, local := sync.CheckStatusCopy(sc.Path)
		statusLine = statusWithCounts(status, managed, "managed", local)
	case effectiveMode == "merge":
		status, linked, local := sync.CheckStatusMerge(sc.Path, cfg.EffectiveSkillsSource())
		statusLine = statusWithCounts(status, linked, "linked", local)
	default:
		statusLine = sync.CheckStatus(sc.Path, cfg.EffectiveSkillsSource()).String()
	}

	namingDisplay := config.EffectiveTargetNaming(sc.TargetNaming)
	if sc.TargetNaming == "" {
		namingDisplay += ui.DimText(" (default)")
	}

	agentBuilder, err := targetsummary.NewGlobalBuilder(cfg)
	if err != nil {
		return err
	}
	agentSummary, err := agentBuilder.GlobalTarget(name, target)
	if err != nil {
		return err
	}

	fmt.Println(theme.Primary().Bold(true).Render(name))
	printTargetSkillsSection(shortenPath(sc.Path), modeDisplay, namingDisplay, statusLine, sc.Include, sc.Exclude)
	printTargetAgentSection(agentSummary)

	return nil
}
