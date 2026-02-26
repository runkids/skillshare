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

func (s *Server) handleLogStats(w http.ResponseWriter, r *http.Request) {
	logType := r.URL.Query().Get("type")
	filename := oplog.OpsFile
	if logType == "audit" {
		filename = oplog.AuditFile
	}

	entries, err := oplog.Read(s.configPath(), filename, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read log: "+err.Error())
		return
	}

	var f oplog.Filter
	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		if t, parseErr := time.Parse(time.RFC3339, sinceStr); parseErr == nil {
			f.Since = t
		}
	}
	if cmd := r.URL.Query().Get("cmd"); cmd != "" {
		f.Cmd = cmd
	}
	if status := r.URL.Query().Get("status"); status != "" {
		f.Status = status
	}

	if !f.IsEmpty() {
		entries = oplog.FilterEntries(entries, f)
	}

	// Compute stats inline (avoid importing cmd/skillshare types)
	type cmdStatsAPI struct {
		Total   int `json:"total"`
		OK      int `json:"ok"`
		Error   int `json:"error"`
		Partial int `json:"partial"`
		Blocked int `json:"blocked"`
	}
	type statsResponse struct {
		Total         int                    `json:"total"`
		SuccessRate   float64                `json:"success_rate"`
		ByCommand     map[string]cmdStatsAPI `json:"by_command"`
		LastOperation *oplog.Entry           `json:"last_operation,omitempty"`
	}

	resp := statsResponse{
		Total:     len(entries),
		ByCommand: make(map[string]cmdStatsAPI),
	}

	if len(entries) > 0 {
		resp.LastOperation = &entries[0]
		okCount := 0
		for _, e := range entries {
			cs := resp.ByCommand[e.Command]
			cs.Total++
			switch e.Status {
			case "ok":
				cs.OK++
				okCount++
			case "error":
				cs.Error++
			case "partial":
				cs.Partial++
			case "blocked":
				cs.Blocked++
			}
			resp.ByCommand[e.Command] = cs
		}
		resp.SuccessRate = float64(okCount) / float64(len(entries))
	}

	writeJSON(w, resp)
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
