package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestHandleListBackups_Empty(t *testing.T) {
	s, _ := newTestServer(t)
	// Isolate backup directory via XDG_DATA_HOME to avoid reading real system backups
	t.Setenv("XDG_DATA_HOME", filepath.Join(t.TempDir(), "data"))
	req := httptest.NewRequest(http.MethodGet, "/api/backups", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Backups []any `json:"backups"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Backups) != 0 {
		t.Errorf("expected 0 backups, got %d", len(resp.Backups))
	}
}
