package server

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/plugin"
)

func (s *Server) pluginService() *plugin.Service {
	return &plugin.Service{ConfigPath: s.configPath(), ProjectRoot: s.projectRoot, StateDir: config.StateDir()}
}

func (s *Server) requireLocalPlugin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !mcpRequestAllowed(r, s.addr) {
			writeError(w, http.StatusForbidden, "Plugin management is available only from the local dashboard")
			return
		}
		// Native downloads can exceed the dashboard's normal 30-second deadline.
		// Limit this extension to the local plugin endpoints.
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(5 * time.Minute))
		next(w, r)
	}
}

type pluginRequest struct {
	Request  plugin.Request `json:"request"`
	Revision string         `json:"revision,omitempty"`
}

func decodePluginRequest(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(new(any)) != io.EOF {
		writeError(w, 400, "invalid plugin request")
		return false
	}
	return true
}

func (s *Server) handlePluginList(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// hosts=false skips the Agent CLIs: the page draws the plugin list from this answer
	// and fills in the Agents when the full one arrives.
	var inventory *plugin.Inventory
	var err error
	if r.URL.Query().Get("hosts") == "false" {
		inventory, err = s.pluginService().Packages()
	} else {
		inventory, err = s.pluginService().Inventory(r.Context())
	}
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, inventory)
}

func (s *Server) handlePluginDiscover(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Source    string `json:"source"`
		SourceRef string `json:"sourceRef,omitempty"`
		Entry     string `json:"entry,omitempty"`
	}
	if !decodePluginRequest(w, r, &body) {
		return
	}
	result, err := plugin.DiscoverOptions(r.Context(), body.Source, body.SourceRef, body.Entry)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, result)
}

func (s *Server) handlePluginPreview(w http.ResponseWriter, r *http.Request) {
	var body pluginRequest
	if !decodePluginRequest(w, r, &body) {
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, err := s.pluginService().Preview(r.Context(), body.Request)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, p)
}

func (s *Server) handlePluginApply(w http.ResponseWriter, r *http.Request) {
	var body pluginRequest
	if !decodePluginRequest(w, r, &body) {
		return
	}
	if body.Revision == "" {
		writeError(w, 400, "preview before applying plugin changes")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	start := time.Now()
	result, err := s.pluginService().Apply(r.Context(), body.Request, body.Revision)
	status := "ok"
	message := ""
	if err != nil {
		status = "error"
		message = err.Error()
	}
	s.writeOpsLog("plugin "+body.Request.Action, status, start, map[string]any{"scope": "ui"}, "")
	if reloadErr := s.reloadConfig(); reloadErr != nil && err == nil {
		message = "Plugin operation completed, but configuration reload failed"
	}
	// Preserve all per-target outcomes even when only some targets succeed.
	writeJSON(w, map[string]any{"result": result, "failure": message})
}

// handlePluginFiles lists the reviewed snapshot of a plugin; an imported one has none and lists nothing.
func (s *Server) handlePluginFiles(w http.ResponseWriter, r *http.Request) {
	files, err := s.pluginService().Files(r.PathValue("name"))
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, map[string]any{"files": files})
}

func (s *Server) handlePluginFile(w http.ResponseWriter, r *http.Request) {
	data, err := s.pluginService().ReadFile(r.PathValue("name"), r.PathValue("filepath"))
	if errors.Is(err, fs.ErrNotExist) {
		writeError(w, http.StatusNotFound, "file not found: "+r.PathValue("filepath"))
		return
	}
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, map[string]any{"content": string(data)})
}
