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
func (s *Server) skillFollowSet() *sourcewalk.FollowSet {
	if s.IsProjectMode() {
		return serverSkillFollowSetAtGitRoot(s.cfg, s.projectRoot)
	}
	return serverSkillFollowSet(s.cfg)
}

func serverSkillFollowSet(cfg *config.Config) *sourcewalk.FollowSet {
	return serverSkillFollowSetAtGitRoot(cfg, cfg.EffectiveGitRoot())
}

func serverSkillFollowSetAtGitRoot(cfg *config.Config, gitRoot string) *sourcewalk.FollowSet {
	var paths []string
	for _, target := range cfg.Targets {
		skills := target.SkillsConfig()
		if skills.IsEnabled() && skills.Path != "" {
			paths = append(paths, skills.Path)
		}
	}
	sort.Strings(paths)
	set := sourcewalk.Follow(cfg.EffectiveSkillsSource(), sourcewalk.FollowOptions{TargetPaths: paths, GitRoot: gitRoot})
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
