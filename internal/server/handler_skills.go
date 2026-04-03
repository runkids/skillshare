package server

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/git"
	"skillshare/internal/install"
	"skillshare/internal/sync"
	"skillshare/internal/trash"
	"skillshare/internal/utils"
)

type skillItem struct {
	Name        string   `json:"name"`
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
	// Snapshot config under RLock, then release before I/O.
	s.mu.RLock()
	source := s.cfg.Source
	s.mu.RUnlock()

	discovered, err := sync.DiscoverSourceSkillsAll(source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]skillItem, 0, len(discovered))
	for _, d := range discovered {
		item := skillItem{
			Name:       filepath.Base(d.SourcePath),
			FlatName:   d.FlatName,
			RelPath:    d.RelPath,
			SourcePath: d.SourcePath,
			IsInRepo:   d.IsInRepo,
			Targets:    d.Targets,
			Disabled:   d.Disabled,
		}

		// Enrich with metadata if available
		if meta, _ := install.ReadMeta(d.SourcePath); meta != nil {
			item.InstalledAt = meta.InstalledAt.Format("2006-01-02T15:04:05Z")
			item.Source = meta.Source
			item.Type = meta.Type
			item.RepoURL = meta.RepoURL
			item.Version = meta.Version
			item.Branch = meta.Branch
		}
		enrichSkillBranch(&item)

		items = append(items, item)
	}

	writeJSON(w, map[string]any{"skills": items})
}

func (s *Server) handleGetSkill(w http.ResponseWriter, r *http.Request) {
	// Snapshot config under RLock, then release before I/O.
	s.mu.RLock()
	source := s.cfg.Source
	s.mu.RUnlock()

	name := r.PathValue("name")

	// Find the skill by flat name or base name
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
			FlatName:   d.FlatName,
			RelPath:    d.RelPath,
			SourcePath: d.SourcePath,
			IsInRepo:   d.IsInRepo,
			Targets:    d.Targets,
			Disabled:   d.Disabled,
		}

		if meta, _ := install.ReadMeta(d.SourcePath); meta != nil {
			item.InstalledAt = meta.InstalledAt.Format("2006-01-02T15:04:05Z")
			item.Source = meta.Source
			item.Type = meta.Type
			item.RepoURL = meta.RepoURL
			item.Version = meta.Version
			item.Branch = meta.Branch
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
			"skill":          item,
			"skillMdContent": skillMdContent,
			"files":          files,
		})
		return
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
	if strings.Contains(fp, "..") {
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

	// Remove from .gitignore
	install.RemoveFromGitIgnore(s.cfg.Source, repoName)

	// Move to trash instead of permanent delete
	if _, err := trash.MoveToTrash(repoPath, repoName, s.trashBase()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to trash repo: "+err.Error())
		return
	}

	// Prune registry entries: the repo itself + skills belonging to it
	trimmedGroup := strings.TrimPrefix(filepath.Base(repoName), "_")
	filtered := make([]config.SkillEntry, 0, len(s.registry.Skills))
	for _, entry := range s.registry.Skills {
		fullName := entry.FullName()
		// Match the repo's own entry (e.g., "_team-skills" or "org/_team-skills")
		if fullName == repoName {
			continue
		}
		// Match tracked skills grouped under this repo (group="team-skills" for repo "_team-skills")
		if entry.Tracked && entry.Group == trimmedGroup {
			continue
		}
		// Match nested members (e.g., "org/_team-skills/sub-skill")
		if strings.HasPrefix(fullName, repoName+"/") {
			continue
		}
		filtered = append(filtered, entry)
	}
	s.registry.Skills = filtered

	regDir := s.cfg.RegistryDir
	if s.IsProjectMode() {
		regDir = filepath.Join(s.projectRoot, ".skillshare")
	}
	if err := s.registry.Save(regDir); err != nil {
		log.Printf("warning: failed to save registry after repo uninstall: %v", err)
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
