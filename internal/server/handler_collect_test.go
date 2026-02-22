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

func TestHandleCollectScan_Empty(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/collect/scan", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Targets    []any `json:"targets"`
		TotalCount int   `json:"totalCount"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.TotalCount != 0 {
		t.Errorf("expected 0 total, got %d", resp.TotalCount)
	}
}

func TestHandleCollectScan_WithLocalSkills(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, _ := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})

	// Create a local skill in target
	localSkill := filepath.Join(tgtPath, "local-skill")
	os.MkdirAll(localSkill, 0755)
	os.WriteFile(filepath.Join(localSkill, "SKILL.md"), []byte("local"), 0644)

	req := httptest.NewRequest(http.MethodGet, "/api/collect/scan", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		TotalCount int `json:"totalCount"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.TotalCount != 1 {
		t.Errorf("expected 1 local skill, got %d", resp.TotalCount)
	}
}

func TestHandleCollect_NoSkills(t *testing.T) {
	s, _ := newTestServer(t)
	body := `{"skills":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/collect", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty skills, got %d", rr.Code)
	}
}
