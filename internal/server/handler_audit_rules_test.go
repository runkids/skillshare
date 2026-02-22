package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleGetAuditRules_NotExist(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/audit/rules", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Exists bool   `json:"exists"`
		Raw    string `json:"raw"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Exists {
		t.Error("expected exists false for non-existent rules")
	}
}

func TestHandleInitAuditRules(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/audit/rules", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Success bool `json:"success"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if !resp.Success {
		t.Error("expected success true")
	}

	// Verify rules now exist
	req2 := httptest.NewRequest(http.MethodGet, "/api/audit/rules", nil)
	rr2 := httptest.NewRecorder()
	s.handler.ServeHTTP(rr2, req2)

	var resp2 struct {
		Exists bool `json:"exists"`
	}
	json.Unmarshal(rr2.Body.Bytes(), &resp2)
	if !resp2.Exists {
		t.Error("expected rules to exist after init")
	}
}

func TestHandleInitAuditRules_AlreadyExists(t *testing.T) {
	s, _ := newTestServer(t)
	// Init once
	req := httptest.NewRequest(http.MethodPost, "/api/audit/rules", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("first init failed: %d: %s", rr.Code, rr.Body.String())
	}

	// Init again — should conflict
	req2 := httptest.NewRequest(http.MethodPost, "/api/audit/rules", nil)
	rr2 := httptest.NewRecorder()
	s.handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusConflict {
		t.Errorf("expected 409 conflict, got %d: %s", rr2.Code, rr2.Body.String())
	}
}

func TestHandlePutAuditRules_InvalidYAML(t *testing.T) {
	s, _ := newTestServer(t)
	body := `{"raw":"not: [valid yaml rules"}`
	req := httptest.NewRequest(http.MethodPut, "/api/audit/rules", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
