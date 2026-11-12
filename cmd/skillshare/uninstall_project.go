package main

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/projectdir"
	"skillshare/internal/trash"
	"skillshare/internal/ui"
)

func cmdUninstallProject(args []string, root string) error {
	start := time.Now()
	opts, showHelp, err := parseUninstallArgs(args)
	if showHelp {
		printUninstallHelp()
		return err
	}
	if err != nil {
		return err
	}

	// Auto-init and the gitignore backfill may print; runUninstallSkills
	// suppresses its own output.
	setupUI := newJSONUISuppressor(opts.jsonOutput)
	defer setupUI.Flush()
	fail := func(err error) error {
		if !opts.jsonOutput {
			return err
		}
		setupUI.Flush()
		return writeJSONError(err)
	}
	if err := ensureProjectConfig(root); err != nil {
		return fail(err)
	}

	projectCfg, loadErr := config.LoadProject(root)
	if loadErr != nil {
		return fail(fmt.Errorf("failed to load project config: %w", loadErr))
	}
	sourceDir := projectCfg.EffectiveSkillsSource(root)
	gitignoreDir, gitignorePrefix := config.ProjectGitignoreTarget(root, sourceDir)

	// Load centralized metadata store for display/reinstall hints.
	skillsStore, _ := install.LoadMetadataWithMigration(sourceDir, "")
	if skillsStore == nil {
		skillsStore = install.NewMetadataStore()
	}

	// Backward compat: ensure operational dirs are gitignored for projects created before v0.17.3.
	_ = ensureProjectGitignore(projectdir.Resolve(root), false)
	setupUI.Flush()

	targetNames := make([]string, len(projectCfg.Targets))
	for i, t := range projectCfg.Targets {
		targetNames[i] = t.Name
	}
	sort.Strings(targetNames)
	mode := &uninstallMode{
		sourceDir:           sourceDir,
		walk:                projectSkillsWalk(root, projectCfg),
		sourceLabel:         ".skillshare/skills",
		trashDir:            trash.ProjectTrashDir(root),
		configPath:          config.ProjectConfigPath(root),
		store:               skillsStore,
		emptySourceErr:      "no skills found in project source",
		confirmSuffix:       " from the project",
		reinstallFlags:      " --project",
		targetNames:         targetNames,
		reportAfterFinalize: true,
		dryRunGitignore: func(*uninstallTarget) string {
			return "would update .skillshare/.gitignore"
		},
		gitignoreDir: gitignoreDir,
		gitignoreEntry: func(t *uninstallTarget) string {
			return gitignorePrefix + "/" + t.name
		},
		afterRemove: func(removed map[string]bool) {
			if err := config.ForgetProjectSkills(root, projectCfg, slices.Collect(maps.Keys(removed))); err != nil {
				ui.Warning("Failed to update the project config and %s: %v", install.LockFileName, err)
			}
		},
		preflight: projectUninstallPreflight,
	}
	return runUninstallSkills(opts, mode, args, start)
}
