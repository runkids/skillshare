package server

import (
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/instructions"
	syncpkg "skillshare/internal/sync"
)

// sharedInstructionsModes are the modes a shared instruction file can reach a
// target with. import needs a target that follows @path lines; prepend and
// append write the content in a managed block for targets that do not.
var sharedInstructionsModes = []string{"import", "prepend", "append", "symlink", "copy"}

// checkSharedMode answers 400 and returns false when a shared file cannot
// reach a target with mode here.
func checkSharedMode(w http.ResponseWriter, mode string) bool {
	if !slices.Contains(sharedInstructionsModes, mode) {
		writeCodedError(w, http.StatusBadRequest, "instructions_invalid_mode", "mode must be import, prepend, append, symlink, or copy", map[string]string{"mode": mode})
		return false
	}
	if mode == "symlink" && !syncpkg.CanCreateFileLink() {
		writeCodedError(w, http.StatusBadRequest, "instructions_file_links_unavailable", "file links need Windows Developer Mode; use copy", map[string]string{})
		return false
	}
	return true
}

// handlePutSharedInstructionsMode — PUT /api/instructions/{name}/targets/{target}/mode
// Changes how one attached target gets the shared file and syncs it again. The
// restore point recorded when the target was first attached stays.
// The success response always includes structured warnings from SyncExtraFile.
func (s *Server) handlePutSharedInstructionsMode(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	name, target := r.PathValue("name"), r.PathValue("target")
	var body struct {
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
	unlock, lockErr := s.lockRemoteExtraMutation()
	if lockErr != nil {
		writeCodedError(w, http.StatusConflict, "instructions_write_failed", lockErr.Error(), map[string]string{"detail": lockErr.Error()})
		return
	}
	defer unlock()
	it, found, ok := s.targetInstructions(target)
	if !found {
		writeCodedError(w, http.StatusNotFound, "instructions_target_not_found", "target not found: "+target, map[string]string{"target": target})
		return
	}
	if !ok {
		writeCodedError(w, http.StatusBadRequest, "instructions_no_file", target+" has no instruction file", map[string]string{"target": target})
		return
	}
	if err := config.ValidateImportMode(body.Mode, target, it.Import); err != nil {
		writeCodedError(w, http.StatusBadRequest, "instructions_import_unavailable", err.Error(), map[string]string{"target": target})
		return
	}
	res := s.instructionsResolver()
	i, j := instructions.Find(s.cfg.Extras, name, it.Path, res)
	if i == -1 {
		writeCodedError(w, http.StatusNotFound, "instructions_shared_not_found", "shared instruction file not found: "+name, map[string]string{"name": name})
		return
	}
	if j == -1 {
		writeCodedError(w, http.StatusNotFound, "instructions_not_attached", name+" is not attached to "+target, map[string]string{"name": name, "target": target})
		return
	}

	s.setSharedTargetMode(w, start, i, j, body.Mode, target, it.Path, "instructions_target_directory",
		map[string]any{"name": name, "target": target, "mode": body.Mode, "scope": "ui"})
}

// setSharedTargetMode switches the j-th target of the i-th extra to mode,
// syncs it again and saves the config; a failure leaves the mode as it was.
// label names the target in a conflict; dirCode is the error code when a
// directory sits at file. The success response lists the sync's structured
// warnings. Callers must hold s.mu.
func (s *Server) setSharedTargetMode(w http.ResponseWriter, start time.Time, i, j int, mode, label, file, dirCode string, args map[string]any) {
	res := s.instructionsResolver()
	extras := *s.sharedExtras()
	tc := &extras[i].Targets[j]
	prev := tc.Mode
	tc.Mode = mode
	validationErr := config.ValidateExtraConnections(extras, res.SourceDir, res.TargetDir, extras[i].Name)
	tc.Mode = prev
	if validationErr != nil {
		if !writeExtraTargetConflict(w, validationErr, label) {
			writeCodedError(w, http.StatusBadRequest, "instructions_mode_failed", validationErr.Error(), map[string]string{"detail": validationErr.Error()})
		}
		return
	}
	if tc.Mode == mode {
		writeJSON(w, map[string]any{"success": true, "warnings": []syncpkg.FileWarning{}})
		return
	}
	args["from"] = tc.Mode
	fail := func(err error) {
		s.writeOpsLog("instructions-mode", "error", start, args, err.Error())
		writeCodedError(w, http.StatusInternalServerError, "instructions_mode_failed", err.Error(), map[string]string{"detail": err.Error()})
	}
	tc.Mode = mode
	if err := config.ValidateExtraConfig(extras[i]); err != nil {
		tc.Mode = prev
		writeCodedError(w, http.StatusBadRequest, "instructions_mode_failed", err.Error(), map[string]string{"detail": err.Error()})
		return
	}
	result, err := syncpkg.SyncExtraFile(instructions.ExtraFile(extras[i], j, res), false, s.projectRoot)
	if err != nil {
		tc.Mode = prev
		fail(err)
		return
	}
	if result.Skipped > 0 {
		tc.Mode = prev
		diagnostic := strings.Join(result.Warnings, "; ")
		s.writeOpsLog("instructions-mode", "error", start, args, diagnostic)
		writeCodedError(w, http.StatusConflict, dirCode, diagnostic, map[string]string{"path": file})
		return
	}
	if err := s.saveAndReloadConfig(); err != nil {
		fail(err)
		return
	}
	s.writeOpsLog("instructions-mode", "ok", start, args, "")
	warnings := append([]syncpkg.FileWarning{}, result.FileWarnings...)
	writeJSON(w, map[string]any{"success": true, "warnings": warnings})
}

// writeExtraTargetConflict preserves a structured conflict for UI callers.
func writeExtraTargetConflict(w http.ResponseWriter, err error, target string) bool {
	var conflict *config.ExtraTargetConflict
	if !errors.As(err, &conflict) {
		return false
	}
	writeCodedError(w, http.StatusConflict, "instructions_target_held", err.Error(), map[string]string{"name": conflict.Name, "target": target})
	return true
}

type sharedRestorePreview struct {
	Kind       string     `json:"kind"`
	Path       string     `json:"path"`
	Content    string     `json:"content"`           // file after restore (kind content)
	Current    string     `json:"current"`           // file now
	LinkTo     string     `json:"link_to,omitempty"` // kind link
	RecordedAt *time.Time `json:"recorded_at,omitempty"`
	// Drift: edits made after attaching are backed up, not restored.
	Drift bool `json:"drift"`
}

// handleSharedInstructionsRestorePreview — GET /api/instructions/{name}/restore-preview?target=X
// Shows what restoring one target puts back, read from the record made when
// the target was attached (see sync.RestoreExtraTarget).
func (s *Server) handleSharedInstructionsRestorePreview(w http.ResponseWriter, r *http.Request) {
	name, target := r.PathValue("name"), r.URL.Query().Get("target")
	if target == "" {
		writeCodedError(w, http.StatusBadRequest, "instructions_target_required", "target is required", map[string]string{})
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	it, found, ok := s.targetInstructions(target)
	if !found {
		writeCodedError(w, http.StatusNotFound, "instructions_target_not_found", "target not found: "+target, map[string]string{"target": target})
		return
	}
	if !ok {
		writeCodedError(w, http.StatusBadRequest, "instructions_no_file", target+" has no instruction file", map[string]string{"target": target})
		return
	}
	res := s.instructionsResolver()
	i, j := instructions.Find(s.cfg.Extras, name, it.Path, res)
	if j == -1 {
		writeCodedError(w, http.StatusNotFound, "instructions_not_attached", name+" is not attached to "+target, map[string]string{"name": name, "target": target})
		return
	}
	f := instructions.ExtraFile(s.cfg.Extras[i], j, res)
	writeJSON(w, restorePreview(f))
}

func restorePreview(f syncpkg.ExtraFile) sharedRestorePreview {
	current, _ := readLimited(f.Target)
	sp := syncpkg.PreviewRestoreExtraTarget(f)
	p := sharedRestorePreview{Kind: sp.Kind, Path: f.Target, Content: sp.Content, Current: string(current), LinkTo: sp.LinkTo, Drift: sp.Drift}
	if !sp.RecordedAt.IsZero() {
		p.RecordedAt = &sp.RecordedAt
	}
	return p
}
