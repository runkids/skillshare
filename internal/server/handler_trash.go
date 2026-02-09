package server

import (
	"net/http"
	"os"
	"time"

	"skillshare/internal/trash"
)

type trashItemJSON struct {
	Name      string `json:"name"`
	Timestamp string `json:"timestamp"`
	Date      string `json:"date"`
	Size      int64  `json:"size"`
	Path      string `json:"path"`
}

// trashBase returns the trash directory for the current mode.
func (s *Server) trashBase() string {
	if s.IsProjectMode() {
		return trash.ProjectTrashDir(s.projectRoot)
	}
	return trash.TrashDir()
}

// handleListTrash returns all trashed items with total size.
func (s *Server) handleListTrash(w http.ResponseWriter, r *http.Request) {
	base := s.trashBase()
	items := trash.List(base)

	out := make([]trashItemJSON, 0, len(items))
	for _, item := range items {
		out = append(out, trashItemJSON{
			Name:      item.Name,
			Timestamp: item.Timestamp,
			Date:      item.Date.Format("2006-01-02T15:04:05Z07:00"),
			Size:      item.Size,
			Path:      item.Path,
		})
	}

	writeJSON(w, map[string]any{
		"items":     out,
		"totalSize": trash.TotalSize(base),
	})
}

// handleRestoreTrash restores a trashed skill back to the source directory.
func (s *Server) handleRestoreTrash(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	name := r.PathValue("name")
	base := s.trashBase()

	entry := trash.FindByName(base, name)
	if entry == nil {
		writeError(w, http.StatusNotFound, "trashed item not found: "+name)
		return
	}

	if err := trash.Restore(entry, s.cfg.Source); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to restore: "+err.Error())
		return
	}

	s.writeOpsLog("trash", "ok", start, map[string]any{
		"action": "restore",
		"name":   name,
		"scope":  "ui",
	}, "")

	writeJSON(w, map[string]any{"success": true})
}

// handleDeleteTrash permanently deletes a single trashed item.
func (s *Server) handleDeleteTrash(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	name := r.PathValue("name")
	base := s.trashBase()

	entry := trash.FindByName(base, name)
	if entry == nil {
		writeError(w, http.StatusNotFound, "trashed item not found: "+name)
		return
	}

	if err := os.RemoveAll(entry.Path); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete: "+err.Error())
		return
	}

	s.writeOpsLog("trash", "ok", start, map[string]any{
		"action": "delete",
		"name":   name,
		"scope":  "ui",
	}, "")

	writeJSON(w, map[string]any{"success": true})
}

// handleEmptyTrash permanently deletes all trashed items.
func (s *Server) handleEmptyTrash(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	base := s.trashBase()
	items := trash.List(base)
	removed := 0

	for _, item := range items {
		if err := os.RemoveAll(item.Path); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to empty trash: "+err.Error())
			return
		}
		removed++
	}

	s.writeOpsLog("trash", "ok", start, map[string]any{
		"action":  "empty",
		"removed": removed,
		"scope":   "ui",
	}, "")

	writeJSON(w, map[string]any{"success": true, "removed": removed})
}
