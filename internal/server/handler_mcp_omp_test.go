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

func TestMCPOmpDashboard(t *testing.T) {
	s, source := newTestServerWithExtras(t, nil, "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PI_CODING_AGENT_DIR", "")
	if err := os.WriteFile(s.configPath(), []byte("source: "+source+"\ntargets: {}\nmcp: {targets: [omp], servers: {}}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	list := httptest.NewRecorder()
	s.handleMCPList(list, httptest.NewRequest(http.MethodGet, "/api/mcp", nil))
	var view struct {
		Paths         map[string]string
		ImportSources map[string][]struct{ Target, Path, PiExtension string }
	}
	path := filepath.Join(home, ".omp", "agent", "mcp.json")
	if list.Code != http.StatusOK || json.Unmarshal(list.Body.Bytes(), &view) != nil || view.Paths["omp"] != path {
		t.Fatalf("OMP list: %s", list.Body.String())
	}
	found := false
	for _, item := range view.ImportSources[""] {
		if item.Target == "omp" {
			found = item.Path == path && item.PiExtension == ""
		}
	}
	if !found {
		t.Fatal("OMP import source missing or treated as a Pi adapter")
	}
	body := `{"mutation":{"name":"docs","server":{"url":"https://example.com/mcp","targets":["omp"]}}}`
	render := httptest.NewRecorder()
	s.handleMCPRender(render, httptest.NewRequest(http.MethodPost, "/api/mcp/render", strings.NewReader(body)))
	var preview struct {
		Rendered []struct{ Target, Path, Content, Error string }
	}
	if render.Code != http.StatusOK || json.Unmarshal(render.Body.Bytes(), &preview) != nil || len(preview.Rendered) != 1 {
		t.Fatalf("OMP render: %s", render.Body.String())
	}
	item := preview.Rendered[0]
	if item.Target != "omp" || item.Path != path || item.Error != "" || !strings.Contains(item.Content, `"type": "http"`) {
		t.Fatalf("OMP preview: %+v", item)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("render created the native file")
	}
}
