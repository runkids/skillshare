package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestHandleOpenInEditor_UsesEchoAsEditor(t *testing.T) {
	// Use auto-detect via $EDITOR so we don't need the binary in the whitelist.
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "true")

	s, src := newTestServer(t)
	addSkill(t, src, "my-skill")

	body := openInEditorRequest{Editor: "auto"}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/resources/my-skill/open-in-editor", bytes.NewReader(raw))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp openInEditorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.PID == 0 {
		t.Errorf("expected pid > 0, got %d", resp.PID)
	}
	if resp.Editor != "true" {
		t.Errorf("expected editor=true, got %s", resp.Editor)
	}
}

func TestHandleOpenInEditor_NotFound(t *testing.T) {
	s, _ := newTestServer(t)
	raw, _ := json.Marshal(openInEditorRequest{Editor: "true"})
	req := httptest.NewRequest(http.MethodPost, "/api/resources/no-skill/open-in-editor", bytes.NewReader(raw))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleOpenInEditor_HeadlessRefuses(t *testing.T) {
	t.Setenv("SKILLSHARE_HEADLESS", "1")
	s, src := newTestServer(t)
	addSkill(t, src, "my-skill")

	raw, _ := json.Marshal(openInEditorRequest{Editor: "true"})
	req := httptest.NewRequest(http.MethodPost, "/api/resources/my-skill/open-in-editor", bytes.NewReader(raw))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
}

func TestHandleOpenInEditor_UnknownBinary(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "my-skill")

	raw, _ := json.Marshal(openInEditorRequest{Editor: "this-is-not-a-real-editor-xyz"})
	req := httptest.NewRequest(http.MethodPost, "/api/resources/my-skill/open-in-editor", bytes.NewReader(raw))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
}

func TestPickEditor_AutoPrefersEditorEnv(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "true")

	cand, name, err := pickEditor("auto")
	if err != nil {
		t.Fatalf("pickEditor: %v", err)
	}
	if cand.bin != "true" || name != "true" {
		t.Errorf("expected bin=true, got bin=%s name=%s", cand.bin, name)
	}
}

func TestSplitCommand(t *testing.T) {
	head, rest := splitCommand("code --wait --new-window")
	if head != "code" {
		t.Errorf("expected head=code, got %s", head)
	}
	if len(rest) != 2 || rest[0] != "--wait" || rest[1] != "--new-window" {
		t.Errorf("unexpected rest: %v", rest)
	}
}

func TestSplitCommand_QuotedPath(t *testing.T) {
	for value, want := range map[string][]string{
		`"C:\Program Files\Notepad++\notepad++.exe" -multiInst`: {`C:\Program Files\Notepad++\notepad++.exe`, "-multiInst"},
		`'/Applications/My Editor.app/edit'`:                    {"/Applications/My Editor.app/edit"},
	} {
		head, rest := splitCommand(value)
		if got := append([]string{head}, rest...); strings.Join(got, "|") != strings.Join(want, "|") {
			t.Errorf("%s: got %q, want %q", value, got, want)
		}
	}
}

// stubPrograms writes shell scripts named by the keys into a directory at the front of
// PATH, or alone on it when only is set.
func stubPrograms(t *testing.T, only bool, scripts map[string]string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script stubs need a Unix shell")
	}
	dir := t.TempDir()
	for name, body := range scripts {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if only {
		t.Setenv("PATH", dir)
	} else {
		t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	return dir
}

// fakeWSLExplorer makes skillshare believe it runs inside WSL with explorer at path.
func fakeWSLExplorer(t *testing.T, path string) {
	t.Helper()
	old := wslExplorer
	wslExplorer = func() (string, bool) { return path, true }
	t.Cleanup(func() { wslExplorer = old })
}

func TestPickEditor_AutoSkipsTerminalEditor(t *testing.T) {
	stubPrograms(t, false, map[string]string{"nano": "exit 0", "gui-editor": "exit 0"})
	t.Setenv("VISUAL", "nano")
	t.Setenv("EDITOR", "gui-editor --wait")

	_, name, err := pickEditor("auto")
	if err != nil || name != "gui-editor" {
		t.Fatalf("pickEditor = %q, %v; want gui-editor", name, err)
	}
}

func TestPickEditor_ExplicitTerminalEditorRefused(t *testing.T) {
	if _, _, err := pickEditor("nano"); err == nil {
		t.Fatal("pickEditor(nano) picked an editor the dashboard cannot show")
	}
}

func TestPickEditor_WSLOpensWithExplorer(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("WSL runs Linux")
	}
	stubPrograms(t, true, map[string]string{"nano": "exit 0"})
	fakeWSLExplorer(t, "/mnt/c/Windows/explorer.exe")
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "nano")

	cand, name, err := pickEditor("auto")
	if err != nil || name != "explorer" || !cand.windowsPath {
		t.Fatalf("pickEditor = %+v %q, %v; want explorer with a Windows path", cand, name, err)
	}
}

func TestOpenFileManager_WSLPassesWindowsPathToExplorer(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("WSL runs Linux")
	}
	record := filepath.Join(t.TempDir(), "explorer-args")
	bin := stubPrograms(t, false, map[string]string{
		"explorer.exe": `echo "$@" > ` + record + "\nexit 1",
		"wslpath":      `echo "win($2)"`,
	})
	fakeWSLExplorer(t, filepath.Join(bin, "explorer.exe"))

	if status, err := openFileManager("/root/.config/skillshare"); err != nil {
		t.Fatalf("openFileManager: %d %v", status, err)
	}
	var got []byte
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if got, _ = os.ReadFile(record); len(got) > 0 {
			break
		}
	}
	if want := "win(/root/.config/skillshare)\n"; string(got) != want {
		t.Fatalf("explorer.exe got %q, want %q", got, want)
	}
}

func TestStartOpener_ReportsXdgOpenFailure(t *testing.T) {
	stubPrograms(t, false, map[string]string{"xdg-open": "exit 3"})
	if err := startOpener(exec.Command("xdg-open", "/some/dir")); err == nil {
		t.Fatal("startOpener reported success when xdg-open found nothing to open with")
	}
}

func TestStartOpener_ExplainsMissingExplorerOnWSL(t *testing.T) {
	stubPrograms(t, false, map[string]string{"xdg-open": "exit 3"})
	old := wslKernel
	wslKernel = func() bool { return true }
	t.Cleanup(func() { wslKernel = old })
	err := startOpener(exec.Command("xdg-open", "/some/dir"))
	if err == nil || !strings.Contains(err.Error(), "found no explorer.exe") {
		t.Fatalf("startOpener error = %v; want it to say no explorer.exe was found", err)
	}
}

func TestStartOpener_IgnoresExplorerExitCode(t *testing.T) {
	stubPrograms(t, false, map[string]string{"explorer.exe": "exit 1"})
	if err := startOpener(exec.Command("explorer.exe", `C:\dir`)); err != nil {
		t.Fatalf("startOpener: %v", err)
	}
}
