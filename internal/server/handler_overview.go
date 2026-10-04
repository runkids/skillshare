package server

import (
	"net/http"
	"path/filepath"
	"strings"

	"skillshare/internal/git"
	"skillshare/internal/install"
	"skillshare/internal/resource"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/sync"
	"skillshare/internal/utils"
	versioncheck "skillshare/internal/version"
)

type trackedRepoItem struct {
	Name       string `json:"name"`
	SkillCount int    `json:"skillCount"`
	Dirty      bool   `json:"dirty"`
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	// Snapshot config under RLock, then release before I/O.
	s.mu.RLock()
	source := s.cfg.EffectiveSkillsSource()
	follow := s.skillFollowSet()
	agentsSource := s.agentsSource()
	extrasSource := s.cfg.EffectiveExtrasSource()
	if s.IsProjectMode() {
		extrasSource = s.projectCfg.EffectiveExtrasSource(s.projectRoot)
	}
	cfgMode := s.cfg.Mode
	targetCount := len(s.cfg.Targets)
	projectRoot := s.projectRoot
	configDir := filepath.Dir(s.configPath())
	s.mu.RUnlock()

	isProjectMode := projectRoot != ""

	// Count skills
	skills, _, err := sync.DiscoverSourceSkillsWithOptions(source, sync.DiscoveryOptions{Follow: follow})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Count top-level source entries (for display)
	topLevelCount := 0
	entries, readErr := sourcewalk.ReadDir(source, sourcewalk.Options{Follow: follow})
	if follow != nil && (readErr != nil || follow.Err() != nil) {
		if readErr == nil {
			readErr = follow.Err()
		}
		writeError(w, http.StatusInternalServerError, readErr.Error())
		return
	}
	for _, e := range entries {
		if e.IsDir() && !utils.IsHidden(e.Name()) {
			topLevelCount++
		}
	}

	mode := cfgMode
	if mode == "" {
		mode = "merge"
	}

	// Tracked repos
	trackedRepos, err := buildTrackedReposWithOptions(source, skills, sourcewalk.Options{Follow: follow})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Count agents
	agentCount := 0
	if agentsSource != "" {
		if agents, discoverErr := (resource.AgentKind{}).Discover(agentsSource); discoverErr == nil {
			agentCount = len(agents)
		}
	}

	resp := map[string]any{
		"source":        source,
		"skillCount":    len(skills),
		"agentCount":    agentCount,
		"topLevelCount": topLevelCount,
		"targetCount":   targetCount,
		"mode":          mode,
		"version":       versioncheck.Version,
		"trackedRepos":  trackedRepos,
		"isProjectMode": isProjectMode,
		"configDir":     configDir,
	}
	if agentsSource != "" {
		resp["agentsSource"] = agentsSource
	}
	if extrasSource != "" {
		resp["extrasSource"] = extrasSource
	}
	if isProjectMode {
		resp["projectRoot"] = projectRoot
	}

	writeJSON(w, resp)
}

func buildTrackedReposWithOptions(sourceDir string, skills []sync.DiscoveredSkill, opts sourcewalk.Options) ([]trackedRepoItem, error) {
	repoNames, err := install.GetTrackedReposWithOptions(sourceDir, opts)
	if err != nil || len(repoNames) == 0 {
		if opts.Follow.Err() != nil {
			return nil, opts.Follow.Err()
		}
		return []trackedRepoItem{}, nil
	}

	items := make([]trackedRepoItem, 0, len(repoNames))
	for _, repoName := range repoNames {
		repoPath := filepath.Join(sourceDir, repoName)

		// Count skills belonging to this repo
		skillCount := 0
		for _, sk := range skills {
			if sk.IsInRepo && strings.HasPrefix(sk.RelPath, repoName+"/") {
				skillCount++
			}
		}

		// Check git dirty status
		dirty, _ := git.IsDirty(repoPath)

		items = append(items, trackedRepoItem{
			Name:       repoName,
			SkillCount: skillCount,
			Dirty:      dirty,
		})
	}
	return items, nil
}
