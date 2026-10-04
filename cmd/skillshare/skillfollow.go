package main

import (
	"sort"

	"skillshare/internal/config"
	"skillshare/internal/sourcewalk"
)

// skillFollowSet binds discovery policy to the current command's resolved config.
// Keeping a nil set without declarations preserves legacy traversal and output.
func skillFollowSet(source string, targets map[string]config.TargetConfig, gitRoot string) *sourcewalk.FollowSet {
	var paths []string
	for _, target := range targets {
		skills := target.SkillsConfig()
		if skills.IsEnabled() && skills.Path != "" {
			paths = append(paths, skills.Path)
		}
	}
	sort.Strings(paths)
	set := sourcewalk.Follow(source, sourcewalk.FollowOptions{TargetPaths: paths, GitRoot: gitRoot})
	if !set.Active() && len(set.Warnings()) == 0 {
		return nil
	}
	return &set
}

func globalSkillFollowSet(cfg *config.Config) *sourcewalk.FollowSet {
	return skillFollowSet(cfg.EffectiveSkillsSource(), cfg.Targets, cfg.EffectiveGitRoot())
}

// firstFollowSet preserves existing private helper callers while commands pass
// the operation's config-bound set explicitly.
func firstFollowSet(sets []*sourcewalk.FollowSet) *sourcewalk.FollowSet {
	if len(sets) == 0 {
		return nil
	}
	return sets[0]
}
