package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/mcp"
	syncpkg "skillshare/internal/sync"
)

func (s *Server) mcpService() *mcp.Service {
	return &mcp.Service{ConfigPath: s.configPath(), ProjectRoot: s.projectRoot, StateDir: config.StateDir(), ConfigDirs: mcp.ConfigDirsFromEnv(), BackupSource: func(path string) (string, error) {
		return syncpkg.StoreBackup(path, syncpkg.BackupReasonMigrate)
	}}
}

type mcpRequest struct {
	Mutation mcp.Mutation `json:"mutation"`
	Revision string       `json:"revision"`
	Sync     bool         `json:"sync"`
	BackupID string       `json:"backupId"`
	Preview  bool         `json:"preview"`
	// Root, with sync and no mutation, applies only that mcp.projects root's changes.
	Root string `json:"root"`
}

func decodeMCPRequest(w http.ResponseWriter, r *http.Request, value any) bool {
	var extra bool
	err := decodeJSONWith(w, r, defaultJSONBodyLimit, func(d *json.Decoder) error {
		d.DisallowUnknownFields()
		if err := d.Decode(value); err != nil {
			return err
		}
		extra = d.Decode(new(any)) != io.EOF
		return nil
	})
	switch {
	case errors.Is(err, errBodyTooLarge):
		return false
	case err != nil:
		writeError(w, 400, "invalid MCP request")
		return false
	case extra:
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
	paths := service.ConfiguredClientPaths(source)
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
	// A root with its own project config is managed from two places; the plan reports a
	// conflict when both hold one entry, so the dashboard says so up front.
	ownConfig := []string{}
	for root := range source.Projects {
		if _, err := os.Stat(filepath.Join(root, ".skillshare", "config.yaml")); err == nil {
			ownConfig = append(ownConfig, root)
		}
	}
	slices.Sort(ownConfig)
	detected := service.DetectedClients(paths)
	if !s.IsProjectMode() {
		detected = append(detected, mcp.DetectedAccounts(source.Accounts)...)
	}
	writeJSON(w, map[string]any{"source": source, "paths": paths, "importSources": service.ImportSources(source), "detected": detected, "plan": p, "previewError": message, "backups": backups, "projectConfigs": ownConfig, "unmanaged": service.FindUnmanaged(source)})
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
// It reads configuration only; it neither executes servers nor writes files.
func (s *Server) handleMCPRender(w http.ResponseWriter, r *http.Request) {
	var body mcpRequest
	if !decodeMCPRequest(w, r, &body) {
		return
	}
	if body.Mutation.Server == nil {
		writeError(w, 400, "server is required")
		return
	}
	service := s.mcpService()
	if body.Mutation.Project != "" {
		if service.ProjectRoot != "" {
			writeError(w, 400, "MCP project overrides are not available in project mode; omit mutation.project")
			return
		}
		source, err := mcp.LoadSource(service.ConfigPath)
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
		if _, ok := source.Projects[body.Mutation.Project]; !ok {
			writeError(w, 400, "unknown MCP project")
			return
		}
		service.ProjectRoot = body.Mutation.Project
	}
	writeJSON(w, map[string]any{"rendered": service.RenderNative(body.Mutation.Name, *body.Mutation.Server)})
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
	if body.Root != "" && (!body.Sync || !reflect.DeepEqual(body.Mutation, mcp.Mutation{})) {
		writeError(w, 400, "a project sync takes no changes")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	start := time.Now()
	var result *mcp.Result
	var err error
	logArgs := map[string]any{"scope": "ui"}
	if body.Root != "" {
		logArgs["project"] = body.Root
		result, err = s.mcpService().ApplyProject(body.Revision, body.Root)
	} else {
		result, err = s.mcpService().Mutate(body.Mutation, body.Revision, body.Sync)
	}
	status := "ok"
	if err != nil {
		status = "error"
	}
	s.writeOpsLog("mcp configure", status, start, logArgs, "")
	if errors.Is(err, mcp.ErrUnknownProject) {
		writeError(w, 400, err.Error())
		return
	}
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
		// PiExtension picks one of Pi's files: pi-mcp-adapter for the adapter's, anything
		// else for mcp.json. Pasted content ignores it.
		PiExtension string `json:"piExtension"`
		// Root reads the target's file in that mcp.projects root instead of this scope.
		Root string `json:"root"`
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
	} else if body.Root != "" {
		candidates, err = s.mcpService().ImportProjectClientMode(body.Root, body.From, body.PiExtension)
	} else {
		candidates, err = s.mcpService().ImportClientMode(body.From, body.PiExtension)
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
