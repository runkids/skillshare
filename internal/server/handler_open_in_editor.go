package server

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"

	"skillshare/internal/childproc"
	"skillshare/internal/wsl"
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
	// windowsPath passes the path in Windows form, for a Windows program started from WSL.
	windowsPath bool
}

// wslExplorer reports explorer.exe when skillshare runs inside WSL; tests replace it.
var wslExplorer = wsl.Explorer

// wslKernel reports a WSL kernel, where xdg-open runs only when explorer.exe was not found; tests replace it.
var wslKernel = wsl.Kernel

// xdgOpenGrace is how long startOpener waits for xdg-open to fail before reporting success.
const xdgOpenGrace = 2 * time.Second

// terminalEditors run inside a terminal. The dashboard starts editors with no terminal,
// so one of these would run where nobody can see it.
var terminalEditors = map[string]bool{
	"vi": true, "vim": true, "nvim": true, "nano": true, "pico": true, "micro": true, "hx": true, "helix": true,
}

func isTerminalEditor(bin string) bool {
	return terminalEditors[strings.TrimSuffix(strings.ToLower(filepath.Base(bin)), ".exe")]
}

// knownEditors returns explicit editor aliases the API accepts.
func knownEditors() map[string]editorCandidate {
	return map[string]editorCandidate{
		"code":     {name: "code", bin: "code", args: []string{"--goto"}},
		"cursor":   {name: "cursor", bin: "cursor", args: nil},
		"windsurf": {name: "windsurf", bin: "windsurf", args: nil},
		"subl":     {name: "subl", bin: "subl", args: nil},
		"sublime":  {name: "subl", bin: "subl", args: nil},
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
// Body: {"editor": "auto" | "code" | "cursor" | "zed" | ...}
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
		if err := decodeJSON(w, r, &req, defaultJSONBodyLimit); errors.Is(err, errBodyTooLarge) {
			return
		}
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

	picked, cmd, status, err := launchEditor(req.Editor, targetPath)
	if err != nil {
		writeError(w, status, err.Error())
		return
	}

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

// launchEditor opens path in the requested editor, or the one pickEditor finds, without
// waiting for it. On failure it returns the HTTP status to report.
func launchEditor(requested, path string) (string, *exec.Cmd, int, error) {
	editor, picked, err := pickEditor(strings.TrimSpace(requested))
	if err != nil {
		return "", nil, http.StatusConflict, fmt.Errorf("no editor available: %w", err)
	}

	if editor.windowsPath {
		if path, err = wsl.WindowsPath(path); err != nil {
			return "", nil, http.StatusInternalServerError, fmt.Errorf("failed to launch %s: %w", editor.name, err)
		}
	}

	cmd := exec.Command(editor.bin, append(editor.args, path)...) //nolint:gosec // editor choice is explicit
	// Detach from the current process so the server doesn't wait on the editor.
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := startOpener(cmd); err != nil {
		return "", nil, http.StatusInternalServerError, fmt.Errorf("failed to launch %s: %w", editor.name, err)
	}
	return picked, cmd, 0, nil
}

// startOpener starts cmd without waiting for the program it opens, and reaps it in the
// background. xdg-open exits non-zero when it cannot show the path, as on a machine with
// no desktop, so its early failure is returned instead of reported as opened. Other openers are not judged by exit code: Explorer exits 1 even on success.
func startOpener(cmd *exec.Cmd) error {
	if err := childproc.StartDetached(cmd); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	if filepath.Base(cmd.Path) != "xdg-open" {
		return nil
	}
	select {
	case err := <-done:
		if err != nil {
			err = fmt.Errorf("xdg-open could not open %s: %w", cmd.Args[len(cmd.Args)-1], err)
			if wslKernel() {
				err = fmt.Errorf("%w; this runs on a WSL kernel but found no explorer.exe on PATH or the C: drive, so Windows could not open it (is Windows interop off, or is this a container?)", err)
			}
			return err
		}
	case <-time.After(xdgOpenGrace):
	}
	return nil
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

	// Auto: respect $VISUAL / $EDITOR first, unless it needs a terminal.
	for _, env := range []string{"VISUAL", "EDITOR"} {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			// $EDITOR may include flags, e.g. "code --wait".
			head, rest := splitCommand(v)
			if isTerminalEditor(head) {
				continue
			}
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
		// Inside WSL, Windows opens the file with the program it is associated with.
		if explorer, ok := wslExplorer(); ok {
			return editorCandidate{name: "explorer", bin: explorer, windowsPath: true}, "explorer", nil
		}
		if _, err := exec.LookPath("xdg-open"); err == nil {
			return editorCandidate{name: "xdg-open", bin: "xdg-open"}, "xdg-open", nil
		}
	case "windows":
		if _, err := exec.LookPath("rundll32"); err == nil {
			return editorCandidate{name: "rundll32", bin: "rundll32", args: []string{"url.dll,FileProtocolHandler"}}, "rundll32", nil
		}
	}

	return editorCandidate{}, "", fmt.Errorf("no editor that opens a window was found; set $VISUAL to a GUI editor such as code")
}

// splitCommand splits an $EDITOR value into (bin, args...). Double or single quotes keep a
// field with spaces together, such as "C:\Program Files\Notepad++\notepad++.exe".
// Backslashes stay literal because Windows paths use them.
func splitCommand(v string) (string, []string) {
	var fields []string
	var field strings.Builder
	var quote rune
	inField := false
	for _, r := range v {
		switch {
		case quote != 0 && r == quote:
			quote = 0
		case quote == 0 && (r == '"' || r == '\''):
			quote, inField = r, true
		case quote == 0 && unicode.IsSpace(r):
			if inField {
				fields = append(fields, field.String())
				field.Reset()
				inField = false
			}
		default:
			field.WriteRune(r)
			inField = true
		}
	}
	if inField {
		fields = append(fields, field.String())
	}
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], fields[1:]
}
