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
)

type updateRequest struct {
	Name  string `json:"name"`
	Force bool   `json:"force"`
	All   bool   `json:"all"`
}

type updateResultItem struct {
	Name    string `json:"name"`
	Action  string `json:"action"` // "updated", "up-to-date", "skipped", "error"
	Message string `json:"message,omitempty"`
	IsRepo  bool   `json:"isRepo"`
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	var body updateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if body.All {
		results := s.updateAll(body.Force)
		total := len(results)
		failed := 0
		for _, item := range results {
			if item.Action == "error" {
				failed++
			}
		}
		status := "ok"
		msg := ""
		if failed > 0 {
			status = "partial"
			msg = fmt.Sprintf("%d update(s) failed", failed)
		}
		s.writeOpsLog("update", status, start, map[string]any{
			"name":           "--all",
			"force":          body.Force,
			"results_total":  total,
			"results_failed": failed,
			"scope":          "ui",
		}, msg)
		writeJSON(w, map[string]any{"results": results})
		return
	}

	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required (or use all: true)")
		return
	}

	result := s.updateSingle(body.Name, body.Force)
	status := "ok"
	msg := ""
	if result.Action == "error" {
		status = "error"
		msg = result.Message
	} else if result.Action == "skipped" {
		status = "partial"
		msg = result.Message
	}
	s.writeOpsLog("update", status, start, map[string]any{
		"name":  body.Name,
		"force": body.Force,
		"scope": "ui",
	}, msg)
	writeJSON(w, map[string]any{"results": []updateResultItem{result}})
}

func (s *Server) updateSingle(name string, force bool) updateResultItem {
	// Try tracked repo first (with _ prefix)
	repoName := name
	if !strings.HasPrefix(repoName, "_") {
		repoName = "_" + name
	}
	repoPath := filepath.Join(s.cfg.Source, repoName)

	if install.IsGitRepo(repoPath) {
		return s.updateTrackedRepo(repoName, repoPath, force)
	}

	// Try as regular skill
	skillPath := filepath.Join(s.cfg.Source, name)
	if meta, _ := install.ReadMeta(skillPath); meta != nil && meta.Source != "" {
		return s.updateRegularSkill(name, skillPath)
	}

	// Try original name as git repo path
	origPath := filepath.Join(s.cfg.Source, name)
	if install.IsGitRepo(origPath) {
		return s.updateTrackedRepo(name, origPath, force)
	}

	return updateResultItem{
		Name:    name,
		Action:  "error",
		Message: fmt.Sprintf("'%s' not found as tracked repo or updatable skill", name),
	}
}

func (s *Server) updateTrackedRepo(name, repoPath string, force bool) updateResultItem {
	// Check for uncommitted changes
	if isDirty, _ := git.IsDirty(repoPath); isDirty {
		if !force {
			return updateResultItem{
				Name:    name,
				Action:  "skipped",
				Message: "has uncommitted changes (use force to discard)",
				IsRepo:  true,
			}
		}
		if err := git.Restore(repoPath); err != nil {
			return updateResultItem{
				Name:    name,
				Action:  "error",
				Message: "failed to discard changes: " + err.Error(),
				IsRepo:  true,
			}
		}
	}

	var info *git.UpdateInfo
	var err error
	if force {
		info, err = git.ForcePull(repoPath)
	} else {
		info, err = git.Pull(repoPath)
	}
	if err != nil {
		return updateResultItem{
			Name:    name,
			Action:  "error",
			Message: err.Error(),
			IsRepo:  true,
		}
	}

	if info.UpToDate {
		return updateResultItem{Name: name, Action: "up-to-date", IsRepo: true}
	}

	return updateResultItem{
		Name:    name,
		Action:  "updated",
		Message: fmt.Sprintf("%d commits, %d files changed", len(info.Commits), info.Stats.FilesChanged),
		IsRepo:  true,
	}
}

func (s *Server) updateRegularSkill(name, skillPath string) updateResultItem {
	meta, _ := install.ReadMeta(skillPath)
	source, err := install.ParseSource(meta.Source)
	if err != nil {
		return updateResultItem{
			Name:    name,
			Action:  "error",
			Message: "invalid source: " + err.Error(),
		}
	}

	opts := install.InstallOptions{Force: true, Update: true}
	if _, err = install.Install(source, skillPath, opts); err != nil {
		return updateResultItem{
			Name:    name,
			Action:  "error",
			Message: err.Error(),
		}
	}

	return updateResultItem{
		Name:    name,
		Action:  "updated",
		Message: "reinstalled from source",
	}
}

func (s *Server) updateAll(force bool) []updateResultItem {
	var results []updateResultItem

	// Update tracked repos
	repos, err := install.GetTrackedRepos(s.cfg.Source)
	if err == nil {
		for _, repo := range repos {
			repoPath := filepath.Join(s.cfg.Source, repo)
			results = append(results, s.updateTrackedRepo(repo, repoPath, force))
		}
	}

	// Update regular skills with source metadata
	skills, err := getServerUpdatableSkills(s.cfg.Source)
	if err == nil {
		for _, skill := range skills {
			skillPath := filepath.Join(s.cfg.Source, skill)
			results = append(results, s.updateRegularSkill(skill, skillPath))
		}
	}

	return results
}

// getServerUpdatableSkills returns relative paths of skills that have metadata with a remote source.
// It walks the source directory recursively to find nested skills (e.g. utils/ascii-box-check).
func getServerUpdatableSkills(sourceDir string) ([]string, error) {
	var skills []string
	err := filepath.WalkDir(sourceDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == sourceDir {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		name := d.Name()
		// Skip hidden directories and .git
		if name == ".git" || (len(name) > 0 && name[0] == '.') {
			return filepath.SkipDir
		}
		// Skip tracked repos (_ prefix with .git inside)
		if len(name) > 0 && name[0] == '_' {
			return filepath.SkipDir
		}
		// Check if this directory has updatable metadata
		meta, metaErr := install.ReadMeta(path)
		if metaErr != nil || meta == nil || meta.Source == "" {
			return nil // continue walking into subdirectories
		}
		relPath, relErr := filepath.Rel(sourceDir, path)
		if relErr == nil {
			skills = append(skills, filepath.ToSlash(relPath))
		}
		return filepath.SkipDir // don't recurse into skill directories
	})
	if err != nil {
		return nil, err
	}
	return skills, nil
}
