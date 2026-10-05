package server

import (
	"errors"
	"net/http"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/plugin"
)

// Callers hold s.mu; project targets always resolve from config metadata.
func (s *Server) ompExtensionsService(w http.ResponseWriter, name string) (*plugin.Service, string, bool) {
	tc, found := s.cfg.Targets[name]
	service := s.pluginService()
	target := name
	switch {
	case !found:
	case tc.ProjectRoot() != "":
		if _, tool, _ := config.SplitProjectTarget(name); tool == "omp" {
			service.ProjectRoot, service.Accounts, target = tc.ProjectRoot(), nil, "omp"
		} else {
			found = false
		}
	case name == "omp":
	case !s.IsProjectMode() && tc.Agent == "omp" && tc.ConfigDir != "":
	default:
		found = false
	}
	if !found {
		writeCodedError(w, http.StatusNotFound, "omp_extensions_not_omp", "not an OMP target: "+name, map[string]string{"target": name})
		return nil, "", false
	}
	return service, target, true
}

func (s *Server) handleOMPExtensions(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	name := r.PathValue("name")
	service, target, ok := s.ompExtensionsService(w, name)
	if !ok {
		return
	}
	view, err := service.OMPExtensions(r.Context(), target)
	if err != nil {
		writeCodedError(w, http.StatusBadRequest, "omp_extensions_invalid", err.Error(), nil)
		return
	}
	view.Target = name
	for i := range view.Rows {
		if view.Rows[i].HooksTarget != "" {
			view.Rows[i].HooksTarget = name
		}
	}
	writeJSON(w, view)
}

type ompExtensionsRequest struct {
	Changes []struct {
		Key     string `json:"key"`
		Enabled *bool  `json:"enabled"`
	} `json:"changes"`
	Revision string `json:"revision"`
}

func decodeOMPExtensionsRequest(w http.ResponseWriter, r *http.Request) ([]plugin.OMPExtensionChange, string, bool) {
	var body ompExtensionsRequest
	if !decodePluginRequest(w, r, &body) {
		return nil, "", false
	}
	if body.Revision == "" {
		writeCodedError(w, http.StatusBadRequest, "omp_extensions_preview_first", "refresh the inventory and preview before applying changes", nil)
		return nil, "", false
	}
	changes := make([]plugin.OMPExtensionChange, 0, len(body.Changes))
	for _, c := range body.Changes {
		if c.Enabled == nil {
			writeCodedError(w, http.StatusBadRequest, "omp_extensions_invalid", "each change requires enabled as a boolean", nil)
			return nil, "", false
		}
		changes = append(changes, plugin.OMPExtensionChange{Key: c.Key, Enabled: *c.Enabled})
	}
	return changes, body.Revision, true
}

func (s *Server) handleOMPExtensionsPreview(w http.ResponseWriter, r *http.Request) {
	changes, revision, ok := decodeOMPExtensionsRequest(w, r)
	if !ok {
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	service, target, ok := s.ompExtensionsService(w, r.PathValue("name"))
	if !ok {
		return
	}
	plan, err := service.PreviewOMPExtensions(r.Context(), target, changes, revision)
	if err != nil {
		writeOMPExtensionsError(w, err)
		return
	}
	writeJSON(w, plan)
}

func (s *Server) handleOMPExtensionsApply(w http.ResponseWriter, r *http.Request) {
	changes, revision, ok := decodeOMPExtensionsRequest(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	name := r.PathValue("name")
	service, target, ok := s.ompExtensionsService(w, name)
	if !ok {
		return
	}
	start := time.Now()
	plan, err := service.ApplyOMPExtensions(r.Context(), target, changes, revision)
	status, message := "ok", ""
	if err != nil {
		status, message = "error", err.Error()
	}
	s.writeOpsLog("omp-extensions", status, start, map[string]any{"scope": "ui", "target": name, "changes": len(changes)}, message)
	if err != nil {
		writeOMPExtensionsError(w, err)
		return
	}
	writeJSON(w, plan)
}

func writeOMPExtensionsError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, plugin.ErrOMPExtensionsStale):
		writeCodedError(w, http.StatusConflict, "omp_extensions_stale", err.Error(), nil)
	case errors.Is(err, plugin.ErrOMPExtensionsBusy):
		writeCodedError(w, http.StatusConflict, "omp_extensions_busy", err.Error(), nil)
	case errors.Is(err, plugin.ErrOMPExtensionsReadOnly):
		writeCodedError(w, http.StatusForbidden, "omp_extensions_read_only", err.Error(), nil)
	default:
		writeCodedError(w, http.StatusBadRequest, "omp_extensions_invalid", err.Error(), nil)
	}
}
