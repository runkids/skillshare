package main

import (
	"fmt"
	"path/filepath"
	"sort"

	"skillshare/internal/config"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/utils"
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

// configuredSkillFollowSet supports source-only init and hub entry points.
func configuredSkillFollowSet(source string) *sourcewalk.FollowSet {
	if cfg, err := config.Load(); err == nil && utils.PathsEqual(source, cfg.EffectiveSkillsSource()) {
		return globalSkillFollowSet(cfg)
	}
	return skillFollowSet(source, nil, source)
}

// skillfollowPauses names each unavailable entry that pauses prune, with the
// step that resumes cleanup.
func skillfollowPauses(source string, follow *sourcewalk.FollowSet) []string {
	if follow == nil {
		return nil
	}
	var messages []string
	for _, entry := range follow.Unavailable() {
		messages = append(messages, fmt.Sprintf("prune paused: %s is %s; restore or fix %s, or remove %s from .skillfollow[.local], to resume cleanup",
			entry.Name, entry.State, filepath.Join(source, entry.Name), entry.Name))
	}
	return messages
}

// firstFollowSet preserves existing private helper callers while commands pass
// the operation's config-bound set explicitly.
func firstFollowSet(sets []*sourcewalk.FollowSet) *sourcewalk.FollowSet {
	if len(sets) == 0 {
		return nil
	}
	return sets[0]
}
