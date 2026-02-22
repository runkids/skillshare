package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleCheck_EmptySource(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/check", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		TrackedRepos []any `json:"tracked_repos"`
		Skills       []any `json:"skills"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.TrackedRepos) != 0 {
		t.Errorf("expected 0 tracked repos, got %d", len(resp.TrackedRepos))
	}
	if len(resp.Skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(resp.Skills))
	}
}
