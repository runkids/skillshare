package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
)

func TestHandleListTargets_Empty(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/targets", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Targets []any `json:"targets"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Targets) != 0 {
		t.Errorf("expected 0 targets, got %d", len(resp.Targets))
	}
}

func TestHandleListTargets_WithTargets(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, _ := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})

	req := httptest.NewRequest(http.MethodGet, "/api/targets", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Targets []map[string]any `json:"targets"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(resp.Targets))
	}
	if resp.Targets[0]["name"] != "claude" {
		t.Errorf("expected target name 'claude', got %v", resp.Targets[0]["name"])
	}
}

func TestHandleAddTarget_Success(t *testing.T) {
	s, _ := newTestServer(t)
	body := `{"name":"test-target","path":"/tmp/test-target"}`
	req := httptest.NewRequest(http.MethodPost, "/api/targets", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["success"] != true {
		t.Error("expected success true")
	}
}

func TestHandleAddTarget_MissingName(t *testing.T) {
	s, _ := newTestServer(t)
	body := `{"path":"/tmp/test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/targets", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing name, got %d", rr.Code)
	}
}

func TestHandleAddTarget_Duplicate(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, _ := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})

	body := `{"name":"claude","path":"/tmp/another"}`
	req := httptest.NewRequest(http.MethodPost, "/api/targets", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409 for duplicate target, got %d", rr.Code)
	}
}

func TestHandleRemoveTarget_Success(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, _ := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})

	req := httptest.NewRequest(http.MethodDelete, "/api/targets/claude", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleRemoveTarget_NotFound(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/targets/nonexistent", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestHandleUpdateTarget_Mode(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, _ := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})

	body := `{"mode":"symlink"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/targets/claude", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleUpdateTarget_NotFound(t *testing.T) {
	s, _ := newTestServer(t)
	body := `{"mode":"merge"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/targets/nonexistent", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestHandleUpdateTarget_IncludeExclude_Persisted(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, _ := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})

	// PATCH include/exclude
	body := `{"include":["my-skill","other-*"],"exclude":["tmp-*"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/targets/claude", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("PATCH expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify disk persistence
	diskCfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config from disk: %v", err)
	}
	tgt, ok := diskCfg.Targets["claude"]
	if !ok {
		t.Fatal("target 'claude' not found in disk config")
	}
	diskSc := tgt.SkillsConfig()
	if len(diskSc.Include) != 2 || diskSc.Include[0] != "my-skill" || diskSc.Include[1] != "other-*" {
		t.Errorf("disk include mismatch: got %v", diskSc.Include)
	}
	if len(diskSc.Exclude) != 1 || diskSc.Exclude[0] != "tmp-*" {
		t.Errorf("disk exclude mismatch: got %v", diskSc.Exclude)
	}

	// Verify in-memory state was reloaded correctly
	memTgt, ok := s.cfg.Targets["claude"]
	if !ok {
		t.Fatal("target 'claude' not in in-memory config")
	}
	memSc := memTgt.SkillsConfig()
	if len(memSc.Include) != 2 || memSc.Include[0] != "my-skill" {
		t.Errorf("in-memory include mismatch: got %v", memSc.Include)
	}
	if len(memSc.Exclude) != 1 || memSc.Exclude[0] != "tmp-*" {
		t.Errorf("in-memory exclude mismatch: got %v", memSc.Exclude)
	}

	// Verify GET /api/targets returns the filters
	req2 := httptest.NewRequest(http.MethodGet, "/api/targets", nil)
	rr2 := httptest.NewRecorder()
	s.handler.ServeHTTP(rr2, req2)

	var resp struct {
		Targets []struct {
			Name    string   `json:"name"`
			Include []string `json:"include"`
			Exclude []string `json:"exclude"`
		} `json:"targets"`
	}
	json.Unmarshal(rr2.Body.Bytes(), &resp)
	if len(resp.Targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(resp.Targets))
	}
	if len(resp.Targets[0].Include) != 2 {
		t.Errorf("GET include mismatch: got %v", resp.Targets[0].Include)
	}
	if len(resp.Targets[0].Exclude) != 1 {
		t.Errorf("GET exclude mismatch: got %v", resp.Targets[0].Exclude)
	}
}

func TestHandleUpdateTarget_ClearFilters(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, _ := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})

	// First set filters
	body := `{"include":["a"],"exclude":["b"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/targets/claude", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("set filters: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Clear filters by sending empty arrays
	body2 := `{"include":[],"exclude":[]}`
	req2 := httptest.NewRequest(http.MethodPatch, "/api/targets/claude", strings.NewReader(body2))
	rr2 := httptest.NewRecorder()
	s.handler.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("clear filters: expected 200, got %d: %s", rr2.Code, rr2.Body.String())
	}

	// Verify disk config has empty filters
	diskCfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config from disk: %v", err)
	}
	tgt := diskCfg.Targets["claude"]
	if len(tgt.Include) != 0 {
		t.Errorf("expected empty include after clear, got %v", tgt.Include)
	}
	if len(tgt.Exclude) != 0 {
		t.Errorf("expected empty exclude after clear, got %v", tgt.Exclude)
	}

	// Verify GET also returns empty filters
	req3 := httptest.NewRequest(http.MethodGet, "/api/targets", nil)
	rr3 := httptest.NewRecorder()
	s.handler.ServeHTTP(rr3, req3)

	var resp struct {
		Targets []struct {
			Include []string `json:"include"`
			Exclude []string `json:"exclude"`
		} `json:"targets"`
	}
	json.Unmarshal(rr3.Body.Bytes(), &resp)
	if len(resp.Targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(resp.Targets))
	}
	if len(resp.Targets[0].Include) != 0 {
		t.Errorf("GET include should be empty after clear, got %v", resp.Targets[0].Include)
	}
	if len(resp.Targets[0].Exclude) != 0 {
		t.Errorf("GET exclude should be empty after clear, got %v", resp.Targets[0].Exclude)
	}
}
