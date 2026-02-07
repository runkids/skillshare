package server

import (
	"encoding/json"
	"net/http"

	"skillshare/internal/backup"
)

type backupInfoJSON struct {
	Timestamp string   `json:"timestamp"`
	Path      string   `json:"path"`
	Targets   []string `json:"targets"`
	Date      string   `json:"date"`
	SizeMB    float64  `json:"sizeMB"`
}

func toBackupJSON(b backup.BackupInfo) backupInfoJSON {
	return backupInfoJSON{
		Timestamp: b.Timestamp,
		Path:      b.Path,
		Targets:   b.Targets,
		Date:      b.Date.Format("2006-01-02T15:04:05Z07:00"),
		SizeMB:    float64(backup.Size(b.Path)) / 1024 / 1024,
	}
}

// handleListBackups returns all backups
func (s *Server) handleListBackups(w http.ResponseWriter, r *http.Request) {
	backups, err := backup.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]backupInfoJSON, 0, len(backups))
	for _, b := range backups {
		items = append(items, toBackupJSON(b))
	}

	total, _ := backup.TotalSize()
	writeJSON(w, map[string]any{
		"backups":     items,
		"totalSizeMB": float64(total) / 1024 / 1024,
	})
}

// handleCreateBackup creates a backup of target(s)
func (s *Server) handleCreateBackup(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var body struct {
		Target string `json:"target"` // empty = all targets
	}
	json.NewDecoder(r.Body).Decode(&body)

	targets := make(map[string]string)
	if body.Target != "" {
		t, ok := s.cfg.Targets[body.Target]
		if !ok {
			writeError(w, http.StatusBadRequest, "target not found: "+body.Target)
			return
		}
		targets[body.Target] = t.Path
	} else {
		for name, t := range s.cfg.Targets {
			targets[name] = t.Path
		}
	}

	var created []string
	for name, path := range targets {
		bp, err := backup.Create(name, path)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "backup failed for "+name+": "+err.Error())
			return
		}
		if bp != "" {
			created = append(created, name)
		}
	}

	writeJSON(w, map[string]any{
		"success":         true,
		"backedUpTargets": created,
	})
}

// handleCleanupBackups removes old backups
func (s *Server) handleCleanupBackups(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := backup.DefaultCleanupConfig()
	removed, err := backup.Cleanup(cfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, map[string]any{
		"success": true,
		"removed": removed,
	})
}

// handleRestore restores a backup to a target
func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var body struct {
		Timestamp string `json:"timestamp"`
		Target    string `json:"target"`
		Force     bool   `json:"force"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if body.Timestamp == "" || body.Target == "" {
		writeError(w, http.StatusBadRequest, "timestamp and target are required")
		return
	}

	// Verify target exists in config
	t, ok := s.cfg.Targets[body.Target]
	if !ok {
		writeError(w, http.StatusBadRequest, "target not found: "+body.Target)
		return
	}

	// Find backup
	bk, err := backup.GetBackupByTimestamp(body.Timestamp)
	if err != nil {
		writeError(w, http.StatusNotFound, "backup not found: "+err.Error())
		return
	}

	opts := backup.RestoreOptions{Force: body.Force}

	// Validate first
	if err := backup.ValidateRestore(bk.Path, body.Target, t.Path, opts); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	// Restore
	if err := backup.RestoreToPath(bk.Path, body.Target, t.Path, opts); err != nil {
		writeError(w, http.StatusInternalServerError, "restore failed: "+err.Error())
		return
	}

	writeJSON(w, map[string]any{
		"success":   true,
		"target":    body.Target,
		"timestamp": body.Timestamp,
	})
}
