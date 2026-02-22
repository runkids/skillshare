package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleOverview_Empty(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/overview", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["skillCount"].(float64) != 0 {
		t.Errorf("expected 0 skills, got %v", resp["skillCount"])
	}
	if resp["targetCount"].(float64) != 0 {
		t.Errorf("expected 0 targets, got %v", resp["targetCount"])
	}
	if resp["isProjectMode"].(bool) {
		t.Error("expected isProjectMode false")
	}
}

func TestHandleOverview_WithSkills(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "alpha")
	addSkill(t, src, "beta")

	req := httptest.NewRequest(http.MethodGet, "/api/overview", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["skillCount"].(float64) != 2 {
		t.Errorf("expected 2 skills, got %v", resp["skillCount"])
	}
}

func TestHandleOverview_ProjectMode(t *testing.T) {
	tmp := t.TempDir()
	s, _ := newTestServer(t)
	s.projectRoot = tmp // simulate project mode

	req := httptest.NewRequest(http.MethodGet, "/api/overview", nil)
	rr := httptest.NewRecorder()
	// Use mux directly to bypass config auto-reload middleware
	// (project config file doesn't exist in test)
	s.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if !resp["isProjectMode"].(bool) {
		t.Error("expected isProjectMode true")
	}
	if resp["projectRoot"] != tmp {
		t.Errorf("expected projectRoot %q, got %v", tmp, resp["projectRoot"])
	}
}
