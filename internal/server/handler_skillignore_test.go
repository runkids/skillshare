package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

// In project mode the route serves the project skills source; the handler now
// reads it through skillsSource() like the other source-reading routes.
func TestHandleGetSkillignore_ProjectModeUsesProjectSource(t *testing.T) {
	s, projectRoot := newTestProjectServerWithExtras(t, nil)

	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/skillignore", nil))

	var resp skillignoreResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v (body=%s)", err, rr.Body.String())
	}
	if !strings.HasPrefix(resp.Path, projectRoot+string(filepath.Separator)) {
		t.Fatalf("expected .skillignore under the project source %s, got %s", projectRoot, resp.Path)
	}
}
