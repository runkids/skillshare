package server

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/instructions"
	syncpkg "skillshare/internal/sync"
)

// A location is a target of a shared instruction file that is not the global
// instruction file of a listed tool (instructionTargets): any folder the file
// is written to, with an optional custom file name, like the CLI's
// `extras <name> --add-target <dir> --as <file>`. In project mode every target
// is a location, stored relative to the project root.

type sharedInstructionsLocation struct {
	Path   string `json:"path"` // target folder as stored in the config
	File   string `json:"file"` // resolved file written
	As     string `json:"as,omitempty"`
	Mode   string `json:"mode"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// sharedLocations returns the locations of extra: its targets whose file is
// none of tools. Callers must hold s.mu.
func (s *Server) sharedLocations(extra config.ExtraConfig, tools []sharedInstructionsTarget) []sharedInstructionsLocation {
	res := s.instructionsResolver()
	out := []sharedInstructionsLocation{}
	for j, tc := range extra.Targets {
		f := instructions.ExtraFile(extra, j, res)
		if slices.ContainsFunc(tools, func(t sharedInstructionsTarget) bool { return filepath.Clean(t.Path) == filepath.Clean(f.Target) }) {
			continue
		}
		loc := sharedInstructionsLocation{Path: tc.Path, File: f.Target, As: tc.As, Mode: f.Mode}
		loc.Status, loc.Reason = instructions.FileStatus(f)
		out = append(out, loc)
	}
	return out
}

// findLocation returns the index of the named shared file and of its target
// stored as path (compared after expanding ~ or, in project mode, joining the
// project root), or -1. Callers must hold s.mu.
func (s *Server) findLocation(name, path string) (int, int) {
	for i, extra := range *s.sharedExtras() {
		if extra.Name != name || extra.File == "" {
			continue
		}
		want := filepath.Clean(resolveExtrasTargetPath(s.projectRoot, path))
		for j, tc := range extra.Targets {
			if tc.Path == path || filepath.Clean(resolveExtrasTargetPath(s.projectRoot, tc.Path)) == want {
				return i, j
			}
		}
		return i, -1
	}
	return -1, -1
}

// handleAddSharedInstructionsLocation — POST /api/instructions/{name}/locations
// Writes the shared file into any folder, under its own name or as, and syncs
// that location right away. Nothing is saved when it cannot be written.
func (s *Server) handleAddSharedInstructionsLocation(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	name := r.PathValue("name")
	var body struct {
		Path string `json:"path"`
		As   string `json:"as"`
		Mode string `json:"mode"`
	}
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeCodedError(w, http.StatusBadRequest, "instructions_invalid_json", "invalid JSON body", map[string]string{})
		}
		return
	}
	if body.Path == "" {
		writeCodedError(w, http.StatusBadRequest, "instructions_location_required", "path is required", map[string]string{})
		return
	}
	if body.Mode == "" {
		body.Mode = "symlink"
		if !syncpkg.CanCreateFileLink() {
			body.Mode = "copy"
		}
	}
	if !checkSharedMode(w, body.Mode) {
		return
	}
	// stored is the folder as saved in the config; dir is its absolute path.
	stored := filepath.Clean(config.ExpandPath(body.Path))
	dir := stored
	if s.IsProjectMode() {
		rel, ok := projectRelativeDir(body.Path)
		if !ok {
			writeCodedError(w, http.StatusBadRequest, "instructions_location_outside_project", "folder must be relative and inside the project: "+body.Path, map[string]string{"path": body.Path})
			return
		}
		stored, dir = rel, filepath.Join(s.projectRoot, filepath.FromSlash(rel))
	} else if !filepath.IsAbs(dir) {
		writeCodedError(w, http.StatusBadRequest, "instructions_location_not_absolute", "folder must be an absolute path or start with ~: "+body.Path, map[string]string{"path": body.Path})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	unlock := s.lockInstructionWrite(w, s.addLocationInstructionMutation(name, stored, body.As))
	if unlock == nil {
		return
	}
	defer unlock()
	i, j := s.findLocation(name, stored)
	if i == -1 {
		writeCodedError(w, http.StatusNotFound, "instructions_shared_not_found", "shared instruction file not found: "+name, map[string]string{"name": name})
		return
	}
	extras := *s.sharedExtras()
	extra := extras[i]
	tc := config.ExtraTargetConfig{Path: stored, Mode: body.Mode}
	if body.As != extra.File {
		tc.As = body.As
	}
	if err := config.ValidateExtraConfig(config.ExtraConfig{Name: name, File: extra.File, Targets: []config.ExtraTargetConfig{tc}}); err != nil {
		writeCodedError(w, http.StatusBadRequest, "instructions_invalid_file_name", err.Error(), map[string]string{"as": body.As, "detail": err.Error()})
		return
	}
	as := tc.As
	if as == "" {
		as = extra.File
	}
	file := filepath.Join(dir, as)
	for _, t := range s.instructionTargets() {
		if filepath.Clean(t.Path) == file {
			writeCodedError(w, http.StatusConflict, "instructions_location_is_target", file+" is the instruction file of "+t.Name, map[string]string{"target": t.Name, "path": file})
			return
		}
	}
	// After the tool check: a connected tool's folder is also a target of the extra.
	if j != -1 {
		writeCodedError(w, http.StatusConflict, "instructions_location_exists", name+" already has the location "+stored, map[string]string{"path": stored})
		return
	}
	if info, err := os.Lstat(file); err == nil && info.IsDir() {
		writeCodedError(w, http.StatusConflict, "instructions_location_directory", file+" is a directory", map[string]string{"path": file})
		return
	}

	prev := extras[i].Targets
	extras[i].Targets = append(slices.Clone(prev), tc)
	if err := s.validateSharedExtras(name); err != nil {
		extras[i].Targets = prev
		if !writeExtraTargetConflict(w, err, file) {
			writeCodedError(w, http.StatusBadRequest, "instructions_location_failed", err.Error(), map[string]string{"detail": err.Error()})
		}
		return
	}
	args := map[string]any{"name": name, "path": stored, "as": tc.As, "mode": tc.Mode, "action": "add", "scope": "ui"}
	fail := func(status int, code string, err string, params map[string]string) {
		extras[i].Targets = prev
		s.writeOpsLog("instructions-location", "error", start, args, err)
		writeCodedError(w, status, code, err, params)
	}
	f := instructions.ExtraFile(extras[i], len(prev), s.instructionsResolver())
	result, err := syncpkg.SyncExtraFile(f, false, s.projectRoot)
	if err != nil {
		fail(http.StatusInternalServerError, "instructions_location_failed", err.Error(), map[string]string{"detail": err.Error()})
		return
	}
	if result.Skipped > 0 {
		fail(http.StatusConflict, "instructions_location_directory", file+" is a directory", map[string]string{"path": file})
		return
	}
	if err := s.saveConfig(); err != nil {
		// The file is already written: put back what it held, so no location is left
		// on disk that the config does not know about.
		err = fmt.Errorf("failed to save config: %w", err)
		if _, rerr := syncpkg.RestoreExtraTarget(f); rerr != nil {
			err = fmt.Errorf("%w; restoring %s also failed: %v", err, file, rerr)
		}
		fail(http.StatusInternalServerError, "instructions_location_failed", err.Error(), map[string]string{"detail": err.Error()})
		return
	}
	if err := s.reloadConfig(); err != nil {
		// Saved: the location exists, so it stays; only the answer reports the error.
		s.writeOpsLog("instructions-location", "error", start, args, err.Error())
		writeCodedError(w, http.StatusInternalServerError, "instructions_location_failed", "failed to reload config: "+err.Error(), map[string]string{"detail": err.Error()})
		return
	}
	s.writeOpsLog("instructions-location", "ok", start, args, "")
	writeJSON(w, map[string]any{"success": true, "warnings": append([]syncpkg.FileWarning{}, result.FileWarnings...)})
}

// projectRelativeDir cleans a project-mode location folder to a relative
// path with forward slashes. It refuses absolute paths, ~ and paths that
// leave the project root.
func projectRelativeDir(path string) (string, bool) {
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") ||
		strings.HasPrefix(path, "/") || strings.HasPrefix(path, "\\") || filepath.IsAbs(path) || filepath.VolumeName(path) != "" {
		return "", false
	}
	rel := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return "", false
	}
	return rel, true
}

// lookupLocation finds the location named by path, answering 404 when the
// shared file or the location is missing. Callers must hold s.mu.
func (s *Server) lookupLocation(w http.ResponseWriter, name, path string) (int, int, bool) {
	if path == "" {
		writeCodedError(w, http.StatusBadRequest, "instructions_location_required", "path is required", map[string]string{})
		return 0, 0, false
	}
	i, j := s.findLocation(name, path)
	if i == -1 {
		writeCodedError(w, http.StatusNotFound, "instructions_shared_not_found", "shared instruction file not found: "+name, map[string]string{"name": name})
		return 0, 0, false
	}
	if j == -1 {
		writeCodedError(w, http.StatusNotFound, "instructions_location_not_found", name+" has no location "+path, map[string]string{"path": path})
		return 0, 0, false
	}
	return i, j, true
}

// handlePutSharedInstructionsLocationMode — PUT /api/instructions/{name}/locations/mode
func (s *Server) handlePutSharedInstructionsLocationMode(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	name := r.PathValue("name")
	var body struct {
		Path string `json:"path"`
		Mode string `json:"mode"`
	}
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeCodedError(w, http.StatusBadRequest, "instructions_invalid_json", "invalid JSON body", map[string]string{})
		}
		return
	}
	if !checkSharedMode(w, body.Mode) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock := s.lockInstructionWrite(w, s.locationInstructionMutation(name, body.Path))
	if unlock == nil {
		return
	}
	defer unlock()
	i, j, ok := s.lookupLocation(w, name, body.Path)
	if !ok {
		return
	}
	file := instructions.ExtraFile((*s.sharedExtras())[i], j, s.instructionsResolver()).Target
	s.setSharedTargetMode(w, start, i, j, body.Mode, file, file, "instructions_location_directory",
		map[string]any{"name": name, "path": body.Path, "mode": body.Mode, "scope": "ui"})
}

// handleDeleteSharedInstructionsLocation — DELETE /api/instructions/{name}/locations?path=X
// Puts back what the location held before the shared file was attached and
// removes it from the config.
func (s *Server) handleDeleteSharedInstructionsLocation(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	name, path := r.PathValue("name"), r.URL.Query().Get("path")
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock := s.lockInstructionWrite(w, s.locationInstructionMutation(name, path))
	if unlock == nil {
		return
	}
	defer unlock()
	i, j, ok := s.lookupLocation(w, name, path)
	if !ok {
		return
	}
	args := map[string]any{"name": name, "path": path, "action": "remove", "scope": "ui"}
	extras := *s.sharedExtras()
	if _, err := syncpkg.RestoreExtraTarget(instructions.ExtraFile(extras[i], j, s.instructionsResolver())); err != nil {
		s.writeOpsLog("instructions-location", "error", start, args, err.Error())
		writeCodedError(w, http.StatusInternalServerError, "instructions_restore_failed", err.Error(), map[string]string{"name": name, "detail": err.Error()})
		return
	}
	extras[i].Targets = slices.Delete(slices.Clone(extras[i].Targets), j, j+1)
	if err := s.saveAndReloadConfig(); err != nil {
		s.writeOpsLog("instructions-location", "error", start, args, err.Error())
		writeCodedError(w, http.StatusInternalServerError, "instructions_save_failed", err.Error(), map[string]string{"name": name, "detail": err.Error()})
		return
	}
	s.writeOpsLog("instructions-location", "ok", start, args, "")
	writeJSON(w, map[string]any{"success": true, "warnings": []syncpkg.FileWarning{}})
}

// handleSharedInstructionsLocationRestorePreview — GET /api/instructions/{name}/locations/restore-preview?path=X
func (s *Server) handleSharedInstructionsLocationRestorePreview(w http.ResponseWriter, r *http.Request) {
	name, path := r.PathValue("name"), r.URL.Query().Get("path")
	s.mu.RLock()
	defer s.mu.RUnlock()
	i, j, ok := s.lookupLocation(w, name, path)
	if !ok {
		return
	}
	writeJSON(w, restorePreview(instructions.ExtraFile((*s.sharedExtras())[i], j, s.instructionsResolver())))
}
