package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
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
		Installed bool   `json:"installed"`
		Detected  bool   `json:"detected"`
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
		items = append(items, availTarget{
			Name:      name,
			Path:      tc.Path,
			Installed: installed,
			Detected:  detected,
		})
	}

	writeJSON(w, map[string]any{"targets": items})
}
