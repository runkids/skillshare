package server

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"skillshare/internal/config"
	ssync "skillshare/internal/sync"
	"skillshare/internal/utils"
)

type targetItem struct {
	Name               string   `json:"name"`
	Path               string   `json:"path"`
	Mode               string   `json:"mode"`
	Status             string   `json:"status"`
	LinkedCount        int      `json:"linkedCount"`
	LocalCount         int      `json:"localCount"`
	Include            []string `json:"include"`
	Exclude            []string `json:"exclude"`
	ExpectedSkillCount int      `json:"expectedSkillCount"`
}

func (s *Server) handleListTargets(w http.ResponseWriter, r *http.Request) {
	items := make([]targetItem, 0, len(s.cfg.Targets))
	discovered, discoveredErr := ssync.DiscoverSourceSkills(s.cfg.Source)

	globalMode := s.cfg.Mode
	if globalMode == "" {
		globalMode = "merge"
	}

	for name, target := range s.cfg.Targets {
		mode := target.Mode
		if mode == "" {
			mode = globalMode
		}

		item := targetItem{
			Name: name,
			Path: target.Path,
			Mode: mode,
			Include: func() []string {
				if len(target.Include) == 0 {
					return []string{}
				}
				return append([]string(nil), target.Include...)
			}(),
			Exclude: func() []string {
				if len(target.Exclude) == 0 {
					return []string{}
				}
				return append([]string(nil), target.Exclude...)
			}(),
		}

		switch mode {
		case "merge":
			if discoveredErr == nil {
				filtered, err := ssync.FilterSkills(discovered, target.Include, target.Exclude)
				if err != nil {
					writeError(w, http.StatusBadRequest, "invalid include/exclude for target "+name+": "+err.Error())
					return
				}
				filtered = ssync.FilterSkillsByTarget(filtered, name)
				item.ExpectedSkillCount = len(filtered)
			}
			status, linked, local := ssync.CheckStatusMerge(target.Path, s.cfg.Source)
			item.Status = status.String()
			item.LinkedCount = linked
			item.LocalCount = local
		case "copy":
			if discoveredErr == nil {
				filtered, err := ssync.FilterSkills(discovered, target.Include, target.Exclude)
				if err != nil {
					writeError(w, http.StatusBadRequest, "invalid include/exclude for target "+name+": "+err.Error())
					return
				}
				filtered = ssync.FilterSkillsByTarget(filtered, name)
				item.ExpectedSkillCount = len(filtered)
			}
			status, managed, local := ssync.CheckStatusCopy(target.Path)
			item.Status = status.String()
			item.LinkedCount = managed // reuse field for managed count
			item.LocalCount = local
		default:
			status := ssync.CheckStatus(target.Path, s.cfg.Source)
			item.Status = status.String()
		}

		items = append(items, item)
	}

	// Count source skills for drift detection
	sourceSkillCount := 0
	if discoveredErr == nil {
		sourceSkillCount = len(discovered)
	}

	writeJSON(w, map[string]any{"targets": items, "sourceSkillCount": sourceSkillCount})
}

func (s *Server) handleAddTarget(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	var body struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	// In project mode, path can be resolved from known targets
	if body.Path == "" {
		if s.IsProjectMode() {
			if known, ok := config.LookupProjectTarget(body.Name); ok {
				body.Path = known.Path
			} else {
				writeError(w, http.StatusBadRequest, "unknown target, path is required")
				return
			}
		} else {
			writeError(w, http.StatusBadRequest, "name and path are required")
			return
		}
	}

	if _, exists := s.cfg.Targets[body.Name]; exists {
		writeError(w, http.StatusConflict, "target already exists: "+body.Name)
		return
	}

	s.cfg.Targets[body.Name] = config.TargetConfig{Path: body.Path}

	// In project mode, also update the project config
	if s.IsProjectMode() {
		s.projectCfg.Targets = append(s.projectCfg.Targets, config.ProjectTargetEntry{Name: body.Name})
	}

	if err := s.saveConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save config: "+err.Error())
		return
	}

	s.writeOpsLog("target", "ok", start, map[string]any{
		"action": "add",
		"name":   body.Name,
		"target": body.Path,
		"scope":  "ui",
	}, "")

	writeJSON(w, map[string]any{"success": true})
}

func (s *Server) handleRemoveTarget(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	name := r.PathValue("name")

	target, exists := s.cfg.Targets[name]
	if !exists {
		writeError(w, http.StatusNotFound, "target not found: "+name)
		return
	}

	// Clean up symlinks/manifest from the target before deleting from config
	info, err := os.Lstat(target.Path)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			// Symlink mode: entire directory is a symlink
			os.Remove(target.Path)
		} else if info.IsDir() {
			// Remove manifest if present (copy mode)
			ssync.RemoveManifest(target.Path)
			// Merge mode: remove individual skill symlinks pointing to source
			s.unlinkMergeSymlinks(target.Path)
		}
	}

	delete(s.cfg.Targets, name)

	// In project mode, also remove from project config
	if s.IsProjectMode() {
		filtered := make([]config.ProjectTargetEntry, 0, len(s.projectCfg.Targets))
		for _, t := range s.projectCfg.Targets {
			if t.Name != name {
				filtered = append(filtered, t)
			}
		}
		s.projectCfg.Targets = filtered
	}

	if err := s.saveConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save config: "+err.Error())
		return
	}

	s.writeOpsLog("target", "ok", start, map[string]any{
		"action": "remove",
		"name":   name,
		"target": target.Path,
		"scope":  "ui",
	}, "")

	writeJSON(w, map[string]any{"success": true, "name": name})
}

func (s *Server) handleUpdateTarget(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	name := r.PathValue("name")

	target, exists := s.cfg.Targets[name]
	if !exists {
		writeError(w, http.StatusNotFound, "target not found: "+name)
		return
	}

	var body struct {
		Include *[]string `json:"include"` // null = no change, [] = clear
		Exclude *[]string `json:"exclude"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// Validate patterns
	if body.Include != nil {
		if _, err := ssync.FilterSkills(nil, *body.Include, nil); err != nil {
			writeError(w, http.StatusBadRequest, "invalid include pattern: "+err.Error())
			return
		}
		target.Include = *body.Include
	}
	if body.Exclude != nil {
		if _, err := ssync.FilterSkills(nil, nil, *body.Exclude); err != nil {
			writeError(w, http.StatusBadRequest, "invalid exclude pattern: "+err.Error())
			return
		}
		target.Exclude = *body.Exclude
	}

	s.cfg.Targets[name] = target

	// In project mode, also update the project config
	if s.IsProjectMode() {
		for i := range s.projectCfg.Targets {
			if s.projectCfg.Targets[i].Name == name {
				if body.Include != nil {
					s.projectCfg.Targets[i].Include = *body.Include
				}
				if body.Exclude != nil {
					s.projectCfg.Targets[i].Exclude = *body.Exclude
				}
				break
			}
		}
	}

	if err := s.saveConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save config: "+err.Error())
		return
	}

	s.writeOpsLog("target", "ok", start, map[string]any{
		"action": "filter",
		"name":   name,
		"scope":  "ui",
	}, "")

	writeJSON(w, map[string]any{"success": true})
}

// unlinkMergeSymlinks removes symlinks in targetPath that point under the
// source directory and copies the skill contents back as real files.
func (s *Server) unlinkMergeSymlinks(targetPath string) {
	entries, err := os.ReadDir(targetPath)
	if err != nil {
		return
	}

	absSource, err := filepath.Abs(s.cfg.Source)
	if err != nil {
		return
	}
	absSourcePrefix := absSource + string(filepath.Separator)

	for _, entry := range entries {
		skillPath := filepath.Join(targetPath, entry.Name())

		if !utils.IsSymlinkOrJunction(skillPath) {
			continue
		}

		absLink, err := utils.ResolveLinkTarget(skillPath)
		if err != nil {
			continue
		}

		if !utils.PathHasPrefix(absLink, absSourcePrefix) {
			continue
		}

		// Remove symlink and copy the skill back if source still exists
		os.Remove(skillPath)
		if _, statErr := os.Stat(absLink); statErr == nil {
			_ = copySkillDir(absLink, skillPath)
		}
	}
}

// copySkillDir copies a directory tree from src to dst.
func copySkillDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(src, path)
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		dstFile, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}
