package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/git"
	"skillshare/internal/install"
	"skillshare/internal/resource"
	managedhooks "skillshare/internal/resources/hooks"
	managedrules "skillshare/internal/resources/rules"
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
	source := s.cfg.Source
	agentsSource := s.agentsSource()
	extrasSource := s.cfg.ExtrasSource
	if s.IsProjectMode() {
		extrasSource = filepath.Join(s.projectRoot, ".skillshare", "extras")
	}
	cfgMode := s.cfg.Mode
	targetCount := len(s.cfg.Targets)
	projectRoot := s.projectRoot
	s.mu.RUnlock()

	isProjectMode := projectRoot != ""

	skills, err := sync.DiscoverSourceSkills(source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	topLevelCount := 0
	entries, _ := os.ReadDir(source)
	for _, entry := range entries {
		if entry.IsDir() && !utils.IsHidden(entry.Name()) {
			topLevelCount++
		}
	}

	mode := cfgMode
	if mode == "" {
		mode = "merge"
	}

	trackedRepos := buildTrackedRepos(source, skills)

	agentCount := 0
	if agentsSource != "" {
		if agents, discoverErr := (resource.AgentKind{}).Discover(agentsSource); discoverErr == nil {
			agentCount = len(agents)
		}
	}

	managedRuleRecords, err := managedrules.NewStore(s.managedRulesProjectRoot()).List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list managed rules: "+err.Error())
		return
	}
	managedHookRecords, err := managedhooks.NewStore(s.managedHooksProjectRoot()).List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list managed hooks: "+err.Error())
		return
	}

	resp := map[string]any{
		"source":            source,
		"skillCount":        len(skills),
		"agentCount":        agentCount,
		"managedRulesCount": len(managedRuleRecords),
		"managedHooksCount": len(managedHookRecords),
		"topLevelCount":     topLevelCount,
		"targetCount":       targetCount,
		"mode":              mode,
		"version":           versioncheck.Version,
		"trackedRepos":      trackedRepos,
		"isProjectMode":     isProjectMode,
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

func buildTrackedRepos(sourceDir string, skills []sync.DiscoveredSkill) []trackedRepoItem {
	repoNames, err := install.GetTrackedRepos(sourceDir)
	if err != nil || len(repoNames) == 0 {
		return []trackedRepoItem{}
	}

	items := make([]trackedRepoItem, 0, len(repoNames))
	for _, repoName := range repoNames {
		repoPath := filepath.Join(sourceDir, repoName)

		skillCount := 0
		for _, sk := range skills {
			if sk.IsInRepo && strings.HasPrefix(sk.RelPath, repoName+"/") {
				skillCount++
			}
		}

		dirty, _ := git.IsDirty(repoPath)

		items = append(items, trackedRepoItem{
			Name:       repoName,
			SkillCount: skillCount,
			Dirty:      dirty,
		})
	}
	return items
}
