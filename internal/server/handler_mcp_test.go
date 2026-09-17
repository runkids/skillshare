package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPAPIPreviewAndApply(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	body := `{"mutation":{"name":"docs","server":{"url":"https://example.com/mcp","targets":["claude"]}}}`
	preview := httptest.NewRecorder()
	s.handleMCPPreview(preview, httptest.NewRequest(http.MethodPost, "/api/mcp/preview", strings.NewReader(body)))
	if preview.Code != 200 {
		t.Fatalf("preview: %s", preview.Body.String())
	}
	var p struct {
		Revision string `json:"revision"`
	}
	if err := json.Unmarshal(preview.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude.json")); !os.IsNotExist(err) {
		t.Fatal("preview wrote native file")
	}
	apply := httptest.NewRecorder()
	request := strings.TrimSuffix(body, "}") + `,"sync":true,"revision":"` + p.Revision + `"}`
	s.handleMCPConfigure(apply, httptest.NewRequest(http.MethodPost, "/api/mcp", strings.NewReader(request)))
	if apply.Code != 200 {
		t.Fatalf("apply: %s", apply.Body.String())
	}
	if _, err := os.Stat(filepath.Join(home, ".claude.json")); err != nil {
		t.Fatal(err)
	}
}

func TestMCPListIgnoresUnresolvableUnusedTarget(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	xdg := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", xdg)
	for _, name := range []string{"opencode.json", "opencode.jsonc"} {
		path := filepath.Join(xdg, "opencode", name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	w := httptest.NewRecorder()
	s.handleMCPList(w, httptest.NewRequest(http.MethodGet, "/api/mcp", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
}

func TestMCPAPIRequiresPreviewForSync(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	w := httptest.NewRecorder()
	s.handleMCPSync(w, httptest.NewRequest(http.MethodPost, "/api/mcp/sync", strings.NewReader(`{}`)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected missing revision rejection: %d", w.Code)
	}
}
