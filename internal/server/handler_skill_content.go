package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/resource"
	"skillshare/internal/sync"
)

// skillContentRequest is the JSON body for PUT /api/resources/{name}/content
type skillContentRequest struct {
	Content string `json:"content"`
}

// skillContentResponse is the JSON response on successful save.
type skillContentResponse struct {
	BytesWritten int    `json:"bytesWritten"`
	Path         string `json:"path"`
	ContentType  string `json:"contentType"`
	SavedAt      string `json:"savedAt"`
}

// handlePutSkillContent saves the SKILL.md (or agent markdown) content to disk.
// Request body: {"content": "..."}
// Writes atomically via <file>.tmp + rename.
func (s *Server) handlePutSkillContent(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	s.mu.RLock()
	source := s.skillsSource()
	agentsSource := s.agentsSource()
	s.mu.RUnlock()

	name := r.PathValue("name")
	kind := r.URL.Query().Get("kind")
	if kind != "" && kind != "skill" && kind != "agent" {
		writeError(w, http.StatusBadRequest, "invalid kind: "+kind)
		return
	}

	var req skillContentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	// Reject writes above a conservative ceiling to guard against accidents.
	const maxBytes = 2 * 1024 * 1024 // 2 MiB
	if len(req.Content) > maxBytes {
		writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("content exceeds %d bytes", maxBytes))
		return
	}

	targetPath, resolvedKind, err := s.resolveEditableSkillPath(source, agentsSource, name, kind)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	if err := writeFileAtomic(targetPath, []byte(req.Content), 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save: "+err.Error())
		return
	}

	s.writeOpsLog("skill.edit", "ok", start, map[string]any{
		"name":   name,
		"kind":   resolvedKind,
		"path":   targetPath,
		"length": len(req.Content),
	}, "")

	writeJSON(w, skillContentResponse{
		BytesWritten: len(req.Content),
		Path:         targetPath,
		ContentType:  "text/markdown",
		SavedAt:      time.Now().UTC().Format(time.RFC3339),
	})
}

// resolveEditableSkillPath locates the on-disk markdown file for a skill or agent.
// For skills this is <skillDir>/SKILL.md; for agents this is the agent's single .md file.
// Returns (absPath, resolvedKind, error).
func (s *Server) resolveEditableSkillPath(source, agentsSource, name, kind string) (string, string, error) {
	if kind != "agent" && source != "" {
		discovered, err := sync.DiscoverSourceSkillsAll(source)
		if err == nil {
			for _, d := range discovered {
				baseName := filepath.Base(d.SourcePath)
				if d.FlatName != name && baseName != name {
					continue
				}
				skillMd := filepath.Join(d.SourcePath, "SKILL.md")
				if !withinDir(skillMd, d.SourcePath) {
					return "", "", fmt.Errorf("invalid skill path")
				}
				return skillMd, "skill", nil
			}
		}
	}

	if kind != "skill" && agentsSource != "" {
		agents, _ := resource.AgentKind{}.Discover(agentsSource)
		for _, d := range agents {
			if !matchesAgentName(d, name) {
				continue
			}
			if !withinDir(d.SourcePath, agentsSource) {
				return "", "", fmt.Errorf("invalid agent path")
			}
			return d.SourcePath, "agent", nil
		}
	}

	return "", "", fmt.Errorf("skill not found: %s", name)
}

// withinDir reports whether path is inside (or equal to) dir.
// Uses filepath.Rel for correctness on case-insensitive filesystems.
func withinDir(path, dir string) bool {
	rel, err := filepath.Rel(filepath.Clean(dir), filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel))
}

// writeFileAtomic writes data to a temp file in the same directory, then renames it
// into place. This prevents partial writes if the process is killed mid-write.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".skillshare-"+filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("chmod temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("sync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return fmt.Errorf("rename temp: %w", err)
	}
	return nil
}
