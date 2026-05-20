package server

import (
	"net/http"

	"skillshare/internal/hub"
)

func (s *Server) handleHubIndex(w http.ResponseWriter, r *http.Request) {
	// Snapshot config under RLock, then release before I/O.
	s.mu.RLock()
	sourcePath := s.skillsSource()
	s.mu.RUnlock()

	idx, err := hub.BuildIndex(sourcePath, false, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, idx)
}
