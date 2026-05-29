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

// TestHandleExtrasCreate_WithExtension verifies that creating an extra with a
// target extension persists the extension and forces the target into copy mode,
// even when the request asks for a different mode.
func TestHandleExtrasCreate_WithExtension(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")

	targetDir := t.TempDir()
	// Request mode "merge" but with an extension — should be coerced to copy.
	body := `{"name":"agents","targets":[{"path":"` + targetDir + `","mode":"merge","extension":"codex-agents"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/extras", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	s.mu.RLock()
	extras := s.cfg.Extras
	s.mu.RUnlock()

	var found bool
	for _, e := range extras {
		if e.Name != "agents" {
			continue
		}
		found = true
		if len(e.Targets) != 1 {
			t.Fatalf("expected 1 target, got %d", len(e.Targets))
		}
		tt := e.Targets[0]
		if tt.Extension != "codex-agents" {
			t.Errorf("extension = %q, want codex-agents", tt.Extension)
		}
		if tt.Mode != "copy" {
			t.Errorf("mode = %q, want copy (extension implies copy)", tt.Mode)
		}
	}
	if !found {
		t.Error("extra 'agents' not found in config after create")
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

// writeExtension creates a directory-form extension under dir/<name>/.
func writeExtension(t *testing.T, dir, name string) {
	t.Helper()
	extDir := filepath.Join(dir, name)
	if err := os.MkdirAll(extDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extDir, "extension.yaml"), []byte("run: [\"cat\"]\noutput_ext: toml\n"), 0644); err != nil {
		t.Fatal(err)
	}
}

// TestHandleExtrasExtensions verifies GET /api/extras/extensions lists the
// extensions installed under the global extensions directory.
func TestHandleExtrasExtensions(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")

	extRoot := filepath.Join(filepath.Dir(config.ConfigPath()), "extensions")
	writeExtension(t, extRoot, "codex-agents")
	writeExtension(t, extRoot, "gemini-commands")

	req := httptest.NewRequest(http.MethodGet, "/api/extras/extensions", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Extensions []string `json:"extensions"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(resp.Extensions) != 2 || resp.Extensions[0] != "codex-agents" || resp.Extensions[1] != "gemini-commands" {
		t.Errorf("got %v, want [codex-agents gemini-commands]", resp.Extensions)
	}
}

// TestHandleExtrasExtensions_ProjectMode verifies the endpoint reads the
// project extensions directory (.skillshare/extensions) in project mode.
func TestHandleExtrasExtensions_ProjectMode(t *testing.T) {
	s, projectRoot := newTestProjectServerWithExtras(t, nil)

	extRoot := filepath.Join(projectRoot, ".skillshare", "extensions")
	writeExtension(t, extRoot, "codex-agents")

	req := httptest.NewRequest(http.MethodGet, "/api/extras/extensions", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Extensions []string `json:"extensions"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(resp.Extensions) != 1 || resp.Extensions[0] != "codex-agents" {
		t.Errorf("got %v, want [codex-agents]", resp.Extensions)
	}
}

// TestHandleExtrasMode_ExtensionImpliesCopy verifies that setting an extension
// on a target persists it and forces the mode to copy.
func TestHandleExtrasMode_ExtensionImpliesCopy(t *testing.T) {
	targetDir := t.TempDir()
	extras := []config.ExtraConfig{{
		Name:    "agents",
		Targets: []config.ExtraTargetConfig{{Path: targetDir, Mode: "merge"}},
	}}
	s, _ := newTestServerWithExtras(t, extras, "")

	body := `{"target":"` + targetDir + `","extension":"codex-agents"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/extras/agents/mode", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	s.mu.RLock()
	tt := s.cfg.Extras[0].Targets[0]
	s.mu.RUnlock()
	if tt.Extension != "codex-agents" {
		t.Errorf("extension = %q, want codex-agents", tt.Extension)
	}
	if tt.Mode != "copy" {
		t.Errorf("mode = %q, want copy (extension implies copy)", tt.Mode)
	}
}

// TestHandleExtrasMode_ExtensionRejectsNonCopy verifies that an explicit
// non-copy mode combined with an extension is rejected.
func TestHandleExtrasMode_ExtensionRejectsNonCopy(t *testing.T) {
	targetDir := t.TempDir()
	extras := []config.ExtraConfig{{
		Name:    "agents",
		Targets: []config.ExtraTargetConfig{{Path: targetDir, Mode: "merge"}},
	}}
	s, _ := newTestServerWithExtras(t, extras, "")

	body := `{"target":"` + targetDir + `","mode":"symlink","extension":"codex-agents"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/extras/agents/mode", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code == http.StatusOK {
		t.Fatalf("expected non-200 for extension + symlink mode, got 200")
	}
}

// TestHandleExtras_IncludesExtension verifies GET /api/extras surfaces the
// per-target extension value.
func TestHandleExtras_IncludesExtension(t *testing.T) {
	targetDir := t.TempDir()
	extras := []config.ExtraConfig{{
		Name:    "agents",
		Targets: []config.ExtraTargetConfig{{Path: targetDir, Mode: "copy", Extension: "codex-agents"}},
	}}
	s, _ := newTestServerWithExtras(t, extras, "")

	req := httptest.NewRequest(http.MethodGet, "/api/extras", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Extras []struct {
			Targets []struct {
				Extension string `json:"extension"`
			} `json:"targets"`
		} `json:"extras"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(resp.Extras) != 1 || len(resp.Extras[0].Targets) != 1 {
		t.Fatalf("unexpected shape: %s", rr.Body.String())
	}
	if resp.Extras[0].Targets[0].Extension != "codex-agents" {
		t.Errorf("extension = %q, want codex-agents", resp.Extras[0].Targets[0].Extension)
	}
}

// TestHandleExtrasCreate_PreservesEmptyExtrasSource verifies that POST /api/extras
// does NOT silently write the legacy extras_source field when the user has
// not explicitly set it. The runtime fallback is provided by
// Config.EffectiveExtrasSource(), so backfilling on save would surprise
// users with an unexpected diff in their config.yaml.
func TestHandleExtrasCreate_PreservesEmptyExtrasSource(t *testing.T) {
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

	// Reload config from disk and verify extras_source is still empty —
	// no silent backfill — and that EffectiveExtrasSource() resolves to
	// the derived default.
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to reload config: %v", err)
	}
	if cfg.ExtrasSource != "" {
		t.Errorf("expected empty ExtrasSource (no silent backfill), got %q", cfg.ExtrasSource)
	}
	if cfg.Sources.Extras != "" {
		t.Errorf("expected empty Sources.Extras (no silent backfill), got %q", cfg.Sources.Extras)
	}
	expected := config.ExtrasParentDir(sourceDir)
	if got := cfg.EffectiveExtrasSource(); got != expected {
		t.Errorf("expected EffectiveExtrasSource %q (derived), got %q", expected, got)
	}
}
