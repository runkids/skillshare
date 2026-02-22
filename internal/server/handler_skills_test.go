package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleListSkills_Empty(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/skills", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Skills []any `json:"skills"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(resp.Skills))
	}
}

func TestHandleListSkills_WithSkills(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "alpha")
	addSkill(t, src, "beta")

	req := httptest.NewRequest(http.MethodGet, "/api/skills", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Skills []map[string]any `json:"skills"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(resp.Skills))
	}
}

func TestHandleGetSkill_Found(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "my-skill")

	req := httptest.NewRequest(http.MethodGet, "/api/skills/my-skill", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	skill := resp["skill"].(map[string]any)
	if skill["flatName"] != "my-skill" {
		t.Errorf("expected flatName 'my-skill', got %v", skill["flatName"])
	}
}

func TestHandleGetSkill_NotFound(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/skills/nonexistent", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestHandleGetSkillFile_PathTraversal(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "my-skill")

	// Go's HTTP mux cleans ".." from URL paths before routing, so we need
	// to bypass mux and call the handler directly with a crafted PathValue.
	// Instead, test that a valid-looking but still-traversal path is rejected.
	// The handler checks strings.Contains(fp, "..").
	req := httptest.NewRequest(http.MethodGet, "/api/skills/my-skill/files/sub%2F..%2F..%2Fetc%2Fpasswd", nil)
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	// The mux will decode %2F as / and clean the path, which may result in
	// 404 or the handler never seeing "..". Either non-200 is acceptable.
	if rr.Code == http.StatusOK {
		t.Error("expected non-200 for path traversal attempt")
	}
}
