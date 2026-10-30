package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/plugin"
)

func TestPluginAPIRequiresPreviewAndLocalOrigin(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	for _, body := range []string{`{"request":{"action":"sync"}}`, `{"request":{"action":"sync"},"unknown":true}`} {
		w := httptest.NewRecorder()
		s.handlePluginApply(w, httptest.NewRequest(http.MethodPost, "/api/plugins/apply", strings.NewReader(body)))
		if w.Code != 400 {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
	}
	r := httptest.NewRequest(http.MethodPost, "http://attacker.example/api/plugins/apply", strings.NewReader(`{}`))
	r.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	s.requireLocalPlugin(s.handlePluginApply)(w, r)
	if w.Code != 403 {
		t.Fatalf("rebind accepted: %d", w.Code)
	}
}

// The Plugins page draws its Agents from targetDefinitions, so a target that is another
// config directory of an Agent has to be one of them.
func TestPluginListIncludesAnAccountOfAnAgent(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	s.cfg.Targets = map[string]config.TargetConfig{"claude-work": {Agent: "claude", ConfigDir: t.TempDir()}}
	w := httptest.NewRecorder()
	s.handlePluginList(w, httptest.NewRequest(http.MethodGet, "/api/plugins?hosts=false", nil))
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	type definition struct {
		Target string `json:"target"`
		Label  string `json:"label"`
	}
	var inventory struct {
		TargetDefinitions []definition `json:"targetDefinitions"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &inventory); err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(inventory.TargetDefinitions, func(d definition) bool { return d.Target == "claude-work" }) {
		t.Fatalf("the account is missing: %+v", inventory.TargetDefinitions)
	}
}

// A source that cannot be reached answers with a key the dashboard translates, not git's output.
func TestPluginDiscoverNamesANetworkFailure(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	w := httptest.NewRecorder()
	s.handlePluginDiscover(w, httptest.NewRequest(http.MethodPost, "/api/plugins/discover", strings.NewReader(`{"name":"demo","source":"https://127.0.0.1:1/demo.git"}`)))
	var body struct {
		Code string `json:"error_code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || w.Code != 400 || body.Code != "plugins.error.network" {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
}

// Only one plugin change runs at a time; a batch update that cannot start says so before streaming.
func TestPluginUpdateStreamRefusesWhilePluginsAreLocked(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	unlock, err := s.pluginService().Lock()
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	w := httptest.NewRecorder()
	s.handlePluginUpdateStream(w, httptest.NewRequest(http.MethodPost, "/api/plugins/update/stream", strings.NewReader(`{"items":[{"name":"demo","targets":["claude"]}]}`)))
	if w.Code != http.StatusConflict {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
}

// Each plugin ends with its own result, so one that fails leaves the others to run.
func TestPluginUpdateStreamReportsEachPluginThenDone(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	w := httptest.NewRecorder()
	s.handlePluginUpdateStream(w, httptest.NewRequest(http.MethodPost, "/api/plugins/update/stream", strings.NewReader(`{"items":[{"name":"gone","targets":[]},{"name":"other","targets":[]}]}`)))
	body := w.Body.String()
	if strings.Count(body, "event: result") != 2 || !strings.Contains(body, `not found`) || !strings.HasSuffix(strings.TrimSpace(body), `data: {"failure":""}`) {
		t.Fatalf("stream:\n%s", body)
	}
}

// hosts=false returns declarations even before any machine-local native binding
// exists; it never claims that another host's imported plugin is installed here.
func TestPluginListIncludesUnboundNativeImport(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	service := s.pluginService()
	if err := os.WriteFile(service.ConfigPath, []byte("plugins:\n  version: 2\n  packages:\n    router:\n      bindings:\n        omp: {imported: true}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	s.handlePluginList(w, httptest.NewRequest(http.MethodGet, "/api/plugins?hosts=false", nil))
	var result plugin.Inventory
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	b := result.Packages["router"].Bindings["omp"]
	if !b.Imported || b.ID != "" || len(result.Hosts) != 0 {
		t.Fatalf("foreign native binding: %+v", result)
	}
}

func TestPluginPreviewPreservesLocalizedError(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	w := httptest.NewRecorder()
	s.handlePluginPreview(w, httptest.NewRequest(http.MethodPost, "/api/plugins/preview", strings.NewReader(`{"request":{"action":"import","from":"omp","plugin":"npm-package"}}`)))
	var body struct {
		Code string `json:"error_code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || w.Code != 400 || body.Code != "plugins.error.ompPackageImport" {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
}
