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
	service := &plugin.Service{ConfigPath: s.configPath(), ProjectRoot: s.projectRoot, StateDir: config.StateDir()}
	if s.IsProjectMode() {
		service.GlobalConfigPath = config.ConfigPath()
	} else {
		service.Accounts = pluginAccounts(s.cfg)
	}
	return service
}

// pluginAccounts are the configured targets that are another config directory of an
// Agent, such as a second Claude account. They are plugin targets of their own.
func pluginAccounts(cfg *config.Config) map[string]plugin.Account {
	accounts := map[string]plugin.Account{}
	for name, target := range cfg.Targets {
		if target.Agent != "" && target.ConfigDir != "" {
			accounts[name] = plugin.Account{Agent: target.Agent, Dir: target.ConfigDir, CLI: target.CLI}
		}
	}
	return accounts
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
	err := decodeJSONWith(w, r, defaultJSONBodyLimit, func(d *json.Decoder) error {
		d.DisallowUnknownFields()
		if err := d.Decode(v); err != nil {
			return err
		}
		if err := d.Decode(new(any)); err != io.EOF {
			return errors.New("expected a single JSON document")
		}
		return nil
	})
	if err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, 400, "invalid plugin request")
		}
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
	// Name is set for a plugin already managed here, whose snapshot can answer instead of the network.
	var body struct {
		Name      string `json:"name,omitempty"`
		Source    string `json:"source"`
		SourceRef string `json:"sourceRef,omitempty"`
		Entry     string `json:"entry,omitempty"`
	}
	if !decodePluginRequest(w, r, &body) {
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result, err := s.pluginService().DiscoverManaged(r.Context(), body.Name, body.Source, body.SourceRef, body.Entry)
	if err != nil {
		// A fixed failure carries its key as the error code, so the dashboard can translate it.
		if key, args := plugin.ErrorKey(err); key != "" {
			writeCodedError(w, 400, key, err.Error(), args)
			return
		}
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
	s.mu.RLock()
	defer s.mu.RUnlock()
	files, err := s.pluginService().Files(r.PathValue("name"))
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, map[string]any{"files": files})
}

func (s *Server) handlePluginFile(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
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
