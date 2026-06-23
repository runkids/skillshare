package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"skillshare/internal/install"
	syncpkg "skillshare/internal/sync"
)

// extensionInfo is the JSON shape for one extension in the management list.
type extensionInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Builtin     bool     `json:"builtin"`
	Installed   bool     `json:"installed"`
	UsedBy      []string `json:"used_by,omitempty"` // names of extras referencing this extension
}

// extensionUsage maps each extension name to the extras (by name) whose targets
// reference it, so the UI can warn before removing an in-use extension.
func (s *Server) extensionUsage() map[string][]string {
	usage := make(map[string][]string)
	for _, extra := range s.extrasConfig() {
		seen := make(map[string]bool)
		for _, t := range extra.Targets {
			if t.Extension == "" || seen[t.Extension] {
				continue
			}
			seen[t.Extension] = true
			usage[t.Extension] = append(usage[t.Extension], extra.Name)
		}
	}
	return usage
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

	usage := s.extensionUsage()
	infos := make([]extensionInfo, 0, len(names))
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		info := extensionInfo{Name: name, Builtin: install.IsBuiltinExtension(name), Installed: true, UsedBy: usage[name]}
		// Built-ins use the catalog description (consistent, formal wording);
		// local extensions fall back to their own extension.yaml description.
		if info.Builtin {
			info.Description = install.BuiltinExtensionDescription(name)
		} else if spec, derr := syncpkg.LoadExtensionSpec(filepath.Join(dir, name), name); derr == nil {
			info.Description = spec.Description
		}
		infos = append(infos, info)
		seen[name] = true
	}
	// Append built-ins that aren't installed yet (available to download).
	for _, b := range install.BuiltinExtensions {
		if !seen[b.Name] {
			infos = append(infos, extensionInfo{Name: b.Name, Description: b.Description, Builtin: true, Installed: false})
		}
	}

	writeJSON(w, map[string]any{"extensions": infos})
}

// handleExtensionsInstall — POST /api/extensions/install {"name": "codex-agents"}
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
// Opens the current mode's extensions directory in the OS file manager
// (Finder on macOS, Explorer on Windows, the default handler on Linux/BSD).
func (s *Server) handleExtensionsOpen(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	if os.Getenv("SKILLSHARE_HEADLESS") == "1" {
		writeError(w, http.StatusConflict, "refusing to open file manager: SKILLSHARE_HEADLESS=1")
		return
	}

	dir := s.extensionsDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create extensions dir: "+err.Error())
		return
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", dir)
	case "windows":
		cmd = exec.Command("explorer", dir)
	case "linux", "freebsd", "openbsd", "netbsd":
		cmd = exec.Command("xdg-open", dir)
	default:
		writeError(w, http.StatusNotImplemented, "opening a file manager is not supported on this platform")
		return
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	if err := cmd.Start(); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to open file manager: %s", err))
		return
	}
	// Explorer returns a non-zero exit code even on success; ignore Wait result.
	go func() { _ = cmd.Wait() }()

	s.writeOpsLog("extensions-open", "ok", start, map[string]any{
		"path":  dir,
		"scope": "ui",
	}, "")
	writeJSON(w, map[string]any{"path": dir})
}

// handleExtensionsRemove — DELETE /api/extensions/{name}
// Removes an installed extension from the current mode's extensions directory.
// The UI is expected to warn the user first when the extension is still
// referenced by an extra (see extensionInfo.UsedBy); this endpoint performs the
// removal unconditionally once confirmed.
func (s *Server) handleExtensionsRemove(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	name := r.PathValue("name")
	// Guard against path traversal: name must be a single path component.
	if name == "" || name == "." || name == ".." || name != filepath.Base(name) {
		writeError(w, http.StatusBadRequest, "invalid extension name")
		return
	}

	path := filepath.Join(s.extensionsDir(), name)
	if _, err := os.Stat(path); err != nil {
		writeError(w, http.StatusNotFound, "extension not installed: "+name)
		return
	}
	if err := os.RemoveAll(path); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove extension: "+err.Error())
		return
	}

	s.writeOpsLog("extensions-remove", "ok", start, map[string]any{
		"name":  name,
		"scope": "ui",
	}, "")
	writeJSON(w, map[string]any{"success": true, "name": name})
}
