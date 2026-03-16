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

func TestHandleSyncMatrix_Empty(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/sync-matrix", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Entries []any `json:"entries"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(resp.Entries))
	}
}

func TestHandleSyncMatrix_WithSkillsAndTargets(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, sourceDir := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})
	addSkill(t, sourceDir, "frontend-design")
	addSkill(t, sourceDir, "backend-api")
	req := httptest.NewRequest(http.MethodGet, "/api/sync-matrix", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Entries []struct {
			Skill  string `json:"skill"`
			Target string `json:"target"`
			Status string `json:"status"`
		} `json:"entries"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(resp.Entries))
	}
	for _, e := range resp.Entries {
		if e.Target != "claude" {
			t.Errorf("expected target 'claude', got %q", e.Target)
		}
		if e.Status != "synced" {
			t.Errorf("skill %q: expected 'synced', got %q", e.Skill, e.Status)
		}
	}
}

func TestHandleSyncMatrix_WithFilters(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, sourceDir := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})
	addSkill(t, sourceDir, "frontend-design")
	addSkill(t, sourceDir, "backend-api")
	s.cfg.Targets["claude"] = config.TargetConfig{Path: tgtPath, Mode: "merge", Exclude: []string{"backend*"}}
	if err := s.cfg.Save(); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/sync-matrix", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	var resp struct {
		Entries []struct {
			Skill  string `json:"skill"`
			Status string `json:"status"`
			Reason string `json:"reason"`
		} `json:"entries"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(resp.Entries))
	}
	statusMap := map[string]string{}
	reasonMap := map[string]string{}
	for _, e := range resp.Entries {
		statusMap[e.Skill] = e.Status
		reasonMap[e.Skill] = e.Reason
	}
	if statusMap["frontend-design"] != "synced" {
		t.Errorf("frontend-design: expected synced, got %q", statusMap["frontend-design"])
	}
	if statusMap["backend-api"] != "excluded" {
		t.Errorf("backend-api: expected excluded, got %q", statusMap["backend-api"])
	}
	if reasonMap["backend-api"] != "backend*" {
		t.Errorf("backend-api reason: expected 'backend*', got %q", reasonMap["backend-api"])
	}
}

func TestHandleSyncMatrix_TargetQueryParam(t *testing.T) {
	tgt1 := filepath.Join(t.TempDir(), "claude-skills")
	tgt2 := filepath.Join(t.TempDir(), "cursor-skills")
	s, sourceDir := newTestServerWithTargets(t, map[string]string{"claude": tgt1, "cursor": tgt2})
	addSkill(t, sourceDir, "my-skill")
	req := httptest.NewRequest(http.MethodGet, "/api/sync-matrix?target=claude", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	var resp struct {
		Entries []struct {
			Target string `json:"target"`
		} `json:"entries"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	for _, e := range resp.Entries {
		if e.Target != "claude" {
			t.Errorf("expected only claude entries, got %q", e.Target)
		}
	}
}

func TestHandleSyncMatrixPreview_Basic(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, sourceDir := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})
	addSkill(t, sourceDir, "frontend-design")
	addSkill(t, sourceDir, "backend-api")
	addSkill(t, sourceDir, "legacy-tool")
	body := `{"target":"claude","include":["frontend*","backend*"],"exclude":["legacy*"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/sync-matrix/preview", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Entries []struct {
			Skill  string `json:"skill"`
			Status string `json:"status"`
		} `json:"entries"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(resp.Entries))
	}
	statusMap := map[string]string{}
	for _, e := range resp.Entries {
		statusMap[e.Skill] = e.Status
	}
	if statusMap["frontend-design"] != "synced" {
		t.Errorf("frontend-design: expected synced, got %q", statusMap["frontend-design"])
	}
	if statusMap["backend-api"] != "synced" {
		t.Errorf("backend-api: expected synced, got %q", statusMap["backend-api"])
	}
	if statusMap["legacy-tool"] != "not_included" {
		t.Errorf("legacy-tool: expected not_included, got %q", statusMap["legacy-tool"])
	}
}

func TestHandleSyncMatrixPreview_InvalidPattern(t *testing.T) {
	s, _ := newTestServer(t)
	body := `{"target":"claude","include":["[unclosed"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/sync-matrix/preview", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestHandleSyncMatrixPreview_MissingTarget(t *testing.T) {
	s, _ := newTestServer(t)
	body := `{"include":["*"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/sync-matrix/preview", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}
