package main

import (
	"fmt"
	"os"
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

	if len(rest) == 0 {
		printTrashHelp()
		return nil
	}

	sub := rest[0]
	subArgs := rest[1:]

	switch sub {
	case "list", "ls":
		return trashList(mode, cwd)
	case "restore":
		return trashRestore(mode, cwd, subArgs)
	case "delete", "rm":
		return trashDelete(mode, cwd, subArgs)
	case "empty":
		return trashEmpty(mode, cwd)
	case "--help", "-h", "help":
		printTrashHelp()
		return nil
	default:
		printTrashHelp()
		return fmt.Errorf("unknown subcommand: %s", sub)
	}
}

func trashList(mode runMode, cwd string) error {
	trashBase := resolveTrashBase(mode, cwd)
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

func trashRestore(mode runMode, cwd string, args []string) error {
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

	trashBase := resolveTrashBase(mode, cwd)
	entry := trash.FindByName(trashBase, name)
	if entry == nil {
		cmdErr := fmt.Errorf("'%s' not found in trash", name)
		logTrashOp(cfgPath, "restore", 0, name, start, cmdErr)
		return cmdErr
	}

	destDir, err := resolveSourceDir(mode, cwd)
	if err != nil {
		logTrashOp(cfgPath, "restore", 0, name, start, err)
		return err
	}

	if err := trash.Restore(entry, destDir); err != nil {
		logTrashOp(cfgPath, "restore", 0, name, start, err)
		return err
	}

	ui.Success("Restored: %s", name)
	age := time.Since(entry.Date)
	ui.Info("Trashed %s ago, now back in %s", formatAge(age), destDir)
	ui.SectionLabel("Next Steps")
	ui.Info("Run 'skillshare sync' to update targets")

	logTrashOp(cfgPath, "restore", 1, name, start, nil)
	return nil
}

func trashDelete(mode runMode, cwd string, args []string) error {
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

	trashBase := resolveTrashBase(mode, cwd)
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

func trashEmpty(mode runMode, cwd string) error {
	start := time.Now()
	cfgPath := resolveTrashCfgPath(mode, cwd)

	trashBase := resolveTrashBase(mode, cwd)
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

func resolveTrashBase(mode runMode, cwd string) string {
	if mode == modeProject {
		return trash.ProjectTrashDir(cwd)
	}
	return trash.TrashDir()
}

func resolveSourceDir(mode runMode, cwd string) (string, error) {
	if mode == modeProject {
		return fmt.Sprintf("%s/.skillshare/skills", cwd), nil
	}
	cfg, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w", err)
	}
	return cfg.Source, nil
}

func formatAge(d time.Duration) string {
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
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
	fmt.Println(`Usage: skillshare trash <command> [options]

Manage uninstalled skills in the trash.

Commands:
  list, ls              List trashed skills
  restore <name>        Restore most recent trashed version to source
  delete, rm <name>     Permanently delete a single item from trash
  empty                 Permanently delete all items from trash

Options:
  --project, -p         Use project-level trash
  --global, -g          Use global trash
  --help, -h            Show this help

Examples:
  skillshare trash list                    # List trashed skills
  skillshare trash restore my-skill        # Restore from trash
  skillshare trash restore my-skill -p     # Restore in project mode
  skillshare trash delete my-skill         # Permanently delete from trash
  skillshare trash empty                   # Empty the trash`)
}
