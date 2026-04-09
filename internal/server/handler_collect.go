package server

import (
	"encoding/json"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"time"

	ssync "skillshare/internal/sync"
)

// --- Scan types ---

type localSkillItem struct {
	Name       string `json:"name"`
	Kind       string `json:"kind,omitempty"`
	Path       string `json:"path"`
	TargetName string `json:"targetName"`
	Size       int64  `json:"size"`
	ModTime    string `json:"modTime"`
}

type scanTarget struct {
	TargetName string           `json:"targetName"`
	Skills     []localSkillItem `json:"skills"`
}

// --- Collect types ---

type collectSkillRef struct {
	Name       string `json:"name"`
	TargetName string `json:"targetName"`
	Kind       string `json:"kind,omitempty"`
}

// handleCollectScan scans targets for local (non-symlinked) skills and/or agents.
// GET /api/collect/scan?target=<name>&kind=skill|agent  (both optional)
// When kind is omitted, scans for both skills and agents.
func (s *Server) handleCollectScan(w http.ResponseWriter, r *http.Request) {
	// Snapshot config under RLock, then release before I/O.
	s.mu.RLock()
	source := s.cfg.Source
	globalMode := s.cfg.Mode
	targets := s.cloneTargets()
	agentsSource := s.agentsSource()
	s.mu.RUnlock()

	filterTarget := r.URL.Query().Get("target")
	kind := r.URL.Query().Get("kind")
	if kind != "" && kind != kindSkill && kind != kindAgent {
		writeError(w, http.StatusBadRequest, "invalid kind: must be 'skill', 'agent', or empty")
		return
	}

	// Collect items per target, merging skills and agents.
	targetItems := make(map[string][]localSkillItem)
	totalCount := 0

	// --- Skill scan ---
	if kind != kindAgent {
		for name, target := range targets {
			if filterTarget != "" && filterTarget != name {
				continue
			}

			sc := target.SkillsConfig()
			mode := ssync.EffectiveMode(sc.Mode)
			if sc.Mode == "" && globalMode != "" {
				mode = globalMode
			}
			locals, err := ssync.FindLocalSkills(sc.Path, source, mode)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "scan failed for "+name+": "+err.Error())
				return
			}

			for _, sk := range locals {
				targetItems[name] = append(targetItems[name], localSkillItem{
					Name:       sk.Name,
					Kind:       kindSkill,
					Path:       sk.Path,
					TargetName: name,
					Size:       ssync.CalculateDirSize(sk.Path),
					ModTime:    sk.ModTime.Format(time.RFC3339),
				})
			}
		}
	}

	// --- Agent scan ---
	if kind != kindSkill {
		builtinAgents := s.builtinAgentTargets()
		for name, target := range targets {
			if filterTarget != "" && filterTarget != name {
				continue
			}
			agentPath := resolveAgentPath(target, builtinAgents, name)
			if agentPath == "" || agentsSource == "" {
				continue
			}
			localAgents, err := ssync.FindLocalAgents(agentPath, agentsSource)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "agent scan failed for "+name+": "+err.Error())
				return
			}
			for _, ag := range localAgents {
				var size int64
				var modTime string
				if info, err := os.Stat(ag.Path); err == nil {
					size = info.Size()
					modTime = info.ModTime().Format(time.RFC3339)
				}
				targetItems[name] = append(targetItems[name], localSkillItem{
					Name:       ag.Name,
					Kind:       kindAgent,
					Path:       ag.Path,
					TargetName: name,
					Size:       size,
					ModTime:    modTime,
				})
			}
		}
	}

	// Build response from merged map.
	var scanTargets []scanTarget
	for name := range targets {
		items := targetItems[name]
		if len(items) == 0 && kind != "" {
			// When filtering by kind, skip targets with no items of that kind.
			continue
		}
		totalCount += len(items)
		if items == nil {
			items = []localSkillItem{}
		}
		scanTargets = append(scanTargets, scanTarget{
			TargetName: name,
			Skills:     items,
		})
	}

	if scanTargets == nil {
		scanTargets = []scanTarget{}
	}

	writeJSON(w, map[string]any{
		"targets":    scanTargets,
		"totalCount": totalCount,
	})
}

// handleCollect pulls selected local skills and/or agents from targets to source.
// POST /api/collect  { skills: [{name, targetName, kind?}], force: bool }
// Items with kind="agent" are pulled as agents; others as skills (backward compat).
func (s *Server) handleCollect(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	var body struct {
		Skills []collectSkillRef `json:"skills"`
		Force  bool              `json:"force"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if len(body.Skills) == 0 {
		writeError(w, http.StatusBadRequest, "no skills specified")
		return
	}

	// Split items by kind.
	var skillRefs, agentRefs []collectSkillRef
	for _, ref := range body.Skills {
		if ref.Kind == kindAgent {
			agentRefs = append(agentRefs, ref)
		} else {
			skillRefs = append(skillRefs, ref)
		}
	}

	opts := ssync.PullOptions{Force: body.Force}

	// Merged results across skills and agents.
	var allPulled, allSkipped []string
	allFailed := make(map[string]error)
	var skillsPulled, agentsPulled int

	// --- Pull skills ---
	if len(skillRefs) > 0 {
		var resolved []ssync.LocalSkillInfo
		for _, ref := range skillRefs {
			target, ok := s.cfg.Targets[ref.TargetName]
			if !ok {
				writeError(w, http.StatusBadRequest, "unknown target: "+ref.TargetName)
				return
			}

			skillPath := filepath.Join(target.SkillsConfig().Path, ref.Name)
			info, err := os.Lstat(skillPath)
			if err != nil {
				writeError(w, http.StatusBadRequest, "skill not found: "+ref.Name+" in "+ref.TargetName)
				return
			}
			if info.Mode()&os.ModeSymlink != 0 {
				writeError(w, http.StatusBadRequest, "skill is a symlink (not local): "+ref.Name)
				return
			}
			if !info.IsDir() {
				writeError(w, http.StatusBadRequest, "skill is not a directory: "+ref.Name)
				return
			}

			resolved = append(resolved, ssync.LocalSkillInfo{
				Name:       ref.Name,
				Path:       skillPath,
				TargetName: ref.TargetName,
			})
		}

		result, err := ssync.PullSkills(resolved, s.cfg.Source, opts)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "collect failed: "+err.Error())
			return
		}
		skillsPulled = len(result.Pulled)
		allPulled = append(allPulled, result.Pulled...)
		allSkipped = append(allSkipped, result.Skipped...)
		maps.Copy(allFailed, result.Failed)
	}

	// --- Pull agents ---
	if len(agentRefs) > 0 {
		builtinAgents := s.builtinAgentTargets()
		agentsSource := s.agentsSource()

		var resolved []ssync.LocalAgentInfo
		for _, ref := range agentRefs {
			target, ok := s.cfg.Targets[ref.TargetName]
			if !ok {
				writeError(w, http.StatusBadRequest, "unknown target: "+ref.TargetName)
				return
			}

			agentPath := resolveAgentPath(target, builtinAgents, ref.TargetName)
			if agentPath == "" {
				writeError(w, http.StatusBadRequest, "no agent path for target: "+ref.TargetName)
				return
			}

			filePath := filepath.Join(agentPath, ref.Name)
			info, err := os.Lstat(filePath)
			if err != nil {
				writeError(w, http.StatusBadRequest, "agent not found: "+ref.Name+" in "+ref.TargetName)
				return
			}
			if info.Mode()&os.ModeSymlink != 0 {
				writeError(w, http.StatusBadRequest, "agent is a symlink (not local): "+ref.Name)
				return
			}

			resolved = append(resolved, ssync.LocalAgentInfo{
				Name:       ref.Name,
				Path:       filePath,
				TargetName: ref.TargetName,
			})
		}

		result, err := ssync.PullAgents(resolved, agentsSource, opts)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "agent collect failed: "+err.Error())
			return
		}
		agentsPulled = len(result.Pulled)
		allPulled = append(allPulled, result.Pulled...)
		allSkipped = append(allSkipped, result.Skipped...)
		maps.Copy(allFailed, result.Failed)
	}

	// Convert Failed map to string values for JSON
	failed := make(map[string]string, len(allFailed))
	for k, v := range allFailed {
		failed[k] = v.Error()
	}

	status := "ok"
	msg := ""
	if len(allFailed) > 0 {
		status = "partial"
		msg = "some items failed to collect"
	}
	s.writeOpsLog("collect", status, start, map[string]any{
		"skills_selected": len(skillRefs),
		"skills_pulled":   skillsPulled,
		"agents_selected": len(agentRefs),
		"agents_pulled":   agentsPulled,
		"total_pulled":    len(allPulled),
		"total_skipped":   len(allSkipped),
		"total_failed":    len(allFailed),
		"force":           body.Force,
		"scope":           "ui",
	}, msg)

	writeJSON(w, map[string]any{
		"pulled":  allPulled,
		"skipped": allSkipped,
		"failed":  failed,
	})
}
