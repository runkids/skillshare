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
	// Count skills
	skills, err := s.cache.Discover(s.cfg.Source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Count top-level source entries (for display)
	topLevelCount := 0
	entries, _ := os.ReadDir(s.cfg.Source)
	for _, e := range entries {
		if e.IsDir() && !utils.IsHidden(e.Name()) {
			topLevelCount++
		}
	}

	mode := s.cfg.Mode
	if mode == "" {
		mode = "merge"
	}

	// Tracked repos
	trackedRepos := buildTrackedRepos(s.cfg.Source, skills)

	resp := map[string]any{
		"source":        s.cfg.Source,
		"skillCount":    len(skills),
		"topLevelCount": topLevelCount,
		"targetCount":   len(s.cfg.Targets),
		"mode":          mode,
		"version":       versioncheck.Version,
		"trackedRepos":  trackedRepos,
		"isProjectMode": s.IsProjectMode(),
	}
	if s.IsProjectMode() {
		resp["projectRoot"] = s.projectRoot
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
