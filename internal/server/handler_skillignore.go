package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"skillshare/internal/sync"
)

func (s *Server) handleGetSkillignore(w http.ResponseWriter, r *http.Request) {
	// Snapshot source path under RLock, then release before I/O.
	s.mu.RLock()
	source := s.cfg.Source
	s.mu.RUnlock()

	ignorePath := filepath.Join(source, ".skillignore")

	raw, err := os.ReadFile(ignorePath)
	if err != nil {
		// File doesn't exist — return minimal response
		writeJSON(w, map[string]any{
			"exists": false,
			"path":   ignorePath,
			"raw":    "",
		})
		return
	}

	// Discover skills with stats to get ignore information
	_, stats, discoverErr := sync.DiscoverSourceSkillsWithStats(source)

	resp := map[string]any{
		"exists": true,
		"path":   ignorePath,
		"raw":    string(raw),
	}

	if discoverErr == nil && stats != nil {
		resp["stats"] = map[string]any{
			"pattern_count":  stats.PatternCount(),
			"ignored_count":  stats.IgnoredCount(),
			"patterns":       stats.Patterns,
			"ignored_skills": stats.IgnoredSkills,
		}
	}

	writeJSON(w, resp)
}

func (s *Server) handlePutSkillignore(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	var body struct {
		Raw string `json:"raw"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	source := s.cfg.Source
	ignorePath := filepath.Join(source, ".skillignore")

	if body.Raw == "" {
		// Empty raw → delete the file
		if err := os.Remove(ignorePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusInternalServerError, "failed to delete .skillignore: "+err.Error())
			return
		}
	} else {
		// Write .skillignore
		if err := os.WriteFile(ignorePath, []byte(body.Raw), 0644); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to write .skillignore: "+err.Error())
			return
		}
	}

	s.writeOpsLog("skillignore", "ok", start, map[string]any{
		"scope": "ui",
	}, "")

	writeJSON(w, map[string]any{"success": true})
}
