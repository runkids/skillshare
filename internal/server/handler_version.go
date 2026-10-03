package server

import (
	"net/http"

	"skillshare/internal/config"
	versioncheck "skillshare/internal/version"
)

func (s *Server) handleVersionCheck(w http.ResponseWriter, r *http.Request) {
	// Snapshot config under RLock, then release before I/O.
	s.mu.RLock()
	source := s.cfg.EffectiveSkillsSource()
	projectRoot := s.projectRoot
	s.mu.RUnlock()

	isProjectMode := projectRoot != ""

	cliVersion := versioncheck.Version
	cliUpdateAvailable := false
	var cliLatest *string
	cliDevMode := cliVersion == "" || cliVersion == "dev"

	// Dev builds expose a simulated update so the browser update/restart flow can be tested.
	if cliDevMode {
		latest := "dev-ui-flow"
		cliUpdateAvailable = true
		cliLatest = &latest
	} else if result := versioncheck.Check(cliVersion, versioncheck.InstallDirect); result != nil {
		// CLI version check (uses 24h cache)
		cliUpdateAvailable = result.UpdateAvailable
		cliLatest = &result.LatestVersion
	}

	// Skill version (local) — always check global source for built-in skill
	skillSourceDir := source
	if isProjectMode {
		if globalCfg, err := config.Load(); err == nil {
			skillSourceDir = globalCfg.Source
		}
	}
	skillVersion := versioncheck.ReadLocalSkillVersion(skillSourceDir)

	// Skill version (remote) — uses 24h cache, network call with 3s timeout
	var skillLatest *string
	skillUpdateAvailable := false
	if skillVersion != "" {
		if remote := versioncheck.CachedRemoteSkillVersion(); remote != "" {
			skillLatest = &remote
			skillUpdateAvailable = versioncheck.SkillOutdated(skillVersion, remote)
		}
	}

	writeJSON(w, map[string]any{
		"cliVersion":           cliVersion,
		"cliLatest":            cliLatest,
		"cliUpdateAvailable":   cliUpdateAvailable,
		"cliDevMode":           cliDevMode,
		"skillVersion":         skillVersion,
		"skillLatest":          skillLatest,
		"skillUpdateAvailable": skillUpdateAvailable,
	})
}
