package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/trash"
	"skillshare/internal/ui"
)

func cmdTrash(args []string) error {
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

	// Extract kind filter (e.g. "skillshare trash agents list" or "--all").
	kind, rest := parseKindArgWithAll(rest)

	if len(rest) == 0 {
		printTrashHelp()
		return nil
	}

	sub := rest[0]
	subArgs := rest[1:]

	// Parse --no-tui from subArgs for list command
	noTUI := false
	var filteredArgs []string
	for _, arg := range subArgs {
		if arg == "--no-tui" {
			noTUI = true
		} else {
			filteredArgs = append(filteredArgs, arg)
		}
	}

	switch sub {
	case "list", "ls":
		return trashList(mode, cwd, noTUI, kind)
	case "restore":
		return trashRestore(mode, cwd, filteredArgs, kind)
	case "delete", "rm":
		return trashDelete(mode, cwd, filteredArgs, kind)
	case "empty":
		return trashEmpty(mode, cwd, kind)
	case "--help", "-h", "help":
		printTrashHelp()
		return nil
	default:
		printTrashHelp()
		return fmt.Errorf("unknown subcommand: %s", sub)
	}
}

func trashList(mode runMode, cwd string, noTUI bool, kind resourceKindFilter) error {
	// TUI path: merge skill + agent trash when kind includes both
	if shouldLaunchTUI(noTUI, nil) {
		var items []trash.TrashEntry

		if kind.IncludesSkills() {
			skillBase := resolveTrashBase(mode, cwd, kindSkills)
			for _, e := range trash.List(skillBase) {
				e.Kind = "skill"
				items = append(items, e)
			}
		}
		if kind.IncludesAgents() {
			agentBase := resolveTrashBase(mode, cwd, kindAgents)
			for _, e := range trash.List(agentBase) {
				e.Kind = "agent"
				items = append(items, e)
			}
		}

		if len(items) == 0 {
			ui.Info("Trash is empty")
			return nil
		}

		// Sort merged list by date (newest first)
		sort.Slice(items, func(i, j int) bool {
			return items[i].Date.After(items[j].Date)
		})

		modeLabel := "global"
		if mode == modeProject {
			modeLabel = "project"
		}
		skillTrashBase := resolveTrashBase(mode, cwd, kindSkills)
		agentTrashBase := resolveTrashBase(mode, cwd, kindAgents)
		cfgPath := resolveTrashCfgPath(mode, cwd)
		destDir, err := resolveSourceDir(mode, cwd, kindSkills)
		if err != nil {
			return err
		}
		agentDestDir, err := resolveSourceDir(mode, cwd, kindAgents)
		if err != nil {
			return err
		}
		return runTrashTUI(items, skillTrashBase, agentTrashBase, destDir, agentDestDir, cfgPath, modeLabel)
	}

	// Plain text path (unchanged) — list single kind
	trashBase := resolveTrashBase(mode, cwd, kind)
	items := trash.List(trashBase)

	if len(items) == 0 {
		ui.Info("Trash is empty")
		return nil
	}

	ui.Header("Trash")
	for _, item := range items {
		age := time.Since(item.Date)
		ageStr := formatAge(age)
		sizeStr := formatBytes(item.Size)
		ui.Info("  %s  (%s, %s ago)", item.Name, sizeStr, ageStr)
	}

	totalSize := trash.TotalSize(trashBase)
	fmt.Println()
	ui.Info("%d item(s), %s total", len(items), formatBytes(totalSize))
	ui.Info("Items are automatically cleaned up after 7 days")

	return nil
}

func trashRestore(mode runMode, cwd string, args []string, kind resourceKindFilter) error {
	start := time.Now()

	var name string
	for _, arg := range args {
		switch {
		case arg == "--help" || arg == "-h":
			printTrashHelp()
			return nil
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unknown option: %s", arg)
		default:
			if name != "" {
				return fmt.Errorf("unexpected argument: %s", arg)
			}
			name = arg
		}
	}

	if name == "" {
		printTrashHelp()
		return fmt.Errorf("skill name is required")
	}

	cfgPath := resolveTrashCfgPath(mode, cwd)

	trashBase := resolveTrashBase(mode, cwd, kind)
	entry := trash.FindByName(trashBase, name)
	if entry == nil {
		cmdErr := fmt.Errorf("'%s' not found in trash", name)
		logTrashOp(cfgPath, "restore", 0, name, start, cmdErr)
		return cmdErr
	}

	destDir, err := resolveSourceDir(mode, cwd, kind)
	if err != nil {
		logTrashOp(cfgPath, "restore", 0, name, start, err)
		return err
	}

	if kind == kindAgents {
		if err := trash.RestoreAgent(entry, destDir); err != nil {
			logTrashOp(cfgPath, "restore", 0, name, start, err)
			return err
		}
	} else {
		if err := trash.Restore(entry, destDir); err != nil {
			logTrashOp(cfgPath, "restore", 0, name, start, err)
			return err
		}
	}

	ui.Success("Restored: %s", name)
	age := time.Since(entry.Date)
	ui.Info("Trashed %s ago, now back in %s", formatAge(age), destDir)
	ui.SectionLabel("Next Steps")
	syncHint := "skillshare sync"
	if kind == kindAgents {
		syncHint = "skillshare sync agents"
	}
	ui.Info("Run '%s' to update targets", syncHint)

	logTrashOp(cfgPath, "restore", 1, name, start, nil)
	return nil
}

func trashDelete(mode runMode, cwd string, args []string, kind resourceKindFilter) error {
	var name string
	for _, arg := range args {
		switch {
		case arg == "--help" || arg == "-h":
			printTrashHelp()
			return nil
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unknown option: %s", arg)
		default:
			if name != "" {
				return fmt.Errorf("unexpected argument: %s", arg)
			}
			name = arg
		}
	}

	if name == "" {
		printTrashHelp()
		return fmt.Errorf("skill name is required")
	}

	trashBase := resolveTrashBase(mode, cwd, kind)
	entry := trash.FindByName(trashBase, name)
	if entry == nil {
		return fmt.Errorf("'%s' not found in trash", name)
	}

	if err := os.RemoveAll(entry.Path); err != nil {
		return fmt.Errorf("failed to delete '%s': %w", name, err)
	}

	ui.Success("Permanently deleted: %s", name)
	return nil
}

func trashEmpty(mode runMode, cwd string, kind resourceKindFilter) error {
	start := time.Now()
	cfgPath := resolveTrashCfgPath(mode, cwd)

	trashBase := resolveTrashBase(mode, cwd, kind)
	items := trash.List(trashBase)

	if len(items) == 0 {
		ui.Info("Trash is already empty")
		return nil
	}

	ui.Warning("This will permanently delete %d item(s) from trash", len(items))
	fmt.Print("Continue? [y/N]: ")
	var input string
	fmt.Scanln(&input)
	input = strings.ToLower(strings.TrimSpace(input))
	if input != "y" && input != "yes" {
		ui.Info("Cancelled")
		return nil
	}

	removed := 0
	for _, item := range items {
		if err := os.RemoveAll(item.Path); err != nil {
			cmdErr := fmt.Errorf("failed to delete '%s': %w", item.Name, err)
			logTrashOp(cfgPath, "empty", removed, "", start, cmdErr)
			return cmdErr
		}
		removed++
	}

	ui.Success("Emptied trash: %d item(s) permanently deleted", removed)
	logTrashOp(cfgPath, "empty", removed, "", start, nil)
	return nil
}

func resolveTrashBase(mode runMode, cwd string, kind resourceKindFilter) string {
	if kind == kindAgents {
		if mode == modeProject {
			return trash.ProjectAgentTrashDir(cwd)
		}
		return trash.AgentTrashDir()
	}
	if mode == modeProject {
		return trash.ProjectTrashDir(cwd)
	}
	return trash.TrashDir()
}

func resolveSourceDir(mode runMode, cwd string, kind resourceKindFilter) (string, error) {
	if mode == modeProject {
		projCfg, err := config.LoadProject(cwd)
		if err != nil {
			return "", fmt.Errorf("failed to load project config: %w", err)
		}
		if kind == kindAgents {
			return projCfg.EffectiveAgentsSource(cwd), nil
		}
		return projCfg.EffectiveSkillsSource(cwd), nil
	}
	if kind == kindAgents {
		cfg, err := config.Load()
		if err != nil {
			return "", fmt.Errorf("failed to load config: %w", err)
		}
		return cfg.EffectiveAgentsSource(), nil
	}
	cfg, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w", err)
	}
	return cfg.Source, nil
}

// formatAge is an alias for the shared formatDurationShort.
func formatAge(d time.Duration) string {
	return formatDurationShort(d)
}

func resolveTrashCfgPath(mode runMode, cwd string) string {
	if mode == modeProject {
		return config.ProjectConfigPath(cwd)
	}
	return config.ConfigPath()
}

func logTrashOp(cfgPath string, action string, count int, name string, start time.Time, cmdErr error) {
	e := oplog.NewEntry("trash", statusFromErr(cmdErr), time.Since(start))
	a := map[string]any{"action": action}
	if count > 0 {
		a["items"] = count
	}
	if name != "" {
		a["name"] = name
	}
	e.Args = a
	if cmdErr != nil {
		e.Message = cmdErr.Error()
	}
	oplog.WriteWithLimit(cfgPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck
}

func printTrashHelp() {
	fmt.Println(`Usage: skillshare trash [agents] <command> [options]

Manage uninstalled skills in the trash.

Commands:
  list, ls              List trashed skills (interactive TUI in TTY)
  restore <name>        Restore most recent trashed version to source
  delete, rm <name>     Permanently delete a single item from trash
  empty                 Permanently delete all items from trash

Options:
  --all                 Include both skills and agents
  --no-tui              Disable interactive TUI, use plain text output
  --project, -p         Use project-level trash
  --global, -g          Use global trash
  --help, -h            Show this help

Examples:
  skillshare trash list                    # Interactive TUI (in TTY)
  skillshare trash list --no-tui           # Plain text output
  skillshare trash restore my-skill        # Restore from trash
  skillshare trash restore my-skill -p     # Restore in project mode
  skillshare trash delete my-skill         # Permanently delete from trash
  skillshare trash empty                   # Empty the trash
  skillshare trash agents list             # List trashed agents
  skillshare trash agents restore tutor    # Restore an agent from trash
  skillshare trash --all list              # List trashed skills + agents`)
}
