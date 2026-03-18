package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
)

func TestNormalizeBasePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"/", ""},
		{"/foo", "/foo"},
		{"/foo/", "/foo"},
		{"foo", "/foo"},
		{"foo/", "/foo"},
		{"/a/b/c", "/a/b/c"},
		{"/a/b/c/", "/a/b/c"},
		{"a/b", "/a/b"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeBasePath(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeBasePath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func newTestServerWithBasePath(t *testing.T, basePath string) *Server {
	t.Helper()
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "skills")
	os.MkdirAll(sourceDir, 0755)
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmp, "state"))
	cfgPath := filepath.Join(tmp, "config", "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", cfgPath)
	os.MkdirAll(filepath.Dir(cfgPath), 0755)
	raw := "source: " + sourceDir + "\nmode: merge\ntargets: {}\n"
	os.WriteFile(cfgPath, []byte(raw), 0644)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	// Create a fake UI dist dir so wrapBasePath doesn't skip
	uiDir := filepath.Join(tmp, "uidist")
	os.MkdirAll(uiDir, 0755)
	os.WriteFile(filepath.Join(uiDir, "index.html"), []byte("<!DOCTYPE html><html><head><title>Test</title></head><body>Skillshare UI</body></html>"), 0644)
	s := New(cfg, "127.0.0.1:0", basePath, uiDir)
	return s
}

func TestBasePath_APIHealthWithPrefix(t *testing.T) {
	s := newTestServerWithBasePath(t, "/app")
	req := httptest.NewRequest(http.MethodGet, "/app/api/health", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /app/api/health: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestBasePath_APIHealthWithoutPrefix_Returns404(t *testing.T) {
	s := newTestServerWithBasePath(t, "/app")
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("GET /api/health with basePath=/app: expected 404, got %d", rr.Code)
	}
}

func TestBasePath_BarePathRedirect(t *testing.T) {
	s := newTestServerWithBasePath(t, "/app")
	req := httptest.NewRequest(http.MethodGet, "/app", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusMovedPermanently {
		t.Fatalf("GET /app: expected 301, got %d", rr.Code)
	}
	loc := rr.Header().Get("Location")
	if loc != "/app/" {
		t.Errorf("expected redirect to /app/, got %s", loc)
	}
}

func TestBasePath_RootServesIndex(t *testing.T) {
	s := newTestServerWithBasePath(t, "/app")
	req := httptest.NewRequest(http.MethodGet, "/app/", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /app/: expected 200, got %d", rr.Code)
	}
}

func TestBasePath_Empty_NoStripPrefix(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/health without basePath: expected 200, got %d", rr.Code)
	}
}

func TestBasePath_MultiLevel(t *testing.T) {
	s := newTestServerWithBasePath(t, "/tools/skillshare")
	req := httptest.NewRequest(http.MethodGet, "/tools/skillshare/api/health", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /tools/skillshare/api/health: expected 200, got %d", rr.Code)
	}
}

func TestBasePath_IndexHtmlInjection(t *testing.T) {
	tmp := t.TempDir()
	indexHTML := `<!DOCTYPE html><html><head><title>Test</title></head><body></body></html>`
	os.WriteFile(filepath.Join(tmp, "index.html"), []byte(indexHTML), 0644)

	handler := spaHandlerFromDisk(tmp, "/app")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, `window.__BASE_PATH__="/app"`) {
		t.Errorf("expected __BASE_PATH__ injection, got:\n%s", body)
	}
}

func TestBasePath_IndexHtmlNoInjectionWhenEmpty(t *testing.T) {
	tmp := t.TempDir()
	indexHTML := `<!DOCTYPE html><html><head><title>Test</title></head><body></body></html>`
	os.WriteFile(filepath.Join(tmp, "index.html"), []byte(indexHTML), 0644)

	handler := spaHandlerFromDisk(tmp, "")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	body := rr.Body.String()
	if strings.Contains(body, "__BASE_PATH__") {
		t.Error("expected no __BASE_PATH__ injection when basePath is empty")
	}
}

func TestBasePath_SPAFallbackWithInjection(t *testing.T) {
	tmp := t.TempDir()
	indexHTML := `<!DOCTYPE html><html><head><title>Test</title></head><body></body></html>`
	os.WriteFile(filepath.Join(tmp, "index.html"), []byte(indexHTML), 0644)

	handler := spaHandlerFromDisk(tmp, "/myapp")
	// Request a non-existent path — should serve injected index.html (SPA fallback)
	req := httptest.NewRequest(http.MethodGet, "/skills/my-skill", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, `window.__BASE_PATH__="/myapp"`) {
		t.Errorf("SPA fallback should inject __BASE_PATH__, got:\n%s", body)
	}
}
