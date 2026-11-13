package server

import "net/http"

// refuseUnavailableProfile requires s.mu and runs before discovery or SSE initialization.
func (s *Server) refuseUnavailableProfile(w http.ResponseWriter) bool {
	err := s.activeProfileError()
	if err == nil {
		return false
	}
	writeCodedError(w, http.StatusConflict, "profile_unavailable", err.Error(), nil)
	return true
}

// activeProfileError requires the server config lock to be held.
func (s *Server) activeProfileError() error {
	if s.IsProjectMode() && s.projectCfg != nil {
		return s.projectCfg.ActiveProfileError()
	}
	return s.cfg.ActiveProfileError()
}
