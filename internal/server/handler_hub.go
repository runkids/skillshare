package server

import (
	"net/http"
	"path/filepath"

	"skillshare/internal/hub"
)

func (s *Server) handleHubIndex(w http.ResponseWriter, r *http.Request) {
	// Snapshot config under RLock, then release before I/O.
	s.mu.RLock()
	sourcePath := s.cfg.Source
	projectRoot := s.projectRoot
	s.mu.RUnlock()

	if projectRoot != "" {
		sourcePath = filepath.Join(projectRoot, ".skillshare", "skills")
	}

	idx, err := hub.BuildIndex(sourcePath, false, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, idx)
}
