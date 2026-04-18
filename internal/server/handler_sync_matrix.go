package server

import (
	"encoding/json"
	"net/http"

	"skillshare/internal/config"
	"skillshare/internal/resource"
	ssync "skillshare/internal/sync"
)

type syncMatrixEntry struct {
	Skill        string            `json:"skill"`
	Target       string            `json:"target"`
	Status       string            `json:"status"`
	Reason       string            `json:"reason"`
	ReasonCode   string            `json:"reasonCode,omitempty"`
	ReasonParams map[string]string `json:"reasonParams,omitempty"`
	Kind         string            `json:"kind,omitempty"`
}

func syncMatrixReason(status, reason string) (string, map[string]string) {
	if reason == "symlink mode — filters not applicable" {
		return "sync_matrix.symlink_filters_not_applicable", nil
	}
	switch status {
	case "synced":
		return "sync_matrix.synced", nil
	case "excluded":
		return "sync_matrix.excluded", map[string]string{"pattern": reason}
	case "not_included":
		return "sync_matrix.not_included", nil
	case "skill_target_mismatch":
		return "sync_matrix.skill_target_mismatch", nil
	default:
		return "", nil
	}
}

func newSyncMatrixEntry(skill, target, status, reason, kind string) syncMatrixEntry {
	reasonCode, reasonParams := syncMatrixReason(status, reason)
	return syncMatrixEntry{
		Skill:        skill,
		Target:       target,
		Status:       status,
		Reason:       reason,
		ReasonCode:   reasonCode,
		ReasonParams: reasonParams,
		Kind:         kind,
	}
}

func (s *Server) handleSyncMatrix(w http.ResponseWriter, r *http.Request) {
	// Snapshot config under RLock, then release before I/O.
	s.mu.RLock()
	source := s.cfg.Source
	agentsSource := s.agentsSource()
	targets := s.cloneTargets()
	s.mu.RUnlock()

	skills, err := ssync.DiscoverSourceSkills(source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to discover skills: "+err.Error())
		return
	}

	var agents []resource.DiscoveredResource
	if agentsSource != "" {
		discovered, _ := resource.AgentKind{}.Discover(agentsSource)
		agents = resource.ActiveAgents(discovered)
	}
	var builtinAgents map[string]config.TargetConfig
	if s.IsProjectMode() {
		builtinAgents = config.ProjectAgentTargets()
	} else {
		builtinAgents = config.DefaultAgentTargets()
	}

	targetFilter := r.URL.Query().Get("target")

	var entries []syncMatrixEntry
	for name, target := range targets {
		if targetFilter != "" && name != targetFilter {
			continue
		}
		// Skills
		sc := target.SkillsConfig()
		if sc.Mode == "symlink" {
			for _, skill := range skills {
				entries = append(entries, newSyncMatrixEntry(skill.FlatName, name, "na", "symlink mode — filters not applicable", ""))
			}
		} else {
			for _, skill := range skills {
				status, reason := ssync.ClassifySkillForTarget(skill.FlatName, skill.Targets, name, sc.Include, sc.Exclude)
				entries = append(entries, newSyncMatrixEntry(skill.FlatName, name, status, reason, ""))
			}
		}
		// Agents — resolve path from user config or builtin defaults
		ac := target.AgentsConfig()
		agentPath := ac.Path
		if agentPath == "" {
			if builtin, ok := builtinAgents[name]; ok {
				agentPath = builtin.Path
			}
		}
		if agentPath == "" || len(agents) == 0 {
			continue
		}
		agentMode := ac.Mode
		if agentMode == "" {
			agentMode = "merge"
		}
		if agentMode == "symlink" {
			for _, agent := range agents {
				entries = append(entries, newSyncMatrixEntry(agent.FlatName, name, "na", "symlink mode — filters not applicable", "agent"))
			}
		} else {
			for _, agent := range agents {
				status, reason := ssync.ClassifySkillForTarget(agent.FlatName, agent.Targets, name, ac.Include, ac.Exclude)
				entries = append(entries, newSyncMatrixEntry(agent.FlatName, name, status, reason, "agent"))
			}
		}
	}

	writeJSON(w, map[string]any{"entries": entries})
}

func (s *Server) handleSyncMatrixPreview(w http.ResponseWriter, r *http.Request) {
	// Snapshot config under RLock, then release before I/O.
	s.mu.RLock()
	source := s.cfg.Source
	agentsSource := s.agentsSource()
	targets := s.cloneTargets()
	s.mu.RUnlock()

	var body struct {
		Target       string   `json:"target"`
		Include      []string `json:"include"`
		Exclude      []string `json:"exclude"`
		AgentInclude []string `json:"agent_include"`
		AgentExclude []string `json:"agent_exclude"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if body.Target == "" {
		writeError(w, http.StatusBadRequest, "target is required")
		return
	}

	// Validate skill patterns
	if _, err := ssync.FilterSkills(nil, body.Include, nil); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := ssync.FilterSkills(nil, nil, body.Exclude); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// Validate agent patterns
	if _, err := ssync.FilterSkills(nil, body.AgentInclude, nil); err != nil {
		writeError(w, http.StatusBadRequest, "invalid agent include pattern: "+err.Error())
		return
	}
	if _, err := ssync.FilterSkills(nil, nil, body.AgentExclude); err != nil {
		writeError(w, http.StatusBadRequest, "invalid agent exclude pattern: "+err.Error())
		return
	}

	skills, err := ssync.DiscoverSourceSkills(source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to discover skills: "+err.Error())
		return
	}

	var entries []syncMatrixEntry
	for _, skill := range skills {
		status, reason := ssync.ClassifySkillForTarget(skill.FlatName, skill.Targets, body.Target, body.Include, body.Exclude)
		entries = append(entries, newSyncMatrixEntry(skill.FlatName, body.Target, status, reason, ""))
	}

	// Agents — resolve path from config or builtin defaults
	target, ok := targets[body.Target]
	if ok && agentsSource != "" {
		ac := target.AgentsConfig()
		agentPath := ac.Path
		if agentPath == "" {
			var previewBuiltin map[string]config.TargetConfig
			if s.IsProjectMode() {
				previewBuiltin = config.ProjectAgentTargets()
			} else {
				previewBuiltin = config.DefaultAgentTargets()
			}
			if builtin, found := previewBuiltin[body.Target]; found {
				agentPath = builtin.Path
			}
		}
		if agentPath != "" {
			discovered, _ := resource.AgentKind{}.Discover(agentsSource)
			agents := resource.ActiveAgents(discovered)
			agentMode := ac.Mode
			if agentMode == "" {
				agentMode = "merge"
			}
			if agentMode == "symlink" {
				for _, agent := range agents {
					entries = append(entries, newSyncMatrixEntry(agent.FlatName, body.Target, "na", "symlink mode — filters not applicable", "agent"))
				}
			} else {
				for _, agent := range agents {
					status, reason := ssync.ClassifySkillForTarget(agent.FlatName, agent.Targets, body.Target, body.AgentInclude, body.AgentExclude)
					entries = append(entries, newSyncMatrixEntry(agent.FlatName, body.Target, status, reason, "agent"))
				}
			}
		}
	}

	writeJSON(w, map[string]any{"entries": entries})
}
