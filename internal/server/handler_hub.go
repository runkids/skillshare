package server

import (
	"net/http"
	"path/filepath"

	"skillshare/internal/hub"
)

func (s *Server) handleHubIndex(w http.ResponseWriter, r *http.Request) {
	sourcePath := s.cfg.Source
	if s.IsProjectMode() {
		sourcePath = filepath.Join(s.projectRoot, ".skillshare", "skills")
	}

	discovered, err := s.cache.Discover(sourcePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	idx, err := hub.BuildIndex(sourcePath, discovered, false, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, idx)
}
