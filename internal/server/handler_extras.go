package server

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"skillshare/internal/config"
	syncpkg "skillshare/internal/sync"
	"skillshare/internal/utils"
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
	File         string             `json:"file,omitempty"` // single-file extra: the synced file in source_dir
	SourceDir    string             `json:"source_dir"`
	SourceType   string             `json:"source_type"`
	FileCount    int                `json:"file_count"`
	SourceExists bool               `json:"source_exists"`
	Targets      []extrasTargetInfo `json:"targets"`
}

// extrasTargetInfo is the per-target sync status inside an extra entry.
type extrasTargetInfo struct {
	Path      string   `json:"path"`
	Mode      string   `json:"mode"`
	Flatten   bool     `json:"flatten"`
	Extension string   `json:"extension,omitempty"`
	As        string   `json:"as,omitempty"` // single-file extra: target filename
	Include   []string `json:"include,omitempty"`
	Exclude   []string `json:"exclude,omitempty"`
	FileCount int      `json:"file_count"` // source files this target syncs after include/exclude
	Status    string   `json:"status"`     // "synced", "drift", "modified", "not synced", "no source"
}

// extrasSourceDir returns the source directory for the named extra in the
// current mode.
func (s *Server) extrasSourceDir(extra config.ExtraConfig) string {
	if s.IsProjectMode() {
		return config.ResolveExtrasSourceDirProject(extra, s.projectCfg.EffectiveExtrasSource(s.projectRoot), s.projectRoot)
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

func resolveExtrasTargetPath(projectRoot, path string) string {
	resolved := config.ExpandPath(path)
	if projectRoot != "" && !filepath.IsAbs(resolved) {
		return filepath.Join(projectRoot, filepath.FromSlash(resolved))
	}
	return resolved
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
			sourceDir = config.ResolveExtrasSourceDirProject(extra, projectExtrasParent, projectRoot)
		} else {
			sourceDir = config.ResolveExtrasSourceDir(extra, extrasSource, source)
		}
		entry := extrasListEntry{
			Name:       extra.Name,
			File:       extra.File,
			SourceDir:  sourceDir,
			SourceType: config.ResolveExtrasSourceType(extra, resolvedExtrasSource),
		}

		files, err := syncpkg.DiscoverExtraSource(sourceDir, extra.File)
		if err != nil {
			entry.SourceExists = false
			entry.FileCount = 0
		} else {
			entry.SourceExists = true
			entry.FileCount = len(files)
		}

		entry.Targets = make([]extrasTargetInfo, 0, len(extra.Targets))
		for _, t := range extra.Targets {
			m := syncpkg.ExtraTargetMode(t.Mode, extra.File != "")
			targetPath := resolveExtrasTargetPath(projectRoot, t.Path)
			ti := extrasTargetInfo{
				Path:      t.Path,
				Mode:      m,
				Flatten:   t.Flatten,
				Extension: t.Extension,
				As:        t.As,
				Include:   t.Include,
				Exclude:   t.Exclude,
			}
			// A missing source leaves targetFiles empty; entry.SourceExists reports it.
			targetFiles, _, _ := syncpkg.DiscoverExtraTargetFiles(sourceDir, extra.File, t)
			ti.FileCount = len(targetFiles)

			if extra.File != "" {
				ti.Status = syncpkg.ExtraFileStatus(syncpkg.NewExtraFile(sourceDir, extra.File, targetPath, t.As, m))
				entry.Targets = append(entry.Targets, ti)
				continue
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
			} else if _, statErr := os.Stat(targetPath); os.IsNotExist(statErr) {
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
				ti.Status = syncpkg.CheckSyncStatus(targetFiles, sourceDir, targetPath, m, t.Flatten, outputExt)
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
			sourceDir = config.ResolveExtrasSourceDirProject(extra, projectExtrasParent, projectRoot)
		} else {
			sourceDir = config.ResolveExtrasSourceDir(extra, extrasSource, source)
		}
		if extra.File != "" {
			out = append(out, extraFileDiffEntries(extra, sourceDir, projectRoot)...)
			continue
		}
		for _, t := range extra.Targets {
			files, _, err := syncpkg.DiscoverExtraTargetFiles(sourceDir, "", t)
			if err != nil {
				// Source doesn't exist — report the target as needing creation
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
				continue
			}

			m := syncpkg.ExtraTargetMode(t.Mode, extra.File != "")
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

			targetPath := resolveExtrasTargetPath(projectRoot, t.Path)
			items := buildExtrasDiffItems(files, sourceDir, targetPath, m, t.Flatten, outputExt)
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

// extraFileDiffEntries reports each target of a single-file extra as one item.
func extraFileDiffEntries(extra config.ExtraConfig, sourceDir, projectRoot string) []extrasDiffEntry {
	var out []extrasDiffEntry
	for _, t := range extra.Targets {
		f := syncpkg.NewExtraFile(sourceDir, extra.File, resolveExtrasTargetPath(projectRoot, t.Path), t.As, t.Mode)
		entry := extrasDiffEntry{Name: extra.Name, Target: t.Path, Mode: f.Mode, Items: []extrasDiffItem{}}
		switch status := syncpkg.ExtraFileStatus(f); status {
		case "synced":
			entry.Synced = true
		case "no source":
			entry.Items = append(entry.Items, extrasDiffItem{Action: "create", File: extra.File, Reason: "no source file"})
		case "not synced":
			entry.Items = append(entry.Items, extrasDiffItem{Action: "create", File: filepath.Base(f.Target), Reason: "missing in target"})
		default:
			entry.Items = append(entry.Items, extrasDiffItem{Action: "update", File: filepath.Base(f.Target), Reason: status})
		}
		out = append(out, entry)
	}
	return out
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
			if utils.IsLinkMode(targetFile, info.Mode()) {
				link, readErr := os.Readlink(targetFile)
				if readErr != nil || filepath.Clean(resolveExtrasTargetPath(filepath.Dir(targetFile), link)) != filepath.Clean(sourceFile) {
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
			} else if outputExt == "" && !filesEqual(sourceFile, targetFile) {
				items = append(items, extrasDiffItem{
					Action: "update",
					File:   rel,
					Reason: "content differs",
				})
			}
		}
	}

	return items
}

func filesEqual(a, b string) bool {
	left, err := os.ReadFile(a)
	if err != nil {
		return false
	}
	right, err := os.ReadFile(b)
	if err != nil {
		return false
	}
	return bytes.Equal(left, right)
}

// handleExtrasCreate — POST /api/extras
func (s *Server) handleExtrasCreate(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var body struct {
		Name    string `json:"name"`
		Source  string `json:"source,omitempty"`
		Folder  string `json:"folder,omitempty"` // a folder in the extras directory, shared by several single-file extras
		File    string `json:"file,omitempty"`   // single-file extra: the file in the source dir
		Targets []struct {
			Path      string `json:"path"`
			Mode      string `json:"mode"`
			Flatten   bool   `json:"flatten"`
			Extension string `json:"extension,omitempty"`
			As        string `json:"as,omitempty"` // single-file extra: target filename
		} `json:"targets"`
	}
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
		}
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
	if body.Folder != "" && body.Source != "" {
		writeError(w, http.StatusBadRequest, "source and folder cannot both be set")
		return
	}
	if body.Folder == "." || body.Folder == ".." || strings.ContainsAny(body.Folder, `/\`) {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("folder %q must be a plain folder name", body.Folder))
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

	// A folder other than the name becomes the source: absolute in global
	// mode, relative to the project root in project mode.
	if body.Folder != "" && body.Folder != body.Name {
		if s.IsProjectMode() {
			dir := filepath.Join(s.projectCfg.EffectiveExtrasSource(s.projectRoot), body.Folder)
			rel, err := filepath.Rel(s.projectRoot, dir)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			body.Source = filepath.ToSlash(rel)
		} else {
			body.Source = filepath.Join(s.cfg.EffectiveExtrasSource(), body.Folder)
		}
	}

	// Build ExtraConfig
	extra := config.ExtraConfig{Name: body.Name, Source: body.Source, File: body.File}
	for _, t := range body.Targets {
		et := config.ExtraTargetConfig{Path: t.Path, Mode: t.Mode, Flatten: t.Flatten, Extension: t.Extension, As: t.As}
		et.Normalize()
		extra.Targets = append(extra.Targets, et)
	}
	if err := config.ValidateExtraConfig(extra); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Append to config (extras source resolution is handled by
	// s.cfg.EffectiveExtrasSource() inside s.extrasSourceDir; no backfill needed).
	// The same ownership and import checks add-target runs.
	var validateErr error
	if s.IsProjectMode() {
		prev := s.projectCfg.Extras
		s.projectCfg.Extras = append(slices.Clone(prev), extra)
		if validateErr = s.projectCfg.ValidateExtras(s.projectRoot, extra.Name); validateErr != nil {
			s.projectCfg.Extras = prev
		}
	} else {
		prev := s.cfg.Extras
		s.cfg.Extras = append(slices.Clone(prev), extra)
		if validateErr = s.cfg.ValidateExtras(extra.Name); validateErr != nil {
			s.cfg.Extras = prev
		}
	}
	if validateErr != nil {
		var conflict *config.ExtraTargetConflict
		if errors.As(validateErr, &conflict) {
			writeExtraTargetConflict(w, validateErr, conflict.Target)
		} else {
			writeError(w, http.StatusBadRequest, validateErr.Error())
		}
		return
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

type extraTargetSyncResult struct {
	Target    string   `json:"target"`
	Mode      string   `json:"mode"`
	Synced    int      `json:"synced"`
	Skipped   int      `json:"skipped"`
	Pruned    int      `json:"pruned"`
	Errors    []string `json:"errors,omitempty"`
	Error     string   `json:"error,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
	SkippedBy string   `json:"skipped_by,omitempty"`
}

type extraSyncResult struct {
	Name     string                  `json:"name"`
	Targets  []extraTargetSyncResult `json:"targets"`
	Warnings []string                `json:"warnings,omitempty"`
}

// handleExtrasSync — POST /api/extras/sync
func (s *Server) handleExtrasSync(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var body struct {
		Name   string `json:"name"`
		DryRun bool   `json:"dry_run"`
		Force  bool   `json:"force"`
	}
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); errors.Is(err, errBodyTooLarge) {
		return
	} else if err != nil && r.ContentLength > 0 {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	s.mu.RLock()
	var validationErr error
	if s.IsProjectMode() {
		validationErr = s.projectCfg.ValidateExtras(s.projectRoot)
	} else {
		validationErr = s.cfg.ValidateExtras()
	}
	if validationErr != nil {
		s.mu.RUnlock()
		if !writeExtraTargetConflict(w, validationErr, body.Name) {
			writeError(w, http.StatusBadRequest, validationErr.Error())
		}
		return
	}
	results := s.syncExtras(body.Name, body.DryRun, body.Force)
	s.mu.RUnlock()

	if body.Name != "" && len(results) == 0 {
		writeError(w, http.StatusNotFound, "extra not found: "+body.Name)
		return
	}

	status, errCount := extrasSyncStatus(results)
	s.writeOpsLog("extras-sync", status, start, map[string]any{
		"name":   body.Name,
		"dryRun": body.DryRun,
		"force":  body.Force,
		"count":  len(results),
		"errors": errCount,
		"scope":  "ui",
	}, "")

	writeJSON(w, map[string]any{"extras": results})
}

// extrasSyncStatus counts errors like `sync extras` (one per failed target plus
// each per-file error) and returns the oplog status: "ok" without errors,
// "error" when every target that ran failed, else "partial". Skipped targets
// and extras with a missing source are not failures.
func extrasSyncStatus(results []extraSyncResult) (string, int) {
	var ran, failed, errCount int
	for _, extra := range results {
		for _, t := range extra.Targets {
			if t.SkippedBy != "" {
				continue
			}
			ran++
			n := len(t.Errors)
			if t.Error != "" {
				n++
			}
			if n > 0 {
				failed++
				errCount += n
			}
		}
	}
	switch {
	case errCount == 0:
		return "ok", 0
	case failed == ran:
		return "error", errCount
	default:
		return "partial", errCount
	}
}

// syncExtras syncs every extra (or only the one named) into its targets.
// Per-target failures are reported in the results. Callers must hold s.mu.
func (s *Server) syncExtras(name string, dryRun, force bool) []extraSyncResult {
	projectRoot := s.projectRoot
	source := s.cfg.EffectiveSkillsSource()
	extrasSource := s.cfg.EffectiveExtrasSource()
	var projectExtrasParent string
	if s.IsProjectMode() {
		projectExtrasParent = s.projectCfg.EffectiveExtrasSource(s.projectRoot)
	}

	opts := syncpkg.ExtraRunOptions{
		DryRun:           dryRun,
		Force:            force,
		ProjectRoot:      projectRoot,
		ResolvePath:      func(path string) string { return resolveExtrasTargetPath(projectRoot, path) },
		ResolveExtension: s.resolveExtensionSpec,
	}
	for _, extra := range s.extrasConfig() {
		if extra.Name == config.AgentsExtraName {
			opts.AgentTargetPaths = s.extrasAgentTargetPaths()
			break
		}
	}

	results := make([]extraSyncResult, 0)

	for _, extra := range s.extrasConfig() {
		if name != "" && extra.Name != name {
			continue
		}
		var sourceDir string
		if projectRoot != "" {
			sourceDir = config.ResolveExtrasSourceDirProject(extra, projectExtrasParent, projectRoot)
		} else {
			sourceDir = config.ResolveExtrasSourceDir(extra, extrasSource, source)
		}

		run := syncpkg.RunExtraTargets(extra, sourceDir, opts)
		result := extraSyncResult{
			Name:    extra.Name,
			Targets: make([]extraTargetSyncResult, 0, len(extra.Targets)),
		}
		if run.SourceMissing {
			result.Warnings = []string{fmt.Sprintf("Source directory does not exist: %s. Create it to start syncing %s", sourceDir, extra.Name)}
		}

		for _, tr := range run.Targets {
			t := tr.Target
			out := extraTargetSyncResult{
				Target: t.Path,
				Mode:   syncpkg.ExtraTargetMode(t.Mode, extra.File != ""),
				Errors: []string{},
			}
			switch {
			case tr.SkippedBy != "":
				out.Mode = tr.Mode
				out.SkippedBy = tr.SkippedBy
			case tr.ExtensionErr != nil:
				out.Error = "extension " + t.Extension + ": " + tr.ExtensionErr.Error()
			case tr.ModeErr != nil:
				out.Error = "extension " + t.Extension + ": " + tr.ModeErr.Error()
			case tr.Err != nil:
				out.Mode = tr.Mode
				out.Error = tr.Err.Error()
			default:
				out.Mode = tr.Mode
				out.Synced = tr.Result.Synced
				out.Skipped = tr.Result.Skipped
				out.Pruned = tr.Result.Pruned
				out.Warnings = tr.Result.Warnings
				if tr.Result.Errors != nil {
					out.Errors = tr.Result.Errors
				}
			}
			result.Targets = append(result.Targets, out)
		}

		results = append(results, result)
	}

	return results
}

// extrasAgentTargetPaths returns the agents sync target paths for
// syncpkg.ExtraRunOptions.AgentTargetPaths. Callers must hold s.mu.
func (s *Server) extrasAgentTargetPaths() map[string]bool {
	builtinAgents := s.builtinAgentTargets()
	var paths []string
	for name, target := range s.cfg.Targets {
		if p := resolveAgentPath(target, builtinAgents, name, s.IsProjectMode()); p != "" {
			paths = append(paths, resolveExtrasTargetPath(s.projectRoot, p))
		}
	}
	return syncpkg.AgentTargetPaths(s.agentsSource(), paths)
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
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
		}
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
	unlock, lockErr := s.lockRemoteExtraMutation(name)
	if lockErr != nil {
		writeError(w, http.StatusConflict, lockErr.Error())
		return
	}
	defer unlock()

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
				// force copy, overriding any prior mode on the target. Filters
				// are not dropped here: symlink with filters fails validation.
				if newExtension != "" && body.Mode != "" && body.Mode != "copy" {
					writeError(w, http.StatusBadRequest, "extension requires copy mode, but mode "+body.Mode+" was set on the target")
					return
				}
				newMode = config.ExtraTargetConfig{Mode: newMode, Extension: newExtension}.EffectiveMode()

				// Validate the combination
				if err := config.ValidateExtraFlatten(newFlatten, newMode); err != nil {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
				if extra.File != "" || config.ManagedExtraMode(newMode) || len(t.Include) > 0 || len(t.Exclude) > 0 {
					changed := config.ExtraTargetConfig{Path: t.Path, Mode: newMode, Flatten: newFlatten, Extension: newExtension, As: t.As, Include: t.Include, Exclude: t.Exclude}
					if err := config.ValidateExtraConfig(config.ExtraConfig{Name: extra.Name, File: extra.File, Targets: []config.ExtraTargetConfig{changed}}); err != nil {
						writeError(w, http.StatusBadRequest, err.Error())
						return
					}
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

// handleExtrasAddTarget — POST /api/extras/{name}/targets
// Adds a new target to an existing extra. Config-only (no sync).
func (s *Server) handleExtrasAddTarget(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	name := r.PathValue("name")

	var body struct {
		Path    string `json:"path"`
		Mode    string `json:"mode"`
		Flatten bool   `json:"flatten"`
		As      string `json:"as"` // single-file extra: target filename
	}
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
		}
		return
	}
	if body.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}
	if err := config.ValidateExtraMode(body.Mode); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := config.ValidateExtraFlatten(body.Flatten, body.Mode); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, lockErr := s.lockRemoteExtraMutation(name)
	if lockErr != nil {
		writeError(w, http.StatusConflict, lockErr.Error())
		return
	}
	defer unlock()

	extras := s.extrasConfig()
	idx := -1
	for i := range extras {
		if extras[i].Name == name {
			idx = i
			break
		}
	}
	if idx == -1 {
		writeError(w, http.StatusNotFound, "extra not found: "+name)
		return
	}
	newPath := filepath.Clean(resolveExtrasTargetPath(s.projectRoot, body.Path))
	for _, t := range extras[idx].Targets {
		if filepath.Clean(resolveExtrasTargetPath(s.projectRoot, t.Path)) == newPath {
			writeError(w, http.StatusConflict, "target already exists: "+body.Path)
			return
		}
	}

	storedPath := body.Path
	if !s.IsProjectMode() {
		storedPath = newPath
	}
	et := config.ExtraTargetConfig{Path: storedPath, Flatten: body.Flatten, As: body.As}
	if body.Mode != "" {
		et.Mode = body.Mode
	}
	if err := config.ValidateExtraConfig(config.ExtraConfig{Name: name, File: extras[idx].File, Targets: []config.ExtraTargetConfig{et}}); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	prev := extras[idx].Targets
	extras[idx].Targets = append(slices.Clone(prev), et)

	// The same ownership and import checks the CLI runs after --add-target.
	var validateErr error
	if s.IsProjectMode() {
		validateErr = s.projectCfg.ValidateExtras(s.projectRoot, name)
	} else {
		validateErr = s.cfg.ValidateExtras(name)
	}
	if validateErr != nil {
		extras[idx].Targets = prev
		if !writeExtraTargetConflict(w, validateErr, body.Path) {
			writeError(w, http.StatusBadRequest, validateErr.Error())
		}
		return
	}

	if err := s.saveAndReloadConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeOpsLog("extras-target", "ok", start, map[string]any{
		"name": name, "target": body.Path, "action": "add", "scope": "ui",
	}, "")

	writeJSON(w, map[string]any{"success": true, "name": name, "target": body.Path})
}

// handleExtrasRemoveTarget — DELETE /api/extras/{name}/targets
// Removes one target from an existing extra. For a directory extra it is
// config-only; a single-file extra's target is restored first (its link or
// import line goes and the file it replaced comes back), like --prune.
func (s *Server) handleExtrasRemoveTarget(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	name := r.PathValue("name")

	var body struct {
		Path string `json:"path"`
	}
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
		}
		return
	}
	if body.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, lockErr := s.lockRemoteExtraMutation(name)
	if lockErr != nil {
		writeError(w, http.StatusConflict, lockErr.Error())
		return
	}
	defer unlock()

	extras := s.extrasConfig()
	idx := -1
	for i := range extras {
		if extras[i].Name == name {
			idx = i
			break
		}
	}
	if idx == -1 {
		writeError(w, http.StatusNotFound, "extra not found: "+name)
		return
	}
	tIdx := -1
	for j, t := range extras[idx].Targets {
		if t.Path == body.Path {
			tIdx = j
			break
		}
	}
	if tIdx == -1 {
		writeError(w, http.StatusNotFound, "target not found: "+body.Path)
		return
	}
	if len(extras[idx].Targets) == 1 && extras[idx].File == "" {
		writeError(w, http.StatusBadRequest, "cannot remove the last target; delete the extra instead")
		return
	}

	if extra := extras[idx]; extra.File != "" {
		t := extra.Targets[tIdx]
		f := syncpkg.NewExtraFile(s.extrasSourceDir(extra), extra.File, resolveExtrasTargetPath(s.projectRoot, t.Path), t.As, t.Mode)
		if _, err := syncpkg.RestoreExtraTarget(f); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to restore target: "+err.Error())
			return
		}
	}

	extras[idx].Targets = append(extras[idx].Targets[:tIdx], extras[idx].Targets[tIdx+1:]...)

	if err := s.saveAndReloadConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeOpsLog("extras-target", "ok", start, map[string]any{
		"name": name, "target": body.Path, "action": "remove", "scope": "ui",
	}, "")

	writeJSON(w, map[string]any{"success": true, "name": name, "target": body.Path})
}
