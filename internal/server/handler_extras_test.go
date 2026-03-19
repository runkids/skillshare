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
)

// newTestServerWithExtras creates a test server with pre-configured extras
// persisted in the config file on disk.
// It creates source directories so handlers don't fail on "source not found".
func newTestServerWithExtras(t *testing.T, extras []config.ExtraConfig, extrasSource string) (*Server, string) {
	t.Helper()
	tmp := t.TempDir()
	sourceDir := filepath.Join(tmp, "skills")
	os.MkdirAll(sourceDir, 0755)
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmp, "state"))

	cfgPath := filepath.Join(tmp, "config", "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", cfgPath)
	os.MkdirAll(filepath.Dir(cfgPath), 0755)

	// Build a config with extras and save it, so the auto-reload middleware
	// can read extras back from disk.
	raw := "source: " + sourceDir + "\nmode: merge\ntargets: {}\n"
	if extrasSource != "" {
		raw += "extras_source: " + extrasSource + "\n"
	}
	os.WriteFile(cfgPath, []byte(raw), 0644)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	cfg.Extras = extras

	// Persist extras to disk so auto-reload picks them up.
	if err := cfg.Save(); err != nil {
		t.Fatalf("failed to save config with extras: %v", err)
	}

	// Create source directories for each extra so DiscoverExtraFiles succeeds.
	for _, extra := range extras {
		dir := config.ResolveExtrasSourceDir(extra, extrasSource, sourceDir)
		os.MkdirAll(dir, 0755)
	}

	s := New(cfg, "127.0.0.1:0", "", "")
	return s, sourceDir
}

// newTestProjectServerWithExtras creates a project-mode test server with extras.
func newTestProjectServerWithExtras(t *testing.T, extras []config.ExtraConfig) (*Server, string) {
	t.Helper()
	tmp := t.TempDir()

	// Global config (required by server)
	sourceDir := filepath.Join(tmp, "skills")
	os.MkdirAll(sourceDir, 0755)
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmp, "state"))

	cfgPath := filepath.Join(tmp, "config", "config.yaml")
	t.Setenv("SKILLSHARE_CONFIG", cfgPath)
	os.MkdirAll(filepath.Dir(cfgPath), 0755)
	os.WriteFile(cfgPath, []byte("source: "+sourceDir+"\nmode: merge\ntargets: {}\n"), 0644)

	globalCfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load global config: %v", err)
	}

	// Project root with .skillshare/config.yaml
	projectRoot := filepath.Join(tmp, "project")
	projectCfgDir := filepath.Join(projectRoot, ".skillshare")
	os.MkdirAll(projectCfgDir, 0755)

	projectCfg := &config.ProjectConfig{
		Extras: extras,
	}
	if err := projectCfg.Save(projectRoot); err != nil {
		t.Fatalf("failed to save project config: %v", err)
	}

	// Create source directories for project extras.
	for _, extra := range extras {
		dir := config.ExtrasSourceDirProject(projectRoot, extra.Name)
		os.MkdirAll(dir, 0755)
	}

	s := NewProject(globalCfg, projectCfg, projectRoot, "127.0.0.1:0", "", "")
	return s, projectRoot
}

// TestHandleExtras_IncludesSourceType verifies GET /api/extras returns
// the source_type field with correct values based on per-extra source
// and global extras_source settings.
func TestHandleExtras_IncludesSourceType(t *testing.T) {
	customSourceDir := filepath.Join(t.TempDir(), "custom-rules")

	extras := []config.ExtraConfig{
		{
			Name:   "rules",
			Source: customSourceDir,
			Targets: []config.ExtraTargetConfig{
				{Path: t.TempDir(), Mode: "merge"},
			},
		},
		{
			Name: "commands",
			// No per-extra source — should fall back to default
			Targets: []config.ExtraTargetConfig{
				{Path: t.TempDir(), Mode: "merge"},
			},
		},
	}

	s, _ := newTestServerWithExtras(t, extras, "")

	req := httptest.NewRequest(http.MethodGet, "/api/extras", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Extras []struct {
			Name       string `json:"name"`
			SourceType string `json:"source_type"`
		} `json:"extras"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(resp.Extras) != 2 {
		t.Fatalf("expected 2 extras, got %d", len(resp.Extras))
	}

	// "rules" has per-extra Source set -> "per-extra"
	if resp.Extras[0].Name != "rules" {
		t.Errorf("expected first extra name 'rules', got %q", resp.Extras[0].Name)
	}
	if resp.Extras[0].SourceType != "per-extra" {
		t.Errorf("expected source_type 'per-extra' for rules, got %q", resp.Extras[0].SourceType)
	}

	// "commands" has no per-extra Source and no extras_source -> "default"
	if resp.Extras[1].Name != "commands" {
		t.Errorf("expected second extra name 'commands', got %q", resp.Extras[1].Name)
	}
	if resp.Extras[1].SourceType != "default" {
		t.Errorf("expected source_type 'default' for commands, got %q", resp.Extras[1].SourceType)
	}
}

// TestHandleExtras_IncludesSourceType_ExtrasSource verifies that when
// extras_source is set globally and the extra has no per-extra source,
// source_type is "extras_source".
func TestHandleExtras_IncludesSourceType_ExtrasSource(t *testing.T) {
	extrasSourceDir := filepath.Join(t.TempDir(), "shared-extras")

	extras := []config.ExtraConfig{
		{
			Name: "hooks",
			Targets: []config.ExtraTargetConfig{
				{Path: t.TempDir(), Mode: "merge"},
			},
		},
	}

	s, _ := newTestServerWithExtras(t, extras, extrasSourceDir)

	req := httptest.NewRequest(http.MethodGet, "/api/extras", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Extras []struct {
			Name       string `json:"name"`
			SourceType string `json:"source_type"`
		} `json:"extras"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(resp.Extras) != 1 {
		t.Fatalf("expected 1 extra, got %d", len(resp.Extras))
	}

	if resp.Extras[0].SourceType != "extras_source" {
		t.Errorf("expected source_type 'extras_source', got %q", resp.Extras[0].SourceType)
	}
}

// TestHandleExtrasCreate_WithSource verifies POST /api/extras accepts
// a "source" field and persists it in the created ExtraConfig.
func TestHandleExtrasCreate_WithSource(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")

	customSource := filepath.Join(t.TempDir(), "custom-src")
	targetDir := t.TempDir()
	body := `{"name":"rules","source":"` + customSource + `","targets":[{"path":"` + targetDir + `","mode":"merge"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/extras", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Success bool   `json:"success"`
		Name    string `json:"name"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}

	// Verify the extra was added to config with Source set.
	s.mu.RLock()
	extras := s.cfg.Extras
	s.mu.RUnlock()

	found := false
	for _, e := range extras {
		if e.Name == "rules" {
			found = true
			if e.Source != customSource {
				t.Errorf("expected Source %q, got %q", customSource, e.Source)
			}
			break
		}
	}
	if !found {
		t.Error("extra 'rules' not found in config after create")
	}
}

// TestHandleExtrasCreate_WithoutSource verifies POST /api/extras works
// without a "source" field (backward compatibility).
func TestHandleExtrasCreate_WithoutSource(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")

	targetDir := t.TempDir()
	body := `{"name":"commands","targets":[{"path":"` + targetDir + `","mode":"merge"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/extras", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}

	// Verify the extra was added with empty Source.
	s.mu.RLock()
	extras := s.cfg.Extras
	s.mu.RUnlock()

	found := false
	for _, e := range extras {
		if e.Name == "commands" {
			found = true
			if e.Source != "" {
				t.Errorf("expected empty Source, got %q", e.Source)
			}
			break
		}
	}
	if !found {
		t.Error("extra 'commands' not found in config after create")
	}
}

// TestHandleExtras_ProjectMode_DefaultSourceType verifies that in project
// mode, extras without per-extra Source have source_type "default", since
// project mode does not use extras_source. An extra with Source set will
// still report "per-extra" because ResolveExtrasSourceType checks that first.
func TestHandleExtras_ProjectMode_DefaultSourceType(t *testing.T) {
	extras := []config.ExtraConfig{
		{
			Name: "rules",
			// No per-extra source -> "default" in project mode
			Targets: []config.ExtraTargetConfig{
				{Path: t.TempDir(), Mode: "merge"},
			},
		},
		{
			Name: "hooks",
			Targets: []config.ExtraTargetConfig{
				{Path: t.TempDir(), Mode: "merge"},
			},
		},
	}

	s, _ := newTestProjectServerWithExtras(t, extras)

	req := httptest.NewRequest(http.MethodGet, "/api/extras", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Extras []struct {
			Name       string `json:"name"`
			SourceType string `json:"source_type"`
		} `json:"extras"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(resp.Extras) != 2 {
		t.Fatalf("expected 2 extras, got %d", len(resp.Extras))
	}

	for _, entry := range resp.Extras {
		if entry.SourceType != "default" {
			t.Errorf("expected source_type 'default' for %q in project mode, got %q", entry.Name, entry.SourceType)
		}
	}
}

// TestHandleExtrasCreate_BackfillsExtrasSource verifies that POST /api/extras
// auto-populates extras_source in config when it's missing.
func TestHandleExtrasCreate_BackfillsExtrasSource(t *testing.T) {
	// Create server with NO extras_source set
	s, sourceDir := newTestServerWithExtras(t, nil, "")

	// Verify extras_source is empty before create
	if s.cfg.ExtrasSource != "" {
		t.Fatalf("precondition: expected empty ExtrasSource, got %s", s.cfg.ExtrasSource)
	}

	targetDir := t.TempDir()
	body := `{"name":"rules","targets":[{"path":"` + targetDir + `","mode":"merge"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/extras", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Reload config from disk and verify extras_source was backfilled
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to reload config: %v", err)
	}
	expected := config.ExtrasParentDir(sourceDir)
	if cfg.ExtrasSource != expected {
		t.Errorf("expected ExtrasSource %q, got %q", expected, cfg.ExtrasSource)
	}
}
