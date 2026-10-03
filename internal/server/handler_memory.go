package server

import (
	"errors"
	"net/http"
	"os"
	"slices"
	"time"

	"skillshare/internal/memory"
)

// memoryRoot uses the same extra source resolution as CLI global/project mode.
// Callers hold s.mu.
func (s *Server) memoryRoot() (string, error) {
	if s.IsProjectMode() {
		return memory.ProjectRoot(s.projectCfg, s.projectRoot)
	}
	return memory.GlobalRoot(s.cfg)
}

func writeMemoryError(w http.ResponseWriter, err error) {
	status, code := http.StatusBadRequest, "memory_invalid"
	if errors.Is(err, memory.ErrConflict) {
		status, code = http.StatusConflict, "memory_conflict"
	}
	if errors.Is(err, os.ErrExist) {
		status, code = http.StatusConflict, "memory_destination_exists"
	}
	if os.IsNotExist(err) {
		status, code = http.StatusNotFound, "memory_not_found"
	}
	writeCodedError(w, status, code, err.Error(), map[string]string{})
}

func (s *Server) handleMemoryNotes(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	root, err := s.memoryRoot()
	if err != nil {
		writeMemoryError(w, err)
		return
	}
	notes, err := memory.List(root, r.URL.Query().Get("search"))
	if err != nil {
		writeMemoryError(w, err)
		return
	}
	_, initialized, _ := memory.Extra(s.extrasConfig())
	all := notes
	if r.URL.Query().Get("search") != "" {
		all, err = memory.List(root, "")
		if err != nil {
			writeMemoryError(w, err)
			return
		}
	}
	for _, path := range []string{"INDEX.md", "LEARNED.md"} {
		initialized = initialized && slices.ContainsFunc(all, func(note memory.Note) bool {
			return note.Path == path && note.Invalid == ""
		})
	}
	writeJSON(w, map[string]any{"root": root, "initialized": initialized, "notes": notes, "instructions": s.memoryInstructionsByMode(root), "index": memory.InspectIndex(root, all)})
}

func (s *Server) handleMemoryIndex(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var body struct {
		Path    string `json:"path"`
		Version string `json:"version"`
	}
	if err := decodeJSON(w, r, &body, 4096); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
		}
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	root, err := s.memoryRoot()
	if err != nil {
		writeMemoryError(w, err)
		return
	}
	note, err := memory.LinkFromIndex(root, body.Path, body.Version)
	if err != nil {
		s.writeOpsLog("memory-index", "error", start, map[string]any{"path": body.Path, "scope": "ui"}, err.Error())
		writeMemoryError(w, err)
		return
	}
	s.writeOpsLog("memory-index", "ok", start, map[string]any{"path": body.Path, "scope": "ui"}, "")
	writeJSON(w, note)
}

func (s *Server) handleMemoryInit(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	root, err := s.memoryRoot()
	defer func() {
		status, msg := "ok", ""
		if err != nil {
			status, msg = "error", err.Error()
		}
		s.writeOpsLog("memory-init", status, start, map[string]any{"scope": "ui"}, msg)
	}()
	if err != nil {
		writeMemoryError(w, err)
		return
	}
	if err = memory.Init(root); err != nil {
		writeMemoryError(w, err)
		return
	}
	extra, found, err := memory.Extra(s.extrasConfig())
	if err != nil {
		writeMemoryError(w, err)
		return
	}
	if !found {
		extras := s.sharedExtras()
		prev := *extras
		*extras = append(slices.Clone(prev), extra)
		if err = s.saveConfig(); err != nil {
			*extras = prev
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, map[string]any{"success": true, "root": root})
}

func (s *Server) handleMemoryContent(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	root, err := s.memoryRoot()
	if err != nil {
		writeMemoryError(w, err)
		return
	}
	note, err := memory.Read(root, r.URL.Query().Get("path"))
	if err != nil {
		writeMemoryError(w, err)
		return
	}
	writeJSON(w, note)
}

func (s *Server) handleMemoryWrite(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var body struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		Version string `json:"version"`
	}
	if err := decodeJSON(w, r, &body, 6*memory.MaxNoteBytes+4096); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
		}
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	root, err := s.memoryRoot()
	if err != nil {
		writeMemoryError(w, err)
		return
	}
	note, err := memory.Write(root, body.Path, body.Content, body.Version)
	if err != nil {
		s.writeOpsLog("memory-write", "error", start, map[string]any{"path": body.Path, "scope": "ui"}, err.Error())
		writeMemoryError(w, err)
		return
	}
	s.writeOpsLog("memory-write", "ok", start, map[string]any{"path": body.Path, "scope": "ui"}, "")
	writeJSON(w, note)
}

func (s *Server) handleMemoryDelete(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var body struct {
		Path    string `json:"path"`
		Version string `json:"version"`
	}
	if err := decodeJSON(w, r, &body, 4096); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
		}
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	root, err := s.memoryRoot()
	if err != nil {
		writeMemoryError(w, err)
		return
	}
	if err := memory.Delete(root, body.Path, body.Version); err != nil {
		s.writeOpsLog("memory-delete", "error", start, map[string]any{"path": body.Path, "scope": "ui"}, err.Error())
		writeMemoryError(w, err)
		return
	}
	s.writeOpsLog("memory-delete", "ok", start, map[string]any{"path": body.Path, "scope": "ui"}, "")
	writeJSON(w, map[string]any{"success": true, "path": body.Path})
}
