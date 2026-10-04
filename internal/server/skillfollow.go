package server

import (
	"path/filepath"
	"sort"

	"skillshare/internal/config"
	"skillshare/internal/sourcefs"
	"skillshare/internal/sourcewalk"
)

// skillFollowSet snapshots the active skills targets and staging boundary.
// Callers hold s.mu while reading config and share the result for one operation.
// Project mode reads the project's own source and targets, with the project
// root as the Git boundary.
func (s *Server) skillFollowSet() *sourcewalk.FollowSet {
	if !s.IsProjectMode() {
		return serverSkillFollowSet(s.cfg)
	}
	targets, _ := config.ResolveValidProjectTargets(s.projectRoot, s.projectCfg)
	return followSetFor(s.skillsSource(), targets, s.projectRoot)
}

// serverSkillFollowSet binds discovery policy to the global server configuration.
// Keeping a nil set without declarations preserves legacy traversal and output.
func serverSkillFollowSet(cfg *config.Config) *sourcewalk.FollowSet {
	return followSetFor(cfg.EffectiveSkillsSource(), cfg.Targets, cfg.EffectiveGitRoot())
}

func followSetFor(source string, targets map[string]config.TargetConfig, gitRoot string) *sourcewalk.FollowSet {
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

// followedSkillWriteError also refuses unavailable declared trees, before any write.
func followedSkillWriteError(source, rel string, follow *sourcewalk.FollowSet) error {
	if follow != nil {
		if entry, ok := follow.InFollowed(filepath.ToSlash(rel)); ok {
			return &sourcefs.LinkError{Path: filepath.Join(source, entry.Name)}
		}
	}
	return nil
}
