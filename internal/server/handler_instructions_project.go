package server

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/instructions"
)

// In project mode every target reads the one ./AGENTS.md, so there is nothing
// to share or sync; a few targets need a small file (a shim) to reach it.

func (s *Server) requireProjectInstructions(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.IsProjectMode() {
			writeCodedError(w, http.StatusBadRequest, "instructions_project_required", "the project AGENTS.md is available in project mode", map[string]string{})
			return
		}
		next(w, r)
	}
}

// projectReaches reports how each configured target reaches ./AGENTS.md.
// Callers must hold s.mu.
func (s *Server) projectReaches() []instructions.Reach {
	names := make([]string, 0, len(s.cfg.Targets))
	for name := range s.cfg.Targets {
		names = append(names, name)
	}
	sort.Strings(names)
	out := []instructions.Reach{}
	for _, name := range names {
		if it, ok := config.TargetInstructions(name, s.cfg.Targets[name], true); ok {
			out = append(out, instructions.ProjectReach(s.projectRoot, name, it))
		}
	}
	return out
}

// handleGetProjectInstructions — GET /api/instructions/project
func (s *Server) handleGetProjectInstructions(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	path := filepath.Join(s.projectRoot, instructions.AgentsFile)
	resp := map[string]any{"path": path, "exists": false, "content": "", "size": 0, "targets": s.projectReaches()}
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		data, err := readLimited(path)
		if err != nil {
			writeCodedError(w, http.StatusInternalServerError, "instructions_read_failed", err.Error(), map[string]string{"detail": err.Error()})
			return
		}
		resp["exists"], resp["content"], resp["size"] = true, string(data), info.Size()
	}
	writeJSON(w, resp)
}

// handlePutProjectInstructions — PUT /api/instructions/project
func (s *Server) handlePutProjectInstructions(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var body struct {
		Content string `json:"content"`
	}
	if err := decodeJSON(w, r, &body, maxInstructionsBytes+4096); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeCodedError(w, http.StatusBadRequest, "instructions_invalid_json", "invalid JSON body", map[string]string{})
		}
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, lockErr := s.lockRemoteExtraMutation()
	if lockErr != nil {
		writeCodedError(w, http.StatusConflict, "instructions_write_failed", lockErr.Error(), map[string]string{"detail": lockErr.Error()})
		return
	}
	defer unlock()
	path := filepath.Join(s.projectRoot, instructions.AgentsFile)
	if err := writeInstructionsFile(path, body.Content); err != nil {
		s.writeOpsLog("instructions-edit", "error", start, map[string]any{"path": path, "scope": "ui"}, err.Error())
		writeCodedError(w, http.StatusInternalServerError, "instructions_write_failed", err.Error(), map[string]string{"detail": err.Error()})
		return
	}
	s.writeOpsLog("instructions-edit", "ok", start, map[string]any{"path": path, "scope": "ui"}, "")
	writeJSON(w, map[string]any{"success": true, "path": path})
}

// handleProjectInstructionsShim — POST /api/instructions/project/shim
// Adds the shim a target needs to read ./AGENTS.md.
func (s *Server) handleProjectInstructionsShim(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var body struct {
		Target string `json:"target"`
	}
	decodeErr := decodeJSON(w, r, &body, defaultJSONBodyLimit)
	if errors.Is(decodeErr, errBodyTooLarge) {
		return
	}
	if decodeErr != nil || body.Target == "" {
		writeCodedError(w, http.StatusBadRequest, "instructions_target_required", "target is required", map[string]string{})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, lockErr := s.lockRemoteExtraMutation()
	if lockErr != nil {
		writeCodedError(w, http.StatusConflict, "instructions_write_failed", lockErr.Error(), map[string]string{"detail": lockErr.Error()})
		return
	}
	defer unlock()
	tc, found := s.cfg.Targets[body.Target]
	if !found {
		writeCodedError(w, http.StatusNotFound, "instructions_target_not_found", "target not found: "+body.Target, map[string]string{"target": body.Target})
		return
	}
	it, ok := config.TargetInstructions(body.Target, tc, true)
	if !ok {
		writeCodedError(w, http.StatusBadRequest, "instructions_no_file", body.Target+" has no instruction file", map[string]string{"target": body.Target})
		return
	}
	reach := instructions.ProjectReach(s.projectRoot, body.Target, it)
	if reach.Shim == "" {
		writeCodedError(w, http.StatusBadRequest, "instructions_no_change_needed", body.Target+" needs no change to read "+instructions.AgentsFile, map[string]string{"target": body.Target})
		return
	}
	args := map[string]any{"target": body.Target, "shim": reach.Shim, "scope": "ui"}
	if err := instructions.ApplyShim(s.projectRoot, it, reach.Shim); err != nil {
		s.writeOpsLog("instructions-shim", "error", start, args, err.Error())
		writeCodedError(w, http.StatusInternalServerError, "instructions_write_failed", err.Error(), map[string]string{"detail": err.Error()})
		return
	}
	s.writeOpsLog("instructions-shim", "ok", start, args, "")
	writeJSON(w, map[string]any{"success": true, "target": instructions.ProjectReach(s.projectRoot, body.Target, it)})
}
