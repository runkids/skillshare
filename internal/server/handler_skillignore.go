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

type skillignoreStats struct {
	PatternCount  int      `json:"pattern_count"`
	IgnoredCount  int      `json:"ignored_count"`
	Patterns      []string `json:"patterns"`
	IgnoredSkills []string `json:"ignored_skills"`
}

type skillignoreResponse struct {
	Exists bool              `json:"exists"`
	Path   string            `json:"path"`
	Raw    string            `json:"raw"`
	Stats  *skillignoreStats `json:"stats,omitempty"`
}

func (s *Server) handleGetSkillignore(w http.ResponseWriter, r *http.Request) {
	// Snapshot source path under RLock, then release before I/O.
	s.mu.RLock()
	source := s.cfg.Source
	s.mu.RUnlock()

	ignorePath := filepath.Join(source, ".skillignore")

	raw, err := os.ReadFile(ignorePath)
	if err != nil {
		writeJSON(w, skillignoreResponse{
			Path: ignorePath,
		})
		return
	}

	resp := skillignoreResponse{
		Exists: true,
		Path:   ignorePath,
		Raw:    string(raw),
	}

	// Discover skills with stats to get ignore information
	_, stats, discoverErr := sync.DiscoverSourceSkillsWithStats(source)
	if discoverErr == nil && stats != nil {
		patterns := stats.Patterns
		if patterns == nil {
			patterns = []string{}
		}
		ignoredSkills := stats.IgnoredSkills
		if ignoredSkills == nil {
			ignoredSkills = []string{}
		}
		resp.Stats = &skillignoreStats{
			PatternCount:  stats.PatternCount(),
			IgnoredCount:  stats.IgnoredCount(),
			Patterns:      patterns,
			IgnoredSkills: ignoredSkills,
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
		if err := os.Remove(ignorePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusInternalServerError, "failed to delete .skillignore: "+err.Error())
			return
		}
	} else {
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
