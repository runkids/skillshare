package server

import (
	"errors"
	"net/http"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/plugin"
)

type piExtensionsRequest struct {
	Changes  []plugin.PiExtensionChange `json:"changes"`
	Revision string                     `json:"revision,omitempty"`
}

// piExtensionsService resolves a Pi target, "pi", a Pi account or a project's Pi,
// to the plugin service and target name its settings belong to. It writes a 404
// for any other target. Callers must hold s.mu.
func (s *Server) piExtensionsService(w http.ResponseWriter, name string) (*plugin.Service, string, bool) {
	tc, found := s.cfg.Targets[name]
	service := s.pluginService()
	target := name
	switch {
	case !found:
	case tc.ProjectRoot() != "":
		// A project of the global config, named <project>@pi.
		if _, tool, _ := config.SplitProjectTarget(name); tool == "pi" {
			service.ProjectRoot, service.Accounts, target = tc.ProjectRoot(), nil, "pi"
		} else {
			found = false
		}
	case name == "pi":
	case !s.IsProjectMode() && tc.Agent == "pi" && tc.ConfigDir != "":
	default:
		found = false
	}
	if !found {
		writeCodedError(w, http.StatusNotFound, "pi_extensions_not_pi", "not a Pi target: "+name, map[string]string{"target": name})
		return nil, "", false
	}
	service.ExtrasSources = map[string]string{}
	for _, extra := range s.extrasConfig() {
		service.ExtrasSources[extra.Name] = s.extrasSourceDir(extra)
	}
	return service, target, true
}

func (s *Server) handlePiExtensions(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	service, target, ok := s.piExtensionsService(w, r.PathValue("name"))
	if !ok {
		return
	}
	view, err := service.PiExtensions(r.Context(), target)
	if err != nil {
		writePiExtensionsError(w, err)
		return
	}
	view.Target = r.PathValue("name")
	writeJSON(w, view)
}

func (s *Server) handlePiExtensionsPreview(w http.ResponseWriter, r *http.Request) {
	var body piExtensionsRequest
	if !decodePluginRequest(w, r, &body) {
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	service, target, ok := s.piExtensionsService(w, r.PathValue("name"))
	if !ok {
		return
	}
	plan, err := service.PreviewPiExtensions(r.Context(), target, body.Changes)
	if err != nil {
		writePiExtensionsError(w, err)
		return
	}
	writeJSON(w, plan)
}

func (s *Server) handlePiExtensionsApply(w http.ResponseWriter, r *http.Request) {
	var body piExtensionsRequest
	if !decodePluginRequest(w, r, &body) {
		return
	}
	if body.Revision == "" {
		writeCodedError(w, http.StatusBadRequest, "pi_extensions_preview_first", "preview before applying extension changes", nil)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	name := r.PathValue("name")
	service, target, ok := s.piExtensionsService(w, name)
	if !ok {
		return
	}
	start := time.Now()
	plan, err := service.ApplyPiExtensions(r.Context(), target, body.Changes, body.Revision)
	status, message := "ok", ""
	if err != nil {
		status, message = "error", err.Error()
	}
	s.writeOpsLog("pi-extensions", status, start, map[string]any{"scope": "ui", "target": name, "changes": len(body.Changes)}, message)
	if err != nil {
		writePiExtensionsError(w, err)
		return
	}
	writeJSON(w, plan)
}

// writePiExtensionsError answers with a code the dashboard translates.
func writePiExtensionsError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, plugin.ErrPiExtensionsStale):
		writeCodedError(w, http.StatusConflict, "pi_extensions_stale", err.Error(), nil)
	case errors.Is(err, plugin.ErrPiExtensionsBusy):
		writeCodedError(w, http.StatusConflict, "pi_extensions_busy", err.Error(), nil)
	case errors.Is(err, plugin.ErrPiExtensionsReadOnly):
		writeCodedError(w, http.StatusForbidden, "pi_extensions_read_only", err.Error(), nil)
	default:
		writeCodedError(w, http.StatusBadRequest, "pi_extensions_invalid", err.Error(), nil)
	}
}
