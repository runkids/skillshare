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
	s.handleToggleSkill(w, r, false)
}

func (s *Server) handleEnableSkill(w http.ResponseWriter, r *http.Request) {
	s.handleToggleSkill(w, r, true)
}

func (s *Server) handleToggleSkill(w http.ResponseWriter, r *http.Request, enable bool) {
	start := time.Now()

	name := r.PathValue("name")

	// Resolve under RLock — discovery is I/O-heavy, don't hold write lock
	s.mu.RLock()
	source := s.cfg.Source
	s.mu.RUnlock()

	relPath, err := s.resolveSkillRelPath(source, name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	ignorePath := filepath.Join(source, ".skillignore")

	// Write lock only for the file mutation
	s.mu.Lock()
	defer s.mu.Unlock()

	action := "disable"
	disabled := true

	if enable {
		action = "enable"
		disabled = false
		removed, err := skillignore.RemovePattern(ignorePath, relPath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update .skillignore: "+err.Error())
			return
		}
		if !removed {
			writeJSON(w, map[string]any{"success": true, "name": name, "disabled": false, "message": "not disabled"})
			return
		}
	} else {
		added, err := skillignore.AddPattern(ignorePath, relPath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update .skillignore: "+err.Error())
			return
		}
		if !added {
			writeJSON(w, map[string]any{"success": true, "name": name, "disabled": true, "message": "already disabled"})
			return
		}
	}

	s.writeOpsLog(action, "ok", start, map[string]any{
		"name":  name,
		"scope": "ui",
	}, "")

	writeJSON(w, map[string]any{"success": true, "name": name, "disabled": disabled})
}

// resolveSkillRelPath finds a skill's relPath by flatName or baseName.
// Searches all skills including disabled ones. Caller must provide source path.
func (s *Server) resolveSkillRelPath(source, name string) (string, error) {
	discovered, err := ssync.DiscoverSourceSkillsAll(source)
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
