package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestHandleDiff_Empty(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/diff", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Diffs []any `json:"diffs"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Diffs) != 0 {
		t.Errorf("expected 0 diffs, got %d", len(resp.Diffs))
	}
}

func TestHandleDiff_WithTarget(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, src := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})
	addSkill(t, src, "alpha")

	req := httptest.NewRequest(http.MethodGet, "/api/diff", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Diffs []map[string]any `json:"diffs"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Diffs) != 1 {
		t.Fatalf("expected 1 diff target, got %d", len(resp.Diffs))
	}
}
