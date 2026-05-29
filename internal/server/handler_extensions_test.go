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
