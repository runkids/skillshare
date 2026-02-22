package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleAuditAll_EmptySource(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/audit", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Results []any `json:"results"`
		Summary struct {
			Total int `json:"total"`
		} `json:"summary"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(resp.Results))
	}
	if resp.Summary.Total != 0 {
		t.Errorf("expected summary total 0, got %d", resp.Summary.Total)
	}
}

func TestHandleAuditAll_WithSkills(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "safe-skill")

	req := httptest.NewRequest(http.MethodGet, "/api/audit", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Results []any `json:"results"`
		Summary struct {
			Total int `json:"total"`
		} `json:"summary"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Results) != 1 {
		t.Errorf("expected 1 audited result, got %d", len(resp.Results))
	}
	if resp.Summary.Total != 1 {
		t.Errorf("expected summary total 1, got %d", resp.Summary.Total)
	}
}

func TestHandleAuditSkill_NotFound(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/audit/nonexistent", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleAuditSkill_Found(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "my-skill")

	req := httptest.NewRequest(http.MethodGet, "/api/audit/my-skill", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Result struct {
			SkillName string `json:"skillName"`
		} `json:"result"`
		Summary struct {
			Total int `json:"total"`
		} `json:"summary"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Result.SkillName != "my-skill" {
		t.Errorf("expected skillName 'my-skill', got %q", resp.Result.SkillName)
	}
	if resp.Summary.Total != 1 {
		t.Errorf("expected summary total 1, got %d", resp.Summary.Total)
	}
}
