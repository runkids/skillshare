package main

import (
	"fmt"
	"sort"
	"strings"
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

	if err := ensureProjectConfig(root); err != nil {
		return err
	}

	projectCfg, loadErr := config.LoadProject(root)
	if loadErr != nil {
		return fmt.Errorf("failed to load project config: %w", loadErr)
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

	// --json is global-only; project mode prints text and still confirms.
	opts.jsonOutput = false

	targetNames := make([]string, len(projectCfg.Targets))
	for i, t := range projectCfg.Targets {
		targetNames[i] = t.Name
	}
	sort.Strings(targetNames)
	targets, _ := config.ResolveValidProjectTargets(root, projectCfg)
	mode := &uninstallMode{
		follow:              skillFollowSet(sourceDir, targets, root),
		sourceDir:           sourceDir,
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
		gitignoreEntries: func(succeeded []*uninstallTarget) (string, []string) {
			entries := make([]string, len(succeeded))
			for i, t := range succeeded {
				entries[i] = gitignorePrefix + "/" + t.name
			}
			return gitignoreDir, entries
		},
		afterRemove: func(removed map[string]bool) { pruneProjectLock(root, removed) },
		preflight:   projectUninstallPreflight,
	}
	return runUninstallSkills(opts, mode, args, start)
}

// pruneProjectLock drops lock pins for removed skills. The config keeps the
// entry until the next reconcile; the pin goes now.
func pruneProjectLock(root string, removed map[string]bool) {
	lockDir := projectdir.Resolve(root)
	lock, lockErr := install.LoadLock(lockDir)
	if lockErr != nil {
		return
	}
	for name := range lock.Skills {
		for r := range removed {
			// A group uninstall names the folder; its pins are "folder/skill".
			if name == r || strings.HasPrefix(name, r+"/") {
				delete(lock.Skills, name)
			}
		}
	}
	if saveErr := lock.Save(lockDir); saveErr != nil {
		ui.Warning("Failed to update %s: %v", install.LockFileName, saveErr)
	}
}
