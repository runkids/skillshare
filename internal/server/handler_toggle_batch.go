package server

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"skillshare/internal/toggle"
)

type batchToggleRequest struct {
	Names  []string `json:"names"`
	Kind   string   `json:"kind"`
	Enable bool     `json:"enable"`
}

type batchToggleItemResult struct {
	Name     string `json:"name"`
	Success  bool   `json:"success"`
	Disabled bool   `json:"disabled"`
	Error    string `json:"error,omitempty"`
}

type batchToggleResponse struct {
	Results []batchToggleItemResult `json:"results"`
	Summary struct {
		Updated   int `json:"updated"`
		Unchanged int `json:"unchanged"`
		Failed    int `json:"failed"`
	} `json:"summary"`
}

// handleBatchToggleSkills handles POST /api/resources/batch/toggle.
// Enables or disables many skills or agents at once by adding/removing their
// paths in the shared .skillignore / .agentignore file. One kind per request.
func (s *Server) handleBatchToggleSkills(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req batchToggleRequest
	if err := decodeJSON(w, r, &req, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		}
		return
	}
	if req.Kind != "" && req.Kind != "agent" && req.Kind != "skill" {
		writeError(w, http.StatusBadRequest, "invalid kind: "+req.Kind)
		return
	}

	// Snapshot config under read lock, then discover without holding the lock.
	s.mu.RLock()
	source := s.cfg.EffectiveSkillsSource()
	agentsSource := s.agentsSource()
	s.mu.RUnlock()

	// Discover once and look names up the way the single-resource toggle does.
	src := toggle.Source{Agent: req.Kind == "agent", Root: source, Walk: s.skillsWalk()}
	if src.Agent {
		if agentsSource == "" {
			writeError(w, http.StatusNotFound, "no agents source configured")
			return
		}
		src.Root = agentsSource
	}
	items, err := src.Discover()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var resp batchToggleResponse
	resp.Results = make([]batchToggleItemResult, len(req.Names))
	fail := func(i int, disabled bool, msg string) {
		resp.Results[i] = batchToggleItemResult{Name: req.Names[i], Disabled: disabled, Error: msg}
		resp.Summary.Failed++
	}
	// Each name to write, so its outcome can be checked after writing.
	written := map[int]string{}
	var relPaths []string
	for i, name := range req.Names {
		matches := items.Named(name)
		if len(matches) == 0 {
			fail(i, false, "resource not found: "+name)
			continue
		}
		if req.Enable && !matches[0].Disabled {
			resp.Results[i] = batchToggleItemResult{Name: name, Success: true}
			resp.Summary.Unchanged++
			continue
		}
		written[i] = matches[0].RelPath
		relPaths = append(relPaths, matches[0].RelPath)
	}

	// Hold the write lock while writing and re-checking the outcome.
	if len(relPaths) > 0 {
		s.mu.Lock()
		res, err := src.Apply(req.Enable, relPaths, relPaths)
		s.mu.Unlock()
		counted := map[string]bool{}
		for i := range req.Names {
			relPath, ok := written[i]
			if !ok {
				continue
			}
			if err != nil {
				fail(i, req.Enable, err.Error())
				continue
			}
			// A written rule can still be overridden by another one, such as
			// a "!" line in the .local file; Apply re-checks it as discovery sees it.
			if msg := res.Items[slices.Index(relPaths, relPath)].Blocked; msg != "" {
				fail(i, req.Enable, msg)
				continue
			}
			resp.Results[i] = batchToggleItemResult{Name: req.Names[i], Success: true, Disabled: !req.Enable}
			if res.Changed[relPath] && !counted[relPath] {
				counted[relPath] = true
				resp.Summary.Updated++
			} else {
				resp.Summary.Unchanged++
			}
		}
	}

	status, msg := "ok", ""
	if resp.Summary.Failed > 0 {
		status, msg = "partial", fmt.Sprintf("%d of %d failed", resp.Summary.Failed, len(req.Names))
		if resp.Summary.Failed == len(req.Names) {
			status = "error"
		}
	}
	action := "batch-disable"
	if req.Enable {
		action = "batch-enable"
	}
	s.writeOpsLog(action, status, start, map[string]any{
		"kind":      req.Kind,
		"enable":    req.Enable,
		"count":     len(req.Names),
		"updated":   resp.Summary.Updated,
		"unchanged": resp.Summary.Unchanged,
		"failed":    resp.Summary.Failed,
		"scope":     "ui",
	}, msg)

	writeJSON(w, resp)
}
