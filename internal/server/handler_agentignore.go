package server

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"skillshare/internal/resource"
	"skillshare/internal/skillignore"
	"skillshare/internal/utils"
)

type agentignoreStats struct {
	PatternCount  int      `json:"pattern_count"`
	IgnoredCount  int      `json:"ignored_count"`
	Patterns      []string `json:"patterns"`
	IgnoredAgents []string `json:"ignored_agents"`
}

type agentignoreResponse struct {
	Exists bool              `json:"exists"`
	Path   string            `json:"path"`
	Raw    string            `json:"raw"`
	Local  *ignoreLocalFile  `json:"local,omitempty"`
	Stats  *agentignoreStats `json:"stats,omitempty"`
}

func (s *Server) handleGetAgentignore(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	source := s.agentsSource()
	s.mu.RUnlock()

	if source == "" {
		writeJSON(w, agentignoreResponse{Path: "", Raw: ""})
		return
	}

	resolved := utils.ResolveSymlink(source)
	ignorePath := filepath.Join(resolved, ".agentignore")

	raw, err := os.ReadFile(ignorePath)

	resp := agentignoreResponse{
		Exists: err == nil,
		Path:   ignorePath,
		Raw:    string(raw),
		Local:  readIgnoreLocal(ignorePath),
	}

	// Discover agents to compute ignore stats
	matcher := skillignore.ReadAgentIgnoreMatcher(resolved)
	if matcher.HasRules() {
		patterns := matcher.Patterns()
		if patterns == nil {
			patterns = []string{}
		}

		all, _ := resource.AgentKind{}.Discover(source)
		var ignored []string
		for _, a := range all {
			if a.Disabled {
				ignored = append(ignored, a.RelPath)
			}
		}
		if ignored == nil {
			ignored = []string{}
		}

		resp.Stats = &agentignoreStats{
			PatternCount:  len(patterns),
			IgnoredCount:  len(ignored),
			Patterns:      patterns,
			IgnoredAgents: ignored,
		}
	}

	writeJSON(w, resp)
}

func (s *Server) handlePutAgentignore(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	var body struct {
		Raw string `json:"raw"`
	}
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
		}
		return
	}

	source := s.agentsSource()
	if source == "" {
		writeError(w, http.StatusBadRequest, "agents source not configured")
		return
	}

	resolved := utils.ResolveSymlink(source)
	ignorePath := filepath.Join(resolved, ".agentignore")

	if body.Raw == "" {
		if err := os.Remove(ignorePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusInternalServerError, "failed to delete .agentignore: "+err.Error())
			return
		}
	} else {
		// Ensure directory exists
		if err := os.MkdirAll(resolved, 0755); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create agents directory: "+err.Error())
			return
		}
		if err := os.WriteFile(ignorePath, []byte(body.Raw), 0644); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to write .agentignore: "+err.Error())
			return
		}
	}

	s.writeOpsLog("agentignore", "ok", start, map[string]any{
		"scope": "ui",
	}, "")

	writeJSON(w, map[string]any{"success": true})
}
