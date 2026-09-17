package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"time"

	"gopkg.in/yaml.v3"
	"skillshare/internal/config"
)

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	// Best-effort refresh so UI reflects external config edits without restart.
	// Keep endpoint usable even when config is temporarily invalid.
	s.mu.Lock()
	_ = s.reloadConfig()
	cfgObj := any(s.cfg)
	if s.IsProjectMode() {
		cfgObj = s.projectCfg
	}
	s.mu.Unlock()

	raw, err := os.ReadFile(s.configPath())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read config: "+err.Error())
		return
	}

	writeJSON(w, map[string]any{
		"config": cfgObj,
		"raw":    string(raw),
	})
}

var syncModes = []string{"merge", "copy", "symlink"}

// handlePatchConfig — PATCH /api/config
// Sets single settings the Settings page owns, so the UI never rewrites the whole YAML.
// Both fields live in the global config only; project mode has no default mode or log limit.
func (s *Server) handlePatchConfig(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var body struct {
		Mode          *string `json:"mode"`
		LogMaxEntries *int    `json:"logMaxEntries"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Mode == nil && body.LogMaxEntries == nil {
		writeError(w, http.StatusBadRequest, "mode or logMaxEntries is required")
		return
	}
	if body.Mode != nil && !slices.Contains(syncModes, *body.Mode) {
		writeError(w, http.StatusBadRequest, "invalid mode: "+*body.Mode)
		return
	}
	if body.LogMaxEntries != nil && *body.LogMaxEntries < 0 {
		writeError(w, http.StatusBadRequest, "logMaxEntries cannot be negative")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.IsProjectMode() {
		writeError(w, http.StatusBadRequest, "these settings are global only")
		return
	}
	// /api/config skips the auto-reload middleware, so pick up outside edits before writing.
	_ = s.reloadConfig()
	args := map[string]any{"scope": "ui"}
	if body.Mode != nil {
		s.cfg.Mode = *body.Mode
		args["mode"] = *body.Mode
	}
	if body.LogMaxEntries != nil {
		s.cfg.Log.MaxEntries = body.LogMaxEntries
		args["log_max_entries"] = *body.LogMaxEntries
	}
	if err := s.saveConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save config: "+err.Error())
		return
	}

	s.writeOpsLog("config-patch", "ok", start, args, "")
	writeJSON(w, map[string]any{"mode": s.cfg.Mode, "logMaxEntries": s.cfg.Log.MaxEntries})
}

func (s *Server) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	var body struct {
		Raw string `json:"raw"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// Validate YAML syntax + semantic validation before saving
	var warnings []string
	if s.IsProjectMode() {
		var testCfg config.ProjectConfig
		if err := yaml.Unmarshal([]byte(body.Raw), &testCfg); err != nil {
			writeError(w, http.StatusBadRequest, "invalid YAML: "+err.Error())
			return
		}
		w2, validErr := config.ValidateProjectConfig(&testCfg, s.projectRoot)
		if validErr == nil {
			validErr = config.ValidateMCP(testCfg.MCP, testCfg.Sources.MCP)
		}
		if validErr != nil {
			writeError(w, http.StatusBadRequest, validErr.Error())
			return
		}
		warnings = w2
	} else {
		var testCfg config.Config
		if err := yaml.Unmarshal([]byte(body.Raw), &testCfg); err != nil {
			writeError(w, http.StatusBadRequest, "invalid YAML: "+err.Error())
			return
		}
		w2, validErr := config.ValidateConfig(&testCfg)
		if validErr == nil {
			validErr = config.ValidateMCP(testCfg.MCP, testCfg.Sources.MCP)
		}
		if validErr != nil {
			writeError(w, http.StatusBadRequest, validErr.Error())
			return
		}
		warnings = w2
	}

	if err := os.WriteFile(s.configPath(), []byte(body.Raw), 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write config: "+err.Error())
		return
	}

	if err := s.reloadConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, "config saved but failed to reload: "+err.Error())
		return
	}

	s.writeOpsLog("config", "ok", start, map[string]any{
		"scope": "ui",
	}, "")

	writeJSON(w, map[string]any{
		"success":  true,
		"warnings": warnings,
	})
}

func (s *Server) handleAvailableTargets(w http.ResponseWriter, r *http.Request) {
	// Snapshot config under RLock, then release before I/O.
	s.mu.RLock()
	projectRoot := s.projectRoot
	targets := s.cloneTargets()
	s.mu.RUnlock()

	isProjectMode := projectRoot != ""

	var defaults map[string]config.TargetConfig
	if isProjectMode {
		defaults = config.ProjectTargets()
	} else {
		defaults = config.DefaultTargets()
	}

	type availTarget struct {
		Name      string `json:"name"`
		Path      string `json:"path"`
		AgentPath string `json:"agentPath,omitempty"`
		Installed bool   `json:"installed"`
		Detected  bool   `json:"detected"`
	}

	var agentDefaults map[string]config.TargetConfig
	if isProjectMode {
		agentDefaults = config.ProjectAgentTargets()
	} else {
		agentDefaults = config.DefaultAgentTargets()
	}

	items := make([]availTarget, 0, len(defaults))
	for name, tc := range defaults {
		_, installed := targets[name]
		// Check if the tool's config directory exists (parent of skills path)
		detected := false
		if !installed {
			parentDir := filepath.Dir(tc.Path)
			if _, err := os.Stat(parentDir); err == nil {
				detected = true
			}
		}
		item := availTarget{
			Name:      name,
			Path:      tc.Path,
			Installed: installed,
			Detected:  detected,
		}
		if agentTC, ok := agentDefaults[name]; ok {
			item.AgentPath = agentTC.Path
		}
		items = append(items, item)
	}

	writeJSON(w, map[string]any{"targets": items})
}
