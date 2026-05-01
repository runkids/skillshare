package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/git"
	"skillshare/internal/install"
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
	source := s.cfg.Source
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

// patchSourceRequest is the JSON body for PATCH /api/resources/{name}/source.
type patchSourceRequest struct {
	Source string `json:"source"`
}

// handlePatchSkillSource updates the source URL for a tracked skill or agent.
// It updates the metadata store entry and, for tracked repos with a .git directory,
// also updates the git remote origin URL.
func (s *Server) handlePatchSkillSource(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	name := r.PathValue("name")
	kind := r.URL.Query().Get("kind") // optional: "skill" or "agent"

	var req patchSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	req.Source = strings.TrimSpace(req.Source)
	if req.Source == "" {
		writeError(w, http.StatusBadRequest, "source cannot be empty")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	parsed, parseErr := install.ParseSourceWithOptions(req.Source, s.parseOpts())
	if parseErr != nil {
		writeError(w, http.StatusBadRequest, "invalid source: "+parseErr.Error())
		return
	}

	newRepoURL := req.Source
	if parsed.IsGit() {
		newRepoURL = parsed.CloneURL
	}

	source := s.cfg.Source
	agentsSource := s.agentsSource()

	m := s.findMetadataEntry(name, kind, source, agentsSource)
	if m == nil {
		writeError(w, http.StatusNotFound, "resource not found: "+name)
		return
	}
	entry := m.Entry
	if entry == nil {
		entry = &install.MetadataEntry{}
		m.Store.Set(m.RelPath, entry)
	}

	// Detect tracked repo by walking up for .git, bounded by storeDir.
	// This handles both direct tracked repos (IsInRepo=true) and --into repos.
	repoRoot := findRepoRoot(m.SourcePath, m.StoreDir)
	isTracked := repoRoot != ""

	// For tracked repos with missing metadata, infer RepoURL from git remote.
	oldRepoURL := entry.RepoURL
	if oldRepoURL == "" && isTracked {
		if remoteURL, err := git.GetRemoteURL(repoRoot); err == nil && remoteURL != "" {
			oldRepoURL = remoteURL
		}
	}

	oldSource := entry.Source
	updated := 0

	// For tracked repos, update ALL skills sharing the same git repo.
	if oldRepoURL != "" && isTracked {
		oldBase := strings.TrimSuffix(oldRepoURL, ".git")
		newBase := strings.TrimSuffix(newRepoURL, ".git")
		for _, key := range m.Store.List() {
			e := m.Store.Get(key)
			if e == nil || e.RepoURL != oldRepoURL {
				continue
			}
			e.RepoURL = newRepoURL
			if oldBase != newBase {
				e.Source = strings.Replace(e.Source, oldBase, newBase, 1)
			}
			updated++
		}
		// Also update the entry itself if it was freshly created (no RepoURL yet).
		if entry.RepoURL != newRepoURL {
			entry.Source = req.Source
			entry.RepoURL = newRepoURL
			updated++
		}
	} else {
		entry.Source = req.Source
		entry.RepoURL = newRepoURL
		updated = 1
	}

	if err := m.Store.Save(m.StoreDir); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save metadata: "+err.Error())
		return
	}

	// Update git remote origin if this is a tracked repo.
	if repoRoot != "" {
		_ = git.SetRemoteURL(repoRoot, newRepoURL)
	}

	s.writeOpsLog("skill.source", "ok", start, map[string]any{
		"name":      name,
		"kind":      m.Kind,
		"oldSource": oldSource,
		"newSource": req.Source,
		"updated":   updated,
	}, "")

	writeJSON(w, map[string]any{
		"success": true,
		"source":  entry.Source,
		"repoUrl": entry.RepoURL,
		"updated": updated,
	})
}

// metadataLookup holds the result of findMetadataEntry.
type metadataLookup struct {
	Store      *install.MetadataStore
	StoreDir   string
	Entry      *install.MetadataEntry // nil if resource exists on disk but has no metadata
	RelPath    string
	SourcePath string
	IsInRepo   bool
	Kind       string // "skill" or "agent"
}

// findMetadataEntry looks up a metadata entry by name across skills and agents stores.
func (s *Server) findMetadataEntry(name, kind, source, agentsSource string) *metadataLookup {
	if kind != "agent" && source != "" {
		discovered, err := sync.DiscoverSourceSkillsAll(source)
		if err == nil {
			for _, d := range discovered {
				if d.FlatName != name && filepath.Base(d.SourcePath) != name {
					continue
				}
				return &metadataLookup{
					Store: s.skillsStore, StoreDir: source,
					Entry:   s.skillsStore.GetByPath(d.RelPath),
					RelPath: d.RelPath, SourcePath: d.SourcePath,
					IsInRepo: d.IsInRepo, Kind: "skill",
				}
			}
		}
	}

	if kind != "skill" && agentsSource != "" {
		agents, _ := resource.AgentKind{}.Discover(agentsSource)
		for _, d := range agents {
			if d.FlatName != name && d.Name != name {
				continue
			}
			agentKey := strings.TrimSuffix(d.RelPath, ".md")
			return &metadataLookup{
				Store: s.agentsStore, StoreDir: agentsSource,
				Entry:   s.agentsStore.GetByPath(agentKey),
				RelPath: agentKey, SourcePath: filepath.Dir(d.SourcePath),
				IsInRepo: d.IsInRepo, Kind: "agent",
			}
		}
	}

	return nil
}

// findRepoRoot walks up from path looking for a .git directory, bounded by root.
// Returns the directory containing .git, or "" if none found.
func findRepoRoot(path, root string) string {
	cleanRoot := filepath.Clean(root)
	for dir := path; dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		if !strings.HasPrefix(filepath.Clean(dir), cleanRoot) {
			break
		}
		if fi, err := os.Stat(filepath.Join(dir, ".git")); err == nil && fi.IsDir() {
			return dir
		}
	}
	return ""
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
