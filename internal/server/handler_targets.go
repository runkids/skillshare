package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"skillshare/internal/config"
	ssync "skillshare/internal/sync"
	"skillshare/internal/targetsummary"
	"skillshare/internal/utils"
)

type targetItem struct {
	Name               string   `json:"name"`
	Path               string   `json:"path"`
	Mode               string   `json:"mode"`
	TargetNaming       string   `json:"targetNaming"`
	Status             string   `json:"status"`
	LinkedCount        int      `json:"linkedCount"`
	LocalCount         int      `json:"localCount"`
	Include            []string `json:"include"`
	Exclude            []string `json:"exclude"`
	ExpectedSkillCount int      `json:"expectedSkillCount"`
	SkippedSkillCount  int      `json:"skippedSkillCount,omitempty"`
	CollisionCount     int      `json:"collisionCount,omitempty"`
	AgentPath          string   `json:"agentPath,omitempty"`
	AgentMode          string   `json:"agentMode,omitempty"`
	AgentInclude       []string `json:"agentInclude,omitempty"`
	AgentExclude       []string `json:"agentExclude,omitempty"`
	AgentLinkedCount   *int     `json:"agentLinkedCount,omitempty"`
	AgentLocalCount    *int     `json:"agentLocalCount,omitempty"`
	AgentExpectedCount *int     `json:"agentExpectedCount,omitempty"`
}

var removeTargetPath = os.Remove

func (s *Server) handleListTargets(w http.ResponseWriter, r *http.Request) {
	// Snapshot config under RLock, then release before I/O.
	s.mu.RLock()
	source := s.cfg.EffectiveSkillsSource()
	cfgMode := s.cfg.Mode
	targets := s.cloneTargets()
	isProjectMode := s.IsProjectMode()
	projectRoot := s.projectRoot
	agentsSourcePath := s.agentsSource()
	cfgSnapshot := *s.cfg
	var projectEntries []config.ProjectTargetEntry
	if isProjectMode && s.projectCfg != nil {
		projectEntries = append([]config.ProjectTargetEntry(nil), s.projectCfg.Targets...)
	}
	s.mu.RUnlock()

	globalMode := cfgMode
	if globalMode == "" {
		globalMode = "merge"
	}

	projectEntryByName := make(map[string]config.ProjectTargetEntry, len(projectEntries))
	for _, entry := range projectEntries {
		projectEntryByName[entry.Name] = entry
	}

	var (
		agentBuilder *targetsummary.Builder
		err          error
	)
	if isProjectMode {
		agentBuilder, err = targetsummary.NewProjectBuilder(agentsSourcePath, projectRoot)
	} else {
		agentBuilder, err = targetsummary.NewGlobalBuilder(&cfgSnapshot)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to discover agents: "+err.Error())
		return
	}

	items := make([]targetItem, 0, len(targets))
	discovered, discoveredErr := ssync.DiscoverSourceSkills(source)

	for name, target := range targets {
		sc := target.SkillsConfig()
		mode := sc.Mode
		if mode == "" {
			mode = globalMode
		}

		item := targetItem{
			Name:         name,
			Path:         sc.Path,
			Mode:         mode,
			TargetNaming: config.EffectiveTargetNaming(sc.TargetNaming),
			Include: func() []string {
				if len(sc.Include) == 0 {
					return []string{}
				}
				return append([]string(nil), sc.Include...)
			}(),
			Exclude: func() []string {
				if len(sc.Exclude) == 0 {
					return []string{}
				}
				return append([]string(nil), sc.Exclude...)
			}(),
		}

		switch mode {
		case "merge", "copy":
			if discoveredErr == nil {
				filtered, err := ssync.FilterSkills(discovered, sc.Include, sc.Exclude)
				if err != nil {
					writeError(w, http.StatusBadRequest, "invalid include/exclude for target "+name+": "+err.Error())
					return
				}
				filtered = ssync.FilterSkillsByTarget(filtered, name)
				resolution, resErr := ssync.ResolveTargetSkillsForTarget(name, config.ResourceTargetConfig{
					Path:         sc.Path,
					TargetNaming: sc.TargetNaming,
				}, filtered)
				if resErr == nil {
					item.ExpectedSkillCount = len(resolution.Skills)
					item.SkippedSkillCount = len(filtered) - len(resolution.Skills)
					item.CollisionCount = len(resolution.Collisions)
				} else {
					item.ExpectedSkillCount = len(filtered)
				}
			}
			if mode == "merge" {
				status, linked, local := ssync.CheckStatusMerge(sc.Path, source)
				item.Status = status.String()
				item.LinkedCount = linked
				item.LocalCount = local
			} else {
				status, managed, local := ssync.CheckStatusCopy(sc.Path)
				item.Status = status.String()
				item.LinkedCount = managed
				item.LocalCount = local
			}
		default:
			status := ssync.CheckStatus(sc.Path, source)
			item.Status = status.String()
		}

		var agentSummary *targetsummary.AgentSummary
		if isProjectMode {
			if entry, ok := projectEntryByName[name]; ok {
				agentSummary, err = agentBuilder.ProjectTarget(entry)
			}
		} else {
			agentSummary, err = agentBuilder.GlobalTarget(name, target)
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid agent include/exclude for target "+name+": "+err.Error())
			return
		}
		if agentSummary != nil {
			item.AgentPath = agentSummary.Path
			item.AgentMode = agentSummary.Mode
			item.AgentInclude = agentSummary.Include
			item.AgentExclude = agentSummary.Exclude
			item.AgentLinkedCount = intPtr(agentSummary.ManagedCount)
			item.AgentLocalCount = intPtr(agentSummary.LocalCount)
			item.AgentExpectedCount = intPtr(agentSummary.ExpectedCount)
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

func intPtr(v int) *int {
	return &v
}

func (s *Server) handleAddTarget(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	var body struct {
		Name      string `json:"name"`
		Path      string `json:"path"`
		AgentPath string `json:"agentPath"`
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

	tc := config.TargetConfig{Skills: &config.ResourceTargetConfig{Path: body.Path}}
	if body.AgentPath != "" {
		tc.Agents = &config.ResourceTargetConfig{Path: body.AgentPath}
	}
	s.cfg.Targets[body.Name] = tc

	// In project mode, also update the project config
	if s.IsProjectMode() {
		s.projectCfg.Targets = append(s.projectCfg.Targets, config.ProjectTargetEntry{Name: body.Name})
	}

	if err := s.saveAndReloadConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
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

	sc := target.SkillsConfig()

	// Clean up symlinks/manifest from the target before deleting from config
	info, err := os.Lstat(sc.Path)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			// Symlink mode: entire directory is a symlink
			if err := removeTargetPath(sc.Path); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to remove target symlink: "+err.Error())
				return
			}
		} else if info.IsDir() {
			// Remove manifest if present (merge/copy mode)
			if err := ssync.RemoveManifest(sc.Path); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to remove target manifest: "+err.Error())
				return
			}
			// Merge mode: remove individual skill symlinks pointing to source
			if err := s.unlinkMergeSymlinks(sc.Path); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to clean target symlinks: "+err.Error())
				return
			}
		}
	} else if !os.IsNotExist(err) {
		writeError(w, http.StatusInternalServerError, "failed to inspect target: "+err.Error())
		return
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

	if err := s.saveAndReloadConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeOpsLog("target", "ok", start, map[string]any{
		"action": "remove",
		"name":   name,
		"target": sc.Path,
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
		Include      *[]string `json:"include"` // null = no change, [] = clear
		Exclude      *[]string `json:"exclude"`
		Mode         *string   `json:"mode"`
		TargetNaming *string   `json:"target_naming"`
		AgentMode    *string   `json:"agent_mode"`
		AgentInclude *[]string `json:"agent_include"`
		AgentExclude *[]string `json:"agent_exclude"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	target.EnsureSkills()

	// Validate patterns
	if body.Include != nil {
		if _, err := ssync.FilterSkills(nil, *body.Include, nil); err != nil {
			writeError(w, http.StatusBadRequest, "invalid include pattern: "+err.Error())
			return
		}
		target.Skills.Include = *body.Include
	}
	if body.Exclude != nil {
		if _, err := ssync.FilterSkills(nil, nil, *body.Exclude); err != nil {
			writeError(w, http.StatusBadRequest, "invalid exclude pattern: "+err.Error())
			return
		}
		target.Skills.Exclude = *body.Exclude
	}

	if body.Mode != nil {
		switch *body.Mode {
		case "merge", "symlink", "copy":
			target.Skills.Mode = *body.Mode
		default:
			writeError(w, http.StatusBadRequest, "invalid mode: "+*body.Mode+"; must be merge, symlink, or copy")
			return
		}
	}

	if body.TargetNaming != nil {
		if !config.IsValidTargetNaming(*body.TargetNaming) {
			writeError(w, http.StatusBadRequest, "invalid target_naming: "+*body.TargetNaming+"; must be flat or standard")
			return
		}
		target.Skills.TargetNaming = *body.TargetNaming
	}

	if body.AgentMode != nil {
		switch *body.AgentMode {
		case "merge", "symlink", "copy":
			target.EnsureAgents().Mode = *body.AgentMode
		default:
			writeError(w, http.StatusBadRequest, "invalid agent_mode: "+*body.AgentMode+"; must be merge, symlink, or copy")
			return
		}
	}

	if body.AgentInclude != nil {
		if _, err := ssync.FilterSkills(nil, *body.AgentInclude, nil); err != nil {
			writeError(w, http.StatusBadRequest, "invalid agent include pattern: "+err.Error())
			return
		}
		target.EnsureAgents().Include = *body.AgentInclude
	}
	if body.AgentExclude != nil {
		if _, err := ssync.FilterSkills(nil, nil, *body.AgentExclude); err != nil {
			writeError(w, http.StatusBadRequest, "invalid agent exclude pattern: "+err.Error())
			return
		}
		target.EnsureAgents().Exclude = *body.AgentExclude
	}

	s.cfg.Targets[name] = target

	// In project mode, also update the project config
	if s.IsProjectMode() {
		for i := range s.projectCfg.Targets {
			if s.projectCfg.Targets[i].Name == name {
				sk := s.projectCfg.Targets[i].EnsureSkills()
				if body.Include != nil {
					sk.Include = *body.Include
				}
				if body.Exclude != nil {
					sk.Exclude = *body.Exclude
				}
				if body.Mode != nil {
					sk.Mode = *body.Mode
				}
				if body.TargetNaming != nil {
					sk.TargetNaming = *body.TargetNaming
				}
				if body.AgentMode != nil {
					s.projectCfg.Targets[i].EnsureAgents().Mode = *body.AgentMode
				}
				if body.AgentInclude != nil {
					s.projectCfg.Targets[i].EnsureAgents().Include = *body.AgentInclude
				}
				if body.AgentExclude != nil {
					s.projectCfg.Targets[i].EnsureAgents().Exclude = *body.AgentExclude
				}
				break
			}
		}
	}

	if err := s.saveAndReloadConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	hasFilter := body.Include != nil || body.Exclude != nil || body.AgentInclude != nil || body.AgentExclude != nil
	hasSetting := body.Mode != nil || body.TargetNaming != nil || body.AgentMode != nil
	action := "filter"
	if hasSetting && hasFilter {
		action = "settings+filter"
	} else if hasSetting {
		action = "settings"
	}
	s.writeOpsLog("target", "ok", start, map[string]any{
		"action": action,
		"name":   name,
		"scope":  "ui",
	}, "")

	writeJSON(w, map[string]any{"success": true})
}

// unlinkMergeSymlinks removes symlinks in targetPath that point under the
// source directory and copies the skill contents back as real files.
func (s *Server) unlinkMergeSymlinks(targetPath string) error {
	entries, err := os.ReadDir(targetPath)
	if err != nil {
		return err
	}

	absSource, err := filepath.Abs(s.cfg.EffectiveSkillsSource())
	if err != nil {
		return err
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
		if err := removeTargetPath(skillPath); err != nil {
			return fmt.Errorf("remove %s: %w", skillPath, err)
		}
		if _, statErr := os.Stat(absLink); statErr == nil {
			if err := copySkillDir(absLink, skillPath); err != nil {
				return fmt.Errorf("restore %s from %s: %w", skillPath, absLink, err)
			}
		}
	}
	return nil
}

// copySkillDir copies a directory tree from src to dst.
func copySkillDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("failed to compute relative path from %s to %s: %w", src, path, err)
		}
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
