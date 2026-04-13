package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
)

func TestHandleManagedCapabilities_ReturnsFamiliesAndTargets(t *testing.T) {
	tmp := t.TempDir()
	s, _ := newTestServerWithTargets(t, map[string]string{
		"pi": filepath.Join(tmp, "home", ".pi", "agent", "skills"),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/managed/capabilities", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Families map[string]struct {
			SupportsRules bool `json:"SupportsRules"`
			SupportsHooks bool `json:"SupportsHooks"`
		} `json:"Families"`
		Targets map[string]struct {
			RulesFamily string `json:"rulesFamily"`
			HooksFamily string `json:"hooksFamily"`
		} `json:"Targets"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	pi, ok := resp.Families["pi"]
	if !ok {
		t.Fatal("expected pi family in response")
	}
	if !pi.SupportsRules || pi.SupportsHooks {
		t.Fatalf("pi family = %#v, want rules-only support", pi)
	}

	piTarget, ok := resp.Targets["pi"]
	if !ok {
		t.Fatal("expected pi target in response")
	}
	if piTarget.RulesFamily != "pi" || piTarget.HooksFamily != "" {
		t.Fatalf("pi target = %#v, want rules family only", piTarget)
	}
	if _, ok := resp.Targets["claude"]; ok {
		t.Fatalf("configured targets = %#v, want unrelated default targets excluded", resp.Targets)
	}
}

func TestHandleManagedCapabilities_UsesConfiguredTargets(t *testing.T) {
	s, _, _, _ := newManagedProjectServer(t, "claude-code")

	req := httptest.NewRequest(http.MethodGet, "/api/managed/capabilities", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Targets map[string]struct {
			RulesFamily string `json:"rulesFamily"`
			HooksFamily string `json:"hooksFamily"`
		} `json:"Targets"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	custom, ok := resp.Targets["claude-code"]
	if !ok {
		t.Fatalf("configured targets = %#v, want claude-code entry", resp.Targets)
	}
	if custom.RulesFamily != "claude" || custom.HooksFamily != "claude" {
		t.Fatalf("claude-code target = %#v, want claude rules/hooks family", custom)
	}
	if _, ok := resp.Targets["pi"]; ok {
		t.Fatalf("configured targets = %#v, want built-in defaults excluded", resp.Targets)
	}
	if _, ok := resp.Targets["claude"]; ok {
		t.Fatalf("configured targets = %#v, want configured alias name preserved", resp.Targets)
	}
}

func TestHandleManagedCapabilities_UsesNestedSkillsConfigPath(t *testing.T) {
	s, _ := newTestServer(t)

	s.mu.Lock()
	s.cfg.Targets = map[string]config.TargetConfig{
		"my-codex": {
			Skills: &config.ResourceTargetConfig{
				Path: filepath.Join(t.TempDir(), "home", ".agents", "skills"),
			},
		},
	}
	if err := s.cfg.Save(); err != nil {
		s.mu.Unlock()
		t.Fatalf("save config: %v", err)
	}
	s.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/managed/capabilities", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Targets map[string]struct {
			RulesFamily string `json:"rulesFamily"`
			HooksFamily string `json:"hooksFamily"`
		} `json:"Targets"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	custom, ok := resp.Targets["my-codex"]
	if !ok {
		t.Fatalf("configured targets = %#v, want my-codex entry", resp.Targets)
	}
	if custom.RulesFamily != "codex" || custom.HooksFamily != "codex" {
		t.Fatalf("my-codex target = %#v, want codex rules/hooks family", custom)
	}
	if _, ok := resp.Targets["pi"]; ok {
		t.Fatalf("configured targets = %#v, want built-in defaults excluded", resp.Targets)
	}
}
