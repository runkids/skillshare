package server

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"skillshare/internal/sourcefs"
	"skillshare/internal/trash"
)

type trashKind string

const (
	trashKindAll   trashKind = "all"
	trashKindSkill trashKind = "skill"
	trashKindAgent trashKind = "agent"
)

type trashItemJSON struct {
	Name      string `json:"name"`
	Kind      string `json:"kind,omitempty"`
	Timestamp string `json:"timestamp"`
	Date      string `json:"date"`
	Size      int64  `json:"size"`
	Path      string `json:"path"`
}

type resolvedTrashEntry struct {
	entry *trash.TrashEntry
	kind  trashKind
	dest  string
}

// trashBase returns the trash directory for the current mode.
func (s *Server) trashBase() string {
	if s.IsProjectMode() {
		return trash.ProjectTrashDir(s.projectRoot)
	}
	return trash.TrashDir()
}

// agentTrashBase returns the agent trash directory for the current mode.
func (s *Server) agentTrashBase() string {
	if s.IsProjectMode() {
		return trash.ProjectAgentTrashDir(s.projectRoot)
	}
	return trash.AgentTrashDir()
}

func parseTrashKind(raw string) (trashKind, error) {
	switch raw {
	case "", "all":
		return trashKindAll, nil
	case "skill", "skills":
		return trashKindSkill, nil
	case "agent", "agents":
		return trashKindAgent, nil
	default:
		return "", fmt.Errorf("invalid trash kind %q", raw)
	}
}

func (s *Server) trashDest(kind trashKind) string {
	switch kind {
	case trashKindAgent:
		return s.agentsSource()
	default:
		return s.skillsSource()
	}
}

func (s *Server) findTrashEntry(name string, kind trashKind) (*resolvedTrashEntry, error) {
	if kind == trashKindSkill || kind == trashKindAll {
		base := s.trashBase()
		if entry := trash.FindByName(base, name); entry != nil {
			return &resolvedTrashEntry{
				entry: entry,
				kind:  trashKindSkill,
				dest:  s.trashDest(trashKindSkill),
			}, nil
		}
	}

	if kind == trashKindAgent || kind == trashKindAll {
		base := s.agentTrashBase()
		if entry := trash.FindByName(base, name); entry != nil {
			return &resolvedTrashEntry{
				entry: entry,
				kind:  trashKindAgent,
				dest:  s.trashDest(trashKindAgent),
			}, nil
		}
	}

	return nil, nil
}

// handleListTrash returns all trashed items with total size.
func (s *Server) handleListTrash(w http.ResponseWriter, r *http.Request) {
	// Snapshot config under RLock, then release before I/O.
	s.mu.RLock()
	base := s.trashBase()
	agentBase := s.agentTrashBase()
	s.mu.RUnlock()

	items := trash.List(base)
	agentItems := trash.List(agentBase)

	out := make([]trashItemJSON, 0, len(items)+len(agentItems))
	for _, item := range items {
		out = append(out, trashItemJSON{
			Name:      item.Name,
			Kind:      "skill",
			Timestamp: item.Timestamp,
			Date:      item.Date.Format("2006-01-02T15:04:05Z07:00"),
			Size:      item.Size,
			Path:      item.Path,
		})
	}
	for _, item := range agentItems {
		out = append(out, trashItemJSON{
			Name:      item.Name,
			Kind:      "agent",
			Timestamp: item.Timestamp,
			Date:      item.Date.Format("2006-01-02T15:04:05Z07:00"),
			Size:      item.Size,
			Path:      item.Path,
		})
	}

	totalSize := trash.TotalSize(base) + trash.TotalSize(agentBase)
	writeJSON(w, map[string]any{
		"items":     out,
		"totalSize": totalSize,
	})
}

// handleRestoreTrash restores a trashed skill or agent back to its source directory.
func (s *Server) handleRestoreTrash(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	name := r.PathValue("name")
	kind, err := parseTrashKind(r.URL.Query().Get("kind"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	resolved, err := s.findTrashEntry(name, kind)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve trashed item: "+err.Error())
		return
	}
	if resolved == nil {
		writeError(w, http.StatusNotFound, "trashed item not found: "+name)
		return
	}

	switch resolved.kind {
	case trashKindAgent:
		err = trash.RestoreAgent(resolved.entry, resolved.dest)
	default:
		err = trash.Restore(resolved.entry, resolved.dest)
	}
	if errors.Is(err, sourcefs.ErrLink) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to restore: "+err.Error())
		return
	}

	s.writeOpsLog("trash", "ok", start, map[string]any{
		"action": "restore",
		"name":   name,
		"kind":   string(resolved.kind),
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
	kind, err := parseTrashKind(r.URL.Query().Get("kind"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	resolved, err := s.findTrashEntry(name, kind)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve trashed item: "+err.Error())
		return
	}
	if resolved == nil {
		writeError(w, http.StatusNotFound, "trashed item not found: "+name)
		return
	}

	if err := os.RemoveAll(resolved.entry.Path); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete: "+err.Error())
		return
	}

	s.writeOpsLog("trash", "ok", start, map[string]any{
		"action": "delete",
		"name":   name,
		"kind":   string(resolved.kind),
		"scope":  "ui",
	}, "")

	writeJSON(w, map[string]any{"success": true})
}

// handleEmptyTrash permanently deletes all trashed items.
func (s *Server) handleEmptyTrash(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	kind, err := parseTrashKind(r.URL.Query().Get("kind"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	type emptyTarget struct {
		base string
	}
	targets := make([]emptyTarget, 0, 2)
	if kind == trashKindAll || kind == trashKindSkill {
		targets = append(targets, emptyTarget{base: s.trashBase()})
	}
	if kind == trashKindAll || kind == trashKindAgent {
		targets = append(targets, emptyTarget{base: s.agentTrashBase()})
	}

	removed := 0

	for _, target := range targets {
		items := trash.List(target.base)
		for _, item := range items {
			if err := os.RemoveAll(item.Path); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to empty trash: "+err.Error())
				return
			}
			removed++
		}
	}

	s.writeOpsLog("trash", "ok", start, map[string]any{
		"action":  "empty",
		"kind":    string(kind),
		"removed": removed,
		"scope":   "ui",
	}, "")

	writeJSON(w, map[string]any{"success": true, "removed": removed})
}
