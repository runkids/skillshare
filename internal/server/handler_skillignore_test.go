package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHandleGetSkillignore_IncludesLocalFile(t *testing.T) {
	s, src := newTestServer(t)
	if err := os.WriteFile(filepath.Join(src, ".skillignore.local"), []byte("!feature-radar\n"), 0644); err != nil {
		t.Fatalf("seed .skillignore.local: %v", err)
	}

	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/skillignore", nil))

	var resp skillignoreResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v (body=%s)", err, rr.Body.String())
	}
	if resp.Local == nil || resp.Local.Raw != "!feature-radar\n" {
		t.Fatalf("expected local file contents, got %+v", resp.Local)
	}
}

func TestHandleGetSkillignore_NoLocalFile(t *testing.T) {
	s, _ := newTestServer(t)

	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/skillignore", nil))

	var resp skillignoreResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v (body=%s)", err, rr.Body.String())
	}
	if resp.Local != nil {
		t.Fatalf("expected no local file, got %+v", resp.Local)
	}
}
