package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/mcp"
)

func (s *Server) mcpService() *mcp.Service {
	return &mcp.Service{ConfigPath: s.configPath(), ProjectRoot: s.projectRoot, StateDir: config.StateDir(), ConfigDirs: mcp.ConfigDirsFromEnv()}
}

type mcpRequest struct {
	Mutation mcp.Mutation `json:"mutation"`
	Revision string       `json:"revision"`
	Sync     bool         `json:"sync"`
	BackupID string       `json:"backupId"`
	Preview  bool         `json:"preview"`
}

func decodeMCPRequest(w http.ResponseWriter, r *http.Request, value any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		writeError(w, 400, "invalid MCP request")
		return false
	}
	if d.Decode(new(any)) != io.EOF {
		writeError(w, 400, "expected one MCP request")
		return false
	}
	return true
}

func (s *Server) handleMCPList(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	service := s.mcpService()
	source, err := mcp.LoadSource(service.ConfigPath)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	// Unresolvable paths are omitted; selected targets report them via previewError.
	paths := service.ClientPaths()
	p, previewErr := service.Preview()
	message := ""
	if previewErr != nil {
		message = previewErr.Error()
	}
	backups, err := service.Backups()
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]any{"source": source, "paths": paths, "detected": service.DetectedClients(paths), "plan": p, "previewError": message, "backups": backups})
}

func (s *Server) handleMCPPreview(w http.ResponseWriter, r *http.Request) {
	var body mcpRequest
	if !decodeMCPRequest(w, r, &body) {
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, err := s.mcpService().PreviewMutation(body.Mutation)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, p)
}

// handleMCPRender shows the server in the request as each of its targets would store it.
// It reads and writes nothing, so an unsaved form can ask.
func (s *Server) handleMCPRender(w http.ResponseWriter, r *http.Request) {
	var body mcpRequest
	if !decodeMCPRequest(w, r, &body) {
		return
	}
	if body.Mutation.Server == nil {
		writeError(w, 400, "server is required")
		return
	}
	writeJSON(w, map[string]any{"rendered": s.mcpService().RenderNative(body.Mutation.Name, *body.Mutation.Server)})
}

func (s *Server) handleMCPConfigure(w http.ResponseWriter, r *http.Request) {
	var body mcpRequest
	if !decodeMCPRequest(w, r, &body) {
		return
	}
	if body.Revision == "" {
		writeError(w, 400, "preview the MCP changes before saving")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	start := time.Now()
	result, err := s.mcpService().Mutate(body.Mutation, body.Revision, body.Sync)
	status := "ok"
	if err != nil {
		status = "error"
	}
	s.writeOpsLog("mcp configure", status, start, map[string]any{"scope": "ui"}, "")
	if err != nil {
		writeMCPFailure(w, result, err)
		return
	}
	if err := s.reloadConfig(); err != nil {
		writeError(w, 500, "MCP saved, but config reload failed")
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleMCPImport(w http.ResponseWriter, r *http.Request) {
	var body struct {
		From    string `json:"from"`
		Content string `json:"content"`
		Name    string `json:"name"`
	}
	if !decodeMCPRequest(w, r, &body) {
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var candidates []mcp.Candidate
	var err error
	if body.Content != "" {
		candidates, err = mcp.Import(body.From, []byte(body.Content), body.Name)
	} else {
		candidates, err = s.mcpService().ImportClient(body.From)
	}
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, map[string]any{"candidates": candidates})
}

func (s *Server) handleMCPRestore(w http.ResponseWriter, r *http.Request) {
	var body mcpRequest
	if !decodeMCPRequest(w, r, &body) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	service := s.mcpService()
	if body.Preview {
		p, err := service.PreviewRestore(body.BackupID)
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
		writeJSON(w, p)
		return
	}
	if body.Revision == "" {
		writeError(w, 400, "preview before restoring")
		return
	}
	start := time.Now()
	result, err := service.Restore(body.BackupID, body.Revision)
	status := "ok"
	if err != nil {
		status = "error"
	}
	s.writeOpsLog("mcp restore", status, start, map[string]any{"scope": "ui"}, "")
	if err != nil {
		writeMCPFailure(w, result, err)
		return
	}
	writeJSON(w, result)
}

func writeMCPFailure(w http.ResponseWriter, result *mcp.Result, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusConflict)
	message := err.Error()
	if result != nil && len(result.Applied) > 0 {
		message += fmt.Sprintf("; already applied: %s; backup IDs: %s", strings.Join(result.Applied, ", "), strings.Join(result.BackupIDs, ", "))
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"error": message, "result": result})
}
