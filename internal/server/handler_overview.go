package server

import (
	"net/http"
	"path/filepath"

	"skillshare/internal/git"
	"skillshare/internal/install"
	"skillshare/internal/resource"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/sync"
	"skillshare/internal/utils"
	versioncheck "skillshare/internal/version"
)

type trackedRepoItem struct {
	Name         string `json:"name"`
	SkillCount   int    `json:"skillCount"`
	VariantCount int    `json:"variantCount"`
	Dirty        bool   `json:"dirty"`
	NoCheckout   bool   `json:"noCheckout,omitempty"`
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	// Snapshot config under RLock, then release before I/O.
	s.mu.RLock()
	source := s.cfg.EffectiveSkillsSource()
	walk := s.skillsWalk()
	agentsSource := s.agentsSource()
	extrasSource := s.cfg.EffectiveExtrasSource()
	if s.IsProjectMode() {
		extrasSource = s.projectCfg.EffectiveExtrasSource(s.projectRoot)
	}
	cfgMode := s.cfg.Mode
	targetCount := len(s.cfg.Targets)
	projectRoot := s.projectRoot
	configDir := filepath.Dir(s.configPath())
	// Copy declaration fields while locked: removals mutate the cached store.
	var store *install.MetadataStore
	if !s.IsProjectMode() && s.skillsStore != nil {
		store = install.NewMetadataStore()
		for key, entry := range s.skillsStore.Entries {
			store.Set(key, copyMetadataEntry(entry))
		}
	}
	s.mu.RUnlock()

	isProjectMode := projectRoot != ""

	// Count skills
	skills, err := sync.DiscoverSourceSkills(source, walk)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Count top-level source entries (for display)
	topLevelCount := 0
	entries, _ := sourcewalk.ReadDir(source, walk)
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
	trackedRepos := buildTrackedRepos(source, skills, walk, store)

	// Count agents
	agentCount := 0
	if agentsSource != "" {
		if agents, discoverErr := (resource.AgentKind{}).Discover(agentsSource); discoverErr == nil {
			agentCount = len(agents)
		}
	}

	counts := sync.CountSkills(skills)
	resp := map[string]any{
		"source":        source,
		"skillCount":    counts.Skills,
		"variantCount":  counts.Variants,
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

func buildTrackedRepos(sourceDir string, skills []sync.DiscoveredSkill, walk sourcewalk.Options, store *install.MetadataStore) []trackedRepoItem {
	repoNames, _ := install.GetTrackedRepos(sourceDir, walk)

	_, counts := sync.CountSkillsByRepo(skills)
	items := make([]trackedRepoItem, 0, len(repoNames))
	for _, repoName := range repoNames {
		repoPath := filepath.Join(sourceDir, repoName)

		// Check git dirty status
		dirty, _ := git.IsDirty(repoPath)

		items = append(items, trackedRepoItem{
			Name:         repoName,
			SkillCount:   counts[repoName].Skills,
			VariantCount: counts[repoName].Variants,
			Dirty:        dirty,
		})
	}
	for _, name := range install.DeclarationsWithoutCheckout(sourceDir, store) {
		items = append(items, trackedRepoItem{Name: name, NoCheckout: true})
	}
	return items
}
