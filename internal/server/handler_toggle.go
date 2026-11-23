package server

import (
	"net/http"
	"time"

	"skillshare/internal/toggle"
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
	kind := r.URL.Query().Get("kind")
	if kind != "" && kind != "agent" && kind != "skill" {
		writeError(w, http.StatusBadRequest, "invalid kind: "+kind)
		return
	}

	// Resolve under RLock — discovery is I/O-heavy, don't hold write lock
	s.mu.RLock()
	source := s.cfg.EffectiveSkillsSource()
	agentsSource := s.agentsSource()
	s.mu.RUnlock()

	src := toggle.Source{Agent: kind == "agent", Root: source, Walk: s.skillsWalk()}
	if src.Agent {
		if agentsSource == "" {
			writeError(w, http.StatusNotFound, "agent not found: "+name)
			return
		}
		src.Root = agentsSource
	}
	items, err := src.Discover()
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	matches := items.Named(name)
	if len(matches) == 0 {
		what := "skill"
		if src.Agent {
			what = "agent"
		}
		writeError(w, http.StatusNotFound, what+" not found: "+name)
		return
	}
	relPath := matches[0].RelPath

	// Write lock only for the file mutation
	s.mu.Lock()
	defer s.mu.Unlock()

	action, disabled := "disable", true
	if enable {
		action, disabled = "enable", false
		if !matches[0].Disabled {
			writeJSON(w, map[string]any{"success": true, "name": name, "disabled": false, "message": "not disabled"})
			return
		}
	}

	res, err := src.Apply(enable, []string{relPath}, []string{relPath})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	logArgs := map[string]any{
		"name":  name,
		"kind":  kind,
		"scope": "ui",
	}
	if msg := res.Items[0].Blocked; msg != "" {
		// The ignore file may already have changed, so the attempt is still logged.
		s.writeOpsLog(action, "error", start, logArgs, msg)
		writeError(w, http.StatusConflict, msg)
		return
	}
	if !enable && !res.Changed[relPath] {
		writeJSON(w, map[string]any{"success": true, "name": name, "disabled": true, "message": "already disabled"})
		return
	}

	s.writeOpsLog(action, "ok", start, logArgs, "")

	writeJSON(w, map[string]any{"success": true, "name": name, "disabled": disabled})
}
