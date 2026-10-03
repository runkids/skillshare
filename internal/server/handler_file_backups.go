package server

import (
	"errors"
	"net/http"
	"path/filepath"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/instructions"
	"skillshare/internal/memory"
	syncpkg "skillshare/internal/sync"
)

type fileBackupJSON struct {
	Path     string `json:"path"`
	Versions int    `json:"versions"`
	Latest   string `json:"latest"`
	Target   string `json:"target,omitempty"`
	Extra    string `json:"extra,omitempty"`
	// Source marks the shared file itself, not a place it is put.
	Source bool `json:"source,omitempty"`
}

type fileBackupCurrentJSON struct {
	Exists bool   `json:"exists"`
	LinkTo string `json:"link_to,omitempty"`
}

type fileBackupVersionJSON struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Reason  string `json:"reason"`
	Time    string `json:"time"`
	Size    int64  `json:"size"`
	Preview string `json:"preview"`
	LinkTo  string `json:"link_to,omitempty"`
	None    bool   `json:"none,omitempty"`
}

// fileBackupInScope reports whether path may be shown in the current mode:
// every file in global mode; project files and its configured memory source in project mode.
func (s *Server) fileBackupInScope(path string) bool {
	if !s.IsProjectMode() {
		return true
	}
	if syncpkg.PathInside(s.projectRoot, path) {
		return true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, found, err := memory.Extra(s.extrasConfig()); !found || err != nil {
		return false
	}
	root, err := s.memoryRoot()
	return err == nil && syncpkg.PathInside(root, path)
}

// fileBackupOwners maps file paths to the target whose instruction file it is
// and the shared (single-file extra) file that uses it, best effort; sources
// are the shared files themselves. Callers must hold s.mu.
func (s *Server) fileBackupOwners() (targets, extras map[string]string, sources map[string]bool) {
	targets, extras, sources = map[string]string{}, map[string]string{}, map[string]bool{}
	clean := func(p string) string { return filepath.Clean(config.ExpandPath(p)) }
	for name := range s.cfg.Targets {
		if it, _, ok := s.targetInstructions(name); ok && it.Path != "" {
			targets[clean(it.Path)] = name
		}
	}
	for _, r := range s.instructionRiders() {
		if r.Path != "" {
			if _, taken := targets[clean(r.Path)]; !taken {
				targets[clean(r.Path)] = r.Name
			}
		}
	}
	resolver := s.instructionsResolver()
	for _, extra := range s.extrasConfig() {
		if extra.File == "" {
			continue
		}
		if src, err := filepath.Abs(filepath.Join(s.extrasSourceDir(extra), extra.File)); err == nil {
			extras[filepath.Clean(src)] = extra.Name
			sources[filepath.Clean(src)] = true
		}
		for j := range extra.Targets {
			extras[filepath.Clean(instructions.ExtraFile(extra, j, resolver).Target)] = extra.Name
		}
	}
	return targets, extras, sources
}

// handleListFileBackups — GET /api/file-backups
func (s *Server) handleListFileBackups(w http.ResponseWriter, r *http.Request) {
	files, err := syncpkg.ListFileBackups()
	if err != nil {
		writeCodedError(w, http.StatusInternalServerError, "file_backup_failed", err.Error(), map[string]string{"detail": err.Error()})
		return
	}
	s.mu.RLock()
	targets, extras, sources := s.fileBackupOwners()
	s.mu.RUnlock()

	items := make([]fileBackupJSON, 0, len(files))
	for _, f := range files {
		if !s.fileBackupInScope(f.Path) {
			continue
		}
		items = append(items, fileBackupJSON{
			Path:     f.Path,
			Versions: f.Versions,
			Latest:   f.Latest.Format(time.RFC3339),
			Target:   targets[f.Path],
			Extra:    extras[f.Path],
			Source:   sources[f.Path],
		})
	}
	writeJSON(w, map[string]any{"files": items})
}

// fileBackupPath validates the path query or body value against the mode's
// scope; it writes the error response and returns false when invalid.
func (s *Server) fileBackupPath(w http.ResponseWriter, path string) (string, bool) {
	if path == "" {
		writeCodedError(w, http.StatusBadRequest, "file_backup_not_found", "path is required", map[string]string{"path": path})
		return "", false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		writeCodedError(w, http.StatusBadRequest, "file_backup_not_found", err.Error(), map[string]string{"path": path})
		return "", false
	}
	if !s.fileBackupInScope(abs) {
		writeCodedError(w, http.StatusForbidden, "file_backup_outside_project", abs+" is outside the project", map[string]string{"path": abs})
		return "", false
	}
	return abs, true
}

// writeFileBackupError maps sync errors to coded API errors.
func writeFileBackupError(w http.ResponseWriter, path string, err error) {
	var linkErr *syncpkg.FileBackupLinkError
	switch {
	case errors.As(err, &linkErr):
		writeCodedError(w, http.StatusConflict, "file_backup_is_link", err.Error(), map[string]string{"path": linkErr.Path, "link_to": linkErr.LinkTo})
	case errors.Is(err, syncpkg.ErrFileBackupNotFound), errors.Is(err, syncpkg.ErrInvalidFileBackupID):
		writeCodedError(w, http.StatusNotFound, "file_backup_not_found", err.Error(), map[string]string{"path": path})
	default:
		writeCodedError(w, http.StatusInternalServerError, "file_backup_failed", err.Error(), map[string]string{"detail": err.Error()})
	}
}

func toFileBackupCurrent(path string) fileBackupCurrentJSON {
	st := syncpkg.CurrentFileState(path)
	return fileBackupCurrentJSON{Exists: st.Exists, LinkTo: st.LinkTo}
}

// handleFileBackupVersions — GET /api/file-backups/versions?path=
func (s *Server) handleFileBackupVersions(w http.ResponseWriter, r *http.Request) {
	path, ok := s.fileBackupPath(w, r.URL.Query().Get("path"))
	if !ok {
		return
	}
	versions, err := syncpkg.FileBackupVersions(path)
	if err != nil {
		writeFileBackupError(w, path, err)
		return
	}
	items := make([]fileBackupVersionJSON, 0, len(versions))
	for _, v := range versions {
		items = append(items, fileBackupVersionJSON{
			ID:      v.ID,
			Kind:    v.Kind,
			Reason:  v.Reason,
			Time:    v.Time.Format(time.RFC3339),
			Size:    v.Size,
			Preview: v.Preview,
			LinkTo:  v.LinkTo,
			None:    v.None,
		})
	}
	writeJSON(w, map[string]any{"path": path, "current": toFileBackupCurrent(path), "versions": items})
}

// handleFileBackupVersion — GET /api/file-backups/version?path=&id=
// Returns the version's content and the current file text for a diff.
func (s *Server) handleFileBackupVersion(w http.ResponseWriter, r *http.Request) {
	path, ok := s.fileBackupPath(w, r.URL.Query().Get("path"))
	if !ok {
		return
	}
	data, _, err := syncpkg.ReadFileBackupVersion(path, r.URL.Query().Get("id"))
	if err != nil {
		writeFileBackupError(w, path, err)
		return
	}
	// A link reads as its destination's text, which is what the diff replaces.
	current := ""
	if b, err := readLimited(path); err == nil {
		current = string(b)
	}
	writeJSON(w, map[string]any{"content": string(data), "current": current})
}

// handleRestoreFileBackup — POST /api/file-backups/restore {path, id, unlink}
func (s *Server) handleRestoreFileBackup(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var body struct {
		Path   string `json:"path"`
		ID     string `json:"id"`
		Unlink bool   `json:"unlink"`
	}
	if err := decodeJSON(w, r, &body, 64*1024); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
		}
		return
	}
	path, ok := s.fileBackupPath(w, body.Path)
	if !ok {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	saved, err := syncpkg.RestoreFileBackup(path, body.ID, body.Unlink)
	args := map[string]any{"action": "restore-file", "path": path, "id": body.ID, "scope": "ui"}
	if err != nil {
		var linkErr *syncpkg.FileBackupLinkError
		if !errors.As(err, &linkErr) && !errors.Is(err, syncpkg.ErrFileBackupNotFound) && !errors.Is(err, syncpkg.ErrInvalidFileBackupID) {
			s.writeOpsLog("restore", "error", start, args, err.Error())
		}
		writeFileBackupError(w, path, err)
		return
	}
	if saved != "" {
		args["backup_id"] = saved
	}
	s.writeOpsLog("restore", "ok", start, args, "")
	resp := map[string]any{"success": true}
	if saved != "" {
		resp["backup_id"] = saved
	}
	writeJSON(w, resp)
}
