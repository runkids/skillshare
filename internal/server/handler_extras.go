package server

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/config"
	syncpkg "skillshare/internal/sync"
)

// resolveExtensionSpec resolves a target's extension value into a transform
// spec, mirroring the CLI's resolveExtension. A bare name resolves under the
// current mode's extensions dir; a path (contains a separator or starts with
// ".") is used directly. An empty value yields (nil, nil) — no transform.
func (s *Server) resolveExtensionSpec(ext string) (*syncpkg.ExtensionSpec, error) {
	if ext == "" {
		return nil, nil
	}
	execPath := filepath.Join(s.extensionsDir(), ext)
	if strings.ContainsAny(ext, "/\\") || strings.HasPrefix(ext, ".") {
		execPath = config.ExpandPath(ext)
	}
	return syncpkg.LoadExtensionSpec(execPath, ext)
}

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
	Path      string `json:"path"`
	Mode      string `json:"mode"`
	Flatten   bool   `json:"flatten"`
	Extension string `json:"extension,omitempty"`
	Status    string `json:"status"` // "synced", "drift", "not synced", "no source"
}

// extrasSourceDir returns the source directory for the named extra in the
// current mode.
func (s *Server) extrasSourceDir(extra config.ExtraConfig) string {
	if s.IsProjectMode() {
		return config.ExtrasSourceDirProject(s.projectCfg.EffectiveExtrasSource(s.projectRoot), extra.Name)
	}
	return config.ResolveExtrasSourceDir(extra, s.cfg.EffectiveExtrasSource(), s.cfg.EffectiveSkillsSource())
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
	source := s.cfg.EffectiveSkillsSource()
	extrasSource := s.cfg.EffectiveExtrasSource()
	// rawExtrasSource is the user's explicit configuration value (either the
	// legacy extras_source field or the new sources.extras field). It is empty
	// only when the user did not configure either, so source_type can
	// distinguish "default" (derived) from "extras_source" (user-configured).
	rawExtrasSource := s.cfg.ExtrasSource
	if rawExtrasSource == "" {
		rawExtrasSource = s.cfg.Sources.Extras
	}
	var projectExtrasParent string
	if s.IsProjectMode() {
		projectExtrasParent = s.projectCfg.EffectiveExtrasSource(s.projectRoot)
	}
	s.mu.RUnlock()

	isProjectMode := projectRoot != ""

	// Resolve the extras_source value for source_type resolution (global mode only).
	resolvedExtrasSource := ""
	if !isProjectMode {
		resolvedExtrasSource = rawExtrasSource
	}

	entries := make([]extrasListEntry, 0, len(extras))
	for _, extra := range extras {
		var sourceDir string
		if isProjectMode {
			sourceDir = config.ExtrasSourceDirProject(projectExtrasParent, extra.Name)
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
				Path:      t.Path,
				Mode:      m,
				Flatten:   t.Flatten,
				Extension: t.Extension,
			}

			// Transform extensions use copy semantics. Resolve the mode through
			// the shared resolver instead of EffectiveMode, whose generic "merge"
			// default for an empty mode would make a valid extension target always
			// report drift.
			if t.Extension != "" {
				resolved, modeErr := syncpkg.ResolveExtensionMode(t.Mode)
				if modeErr != nil {
					// Invalid config (extension with a non-copy mode): sync rejects
					// it with a clear error, so flag drift here rather than guessing.
					ti.Status = "drift"
					entry.Targets = append(entry.Targets, ti)
					continue
				}
				m = resolved
				ti.Mode = m
			}

			if !entry.SourceExists {
				ti.Status = "no source"
			} else if _, statErr := os.Stat(t.Path); os.IsNotExist(statErr) {
				ti.Status = "not synced"
			} else {
				// A transform extension renames output (e.g. .md → .toml), so
				// resolve its output_ext to compare against the right target
				// filenames — otherwise status is always reported as drift.
				outputExt := ""
				if t.Extension != "" {
					if spec, serr := s.resolveExtensionSpec(t.Extension); serr == nil && spec != nil {
						outputExt = spec.OutputExt
					} else if serr != nil {
						log.Printf("warning: extension %q for extra %q could not be resolved (%v); sync status may be inaccurate", t.Extension, extra.Name, serr)
					}
				}
				ti.Status = syncpkg.CheckSyncStatus(files, sourceDir, t.Path, m, t.Flatten, outputExt)
			}

			entry.Targets = append(entry.Targets, ti)
		}

		entries = append(entries, entry)
	}

	writeJSON(w, map[string]any{"extras": entries})
}

// handleExtrasExtensions — GET /api/extras/extensions
// Lists transform extensions available in the current mode's extensions
// directory (~/.config/skillshare/extensions globally, .skillshare/extensions
// in project mode). Used by the UI to populate the per-target extension picker.
func (s *Server) handleExtrasExtensions(w http.ResponseWriter, r *http.Request) {
	extDir := filepath.Join(filepath.Dir(s.configPath()), "extensions")
	names, err := syncpkg.ListExtensions(extDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if names == nil {
		names = []string{}
	}
	writeJSON(w, map[string]any{"extensions": names})
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
	source := s.cfg.EffectiveSkillsSource()
	extrasSource := s.cfg.EffectiveExtrasSource()
	var projectExtrasParent string
	if s.IsProjectMode() {
		projectExtrasParent = s.projectCfg.EffectiveExtrasSource(s.projectRoot)
	}
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
			sourceDir = config.ExtrasSourceDirProject(projectExtrasParent, extra.Name)
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
			// Transform extensions use copy semantics; resolve through the shared
			// resolver so the diff isn't computed against the merge default. On an
			// invalid (non-copy) mode, leave m as-is — sync surfaces that error.
			// Resolve output_ext too, so the diff compares against the transformed
			// target filenames (e.g. .md → .toml) instead of reporting false drift.
			outputExt := ""
			if t.Extension != "" {
				if resolved, modeErr := syncpkg.ResolveExtensionMode(t.Mode); modeErr == nil {
					m = resolved
				}
				if spec, serr := s.resolveExtensionSpec(t.Extension); serr == nil && spec != nil {
					outputExt = spec.OutputExt
				} else if serr != nil {
					log.Printf("warning: extension %q for extra %q could not be resolved (%v); diff may report false drift", t.Extension, extra.Name, serr)
				}
			}

			items := buildExtrasDiffItems(files, sourceDir, t.Path, m, t.Flatten, outputExt)
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

// buildExtrasDiffItems returns the list of files that differ between source and
// target. When outputExt is non-empty a transform extension is in effect, so
// the expected target file carries the transformed extension (e.g. foo.md →
// foo.toml) — matching sync and status, which otherwise reports false drift.
func buildExtrasDiffItems(sourceFiles []string, sourceDir, targetDir, mode string, flatten bool, outputExt string) []extrasDiffItem {
	var items []extrasDiffItem
	seen := make(map[string]bool)

	for _, rel := range sourceFiles {
		tgtRel, ok := syncpkg.FlattenRel(rel, flatten, seen)
		if !ok {
			continue
		}
		tgtRel = syncpkg.ApplyOutputExt(tgtRel, outputExt)
		sourceFile := filepath.Join(sourceDir, rel)
		targetFile := filepath.Join(targetDir, tgtRel)

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
			Path      string `json:"path"`
			Mode      string `json:"mode"`
			Flatten   bool   `json:"flatten"`
			Extension string `json:"extension,omitempty"`
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

	// Validate mode and flatten values
	for _, t := range body.Targets {
		if err := config.ValidateExtraMode(t.Mode); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if t.Flatten {
			if err := config.ValidateExtraFlatten(true, t.Mode); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
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
		et := config.ExtraTargetConfig{Path: t.Path, Flatten: t.Flatten}
		if t.Mode != "" {
			et.Mode = t.Mode
		}
		if t.Extension != "" {
			// A transform extension only makes sense with copy mode.
			et.Extension = t.Extension
			et.Mode = "copy"
		}
		extra.Targets = append(extra.Targets, et)
	}

	// Append to config (extras source resolution is handled by
	// s.cfg.EffectiveExtrasSource() inside s.extrasSourceDir; no backfill needed).
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
	source := s.cfg.EffectiveSkillsSource()
	extrasSource := s.cfg.EffectiveExtrasSource()
	var projectExtrasParent string
	if s.IsProjectMode() {
		projectExtrasParent = s.projectCfg.EffectiveExtrasSource(s.projectRoot)
	}
	s.mu.RUnlock()

	type targetSyncResult struct {
		Target   string   `json:"target"`
		Mode     string   `json:"mode"`
		Synced   int      `json:"synced"`
		Skipped  int      `json:"skipped"`
		Pruned   int      `json:"pruned"`
		Errors   []string `json:"errors,omitempty"`
		Error    string   `json:"error,omitempty"`
		Warnings []string `json:"warnings,omitempty"`
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
			sourceDir = config.ExtrasSourceDirProject(projectExtrasParent, extra.Name)
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

			// Resolve the per-target transform extension (if any) so the UI
			// sync applies it just like the CLI — otherwise files are copied
			// verbatim instead of being transformed (e.g. .md left as .md).
			spec, specErr := s.resolveExtensionSpec(t.Extension)
			if specErr != nil {
				tr.Error = "extension " + t.Extension + ": " + specErr.Error()
				result.Targets = append(result.Targets, tr)
				continue
			}

			// Transform extensions emit generated files via copy semantics.
			// Resolve through the shared resolver so the CLI, sync, status, and
			// diff paths all enforce one contract (empty/copy → copy, else error).
			if spec != nil {
				resolved, modeErr := syncpkg.ResolveExtensionMode(t.Mode)
				if modeErr != nil {
					tr.Error = "extension " + t.Extension + ": " + modeErr.Error()
					result.Targets = append(result.Targets, tr)
					continue
				}
				m = resolved
				tr.Mode = m
			}

			res, err := syncpkg.SyncExtra(sourceDir, t.Path, m, body.DryRun, body.Force, t.Flatten, projectRoot, spec)
			if err != nil {
				tr.Error = err.Error()
			} else {
				tr.Synced = res.Synced
				tr.Skipped = res.Skipped
				tr.Pruned = res.Pruned
				tr.Errors = res.Errors
				tr.Warnings = res.Warnings
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
		Target    string  `json:"target"`
		Mode      string  `json:"mode"`
		Flatten   *bool   `json:"flatten,omitempty"`
		Extension *string `json:"extension,omitempty"`
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

	// Find extra and target, update mode and flatten in-place
	found := false
	for i, extra := range extras {
		if extra.Name != name {
			continue
		}
		for j, t := range extra.Targets {
			if t.Path == body.Target {
				// Determine effective mode, flatten, and extension after this change
				newMode := body.Mode
				if newMode == "" {
					newMode = t.Mode
				}
				newFlatten := t.Flatten
				if body.Flatten != nil {
					newFlatten = *body.Flatten
				}
				newExtension := t.Extension
				if body.Extension != nil {
					newExtension = *body.Extension
				}

				// An extension transform implies copy mode. Reject only a mode
				// explicitly requested in this call that conflicts; otherwise
				// force copy, overriding any prior mode on the target.
				if newExtension != "" {
					if body.Mode != "" && body.Mode != "copy" {
						writeError(w, http.StatusBadRequest, "extension requires copy mode, but mode "+body.Mode+" was set on the target")
						return
					}
					newMode = "copy"
				}

				// Validate the combination
				if err := config.ValidateExtraFlatten(newFlatten, newMode); err != nil {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}

				extras[i].Targets[j].Mode = newMode
				if body.Flatten != nil {
					extras[i].Targets[j].Flatten = *body.Flatten
				}
				if body.Extension != nil {
					extras[i].Targets[j].Extension = *body.Extension
				}

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
