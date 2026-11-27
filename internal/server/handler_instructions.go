package server

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/instructions"
	syncpkg "skillshare/internal/sync"
)

// maxInstructionsBytes bounds the instruction files the API reads or writes.
const maxInstructionsBytes = 1 << 20

// instructionsResolver locates extras files in the current mode. Callers must
// hold s.mu.
func (s *Server) instructionsResolver() instructions.Resolver {
	root := s.projectRoot
	return instructions.Resolver{
		SourceDir: s.extrasSourceDir,
		TargetDir: func(p string) string { return resolveExtrasTargetPath(root, p) },
	}
}

// targetInstructions returns the named target's instruction file with
// absolute paths. In global mode a rider of a configured target (a tool that
// reads its skills but keeps its own file, see config.InstructionRiders)
// resolves too, so its file can be edited and shared without configuring it.
// Callers must hold s.mu.
func (s *Server) targetInstructions(name string) (config.InstructionsTarget, bool, bool) {
	tc, found := s.cfg.Targets[name]
	if !found {
		if rider, ok := s.instructionRider(name); ok {
			return rider.InstructionsTarget, true, true
		}
		return config.InstructionsTarget{}, false, false
	}
	it, ok := config.TargetInstructions(name, tc, s.IsProjectMode())
	if !ok {
		return config.InstructionsTarget{}, true, false
	}
	return instructions.Resolve(it, s.projectRoot), true, true
}

// instructionRiders returns the riders of every configured target, sorted by
// name, each listed once. Empty in project mode. Callers must hold s.mu.
func (s *Server) instructionRiders() []config.InstructionsRider {
	if s.IsProjectMode() {
		return nil
	}
	names := make([]string, 0, len(s.cfg.Targets))
	for name := range s.cfg.Targets {
		names = append(names, name)
	}
	sort.Strings(names)
	seen := map[string]bool{}
	var out []config.InstructionsRider
	for _, name := range names {
		for _, r := range config.InstructionRiders(name, s.cfg.Targets) {
			if !seen[r.Name] {
				seen[r.Name] = true
				out = append(out, r)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// instructionRider finds a rider by name. Callers must hold s.mu.
func (s *Server) instructionRider(name string) (config.InstructionsRider, bool) {
	for _, r := range s.instructionRiders() {
		if r.Name == name {
			return r, true
		}
	}
	return config.InstructionsRider{}, false
}

type instructionsRiderFile struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
}

type instructionsFileResponse struct {
	Target      string                           `json:"target"`
	Project     bool                             `json:"project"`
	Supported   bool                             `json:"supported"`
	Custom      bool                             `json:"custom"`          // the file is the one set in the target's config
	Setup       *config.TargetInstructionsConfig `json:"setup,omitempty"` // that setting as written
	Path        string                           `json:"path,omitempty"`
	Exists      bool                             `json:"exists"`
	Content     string                           `json:"content"`
	Size        int64                            `json:"size"`
	LinkTo      string                           `json:"link_to,omitempty"`     // symlink destination of the file
	LinkShared  string                           `json:"link_shared,omitempty"` // shared instruction file that link points to
	Import      bool                             `json:"import"`
	MaxChars    int                              `json:"max_chars,omitempty"`
	ReadOrder   []instructions.Entry             `json:"read_order"`
	ImportLines []int                            `json:"import_lines"`
	Shared      []instructions.Assignment        `json:"shared"`
	Convert     []string                         `json:"convert"` // conversion methods on offer
	Blocked     map[string]string                `json:"convert_blocked,omitempty"`
	RiderOf     string                           `json:"rider_of,omitempty"`     // configured target whose skills this unconfigured tool reads
	Riders      []instructionsRiderFile          `json:"riders"`                 // tools reading this target's skills from their own file
	ReadBy      []string                         `json:"read_by"`                // other tools that read this very file
	DefaultPath string                           `json:"default_path,omitempty"` // built-in file, used when no location is set
}

// handleGetTargetInstructions — GET /api/targets/{name}/instructions
func (s *Server) handleGetTargetInstructions(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	s.mu.RLock()
	defer s.mu.RUnlock()

	it, found, ok := s.targetInstructions(name)
	if !found {
		writeCodedError(w, http.StatusNotFound, "instructions_target_not_found", "target not found: "+name, map[string]string{"target": name})
		return
	}
	resp := instructionsFileResponse{Target: name, Project: s.IsProjectMode(), ReadOrder: []instructions.Entry{}, ImportLines: []int{}, Shared: []instructions.Assignment{}, Convert: []string{}, Riders: []instructionsRiderFile{}, ReadBy: []string{}}
	tc, configured := s.cfg.Targets[name]
	if configured && !s.IsProjectMode() {
		for _, r := range config.InstructionRiders(name, s.cfg.Targets) {
			_, err := os.Stat(r.Path)
			resp.Riders = append(resp.Riders, instructionsRiderFile{Name: r.Name, Path: r.Path, Exists: err == nil})
		}
	} else if rider, isRider := s.instructionRider(name); !configured && isRider {
		resp.RiderOf = rider.Via
	}
	if !ok {
		writeJSON(w, resp)
		return
	}
	resp.Supported, resp.Path, resp.Import, resp.MaxChars = true, it.Path, it.Import, it.MaxChars
	resp.Setup = tc.Instructions
	resp.Custom = resp.Setup != nil
	if configured {
		plain := tc
		plain.Instructions = nil
		if def, ok := config.TargetInstructions(name, plain, s.IsProjectMode()); ok {
			resp.DefaultPath = instructions.Resolve(def, s.projectRoot).Path
		}
	}
	if !s.IsProjectMode() {
		if readers := config.InstructionReaders(name, it.Path); readers != nil {
			resp.ReadBy = readers
		}
	}
	resp.ReadOrder = instructions.ReadOrder(it, !s.IsProjectMode())
	if info, err := os.Stat(it.Path); err == nil && !info.IsDir() {
		data, err := readLimited(it.Path)
		if err != nil {
			writeCodedError(w, http.StatusInternalServerError, "instructions_read_failed", err.Error(), map[string]string{"detail": err.Error()})
			return
		}
		resp.Exists, resp.Size, resp.Content = true, info.Size(), string(data)
		resp.ImportLines = instructions.ImportLines(resp.Content)
	}
	if dest, err := os.Readlink(it.Path); err == nil {
		resp.LinkTo = dest
		resp.LinkShared = s.sharedLinkName(it.Path)
	}
	resp.Shared = instructions.Assignments(instructions.Shared(s.extrasConfig()), it.Path, s.instructionsResolver())
	resp.Convert, resp.Blocked = convertMethods(it, resp.Exists && instructions.HasMovableContent(resp.Content), s.IsProjectMode(), s.usesShared(it.Path))
	writeJSON(w, resp)
}

// sharedLinkName returns the shared instruction file (single-file extra) that
// the symlink at path points to, or "" when it is not one, such as a link the
// user made to their own dotfiles. Callers must hold s.mu.
func (s *Server) sharedLinkName(path string) string {
	return config.ExtraSourceAt(s.extrasConfig(), path, s.extrasSourceDir)
}

// usesShared reports whether a shared instruction file is attached to path,
// by config or by a managed import block left in the file. Callers must hold
// s.mu.
func (s *Server) usesShared(path string) bool {
	if len(instructions.Assignments(s.extrasConfig(), path, s.instructionsResolver())) > 0 {
		return true
	}
	data, err := readLimited(path)
	return err == nil && len(syncpkg.ManagedImportLines(string(data))) > 0
}

// convertMethods lists the ways a file can become an AGENTS.md, and why the
// others are not offered. shared (a shared instruction file is attached)
// blocks rename: sync would recreate the file and hide AGENTS.md again.
func convertMethods(it config.InstructionsTarget, exists, project, shared bool) ([]string, map[string]string) {
	methods, blocked := []string{}, map[string]string{}
	if !exists || filepath.Base(it.Path) == instructions.AgentsFile {
		return methods, nil
	}
	if it.Import {
		methods = append(methods, instructions.MethodImport)
	} else {
		blocked[instructions.MethodImport] = "no_import"
	}
	switch {
	case project && filepath.Base(it.Fallback) == instructions.AgentsFile && shared:
		blocked[instructions.MethodRename] = "shared"
	case project && filepath.Base(it.Fallback) == instructions.AgentsFile:
		methods = append(methods, instructions.MethodRename)
	default:
		blocked[instructions.MethodRename] = "global"
		if project {
			blocked[instructions.MethodRename] = "no_fallback"
		}
	}
	return append(methods, instructions.MethodCopy), blocked
}

func readLimited(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, maxInstructionsBytes))
}

// handlePutTargetInstructions — PUT /api/targets/{name}/instructions
func (s *Server) handlePutTargetInstructions(w http.ResponseWriter, r *http.Request) {
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
	unlock := s.lockInstructionWrite(w, s.targetInstructionMutation([]string{name}))
	if unlock == nil {
		return
	}
	defer unlock()
	it, found, ok := s.targetInstructions(name)
	if !found {
		writeCodedError(w, http.StatusNotFound, "instructions_target_not_found", "target not found: "+name, map[string]string{"target": name})
		return
	}
	if !ok {
		writeCodedError(w, http.StatusBadRequest, "instructions_no_file", name+" has no instruction file", map[string]string{"target": name})
		return
	}
	if shared := s.sharedLinkName(it.Path); shared != "" {
		writeCodedError(w, http.StatusConflict, "instructions_edit_shared", it.Path+" is a link to the shared instruction file "+shared+"; edit the shared file instead", map[string]string{"path": it.Path, "name": shared})
		return
	}
	if err := writeInstructionsFile(it.Path, body.Content); err != nil {
		s.writeOpsLog("instructions-edit", "error", start, map[string]any{"target": name, "scope": "ui"}, err.Error())
		writeCodedError(w, http.StatusInternalServerError, "instructions_write_failed", err.Error(), map[string]string{"detail": err.Error()})
		return
	}
	s.writeOpsLog("instructions-edit", "ok", start, map[string]any{"target": name, "path": it.Path, "scope": "ui"}, "")
	writeJSON(w, map[string]any{"success": true, "path": it.Path})
}

// writeInstructionsFile backs up an existing file, then writes content.
func writeInstructionsFile(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		if err := syncpkg.BackupFile(path, syncpkg.BackupReasonEdit); err != nil {
			return err
		}
	}
	return instructions.WriteFile(path, content)
}

// handleConvertTargetInstructions — POST /api/targets/{name}/instructions/convert
// Previews (apply=false) or applies a conversion of the target's file into an
// AGENTS.md. share_as (global mode, import) makes the AGENTS.md a new shared
// instruction file that the target imports; share_into appends the content to
// an existing shared file instead.
func (s *Server) handleConvertTargetInstructions(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	name := r.PathValue("name")
	var body struct {
		Method        string `json:"method"`
		KeepToolLines bool   `json:"keep_tool_lines"`
		ShareAs       string `json:"share_as,omitempty"`
		ShareInto     string `json:"share_into,omitempty"`
		Apply         bool   `json:"apply"`
	}
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeCodedError(w, http.StatusBadRequest, "instructions_invalid_json", "invalid JSON body", map[string]string{})
		}
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if body.Apply {
		unlock := s.lockInstructionWrite(w, s.targetInstructionMutation([]string{name}, body.ShareInto))
		if unlock == nil {
			return
		}
		defer unlock()
	}
	it, found, ok := s.targetInstructions(name)
	if !found {
		writeCodedError(w, http.StatusNotFound, "instructions_target_not_found", "target not found: "+name, map[string]string{"target": name})
		return
	}
	if !ok {
		writeCodedError(w, http.StatusBadRequest, "instructions_no_file", name+" has no instruction file", map[string]string{"target": name})
		return
	}
	methods, _ := convertMethods(it, true, s.IsProjectMode(), s.usesShared(it.Path))
	if !slices.Contains(methods, body.Method) {
		writeCodedError(w, http.StatusBadRequest, "instructions_method_unavailable", "method "+body.Method+" is not available for "+name, map[string]string{"method": body.Method, "target": name})
		return
	}
	opts := instructions.ConvertOptions{Method: body.Method, KeepToolLines: body.KeepToolLines, Project: s.IsProjectMode()}
	var shared config.ExtraConfig
	into := -1 // index of the existing shared file for share_into
	if body.ShareAs != "" || body.ShareInto != "" {
		if s.IsProjectMode() || body.Method != instructions.MethodImport {
			writeCodedError(w, http.StatusBadRequest, "instructions_sharing_requires_import", "sharing is only possible for import in global mode", map[string]string{})
			return
		}
		if body.ShareAs != "" && body.ShareInto != "" {
			writeCodedError(w, http.StatusBadRequest, "instructions_share_choice", "share_as and share_into cannot both be set", map[string]string{})
			return
		}
	}
	if body.ShareInto != "" {
		i, j := instructions.Find(s.cfg.Extras, body.ShareInto, it.Path, s.instructionsResolver())
		if i == -1 {
			writeCodedError(w, http.StatusNotFound, "instructions_shared_not_found", "shared instruction file not found: "+body.ShareInto, map[string]string{"name": body.ShareInto})
			return
		}
		if j != -1 {
			writeCodedError(w, http.StatusConflict, "instructions_already_attached", name+" already uses "+body.ShareInto, map[string]string{"target": name, "name": body.ShareInto})
			return
		}
		into, shared = i, s.cfg.Extras[i]
		opts.Dest, opts.Managed, opts.Append = filepath.Join(s.extrasSourceDir(shared), shared.File), true, true
	}
	if body.ShareAs != "" {
		if err := config.ValidateExtraName(body.ShareAs); err != nil {
			writeCodedError(w, http.StatusBadRequest, "instructions_invalid_name", err.Error(), map[string]string{"name": body.ShareAs})
			return
		}
		if err := config.ValidateExtraNameUnique(body.ShareAs, s.extrasConfig()); err != nil {
			writeCodedError(w, http.StatusConflict, "instructions_name_taken", err.Error(), map[string]string{"name": body.ShareAs})
			return
		}
		shared = config.ExtraConfig{Name: body.ShareAs, File: instructions.AgentsFile, Targets: []config.ExtraTargetConfig{{
			Path: filepath.Dir(it.Path), As: filepath.Base(it.Path), Mode: "import",
		}}}
		opts.Dest, opts.Managed = filepath.Join(s.extrasSourceDir(shared), instructions.AgentsFile), true
	}
	changes, err := instructions.PlanConvert(it.Path, opts)
	if err != nil {
		writeCodedError(w, http.StatusBadRequest, "instructions_convert_failed", err.Error(), map[string]string{"detail": err.Error()})
		return
	}
	if !body.Apply {
		writeJSON(w, map[string]any{"changes": changes})
		return
	}
	if !s.checkInstructionChanges(w, changes) {
		return
	}

	args := map[string]any{"target": name, "method": body.Method, "share_as": body.ShareAs, "share_into": body.ShareInto, "scope": "ui"}
	if err := instructions.Apply(changes); err != nil {
		s.writeOpsLog("instructions-convert", "error", start, args, err.Error())
		writeCodedError(w, http.StatusInternalServerError, "instructions_convert_failed", err.Error(), map[string]string{"detail": err.Error()})
		return
	}
	if body.ShareAs != "" || into != -1 {
		if err := s.saveConvertedShared(it.Path, shared, into); err != nil {
			s.writeOpsLog("instructions-convert", "error", start, args, err.Error())
			writeCodedError(w, http.StatusInternalServerError, "instructions_convert_failed", err.Error(), map[string]string{"detail": err.Error()})
			return
		}
	}
	s.writeOpsLog("instructions-convert", "ok", start, args, "")
	writeJSON(w, map[string]any{"success": true, "changes": changes})
}

func (s *Server) saveConvertedShared(path string, shared config.ExtraConfig, into int) error {
	if into != -1 {
		tc := config.ExtraTargetConfig{Path: filepath.Dir(path), Mode: "import"}
		if base := filepath.Base(path); base != shared.File {
			tc.As = base
		}
		s.cfg.Extras[into].Targets = append(s.cfg.Extras[into].Targets, tc)
	} else {
		s.cfg.Extras = append(s.cfg.Extras, shared)
	}
	return s.saveAndReloadConfig()
}
