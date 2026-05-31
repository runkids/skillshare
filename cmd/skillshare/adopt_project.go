package main

import (
	"fmt"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/trash"
)

// cmdAdoptProject builds the adoptContext from the project runtime and runs
// adopt, then reconciles ProjectConfig.Skills[] so adopted skills are tracked.
func cmdAdoptProject(opts adoptOptions, root string, start time.Time) error {
	runtime, err := loadProjectRuntime(root)
	if err != nil {
		return adoptCommandError(err, opts.jsonOutput)
	}

	agentsTarget, ok := findAgentsTarget(runtime.targets)
	if !ok {
		return adoptCommandError(fmt.Errorf("universal/agents target not configured in project; nothing to adopt"), opts.jsonOutput)
	}
	sc := agentsTarget.SkillsConfig()

	allTargets := make(map[string]string, len(runtime.targets))
	for name, t := range runtime.targets {
		allTargets[name] = t.SkillsConfig().Path
	}

	actx := adoptContext{
		agentsPath:  sc.Path,
		sourcePath:  runtime.sourcePath,
		syncMode:    adoptSyncMode(sc.Mode, ""),
		defaultMode: "", // project has no config-level mode; per-target mode resolves, falling back to merge
		projectRoot: root,
		allTargets:  allTargets,
		targets:     runtime.targets,
		trashBase:   trash.ProjectTrashDir(root),
		configPath:  config.ProjectConfigPath(root),
	}

	if err := runAdoptCommand(actx, opts, start); err != nil {
		return err
	}

	if opts.dryRun {
		return nil
	}

	// Reload metadata then reconcile project config so adopted skills are tracked.
	if freshStore, loadErr := install.LoadMetadata(runtime.sourcePath); loadErr == nil {
		runtime.skillsStore = freshStore
	}
	return reconcileProjectRemoteSkills(runtime)
}
