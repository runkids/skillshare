package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/hooks"
)

func (s *Server) hooksService() *hooks.Service {
	return &hooks.Service{ConfigPath: s.configPath(), ProjectRoot: s.projectRoot, StateDir: config.StateDir(), ConfigDirs: hooks.ConfigDirsFromEnv()}
}

// requireLocalHooks applies the MCP DNS-rebinding guard: hooks write commands that Agents run.
func (s *Server) requireLocalHooks(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !mcpRequestAllowed(r, s.addr) {
			writeError(w, http.StatusForbidden, "Hooks settings are available only when the dashboard is opened by localhost or an IP address, not a domain name")
			return
		}
		next(w, r)
	}
}

type hooksRequest struct {
	Mutation hooks.Mutation `json:"mutation"`
	Revision string         `json:"revision"`
	Sync     bool           `json:"sync"`
	// Root, with sync and no mutation, applies only that hooks.projects root's changes.
	Root string `json:"root"`
}

type hooksRestoreRequest struct {
	BackupID string `json:"backupId"`
	Preview  bool   `json:"preview"`
	Revision string `json:"revision"`
}

func decodeHooksRequest(w http.ResponseWriter, r *http.Request, value any) bool {
	var extra bool
	err := decodeJSONWith(w, r, defaultJSONBodyLimit, func(d *json.Decoder) error {
		d.DisallowUnknownFields()
		if err := d.Decode(value); err != nil {
			return err
		}
		extra = d.Decode(new(any)) != io.EOF
		return nil
	})
	switch {
	case errors.Is(err, errBodyTooLarge):
		return false
	case err != nil:
		writeError(w, http.StatusBadRequest, "invalid hooks request")
		return false
	case extra:
		writeError(w, http.StatusBadRequest, "expected one hooks request")
		return false
	}
	return true
}

func (s *Server) handleHooksList(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	inventory, err := s.hooksService().List()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, inventory)
}

func (s *Server) handleHooksPreview(w http.ResponseWriter, r *http.Request) {
	var body hooksRequest
	if !decodeHooksRequest(w, r, &body) {
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, err := s.hooksService().PreviewMutation(body.Mutation)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, p)
}

// handleHooksRender shows one hook's native files without planning, saving or writing.
func (s *Server) handleHooksRender(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Mutation hooks.Mutation `json:"mutation"`
	}
	if !decodeHooksRequest(w, r, &body) {
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	files, err := s.hooksService().RenderNative(body.Mutation)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, map[string]any{"rendered": files})
}

func (s *Server) handleHooksConfigure(w http.ResponseWriter, r *http.Request) {
	var body hooksRequest
	if !decodeHooksRequest(w, r, &body) {
		return
	}
	if body.Revision == "" {
		writeError(w, http.StatusBadRequest, "preview the hooks changes before saving")
		return
	}
	if body.Root != "" && (!body.Sync || !reflect.DeepEqual(body.Mutation, hooks.Mutation{})) {
		writeError(w, http.StatusBadRequest, "a project sync takes no changes")
		return
	}
	if body.Root != "" && s.IsProjectMode() {
		writeError(w, http.StatusBadRequest, "hooks.projects roots are available only in global mode; omit root")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	start := time.Now()
	var result *hooks.Result
	var err error
	args := map[string]any{"scope": "ui", "sync": body.Sync}
	if body.Root != "" {
		args["project"] = body.Root
		result, err = s.hooksService().ApplyProject(body.Revision, body.Root)
	} else {
		result, err = s.hooksService().Mutate(body.Mutation, body.Revision, body.Sync)
	}
	if body.Mutation.Name != "" {
		args["name"] = body.Mutation.Name
	}
	s.writeOpsLog("hooks configure", opStatus(err), start, args, "")
	if errors.Is(err, hooks.ErrUnknownProject) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeHooksFailure(w, result, err)
		return
	}
	if err := s.reloadConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, "hooks saved, but config reload failed")
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleHooksImport(w http.ResponseWriter, r *http.Request) {
	var body hooks.ImportRequest
	if !decodeHooksRequest(w, r, &body) {
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	candidates, err := s.hooksService().Import(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if candidates == nil {
		candidates = []hooks.Candidate{}
	}
	writeJSON(w, map[string]any{"candidates": candidates})
}

func (s *Server) handleHooksRestore(w http.ResponseWriter, r *http.Request) {
	var body hooksRestoreRequest
	if !decodeHooksRequest(w, r, &body) {
		return
	}
	if body.BackupID == "" {
		writeError(w, http.StatusBadRequest, "backupId is required")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	service := s.hooksService()
	if body.Preview {
		p, err := service.PreviewRestore(body.BackupID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, p)
		return
	}
	if body.Revision == "" {
		writeError(w, http.StatusBadRequest, "preview before restoring")
		return
	}
	start := time.Now()
	result, err := service.Restore(body.BackupID, body.Revision)
	s.writeOpsLog("hooks restore", opStatus(err), start, map[string]any{"scope": "ui", "backup": body.BackupID}, "")
	if err != nil {
		writeHooksFailure(w, result, err)
		return
	}
	writeJSON(w, result)
}

func opStatus(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}

// writeHooksFailure reports stale previews, conflicts and partial writes as 409 with the
// result, so the dashboard can show what was already applied; other errors are 400.
func writeHooksFailure(w http.ResponseWriter, result *hooks.Result, err error) {
	partial := result != nil && len(result.Applied) > 0
	if !partial && !errors.Is(err, hooks.ErrStaleRevision) && !errors.Is(err, hooks.ErrConflict) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	message := err.Error()
	if partial {
		message += fmt.Sprintf("; already applied: %s; backup IDs: %s", strings.Join(result.Applied, ", "), strings.Join(result.BackupIDs, ", "))
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusConflict)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": message, "result": result})
}
