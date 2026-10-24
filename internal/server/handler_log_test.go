package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
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

func TestHandleLog_SelectedGlobalConfig(t *testing.T) {
	s, _ := newTestServer(t)
	raw, err := os.ReadFile(config.ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	selected := filepath.Join(t.TempDir(), "shared", "skillshare", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(selected), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(selected, raw, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SKILLSHARE_CONFIG", selected)
	canonical := filepath.Join(config.BaseDir(), "config.yaml")
	if err := oplog.Write(canonical, oplog.OpsFile, oplog.Entry{Command: "project", Status: "ok"}); err != nil {
		t.Fatal(err)
	}
	for _, url := range []string{"/api/log?cmd=project", "/api/log/stats?cmd=project"} {
		rr := projectRequest(t, s, http.MethodGet, url, "")
		var payload struct {
			Total int `json:"total"`
		}
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", url, rr.Code, rr.Body)
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Total != 1 {
			t.Errorf("%s missed canonical project log: %s", url, rr.Body)
		}
	}
	rr := projectRequest(t, s, http.MethodDelete, "/api/log", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("clear: %d %s", rr.Code, rr.Body)
	}
	entries, err := oplog.Read(canonical, oplog.OpsFile, 0)
	if err != nil || len(entries) != 0 {
		t.Fatalf("clear missed canonical log: %v %v", entries, err)
	}
}
