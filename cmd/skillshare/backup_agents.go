package main

import (
	"fmt"
	"time"

	"skillshare/internal/backup"
	"skillshare/internal/config"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
)

// createAgentBackup backs up agent target directories.
// Agent backups use "<target>-agents" as the backup entry name.
// In project mode, backups are stored under <project-dir>/backups/.
func createAgentBackup(mode runMode, cwd, targetName string, dryRun bool) error {
	backupDir, targets, err := resolveAgentBackupContext(mode, cwd)
	if err != nil {
		return err
	}

	start := time.Now()
	var names []string
	for _, at := range targets {
		if targetName == "" || at.name == targetName {
			names = append(names, at.name+"-agents")
		}
	}
	width := ui.RowWidth(names...)

	created, failed := 0, 0
	for _, at := range targets {
		if targetName != "" && at.name != targetName {
			continue
		}

		entryName := at.name + "-agents"

		if dryRun {
			ui.Row(ui.MarkNone, entryName, "would back up "+utils.FoldHomePath(at.agentPath), width)
			continue
		}

		backupPath, backupErr := backup.CreateInDir(backupDir, entryName, at.agentPath)
		if backupErr != nil {
			ui.Row(ui.MarkFail, entryName, backupErr.Error(), width)
			failed++
			continue
		}
		if backupPath != "" {
			ui.Row(ui.MarkOK, entryName, utils.FoldHomePath(backupPath), width)
			created++
		} else {
			ui.Row(ui.MarkNone, entryName, ui.DimText("nothing to back up"), width)
		}
	}

	if len(names) > 0 {
		fmt.Println()
	}
	if dryRun {
		ui.DryRun()
		return nil
	}
	printBackupDone(created, failed, "agents of ", start)
	return nil
}

// restoreAgentBackup restores agent target directories from backup.
func restoreAgentBackup(mode runMode, cwd, targetName, fromTimestamp string, force, dryRun bool) error {
	if targetName == "" {
		return fmt.Errorf("usage: skillshare restore agents <target> [--from <timestamp>] [--force] [--dry-run]")
	}

	backupDir, targets, err := resolveAgentBackupContext(mode, cwd)
	if err != nil {
		return err
	}

	// Find the target's agent path.
	var agentPath string
	for _, at := range targets {
		if at.name == targetName {
			agentPath = at.agentPath
			break
		}
	}
	if agentPath == "" {
		return fmt.Errorf("target '%s' has no agent path configured", targetName)
	}

	entryName := targetName + "-agents"
	if dryRun {
		ui.Done(ui.MarkNone, fmt.Sprintf("Would restore %s to %s", entryName, utils.FoldHomePath(agentPath)), 0)
		fmt.Println()
		ui.DryRun()
		return nil
	}

	opts := backup.RestoreOptions{Force: force}
	if fromTimestamp != "" {
		return restoreFromTimestampInDir(backupDir, entryName, agentPath, fromTimestamp, opts)
	}
	return restoreFromLatestInDir(backupDir, entryName, agentPath, opts)
}

// agentTarget holds resolved name + agent path for backup/restore.
type agentTarget struct {
	name      string
	agentPath string
}

// resolveAgentBackupContext returns the backup directory and agent-capable targets
// for the given mode.
func resolveAgentBackupContext(mode runMode, cwd string) (string, []agentTarget, error) {
	if mode == modeProject {
		return resolveProjectAgentBackupContext(cwd)
	}
	return resolveGlobalAgentBackupContext()
}

func resolveGlobalAgentBackupContext() (string, []agentTarget, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", nil, err
	}
	return resolveGlobalAgentBackupContextFromCfg(cfg)
}

// resolveGlobalAgentBackupContextFromCfg is like resolveGlobalAgentBackupContext
// but accepts an already-loaded config to avoid redundant config.Load() calls.
func resolveGlobalAgentBackupContextFromCfg(cfg *config.Config) (string, []agentTarget, error) {
	builtinAgents := config.DefaultAgentTargets()
	var targets []agentTarget
	for name := range cfg.Targets {
		agentPath := resolveAgentTargetPath(cfg.Targets[name], builtinAgents, name)
		if agentPath != "" {
			targets = append(targets, agentTarget{name: name, agentPath: agentPath})
		}
	}

	return backup.BackupDir(), targets, nil
}

func resolveProjectAgentBackupContext(cwd string) (string, []agentTarget, error) {
	projCfg, err := config.LoadProject(cwd)
	if err != nil {
		return "", nil, fmt.Errorf("cannot load project config: %w", err)
	}

	builtinAgents := config.ProjectAgentTargets()
	var targets []agentTarget
	for _, entry := range projCfg.Targets {
		agentPath := resolveProjectAgentTargetPath(entry, builtinAgents, cwd)
		if agentPath != "" {
			targets = append(targets, agentTarget{name: entry.Name, agentPath: agentPath})
		}
	}

	backupDir := backup.ProjectBackupDir(cwd)
	return backupDir, targets, nil
}
