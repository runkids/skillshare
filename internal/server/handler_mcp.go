package server

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/mcp"
)

// mcpConfigPath returns the MCP config file path for the current mode.
func (s *Server) mcpConfigPath() string {
	if s.IsProjectMode() {
		return mcp.ProjectMCPConfigPath(s.projectRoot)
	}
	return mcp.MCPConfigPath(config.BaseDir())
}

// mcpStateDir returns the directory for mcp_state.yaml.
// In project mode this is .skillshare/; in global mode it is the config base dir.
func (s *Server) mcpStateDir() string {
	if s.IsProjectMode() {
		return s.projectRoot + "/.skillshare"
	}
	return config.BaseDir()
}

// handleMCPList — GET /api/mcp
func (s *Server) handleMCPList(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	mcpCfgPath := s.mcpConfigPath()
	projectMode := s.IsProjectMode()
	projectRoot := s.projectRoot
	mcpMode := s.cfg.MCPMode
	s.mu.RUnlock()

	cfg, err := mcp.LoadMCPConfig(mcpCfgPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load mcp config: "+err.Error())
		return
	}

	targets := mcp.MCPTargetsForMode(projectMode)

	type targetStatus struct {
		Name   string `json:"name"`
		Path   string `json:"path"`
		Status string `json:"status"` // "synced", "not synced", "skipped"
	}

	targetStatuses := make([]targetStatus, 0, len(targets))
	for _, t := range targets {
		var targetPath string
		if projectMode {
			targetPath = t.ProjectConfigPath(projectRoot)
		} else {
			targetPath = t.GlobalConfigPath()
		}

		ts := targetStatus{
			Name: t.Name,
			Path: targetPath,
		}

		if targetPath == "" {
			ts.Status = "skipped"
		} else if _, err := os.Stat(targetPath); os.IsNotExist(err) {
			ts.Status = "not synced"
		} else {
			ts.Status = "synced"
		}

		targetStatuses = append(targetStatuses, ts)
	}

	if mcpMode == "" {
		mcpMode = "merge"
	}

	writeJSON(w, map[string]any{
		"servers":  cfg.Servers,
		"targets":  targetStatuses,
		"mcp_mode": mcpMode,
	})
}

// handleMCPCreate — POST /api/mcp
func (s *Server) handleMCPCreate(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var body struct {
		Name    string            `json:"name"`
		Command string            `json:"command,omitempty"`
		Args    []string          `json:"args,omitempty"`
		URL     string            `json:"url,omitempty"`
		Headers map[string]string `json:"headers,omitempty"`
		Env     map[string]string `json:"env,omitempty"`
		Targets []string          `json:"targets,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if body.Command == "" && body.URL == "" {
		writeError(w, http.StatusBadRequest, "command or url is required")
		return
	}
	if body.Command != "" && body.URL != "" {
		writeError(w, http.StatusBadRequest, "cannot specify both command and url")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	mcpCfgPath := s.mcpConfigPath()

	cfg, err := mcp.LoadMCPConfig(mcpCfgPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load mcp config: "+err.Error())
		return
	}

	if _, exists := cfg.Servers[body.Name]; exists {
		writeError(w, http.StatusConflict, "server already exists: "+body.Name)
		return
	}

	srv := mcp.MCPServer{
		Command: body.Command,
		Args:    body.Args,
		URL:     body.URL,
		Headers: body.Headers,
		Env:     body.Env,
		Targets: body.Targets,
	}
	cfg.Servers[body.Name] = srv

	if err := cfg.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := cfg.Save(mcpCfgPath); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save mcp config: "+err.Error())
		return
	}

	s.writeOpsLog("mcp-create", "ok", start, map[string]any{
		"name":  body.Name,
		"scope": "ui",
	}, "")

	writeJSON(w, map[string]any{"success": true, "name": body.Name})
}

// handleMCPDelete — DELETE /api/mcp/{name}
func (s *Server) handleMCPDelete(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	name := r.PathValue("name")

	s.mu.Lock()
	defer s.mu.Unlock()

	mcpCfgPath := s.mcpConfigPath()

	cfg, err := mcp.LoadMCPConfig(mcpCfgPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load mcp config: "+err.Error())
		return
	}

	if _, exists := cfg.Servers[name]; !exists {
		writeError(w, http.StatusNotFound, "server not found: "+name)
		return
	}

	delete(cfg.Servers, name)

	if err := cfg.Save(mcpCfgPath); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save mcp config: "+err.Error())
		return
	}

	s.writeOpsLog("mcp-delete", "ok", start, map[string]any{
		"name":  name,
		"scope": "ui",
	}, "")

	writeJSON(w, map[string]any{"success": true, "name": name})
}

// handleMCPUpdate — PATCH /api/mcp/{name}
func (s *Server) handleMCPUpdate(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	name := r.PathValue("name")

	var body struct {
		Disabled *bool `json:"disabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	mcpCfgPath := s.mcpConfigPath()

	cfg, err := mcp.LoadMCPConfig(mcpCfgPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load mcp config: "+err.Error())
		return
	}

	srv, exists := cfg.Servers[name]
	if !exists {
		writeError(w, http.StatusNotFound, "server not found: "+name)
		return
	}

	if body.Disabled != nil {
		srv.Disabled = *body.Disabled
	}
	cfg.Servers[name] = srv

	if err := cfg.Save(mcpCfgPath); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save mcp config: "+err.Error())
		return
	}

	s.writeOpsLog("mcp-update", "ok", start, map[string]any{
		"name":  name,
		"scope": "ui",
	}, "")

	writeJSON(w, map[string]any{"success": true, "name": name})
}

// handleMCPSync — POST /api/mcp/sync
func (s *Server) handleMCPSync(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var body struct {
		Force  bool `json:"force"`
		DryRun bool `json:"dry_run"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && r.ContentLength > 0 {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	s.mu.RLock()
	mcpCfgPath := s.mcpConfigPath()
	stateDir := s.mcpStateDir()
	projectMode := s.IsProjectMode()
	projectRoot := s.projectRoot
	mcpMode := s.cfg.MCPMode
	s.mu.RUnlock()

	if mcpMode == "" {
		mcpMode = "merge"
	}

	cfg, err := mcp.LoadMCPConfig(mcpCfgPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load mcp config: "+err.Error())
		return
	}

	if err := cfg.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, "mcp config validation failed: "+err.Error())
		return
	}

	targets := mcp.MCPTargetsForMode(projectMode)

	type syncResultEntry struct {
		Target string `json:"target"`
		Status string `json:"status"`
		Path   string `json:"path,omitempty"`
		Error  string `json:"error,omitempty"`
	}

	results := make([]syncResultEntry, 0, len(targets))

	switch mcpMode {
	case "merge":
		statePath := mcp.MCPStatePath(stateDir)
		state, err := mcp.LoadMCPState(statePath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load mcp state: "+err.Error())
			return
		}

		for _, t := range targets {
			var targetPath string
			if projectMode {
				targetPath = t.ProjectConfigPath(projectRoot)
			} else {
				targetPath = t.GlobalConfigPath()
			}

			if targetPath == "" {
				results = append(results, syncResultEntry{
					Target: t.Name,
					Status: "skipped",
				})
				continue
			}

			servers := cfg.ServersForTarget(t.Name)
			prev := state.PreviousServers(t.Name)

			mergeResult, err := mcp.MergeToTarget(targetPath, servers, prev, t, body.DryRun)
			entry := syncResultEntry{
				Target: mergeResult.Target,
				Path:   targetPath,
			}
			if err != nil {
				entry.Status = "error"
				entry.Error = mergeResult.Error
			} else {
				entry.Status = "ok"
				// Update state with current server names
				if !body.DryRun {
					serverNames := make([]string, 0, len(servers))
					for name := range servers {
						serverNames = append(serverNames, name)
					}
					state.UpdateTarget(t.Name, serverNames, targetPath)
				}
			}
			results = append(results, entry)
		}

		if !body.DryRun {
			statePath := mcp.MCPStatePath(stateDir)
			_ = state.Save(statePath)
		}

	case "symlink", "copy":
		genDir := mcp.GeneratedDir(stateDir)
		generatedFiles, err := mcp.GenerateAllTargetFiles(cfg, targets, genDir)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate target files: "+err.Error())
			return
		}

		for _, t := range targets {
			var targetPath string
			if projectMode {
				targetPath = t.ProjectConfigPath(projectRoot)
			} else {
				targetPath = t.GlobalConfigPath()
			}

			if targetPath == "" {
				results = append(results, syncResultEntry{
					Target: t.Name,
					Status: "skipped",
				})
				continue
			}

			genPath, hasGen := generatedFiles[t.Name]
			if !hasGen {
				results = append(results, syncResultEntry{
					Target: t.Name,
					Status: "skipped",
					Path:   targetPath,
				})
				continue
			}

			var sr mcp.SyncResult
			if mcpMode == "symlink" {
				sr = mcp.SyncTarget(t.Name, genPath, targetPath, body.DryRun)
			} else {
				sr = mcp.CopyToTarget(t.Name, genPath, targetPath, body.DryRun)
			}

			entry := syncResultEntry{
				Target: sr.Target,
				Status: sr.Status,
				Path:   sr.Path,
			}
			if sr.Error != "" {
				entry.Error = sr.Error
			}
			results = append(results, entry)
		}

	default:
		writeError(w, http.StatusBadRequest, "unknown mcp_mode: "+mcpMode)
		return
	}

	s.writeOpsLog("mcp-sync", "ok", start, map[string]any{
		"mode":    mcpMode,
		"dryRun":  body.DryRun,
		"targets": len(results),
		"scope":   "ui",
	}, "")

	writeJSON(w, map[string]any{
		"results": results,
		"mode":    mcpMode,
	})
}
