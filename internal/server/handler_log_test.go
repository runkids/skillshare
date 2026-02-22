package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleListLog_Empty(t *testing.T) {
	s, _ := newTestServer(t)

	// Clear any pre-existing logs first
	clearReq := httptest.NewRequest(http.MethodDelete, "/api/log", nil)
	clearRR := httptest.NewRecorder()
	s.handler.ServeHTTP(clearRR, clearReq)

	req := httptest.NewRequest(http.MethodGet, "/api/log", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Entries []any `json:"entries"`
		Total   int   `json:"total"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(resp.Entries))
	}
	if resp.Total != 0 {
		t.Errorf("expected total 0, got %d", resp.Total)
	}
}

func TestHandleListLog_WithEntries(t *testing.T) {
	s, _ := newTestServer(t)

	// Generate a log entry by adding a target
	body := `{"name":"test","path":"/tmp/test"}`
	addReq := httptest.NewRequest(http.MethodPost, "/api/targets", strings.NewReader(body))
	addRR := httptest.NewRecorder()
	s.handler.ServeHTTP(addRR, addReq)

	// Now list logs
	req := httptest.NewRequest(http.MethodGet, "/api/log", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Entries []any `json:"entries"`
		Total   int   `json:"total"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Total == 0 {
		t.Error("expected at least 1 log entry after target add")
	}
}

func TestHandleClearLog(t *testing.T) {
	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/log", nil)
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
