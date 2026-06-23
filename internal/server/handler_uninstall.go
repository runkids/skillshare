package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/git"
	"skillshare/internal/install"
	"skillshare/internal/sync"
	"skillshare/internal/trash"
)

type batchUninstallRequest struct {
	Names []string `json:"names"`
	Kind  string   `json:"kind,omitempty"`
	Force bool     `json:"force"`
}

type batchUninstallItemResult struct {
	Name         string `json:"name"`
	Kind         string `json:"kind,omitempty"`
	Success      bool   `json:"success"`
	MovedToTrash bool   `json:"movedToTrash,omitempty"`
	Error        string `json:"error,omitempty"`
}

type batchUninstallSummary struct {
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
}

func (s *Server) handleBatchUninstall(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	var body batchUninstallRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if len(body.Names) == 0 {
		writeError(w, http.StatusBadRequest, "names array is required and must not be empty")
		return
	}
	if body.Kind != "" && body.Kind != "skill" && body.Kind != "agent" {
		writeError(w, http.StatusBadRequest, "invalid kind: "+body.Kind)
		return
	}

	// Agent-mode batch uninstall
	if body.Kind == "agent" {
		s.handleBatchUninstallAgents(w, body, start)
		return
	}

	// Skill-mode (default) batch uninstall
	s.handleBatchUninstallSkills(w, body, start)
}

func (s *Server) handleBatchUninstallAgents(w http.ResponseWriter, body batchUninstallRequest, start time.Time) {
	agentsSource := s.agentsSource()
	if agentsSource == "" {
		writeError(w, http.StatusInternalServerError, "agents source not configured")
		return
	}

	results := make([]batchUninstallItemResult, 0, len(body.Names))
	var removedNames []string
	succeeded, failed := 0, 0
	var firstErr string

	for _, name := range body.Names {
		res := batchUninstallItemResult{Name: name, Kind: "agent"}

		agent, err := resolveAgentResource(agentsSource, name)
		if err != nil {
			res.Success = false
			res.Error = "agent not found: " + name
			results = append(results, res)
			failed++
			if firstErr == "" {
				firstErr = res.Error
			}
			continue
		}

		displayName := agentMetaKey(agent.RelPath)
		legacySidecar := filepath.Join(filepath.Dir(agent.SourcePath), filepath.Base(displayName)+".skillshare-meta.json")
		if _, err := trash.MoveAgentToTrash(agent.SourcePath, legacySidecar, displayName, s.agentTrashBase()); err != nil {
			res.Success = false
			res.Error = fmt.Sprintf("failed to trash agent: %v", err)
			results = append(results, res)
			failed++
			if firstErr == "" {
				firstErr = res.Error
			}
			continue
		}

		removedNames = append(removedNames, displayName)
		res.Success = true
		res.MovedToTrash = true
		results = append(results, res)
		succeeded++
	}

	if succeeded > 0 && s.agentsStore != nil {
		for _, name := range removedNames {
			s.agentsStore.Remove(name)
		}
		if err := s.agentsStore.Save(agentsSource); err != nil {
			log.Printf("warning: failed to save agent metadata after batch uninstall: %v", err)
		}
	}

	status := "ok"
	if failed > 0 && succeeded > 0 {
		status = "partial"
	} else if failed > 0 {
		status = "error"
	}

	s.writeOpsLog("uninstall", status, start, map[string]any{
		"names": body.Names,
		"kind":  "agent",
		"scope": "ui",
		"count": succeeded,
	}, firstErr)

	writeJSON(w, map[string]any{
		"results": results,
		"summary": batchUninstallSummary{
			Succeeded: succeeded,
			Failed:    failed,
		},
	})
}

func (s *Server) handleBatchUninstallSkills(w http.ResponseWriter, body batchUninstallRequest, start time.Time) {
	// Use DiscoverSourceSkillsAll (not DiscoverSourceSkills) so disabled skills
	// — those listed in .skillignore — are also resolvable. The list handler
	// shows disabled skills, so uninstall must be able to find them too (#190).
	discovered, err := sync.DiscoverSourceSkillsAll(s.cfg.EffectiveSkillsSource())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to discover skills: "+err.Error())
		return
	}

	flatNameMap := make(map[string]*sync.DiscoveredSkill, len(discovered))
	baseNameMap := make(map[string]*sync.DiscoveredSkill, len(discovered))
	for i := range discovered {
		flatNameMap[discovered[i].FlatName] = &discovered[i]
		baseNameMap[filepath.Base(discovered[i].SourcePath)] = &discovered[i]
	}

	results := make([]batchUninstallItemResult, 0, len(body.Names))
	removedPaths := make(map[string]bool) // exact RelPaths of successfully removed items
	var repoEntriesToRemove []string
	succeeded, failed := 0, 0
	var firstErr string

	for _, name := range body.Names {
		res := batchUninstallItemResult{Name: name, Kind: "skill"}

		if strings.HasPrefix(name, "_") {
			repoPath := filepath.Join(s.cfg.EffectiveSkillsSource(), name)
			if !install.IsGitRepo(repoPath) {
				res.Success = false
				res.Error = "not a tracked repository: " + name
				results = append(results, res)
				failed++
				if firstErr == "" {
					firstErr = res.Error
				}
				continue
			}

			if dirty, _ := git.IsDirty(repoPath); !body.Force && dirty {
				res.Success = false
				res.Error = "uncommitted changes (use force to override)"
				results = append(results, res)
				failed++
				if firstErr == "" {
					firstErr = res.Error
				}
				continue
			}

			if _, err := trash.MoveToTrash(repoPath, name, s.trashBase()); err != nil {
				res.Success = false
				res.Error = fmt.Sprintf("failed to trash repo: %v", err)
				results = append(results, res)
				failed++
				if firstErr == "" {
					firstErr = res.Error
				}
				continue
			}

			repoEntriesToRemove = append(repoEntriesToRemove, name)
			removedPaths[name] = true // repo dir name is already the registry path
			res.Success = true
			res.MovedToTrash = true
			results = append(results, res)
			succeeded++
			continue
		}

		skill := flatNameMap[name]
		if skill == nil {
			skill = baseNameMap[name]
		}
		if skill == nil {
			res.Success = false
			res.Error = "skill not found: " + name
			results = append(results, res)
			failed++
			if firstErr == "" {
				firstErr = res.Error
			}
			continue
		}

		if skill.IsInRepo {
			res.Success = false
			res.Error = "skill is inside a tracked repo; uninstall the repo instead"
			results = append(results, res)
			failed++
			if firstErr == "" {
				firstErr = res.Error
			}
			continue
		}

		baseName := filepath.Base(skill.SourcePath)
		if _, err := trash.MoveToTrash(skill.SourcePath, baseName, s.trashBase()); err != nil {
			res.Success = false
			res.Error = fmt.Sprintf("failed to trash skill: %v", err)
			results = append(results, res)
			failed++
			if firstErr == "" {
				firstErr = res.Error
			}
			continue
		}

		removedPaths[skill.RelPath] = true // exact path for registry matching
		res.Success = true
		res.MovedToTrash = true
		results = append(results, res)
		succeeded++
	}

	if len(repoEntriesToRemove) > 0 {
		gitDir := s.gitignoreDir()
		if gitDir != "" {
			entries := repoEntriesToRemove
			if s.IsProjectMode() {
				prefix := s.projectGitignorePrefix()
				entries = make([]string, len(repoEntriesToRemove))
				for i, e := range repoEntriesToRemove {
					entries[i] = prefix + "/" + e
				}
			}
			if _, err := install.RemoveFromGitIgnoreBatch(gitDir, entries); err != nil {
				log.Printf("warning: failed to clean .gitignore: %v", err)
			}
		}
	}

	if succeeded > 0 {
		// removedPaths contains exact RelPaths (e.g. "frontend/vue/vue-best-practices")
		// and repo dir names (e.g. "_team-skills"), collected during the uninstall loop.
		for _, name := range s.skillsStore.List() {
			entry := s.skillsStore.Get(name)
			if entry == nil {
				continue
			}
			if removedPaths[name] {
				s.skillsStore.Remove(name)
				continue
			}
			// Tracked repos: store uses group without "_" prefix (e.g., group="team-skills"
			// for repo dir "_team-skills"). Reconstruct the prefixed name to match removedPaths.
			if entry.Group != "" && removedPaths["_"+entry.Group] {
				s.skillsStore.Remove(name)
				continue
			}
			// When a group directory is uninstalled, also remove its member skills
			memberOfRemoved := false
			for rp := range removedPaths {
				if strings.HasPrefix(name, rp+"/") {
					memberOfRemoved = true
					break
				}
			}
			if memberOfRemoved {
				s.skillsStore.Remove(name)
			}
		}

		if err := s.skillsStore.Save(s.cfg.EffectiveSkillsSource()); err != nil {
			log.Printf("warning: failed to save metadata: %v", err)
		}

		if s.IsProjectMode() {
			if rErr := config.ReconcileProjectSkills(
				s.projectRoot, s.projectCfg, s.skillsStore, s.cfg.EffectiveSkillsSource()); rErr != nil {
				log.Printf("warning: failed to reconcile project skills config: %v", rErr)
			}
		} else {
			if rErr := config.ReconcileGlobalSkills(s.cfg, s.skillsStore); rErr != nil {
				log.Printf("warning: failed to reconcile global skills config: %v", rErr)
			}
		}
	}

	status := "ok"
	if failed > 0 && succeeded > 0 {
		status = "partial"
	} else if failed > 0 {
		status = "error"
	}

	s.writeOpsLog("uninstall", status, start, map[string]any{
		"names": body.Names,
		"force": body.Force,
		"scope": "ui",
		"count": succeeded,
	}, firstErr)

	writeJSON(w, map[string]any{
		"results": results,
		"summary": batchUninstallSummary{
			Succeeded: succeeded,
			Failed:    failed,
		},
	})
}
