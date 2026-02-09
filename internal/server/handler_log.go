package server

import (
	"net/http"
	"strconv"

	"skillshare/internal/oplog"
)

func (s *Server) handleListLog(w http.ResponseWriter, r *http.Request) {
	logType := r.URL.Query().Get("type")
	limitStr := r.URL.Query().Get("limit")

	filename := oplog.OpsFile
	if logType == "audit" {
		filename = oplog.AuditFile
	}

	limit := 100
	if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
		limit = n
	}

	entries, err := oplog.Read(s.configPath(), filename, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read log: "+err.Error())
		return
	}
	if entries == nil {
		entries = []oplog.Entry{}
	}

	// Count total entries (read without limit)
	all, _ := oplog.Read(s.configPath(), filename, 0)
	total := len(all)

	writeJSON(w, map[string]any{
		"entries": entries,
		"total":   total,
	})
}

func (s *Server) handleClearLog(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	logType := r.URL.Query().Get("type")

	filename := oplog.OpsFile
	if logType == "audit" {
		filename = oplog.AuditFile
	}

	if err := oplog.Clear(s.configPath(), filename); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to clear log: "+err.Error())
		return
	}

	writeJSON(w, map[string]any{"success": true})
}
