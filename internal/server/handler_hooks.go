package server

import (
	"net/http"

	"skillshare/internal/inspect"
)

func (s *Server) handleListHooks(w http.ResponseWriter, r *http.Request) {
	projectRoot := ""
	if s.IsProjectMode() {
		projectRoot = s.projectRoot
	}

	items, warnings, err := inspect.ScanHooks(projectRoot)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to scan hooks: "+err.Error())
		return
	}

	writeJSON(w, map[string]any{
		"hooks":    items,
		"warnings": warnings,
	})
}
