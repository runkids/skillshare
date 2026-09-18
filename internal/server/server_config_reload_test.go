package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	s := New(cfg, "127.0.0.1:0", "", "")

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

func TestServer_ReadRequestProceedsDuringAdoptAndReloadsConfig(t *testing.T) {
	s, _ := newTestServer(t)
	sourceB := filepath.Join(t.TempDir(), "skills-b")
	if err := os.MkdirAll(sourceB, 0755); err != nil {
		t.Fatalf("failed to create replacement source: %v", err)
	}
	if err := os.WriteFile(config.ConfigPath(), []byte("source: "+sourceB+"\nmode: symlink\ntargets: {}\n"), 0644); err != nil {
		t.Fatalf("failed to rewrite config: %v", err)
	}

	// The exclusive gate represents an adopt apply already past body decoding.
	// Reads do not take the gate, so the routed request must still auto-reload
	// and complete while adoption owns it.
	s.adoptMu.Lock()
	defer s.adoptMu.Unlock()

	type response struct {
		Source string `json:"source"`
	}
	firstDone := make(chan struct{})
	firstRecorder := httptest.NewRecorder()
	go func() {
		defer close(firstDone)
		req := httptest.NewRequest(http.MethodGet, "/api/overview", nil)
		s.handler.ServeHTTP(firstRecorder, req)
	}()

	select {
	case <-firstDone:
	case <-time.After(2 * time.Second):
		t.Fatal("routed read request blocked behind an active adopt operation")
	}

	if firstRecorder.Code != http.StatusOK {
		t.Fatalf("unexpected status while config lock was busy: %d body=%s", firstRecorder.Code, firstRecorder.Body.String())
	}
	var first response
	if err := json.Unmarshal(firstRecorder.Body.Bytes(), &first); err != nil {
		t.Fatalf("failed to decode response while config lock was busy: %v", err)
	}
	if first.Source != sourceB {
		t.Fatalf("source during adopt = %q, want auto-reloaded source %q", first.Source, sourceB)
	}
}

func TestServer_MutationWaitsForAdoptThenReloadsConfig(t *testing.T) {
	s, _ := newTestServer(t)
	sourceB := filepath.Join(t.TempDir(), "skills-b")
	if err := os.MkdirAll(sourceB, 0755); err != nil {
		t.Fatalf("failed to create replacement source: %v", err)
	}
	if err := os.WriteFile(config.ConfigPath(), []byte("source: "+sourceB+"\nmode: symlink\ntargets: {}\n"), 0644); err != nil {
		t.Fatalf("failed to rewrite config: %v", err)
	}

	seenSource := make(chan string, 1)
	wrapped := s.withConfigAutoReload(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		s.mu.RLock()
		seenSource <- s.cfg.EffectiveSkillsSource()
		s.mu.RUnlock()
		w.WriteHeader(http.StatusNoContent)
	}))

	s.adoptMu.Lock()
	done := make(chan struct{})
	go func() {
		defer close(done)
		req := httptest.NewRequest(http.MethodPost, "/api/test-mutation", nil)
		wrapped.ServeHTTP(httptest.NewRecorder(), req)
	}()

	select {
	case <-done:
		s.adoptMu.Unlock()
		t.Fatal("mutating request bypassed the active adopt operation")
	case <-time.After(100 * time.Millisecond):
	}
	s.adoptMu.Unlock()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("mutating request did not resume after adopt completed")
	}
	if got := <-seenSource; got != sourceB {
		t.Fatalf("mutation used source %q, want freshly reloaded source %q", got, sourceB)
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
	s := New(cfg, "127.0.0.1:0", "", "")

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

func TestServerCloneTargetsDeepCopiesNestedConfig(t *testing.T) {
	s := &Server{cfg: &config.Config{Targets: map[string]config.TargetConfig{
		"claude": {
			Include: []string{"legacy-include"},
			Exclude: []string{"legacy-exclude"},
			Skills: &config.ResourceTargetConfig{
				Mode:    "merge",
				Include: []string{"skill-include"},
				Exclude: []string{"skill-exclude"},
			},
			Agents: &config.ResourceTargetConfig{
				Mode:    "copy",
				Include: []string{"agent-include"},
				Exclude: []string{"agent-exclude"},
			},
		},
	}}}

	s.mu.RLock()
	cloned := s.cloneTargets()
	s.mu.RUnlock()

	copy := cloned["claude"]
	copy.Include[0] = "changed"
	copy.Exclude[0] = "changed"
	copy.Skills.Mode = "copy"
	copy.Skills.Include[0] = "changed"
	copy.Skills.Exclude[0] = "changed"
	copy.Agents.Mode = "merge"
	copy.Agents.Include[0] = "changed"
	copy.Agents.Exclude[0] = "changed"
	delete(cloned, "claude")

	original := s.cfg.Targets["claude"]
	if original.Include[0] != "legacy-include" || original.Exclude[0] != "legacy-exclude" {
		t.Fatalf("legacy target slices were aliased: %+v", original)
	}
	if original.Skills.Mode != "merge" || original.Skills.Include[0] != "skill-include" || original.Skills.Exclude[0] != "skill-exclude" {
		t.Fatalf("skills config was aliased: %+v", original.Skills)
	}
	if original.Agents.Mode != "copy" || original.Agents.Include[0] != "agent-include" || original.Agents.Exclude[0] != "agent-exclude" {
		t.Fatalf("agents config was aliased: %+v", original.Agents)
	}
	if _, ok := s.cfg.Targets["claude"]; !ok {
		t.Fatal("cloned map shared ownership with server config")
	}
}
