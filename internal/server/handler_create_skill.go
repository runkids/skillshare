package server

import (
	"net/http"

	"skillshare/internal/skill"
)

func (s *Server) handleGetTemplates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"patterns":   skill.Patterns,
		"categories": skill.Categories,
	})
}
