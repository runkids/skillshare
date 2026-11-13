package server

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/instructions"
)

// errInstructionsInUse is the error code for moving or removing a target's
// instruction file while shared instruction files are attached to it.
const errInstructionsInUse = "instructions_in_use"

// setTargetInstructionsConfig sets (ic != nil) or clears the user-set
// instruction file of a target in the config of the current mode. Callers
// must hold s.mu and save the config afterwards.
func (s *Server) setTargetInstructionsConfig(name string, ic *config.TargetInstructionsConfig) {
	tc := s.cfg.Targets[name]
	tc.Instructions = ic
	s.cfg.Targets[name] = tc
	if !s.IsProjectMode() {
		return
	}
	for i := range s.projectCfg.Targets {
		if s.projectCfg.Targets[i].Name == name {
			s.projectCfg.Targets[i].Instructions = ic
			return
		}
	}
}

// attachedShared returns the shared instruction files attached to the
// target's current instruction file. Callers must hold s.mu.
func (s *Server) attachedShared(name string) []instructions.Assignment {
	it, _, ok := s.targetInstructions(name)
	if !ok {
		return nil
	}
	return instructions.Assignments(s.extrasConfig(), it.Path, s.instructionsResolver())
}

func writeInstructionsInUse(w http.ResponseWriter, name string) {
	writeCodedError(w, http.StatusConflict, errInstructionsInUse,
		name+" uses shared instruction files; switch it back to its own file first",
		map[string]string{"target": name})
}

// handlePutTargetInstructionsSetup — PUT /api/targets/{name}/instructions/setup
// Tells skillshare which instruction file the target reads.
func (s *Server) handlePutTargetInstructionsSetup(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	name := r.PathValue("name")
	var body struct {
		Path   string `json:"path"`
		Import bool   `json:"import"`
	}
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeCodedError(w, http.StatusBadRequest, "instructions_invalid_json", "invalid JSON body", map[string]string{})
		}
		return
	}
	ic := &config.TargetInstructionsConfig{Path: strings.TrimSpace(body.Path), Import: body.Import}
	if err := config.ValidateTargetInstructions(ic, s.IsProjectMode()); err != nil {
		code := "instructions_path_absolute"
		switch {
		case ic.Path == "":
			code = "instructions_path_empty"
		case strings.HasSuffix(ic.Path, "/") || strings.HasSuffix(ic.Path, `\`):
			code = "instructions_path_directory"
		case s.IsProjectMode():
			code = "instructions_path_relative"
		}
		writeCodedError(w, http.StatusBadRequest, code, err.Error(), map[string]string{"path": ic.Path})
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
	tc, found := s.cfg.Targets[name]
	if !found {
		writeCodedError(w, http.StatusNotFound, "instructions_target_not_found", "target not found: "+name, map[string]string{"target": name})
		return
	}
	if tc.ProjectRoot() != "" {
		writeCodedError(w, http.StatusBadRequest, "instructions_project_target", name+" belongs to a project; edit the project instead", map[string]string{"target": name})
		return
	}
	// Shared files are attached to the current path; moving it would strand them.
	old, _, hadFile := s.targetInstructions(name)
	shared := s.attachedShared(name)
	tc.Instructions = ic
	next, _ := config.TargetInstructions(name, tc, s.IsProjectMode())
	if info, err := os.Stat(instructions.Resolve(next, s.projectRoot).Path); err == nil && info.IsDir() {
		writeCodedError(w, http.StatusBadRequest, "instructions_path_directory", fmt.Sprintf("instructions.path %q must name a file, not a directory", ic.Path), map[string]string{"path": ic.Path})
		return
	}
	if hadFile && len(shared) > 0 && filepath.Clean(old.Path) != filepath.Clean(instructions.Resolve(next, s.projectRoot).Path) {
		writeInstructionsInUse(w, name)
		return
	}

	s.setTargetInstructionsConfig(name, ic)
	args := map[string]any{"target": name, "path": ic.Path, "import": ic.Import, "scope": "ui"}
	if err := s.saveAndReloadConfig(); err != nil {
		s.writeOpsLog("instructions-setup", "error", start, args, err.Error())
		writeCodedError(w, http.StatusInternalServerError, "instructions_save_failed", err.Error(), map[string]string{"detail": err.Error()})
		return
	}
	s.writeOpsLog("instructions-setup", "ok", start, args, "")
	writeJSON(w, map[string]any{"success": true})
}

// handleDeleteTargetInstructionsSetup — DELETE /api/targets/{name}/instructions/setup
// Forgets the user-set instruction file; the built-in one, if any, applies again.
func (s *Server) handleDeleteTargetInstructionsSetup(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	name := r.PathValue("name")
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, lockErr := s.lockRemoteExtraMutation()
	if lockErr != nil {
		writeCodedError(w, http.StatusConflict, "instructions_write_failed", lockErr.Error(), map[string]string{"detail": lockErr.Error()})
		return
	}
	defer unlock()
	tc, found := s.cfg.Targets[name]
	if !found {
		writeCodedError(w, http.StatusNotFound, "instructions_target_not_found", "target not found: "+name, map[string]string{"target": name})
		return
	}
	if tc.Instructions == nil {
		writeJSON(w, map[string]any{"success": true})
		return
	}
	if len(s.attachedShared(name)) > 0 {
		writeInstructionsInUse(w, name)
		return
	}

	s.setTargetInstructionsConfig(name, nil)
	args := map[string]any{"target": name, "scope": "ui"}
	if err := s.saveAndReloadConfig(); err != nil {
		s.writeOpsLog("instructions-setup-remove", "error", start, args, err.Error())
		writeCodedError(w, http.StatusInternalServerError, "instructions_save_failed", err.Error(), map[string]string{"detail": err.Error()})
		return
	}
	s.writeOpsLog("instructions-setup-remove", "ok", start, args, "")
	writeJSON(w, map[string]any{"success": true})
}
