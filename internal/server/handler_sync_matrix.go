package server

import (
	"encoding/json"
	"net/http"

	ssync "skillshare/internal/sync"
)

type syncMatrixEntry struct {
	Skill  string `json:"skill"`
	Target string `json:"target"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

func (s *Server) handleSyncMatrix(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	skills, err := ssync.DiscoverSourceSkills(s.cfg.Source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to discover skills: "+err.Error())
		return
	}

	targetFilter := r.URL.Query().Get("target")

	var entries []syncMatrixEntry
	for name, target := range s.cfg.Targets {
		if targetFilter != "" && name != targetFilter {
			continue
		}
		if target.Mode == "symlink" {
			for _, skill := range skills {
				entries = append(entries, syncMatrixEntry{
					Skill:  skill.FlatName,
					Target: name,
					Status: "na",
					Reason: "symlink mode — filters not applicable",
				})
			}
			continue
		}
		for _, skill := range skills {
			status, reason := ssync.ClassifySkillForTarget(skill.FlatName, skill.Targets, name, target.Include, target.Exclude)
			entries = append(entries, syncMatrixEntry{
				Skill:  skill.FlatName,
				Target: name,
				Status: status,
				Reason: reason,
			})
		}
	}

	writeJSON(w, map[string]any{"entries": entries})
}

func (s *Server) handleSyncMatrixPreview(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var body struct {
		Target  string   `json:"target"`
		Include []string `json:"include"`
		Exclude []string `json:"exclude"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if body.Target == "" {
		writeError(w, http.StatusBadRequest, "target is required")
		return
	}

	// Validate patterns before discovering skills
	if _, err := ssync.FilterSkills(nil, body.Include, nil); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := ssync.FilterSkills(nil, nil, body.Exclude); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	skills, err := ssync.DiscoverSourceSkills(s.cfg.Source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to discover skills: "+err.Error())
		return
	}

	var entries []syncMatrixEntry
	for _, skill := range skills {
		status, reason := ssync.ClassifySkillForTarget(skill.FlatName, skill.Targets, body.Target, body.Include, body.Exclude)
		entries = append(entries, syncMatrixEntry{
			Skill:  skill.FlatName,
			Target: body.Target,
			Status: status,
			Reason: reason,
		})
	}

	writeJSON(w, map[string]any{"entries": entries})
}
