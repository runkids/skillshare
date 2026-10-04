package server

import (
	"sort"

	"skillshare/internal/config"
	"skillshare/internal/sourcewalk"
)

// serverSkillFollowSet binds discovery policy to the server configuration.
// Keeping a nil set without declarations preserves legacy traversal and output.
func serverSkillFollowSet(cfg *config.Config) *sourcewalk.FollowSet {
	source, targets, gitRoot := cfg.EffectiveSkillsSource(), cfg.Targets, cfg.EffectiveGitRoot()
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

// skillFollowSet requires the caller to hold s.mu while reading configuration.
func (s *Server) skillFollowSet() *sourcewalk.FollowSet {
	if !s.IsProjectMode() {
		return serverSkillFollowSet(s.cfg)
	}
	targets, _ := config.ResolveValidProjectTargets(s.projectRoot, s.projectCfg)
	var paths []string
	for _, target := range targets {
		skills := target.SkillsConfig()
		if skills.IsEnabled() && skills.Path != "" {
			paths = append(paths, skills.Path)
		}
	}
	sort.Strings(paths)
	set := sourcewalk.Follow(s.skillsSource(), sourcewalk.FollowOptions{TargetPaths: paths, GitRoot: s.projectRoot})
	if !set.Active() && len(set.Warnings()) == 0 {
		return nil
	}
	return &set
}
