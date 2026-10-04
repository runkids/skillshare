package server

import (
	"errors"
	"net/http"
	"path/filepath"
	"time"

	"skillshare/internal/resource"
	"skillshare/internal/skillignore"
	ssync "skillshare/internal/sync"
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
	follow := s.skillFollowSet()
	agentsSource := s.agentsSource()
	s.mu.RUnlock()

	// Discover once and build a name → status lookup. First match wins, matching
	// the resolve* helpers used by the single-resource toggle handler.
	type entry struct {
		relPath  string
		disabled bool
	}
	lookup := map[string]entry{}
	put := func(key string, e entry) {
		if key == "" {
			return
		}
		if _, exists := lookup[key]; !exists {
			lookup[key] = e
		}
	}

	var ignorePath string
	if req.Kind == "agent" {
		if agentsSource == "" {
			writeError(w, http.StatusNotFound, "no agents source configured")
			return
		}
		discovered, err := resource.AgentKind{}.Discover(agentsSource)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to discover agents: "+err.Error())
			return
		}
		for _, d := range discovered {
			e := entry{relPath: d.RelPath, disabled: d.Disabled}
			put(d.FlatName, e)
			put(d.Name, e)
			put(d.RelPath, e)
			put(agentDisplayName(d.RelPath), e)
		}
		ignorePath = filepath.Join(agentsSource, ".agentignore")
	} else {
		discovered, err := ssync.DiscoverSourceSkillsAllWithOptions(source, ssync.DiscoveryOptions{Follow: follow})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to discover skills: "+err.Error())
			return
		}
		for _, d := range discovered {
			e := entry{relPath: d.RelPath, disabled: d.Disabled}
			put(d.FlatName, e)
			put(filepath.Base(d.SourcePath), e)
			put(d.RelPath, e)
		}
		ignorePath = filepath.Join(source, ".skillignore")
	}

	var resp batchToggleResponse
	resp.Results = make([]batchToggleItemResult, 0, len(req.Names))
	// Each successful result, so its outcome can be checked after writing.
	type pending struct {
		relPath string
		updated bool
	}
	written := map[int]pending{}
	succeed := func(name, relPath string, updated bool) {
		written[len(resp.Results)] = pending{relPath, updated}
		resp.Results = append(resp.Results, batchToggleItemResult{Name: name, Success: true, Disabled: !req.Enable})
		if updated {
			resp.Summary.Updated++
		} else {
			resp.Summary.Unchanged++
		}
	}

	iw, closeIgnore, err := skillignore.OpenWriter(ignorePath, req.Kind != "agent")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to open ignore file: "+err.Error())
		return
	}
	defer closeIgnore()

	// Hold the write lock while writing and re-checking the outcome.
	s.mu.Lock()
	for _, name := range req.Names {
		e, ok := lookup[name]
		if !ok {
			resp.Results = append(resp.Results, batchToggleItemResult{Name: name, Error: "resource not found: " + name})
			resp.Summary.Failed++
			continue
		}

		if req.Kind != "agent" {
			if err := follow.WriteBoundary(source, e.relPath); err != nil {
				resp.Results = append(resp.Results, batchToggleItemResult{Name: name, Error: err.Error()})
				resp.Summary.Failed++
				continue
			}
		}

		if req.Enable {
			if !e.disabled {
				succeed(name, e.relPath, false)
				continue
			}
			removed, err := skillignore.RemovePatternWith(iw, ignorePath, e.relPath)
			if err != nil {
				resp.Results = append(resp.Results, batchToggleItemResult{Name: name, Disabled: true, Error: err.Error()})
				resp.Summary.Failed++
				continue
			}
			if !removed {
				resp.Results = append(resp.Results, batchToggleItemResult{
					Name:     name,
					Disabled: true,
					Error:    "disabled by a glob or directory pattern — edit the ignore file manually to remove the matching rule",
				})
				resp.Summary.Failed++
				continue
			}
			succeed(name, e.relPath, true)
			continue
		}

		added, err := skillignore.AddPatternWith(iw, ignorePath, e.relPath)
		if err != nil {
			resp.Results = append(resp.Results, batchToggleItemResult{Name: name, Error: err.Error()})
			resp.Summary.Failed++
			continue
		}
		succeed(name, e.relPath, added)
	}

	// A written rule can still be overridden by a later one, such as a "!" line
	// in the .local file, so re-check the outcome the way discovery sees it.
	if len(written) > 0 {
		states, err := s.disabledByRelPath(req.Kind, source, agentsSource, follow)
		if err != nil {
			s.mu.Unlock()
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		for i, p := range written {
			msg := toggleOverrideError(req.Kind, req.Enable, states[p.relPath])
			if msg == "" {
				continue
			}
			if p.updated {
				resp.Summary.Updated--
			} else {
				resp.Summary.Unchanged--
			}
			resp.Results[i] = batchToggleItemResult{Name: resp.Results[i].Name, Disabled: req.Enable, Error: msg}
			resp.Summary.Failed++
		}
	}
	s.mu.Unlock()

	action := "batch-disable"
	if req.Enable {
		action = "batch-enable"
	}
	s.writeOpsLog(action, "ok", start, map[string]any{
		"kind":      req.Kind,
		"enable":    req.Enable,
		"count":     len(req.Names),
		"updated":   resp.Summary.Updated,
		"unchanged": resp.Summary.Unchanged,
		"failed":    resp.Summary.Failed,
		"scope":     "ui",
	}, "")

	writeJSON(w, resp)
}
