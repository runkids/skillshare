package server

import (
	"net/http"

	"skillshare/internal/hub"
	ssync "skillshare/internal/sync"
)

func (s *Server) handleHubIndex(w http.ResponseWriter, r *http.Request) {
	// Snapshot config under RLock, then release before I/O.
	s.mu.RLock()
	sourcePath := s.skillsSource()
	follow := s.skillFollowSet()
	s.mu.RUnlock()

	idx, err := hub.BuildIndexWithOptions(sourcePath, false, false, ssync.DiscoveryOptions{Follow: follow})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, idx)
}
