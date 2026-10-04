package server

import (
	"errors"
	"net/http"

	"skillshare/internal/config"
	"skillshare/internal/sourcefs"
	"skillshare/internal/sourcewalk"
	ssync "skillshare/internal/sync"
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
	return ssync.FollowSetFor(s.skillsSource(), targets, s.projectRoot)
}

// serverSkillFollowSet binds discovery policy to the global server configuration.
// Keeping a nil set without declarations preserves legacy traversal and output.
func serverSkillFollowSet(cfg *config.Config) *sourcewalk.FollowSet {
	return ssync.FollowSetFor(cfg.EffectiveSkillsSource(), cfg.Targets, cfg.EffectiveGitRoot())
}

// followWriteStatus answers a declared entry with 409 and an unreadable
// declaration with 500, as the other handlers do.
func followWriteStatus(err error) int {
	if errors.Is(err, sourcefs.ErrLink) {
		return http.StatusConflict
	}
	return http.StatusInternalServerError
}
