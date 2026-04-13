package server

import (
	"net/http"

	"skillshare/internal/resources/managed"
)

func (s *Server) handleManagedCapabilities(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	targets := s.cloneTargets()
	s.mu.RUnlock()

	writeJSON(w, managed.CapabilitySnapshotForTargets(targets))
}
