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

	relPath, isDisabled, err := s.resolveSkillRelPathWithStatus(source, name)
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
		if !isDisabled {
			writeJSON(w, map[string]any{"success": true, "name": name, "disabled": false, "message": "not disabled"})
			return
		}
		removed, err := skillignore.RemovePattern(ignorePath, relPath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update .skillignore: "+err.Error())
			return
		}
		if !removed {
			writeError(w, http.StatusConflict, "skill is disabled by a glob or directory pattern in .skillignore or .skillignore.local — edit the file manually to remove the matching rule")
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
	relPath, _, err := s.resolveSkillRelPathWithStatus(source, name)
	return relPath, err
}

// resolveSkillRelPathWithStatus is like resolveSkillRelPath but also returns
// whether the skill is currently disabled (matched by .skillignore).
func (s *Server) resolveSkillRelPathWithStatus(source, name string) (string, bool, error) {
	discovered, err := ssync.DiscoverSourceSkillsAll(source)
	if err != nil {
		return "", false, fmt.Errorf("failed to discover skills: %w", err)
	}
	for _, d := range discovered {
		baseName := filepath.Base(d.SourcePath)
		if d.FlatName == name || baseName == name || d.RelPath == name {
			return d.RelPath, d.Disabled, nil
		}
	}
	return "", false, fmt.Errorf("skill not found: %s", name)
}
