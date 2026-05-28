package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/install"
	syncpkg "skillshare/internal/sync"
)

// extensionInfo is the JSON shape for one extension in the management list.
type extensionInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Builtin     bool   `json:"builtin"`
	Installed   bool   `json:"installed"`
}

// extensionsDir returns the transform-extensions directory for the current
// mode (~/.config/skillshare/extensions globally, .skillshare/extensions in
// project mode).
func (s *Server) extensionsDir() string {
	return filepath.Join(filepath.Dir(s.configPath()), "extensions")
}

// handleExtensionsList — GET /api/extensions
// Lists installed extensions (with descriptions) merged with the built-in
// catalog, so the UI can show what's installed and what's available to install.
func (s *Server) handleExtensionsList(w http.ResponseWriter, r *http.Request) {
	dir := s.extensionsDir()
	names, err := syncpkg.ListExtensions(dir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	infos := make([]extensionInfo, 0, len(names))
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		info := extensionInfo{Name: name, Builtin: install.IsBuiltinExtension(name), Installed: true}
		if spec, derr := syncpkg.LoadExtensionSpec(filepath.Join(dir, name), name); derr == nil {
			info.Description = spec.Description
		}
		infos = append(infos, info)
		seen[name] = true
	}
	// Append built-ins that aren't installed yet (available to download).
	for _, name := range install.BuiltinExtensions {
		if !seen[name] {
			infos = append(infos, extensionInfo{Name: name, Builtin: true, Installed: false})
		}
	}

	writeJSON(w, map[string]any{"extensions": infos})
}

// handleExtensionsInstall — POST /api/extensions/install {"name": "md2codex"}
// Downloads a built-in extension into the current mode's extensions directory.
func (s *Server) handleExtensionsInstall(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if !install.IsBuiltinExtension(body.Name) {
		writeError(w, http.StatusBadRequest, "unknown built-in extension: "+body.Name)
		return
	}

	dir := s.extensionsDir()
	if err := install.InstallBuiltinExtension(body.Name, dir); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to install extension: "+err.Error())
		return
	}

	s.writeOpsLog("extensions-install", "ok", start, map[string]any{
		"name":  body.Name,
		"scope": "ui",
	}, "")
	writeJSON(w, map[string]any{"success": true, "name": body.Name})
}

// handleExtensionsOpen — POST /api/extensions/open
// Opens the current mode's extensions directory in the user's editor, mirroring
// the skill "open in editor" affordance.
func (s *Server) handleExtensionsOpen(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	if os.Getenv("SKILLSHARE_HEADLESS") == "1" {
		writeError(w, http.StatusConflict, "refusing to launch editor: SKILLSHARE_HEADLESS=1")
		return
	}

	var req openInEditorRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	dir := s.extensionsDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create extensions dir: "+err.Error())
		return
	}

	editor, picked, err := pickEditor(strings.TrimSpace(req.Editor))
	if err != nil {
		writeError(w, http.StatusConflict, "no editor available: "+err.Error())
		return
	}

	cmd := exec.Command(editor.bin, append(editor.args, dir)...) //nolint:gosec // editor choice is explicit
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	if err := cmd.Start(); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to launch %s: %s", editor.bin, err))
		return
	}
	go func() { _ = cmd.Wait() }()

	s.writeOpsLog("extensions-open", "ok", start, map[string]any{
		"editor": picked,
		"path":   dir,
		"scope":  "ui",
	}, "")
	writeJSON(w, openInEditorResponse{Editor: picked, Path: dir, PID: cmd.Process.Pid})
}
