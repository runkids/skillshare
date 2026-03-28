package server

import (
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"skillshare/internal/skillignore"
	ssync "skillshare/internal/sync"
)

func (s *Server) handleDisableSkill(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	name := r.PathValue("name")

	relPath, err := s.resolveSkillRelPath(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	source := s.cfg.Source
	ignorePath := filepath.Join(source, ".skillignore")

	if skillignore.HasPattern(ignorePath, relPath) {
		writeJSON(w, map[string]any{"success": true, "name": name, "disabled": true, "message": "already disabled"})
		return
	}

	if err := skillignore.AddPattern(ignorePath, relPath); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update .skillignore: "+err.Error())
		return
	}

	s.writeOpsLog("disable", "ok", start, map[string]any{
		"name":  name,
		"scope": "ui",
	}, "")

	writeJSON(w, map[string]any{"success": true, "name": name, "disabled": true})
}

func (s *Server) handleEnableSkill(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	name := r.PathValue("name")

	relPath, err := s.resolveSkillRelPath(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	source := s.cfg.Source
	ignorePath := filepath.Join(source, ".skillignore")

	removed, err := skillignore.RemovePattern(ignorePath, relPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update .skillignore: "+err.Error())
		return
	}
	if !removed {
		writeJSON(w, map[string]any{"success": true, "name": name, "disabled": false, "message": "not disabled"})
		return
	}

	s.writeOpsLog("enable", "ok", start, map[string]any{
		"name":  name,
		"scope": "ui",
	}, "")

	writeJSON(w, map[string]any{"success": true, "name": name, "disabled": false})
}

func (s *Server) resolveSkillRelPath(name string) (string, error) {
	discovered, err := ssync.DiscoverSourceSkillsAll(s.cfg.Source)
	if err != nil {
		return "", fmt.Errorf("failed to discover skills: %w", err)
	}
	for _, d := range discovered {
		baseName := filepath.Base(d.SourcePath)
		if d.FlatName == name || baseName == name || d.RelPath == name {
			return d.RelPath, nil
		}
	}
	return "", fmt.Errorf("skill not found: %s", name)
}
