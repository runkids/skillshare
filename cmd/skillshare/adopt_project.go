package main

import (
	"fmt"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/trash"
)

// cmdAdoptProject builds the adoptContext from the project runtime and runs adopt.
// Adopted skills are local project content, so they do not gain remote-install
// metadata or a declarative ProjectConfig.Skills entry.
func cmdAdoptProject(opts adoptOptions, root string, start time.Time) error {
	projectCfg, err := config.LoadProject(root)
	if err != nil {
		return adoptCommandError(err, opts.jsonOutput)
	}
	targets, err := config.ResolveProjectTargets(root, projectCfg)
	if err != nil {
		return adoptCommandError(err, opts.jsonOutput)
	}

	agentsTarget, ok := findAgentsTarget(targets)
	if !ok {
		return adoptCommandError(fmt.Errorf("universal/agents target not configured in project; nothing to adopt"), opts.jsonOutput)
	}
	sc := agentsTarget.SkillsConfig()

	allTargets := make(map[string]string, len(targets))
	for name, t := range targets {
		allTargets[name] = t.SkillsConfig().Path
	}

	actx := adoptContext{
		agentsPath:         sc.Path,
		sourcePath:         projectCfg.EffectiveSkillsSource(root),
		syncMode:           adoptSyncMode(sc.Mode, ""),
		defaultMode:        "", // project has no config-level mode; per-target mode resolves, falling back to merge
		projectRoot:        root,
		fileIgnorePatterns: sync.EffectiveFileIgnorePatterns(projectCfg.Ignore),
		projectSkills:      append([]config.SkillEntry(nil), projectCfg.Skills...),
		allTargets:         allTargets,
		targets:            targets,
		trashBase:          trash.ProjectTrashDir(root),
		configPath:         config.ProjectConfigPath(root),
	}

	return runAdoptCommand(actx, opts, start)
}
