package server

import (
	"encoding/json"
	"net/http"
	"os"

	"skillshare/internal/config"
	ssync "skillshare/internal/sync"
)

type targetItem struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Mode        string `json:"mode"`
	Status      string `json:"status"`
	LinkedCount int    `json:"linkedCount"`
	LocalCount  int    `json:"localCount"`
}

func (s *Server) handleListTargets(w http.ResponseWriter, r *http.Request) {
	items := make([]targetItem, 0, len(s.cfg.Targets))

	globalMode := s.cfg.Mode
	if globalMode == "" {
		globalMode = "merge"
	}

	for name, target := range s.cfg.Targets {
		mode := target.Mode
		if mode == "" {
			mode = globalMode
		}

		item := targetItem{
			Name: name,
			Path: target.Path,
			Mode: mode,
		}

		if mode == "merge" {
			status, linked, local := ssync.CheckStatusMerge(target.Path, s.cfg.Source)
			item.Status = status.String()
			item.LinkedCount = linked
			item.LocalCount = local
		} else {
			status := ssync.CheckStatus(target.Path, s.cfg.Source)
			item.Status = status.String()
		}

		items = append(items, item)
	}

	writeJSON(w, map[string]any{"targets": items})
}

func (s *Server) handleAddTarget(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var body struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if body.Name == "" || body.Path == "" {
		writeError(w, http.StatusBadRequest, "name and path are required")
		return
	}

	if _, exists := s.cfg.Targets[body.Name]; exists {
		writeError(w, http.StatusConflict, "target already exists: "+body.Name)
		return
	}

	s.cfg.Targets[body.Name] = config.TargetConfig{Path: body.Path}
	if err := s.cfg.Save(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save config: "+err.Error())
		return
	}

	writeJSON(w, map[string]any{"success": true})
}

func (s *Server) handleRemoveTarget(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	name := r.PathValue("name")

	target, exists := s.cfg.Targets[name]
	if !exists {
		writeError(w, http.StatusNotFound, "target not found: "+name)
		return
	}

	// Remove symlinks from the target before deleting from config
	info, err := os.Lstat(target.Path)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		os.Remove(target.Path)
	}

	delete(s.cfg.Targets, name)
	if err := s.cfg.Save(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save config: "+err.Error())
		return
	}

	writeJSON(w, map[string]any{"success": true, "name": name})
}
