package server

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/hooks"
	"skillshare/internal/mcp"
	ssync "skillshare/internal/sync"
	"skillshare/internal/targetsummary"
	"skillshare/internal/utils"
)

type targetItem struct {
	Name               string   `json:"name"`
	Project            string   `json:"project,omitempty"` // root of the project this target belongs to
	Agent              string   `json:"agent,omitempty"`   // the built-in Agent this is another config directory of
	ConfigDir          string   `json:"configDir,omitempty"`
	CLI                string   `json:"cli,omitempty"` // the executable that runs its plugin commands, when not the Agent's own
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
	AgentExtension     string   `json:"agentExtension,omitempty"`
	AgentInclude       []string `json:"agentInclude,omitempty"`
	AgentExclude       []string `json:"agentExclude,omitempty"`
	AgentLinkedCount   *int     `json:"agentLinkedCount,omitempty"`
	AgentLocalCount    *int     `json:"agentLocalCount,omitempty"`
	AgentExpectedCount *int     `json:"agentExpectedCount,omitempty"`
	SkillsEnabled      bool     `json:"skillsEnabled"`
	// SkillsReadFrom (skills off) names the enabled targets whose skills folder
	// this tool still reads; SkillsAlsoReadBy (skills on) names the targets with
	// skills off whose tool reads this target's folder.
	SkillsReadFrom   []string `json:"skillsReadFrom,omitempty"`
	SkillsAlsoReadBy []string `json:"skillsAlsoReadBy,omitempty"`
	// SkillsSharedWith names the other targets with skills on that write to this
	// target's skills folder, so the page can warn when their settings differ.
	SkillsSharedWith []string `json:"skillsSharedWith,omitempty"`
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
	discovered, discoveredErr := ssync.DiscoverSourceSkills(source, s.skillsWalk())

	// Project targets are edited under projects, so they are listed only on request:
	// scope=projects for them alone, scope=all for what a sync writes.
	scope := r.URL.Query().Get("scope")
	for name, target := range targets {
		if scope != "all" && (target.ProjectRoot() != "") != (scope == "projects") {
			continue
		}
		sc := target.SkillsConfig()
		mode := sc.Mode
		if mode == "" {
			mode = globalMode
		}

		item := targetItem{
			Name:         name,
			Project:      target.ProjectRoot(),
			Agent:        target.Agent,
			ConfigDir:    target.ConfigDir,
			CLI:          target.CLI,
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
			SkillsEnabled: sc.IsEnabled(),
		}

		switch {
		case !sc.IsEnabled():
			item.Status = "skills off"
			item.SkillsReadFrom = config.SkillsReadFrom(targets, name, projectRoot)
		case mode == "merge" || mode == "copy":
			if discoveredErr == nil {
				filtered, err := ssync.SelectTargetSkills(discovered, name, sc)
				if err != nil {
					writeError(w, http.StatusBadRequest, "invalid include/exclude for target "+name+": "+err.Error())
					return
				}
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
		item.SkillsAlsoReadBy = config.SkillsAlsoReadBy(targets, name, projectRoot)
		item.SkillsSharedWith = config.SkillsSharedWith(targets, name)

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
			item.AgentExtension = agentSummary.Extension
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
		// Agent and ConfigDir add another config directory of a built-in Agent instead.
		Agent     string `json:"agent"`
		ConfigDir string `json:"configDir"`
		CLI       string `json:"cli"`
		// Instructions optionally names the instruction file of a custom target.
		Instructions *config.TargetInstructionsConfig `json:"instructions"`
		// SkillsEnabled false adds the target with skills off (default true).
		SkillsEnabled *bool `json:"skills_enabled"`
	}
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
		}
		return
	}

	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	if body.Agent != "" || body.ConfigDir != "" {
		if s.IsProjectMode() {
			writeError(w, http.StatusBadRequest, "a project has no Agent config directory to add")
			return
		}
		if _, exists := s.cfg.Targets[body.Name]; exists {
			writeError(w, http.StatusConflict, "target already exists: "+body.Name)
			return
		}
		path, err := s.cfg.AddAgentConfigDirTarget(body.Name, body.Agent, body.ConfigDir, body.CLI)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if body.SkillsEnabled != nil && !*body.SkillsEnabled {
			tc := s.cfg.Targets[body.Name]
			tc.EnsureSkills().SetEnabled(false)
			s.cfg.Targets[body.Name] = tc
		}
		if err := s.saveAndReloadConfig(); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.writeOpsLog("target", "ok", start, map[string]any{"action": "add", "name": body.Name, "target": path, "scope": "ui"}, "")
		writeJSON(w, map[string]any{"success": true})
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
	if ic := body.Instructions; ic != nil {
		ic.Path = strings.TrimSpace(ic.Path)
		if err := config.ValidateTargetInstructions(ic, s.IsProjectMode()); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	skillsOff := body.SkillsEnabled != nil && !*body.SkillsEnabled
	tc := config.TargetConfig{Skills: &config.ResourceTargetConfig{Path: body.Path}, Instructions: body.Instructions}
	if !s.IsProjectMode() {
		tc.Skills.Mode = config.NewTargetSkillsMode(s.cfg.TargetNaming, s.cfg.Mode)
	}
	tc.Skills.SetEnabled(!skillsOff)
	if body.AgentPath != "" {
		tc.Agents = &config.ResourceTargetConfig{Path: body.AgentPath}
	}
	s.cfg.Targets[body.Name] = tc

	// In project mode, also update the project config
	if s.IsProjectMode() {
		entry := config.ProjectTargetEntry{Name: body.Name, Instructions: body.Instructions}
		if skillsOff {
			entry.EnsureSkills().SetEnabled(false)
		}
		if mode := config.NewTargetSkillsMode(s.projectCfg.TargetNaming, ""); mode != "" {
			entry.EnsureSkills().Mode = mode // a project target defaults to merge
		}
		s.projectCfg.Targets = append(s.projectCfg.Targets, entry)
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

	if target.ProjectRoot() != "" {
		writeError(w, http.StatusBadRequest, name+" belongs to a project; remove the project instead")
		return
	}
	// Another target writing the same folder (codex and universal) still owns its
	// links; a target with skills off no longer manages its folder.
	if sc.IsEnabled() && config.SkillsPathKeptBy(s.cfg.Targets, name, nil) == "" {
		if status, err := s.detachSkillsTarget(sc.Path); err != nil {
			writeError(w, status, err.Error())
			return
		}
	}

	// Read the MCP config before it is saved without the target: afterwards its own
	// name no longer resolves, so the config no longer loads.
	var warnings []string
	if target.Agent != "" {
		if warning := mcp.ReferenceWarning(s.configPath(), name); warning != "" {
			warnings = append(warnings, warning)
		}
		if warning := hooks.ReferenceWarning(s.configPath(), name); warning != "" {
			warnings = append(warnings, warning)
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

	writeJSON(w, map[string]any{"success": true, "name": name, "warnings": warnings})
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
	if target.ProjectRoot() != "" {
		writeError(w, http.StatusBadRequest, name+" belongs to a project; edit the project instead")
		return
	}
	// Skills and Agents are pointers shared with s.cfg; edit copies so a
	// request rejected halfway leaves the config untouched.
	if target.Skills != nil {
		sk := *target.Skills
		target.Skills = &sk
	}
	if target.Agents != nil {
		ag := *target.Agents
		target.Agents = &ag
	}

	var body struct {
		Include        *[]string `json:"include"` // null = no change, [] = clear
		Exclude        *[]string `json:"exclude"`
		Mode           *string   `json:"mode"`
		TargetNaming   *string   `json:"target_naming"`
		AgentMode      *string   `json:"agent_mode"`
		AgentInclude   *[]string `json:"agent_include"`
		AgentExclude   *[]string `json:"agent_exclude"`
		AgentExtension *string   `json:"agent_extension"` // "" = clear
		SkillsEnabled  *bool     `json:"skills_enabled"`
	}
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
		}
		return
	}

	// An extension converts each agent, so it implies copy mode. Reject a
	// conflicting mode sent in this same call, as extras targets do.
	if body.AgentExtension != nil && *body.AgentExtension != "" && body.AgentMode != nil && *body.AgentMode != "copy" {
		writeError(w, http.StatusBadRequest, "agent extension requires copy mode, but agent_mode "+*body.AgentMode+" was set")
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
			writeError(w, http.StatusBadRequest, "invalid target_naming: "+*body.TargetNaming+"; must be flat, standard, or prefixed")
			return
		}
		target.Skills.TargetNaming = *body.TargetNaming
	}
	// Check the pair the target ends up with: an off target may keep a pair its
	// mode cannot sync, and turning skills on checks it.
	skillsOn := target.SkillsConfig().IsEnabled()
	if body.SkillsEnabled != nil {
		skillsOn = *body.SkillsEnabled
	}
	if skillsOn && (body.Mode != nil || body.TargetNaming != nil || body.SkillsEnabled != nil) {
		if err := config.TargetNamingModeError(target.SkillsConfig().TargetNaming, cmp.Or(target.Skills.Mode, s.cfg.Mode)); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
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

	if body.AgentExtension != nil {
		if *body.AgentExtension != "" {
			target.EnsureAgents().Mode = "copy"
		}
		target.EnsureAgents().Extension = *body.AgentExtension
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

	if body.SkillsEnabled != nil {
		target.Skills.SetEnabled(*body.SkillsEnabled)
	}

	s.cfg.Targets[name] = target

	// In project mode, also update the project config
	if s.IsProjectMode() {
		for i := range s.projectCfg.Targets {
			if s.projectCfg.Targets[i].Name == name {
				sk := s.projectCfg.Targets[i].EnsureSkills()
				if body.SkillsEnabled != nil {
					sk.SetEnabled(*body.SkillsEnabled)
				}
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
				if body.AgentExtension != nil {
					ag := s.projectCfg.Targets[i].EnsureAgents()
					if *body.AgentExtension != "" {
						ag.Mode = "copy"
					}
					ag.Extension = *body.AgentExtension
				}
				break
			}
		}
	}

	if err := s.saveAndReloadConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Skills off applies at once: the folder keeps only what skillshare did not link.
	var detach *ssync.SkillsOffResult
	if body.SkillsEnabled != nil && !*body.SkillsEnabled {
		res, err := ssync.DetachSkills(s.cfg.Targets, name, s.cfg.EffectiveSkillsSource(), false)
		if err != nil {
			s.writeOpsLog("target", "error", start, map[string]any{"action": "skills", "name": name, "enabled": false, "scope": "ui"}, err.Error())
			writeError(w, http.StatusInternalServerError, "skills off saved, but removing links failed: "+err.Error())
			return
		}
		detach = res
	}

	hasFilter := body.Include != nil || body.Exclude != nil || body.AgentInclude != nil || body.AgentExclude != nil
	hasSetting := body.Mode != nil || body.TargetNaming != nil || body.AgentMode != nil || body.AgentExtension != nil || body.SkillsEnabled != nil
	action := "filter"
	if hasSetting && hasFilter {
		action = "settings+filter"
	} else if hasSetting {
		action = "settings"
	}
	logArgs := map[string]any{
		"action": action,
		"name":   name,
		"scope":  "ui",
	}
	if body.SkillsEnabled != nil {
		logArgs["skills_enabled"] = *body.SkillsEnabled
	}
	if detach != nil {
		logArgs["removed"] = len(detach.Removed)
		logArgs["kept"] = len(detach.Kept)
		logArgs["copies"] = len(detach.Copies)
	}
	s.writeOpsLog("target", "ok", start, logArgs, "")

	resp := map[string]any{"success": true}
	if detach != nil {
		resp["detach"] = map[string]any{"removed": detach.Removed, "kept": detach.Kept, "copies": detach.Copies}
	}
	writeJSON(w, resp)
}

// handleSkillsOffPreview reports what turning skills off for a target would
// remove and keep, without writing anything.
func (s *Server) handleSkillsOffPreview(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	targets := s.cloneTargets()
	source := s.cfg.EffectiveSkillsSource()
	s.mu.RUnlock()

	name := r.PathValue("name")
	target, ok := targets[name]
	if !ok {
		writeError(w, http.StatusNotFound, "target not found: "+name)
		return
	}
	if target.ProjectRoot() != "" {
		writeError(w, http.StatusBadRequest, name+" belongs to a project; edit the project instead")
		return
	}
	res, err := ssync.DetachSkills(targets, name, source, true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := map[string]any{"remove": res.Removed, "keep": res.Kept, "copies": res.Copies}
	if res.SharedWith != "" {
		resp["sharedWith"] = res.SharedWith
	}
	writeJSON(w, resp)
}

// detachSkillsTarget removes what sync owns in a skills folder before its target leaves
// the config: the folder symlink, or the manifest and the symlinks into the source.
// Copies and local skills stay.
func (s *Server) detachSkillsTarget(path string) (int, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to inspect target: %w", err)
	}
	if utils.IsLinkMode(path, info.Mode()) {
		// Symlink mode: entire directory is a symlink
		if err := removeTargetPath(path); err != nil {
			return http.StatusInternalServerError, fmt.Errorf("failed to remove target symlink: %w", err)
		}
	} else if info.IsDir() {
		// Remove manifest if present (merge/copy mode)
		if err := ssync.RemoveManifest(path); err != nil {
			return http.StatusInternalServerError, fmt.Errorf("failed to remove target manifest: %w", err)
		}
		// Merge mode: remove individual skill symlinks pointing to source
		if err := s.unlinkMergeSymlinks(path); err != nil {
			return http.StatusInternalServerError, fmt.Errorf("failed to clean target symlinks: %w", err)
		}
	}
	return 0, nil
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
