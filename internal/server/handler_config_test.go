package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
)

func TestHandleGetConfig(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["raw"] == nil || resp["raw"] == "" {
		t.Error("expected non-empty raw config")
	}
	if resp["config"] == nil {
		t.Error("expected config object in response")
	}
}

func TestHandleAvailableTargets(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/config/available-targets", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Targets []map[string]any `json:"targets"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Targets) == 0 {
		t.Error("expected at least 1 available target")
	}
}

func TestHandleAvailableTargets_CodexDetectedViaInstallDir(t *testing.T) {
	// codex writes to universal's ~/.agents/skills, so only ~/.codex tells the
	// dashboard whether Codex is installed.
	home := t.TempDir()
	t.Setenv("HOME", home) // newTestServer only sets HOME when it is unset
	os.MkdirAll(filepath.Join(home, ".agents"), 0755)

	codexDetected := func() bool {
		t.Helper()
		s, _ := newTestServer(t)
		req := httptest.NewRequest(http.MethodGet, "/api/config/available-targets", nil)
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, req)

		var resp struct {
			Targets []struct {
				Name     string `json:"name"`
				Detected bool   `json:"detected"`
			} `json:"targets"`
		}
		json.Unmarshal(rr.Body.Bytes(), &resp)
		for _, tgt := range resp.Targets {
			if tgt.Name == "codex" {
				return tgt.Detected
			}
		}
		t.Fatal("codex missing from available targets")
		return false
	}

	if codexDetected() {
		t.Error("codex detected = true with only ~/.agents present, want false")
	}

	os.MkdirAll(filepath.Join(home, ".codex"), 0755)
	if !codexDetected() {
		t.Error("codex detected = false with ~/.codex present, want true")
	}
}

func TestHandlePutConfig_InvalidJSON(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader("not json"))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlePutConfig_InvalidYAML(t *testing.T) {
	s, _ := newTestServer(t)
	body := `{"raw":"invalid: [yaml: content"}`
	req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlePutConfig_ValidYAML(t *testing.T) {
	s, sourceDir := newTestServer(t)
	body := `{"raw":"source: ` + sourceDir + `\nmode: merge\ntargets: {}\n"}`
	req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Success  bool     `json:"success"`
		Warnings []string `json:"warnings"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if !resp.Success {
		t.Error("expected success true")
	}
}

func TestHandlePutConfig_InvalidSource_400(t *testing.T) {
	s, _ := newTestServer(t)
	body := `{"raw":"source: /nonexistent/path\nmode: merge\ntargets: {}\n"}`
	req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlePutConfig_InvalidMode_400(t *testing.T) {
	s, sourceDir := newTestServer(t)
	body := `{"raw":"source: ` + sourceDir + `\nmode: invalid\ntargets: {}\n"}`
	req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlePatchConfig_SavesModeAndLogLimit(t *testing.T) {
	s, _ := newTestServer(t)

	patch := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, req)
		return rr
	}

	if rr := patch(`{"mode":"nonsense"}`); rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an unknown mode, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr := patch(`{}`); rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when nothing is sent, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr := patch(`{"mode":"copy","logMaxEntries":500}`); rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mode != "copy" {
		t.Fatalf("expected the saved mode to be copy, got %q", cfg.Mode)
	}
	if cfg.Log.MaxEntries == nil || *cfg.Log.MaxEntries != 500 {
		t.Fatalf("expected the saved log limit to be 500, got %v", cfg.Log.MaxEntries)
	}
}

func TestHandlePutConfig_InvalidHooks_400(t *testing.T) {
	s, sourceDir := newTestServer(t)
	body := `{"raw":"source: ` + sourceDir + `\nmode: merge\ntargets: {}\nhooks:\n  entries:\n    guard:\n      bindings:\n        nosuchagent:\n          events: {}\n"}`
	req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlePutConfig_ProjectHooksProjects_400(t *testing.T) {
	s, _ := newTestProjectServerWithExtras(t, nil)
	body := `{"raw":"targets: [claude]\nhooks:\n  projects:\n    /work/app:\n      entries: {}\n"}`
	req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "hooks.projects") {
		t.Errorf("expected 400 naming hooks.projects, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleConfigSaveAcceptsAccountHooks(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprint(project), func(t *testing.T) {
			var s *Server
			if project {
				s, _ = newTestProjectServerWithExtras(t, nil)
			} else {
				s, _ = newTestServer(t)
			}
			home := hooksTestHome(t)
			if err := os.MkdirAll(filepath.Join(home, "skills"), 0755); err != nil {
				t.Fatal(err)
			}
			raw := "source: " + filepath.Join(home, "skills") + "\ntargets:\n  codex-2:\n    agent: codex\n    config_dir: " + filepath.Join(home, ".codex-2") + "\n    skills: {enabled: false}\nhooks:\n  entries:\n    x:\n      bindings:\n        codex-2:\n          events: {Stop: [{hooks: [{type: command, command: x}]}]}\n"
			if project {
				raw = "targets: []\nhooks:\n  entries:\n    x:\n      bindings:\n        codex-2:\n          events: {Stop: [{hooks: [{type: command, command: x}]}]}\n"
			}
			body, _ := json.Marshal(map[string]string{"raw": raw})
			w := httptest.NewRecorder()
			s.handlePutConfig(w, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(string(body))))
			if project {
				if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "unsupported Agent") {
					t.Fatalf("%d %s", w.Code, w.Body)
				}
			} else if w.Code != http.StatusOK {
				t.Fatalf("%d %s", w.Code, w.Body)
			}
		})
	}
}
