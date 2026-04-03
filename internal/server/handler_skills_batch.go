package server

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/install"
	ssync "skillshare/internal/sync"
	"skillshare/internal/utils"
)

// matchesFolder reports whether relPath belongs to the given folder filter.
// folder="" → root-level only (dir == ".")
// folder="*" → all skills
// otherwise → dir == folder || dir has folder/ prefix
func matchesFolder(relPath, folder string) bool {
	if folder == "*" {
		return true
	}
	dir := filepath.ToSlash(filepath.Dir(relPath))
	if folder == "" {
		return dir == "."
	}
	return dir == folder || strings.HasPrefix(dir, folder+"/")
}

type batchSetTargetsRequest struct {
	Folder string `json:"folder"`
	Target string `json:"target"`
}

type batchSetTargetsResponse struct {
	Updated int      `json:"updated"`
	Skipped int      `json:"skipped"`
	Errors  []string `json:"errors"`
}

// handleBatchSetTargets handles POST /api/skills/batch/targets.
// Sets or removes the target for all skills in a folder.
func (s *Server) handleBatchSetTargets(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req batchSetTargetsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	// Reject path traversal in folder
	if strings.Contains(req.Folder, "..") {
		writeError(w, http.StatusBadRequest, "invalid folder: path traversal not allowed")
		return
	}
	folder := filepath.ToSlash(filepath.Clean(req.Folder))
	if req.Folder == "" {
		folder = ""
	}

	// Snapshot config under read lock, then discover without holding the lock.
	s.mu.RLock()
	source := s.cfg.Source
	s.mu.RUnlock()

	discovered, err := ssync.DiscoverSourceSkillsAll(source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to discover skills: "+err.Error())
		return
	}

	var updated, skipped int
	var errors []string

	// Collect paths that need meta hash refresh (outside the lock).
	var updatedPaths []string

	// Acquire write lock only for the file-write loop.
	s.mu.Lock()
	for _, d := range discovered {
		relPath := filepath.ToSlash(d.RelPath)
		if !matchesFolder(relPath, folder) {
			continue
		}

		// Skip disabled skills and tracked-repo members
		if d.Disabled || d.IsInRepo {
			skipped++
			continue
		}

		skillMDPath := filepath.Join(d.SourcePath, "SKILL.md")
		var values []string
		if req.Target != "" {
			values = []string{req.Target}
		}

		if err := utils.SetFrontmatterList(skillMDPath, "metadata.targets", values); err != nil {
			errors = append(errors, d.FlatName+": "+err.Error())
			continue
		}

		updatedPaths = append(updatedPaths, d.SourcePath)
		updated++
	}
	s.mu.Unlock()

	// Recompute file hashes outside the lock so reads aren't blocked.
	for _, p := range updatedPaths {
		install.RefreshMetaHashes(p)
	}

	s.writeOpsLog("batch-set-targets", "ok", start, map[string]any{
		"folder":  req.Folder,
		"target":  req.Target,
		"updated": updated,
		"skipped": skipped,
		"scope":   "ui",
	}, "")

	if errors == nil {
		errors = []string{}
	}
	writeJSON(w, batchSetTargetsResponse{
		Updated: updated,
		Skipped: skipped,
		Errors:  errors,
	})
}

type setSkillTargetsRequest struct {
	Target string `json:"target"`
}

// handleSetSkillTargets handles PATCH /api/skills/{name}/targets.
// Sets or removes the target for a single skill.
func (s *Server) handleSetSkillTargets(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	name := r.PathValue("name")

	var req setSkillTargetsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	s.mu.RLock()
	source := s.cfg.Source
	s.mu.RUnlock()

	discovered, err := ssync.DiscoverSourceSkillsAll(source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to discover skills: "+err.Error())
		return
	}

	for _, d := range discovered {
		baseName := filepath.Base(d.SourcePath)
		if d.FlatName != name && baseName != name {
			continue
		}

		if d.IsInRepo {
			writeError(w, http.StatusBadRequest, "cannot set target on tracked-repo skill; manage targets on the repo instead")
			return
		}

		skillMDPath := filepath.Join(d.SourcePath, "SKILL.md")
		var values []string
		if req.Target != "" {
			values = []string{req.Target}
		}

		s.mu.Lock()
		err := utils.SetFrontmatterList(skillMDPath, "metadata.targets", values)
		s.mu.Unlock()

		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update skill: "+err.Error())
			return
		}

		install.RefreshMetaHashes(d.SourcePath)

		s.writeOpsLog("set-skill-targets", "ok", start, map[string]any{
			"name":   name,
			"target": req.Target,
			"scope":  "ui",
		}, "")

		writeJSON(w, map[string]any{"success": true})
		return
	}

	writeError(w, http.StatusNotFound, "skill not found: "+name)
}
