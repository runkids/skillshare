package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
)

func TestHandleListTargets_Empty(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/targets", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Targets []any `json:"targets"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Targets) != 0 {
		t.Errorf("expected 0 targets, got %d", len(resp.Targets))
	}
}

func TestHandleListTargets_WithTargets(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, _ := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})

	req := httptest.NewRequest(http.MethodGet, "/api/targets", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Targets []map[string]any `json:"targets"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(resp.Targets))
	}
	if resp.Targets[0]["name"] != "claude" {
		t.Errorf("expected target name 'claude', got %v", resp.Targets[0]["name"])
	}
}

func TestHandleAddTarget_Success(t *testing.T) {
	s, _ := newTestServer(t)
	body := `{"name":"test-target","path":"/tmp/test-target"}`
	req := httptest.NewRequest(http.MethodPost, "/api/targets", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["success"] != true {
		t.Error("expected success true")
	}
}

func TestHandleAddTarget_SavesInstructionFile(t *testing.T) {
	s, _ := newTestServer(t)
	body := `{"name":"myagent","path":"/tmp/myagent/skills","instructions":{"path":"~/.myagent/AGENTS.md","import":true}}`
	req := httptest.NewRequest(http.MethodPost, "/api/targets", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := s.cfg.Targets["myagent"].Instructions; got == nil || got.Path != "~/.myagent/AGENTS.md" || !got.Import {
		t.Errorf("instructions = %+v, want ~/.myagent/AGENTS.md with import", got)
	}
}

func TestHandleAddTarget_RejectsRelativeInstructionFile(t *testing.T) {
	s, _ := newTestServer(t)
	body := `{"name":"myagent","path":"/tmp/myagent/skills","instructions":{"path":"AGENTS.md"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/targets", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
	if _, exists := s.cfg.Targets["myagent"]; exists {
		t.Error("target was added despite the invalid instruction file")
	}
}

func TestHandleAddTarget_MissingName(t *testing.T) {
	s, _ := newTestServer(t)
	body := `{"path":"/tmp/test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/targets", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing name, got %d", rr.Code)
	}
}

func TestHandleAddTarget_Duplicate(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, _ := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})

	body := `{"name":"claude","path":"/tmp/another"}`
	req := httptest.NewRequest(http.MethodPost, "/api/targets", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409 for duplicate target, got %d", rr.Code)
	}
}

func TestHandleRemoveTarget_Success(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, _ := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})

	req := httptest.NewRequest(http.MethodDelete, "/api/targets/claude", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleRemoveTarget_CleanupFailureKeepsTarget(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, sourceDir := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})
	addSkill(t, sourceDir, "alpha")

	if err := os.Symlink(filepath.Join(sourceDir, "alpha"), filepath.Join(tgtPath, "alpha")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}
	origRemoveTargetPath := removeTargetPath
	removeTargetPath = func(path string) error {
		if path == filepath.Join(tgtPath, "alpha") {
			return errors.New("remove denied")
		}
		return origRemoveTargetPath(path)
	}
	defer func() { removeTargetPath = origRemoveTargetPath }()

	req := httptest.NewRequest(http.MethodDelete, "/api/targets/claude", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rr.Code, rr.Body.String())
	}
	if _, ok := s.cfg.Targets["claude"]; !ok {
		t.Fatal("target should remain in memory after cleanup failure")
	}

	diskCfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config from disk: %v", err)
	}
	if _, ok := diskCfg.Targets["claude"]; !ok {
		t.Fatal("target should remain on disk after cleanup failure")
	}
}

// Codex and universal share ~/.agents/skills. Removing one must leave the links
// the other still syncs there, not turn them into copies it no longer manages.
func TestHandleRemoveTarget_KeepsSkillsAnotherTargetShares(t *testing.T) {
	shared := filepath.Join(t.TempDir(), "agents-skills")
	s, sourceDir := newTestServerWithTargets(t, map[string]string{"universal": shared, "codex": shared})
	addSkill(t, sourceDir, "alpha")
	link := filepath.Join(shared, "alpha")
	if err := os.Symlink(filepath.Join(sourceDir, "alpha"), link); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodDelete, "/api/targets/codex", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("universal's link was not kept: %v", err)
	}
}

// Removing an account the MCP config still selects succeeds and says where its name
// is left behind, so the next mcp sync does not fail on a target it cannot resolve.
func TestHandleRemoveTarget_WarnsWhenMCPStillNamesTheAccount(t *testing.T) {
	s, sourceDir := newTestServer(t)
	work := filepath.Join(t.TempDir(), "claude-work")
	if err := os.MkdirAll(filepath.Join(work, "skills"), 0755); err != nil {
		t.Fatalf("create account directory: %v", err)
	}
	raw := "source: " + sourceDir + "\nmode: merge\ntargets:\n  claude-work:\n    agent: claude\n    config_dir: " + work +
		"\nmcp:\n  targets: [claude-work]\n  servers:\n    docs:\n      url: https://example.com/mcp\n      targets: [claude-work]\n"
	if err := os.WriteFile(os.Getenv("SKILLSHARE_CONFIG"), []byte(raw), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	s = New(cfg, "127.0.0.1:0", "", "")

	req := httptest.NewRequest(http.MethodDelete, "/api/targets/claude-work", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Warnings []string `json:"warnings"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Warnings) != 1 || !strings.Contains(resp.Warnings[0], "mcp.targets, mcp.servers.docs still name claude-work") {
		t.Fatalf("warnings: %v", resp.Warnings)
	}
}

func TestHandleRemoveTarget_NotFound(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/targets/nonexistent", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestHandleUpdateTarget_Mode(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, _ := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})

	body := `{"mode":"symlink"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/targets/claude", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleUpdateTarget_PrefixedNamingNeedsCopyMode(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, _ := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})

	for _, tc := range []struct {
		body string
		want int
	}{
		{`{"target_naming":"prefixed"}`, http.StatusBadRequest}, // inherits merge
		{`{"mode":"copy","target_naming":"prefixed"}`, http.StatusOK},
		{`{"mode":"merge"}`, http.StatusBadRequest}, // would leave prefixed on merge
	} {
		req := httptest.NewRequest(http.MethodPatch, "/api/targets/claude", strings.NewReader(tc.body))
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, req)
		if rr.Code != tc.want {
			t.Fatalf("%s: expected %d, got %d: %s", tc.body, tc.want, rr.Code, rr.Body.String())
		}
	}
}

func TestHandleUpdateTarget_NotFound(t *testing.T) {
	s, _ := newTestServer(t)
	body := `{"mode":"merge"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/targets/nonexistent", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestHandleUpdateTarget_IncludeExclude_Persisted(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, _ := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})

	// PATCH include/exclude
	body := `{"include":["my-skill","other-*"],"exclude":["tmp-*"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/targets/claude", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("PATCH expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify disk persistence
	diskCfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config from disk: %v", err)
	}
	tgt, ok := diskCfg.Targets["claude"]
	if !ok {
		t.Fatal("target 'claude' not found in disk config")
	}
	diskSc := tgt.SkillsConfig()
	if len(diskSc.Include) != 2 || diskSc.Include[0] != "my-skill" || diskSc.Include[1] != "other-*" {
		t.Errorf("disk include mismatch: got %v", diskSc.Include)
	}
	if len(diskSc.Exclude) != 1 || diskSc.Exclude[0] != "tmp-*" {
		t.Errorf("disk exclude mismatch: got %v", diskSc.Exclude)
	}

	// Verify in-memory state was reloaded correctly
	memTgt, ok := s.cfg.Targets["claude"]
	if !ok {
		t.Fatal("target 'claude' not in in-memory config")
	}
	memSc := memTgt.SkillsConfig()
	if len(memSc.Include) != 2 || memSc.Include[0] != "my-skill" {
		t.Errorf("in-memory include mismatch: got %v", memSc.Include)
	}
	if len(memSc.Exclude) != 1 || memSc.Exclude[0] != "tmp-*" {
		t.Errorf("in-memory exclude mismatch: got %v", memSc.Exclude)
	}

	// Verify GET /api/targets returns the filters
	req2 := httptest.NewRequest(http.MethodGet, "/api/targets", nil)
	rr2 := httptest.NewRecorder()
	s.handler.ServeHTTP(rr2, req2)

	var resp struct {
		Targets []struct {
			Name    string   `json:"name"`
			Include []string `json:"include"`
			Exclude []string `json:"exclude"`
		} `json:"targets"`
	}
	json.Unmarshal(rr2.Body.Bytes(), &resp)
	if len(resp.Targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(resp.Targets))
	}
	if len(resp.Targets[0].Include) != 2 {
		t.Errorf("GET include mismatch: got %v", resp.Targets[0].Include)
	}
	if len(resp.Targets[0].Exclude) != 1 {
		t.Errorf("GET exclude mismatch: got %v", resp.Targets[0].Exclude)
	}
}

func TestHandleUpdateTarget_ClearFilters(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, _ := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})

	// First set filters
	body := `{"include":["a"],"exclude":["b"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/targets/claude", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("set filters: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Clear filters by sending empty arrays
	body2 := `{"include":[],"exclude":[]}`
	req2 := httptest.NewRequest(http.MethodPatch, "/api/targets/claude", strings.NewReader(body2))
	rr2 := httptest.NewRecorder()
	s.handler.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("clear filters: expected 200, got %d: %s", rr2.Code, rr2.Body.String())
	}

	// Verify disk config has empty filters
	diskCfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config from disk: %v", err)
	}
	tgt := diskCfg.Targets["claude"]
	if len(tgt.Include) != 0 {
		t.Errorf("expected empty include after clear, got %v", tgt.Include)
	}
	if len(tgt.Exclude) != 0 {
		t.Errorf("expected empty exclude after clear, got %v", tgt.Exclude)
	}

	// Verify GET also returns empty filters
	req3 := httptest.NewRequest(http.MethodGet, "/api/targets", nil)
	rr3 := httptest.NewRecorder()
	s.handler.ServeHTTP(rr3, req3)

	var resp struct {
		Targets []struct {
			Include []string `json:"include"`
			Exclude []string `json:"exclude"`
		} `json:"targets"`
	}
	json.Unmarshal(rr3.Body.Bytes(), &resp)
	if len(resp.Targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(resp.Targets))
	}
	if len(resp.Targets[0].Include) != 0 {
		t.Errorf("GET include should be empty after clear, got %v", resp.Targets[0].Include)
	}
	if len(resp.Targets[0].Exclude) != 0 {
		t.Errorf("GET exclude should be empty after clear, got %v", resp.Targets[0].Exclude)
	}
}

func TestHandleUpdateTarget_AgentIncludeExclude_Persisted(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, _ := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})

	// PATCH agent include/exclude
	body := `{"agent_include":["review-*"],"agent_exclude":["draft-*"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/targets/claude", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("PATCH expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify disk persistence
	diskCfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config from disk: %v", err)
	}
	tgt, ok := diskCfg.Targets["claude"]
	if !ok {
		t.Fatal("target 'claude' not found in disk config")
	}
	ac := tgt.AgentsConfig()
	if len(ac.Include) != 1 || ac.Include[0] != "review-*" {
		t.Errorf("disk agent include mismatch: got %v", ac.Include)
	}
	if len(ac.Exclude) != 1 || ac.Exclude[0] != "draft-*" {
		t.Errorf("disk agent exclude mismatch: got %v", ac.Exclude)
	}

	// Verify in-memory state
	memTgt := s.cfg.Targets["claude"]
	memAc := memTgt.AgentsConfig()
	if len(memAc.Include) != 1 || memAc.Include[0] != "review-*" {
		t.Errorf("in-memory agent include mismatch: got %v", memAc.Include)
	}
	if len(memAc.Exclude) != 1 || memAc.Exclude[0] != "draft-*" {
		t.Errorf("in-memory agent exclude mismatch: got %v", memAc.Exclude)
	}
}

func TestHandleUpdateTarget_AgentInclude_InvalidPattern(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, _ := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})

	body := `{"agent_include":["[unclosed"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/targets/claude", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid agent include pattern, got %d", rr.Code)
	}
}

func TestHandleUpdateTarget_AgentExclude_InvalidPattern(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, _ := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})

	body := `{"agent_exclude":["[bad"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/targets/claude", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid agent exclude pattern, got %d", rr.Code)
	}
}

func patchTarget(t *testing.T, s *Server, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, "/api/targets/claude", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	return rr
}

func TestHandleUpdateTarget_AgentExtension_ForcesCopy(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, _ := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})

	if rr := patchTarget(t, s, `{"agent_mode":"merge"}`); rr.Code != http.StatusOK {
		t.Fatalf("PATCH mode expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if rr := patchTarget(t, s, `{"agent_extension":"opencode-agents"}`); rr.Code != http.StatusOK {
		t.Fatalf("PATCH extension expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	diskCfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	tgt := diskCfg.Targets["claude"]
	if tgt.Agents == nil || tgt.Agents.Extension != "opencode-agents" || tgt.Agents.Mode != "copy" {
		t.Errorf("disk agents = %+v, want extension opencode-agents in copy mode", tgt.Agents)
	}
}

func TestHandleUpdateTarget_AgentExtension_RejectsConflictingMode(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, _ := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})

	patchTarget(t, s, `{"agent_mode":"merge"}`) // agents block now exists and is shared
	rr := patchTarget(t, s, `{"agent_extension":"opencode-agents","agent_mode":"symlink"}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
	memTgt := s.cfg.Targets["claude"]
	if mode := memTgt.AgentsConfig().Mode; mode == "symlink" {
		t.Error("a rejected request must not change the agent mode")
	}
}

func TestHandleUpdateTarget_AgentExtension_Clear(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, _ := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})

	patchTarget(t, s, `{"agent_extension":"opencode-agents"}`)
	if rr := patchTarget(t, s, `{"agent_extension":""}`); rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	memTgt := s.cfg.Targets["claude"]
	if ext := memTgt.AgentsConfig().Extension; ext != "" {
		t.Errorf("extension = %q, want cleared", ext)
	}
}

func TestHandleUpdateTarget_RejectedRequestChangesNothing(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, _ := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})
	patchTarget(t, s, `{"mode":"merge","agent_mode":"merge"}`) // skills and agents blocks now exist

	rr := patchTarget(t, s, `{"mode":"copy","agent_mode":"copy","agent_include":["["]}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
	memTgt := s.cfg.Targets["claude"]
	if got := memTgt.SkillsConfig().Mode + "/" + memTgt.AgentsConfig().Mode; got != "merge/merge" {
		t.Errorf("modes = %s, want merge/merge after a rejected request", got)
	}
}

// A target can be another config directory of a built-in Agent, such as a second account.
func TestHandleAddTarget_AgentConfigDir(t *testing.T) {
	s, _ := newTestServer(t)
	dir := filepath.Join(t.TempDir(), ".claude-work")
	add := func(body string) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/targets", strings.NewReader(body)))
		return rr
	}
	if rr := add(`{"name":"claude-work","agent":"claude","configDir":"` + dir + `"}`); rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	added := s.cfg.Targets["claude-work"]
	if got := added.SkillsConfig().Path; got != filepath.Join(dir, "skills") {
		t.Errorf("skills path = %q", got)
	}
	saved, err := os.ReadFile(config.ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(saved), "agent: claude") || strings.Contains(string(saved), "path:") {
		t.Errorf("the paths should stay derived:\n%s", saved)
	}
	if rr := add(`{"name":"cursor-work","agent":"cursor","configDir":"` + dir + `2"}`); rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for an Agent without a config directory, got %d", rr.Code)
	}
}

// cli names the compatible executable for the account's plugin commands, such as omo for Pi.
func TestHandleAddTarget_AgentConfigDirCLI(t *testing.T) {
	s, _ := newTestServer(t)
	dir := filepath.Join(t.TempDir(), ".omo", "agent")
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/targets", strings.NewReader(`{"name":"omo","agent":"pi","configDir":"`+dir+`","cli":"omo"}`)))
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if saved, _ := os.ReadFile(config.ConfigPath()); !strings.Contains(string(saved), "cli: omo") {
		t.Errorf("cli was not saved:\n%s", saved)
	}
	rr = httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/targets", nil))
	if !strings.Contains(rr.Body.String(), `"cli":"omo"`) {
		t.Errorf("cli was not listed: %s", rr.Body.String())
	}
}

func TestHandleAvailableTargets_NamesTheConfigDir(t *testing.T) {
	s, _ := newTestServer(t)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/config/available-targets", nil))
	var resp struct {
		Targets []struct{ Name, ConfigDir string } `json:"targets"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	accounts := map[string]bool{"claude": true, "codex": true, "pi": true, "omp": true}
	for _, target := range resp.Targets {
		if accounts[target.Name] != (target.ConfigDir != "") {
			t.Errorf("%s: configDir = %q", target.Name, target.ConfigDir)
		}
	}
}

func TestHandleAddTarget_ProjectPrefixedNamingUsesCopyMode(t *testing.T) {
	s, _ := newTestProjectServerWithExtras(t, nil)
	s.projectCfg.TargetNaming = "prefixed"
	if err := s.projectCfg.Save(s.projectRoot); err != nil { // requests reload the config from disk
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/targets", strings.NewReader(`{"name":"cursor"}`)))
	if rr.Code != http.StatusOK {
		t.Fatalf("add: got %d %s", rr.Code, rr.Body.String())
	}
	if _, invalid, err := config.ValidateProjectConfigForSync(s.projectCfg, s.projectRoot); err != nil || invalid["cursor"] != nil {
		t.Fatalf("added target invalid: err = %v, invalid = %v", err, invalid)
	}
}

func TestHandleAddTarget_GlobalPrefixedNamingUnderMergeUsesCopyMode(t *testing.T) {
	s, _ := newTestServer(t)
	s.cfg.Mode, s.cfg.TargetNaming = "merge", "prefixed" // valid while every target overrides copy
	if err := s.saveConfig(); err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	body := `{"name":"cursor","path":"` + filepath.ToSlash(filepath.Join(t.TempDir(), "skills")) + `"}`
	s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/targets", strings.NewReader(body)))
	if rr.Code != http.StatusOK {
		t.Fatalf("add: got %d %s", rr.Code, rr.Body.String())
	}
	if tc := s.cfg.Targets["cursor"]; tc.SkillsConfig().Mode != "copy" {
		mode := tc.SkillsConfig().Mode
		t.Fatalf("new target mode = %q, want copy", mode)
	}

	rr = httptest.NewRecorder()
	body = `{"name":"claude-work","agent":"claude","configDir":"` + filepath.ToSlash(filepath.Join(t.TempDir(), ".claude-work")) + `"}`
	s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/targets", strings.NewReader(body)))
	if tc := s.cfg.Targets["claude-work"]; rr.Code != http.StatusOK || tc.SkillsConfig().Mode != "copy" {
		t.Fatalf("add config dir: got %d %s, mode %q; want copy", rr.Code, rr.Body.String(), tc.SkillsConfig().Mode)
	}
}

func TestHandleUpdateTarget_ReenablingSkillsChecksPrefixedNaming(t *testing.T) {
	s, _ := newTestServer(t)
	skills := &config.ResourceTargetConfig{Path: filepath.Join(t.TempDir(), "skills"), TargetNaming: "prefixed"}
	skills.SetEnabled(false) // allowed while off: it syncs no skill
	s.cfg.Mode = "merge"
	s.cfg.Targets = map[string]config.TargetConfig{"claude": {Skills: skills}}
	if err := s.saveConfig(); err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPatch, "/api/targets/claude", strings.NewReader(`{"skills_enabled":true}`)))
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "requires copy mode") {
		t.Fatalf("re-enable under merge: got %d %s, want 400", rr.Code, rr.Body.String())
	}
}

func TestHandleUpdateTarget_DisablingSkillsSkipsNamingCheck(t *testing.T) {
	s, _ := newTestServer(t)
	s.cfg.Targets = map[string]config.TargetConfig{"claude": {Skills: &config.ResourceTargetConfig{
		Path: filepath.Join(t.TempDir(), "skills"), Mode: "copy", TargetNaming: "prefixed",
	}}}
	if err := s.saveConfig(); err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPatch, "/api/targets/claude", strings.NewReader(`{"skills_enabled":false,"mode":"merge"}`)))
	if rr.Code != http.StatusOK {
		t.Fatalf("disable with merge: got %d %s, want 200", rr.Code, rr.Body.String())
	}
}
