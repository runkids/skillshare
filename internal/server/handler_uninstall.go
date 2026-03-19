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
	Force bool     `json:"force"`
}

type batchUninstallItemResult struct {
	Name         string `json:"name"`
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

	discovered, err := sync.DiscoverSourceSkills(s.cfg.Source)
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
	var repoEntriesToRemove []string
	succeeded, failed := 0, 0
	var firstErr string

	for _, name := range body.Names {
		res := batchUninstallItemResult{Name: name}

		if strings.HasPrefix(name, "_") {
			repoPath := filepath.Join(s.cfg.Source, name)
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

		res.Success = true
		res.MovedToTrash = true
		results = append(results, res)
		succeeded++
	}

	if len(repoEntriesToRemove) > 0 {
		if _, err := install.RemoveFromGitIgnoreBatch(s.cfg.Source, repoEntriesToRemove); err != nil {
			log.Printf("warning: failed to clean .gitignore: %v", err)
		}
	}

	if succeeded > 0 {
		removedSet := make(map[string]bool)
		for _, res := range results {
			if res.Success {
				removedSet[res.Name] = true
			}
		}
		filtered := make([]config.SkillEntry, 0, len(s.registry.Skills))
		for _, entry := range s.registry.Skills {
			fullName := entry.FullName()
			if removedSet[fullName] || removedSet[entry.Name] {
				continue
			}
			// Tracked repos: registry stores group without "_" prefix (e.g., group="team-skills"
			// for repo dir "_team-skills"). Reconstruct the prefixed name to match removedSet.
			if entry.Group != "" && removedSet["_"+entry.Group] {
				continue
			}
			filtered = append(filtered, entry)
		}
		s.registry.Skills = filtered

		regDir := filepath.Dir(config.ConfigPath())
		if s.IsProjectMode() {
			regDir = filepath.Join(s.projectRoot, ".skillshare")
		}
		if err := s.registry.Save(regDir); err != nil {
			log.Printf("warning: failed to save registry: %v", err)
		}

		if s.IsProjectMode() {
			if rErr := config.ReconcileProjectSkills(
				s.projectRoot, s.projectCfg, s.registry, s.cfg.Source); rErr != nil {
				log.Printf("warning: failed to reconcile project skills config: %v", rErr)
			}
		} else {
			if rErr := config.ReconcileGlobalSkills(s.cfg, s.registry); rErr != nil {
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
