package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleGetHubSaved_Default(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/hub/saved", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Hubs []any `json:"hubs"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	// Default config has no custom hubs
}

func TestHandlePostHubSaved_AddHub(t *testing.T) {
	s, _ := newTestServer(t)
	body := `{"label":"test-hub","url":"https://example.com/index.json"}`
	req := httptest.NewRequest(http.MethodPost, "/api/hub/saved", strings.NewReader(body))
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

func TestHandlePostHubSaved_InvalidJSON(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/hub/saved", strings.NewReader("not json"))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestHandleDeleteHubSaved_NotFound(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/hub/saved/nonexistent", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlePutHubSaved_ReplaceAll(t *testing.T) {
	s, _ := newTestServer(t)
	body := `{"hubs":[{"label":"new-hub","url":"https://example.com"}],"default":"new-hub"}`
	req := httptest.NewRequest(http.MethodPut, "/api/hub/saved", strings.NewReader(body))
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
