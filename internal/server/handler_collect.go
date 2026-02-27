package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	ssync "skillshare/internal/sync"
)

// --- Scan types ---

type localSkillItem struct {
	Name       string `json:"name"`
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
}

// handleCollectScan scans targets for local (non-symlinked) skills.
// GET /api/collect/scan?target=<name>  (optional filter)
func (s *Server) handleCollectScan(w http.ResponseWriter, r *http.Request) {
	filterTarget := r.URL.Query().Get("target")

	var scanTargets []scanTarget
	totalCount := 0

	for name, target := range s.cfg.Targets {
		if filterTarget != "" && filterTarget != name {
			continue
		}

		locals, err := ssync.FindLocalSkills(target.Path, s.cfg.Source)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "scan failed for "+name+": "+err.Error())
			return
		}

		items := make([]localSkillItem, 0, len(locals))
		for _, sk := range locals {
			items = append(items, localSkillItem{
				Name:       sk.Name,
				Path:       sk.Path,
				TargetName: name,
				Size:       ssync.CalculateDirSize(sk.Path),
				ModTime:    sk.ModTime.Format(time.RFC3339),
			})
		}

		totalCount += len(items)
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

// handleCollect pulls selected local skills from targets to source.
// POST /api/collect  { skills: [{name, targetName}], force: bool }
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

	// Resolve each skill ref to a LocalSkillInfo
	var resolved []ssync.LocalSkillInfo
	for _, ref := range body.Skills {
		target, ok := s.cfg.Targets[ref.TargetName]
		if !ok {
			writeError(w, http.StatusBadRequest, "unknown target: "+ref.TargetName)
			return
		}

		skillPath := filepath.Join(target.Path, ref.Name)
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

	result, err := ssync.PullSkills(resolved, s.cfg.Source, ssync.PullOptions{
		Force: body.Force,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "collect failed: "+err.Error())
		return
	}

	// Convert Failed map to string values for JSON
	failed := make(map[string]string, len(result.Failed))
	for k, v := range result.Failed {
		failed[k] = v.Error()
	}

	// Invalidate discovery cache — source skills changed
	if len(result.Pulled) > 0 {
		s.cache.Invalidate(s.cfg.Source)
	}

	status := "ok"
	msg := ""
	if len(result.Failed) > 0 {
		status = "partial"
		msg = "some skills failed to collect"
	}
	s.writeOpsLog("collect", status, start, map[string]any{
		"skills_selected": len(body.Skills),
		"skills_pulled":   len(result.Pulled),
		"skills_skipped":  len(result.Skipped),
		"skills_failed":   len(result.Failed),
		"force":           body.Force,
		"scope":           "ui",
	}, msg)

	writeJSON(w, map[string]any{
		"pulled":  result.Pulled,
		"skipped": result.Skipped,
		"failed":  failed,
	})
}
