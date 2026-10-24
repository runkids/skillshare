package main

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"skillshare/internal/backup"
	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/projects"
	ssync "skillshare/internal/sync"
	"skillshare/internal/utils"
)

// projectResult is a project operation and the sync it ran, if any.
type projectResult struct {
	projects.Result
	Sync *ssync.ExecuteResult
}

// syncProject syncs the enabled skills and agents of the project declared
// under root in cfg, which may be an unsaved candidate. It prepares the run as
// `skillshare sync` does and executes it with the target engine the dashboard
// uses, so only this project's targets change. A nil result means nothing ran.
func syncProject(cfg *config.Config, root string, dryRun bool) (*ssync.ExecuteResult, error) {
	entry, ok := cfg.Projects[root]
	if !ok {
		return nil, fmt.Errorf("%w: %s", projects.ErrNotFound, root)
	}
	abs := config.ExpandPath(root)
	if info, err := os.Stat(abs); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("%w: %s", projects.ErrFolderNotFound, root)
	}
	skills := entry.Skills != nil && entry.Skills.IsEnabled()
	agents := entry.Agents != nil && entry.Agents.IsEnabled()
	var kinds []string
	if skills {
		kinds = append(kinds, "skill")
	}
	if agents {
		kinds = append(kinds, "agent")
	}
	if len(kinds) == 0 {
		return &ssync.ExecuteResult{}, nil
	}

	name := entry.ProjectName(root)
	targets := map[string]config.TargetConfig{}
	for target, tc := range cfg.Targets {
		project, _, isProject := config.SplitProjectTarget(target)
		if isProject && project == name && tc.ProjectRoot() != "" && utils.PathsEqual(tc.ProjectRoot(), abs) {
			targets[target] = tc
		}
	}

	// A target with invalid settings fails alone, as in `skillshare sync`.
	warnings, invalid, err := config.ValidateConfigForSync(cfg)
	if err != nil {
		return nil, err
	}
	var failed []ssync.TargetFailure
	for _, target := range slices.Sorted(maps.Keys(targets)) {
		if err := invalid[target]; err != nil {
			delete(targets, target)
			msg := target + ": invalid config: " + err.Error()
			warnings = append(warnings, msg)
			failed = append(failed, ssync.TargetFailure{Target: target, Part: "config", Error: err.Error(), Message: msg})
		}
	}
	if !dryRun {
		warnings = append(warnings, ssync.BackupTargets(targets, skills, agents, ssync.BackupTargetsOptions{Dir: backup.BackupDir(), Retention: backup.RetentionConfig(cfg), BuiltinAgentTargets: config.DefaultAgentTargets()})...)
	}

	discovery := ssync.TargetDiscovery{}
	if skills {
		if !dryRun {
			warnings = append(warnings, adoptMovedGlobalSkills(cfg)...)
		}
		walk := cfg.SkillsWalk()
		discovery.Skills, discovery.IgnoreStats, err = ssync.DiscoverSourceSkillsWithStatsAndContext(cfg.EffectiveSkillsSource(), walk)
		if err != nil {
			return nil, fmt.Errorf("failed to discover skills: %w", err)
		}
		discovery.SourceIncomplete = walk.Follow.Incomplete()
		warnings = append(warnings, ssync.SourceLinkWarnings(walk, discovery.SourceIncomplete)...)
		if len(discovery.Skills) == 0 {
			warnings = append(warnings, "source directory is empty (0 skills)")
		}
	}
	if agents {
		discovery.Agents, discovery.AllAgents, discovery.AgentsAvailable = ssync.DiscoverTargetAgents(cfg.EffectiveAgentsSource())
	}

	extensionsDir := filepath.Join(filepath.Dir(config.ReadOnlyConfigPath()), "extensions")
	executed, err := ssync.ExecuteTargets(context.Background(), cfg, targets, ssync.ExecuteOptions{
		Kinds: kinds, DryRun: dryRun, AgentsSource: cfg.EffectiveAgentsSource(), Discovery: discovery,
		BuiltinAgentTargets: config.DefaultAgentTargets(), DiagnosticsTargets: cfg.Targets,
		ResolveExtension: func(ext string) (*ssync.ExtensionSpec, error) { return resolveExtension(ext, extensionsDir) },
	})
	executed.Warnings = append(warnings, executed.Warnings...)
	executed.Failed = append(failed, executed.Failed...)
	return &executed, err
}

// adoptMovedGlobalSkills keeps the install record of a skill moved by hand;
// sync never prunes one. A failure is a warning.
func adoptMovedGlobalSkills(cfg *config.Config) []string {
	store, err := install.LoadMetadata(cfg.EffectiveSkillsSource())
	if err == nil {
		err = config.AdoptMovedGlobalSkills(cfg, store)
	}
	if err != nil {
		return []string{fmt.Sprintf("install metadata not updated: %v", err)}
	}
	return nil
}
