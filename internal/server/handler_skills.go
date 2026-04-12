package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/git"
	"skillshare/internal/install"
	"skillshare/internal/resource"
	"skillshare/internal/sync"
	"skillshare/internal/trash"
	"skillshare/internal/utils"
)

type skillItem struct {
	Name        string   `json:"name"`
	Kind        string   `json:"kind"` // "skill" or "agent"
	FlatName    string   `json:"flatName"`
	RelPath     string   `json:"relPath"`
	SourcePath  string   `json:"sourcePath"`
	IsInRepo    bool     `json:"isInRepo"`
	Targets     []string `json:"targets,omitempty"`
	InstalledAt string   `json:"installedAt,omitempty"`
	Source      string   `json:"source,omitempty"`
	Type        string   `json:"type,omitempty"`
	RepoURL     string   `json:"repoUrl,omitempty"`
	Version     string   `json:"version,omitempty"`
	Branch      string   `json:"branch,omitempty"`
	Disabled    bool     `json:"disabled"`
}

// enrichSkillBranch fills item.Branch from metadata, falling back to
// git.GetCurrentBranch for tracked repos without branch in metadata.
func enrichSkillBranch(item *skillItem) {
	if item.Branch == "" && item.IsInRepo {
		if branch, err := git.GetCurrentBranch(item.SourcePath); err == nil {
			item.Branch = branch
		}
	}
}

func (s *Server) handleListSkills(w http.ResponseWriter, r *http.Request) {
	kindFilter := r.URL.Query().Get("kind") // "", "skill", "agent"

	// Snapshot config under RLock, then release before I/O.
	s.mu.RLock()
	source := s.cfg.Source
	agentsSource := s.agentsSource()
	s.mu.RUnlock()

	var items []skillItem

	// Skills
	if kindFilter == "" || kindFilter == "skill" {
		discovered, err := sync.DiscoverSourceSkillsAll(source)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		for _, d := range discovered {
			item := skillItem{
				Name:       filepath.Base(d.SourcePath),
				Kind:       "skill",
				FlatName:   d.FlatName,
				RelPath:    d.RelPath,
				SourcePath: d.SourcePath,
				IsInRepo:   d.IsInRepo,
				Targets:    d.Targets,
				Disabled:   d.Disabled,
			}

			if entry := s.skillsStore.GetByPath(d.RelPath); entry != nil {
				if !entry.InstalledAt.IsZero() {
					item.InstalledAt = entry.InstalledAt.Format(time.RFC3339)
				}
				item.Source = entry.Source
				item.Type = entry.Type
				item.RepoURL = entry.RepoURL
				item.Version = entry.Version
				item.Branch = entry.Branch
			}
			enrichSkillBranch(&item)

			items = append(items, item)
		}
	}

	// Agents — recursive discovery (supports --into subdirectories)
	if (kindFilter == "" || kindFilter == "agent") && agentsSource != "" {
		discovered, _ := resource.AgentKind{}.Discover(agentsSource)
		for _, d := range discovered {
			item := skillItem{
				Name:       d.Name,
				Kind:       "agent",
				FlatName:   d.FlatName,
				RelPath:    d.RelPath,
				SourcePath: d.SourcePath,
				IsInRepo:   d.IsInRepo,
				Disabled:   d.Disabled,
				Targets:    d.Targets,
			}

			// Read from centralized agents metadata store
			agentKey := strings.TrimSuffix(d.RelPath, ".md")
			if entry := s.agentsStore.GetByPath(agentKey); entry != nil {
				if !entry.InstalledAt.IsZero() {
					item.InstalledAt = entry.InstalledAt.Format(time.RFC3339)
				}
				item.Source = entry.Source
				item.Type = entry.Type
				item.RepoURL = entry.RepoURL
				item.Version = entry.Version
			} else if d.RepoRelPath != "" {
				repoPath := filepath.Join(agentsSource, filepath.FromSlash(d.RepoRelPath))
				if repoURL, err := git.GetRemoteURL(repoPath); err == nil {
					item.Source = repoURL
					item.RepoURL = repoURL
				}
				if version, err := git.GetCurrentFullHash(repoPath); err == nil {
					item.Version = version
				}
			}

			items = append(items, item)
		}
	}

	writeJSON(w, map[string]any{"resources": items})
}

func (s *Server) handleGetSkill(w http.ResponseWriter, r *http.Request) {
	// Snapshot config under RLock, then release before I/O.
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

	// Find the skill by flat name or base name
	if kind != "agent" {
		discovered, err := sync.DiscoverSourceSkillsAll(source)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		for _, d := range discovered {
			baseName := filepath.Base(d.SourcePath)
			if d.FlatName != name && baseName != name {
				continue
			}

			item := skillItem{
				Name:       baseName,
				Kind:       "skill",
				FlatName:   d.FlatName,
				RelPath:    d.RelPath,
				SourcePath: d.SourcePath,
				IsInRepo:   d.IsInRepo,
				Targets:    d.Targets,
				Disabled:   d.Disabled,
			}

			if entry := s.skillsStore.GetByPath(d.RelPath); entry != nil {
				if !entry.InstalledAt.IsZero() {
					item.InstalledAt = entry.InstalledAt.Format(time.RFC3339)
				}
				item.Source = entry.Source
				item.Type = entry.Type
				item.RepoURL = entry.RepoURL
				item.Version = entry.Version
				item.Branch = entry.Branch
			}
			enrichSkillBranch(&item)

			// Read SKILL.md content
			skillMdContent := ""
			skillMdPath := filepath.Join(d.SourcePath, "SKILL.md")
			if data, err := os.ReadFile(skillMdPath); err == nil {
				skillMdContent = string(data)
			}

			// List all files in the skill directory
			files := make([]string, 0)
			filepath.Walk(d.SourcePath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if info.IsDir() && utils.IsHidden(info.Name()) {
					return filepath.SkipDir
				}
				if !info.IsDir() {
					rel, _ := filepath.Rel(d.SourcePath, path)
					// Normalize separators
					rel = strings.ReplaceAll(rel, "\\", "/")
					files = append(files, rel)
				}
				return nil
			})

			writeJSON(w, map[string]any{
				"resource":       item,
				"skillMdContent": skillMdContent,
				"files":          files,
			})
			return
		}
	}

	// Fallback: check agents source (recursive — supports --into subdirectories)
	if kind != "skill" && agentsSource != "" {
		agentDiscovered, _ := resource.AgentKind{}.Discover(agentsSource)
		for _, d := range agentDiscovered {
			if !matchesAgentName(d, name) {
				continue
			}

			data, readErr := os.ReadFile(d.SourcePath)
			if readErr != nil {
				continue
			}

			item := skillItem{
				Name:       d.Name,
				Kind:       "agent",
				FlatName:   d.FlatName,
				RelPath:    d.RelPath,
				SourcePath: d.SourcePath,
				IsInRepo:   d.IsInRepo,
				Disabled:   d.Disabled,
				Targets:    d.Targets,
			}

			agentKey := strings.TrimSuffix(d.RelPath, ".md")
			if entry := s.agentsStore.GetByPath(agentKey); entry != nil {
				if !entry.InstalledAt.IsZero() {
					item.InstalledAt = entry.InstalledAt.Format(time.RFC3339)
				}
				item.Source = entry.Source
				item.Type = entry.Type
				item.RepoURL = entry.RepoURL
				item.Version = entry.Version
			} else if d.RepoRelPath != "" {
				repoPath := filepath.Join(agentsSource, filepath.FromSlash(d.RepoRelPath))
				if repoURL, err := git.GetRemoteURL(repoPath); err == nil {
					item.Source = repoURL
					item.RepoURL = repoURL
				}
				if version, err := git.GetCurrentFullHash(repoPath); err == nil {
					item.Version = version
				}
			}

			writeJSON(w, map[string]any{
				"resource":       item,
				"skillMdContent": string(data),
				"files":          []string{filepath.Base(d.RelPath)},
			})
			return
		}
	}

	writeError(w, http.StatusNotFound, "skill not found: "+name)
}

func (s *Server) handleGetSkillFile(w http.ResponseWriter, r *http.Request) {
	// Snapshot config under RLock, then release before I/O.
	s.mu.RLock()
	source := s.cfg.Source
	s.mu.RUnlock()

	name := r.PathValue("name")
	fp := r.PathValue("filepath")

	// Reject path traversal attempts
	if containsTraversalSegment(fp) {
		writeError(w, http.StatusBadRequest, "invalid file path")
		return
	}

	// Find the skill
	discovered, err := sync.DiscoverSourceSkills(source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	for _, d := range discovered {
		baseName := filepath.Base(d.SourcePath)
		if d.FlatName != name && baseName != name {
			continue
		}

		// Resolve and verify the file is within the skill directory
		absPath := filepath.Join(d.SourcePath, filepath.FromSlash(fp))
		absPath = filepath.Clean(absPath)
		skillDir := filepath.Clean(d.SourcePath) + string(filepath.Separator)
		if !strings.HasPrefix(absPath, skillDir) {
			writeError(w, http.StatusBadRequest, "invalid file path")
			return
		}
		if err := ensureResolvedPathWithinRoot(d.SourcePath, absPath); err != nil {
			switch {
			case errors.Is(err, errInvalidSkillFilePath):
				writeError(w, http.StatusBadRequest, "invalid file path")
			case errors.Is(err, os.ErrNotExist):
				writeError(w, http.StatusNotFound, "file not found: "+fp)
			default:
				writeError(w, http.StatusInternalServerError, "failed to read file: "+err.Error())
			}
			return
		}
		info, err := os.Lstat(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				writeError(w, http.StatusNotFound, "file not found: "+fp)
			} else {
				writeError(w, http.StatusInternalServerError, "failed to read file: "+err.Error())
			}
			return
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			writeError(w, http.StatusBadRequest, "invalid file path")
			return
		}

		data, err := os.ReadFile(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				writeError(w, http.StatusNotFound, "file not found: "+fp)
			} else {
				writeError(w, http.StatusInternalServerError, "failed to read file: "+err.Error())
			}
			return
		}

		// Determine content type from extension
		ct := "text/plain"
		switch strings.ToLower(filepath.Ext(absPath)) {
		case ".md":
			ct = "text/markdown"
		case ".json":
			ct = "application/json"
		case ".yaml", ".yml":
			ct = "text/yaml"
		}

		writeJSON(w, map[string]any{
			"content":     string(data),
			"contentType": ct,
			"filename":    filepath.Base(absPath),
		})
		return
	}

	writeError(w, http.StatusNotFound, "skill not found: "+name)
}

func (s *Server) handleSaveSkillFile(w http.ResponseWriter, r *http.Request) {
	// Snapshot config under RLock, then release before I/O.
	s.mu.RLock()
	source := s.cfg.Source
	s.mu.RUnlock()

	var req struct {
		Content *string `json:"content"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			writeError(w, http.StatusBadRequest, "invalid request body: multiple JSON values")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		}
		return
	}
	if req.Content == nil {
		writeError(w, http.StatusBadRequest, "invalid request body: missing content")
		return
	}

	name := r.PathValue("name")
	fp := r.PathValue("filepath")

	// Find the skill using the same discovery behavior as file read.
	discovered, err := sync.DiscoverSourceSkills(source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	for _, d := range discovered {
		baseName := filepath.Base(d.SourcePath)
		if d.FlatName != name && baseName != name {
			continue
		}

		filename, err := writeSkillFile(d.SourcePath, fp, []byte(*req.Content))
		if err != nil {
			switch {
			case errors.Is(err, errInvalidSkillFilePath):
				writeError(w, http.StatusBadRequest, "invalid file path")
			case errors.Is(err, os.ErrNotExist):
				writeError(w, http.StatusNotFound, "file not found: "+fp)
			default:
				writeError(w, http.StatusInternalServerError, "failed to save file: "+err.Error())
			}
			return
		}

		writeJSON(w, map[string]any{
			"filename": filename,
			"content":  *req.Content,
		})
		return
	}

	writeError(w, http.StatusNotFound, "skill not found: "+name)
}

func (s *Server) handleOpenSkillFile(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	source := s.cfg.Source
	s.mu.RUnlock()

	name := r.PathValue("name")
	fp := r.PathValue("filepath")

	if containsTraversalSegment(fp) {
		writeError(w, http.StatusBadRequest, "invalid file path")
		return
	}

	discovered, err := sync.DiscoverSourceSkills(source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	for _, d := range discovered {
		baseName := filepath.Base(d.SourcePath)
		if d.FlatName != name && baseName != name {
			continue
		}

		absPath := filepath.Join(d.SourcePath, filepath.FromSlash(fp))
		absPath = filepath.Clean(absPath)
		skillDir := filepath.Clean(d.SourcePath) + string(filepath.Separator)
		if !strings.HasPrefix(absPath, skillDir) {
			writeError(w, http.StatusBadRequest, "invalid file path")
			return
		}
		if err := ensureResolvedPathWithinRoot(d.SourcePath, absPath); err != nil {
			switch {
			case errors.Is(err, errInvalidSkillFilePath):
				writeError(w, http.StatusBadRequest, "invalid file path")
			case errors.Is(err, os.ErrNotExist):
				writeError(w, http.StatusNotFound, "file not found: "+fp)
			default:
				writeError(w, http.StatusInternalServerError, "failed to open file: "+err.Error())
			}
			return
		}
		info, err := os.Lstat(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				writeError(w, http.StatusNotFound, "file not found: "+fp)
			} else {
				writeError(w, http.StatusInternalServerError, "failed to open file: "+err.Error())
			}
			return
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			writeError(w, http.StatusBadRequest, "invalid file path")
			return
		}

		opener := s.openPath
		if opener == nil {
			opener = openLocalPath
		}
		if err := opener(absPath); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to open file: "+err.Error())
			return
		}

		writeJSON(w, map[string]any{
			"success":  true,
			"filename": filepath.Base(absPath),
		})
		return
	}

	writeError(w, http.StatusNotFound, "skill not found: "+name)
}

func (s *Server) handleUninstallRepo(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	name := strings.TrimSpace(r.PathValue("name"))
	cleanName := filepath.Clean(filepath.FromSlash(name))
	if name == "" || cleanName == "." || cleanName == ".." || filepath.IsAbs(cleanName) || strings.HasPrefix(cleanName, ".."+string(filepath.Separator)) {
		writeError(w, http.StatusBadRequest, "invalid or missing tracked repository name")
		return
	}

	repoName, repoPath, resolveErr := s.resolveTrackedRepo(cleanName)
	if resolveErr != nil {
		writeError(w, http.StatusBadRequest, resolveErr.Error())
		return
	}
	if repoPath == "" {
		writeError(w, http.StatusBadRequest, "not a tracked repository: "+cleanName)
		return
	}

	// Remove from .gitignore (project mode writes to .skillshare/.gitignore with "skills/" prefix)
	gitDir := s.gitignoreDir()
	if s.IsProjectMode() {
		install.RemoveFromGitIgnore(gitDir, filepath.Join("skills", repoName))
	} else {
		install.RemoveFromGitIgnore(gitDir, repoName)
	}

	// Move to trash instead of permanent delete
	if _, err := trash.MoveToTrash(repoPath, repoName, s.trashBase()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to trash repo: "+err.Error())
		return
	}

	// Prune store entries: the repo itself + skills belonging to it.
	// Legacy entries use Group without "_" prefix (e.g., "team-skills" for repo "_team-skills").
	// Only apply legacy matching for top-level repos (no "/" in repoName) to avoid
	// basename collisions between sibling nested repos like org/_team-skills vs dept/_team-skills.
	legacyGroup := ""
	if !strings.Contains(repoName, "/") {
		legacyGroup = strings.TrimPrefix(repoName, "_")
	}
	for _, name := range s.skillsStore.List() {
		entry := s.skillsStore.Get(name)
		if entry == nil {
			continue
		}
		// Match the repo's own entry (e.g., "_team-skills" or "org/_team-skills")
		if name == repoName {
			s.skillsStore.Remove(name)
			continue
		}
		// Match tracked skills grouped under this repo (exact group match)
		if entry.Tracked && entry.Group == repoName {
			s.skillsStore.Remove(name)
			continue
		}
		// Match legacy grouped entries (top-level repos only, e.g., group="team-skills")
		if legacyGroup != "" && entry.Tracked && entry.Group == legacyGroup {
			s.skillsStore.Remove(name)
			continue
		}
		// Match nested members (e.g., "org/_team-skills/sub-skill")
		if strings.HasPrefix(name, repoName+"/") {
			s.skillsStore.Remove(name)
			continue
		}
	}

	if err := s.skillsStore.Save(s.cfg.Source); err != nil {
		log.Printf("warning: failed to save metadata after repo uninstall: %v", err)
	}

	s.writeOpsLog("uninstall", "ok", start, map[string]any{
		"name":  repoName,
		"type":  "repo",
		"scope": "ui",
	}, "")

	writeJSON(w, map[string]any{"success": true, "name": repoName, "movedToTrash": true})
}

func (s *Server) handleUninstallSkill(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	name := r.PathValue("name")
	kind := r.URL.Query().Get("kind")
	if kind != "" && kind != "skill" && kind != "agent" {
		writeError(w, http.StatusBadRequest, "invalid kind: "+kind)
		return
	}

	if kind == "agent" {
		agentsSource := s.agentsSource()
		if agentsSource == "" {
			writeError(w, http.StatusNotFound, "agent not found: "+name)
			return
		}
		agent, err := resolveAgentResource(agentsSource, name)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}

		displayName := agentMetaKey(agent.RelPath)
		legacySidecar := filepath.Join(filepath.Dir(agent.SourcePath), filepath.Base(displayName)+".skillshare-meta.json")
		if _, err := trash.MoveAgentToTrash(agent.SourcePath, legacySidecar, displayName, s.agentTrashBase()); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to trash agent: "+err.Error())
			return
		}

		if s.agentsStore != nil {
			s.agentsStore.Remove(displayName)
			if err := s.agentsStore.Save(agentsSource); err != nil {
				log.Printf("warning: failed to save agent metadata after uninstall: %v", err)
			}
		}

		s.writeOpsLog("uninstall", "ok", start, map[string]any{
			"name":  displayName,
			"type":  "agent",
			"scope": "ui",
		}, "")

		writeJSON(w, map[string]any{"success": true, "name": displayName, "movedToTrash": true})
		return
	}

	// Find skill path
	discovered, err := sync.DiscoverSourceSkills(s.cfg.Source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	for _, d := range discovered {
		baseName := filepath.Base(d.SourcePath)
		if d.FlatName != name && baseName != name {
			continue
		}

		// Don't allow removing skills inside tracked repos
		if d.IsInRepo {
			writeError(w, http.StatusBadRequest, "cannot uninstall skill from tracked repo; use 'skillshare uninstall' for the whole repo")
			return
		}

		if _, err := trash.MoveToTrash(d.SourcePath, baseName, s.trashBase()); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to trash skill: "+err.Error())
			return
		}

		s.writeOpsLog("uninstall", "ok", start, map[string]any{
			"name":  baseName,
			"type":  "skill",
			"scope": "ui",
		}, "")

		writeJSON(w, map[string]any{"success": true, "name": name, "movedToTrash": true})
		return
	}

	writeError(w, http.StatusNotFound, "skill not found: "+name)
}

// resolveTrackedRepo resolves a repo name (flat or nested) to its directory name
// and absolute path under s.cfg.Source. Returns ("", "", nil) if not found.
// Returns a non-nil error for ambiguous matches or internal failures.
func (s *Server) resolveTrackedRepo(input string) (string, string, error) {
	sourceRoot := filepath.Clean(s.cfg.Source)
	candidates := []string{input}
	if !strings.HasPrefix(filepath.Base(input), "_") {
		if dir := filepath.Dir(input); dir != "." && dir != "" {
			candidates = append(candidates, filepath.Join(dir, "_"+filepath.Base(input)))
		} else {
			candidates = append(candidates, "_"+input)
		}
	}
	for _, candidate := range candidates {
		repoPath := filepath.Clean(filepath.Join(sourceRoot, candidate))
		relPath, relErr := filepath.Rel(sourceRoot, repoPath)
		if relErr != nil || relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
			continue
		}
		if install.IsGitRepo(repoPath) {
			return candidate, repoPath, nil
		}
	}

	// Fallback: match nested tracked repos by basename.
	repos, err := install.GetTrackedRepos(s.cfg.Source)
	if err != nil {
		return "", "", fmt.Errorf("failed to list tracked repositories: %w", err)
	}
	var match string
	for _, repo := range repos {
		base := filepath.Base(repo)
		trimmed := strings.TrimPrefix(base, "_")
		if base == input || trimmed == input {
			if match != "" {
				return "", "", fmt.Errorf("multiple tracked repositories match: %s — use the full path", input)
			}
			match = repo
		}
	}
	if match != "" {
		return match, filepath.Join(sourceRoot, match), nil
	}
	return "", "", nil
}
