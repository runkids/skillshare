package server

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"time"

	"skillshare/internal/config"
	syncpkg "skillshare/internal/sync"
)

// handleExtrasPreview — POST /api/extras/{name}/preview
// Shows what draft include and exclude patterns do to the extra's source files.
func (s *Server) handleExtrasPreview(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Include []string `json:"include"`
		Exclude []string `json:"exclude"`
	}
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
		}
		return
	}

	s.mu.RLock()
	extras := s.extrasConfig()
	idx := slices.IndexFunc(extras, func(e config.ExtraConfig) bool { return e.Name == r.PathValue("name") })
	if idx < 0 {
		s.mu.RUnlock()
		writeError(w, http.StatusNotFound, "extra not found: "+r.PathValue("name"))
		return
	}
	sourceDir := s.extrasSourceDir(extras[idx])
	s.mu.RUnlock()

	files, err := syncpkg.DiscoverExtraFiles(sourceDir)
	if err != nil {
		writeError(w, http.StatusBadRequest, "source not readable: "+err.Error())
		return
	}
	writeJSON(w, map[string]any{
		"files":     syncpkg.PreviewExtraFilter(files, body.Include, body.Exclude),
		"unmatched": syncpkg.UnmatchedIncludes(files, body.Include),
	})
}

// handleExtrasEditTarget — PUT /api/extras/{name}/targets
// Replaces one target's settings. A new folder or file name removes what
// skillshare put at the old place (copies stay) and syncs the extra; other
// changes only write config and take effect on the next sync.
func (s *Server) handleExtrasEditTarget(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	name := r.PathValue("name")

	var body struct {
		Path   string `json:"path"` // the target as stored in config
		Target struct {
			Path      string   `json:"path"`
			Mode      string   `json:"mode"`
			Flatten   bool     `json:"flatten"`
			Extension string   `json:"extension"`
			As        string   `json:"as"`
			Include   []string `json:"include"`
			Exclude   []string `json:"exclude"`
		} `json:"target"`
	}
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
		}
		return
	}
	if body.Path == "" || body.Target.Path == "" {
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
	idx := slices.IndexFunc(extras, func(e config.ExtraConfig) bool { return e.Name == name })
	if idx == -1 {
		writeError(w, http.StatusNotFound, "extra not found: "+name)
		return
	}
	extra := extras[idx]
	tIdx := slices.IndexFunc(extra.Targets, func(t config.ExtraTargetConfig) bool { return t.Path == body.Path })
	if tIdx == -1 {
		writeError(w, http.StatusNotFound, "target not found: "+body.Path)
		return
	}
	old := extra.Targets[tIdx]

	in := body.Target
	next := config.ExtraTargetConfig{Path: in.Path, Mode: in.Mode, Flatten: in.Flatten, Extension: in.Extension, As: in.As, Include: in.Include, Exclude: in.Exclude}
	next.Normalize()
	newPath := filepath.Clean(resolveExtrasTargetPath(s.projectRoot, next.Path))
	if !s.IsProjectMode() {
		next.Path = newPath
	}
	oldPath := filepath.Clean(resolveExtrasTargetPath(s.projectRoot, old.Path))
	for j, t := range extra.Targets {
		if j != tIdx && filepath.Clean(resolveExtrasTargetPath(s.projectRoot, t.Path)) == newPath {
			writeError(w, http.StatusConflict, "target already exists: "+in.Path)
			return
		}
	}
	if err := config.ValidateExtraConfig(config.ExtraConfig{Name: name, File: extra.File, Targets: []config.ExtraTargetConfig{next}}); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	prev := extra.Targets
	extras[idx].Targets = slices.Clone(prev)
	extras[idx].Targets[tIdx] = next
	if err := s.validateExtra(name); err != nil {
		extras[idx].Targets = prev
		if !writeExtraTargetConflict(w, err, in.Path) {
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	if err := s.saveAndReloadConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	moved := newPath != oldPath || next.As != old.As
	resp := map[string]any{"success": true, "name": name, "target": next.Path}
	status, msg := "ok", ""
	if moved {
		sourceDir := s.extrasSourceDir(extra)
		oldMode := syncpkg.ExtraTargetMode(old.Mode, extra.File != "")
		var errs []string
		resp["pruned"], _, errs = syncpkg.ClearExtraTarget(extra.File, sourceDir, oldPath, old.As, oldMode, nil)
		results := s.syncExtras(name, false, false)
		resp["extras"] = results
		if len(errs) > 0 {
			resp["prune_errors"] = errs
		}
		if status, _ = extrasSyncStatus(results); len(errs) > 0 {
			status, msg = "partial", errs[0]
		}
	}

	s.writeOpsLog("extras-target", status, start, map[string]any{
		"name": name, "target": next.Path, "from": old.Path, "action": "edit", "scope": "ui",
	}, msg)
	writeJSON(w, resp)
}

// handleExtrasEdit — PATCH /api/extras/{name}
// Renames an extra or points it at another source folder. A rename keeps the
// folder: an extra in its default folder gets that folder as its source. A new
// source must already exist; the old folder is left as it is, and every target
// is relinked to the new one.
func (s *Server) handleExtrasEdit(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	name := r.PathValue("name")

	var body struct {
		Name string `json:"name"`
		// Source is left alone when omitted; "" means the default folder. Global:
		// a path; project: relative to the project root or a path inside it.
		Source *string `json:"source"`
	}
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
		}
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
	idx := slices.IndexFunc(extras, func(e config.ExtraConfig) bool { return e.Name == name })
	if idx == -1 {
		writeError(w, http.StatusNotFound, "extra not found: "+name)
		return
	}
	extra := extras[idx]
	if extra.Remote != "" && (body.Source != nil || body.Name != "" && body.Name != name) {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("%q comes from %s; to rename or move it run: skillshare extras remove %s && skillshare install %s --kind extra --name <new>", name, extra.Remote, name, extra.Remote))
		return
	}

	oldSource := s.extrasSourceDir(extra)
	next := extra

	if body.Name != "" && body.Name != name {
		if err := config.ValidateExtraRename(name, body.Name); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := config.ValidateExtraName(body.Name); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		others := slices.DeleteFunc(slices.Clone(extras), func(e config.ExtraConfig) bool { return e.Name == name })
		if err := config.ValidateExtraNameUnique(body.Name, others); err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		next.Name = body.Name
		if next.Source == "" {
			source, err := s.storedExtraSource(oldSource)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			next.Source = source
		}
	}

	if body.Source != nil {
		source := *body.Source
		if source != "" && s.IsProjectMode() {
			if abs := config.ExpandPath(source); filepath.IsAbs(abs) {
				if rel, err := filepath.Rel(s.projectRoot, abs); err == nil {
					source = rel
				}
			}
			if err := config.ValidateProjectExtraSource(source); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			source = filepath.ToSlash(filepath.Clean(source))
		} else if source != "" {
			source = filepath.Clean(config.ExpandPath(source))
		}
		next.Source = source
	}
	newSource := s.extrasSourceDir(next)
	relink := filepath.Clean(newSource) != filepath.Clean(oldSource)
	if relink {
		if extra.File != "" {
			if info, err := os.Stat(filepath.Join(newSource, extra.File)); err != nil || info.IsDir() {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("%s not found in %s", extra.File, newSource))
				return
			}
		} else if info, err := os.Stat(newSource); err != nil || !info.IsDir() {
			writeError(w, http.StatusBadRequest, "folder not found: "+newSource)
			return
		}
	}

	extras[idx] = next
	if err := s.validateExtra(next.Name); err != nil {
		extras[idx] = extra
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.saveAndReloadConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Links into the old folder go only once the config points elsewhere, so a
	// failed save leaves working targets alone.
	var pruneErrs []string
	if relink {
		for _, t := range extra.Targets {
			mode := syncpkg.ExtraTargetMode(t.Mode, extra.File != "")
			_, _, errs := syncpkg.ClearExtraTarget(extra.File, oldSource, resolveExtrasTargetPath(s.projectRoot, t.Path), t.As, mode, nil)
			pruneErrs = append(pruneErrs, errs...)
		}
	}

	resp := map[string]any{"success": true, "name": next.Name}
	status, msg := "ok", ""
	if relink {
		results := s.syncExtras(next.Name, false, false)
		resp["extras"] = results
		status, _ = extrasSyncStatus(results)
	}
	if len(pruneErrs) > 0 {
		resp["prune_errors"] = pruneErrs
		status, msg = "partial", pruneErrs[0]
	}

	s.writeOpsLog("extras-edit", status, start, map[string]any{
		"name": next.Name, "from": name, "source": next.Source, "scope": "ui",
	}, msg)
	writeJSON(w, resp)
}

// storedExtraSource is dir as an extra's source field: absolute in global
// mode, relative to the project root in project mode.
func (s *Server) storedExtraSource(dir string) (string, error) {
	if !s.IsProjectMode() {
		return dir, nil
	}
	rel, err := filepath.Rel(s.projectRoot, dir)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

// validateExtra runs the ownership and import checks for one extra against the
// whole config. Callers must hold s.mu.
func (s *Server) validateExtra(name string) error {
	if s.IsProjectMode() {
		return s.projectCfg.ValidateExtras(s.projectRoot, name)
	}
	return s.cfg.ValidateExtras(name)
}
