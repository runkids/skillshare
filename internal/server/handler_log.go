package server

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

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

	// Build filter from query params
	var f oplog.Filter
	if cmd := strings.TrimSpace(r.URL.Query().Get("cmd")); cmd != "" {
		f.Cmd = cmd
	}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" {
		f.Status = status
	}
	if sinceStr := strings.TrimSpace(r.URL.Query().Get("since")); sinceStr != "" {
		if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			f.Since = t
		}
	}

	// When filtering, read all then filter then limit
	readLimit := limit
	if !f.IsEmpty() {
		readLimit = 0
	}

	entries, err := oplog.Read(s.configPath(), filename, readLimit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read log: "+err.Error())
		return
	}

	if !f.IsEmpty() {
		entries = oplog.FilterEntries(entries, f)
		if limit > 0 && len(entries) > limit {
			entries = entries[:limit]
		}
	}

	if entries == nil {
		entries = []oplog.Entry{}
	}

	// Count totals and collect distinct commands from unfiltered data
	all, _ := oplog.Read(s.configPath(), filename, 0)
	totalAll := len(all)
	total := totalAll

	cmdSet := make(map[string]struct{}, 8)
	for _, e := range all {
		if e.Command != "" {
			cmdSet[e.Command] = struct{}{}
		}
	}
	commands := make([]string, 0, len(cmdSet))
	for c := range cmdSet {
		commands = append(commands, c)
	}
	sort.Strings(commands)

	if !f.IsEmpty() {
		total = len(oplog.FilterEntries(all, f))
	}

	writeJSON(w, map[string]any{
		"entries":  entries,
		"total":    total,
		"totalAll": totalAll,
		"commands": commands,
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
