package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"skillshare/internal/config"
)

type hubConfigResponse struct {
	Hubs    []hubEntryJSON `json:"hubs"`
	Default string         `json:"default"`
}

type hubEntryJSON struct {
	Label   string `json:"label"`
	URL     string `json:"url"`
	BuiltIn bool   `json:"builtIn,omitempty"`
}

func (s *Server) handleGetHubSaved(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	hub := s.hubConfig()
	resp := hubConfigResponse{
		Default: hub.Default,
		Hubs:    make([]hubEntryJSON, 0, len(hub.Hubs)),
	}
	for _, h := range hub.Hubs {
		resp.Hubs = append(resp.Hubs, hubEntryJSON{
			Label:   h.Label,
			URL:     h.URL,
			BuiltIn: h.BuiltIn,
		})
	}
	writeJSON(w, resp)
}

func (s *Server) handlePutHubSaved(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	start := time.Now()

	var req hubConfigResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	hubCfg := config.HubConfig{
		Default: req.Default,
		Hubs:    make([]config.HubEntry, 0, len(req.Hubs)),
	}
	for _, h := range req.Hubs {
		hubCfg.Hubs = append(hubCfg.Hubs, config.HubEntry{
			Label:   h.Label,
			URL:     h.URL,
			BuiltIn: h.BuiltIn,
		})
	}

	s.setHubConfig(hubCfg)
	if err := s.saveAndReloadConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeOpsLog("hub-save", "ok", start, map[string]any{"count": len(hubCfg.Hubs)}, "")
	writeJSON(w, map[string]bool{"success": true})
}

func (s *Server) handlePostHubSaved(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	start := time.Now()

	var req struct {
		Label string `json:"label"`
		URL   string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	hubCfg := s.hubConfig()
	if err := hubCfg.AddHub(config.HubEntry{Label: req.Label, URL: req.URL}); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	s.setHubConfig(*hubCfg)
	if err := s.saveAndReloadConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeOpsLog("hub-add", "ok", start, map[string]any{"label": req.Label}, "")
	writeJSON(w, map[string]bool{"success": true})
}

func (s *Server) handleDeleteHubSaved(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	start := time.Now()

	label := strings.TrimSpace(r.PathValue("label"))
	if label == "" {
		writeError(w, http.StatusBadRequest, "label is required")
		return
	}

	hubCfg := s.hubConfig()
	if err := hubCfg.RemoveHub(label); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	s.setHubConfig(*hubCfg)
	if err := s.saveAndReloadConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeOpsLog("hub-remove", "ok", start, map[string]any{"label": label}, "")
	writeJSON(w, map[string]bool{"success": true})
}

// hubConfig returns a copy of the current hub config.
func (s *Server) hubConfig() *config.HubConfig {
	if s.IsProjectMode() {
		h := s.projectCfg.Hub
		return &h
	}
	h := s.cfg.Hub
	return &h
}

// setHubConfig sets the hub config on the appropriate config object.
func (s *Server) setHubConfig(h config.HubConfig) {
	if s.IsProjectMode() {
		s.projectCfg.Hub = h
	} else {
		s.cfg.Hub = h
	}
}
