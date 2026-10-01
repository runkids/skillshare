package server

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"time"

	"gopkg.in/yaml.v3"
	"skillshare/internal/config"
	"skillshare/internal/hooks"
	"skillshare/internal/plugin"
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
// All fields live in the global config only; project mode has no default mode, log limit or backup limits.
func (s *Server) handlePatchConfig(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var body struct {
		Mode            *string `json:"mode"`
		LogMaxEntries   *int    `json:"logMaxEntries"`
		BackupMaxCount  *int    `json:"backupMaxCount"`
		BackupMaxSizeMB *int64  `json:"backupMaxSizeMB"`
	}
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
		}
		return
	}
	if body.Mode == nil && body.LogMaxEntries == nil && body.BackupMaxCount == nil && body.BackupMaxSizeMB == nil {
		writeError(w, http.StatusBadRequest, "mode, logMaxEntries, backupMaxCount or backupMaxSizeMB is required")
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
	if body.BackupMaxCount != nil && *body.BackupMaxCount < 0 || body.BackupMaxSizeMB != nil && *body.BackupMaxSizeMB < 0 {
		writeError(w, http.StatusBadRequest, "backup limits cannot be negative")
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
	if body.BackupMaxCount != nil {
		s.cfg.Backup.MaxCount = body.BackupMaxCount
		args["backup_max_count"] = *body.BackupMaxCount
	}
	if body.BackupMaxSizeMB != nil {
		s.cfg.Backup.MaxSizeMB = body.BackupMaxSizeMB
		args["backup_max_size_mb"] = *body.BackupMaxSizeMB
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
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
		}
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
		if err := plugin.Validate([]byte(body.Raw), nil); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		w2, validErr := config.ValidateProjectConfig(&testCfg, s.projectRoot)
		if validErr == nil {
			validErr = config.ValidateMCP(testCfg.MCP, testCfg.Sources.MCP)
		}
		if validErr == nil {
			validErr = hooks.ValidateSection(&testCfg.Hooks, true, nil)
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
		// An account of an Agent is a target for plugins under the name the config gives it.
		if err := plugin.Validate([]byte(body.Raw), testCfg.AgentConfigDirTargetAgents()); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		w2, validErr := config.ValidateConfig(&testCfg)
		if validErr == nil {
			validErr = config.ValidateMCP(testCfg.MCP, testCfg.Sources.MCP, testCfg.AgentConfigDirTargets()...)
		}
		if validErr == nil {
			validErr = hooks.ValidateSection(&testCfg.Hooks, false, testCfg.AgentConfigDirTargetAgents())
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
		// ConfigDir is set when a target can be another config directory of this Agent.
		ConfigDir string `json:"configDir,omitempty"`
		Installed bool   `json:"installed"`
		Detected  bool   `json:"detected"`
		// ReadsFrom names the configured targets with skills on whose skills
		// folder this tool already reads.
		ReadsFrom []string `json:"readsFrom,omitempty"`
		// InstructionsFile is the file name of the tool's instructions file
		// (AGENTS.md, GEMINI.md, ...), empty when it has none.
		InstructionsFile string `json:"instructionsFile,omitempty"`
		// InstructionsPath is that file's default path, for the add dialog's list of writes.
		InstructionsPath string `json:"instructionsPath,omitempty"`
		// ReadBy names the other tools, on this machine or configured, whose
		// default scan paths include this tool's skills folder.
		ReadBy []string `json:"readBy,omitempty"`
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
		// Check if the tool's config directory exists: its declared detect dir,
		// else the parent of the skills path.
		detected := false
		if !installed {
			probe := config.DetectDir(name)
			if probe == "" {
				probe = filepath.Dir(tc.Path)
			}
			if _, err := os.Stat(probe); err == nil {
				detected = true
			}
		}
		item := availTarget{
			Name:      name,
			Path:      tc.Path,
			Installed: installed,
			Detected:  detected,
			ReadsFrom: config.SkillsReadFrom(targets, name, projectRoot),
		}
		if agentTC, ok := agentDefaults[name]; ok {
			item.AgentPath = agentTC.Path
		}
		if it, ok := config.LookupInstructions(name, isProjectMode); ok {
			item.InstructionsFile = filepath.Base(it.Path)
			item.InstructionsPath = it.Path
		}
		if !isProjectMode {
			item.ConfigDir = config.AgentConfigDir(name)
		}
		items = append(items, item)
	}

	// Who reads each folder, so a shared folder (universal) can show the tools it serves.
	present := func(it availTarget) bool { return it.Installed || it.Detected }
	for i := range items {
		folder := filepath.Clean(config.ExpandPath(items[i].Path))
		for _, other := range items {
			if other.Name == items[i].Name || !present(other) {
				continue
			}
			for _, p := range config.RuntimeScanPaths(other.Name, isProjectMode) {
				if filepath.Clean(config.ExpandPath(p)) == folder {
					items[i].ReadBy = append(items[i].ReadBy, other.Name)
					break
				}
			}
		}
		slices.Sort(items[i].ReadBy)
	}

	writeJSON(w, map[string]any{"targets": items})
}
