package server

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"skillshare/internal/config"
	"skillshare/internal/instructions"
	syncpkg "skillshare/internal/sync"
)

// Shared instruction files are single-file extras whose file is AGENTS.md (see
// instructions.IsShared), attached to the global instruction files of targets.
// Other single-file extras belong to the Extras page. In project mode they are
// the project's AGENTS.md extras, with every target listed as a location; the project's
// own ./AGENTS.md is handled in handler_instructions_project.go.

func (s *Server) requireGlobalInstructions(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.IsProjectMode() {
			writeCodedError(w, http.StatusBadRequest, "instructions_global_required", "shared instruction files are set up in global mode", map[string]string{})
			return
		}
		next(w, r)
	}
}

// sharedExtras returns the extras of the current mode, to change in place.
// Callers must hold s.mu.
func (s *Server) sharedExtras() *[]config.ExtraConfig {
	if s.IsProjectMode() {
		return &s.projectCfg.Extras
	}
	return &s.cfg.Extras
}

// validateSharedExtras runs the ownership and import checks of the current
// mode for the named extra. Callers must hold s.mu.
func (s *Server) validateSharedExtras(name string) error {
	if s.IsProjectMode() {
		return s.projectCfg.ValidateExtras(s.projectRoot, name)
	}
	return s.cfg.ValidateExtras(name)
}

type sharedInstructionsFile struct {
	Name    string `json:"name"`
	File    string `json:"file"`
	Path    string `json:"path"`
	Exists  bool   `json:"exists"`
	Size    int64  `json:"size"`
	Chars   int    `json:"chars"`
	Targets int    `json:"targets"`
	// Locations are targets outside the tools listed in targets.
	Locations []sharedInstructionsLocation `json:"locations"`
}

type sharedInstructionsTarget struct {
	LinkedShared string                    `json:"linked_shared,omitempty"` // shared source reached by the target, including untracked links
	Name         string                    `json:"name"`
	Path         string                    `json:"path"`
	Import       bool                      `json:"import"`
	Exists       bool                      `json:"exists"`
	SameAs       string                    `json:"same_as,omitempty"`  // locked to this target: both read the same file
	RiderOf      string                    `json:"rider_of,omitempty"` // not configured; reads this configured target's skills
	MaxChars     int                       `json:"max_chars,omitempty"`
	Assigned     []instructions.Assignment `json:"assigned"`
}

// instructionTargets returns the configured targets with a global instruction
// file and their riders (see config.InstructionRiders), sorted by name.
// Empty in project mode, where tools have no global file. Callers must hold s.mu.
func (s *Server) instructionTargets() []sharedInstructionsTarget {
	if s.IsProjectMode() {
		return []sharedInstructionsTarget{}
	}
	names := make([]string, 0, len(s.cfg.Targets))
	for name := range s.cfg.Targets {
		names = append(names, name)
	}
	sort.Strings(names)
	res := s.instructionsResolver()
	out := []sharedInstructionsTarget{}
	for _, name := range names {
		it, ok := config.TargetInstructions(name, s.cfg.Targets[name], false)
		if !ok {
			continue
		}
		t := sharedInstructionsTarget{Name: name, Path: it.Path, Import: it.Import, MaxChars: it.MaxChars}
		if _, found := s.cfg.Targets[it.SameAs]; found {
			t.SameAs = it.SameAs
		}
		_, err := os.Stat(it.Path)
		t.Exists = err == nil
		t.Assigned = instructions.Assignments(instructions.Shared(s.cfg.Extras), it.Path, res)
		t.LinkedShared = s.sharedLinkName(it.Path)
		out = append(out, t)
	}
	for _, r := range s.instructionRiders() {
		t := sharedInstructionsTarget{Name: r.Name, Path: r.Path, Import: r.Import, MaxChars: r.MaxChars, RiderOf: r.Via}
		_, err := os.Stat(r.Path)
		t.Exists = err == nil
		t.Assigned = instructions.Assignments(instructions.Shared(s.cfg.Extras), r.Path, res)
		t.LinkedShared = s.sharedLinkName(r.Path)
		out = append(out, t)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// handleListSharedInstructions — GET /api/instructions
func (s *Server) handleListSharedInstructions(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tools := s.instructionTargets()
	files := []sharedInstructionsFile{}
	for _, extra := range s.extrasConfig() {
		if !instructions.IsShared(extra) {
			continue
		}
		f := sharedInstructionsFile{Name: extra.Name, File: extra.File, Path: filepath.Join(s.extrasSourceDir(extra), extra.File), Targets: len(extra.Targets), Locations: s.sharedLocations(extra, tools)}
		if data, err := readLimited(f.Path); err == nil {
			f.Exists, f.Size, f.Chars = true, int64(len(data)), utf8.RuneCount(data)
		}
		files = append(files, f)
	}
	// file_links: false on Windows without Developer Mode, where link modes copy.
	writeJSON(w, map[string]any{"files": files, "targets": tools, "file_links": syncpkg.CanCreateFileLink()})
}

// handleCreateSharedInstructions — POST /api/instructions
// Creates a shared instruction file with content, unattached, or moves a
// target's current file into it (from_target) and attaches that target, so
// the content is not read twice.
func (s *Server) handleCreateSharedInstructions(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var body struct {
		Name       string `json:"name"`
		Content    string `json:"content"`
		FromTarget string `json:"from_target,omitempty"`
	}
	if err := decodeJSON(w, r, &body, maxInstructionsBytes+4096); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeCodedError(w, http.StatusBadRequest, "instructions_invalid_json", "invalid JSON body", map[string]string{})
		}
		return
	}
	if err := config.ValidateExtraName(body.Name); err != nil {
		writeCodedError(w, http.StatusBadRequest, "instructions_invalid_name", err.Error(), map[string]string{"name": body.Name})
		return
	}

	if body.FromTarget != "" && s.IsProjectMode() {
		writeCodedError(w, http.StatusBadRequest, "instructions_global_required", "moving a tool's file into a shared file is done in global mode", map[string]string{})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, lockErr := s.lockRemoteExtraMutation()
	if lockErr != nil {
		writeCodedError(w, http.StatusConflict, "instructions_write_failed", lockErr.Error(), map[string]string{"detail": lockErr.Error()})
		return
	}
	defer unlock()
	extras := s.sharedExtras()
	if err := config.ValidateExtraNameUnique(body.Name, *extras); err != nil {
		writeCodedError(w, http.StatusConflict, "instructions_name_taken", err.Error(), map[string]string{"name": body.Name})
		return
	}
	extra := config.ExtraConfig{Name: body.Name, File: instructions.AgentsFile, Targets: []config.ExtraTargetConfig{}}
	path := filepath.Join(s.extrasSourceDir(extra), extra.File)
	if _, err := os.Lstat(path); err == nil {
		writeCodedError(w, http.StatusConflict, "instructions_path_exists", path+" already exists", map[string]string{"path": path})
		return
	}
	args := map[string]any{"name": body.Name, "from_target": body.FromTarget, "scope": "ui"}
	fail := func(status int, err error) {
		s.writeOpsLog("instructions-create", "error", start, args, err.Error())
		writeCodedError(w, status, "instructions_create_failed", err.Error(), map[string]string{"detail": err.Error()})
	}

	if body.FromTarget == "" {
		if err := instructions.WriteFile(path, body.Content); err != nil {
			fail(http.StatusInternalServerError, err)
			return
		}
		*extras = append(*extras, extra)
	} else if status, err := s.moveTargetIntoShared(body.FromTarget, &extra, path); err != nil {
		fail(status, err)
		return
	}
	if err := s.saveAndReloadConfig(); err != nil {
		fail(http.StatusInternalServerError, err)
		return
	}
	s.writeOpsLog("instructions-create", "ok", start, args, "")
	writeJSON(w, map[string]any{"success": true, "name": body.Name, "path": path})
}

// moveTargetIntoShared moves the named target's file into the new shared file
// at path and attaches the target to it. An import target keeps only its
// tool lines plus the managed import (as convert does); any other target is
// backed up and linked. It appends extra to the config. Callers must hold
// s.mu and save the config afterwards.
func (s *Server) moveTargetIntoShared(target string, extra *config.ExtraConfig, path string) (int, error) {
	it, found, ok := s.targetInstructions(target)
	if !found {
		return http.StatusBadRequest, fmt.Errorf("target not found: %s", target)
	}
	if !ok {
		return http.StatusBadRequest, fmt.Errorf("%s has no global instruction file", target)
	}
	if _, linked := s.cfg.Targets[it.SameAs]; linked {
		return http.StatusBadRequest, fmt.Errorf("%s reads the same file as %s; use %s instead", target, it.SameAs, it.SameAs)
	}

	if it.Import {
		changes, err := instructions.PlanConvert(it.Path, instructions.ConvertOptions{
			Method: instructions.MethodImport, KeepToolLines: true, Dest: path, Managed: true,
		})
		if err != nil {
			return http.StatusBadRequest, err
		}
		if err := instructions.Apply(changes); err != nil {
			return http.StatusInternalServerError, err
		}
		extra.Targets = append(extra.Targets, config.ExtraTargetConfig{Path: filepath.Dir(it.Path), As: filepath.Base(it.Path), Mode: "import"})
		s.cfg.Extras = append(s.cfg.Extras, *extra)
		return 0, nil
	}

	data, err := readLimited(it.Path)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("could not read %s: %w", it.Path, err)
	}
	if err := instructions.WriteFile(path, string(data)); err != nil {
		return http.StatusInternalServerError, err
	}
	s.cfg.Extras = append(s.cfg.Extras, *extra)
	if err := s.assignTarget(target, []string{extra.Name}); err != nil {
		// The shared file exists; keep it in the config so it is not orphaned.
		if saveErr := s.saveAndReloadConfig(); saveErr != nil {
			err = fmt.Errorf("%w; %v", err, saveErr)
		}
		return http.StatusInternalServerError, err
	}
	return 0, nil
}

// sharedExtra returns the named single-file extra. Callers must hold s.mu.
func (s *Server) sharedExtra(name string) (config.ExtraConfig, bool) {
	for _, extra := range s.extrasConfig() {
		if extra.Name == name && extra.File != "" {
			return extra, true
		}
	}
	return config.ExtraConfig{}, false
}

// handleGetSharedInstructionsContent — GET /api/instructions/{name}/content
func (s *Server) handleGetSharedInstructionsContent(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	s.mu.RLock()
	defer s.mu.RUnlock()
	extra, ok := s.sharedExtra(name)
	if !ok {
		writeCodedError(w, http.StatusNotFound, "instructions_shared_not_found", "shared instruction file not found: "+name, map[string]string{"name": name})
		return
	}
	path := filepath.Join(s.extrasSourceDir(extra), extra.File)
	data, err := readLimited(path)
	if err != nil && !os.IsNotExist(err) {
		writeCodedError(w, http.StatusInternalServerError, "instructions_read_failed", err.Error(), map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"name": name, "path": path, "exists": err == nil, "content": string(data)})
}

// handlePutSharedInstructionsContent — PUT /api/instructions/{name}/content
func (s *Server) handlePutSharedInstructionsContent(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	name := r.PathValue("name")
	var body struct {
		Content string `json:"content"`
	}
	if err := decodeJSON(w, r, &body, maxInstructionsBytes+4096); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeCodedError(w, http.StatusBadRequest, "instructions_invalid_json", "invalid JSON body", map[string]string{})
		}
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, lockErr := s.lockRemoteExtraMutation()
	if lockErr != nil {
		writeCodedError(w, http.StatusConflict, "instructions_write_failed", lockErr.Error(), map[string]string{"detail": lockErr.Error()})
		return
	}
	defer unlock()
	extra, ok := s.sharedExtra(name)
	if !ok {
		writeCodedError(w, http.StatusNotFound, "instructions_shared_not_found", "shared instruction file not found: "+name, map[string]string{"name": name})
		return
	}
	path := filepath.Join(s.extrasSourceDir(extra), extra.File)
	if err := writeInstructionsFile(path, body.Content); err != nil {
		s.writeOpsLog("instructions-edit", "error", start, map[string]any{"name": name, "scope": "ui"}, err.Error())
		writeCodedError(w, http.StatusInternalServerError, "instructions_write_failed", err.Error(), map[string]string{"detail": err.Error()})
		return
	}
	// Links and import lines read the new content already; copies are rewritten.
	copies := s.syncSharedCopies(extra)
	status, msg := "ok", ""
	for _, c := range copies {
		if c.Error != "" {
			status, msg = "partial", c.Target+": "+c.Error
		}
	}
	s.writeOpsLog("instructions-edit", status, start, map[string]any{"name": name, "path": path, "copies": len(copies), "scope": "ui"}, msg)
	writeJSON(w, map[string]any{"success": true, "path": path, "copies": copies})
}

type sharedCopyResult struct {
	Target   string   `json:"target"`
	Warnings []string `json:"warnings,omitempty"`
	Error    string   `json:"error,omitempty"`
}

// syncSharedCopies rewrites the targets of a shared file that get it as a
// copy (copy mode, or a link mode copying because file links are
// unavailable), as a sync of that file would. A copy the user edited is kept
// as a drift backup first. Callers must hold s.mu.
func (s *Server) syncSharedCopies(extra config.ExtraConfig) []sharedCopyResult {
	names := map[string]string{}
	for _, t := range s.instructionTargets() {
		names[filepath.Clean(t.Path)] = t.Name
	}
	res := s.instructionsResolver()
	out := []sharedCopyResult{}
	for j := range extra.Targets {
		f := instructions.ExtraFile(extra, j, res)
		if f.Mode != "copy" {
			continue
		}
		r := sharedCopyResult{Target: names[filepath.Clean(f.Target)]}
		if r.Target == "" {
			r.Target = f.Target
		}
		result, err := syncpkg.SyncExtraFile(f, false, s.projectRoot)
		if err != nil {
			r.Error = err.Error()
		} else {
			// The copy fallback is how the target is set up, not news about this save.
			r.Warnings = slices.DeleteFunc(result.Warnings, func(w string) bool { return w == syncpkg.FileLinkFallbackWarning })
		}
		out = append(out, r)
	}
	return out
}

// assignTarget attaches exactly want to the named target. Callers must hold
// s.mu and save the config afterwards.
func (s *Server) assignTarget(name string, want []string, warnings ...*[]syncpkg.FileWarning) error {
	it, found, ok := s.targetInstructions(name)
	if !found {
		return fmt.Errorf("target not found: %s", name)
	}
	if !ok {
		return fmt.Errorf("%s has no global instruction file", name)
	}
	if _, linked := s.cfg.Targets[it.SameAs]; linked {
		return fmt.Errorf("%s reads the same file as %s; change %s instead", name, it.SameAs, it.SameAs)
	}
	extras, err := instructions.Assign(s.cfg.Extras, instructions.Target{Name: name, File: it.Path, Import: it.Import}, want, s.instructionsResolver(), warnings...)
	s.cfg.Extras = extras
	return err
}

// handleAssignSharedInstructions — POST /api/instructions/assign
// Sets which shared files each listed target uses; an empty list restores the
// targets' own files.
func (s *Server) handleAssignSharedInstructions(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var body struct {
		Targets []string `json:"targets"`
		Extras  []string `json:"extras"`
	}
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeCodedError(w, http.StatusBadRequest, "instructions_invalid_json", "invalid JSON body", map[string]string{})
		}
		return
	}
	if len(body.Targets) == 0 {
		writeCodedError(w, http.StatusBadRequest, "instructions_target_required", "at least one target is required", map[string]string{})
		return
	}
	if body.Extras == nil {
		body.Extras = []string{}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, lockErr := s.lockRemoteExtraMutation()
	if lockErr != nil {
		writeCodedError(w, http.StatusConflict, "instructions_write_failed", lockErr.Error(), map[string]string{"detail": lockErr.Error()})
		return
	}
	defer unlock()
	// Validate every target before attaching any of them, including connect-all.
	planned := s.cfg.Extras
	for _, name := range body.Targets {
		it, found, ok := s.targetInstructions(name)
		if !found || !ok {
			writeCodedError(w, http.StatusBadRequest, "instructions_no_file", "target has no instruction file: "+name, map[string]string{"target": name})
			return
		}
		if _, linked := s.cfg.Targets[it.SameAs]; linked {
			writeCodedError(w, http.StatusBadRequest, "instructions_same_file", fmt.Sprintf("%s reads the same file as %s; change %s instead", name, it.SameAs, it.SameAs), map[string]string{"target": name, "other": it.SameAs})
			return
		}
		for _, shared := range body.Extras {
			if extra, found := s.sharedExtra(shared); !found || !instructions.IsShared(extra) {
				writeCodedError(w, http.StatusNotFound, "instructions_shared_not_found", fmt.Sprintf("shared instruction file %q not found", shared), map[string]string{"name": shared})
				return
			}
		}
		var err error
		planned, err = instructions.PlanAssign(planned, instructions.Target{Name: name, File: it.Path, Import: it.Import}, body.Extras, s.instructionsResolver())
		if err != nil {
			if !writeExtraTargetConflict(w, err, name) {
				writeCodedError(w, http.StatusBadRequest, "instructions_assign_failed", err.Error(), map[string]string{"detail": err.Error()})
			}
			return
		}
	}
	warnings := []syncpkg.FileWarning{}
	errs := []string{}
	for _, name := range body.Targets {
		if err := s.assignTarget(name, body.Extras, &warnings); err != nil {
			errs = append(errs, err.Error())
		}
	}
	s.saveSharedAfterChange(w, start, "instructions-assign", map[string]any{"targets": body.Targets, "extras": body.Extras, "scope": "ui"}, errs, warnings)
}

// saveSharedAfterChange saves the config (files on disk already changed),
// logs, and answers with the per-target errors.
func (s *Server) saveSharedAfterChange(w http.ResponseWriter, start time.Time, cmd string, args map[string]any, errs []string, warnings ...[]syncpkg.FileWarning) {
	if err := s.saveAndReloadConfig(); err != nil {
		errs = append(errs, err.Error())
	}
	status, msg := "ok", ""
	if len(errs) > 0 {
		status, msg = "partial", fmt.Sprint(errs)
	}
	s.writeOpsLog(cmd, status, start, args, msg)
	if len(errs) > 0 {
		for _, ws := range warnings {
			for _, warning := range ws {
				if warning.Code == "target_directory" {
					writeCodedError(w, http.StatusConflict, "instructions_target_directory", strings.Join(errs, "; "), warning.Params)
					return
				}
			}
		}
		code := "instructions_assign_failed"
		if cmd == "instructions-restore" {
			code = "instructions_restore_failed"
		}
		writeCodedError(w, http.StatusInternalServerError, code, strings.Join(errs, "; "), map[string]string{"detail": strings.Join(errs, "; ")})
		return
	}

	outWarnings := []syncpkg.FileWarning{}
	for _, ws := range warnings {
		outWarnings = append(outWarnings, ws...)
	}
	writeJSON(w, map[string]any{"success": len(errs) == 0, "errors": errs, "warnings": outWarnings})
}

// handleRestoreSharedInstructions — POST /api/instructions/{name}/restore
// Detaches the shared file from one target and puts back what it replaced.
func (s *Server) handleRestoreSharedInstructions(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	name := r.PathValue("name")
	var body struct {
		Target string `json:"target"`
	}
	decodeErr := decodeJSON(w, r, &body, defaultJSONBodyLimit)
	if errors.Is(decodeErr, errBodyTooLarge) {
		return
	}
	if decodeErr != nil || body.Target == "" {
		writeCodedError(w, http.StatusBadRequest, "instructions_target_required", "target is required", map[string]string{})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, lockErr := s.lockRemoteExtraMutation()
	if lockErr != nil {
		writeCodedError(w, http.StatusConflict, "instructions_write_failed", lockErr.Error(), map[string]string{"detail": lockErr.Error()})
		return
	}
	defer unlock()
	it, _, ok := s.targetInstructions(body.Target)
	if !ok {
		writeCodedError(w, http.StatusBadRequest, "instructions_no_file", body.Target+" has no instruction file", map[string]string{"target": body.Target})
		return
	}
	var keep []string
	for _, a := range instructions.Assignments(instructions.Shared(s.cfg.Extras), it.Path, s.instructionsResolver()) {
		if a.Name != name {
			keep = append(keep, a.Name)
		}
	}
	var errs []string
	if err := s.assignTarget(body.Target, keep); err != nil {
		errs = append(errs, err.Error())
	}
	s.saveSharedAfterChange(w, start, "instructions-restore", map[string]any{"name": name, "target": body.Target, "scope": "ui"}, errs)
}

// handleResolveSharedInstructions — POST /api/instructions/{name}/resolve
// Settles a target whose linked file was replaced and edited: collect writes
// the target's content back to the shared file; reapply backs the target up
// and links it again.
func (s *Server) handleResolveSharedInstructions(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	name := r.PathValue("name")
	var body struct {
		Target string `json:"target"`
		Path   string `json:"path"` // a location (see sharedLocations) instead of a target
		Action string `json:"action"`
	}
	decodeErr := decodeJSON(w, r, &body, defaultJSONBodyLimit)
	if errors.Is(decodeErr, errBodyTooLarge) {
		return
	}
	if decodeErr != nil || (body.Target == "" && body.Path == "") {
		writeCodedError(w, http.StatusBadRequest, "instructions_target_required", "target is required", map[string]string{})
		return
	}
	if !slices.Contains([]string{"collect", "reapply"}, body.Action) {
		writeCodedError(w, http.StatusBadRequest, "instructions_invalid_action", "action must be collect or reapply", map[string]string{"action": body.Action})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, lockErr := s.lockRemoteExtraMutation()
	if lockErr != nil {
		writeCodedError(w, http.StatusConflict, "instructions_write_failed", lockErr.Error(), map[string]string{"detail": lockErr.Error()})
		return
	}
	defer unlock()
	res := s.instructionsResolver()
	var i, j int
	if body.Path != "" {
		var ok bool
		if i, j, ok = s.lookupLocation(w, name, body.Path); !ok {
			return
		}
	} else {
		it, _, ok := s.targetInstructions(body.Target)
		if !ok {
			writeCodedError(w, http.StatusBadRequest, "instructions_no_file", body.Target+" has no instruction file", map[string]string{"target": body.Target})
			return
		}
		i, j = instructions.Find(*s.sharedExtras(), name, it.Path, res)
		if j == -1 {
			writeCodedError(w, http.StatusNotFound, "instructions_not_attached", name+" is not attached to "+body.Target, map[string]string{"name": name, "target": body.Target})
			return
		}
	}
	f := instructions.ExtraFile((*s.sharedExtras())[i], j, res)
	var err error
	if body.Action == "collect" {
		err = syncpkg.CollectBackExtraFile(f, s.projectRoot)
	} else {
		err = syncpkg.ReapplyExtraFile(f, s.projectRoot)
	}
	args := map[string]any{"name": name, "target": body.Target, "path": body.Path, "action": body.Action, "scope": "ui"}
	if err != nil {
		s.writeOpsLog("instructions-resolve", "error", start, args, err.Error())
		writeCodedError(w, http.StatusInternalServerError, "instructions_resolve_failed", err.Error(), map[string]string{"detail": err.Error()})
		return
	}
	s.writeOpsLog("instructions-resolve", "ok", start, args, "")
	writeJSON(w, map[string]any{"success": true})
}
