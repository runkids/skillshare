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
	"skillshare/internal/install"
)

// writeExtensionWithDescription creates a directory-form extension carrying a
// description, so the management list can surface it.
func writeExtensionWithDescription(t *testing.T, dir, name, description string) {
	t.Helper()
	extDir := filepath.Join(dir, name)
	if err := os.MkdirAll(extDir, 0755); err != nil {
		t.Fatal(err)
	}
	manifest := "run: [\"cat\"]\noutput_ext: toml\ndescription: " + description + "\n"
	if err := os.WriteFile(filepath.Join(extDir, "extension.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
}

// TestHandleExtensionsList verifies GET /api/extensions merges installed
// extensions (with descriptions and a builtin flag) with the built-in catalog
// entries that are not yet installed.
func TestHandleExtensionsList(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")

	extRoot := filepath.Join(filepath.Dir(config.ConfigPath()), "extensions")
	// One installed built-in (codex-agents) and one installed local extension.
	writeExtensionWithDescription(t, extRoot, "codex-agents", "Markdown to Codex")
	writeExtensionWithDescription(t, extRoot, "my-local", "Local transform")

	req := httptest.NewRequest(http.MethodGet, "/api/extensions", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Extensions []extensionInfo `json:"extensions"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse: %v", err)
	}

	byName := make(map[string]extensionInfo, len(resp.Extensions))
	for _, e := range resp.Extensions {
		byName[e.Name] = e
	}

	// Installed built-in: installed + builtin + catalog description (the
	// catalog wins over the on-disk extension.yaml for built-ins).
	if got, ok := byName["codex-agents"]; !ok {
		t.Error("codex-agents missing from list")
	} else if !got.Installed || !got.Builtin || got.Description != install.BuiltinExtensionDescription("codex-agents") {
		t.Errorf("codex-agents = %+v, want installed builtin with catalog description", got)
	}

	// Installed local extension: installed, NOT builtin.
	if got, ok := byName["my-local"]; !ok {
		t.Error("my-local missing from list")
	} else if !got.Installed || got.Builtin {
		t.Errorf("my-local = %+v, want installed non-builtin", got)
	}

	// Built-in not installed yet (gemini-commands): available to download.
	if got, ok := byName["gemini-commands"]; !ok {
		t.Error("gemini-commands missing from list (should appear as available built-in)")
	} else if got.Installed || !got.Builtin {
		t.Errorf("gemini-commands = %+v, want builtin and not installed", got)
	}
}

// TestHandleExtensionsInstall_RejectsUnknown verifies the install endpoint
// only accepts whitelisted built-in extension names.
func TestHandleExtensionsInstall_RejectsUnknown(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")

	req := httptest.NewRequest(http.MethodPost, "/api/extensions/install",
		strings.NewReader(`{"name":"../etc/passwd"}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown extension, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestHandleExtensionsList_ReportsUsage verifies an installed extension reports
// the extras that reference it, so the UI can warn before removal.
func TestHandleExtensionsList_ReportsUsage(t *testing.T) {
	extras := []config.ExtraConfig{{
		Name:    "agents",
		Targets: []config.ExtraTargetConfig{{Path: t.TempDir(), Mode: "copy", Extension: "to-toml"}},
	}}
	s, _ := newTestServerWithExtras(t, extras, "")
	extRoot := filepath.Join(filepath.Dir(config.ConfigPath()), "extensions")
	writeExtensionWithDescription(t, extRoot, "to-toml", "identity")

	req := httptest.NewRequest(http.MethodGet, "/api/extensions", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Extensions []extensionInfo `json:"extensions"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse: %v", err)
	}
	var found *extensionInfo
	for i := range resp.Extensions {
		if resp.Extensions[i].Name == "to-toml" {
			found = &resp.Extensions[i]
		}
	}
	if found == nil {
		t.Fatal("to-toml missing from list")
	}
	if len(found.UsedBy) != 1 || found.UsedBy[0] != "agents" {
		t.Errorf("UsedBy = %v, want [agents]", found.UsedBy)
	}
}

// TestHandleExtensionsRemove verifies removal of an installed extension and that
// path-traversal names are rejected.
func TestHandleExtensionsRemove(t *testing.T) {
	s, _ := newTestServerWithExtras(t, nil, "")
	extRoot := filepath.Join(filepath.Dir(config.ConfigPath()), "extensions")
	writeExtensionWithDescription(t, extRoot, "to-toml", "identity")

	// Traversal name rejected by the handler guard. ServeMux cleans ".." out of
	// URL paths, so call the handler directly with a crafted path value to
	// exercise the defense-in-depth check.
	bad := httptest.NewRequest(http.MethodDelete, "/api/extensions/x", nil)
	bad.SetPathValue("name", "../evil")
	br := httptest.NewRecorder()
	s.handleExtensionsRemove(br, bad)
	if br.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for traversal name, got %d", br.Code)
	}

	// Successful removal.
	req := httptest.NewRequest(http.MethodDelete, "/api/extensions/to-toml", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if _, err := os.Stat(filepath.Join(extRoot, "to-toml")); err == nil {
		t.Error("extension directory still exists after removal")
	}

	// Removing a non-existent extension is a 404.
	missing := httptest.NewRequest(http.MethodDelete, "/api/extensions/to-toml", nil)
	mr := httptest.NewRecorder()
	s.handler.ServeHTTP(mr, missing)
	if mr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing extension, got %d", mr.Code)
	}
}
