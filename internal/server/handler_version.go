package server

import (
	"net/http"

	"skillshare/internal/config"
	versioncheck "skillshare/internal/version"
)

func (s *Server) handleVersionCheck(w http.ResponseWriter, r *http.Request) {
	cliVersion := versioncheck.Version
	cliUpdateAvailable := false
	var cliLatest *string

	// CLI version check (uses 24h cache)
	if result := versioncheck.Check(cliVersion, versioncheck.InstallDirect); result != nil {
		cliUpdateAvailable = result.UpdateAvailable
		cliLatest = &result.LatestVersion
	}

	// Skill version (local) — always check global source for built-in skill
	skillSourceDir := s.cfg.Source
	if s.IsProjectMode() {
		if globalCfg, err := config.Load(); err == nil {
			skillSourceDir = globalCfg.Source
		}
	}
	skillVersion := versioncheck.ReadLocalSkillVersion(skillSourceDir)

	// Skill version (remote) — network call with 3s timeout
	var skillLatest *string
	skillUpdateAvailable := false
	if skillVersion != "" {
		if remote := versioncheck.FetchRemoteSkillVersion(); remote != "" {
			skillLatest = &remote
			skillUpdateAvailable = remote != skillVersion
		}
	}

	writeJSON(w, map[string]any{
		"cliVersion":           cliVersion,
		"cliLatest":            cliLatest,
		"cliUpdateAvailable":   cliUpdateAvailable,
		"skillVersion":         skillVersion,
		"skillLatest":          skillLatest,
		"skillUpdateAvailable": skillUpdateAvailable,
	})
}
