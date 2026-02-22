package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleGetConfig(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["raw"] == nil || resp["raw"] == "" {
		t.Error("expected non-empty raw config")
	}
	if resp["config"] == nil {
		t.Error("expected config object in response")
	}
}

func TestHandleAvailableTargets(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/config/available-targets", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Targets []map[string]any `json:"targets"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Targets) == 0 {
		t.Error("expected at least 1 available target")
	}
}

func TestHandlePutConfig_InvalidJSON(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader("not json"))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlePutConfig_InvalidYAML(t *testing.T) {
	s, _ := newTestServer(t)
	body := `{"raw":"invalid: [yaml: content"}`
	req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlePutConfig_ValidYAML(t *testing.T) {
	s, _ := newTestServer(t)
	body := `{"raw":"source: /tmp/test\nmode: merge\ntargets: {}\n"}`
	req := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(body))
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
}
