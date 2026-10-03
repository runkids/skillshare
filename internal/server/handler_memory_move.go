package server

import (
	"errors"
	"net/http"
	"time"

	"skillshare/internal/memory"
)

func (s *Server) handleMemoryMove(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var body struct {
		Path    string `json:"path"`
		NewPath string `json:"new_path"`
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
	args := map[string]any{"path": body.Path, "new_path": body.NewPath, "scope": "ui"}
	note, err := memory.Move(root, body.Path, body.NewPath, body.Version)
	if err != nil {
		s.writeOpsLog("memory-move", "error", start, args, err.Error())
		writeMemoryError(w, err)
		return
	}
	s.writeOpsLog("memory-move", "ok", start, args, "")
	writeJSON(w, note)
}
