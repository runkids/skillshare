package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"skillshare/internal/config"
	syncpkg "skillshare/internal/sync"
)

// extrasListEntry is the JSON response shape for a single extra.
type extrasListEntry struct {
	Name         string             `json:"name"`
	SourceDir    string             `json:"source_dir"`
	SourceType   string             `json:"source_type"`
	FileCount    int                `json:"file_count"`
	SourceExists bool               `json:"source_exists"`
	Targets      []extrasTargetInfo `json:"targets"`
}

// extrasTargetInfo is the per-target sync status inside an extra entry.
type extrasTargetInfo struct {
	Path   string `json:"path"`
	Mode   string `json:"mode"`
	Status string `json:"status"` // "synced", "drift", "not synced", "no source"
}

// extrasSourceDir returns the source directory for the named extra in the
// current mode.
func (s *Server) extrasSourceDir(extra config.ExtraConfig) string {
	if s.IsProjectMode() {
		return config.ExtrasSourceDirProject(s.projectRoot, extra.Name)
	}
	return config.ResolveExtrasSourceDir(extra, s.cfg.ExtrasSource, s.cfg.Source)
}

// extrasConfig returns the extras slice for the current mode.
func (s *Server) extrasConfig() []config.ExtraConfig {
	if s.IsProjectMode() {
		return s.projectCfg.Extras
	}
	return s.cfg.Extras
}

// handleExtras — GET /api/extras
func (s *Server) handleExtras(w http.ResponseWriter, r *http.Request) {
	// Snapshot config under RLock, then release before I/O.
	s.mu.RLock()
	extras := s.extrasConfig()
	projectRoot := s.projectRoot
	source := s.cfg.Source
	extrasSource := s.cfg.ExtrasSource
	s.mu.RUnlock()

	isProjectMode := projectRoot != ""

	// Resolve the extras_source value for source_type resolution (global mode only).
	resolvedExtrasSource := ""
	if !isProjectMode {
		resolvedExtrasSource = extrasSource
	}

	entries := make([]extrasListEntry, 0, len(extras))
	for _, extra := range extras {
		var sourceDir string
		if isProjectMode {
			sourceDir = config.ExtrasSourceDirProject(projectRoot, extra.Name)
		} else {
			sourceDir = config.ResolveExtrasSourceDir(extra, extrasSource, source)
		}
		entry := extrasListEntry{
			Name:       extra.Name,
			SourceDir:  sourceDir,
			SourceType: config.ResolveExtrasSourceType(extra, resolvedExtrasSource),
		}

		files, err := syncpkg.DiscoverExtraFiles(sourceDir)
		if err != nil {
			entry.SourceExists = false
			entry.FileCount = 0
		} else {
			entry.SourceExists = true
			entry.FileCount = len(files)
		}

		entry.Targets = make([]extrasTargetInfo, 0, len(extra.Targets))
		for _, t := range extra.Targets {
			m := syncpkg.EffectiveMode(t.Mode)
			ti := extrasTargetInfo{
				Path: t.Path,
				Mode: m,
			}

			if !entry.SourceExists {
				ti.Status = "no source"
			} else if _, statErr := os.Stat(t.Path); os.IsNotExist(statErr) {
				ti.Status = "not synced"
			} else {
				ti.Status = syncpkg.CheckSyncStatus(files, sourceDir, t.Path, m, t.Flatten)
			}

			entry.Targets = append(entry.Targets, ti)
		}

		entries = append(entries, entry)
	}

	writeJSON(w, map[string]any{"extras": entries})
}

// extrasDiffItem represents one file that needs action during sync.
type extrasDiffItem struct {
	Action string `json:"action"` // "create" or "update"
	File   string `json:"file"`
	Reason string `json:"reason"`
}

// extrasDiffEntry is the per-extra/target diff response shape.
type extrasDiffEntry struct {
	Name   string           `json:"name"`
	Target string           `json:"target"`
	Mode   string           `json:"mode"`
	Synced bool             `json:"synced"`
	Items  []extrasDiffItem `json:"items"`
}

// handleExtrasDiff — GET /api/extras/diff
func (s *Server) handleExtrasDiff(w http.ResponseWriter, r *http.Request) {
	// Snapshot config under RLock, then release before I/O.
	s.mu.RLock()
	extras := s.extrasConfig()
	projectRoot := s.projectRoot
	source := s.cfg.Source
	extrasSource := s.cfg.ExtrasSource
	s.mu.RUnlock()

	isProjectMode := projectRoot != ""

	filterName := r.URL.Query().Get("name")

	out := make([]extrasDiffEntry, 0)
	for _, extra := range extras {
		if filterName != "" && extra.Name != filterName {
			continue
		}

		var sourceDir string
		if isProjectMode {
			sourceDir = config.ExtrasSourceDirProject(projectRoot, extra.Name)
		} else {
			sourceDir = config.ResolveExtrasSourceDir(extra, extrasSource, source)
		}
		files, err := syncpkg.DiscoverExtraFiles(sourceDir)
		if err != nil {
			// Source doesn't exist — report every target as needing creation
			for _, t := range extra.Targets {
				m := t.Mode
				if m == "" {
					m = "merge"
				}
				out = append(out, extrasDiffEntry{
					Name:   extra.Name,
					Target: t.Path,
					Mode:   m,
					Synced: false,
					Items:  []extrasDiffItem{{Action: "create", File: "*", Reason: "no source directory"}},
				})
			}
			continue
		}

		for _, t := range extra.Targets {
			m := syncpkg.EffectiveMode(t.Mode)

			items := buildExtrasDiffItems(files, sourceDir, t.Path, m)
			synced := len(items) == 0

			out = append(out, extrasDiffEntry{
				Name:   extra.Name,
				Target: t.Path,
				Mode:   m,
				Synced: synced,
				Items:  items,
			})
		}
	}

	writeJSON(w, map[string]any{"extras": out})
}

// buildExtrasDiffItems returns the list of files that differ between source and target.
func buildExtrasDiffItems(sourceFiles []string, sourceDir, targetDir, mode string) []extrasDiffItem {
	var items []extrasDiffItem

	for _, rel := range sourceFiles {
		sourceFile := filepath.Join(sourceDir, rel)
		targetFile := filepath.Join(targetDir, rel)

		info, err := os.Lstat(targetFile)
		if err != nil {
			// Target file missing
			items = append(items, extrasDiffItem{
				Action: "create",
				File:   rel,
				Reason: "missing in target",
			})
			continue
		}

		switch mode {
		case "symlink", "merge":
			if info.Mode()&os.ModeSymlink != 0 {
				link, readErr := os.Readlink(targetFile)
				if readErr != nil || link != sourceFile {
					items = append(items, extrasDiffItem{
						Action: "update",
						File:   rel,
						Reason: "symlink target mismatch",
					})
				}
			} else {
				items = append(items, extrasDiffItem{
					Action: "update",
					File:   rel,
					Reason: "not a symlink",
				})
			}
		case "copy":
			if !info.Mode().IsRegular() {
				items = append(items, extrasDiffItem{
					Action: "update",
					File:   rel,
					Reason: "not a regular file",
				})
			}
		}
	}

	return items
}

// handleExtrasCreate — POST /api/extras
func (s *Server) handleExtrasCreate(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var body struct {
		Name    string `json:"name"`
		Source  string `json:"source,omitempty"`
		Targets []struct {
			Path string `json:"path"`
			Mode string `json:"mode"`
		} `json:"targets"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if err := config.ValidateExtraName(body.Name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if len(body.Targets) == 0 {
		writeError(w, http.StatusBadRequest, "at least one target is required")
		return
	}

	// Validate mode values
	for _, t := range body.Targets {
		if err := config.ValidateExtraMode(t.Mode); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate
	if err := config.ValidateExtraNameUnique(body.Name, s.extrasConfig()); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	// Build ExtraConfig
	extra := config.ExtraConfig{Name: body.Name, Source: body.Source}
	for _, t := range body.Targets {
		et := config.ExtraTargetConfig{Path: t.Path}
		if t.Mode != "" {
			et.Mode = t.Mode
		}
		extra.Targets = append(extra.Targets, et)
	}

	// Backfill extras_source if not set (global mode only)
	if !s.IsProjectMode() && s.cfg.ExtrasSource == "" {
		s.cfg.ExtrasSource = config.ExtrasParentDir(s.cfg.Source)
	}

	// Append to config
	if s.IsProjectMode() {
		s.projectCfg.Extras = append(s.projectCfg.Extras, extra)
	} else {
		s.cfg.Extras = append(s.cfg.Extras, extra)
	}

	if err := s.saveConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save config: "+err.Error())
		return
	}

	// Create source directory
	sourceDir := s.extrasSourceDir(extra)
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create source directory: "+err.Error())
		return
	}

	if err := s.reloadConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reload config: "+err.Error())
		return
	}

	s.writeOpsLog("extras-init", "ok", start, map[string]any{
		"name":    body.Name,
		"targets": len(body.Targets),
		"scope":   "ui",
	}, "")

	writeJSON(w, map[string]any{"success": true, "name": body.Name})
}

// handleExtrasSync — POST /api/extras/sync
func (s *Server) handleExtrasSync(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var body struct {
		Name   string `json:"name"`
		DryRun bool   `json:"dry_run"`
		Force  bool   `json:"force"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && r.ContentLength > 0 {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	s.mu.RLock()
	extras := s.extrasConfig()
	projectRoot := s.projectRoot
	source := s.cfg.Source
	extrasSource := s.cfg.ExtrasSource
	s.mu.RUnlock()

	type targetSyncResult struct {
		Target  string   `json:"target"`
		Mode    string   `json:"mode"`
		Synced  int      `json:"synced"`
		Skipped int      `json:"skipped"`
		Pruned  int      `json:"pruned"`
		Errors  []string `json:"errors,omitempty"`
		Error   string   `json:"error,omitempty"`
	}
	type extraSyncResult struct {
		Name    string             `json:"name"`
		Targets []targetSyncResult `json:"targets"`
	}

	results := make([]extraSyncResult, 0)

	for _, extra := range extras {
		if body.Name != "" && extra.Name != body.Name {
			continue
		}

		var sourceDir string
		if projectRoot != "" {
			sourceDir = config.ExtrasSourceDirProject(projectRoot, extra.Name)
		} else {
			sourceDir = config.ResolveExtrasSourceDir(extra, extrasSource, source)
		}

		// Auto-create source directory if it doesn't exist
		if _, statErr := os.Stat(sourceDir); os.IsNotExist(statErr) {
			os.MkdirAll(sourceDir, 0755)
		}

		result := extraSyncResult{
			Name:    extra.Name,
			Targets: make([]targetSyncResult, 0, len(extra.Targets)),
		}

		for _, t := range extra.Targets {
			m := syncpkg.EffectiveMode(t.Mode)

			tr := targetSyncResult{
				Target: t.Path,
				Mode:   m,
				Errors: []string{},
			}

			res, err := syncpkg.SyncExtra(sourceDir, t.Path, m, body.DryRun, body.Force, t.Flatten)
			if err != nil {
				tr.Error = err.Error()
			} else {
				tr.Synced = res.Synced
				tr.Skipped = res.Skipped
				tr.Pruned = res.Pruned
				tr.Errors = res.Errors
				if tr.Errors == nil {
					tr.Errors = []string{}
				}
			}

			result.Targets = append(result.Targets, tr)
		}

		results = append(results, result)
	}

	if body.Name != "" && len(results) == 0 {
		writeError(w, http.StatusNotFound, "extra not found: "+body.Name)
		return
	}

	s.writeOpsLog("extras-sync", "ok", start, map[string]any{
		"name":   body.Name,
		"dryRun": body.DryRun,
		"force":  body.Force,
		"count":  len(results),
		"scope":  "ui",
	}, "")

	writeJSON(w, map[string]any{"extras": results})
}

// handleExtrasMode — PATCH /api/extras/{name}/mode
func (s *Server) handleExtrasMode(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	name := r.PathValue("name")

	var body struct {
		Target string `json:"target"`
		Mode   string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if body.Target == "" {
		writeError(w, http.StatusBadRequest, "target is required")
		return
	}
	if err := config.ValidateExtraMode(body.Mode); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	extras := s.extrasConfig()

	// Find extra and target, update mode in-place
	found := false
	for i, extra := range extras {
		if extra.Name != name {
			continue
		}
		for j, t := range extra.Targets {
			if t.Path == body.Target {
				extras[i].Targets[j].Mode = body.Mode
				found = true
				break
			}
		}
		if !found {
			writeError(w, http.StatusNotFound, "target not found: "+body.Target)
			return
		}
		break
	}
	if !found {
		writeError(w, http.StatusNotFound, "extra not found: "+name)
		return
	}

	if err := s.saveAndReloadConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeOpsLog("extras-mode", "ok", start, map[string]any{
		"name":   name,
		"target": body.Target,
		"mode":   body.Mode,
		"scope":  "ui",
	}, "")

	writeJSON(w, map[string]any{"success": true, "name": name, "target": body.Target, "mode": body.Mode})
}

// handleExtrasDelete — DELETE /api/extras/{name}
func (s *Server) handleExtrasDelete(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	name := r.PathValue("name")

	s.mu.Lock()
	defer s.mu.Unlock()

	// Find the extra
	extras := s.extrasConfig()
	idx := -1
	for i, e := range extras {
		if e.Name == name {
			idx = i
			break
		}
	}
	if idx == -1 {
		writeError(w, http.StatusNotFound, "extra not found: "+name)
		return
	}

	// Remove from config
	if s.IsProjectMode() {
		s.projectCfg.Extras = append(s.projectCfg.Extras[:idx], s.projectCfg.Extras[idx+1:]...)
	} else {
		s.cfg.Extras = append(s.cfg.Extras[:idx], s.cfg.Extras[idx+1:]...)
	}

	if err := s.saveAndReloadConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeOpsLog("extras-remove", "ok", start, map[string]any{
		"name":  name,
		"scope": "ui",
	}, "")

	writeJSON(w, map[string]any{"success": true, "name": name})
}
