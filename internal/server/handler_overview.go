package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/git"
	"skillshare/internal/install"
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
	cfgMode := s.cfg.Mode
	targetCount := len(s.cfg.Targets)
	projectRoot := s.projectRoot
	s.mu.RUnlock()

	isProjectMode := projectRoot != ""

	// Count skills
	skills, err := sync.DiscoverSourceSkills(source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Count top-level source entries (for display)
	topLevelCount := 0
	entries, _ := os.ReadDir(source)
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
	trackedRepos := buildTrackedRepos(source, skills)

	resp := map[string]any{
		"source":        source,
		"skillCount":    len(skills),
		"topLevelCount": topLevelCount,
		"targetCount":   targetCount,
		"mode":          mode,
		"version":       versioncheck.Version,
		"trackedRepos":  trackedRepos,
		"isProjectMode": isProjectMode,
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
	return items
}
