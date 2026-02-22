package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleHealth_StatusOK(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", resp["status"])
	}
}

func TestHandleHealth_Uptime(t *testing.T) {
	s, _ := newTestServer(t)
	// startTime is zero by default (server not started), so uptime should be 0
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	uptime, ok := resp["uptime_seconds"].(float64)
	if !ok {
		t.Fatal("expected uptime_seconds to be a number")
	}
	if uptime != 0 {
		t.Errorf("expected uptime 0 for non-started server, got %v", uptime)
	}
}
