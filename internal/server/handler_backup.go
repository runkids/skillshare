package server

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"skillshare/internal/backup"
	"skillshare/internal/config"
	"skillshare/internal/utils"
)

type backupInfoJSON struct {
	Timestamp string            `json:"timestamp"`
	Path      string            `json:"path"`
	Targets   []string          `json:"targets"`
	Entries   []backupEntryJSON `json:"entries"`
	Date      string            `json:"date"`
	SizeBytes int64             `json:"sizeBytes"`
}

// backupEntryJSON is one snapshot folder inside a backup.
type backupEntryJSON struct {
	Name      string `json:"name"`
	SizeBytes int64  `json:"sizeBytes"`
	Files     int    `json:"files"`
}

// toBackupJSON walks each snapshot folder once; the backup's size is their sum.
func toBackupJSON(b backup.BackupInfo) backupInfoJSON {
	entries := make([]backupEntryJSON, 0, len(b.Targets))
	var total int64
	for _, name := range b.Targets {
		size, files := backup.Stats(filepath.Join(b.Path, name))
		entries = append(entries, backupEntryJSON{Name: name, SizeBytes: size, Files: files})
		total += size
	}
	return backupInfoJSON{
		Timestamp: b.Timestamp,
		Path:      b.Path,
		Targets:   b.Targets,
		Entries:   entries,
		Date:      b.Date.Format("2006-01-02T15:04:05Z07:00"),
		SizeBytes: total,
	}
}

// backupDir returns the snapshot directory for the current mode: the global
// one, or <project>/.skillshare/backups (agents only) in project mode.
func (s *Server) backupDir() string {
	if s.IsProjectMode() {
		return backup.ProjectBackupDir(s.projectRoot)
	}
	return backup.BackupDir()
}

// backupRetention is the global config's retention limits; project snapshots
// keep the defaults, since project configs have no backup section.
func (s *Server) backupRetention() backup.CleanupConfig {
	if s.IsProjectMode() {
		return backup.DefaultCleanupConfig()
	}
	return backup.RetentionConfig(s.cfg)
}

// backupRestoreDest maps a snapshot entry to the directory it restores into:
// "<target>-agents" is that target's agents directory, a bare target name its
// skills directory (global mode only; project snapshots hold agents only).
func (s *Server) backupRestoreDest(entry string, targets map[string]config.TargetConfig) (string, bool) {
	if name, ok := backup.AgentsEntryTarget(entry); ok {
		if t, found := targets[name]; found {
			if p := resolveAgentPath(t, s.builtinAgentTargets(), name, s.IsProjectMode()); p != "" {
				return p, true
			}
		}
	}
	if s.IsProjectMode() {
		return "", false
	}
	if t, ok := targets[entry]; ok {
		return t.SkillsConfig().Path, true
	}
	return "", false
}

// handleListBackups returns all backups
func (s *Server) handleListBackups(w http.ResponseWriter, r *http.Request) {
	backups, err := backup.ListInDir(s.backupDir())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Sum the per-backup sizes instead of walking every snapshot a second time.
	items := make([]backupInfoJSON, 0, len(backups))
	var total int64
	for _, b := range backups {
		item := toBackupJSON(b)
		total += item.SizeBytes
		items = append(items, item)
	}

	retention := s.backupRetention()
	writeJSON(w, map[string]any{
		"backups":        items,
		"totalSizeBytes": total,
		"retention": map[string]any{
			"maxAgeDays": int(retention.MaxAge.Hours() / 24),
			"maxCount":   retention.MaxCount,
			"maxSizeMB":  retention.MaxSizeMB,
		},
	})
}

// handleCreateBackup creates a backup of target(s)
func (s *Server) handleCreateBackup(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	var body struct {
		Target string `json:"target"` // empty = all targets
	}
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); errors.Is(err, errBodyTooLarge) {
		return
	}

	targets := make(map[string]string)
	if s.IsProjectMode() {
		// Project snapshots hold agents only, as with `skillshare backup agents -p`.
		builtin := s.builtinAgentTargets()
		for name, t := range s.cfg.Targets {
			entry := name + backup.AgentsEntrySuffix
			if body.Target != "" && body.Target != name && body.Target != entry {
				continue
			}
			if p := resolveAgentPath(t, builtin, name, true); p != "" {
				targets[entry] = p
			}
		}
		if body.Target != "" && len(targets) == 0 {
			writeError(w, http.StatusBadRequest, "target not found: "+body.Target)
			return
		}
	} else if body.Target != "" {
		t, ok := s.cfg.Targets[body.Target]
		if !ok {
			writeError(w, http.StatusBadRequest, "target not found: "+body.Target)
			return
		}
		targets[body.Target] = t.SkillsConfig().Path
	} else {
		for name, t := range s.cfg.Targets {
			targets[name] = t.SkillsConfig().Path
		}
	}

	names := make([]string, 0, len(targets))
	for name := range targets {
		names = append(names, name)
	}
	sort.Strings(names)
	var created []string
	for _, name := range names {
		bp, err := backup.CreateInDir(s.backupDir(), name, targets[name])
		if err != nil {
			writeError(w, http.StatusInternalServerError, "backup failed for "+name+": "+err.Error())
			return
		}
		if bp != "" {
			created = append(created, name)
		}
	}

	s.writeOpsLog("backup", "ok", start, map[string]any{
		"target":         body.Target,
		"targets_total":  len(targets),
		"targets_backed": len(created),
		"scope":          "ui",
	}, "")

	writeJSON(w, map[string]any{
		"success":         true,
		"backedUpTargets": created,
	})
}

// handleCleanupBackups removes old backups
func (s *Server) handleCleanupBackups(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	removed, err := backup.CleanupInDir(s.backupDir(), s.backupRetention())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeOpsLog("backup", "ok", start, map[string]any{
		"action":  "cleanup",
		"removed": removed,
		"scope":   "ui",
	}, "")

	writeJSON(w, map[string]any{
		"success": true,
		"removed": removed,
	})
}

// handleRestore restores a backup to a target
func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	var body struct {
		Timestamp string `json:"timestamp"`
		Target    string `json:"target"`
		Force     bool   `json:"force"`
	}
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
		}
		return
	}

	if body.Timestamp == "" || body.Target == "" {
		writeError(w, http.StatusBadRequest, "timestamp and target are required")
		return
	}

	// Resolve where the snapshot entry restores to
	targetPath, ok := s.backupRestoreDest(body.Target, s.cfg.Targets)
	if !ok {
		writeError(w, http.StatusBadRequest, "target not found: "+body.Target)
		return
	}

	// Find backup
	bk, err := backup.GetBackupByTimestampInDir(s.backupDir(), body.Timestamp)
	if err != nil {
		writeError(w, http.StatusNotFound, "backup not found: "+err.Error())
		return
	}

	opts := backup.RestoreOptions{Force: body.Force}

	// Validate first
	if err := backup.ValidateRestore(bk.Path, body.Target, targetPath, opts); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	// Restore
	if err := backup.RestoreToPath(bk.Path, body.Target, targetPath, opts); err != nil {
		writeError(w, http.StatusInternalServerError, "restore failed: "+err.Error())
		return
	}

	s.writeOpsLog("restore", "ok", start, map[string]any{
		"target": body.Target,
		"from":   body.Timestamp,
		"force":  body.Force,
		"scope":  "ui",
	}, "")

	writeJSON(w, map[string]any{
		"success":   true,
		"target":    body.Target,
		"timestamp": body.Timestamp,
	})
}

// handleValidateRestore checks if a restore would succeed and returns conflict info.
func (s *Server) handleValidateRestore(w http.ResponseWriter, r *http.Request) {
	// Snapshot config under RLock, then release before I/O.
	s.mu.RLock()
	targets := s.cloneTargets()
	s.mu.RUnlock()

	var body struct {
		Timestamp string `json:"timestamp"`
		Target    string `json:"target"`
	}
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
		}
		return
	}

	if body.Timestamp == "" || body.Target == "" {
		writeError(w, http.StatusBadRequest, "timestamp and target are required")
		return
	}

	tPath, ok := s.backupRestoreDest(body.Target, targets)
	if !ok {
		writeError(w, http.StatusBadRequest, "target not found: "+body.Target)
		return
	}

	bk, err := backup.GetBackupByTimestampInDir(s.backupDir(), body.Timestamp)
	if err != nil {
		writeError(w, http.StatusNotFound, "backup not found: "+err.Error())
		return
	}

	backupSize := backup.Size(filepath.Join(bk.Path, body.Target))

	// Check destination state with Lstat to detect symlinks
	isSymlink := false
	var conflicts []string
	info, err := os.Lstat(tPath)
	if err == nil {
		if utils.IsLinkMode(tPath, info.Mode()) {
			isSymlink = true
		} else if info.IsDir() {
			entries, _ := os.ReadDir(tPath)
			for _, e := range entries {
				conflicts = append(conflicts, e.Name())
			}
		}
	}

	// Validate backup source exists for target
	opts := backup.RestoreOptions{Force: true}
	validErr := backup.ValidateRestore(bk.Path, body.Target, tPath, opts)

	errMsg := ""
	if validErr != nil {
		errMsg = validErr.Error()
	}

	writeJSON(w, map[string]any{
		"valid":            validErr == nil,
		"error":            errMsg,
		"conflicts":        conflicts,
		"backupSizeBytes":  backupSize,
		"currentIsSymlink": isSymlink,
	})
}

// handleDeleteAllBackups removes every snapshot folder — DELETE /api/backups
func (s *Server) handleDeleteAllBackups(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	removed, err := backup.DeleteAllInDir(s.backupDir())
	args := map[string]any{"action": "delete-all", "removed": removed, "scope": "ui"}
	if err != nil {
		s.writeOpsLog("backup", "error", start, args, err.Error())
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeOpsLog("backup", "ok", start, args, "")
	writeJSON(w, map[string]any{"success": true, "removed": removed})
}

// handleDeleteBackup removes one snapshot folder — DELETE /api/backups/{timestamp}
func (s *Server) handleDeleteBackup(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	ts := r.PathValue("timestamp")
	if !backup.ValidTimestamp(ts) {
		writeError(w, http.StatusBadRequest, "invalid backup timestamp: "+ts)
		return
	}
	if err := backup.DeleteInDir(s.backupDir(), ts); err != nil {
		if errors.Is(err, backup.ErrNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		s.writeOpsLog("backup", "error", start, map[string]any{"action": "delete", "timestamp": ts, "scope": "ui"}, err.Error())
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeOpsLog("backup", "ok", start, map[string]any{
		"action":    "delete",
		"timestamp": ts,
		"scope":     "ui",
	}, "")
	writeJSON(w, map[string]any{"success": true})
}
