package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
)

func TestServer_AutoReloadsConfigForAPIRequests(t *testing.T) {
	tmp := t.TempDir()
	sourceA := filepath.Join(tmp, "skills-a")
	sourceB := filepath.Join(tmp, "skills-b")
	if err := os.MkdirAll(sourceA, 0755); err != nil {
		t.Fatalf("failed to create sourceA: %v", err)
	}
	if err := os.MkdirAll(sourceB, 0755); err != nil {
		t.Fatalf("failed to create sourceB: %v", err)
	}

	cfgPath := filepath.Join(tmp, "config", "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", cfgPath)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	rawA := "source: " + sourceA + "\nmode: merge\ntargets: {}\n"
	if err := os.WriteFile(cfgPath, []byte(rawA), 0644); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load initial config: %v", err)
	}
	s := New(cfg, "127.0.0.1:0")

	req1 := httptest.NewRequest(http.MethodGet, "/api/overview", nil)
	rr1 := httptest.NewRecorder()
	s.handler.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Fatalf("unexpected status on first overview: %d body=%s", rr1.Code, rr1.Body.String())
	}

	var overview1 struct {
		Source string `json:"source"`
	}
	if err := json.Unmarshal(rr1.Body.Bytes(), &overview1); err != nil {
		t.Fatalf("failed to decode first overview: %v", err)
	}
	if overview1.Source != sourceA {
		t.Fatalf("expected first source %q, got %q", sourceA, overview1.Source)
	}

	rawB := "source: " + sourceB + "\nmode: symlink\ntargets: {}\n"
	if err := os.WriteFile(cfgPath, []byte(rawB), 0644); err != nil {
		t.Fatalf("failed to rewrite config: %v", err)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/overview", nil)
	rr2 := httptest.NewRecorder()
	s.handler.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("unexpected status on second overview: %d body=%s", rr2.Code, rr2.Body.String())
	}

	var overview2 struct {
		Source string `json:"source"`
		Mode   string `json:"mode"`
	}
	if err := json.Unmarshal(rr2.Body.Bytes(), &overview2); err != nil {
		t.Fatalf("failed to decode second overview: %v", err)
	}
	if overview2.Source != sourceB {
		t.Fatalf("expected reloaded source %q, got %q", sourceB, overview2.Source)
	}
	if overview2.Mode != "symlink" {
		t.Fatalf("expected reloaded mode symlink, got %q", overview2.Mode)
	}
}

func TestServer_ConfigEndpointStillWorksWhenConfigTemporarilyInvalid(t *testing.T) {
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "skills")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}

	cfgPath := filepath.Join(tmp, "config", "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", cfgPath)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	validRaw := "source: " + sourceDir + "\ntargets: {}\n"
	if err := os.WriteFile(cfgPath, []byte(validRaw), 0644); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load initial config: %v", err)
	}
	s := New(cfg, "127.0.0.1:0")

	invalidRaw := "source: [\n"
	if err := os.WriteFile(cfgPath, []byte(invalidRaw), 0644); err != nil {
		t.Fatalf("failed to write invalid config: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected /api/config to stay available, got %d body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Raw string `json:"raw"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode config response: %v", err)
	}
	if resp.Raw != invalidRaw {
		t.Fatalf("expected raw config to match file content")
	}
}
