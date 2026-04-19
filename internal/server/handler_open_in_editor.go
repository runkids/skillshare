package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// openInEditorRequest controls which editor to launch.
// editor="auto" (or empty) walks a platform-aware fallback chain.
type openInEditorRequest struct {
	Editor string `json:"editor,omitempty"`
}

type openInEditorResponse struct {
	Editor string `json:"editor"`
	Path   string `json:"path"`
	PID    int    `json:"pid"`
}

// editorCandidate represents an external editor we can try to launch.
type editorCandidate struct {
	// name is the alias the API accepts (e.g. "code", "cursor").
	name string
	// bin is the executable to spawn.
	bin string
	// args are appended before the file path.
	args []string
}

// knownEditors returns explicit editor aliases the API accepts.
func knownEditors() map[string]editorCandidate {
	return map[string]editorCandidate{
		"code":     {name: "code", bin: "code", args: []string{"--goto"}},
		"cursor":   {name: "cursor", bin: "cursor", args: nil},
		"windsurf": {name: "windsurf", bin: "windsurf", args: nil},
		"subl":     {name: "subl", bin: "subl", args: nil},
		"sublime":  {name: "subl", bin: "subl", args: nil},
		"vim":      {name: "vim", bin: "vim", args: nil},
		"nvim":     {name: "nvim", bin: "nvim", args: nil},
		"nano":     {name: "nano", bin: "nano", args: nil},
		"emacs":    {name: "emacs", bin: "emacs", args: nil},
		"idea":     {name: "idea", bin: "idea", args: nil},
		"webstorm": {name: "webstorm", bin: "webstorm", args: nil},
		"goland":   {name: "goland", bin: "goland", args: nil},
		"textmate": {name: "textmate", bin: "mate", args: nil},
		"mate":     {name: "mate", bin: "mate", args: nil},
		"zed":      {name: "zed", bin: "zed", args: nil},
	}
}

// handleOpenSkillInEditor launches the configured (or auto-detected) local
// editor against the skill's canonical markdown file.
//
// POST /api/resources/{name}/open-in-editor
// Body: {"editor": "auto" | "code" | "cursor" | "vim" | ...}
func (s *Server) handleOpenSkillInEditor(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	s.mu.RLock()
	source := s.skillsSource()
	agentsSource := s.agentsSource()
	s.mu.RUnlock()

	name := r.PathValue("name")
	kind := r.URL.Query().Get("kind")
	if kind != "" && kind != "skill" && kind != "agent" {
		writeError(w, http.StatusBadRequest, "invalid kind: "+kind)
		return
	}

	var req openInEditorRequest
	// Body is optional; silently ignore decode errors when empty.
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	targetPath, resolvedKind, err := s.resolveEditableSkillPath(source, agentsSource, name, kind)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	// Skills are directories (SKILL.md + supporting files); open the folder
	// so editors like VS Code / Cursor load the whole workspace. Agents are
	// single .md files and stay as-is.
	if resolvedKind == "skill" {
		targetPath = filepath.Dir(targetPath)
	}

	if os.Getenv("SKILLSHARE_HEADLESS") == "1" {
		writeError(w, http.StatusConflict, "refusing to launch editor: SKILLSHARE_HEADLESS=1")
		return
	}

	editor, picked, err := pickEditor(strings.TrimSpace(req.Editor))
	if err != nil {
		writeError(w, http.StatusConflict, "no editor available: "+err.Error())
		return
	}

	cmd := exec.Command(editor.bin, append(editor.args, targetPath)...) //nolint:gosec // editor choice is explicit
	// Detach from the current process so the server doesn't wait on the editor.
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to launch %s: %s", editor.bin, err))
		return
	}
	// Reap in the background so we don't accumulate zombies on *nix.
	go func() { _ = cmd.Wait() }()

	s.writeOpsLog("skill.openInEditor", "ok", start, map[string]any{
		"name":   name,
		"kind":   resolvedKind,
		"editor": picked,
		"path":   targetPath,
	}, "")

	writeJSON(w, openInEditorResponse{
		Editor: picked,
		Path:   targetPath,
		PID:    cmd.Process.Pid,
	})
}

// pickEditor returns the editor to launch, honouring an explicit request
// (e.g. "code") when it resolves, otherwise walking the fallback chain.
func pickEditor(requested string) (editorCandidate, string, error) {
	if requested != "" && requested != "auto" {
		if cand, ok := knownEditors()[strings.ToLower(requested)]; ok {
			if _, err := exec.LookPath(cand.bin); err == nil {
				return cand, cand.name, nil
			}
			return editorCandidate{}, "", fmt.Errorf("%s not found on PATH", cand.bin)
		}
		// Unknown alias — reject to prevent arbitrary binary execution.
		return editorCandidate{}, "", fmt.Errorf("unsupported editor: %s", requested)
	}

	// Auto: respect $VISUAL / $EDITOR first.
	for _, env := range []string{"VISUAL", "EDITOR"} {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			// $EDITOR may include flags, e.g. "code --wait".
			head, rest := splitCommand(v)
			if _, err := exec.LookPath(head); err == nil {
				return editorCandidate{name: head, bin: head, args: rest}, head, nil
			}
		}
	}

	// GUI IDE preferences.
	for _, alias := range []string{"code", "cursor", "zed", "subl", "windsurf", "idea"} {
		if cand, ok := knownEditors()[alias]; ok {
			if _, err := exec.LookPath(cand.bin); err == nil {
				return cand, cand.name, nil
			}
		}
	}

	// OS native fallback — opens with the user's default handler.
	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("open"); err == nil {
			return editorCandidate{name: "open", bin: "open"}, "open", nil
		}
	case "linux":
		if _, err := exec.LookPath("xdg-open"); err == nil {
			return editorCandidate{name: "xdg-open", bin: "xdg-open"}, "xdg-open", nil
		}
	case "windows":
		if _, err := exec.LookPath("rundll32"); err == nil {
			return editorCandidate{name: "rundll32", bin: "rundll32", args: []string{"url.dll,FileProtocolHandler"}}, "rundll32", nil
		}
	}

	// Terminal fallbacks — nano is friendlier than vim for newcomers.
	for _, alias := range []string{"nano", "vim", "nvim", "emacs"} {
		if cand, ok := knownEditors()[alias]; ok {
			if _, err := exec.LookPath(cand.bin); err == nil {
				return cand, cand.name, nil
			}
		}
	}

	return editorCandidate{}, "", fmt.Errorf("no usable editor found; set $EDITOR or pass editor=<name>")
}

// splitCommand naively splits an $EDITOR value into (bin, args...).
// Quoted arguments are not supported because $EDITOR rarely needs them.
func splitCommand(v string) (string, []string) {
	fields := strings.Fields(v)
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], fields[1:]
}
