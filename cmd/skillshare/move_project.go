package main

import (
	"fmt"

	"skillshare/internal/config"
)

// projectMoveMode is the project counterpart: the source, targets, lockfile
// and .gitignore come from the project, and reconcile also rewrites config.yaml.
func projectMoveMode(root string) (*moveMode, error) {
	if err := ensureProjectConfig(root); err != nil {
		return nil, err
	}
	cfg, err := config.LoadProject(root)
	if err != nil {
		return nil, fmt.Errorf("failed to load project config: %w", err)
	}
	sourceDir := cfg.EffectiveSkillsSource(root)
	store, err := loadMoveStore(sourceDir)
	if err != nil {
		return nil, err
	}
	targets, _ := config.ResolveValidProjectTargets(root, cfg)
	gitignoreDir, prefix := config.ProjectGitignoreTarget(root, sourceDir)
	return &moveMode{
		sourceDir:       sourceDir,
		walk:            config.SkillsWalk(cfg.FollowSourceLinks, sourceDir, targets),
		store:           store,
		targets:         targets,
		cfgPath:         config.ProjectConfigPath(root),
		projectRoot:     root,
		gitignoreDir:    gitignoreDir,
		gitignorePrefix: prefix,
		reconcile:       func() error { return config.ReconcileProjectSkills(root, cfg, store, sourceDir) },
	}, nil
}
