package main

import (
	"skillshare/internal/config"
	"skillshare/internal/sourcewalk"
	ssync "skillshare/internal/sync"
	"skillshare/internal/utils"
)

// skillFollowSet binds discovery policy to the current command's resolved config.
func skillFollowSet(source string, targets map[string]config.TargetConfig, gitRoot string) *sourcewalk.FollowSet {
	return ssync.FollowSetFor(source, targets, gitRoot)
}

func globalSkillFollowSet(cfg *config.Config) *sourcewalk.FollowSet {
	return skillFollowSet(cfg.EffectiveSkillsSource(), cfg.Targets, cfg.EffectiveGitRoot())
}

// configuredSkillFollowSet supports source-only init and hub entry points.
func configuredSkillFollowSet(source string) *sourcewalk.FollowSet {
	if cfg, err := config.Load(); err == nil && utils.PathsEqual(source, cfg.EffectiveSkillsSource()) {
		return globalSkillFollowSet(cfg)
	}
	return skillFollowSet(source, nil, source)
}
