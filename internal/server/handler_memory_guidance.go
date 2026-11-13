package server

import (
	"errors"
	"net/http"
	"path/filepath"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/instructions"
	"skillshare/internal/memory"
)

// The guidance rules live in internal/memory. This file resolves what only the
// server knows (targets, instruction assignments, shared extras, project
// reach) into memory.GuidanceInput and converts the results to JSON.

// memoryInstructionsByMode returns the guidance block of the current scope in
// every update mode, for copying. Callers hold s.mu.
func (s *Server) memoryInstructionsByMode(root string) map[string]string {
	return map[string]string{memory.ModePassive: memory.Instructions(root, s.projectRoot, memory.ModePassive), memory.ModeActive: memory.Instructions(root, s.projectRoot, memory.ModeActive)}
}

// guidanceInput resolves every target's read chain. Callers hold s.mu.
func (s *Server) guidanceInput() (memory.GuidanceInput, error) {
	root, err := s.memoryRoot()
	if err != nil {
		return memory.GuidanceInput{}, err
	}
	in := memory.GuidanceInput{Root: root, ProjectRoot: s.projectRoot, Config: []any{s.extrasConfig(), s.cfg.Targets}}
	if s.IsProjectMode() {
		agents := filepath.Join(s.projectRoot, instructions.AgentsFile)
		for _, reach := range s.projectReaches() {
			it, _ := config.TargetInstructions(reach.Target, s.cfg.Targets[reach.Target], true)
			reader := memory.GuidanceReader{Name: reach.Target, MaxChars: it.MaxChars}
			if reach.Reads {
				reader.Sources = append(reader.Sources, memory.GuidanceSource{Read: agents, Live: true})
			}
			if reach.How != instructions.ReachDirect && reach.How != instructions.ReachLink {
				reader.Sources = append(reader.Sources, memory.GuidanceSource{Read: filepath.Join(s.projectRoot, it.Path), Live: true})
			}
			in.Readers = append(in.Readers, reader)
		}
		return in, nil
	}
	for _, t := range s.instructionTargets() {
		reader := memory.GuidanceReader{Name: t.Name, MaxChars: t.MaxChars}
		own := memory.GuidanceSource{Read: t.Path, Live: true}
		for _, a := range t.Assigned {
			extra, ok := s.sharedExtra(a.Name)
			if !ok {
				continue
			}
			src := memory.GuidanceSource{Read: filepath.Join(s.extrasSourceDir(extra), extra.File), Shared: a.Name, Live: a.Status == "synced"}
			src.Write = src.Read
			if a.Mode != "import" {
				// A link or copy is the target's own file; edit the source.
				own.Write, own.Shared, own.Live = src.Write, a.Name, src.Live
				if !src.Live {
					own = src
				}
				continue
			}
			reader.Sources = append(reader.Sources, src)
		}
		reader.Sources = append([]memory.GuidanceSource{own}, reader.Sources...)
		// A tool that imports shared files gets the block in the first synced one.
		for i, src := range reader.Sources[1:] {
			if src.Live {
				reader.Dest = i + 1
				break
			}
		}
		in.Readers = append(in.Readers, reader)
	}
	return in, nil
}

// handleMemoryGuidance — GET /api/extras/memory/guidance
func (s *Server) handleMemoryGuidance(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	in, err := s.guidanceInput()
	if err != nil {
		writeMemoryError(w, err)
		return
	}
	writeJSON(w, map[string]any{"scope": in.Scope(), "instructions": s.memoryInstructionsByMode(in.Root), "targets": memory.GuidanceStatus(in)})
}

type guidanceRequest struct {
	Targets []string          `json:"targets"`
	Modes   map[string]string `json:"modes"` // target name to passive or active
	Token   string            `json:"token"`
}

func decodeGuidanceRequest(w http.ResponseWriter, r *http.Request) (guidanceRequest, bool) {
	var body guidanceRequest
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeCodedError(w, http.StatusBadRequest, "memory_guidance_invalid", "invalid JSON body", map[string]string{})
		}
		return body, false
	}
	return body, true
}

// handleMemoryGuidancePlan — POST /api/extras/memory/guidance/plan
func (s *Server) handleMemoryGuidancePlan(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeGuidanceRequest(w, r)
	if !ok {
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	in, err := s.guidanceInput()
	if err != nil {
		writeCodedError(w, http.StatusBadRequest, "memory_guidance_invalid", err.Error(), map[string]string{})
		return
	}
	plan, err := memory.PlanGuidance(in, body.Targets, body.Modes)
	if err != nil {
		writeCodedError(w, http.StatusBadRequest, "memory_guidance_invalid", err.Error(), map[string]string{})
		return
	}
	writeJSON(w, plan)
}

// handleMemoryGuidanceApply — POST /api/extras/memory/guidance/apply
// Applies a reviewed plan only if recomputing it gives the same token. Each
// existing file is backed up before it is changed; copies of a changed shared
// file are synced.
func (s *Server) handleMemoryGuidanceApply(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	body, ok := decodeGuidanceRequest(w, r)
	if !ok {
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
	in, err := s.guidanceInput()
	if err != nil {
		writeCodedError(w, http.StatusBadRequest, "memory_guidance_invalid", err.Error(), map[string]string{})
		return
	}
	res, err := memory.ApplyGuidance(in, body.Targets, body.Modes, body.Token, func(c memory.GuidanceChange) []memory.GuidanceFailure {
		var out []memory.GuidanceFailure
		if extra, ok := s.sharedExtra(c.Shared); ok && c.Shared != "" {
			for _, result := range s.syncSharedCopies(extra) {
				if result.Error != "" {
					out = append(out, memory.GuidanceFailure{Path: result.Target, Err: errors.New(result.Error)})
				}
			}
		}
		return out
	})
	if errors.Is(err, memory.ErrGuidanceStale) {
		writeCodedError(w, http.StatusConflict, "memory_guidance_stale", err.Error(), map[string]string{})
		return
	}
	if err != nil {
		writeCodedError(w, http.StatusBadRequest, "memory_guidance_invalid", err.Error(), map[string]string{})
		return
	}
	type failure struct {
		Path  string `json:"path"`
		Error string `json:"error"`
		Code  string `json:"code,omitempty"`
	}
	applied, failures := res.Applied, []failure{}
	for _, f := range res.Failures {
		code := ""
		if errors.Is(f.Err, memory.ErrGuidanceStale) {
			code = "memory_guidance_stale"
		}
		failures = append(failures, failure{Path: f.Path, Error: f.Err.Error(), Code: code})
	}
	status, msg := "ok", ""
	if len(failures) > 0 {
		status, msg = "partial", failures[0].Path+": "+failures[0].Error
	}
	s.writeOpsLog("memory-guidance", status, start, map[string]any{"targets": body.Targets, "modes": body.Modes, "files": applied, "scope": "ui"}, msg)
	// Syncing copies changes what the chains show, so resolve them again.
	in, _ = s.guidanceInput()
	writeJSON(w, map[string]any{"success": len(failures) == 0, "applied": applied, "errors": failures, "targets": memory.GuidanceStatus(in)})
}
