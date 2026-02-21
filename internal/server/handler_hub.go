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

	idx, err := hub.BuildIndex(sourcePath, false, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, idx)
}
