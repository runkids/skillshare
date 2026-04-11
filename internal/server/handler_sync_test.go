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
	managedhooks "skillshare/internal/resources/hooks"
	managedrules "skillshare/internal/resources/rules"
)

func TestHandleSync_DefaultSyncIncludesManagedResourceResults(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, src := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})
	addSkill(t, src, "alpha")

	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{"dryRun":false}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Results) != 3 {
		t.Fatalf("expected 3 sync results, got %d", len(resp.Results))
	}
	if resp.Results[0]["target"] != "claude" {
		t.Errorf("expected target 'claude', got %v", resp.Results[0]["target"])
	}
	if resp.Results[0]["resource"] != "skills" {
		t.Errorf("expected first resource to be skills, got %v", resp.Results[0]["resource"])
	}
}

func TestHandleSync_IgnoredSkillNotPrunedFromRegistry(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, src := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})

	addSkill(t, src, "kept-skill")
	addSkillMeta(t, src, "kept-skill", "github.com/user/kept")

	addSkill(t, src, "ignored-skill")
	addSkillMeta(t, src, "ignored-skill", "github.com/user/ignored")

	if err := os.WriteFile(filepath.Join(src, ".skillignore"), []byte("ignored-skill\n"), 0o644); err != nil {
		t.Fatalf("write .skillignore: %v", err)
	}

	s.skillsStore = install.NewMetadataStore()
	s.skillsStore.Set("kept-skill", &install.MetadataEntry{Source: "github.com/user/kept"})
	s.skillsStore.Set("ignored-skill", &install.MetadataEntry{Source: "github.com/user/ignored"})
	if err := s.skillsStore.Save(src); err != nil {
		t.Fatalf("failed to save metadata: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{"dryRun":false,"kind":"skill"}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	names := s.skillsStore.List()
	if len(names) != 2 {
		t.Fatalf("expected 2 metadata entries after sync, got %d: %v", len(names), names)
	}
}

func TestHandleSync_NoTargets(t *testing.T) {
	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Results []any `json:"results"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Results) != 0 {
		t.Errorf("expected 0 results for no targets, got %d", len(resp.Results))
	}
}

func TestHandleSync_AgentPrunesOrphanWhenSourceEmpty(t *testing.T) {
	s, _ := newTestServer(t)

	agentSource := filepath.Join(t.TempDir(), "agents")
	agentTarget := filepath.Join(t.TempDir(), "claude-agents")
	if err := os.MkdirAll(agentSource, 0o755); err != nil {
		t.Fatalf("mkdir agent source: %v", err)
	}
	if err := os.MkdirAll(agentTarget, 0o755); err != nil {
		t.Fatalf("mkdir agent target: %v", err)
	}

	orphanPath := filepath.Join(agentTarget, "tutor.md")
	if err := os.Symlink(filepath.Join(agentSource, "tutor.md"), orphanPath); err != nil {
		t.Fatalf("seed orphan agent symlink: %v", err)
	}

	s.cfg.AgentsSource = agentSource
	s.cfg.Targets["claude"] = config.TargetConfig{
		Skills: &config.ResourceTargetConfig{Path: filepath.Join(t.TempDir(), "claude-skills")},
		Agents: &config.ResourceTargetConfig{Path: agentTarget},
	}
	if err := s.cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{"kind":"agent"}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	if _, err := os.Lstat(orphanPath); !os.IsNotExist(err) {
		t.Fatalf("expected orphan agent symlink to be pruned, got err=%v", err)
	}

	var resp struct {
		Results []struct {
			Target   string   `json:"target"`
			Resource string   `json:"resource"`
			Pruned   []string `json:"pruned"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal sync response: %v", err)
	}

	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 sync result, got %d", len(resp.Results))
	}
	if resp.Results[0].Target != "claude" {
		t.Fatalf("expected claude target, got %q", resp.Results[0].Target)
	}
	if resp.Results[0].Resource != "agents" {
		t.Fatalf("expected agents resource, got %q", resp.Results[0].Resource)
	}
	if len(resp.Results[0].Pruned) != 1 || resp.Results[0].Pruned[0] != "tutor.md" {
		t.Fatalf("expected pruned tutor.md, got %+v", resp.Results[0].Pruned)
	}
}

func TestHandleSync_InvalidJSONReturnsBadRequest(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, src := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})
	addSkill(t, src, "alpha")

	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{"dryRun":`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}

	if _, err := os.Lstat(filepath.Join(tgtPath, "alpha")); !os.IsNotExist(err) {
		t.Fatalf("expected no sync side effects on invalid JSON, got err=%v", err)
	}
}

func TestHandleSync_ManagedResourcesDoNotRequireSkillSource(t *testing.T) {
	s, projectRoot, _, _ := newManagedProjectServer(t, "claude")
	s.cfg.Source = filepath.Join(t.TempDir(), "missing-source")

	ruleStore := managedrules.NewStore(projectRoot)
	if _, err := ruleStore.Put(managedrules.Save{
		ID:      "claude/manual.md",
		Content: []byte("# Managed rule\n"),
	}); err != nil {
		t.Fatalf("put managed rule: %v", err)
	}

	hookStore := managedhooks.NewStore(projectRoot)
	if _, err := hookStore.Put(managedhooks.Save{
		ID:      "claude/pre-tool-use/bash.yaml",
		Tool:    "claude",
		Event:   "PreToolUse",
		Matcher: "Bash",
		Handlers: []managedhooks.Handler{{
			Type:    "command",
			Command: "./bin/check",
		}},
	}); err != nil {
		t.Fatalf("put managed hook: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{"resources":["rules","hooks"]}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	if _, err := os.Stat(filepath.Join(projectRoot, ".claude", "rules", "manual.md")); err != nil {
		t.Fatalf("expected managed rule to sync: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".claude", "settings.json")); err != nil {
		t.Fatalf("expected managed hook config to sync: %v", err)
	}
}

func TestHandleSync_ManagedRuleFailureStillAttemptsHooks(t *testing.T) {
	s, projectRoot, _, _ := newManagedProjectServer(t, "claude")

	ruleStore := managedrules.NewStore(projectRoot)
	if _, err := ruleStore.Put(managedrules.Save{
		ID:      "claude/manual.md",
		Content: []byte("# Managed rule\n"),
	}); err != nil {
		t.Fatalf("put managed rule: %v", err)
	}

	hookStore := managedhooks.NewStore(projectRoot)
	if _, err := hookStore.Put(managedhooks.Save{
		ID:      "claude/pre-tool-use/bash.yaml",
		Tool:    "claude",
		Event:   "PreToolUse",
		Matcher: "Bash",
		Handlers: []managedhooks.Handler{{
			Type:    "command",
			Command: "./bin/check",
		}},
	}); err != nil {
		t.Fatalf("put managed hook: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(projectRoot, ".claude"), 0o755); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".claude", "rules"), []byte("not-a-directory"), 0o644); err != nil {
		t.Fatalf("seed invalid rules path: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{"resources":["rules","hooks"]}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rr.Code, rr.Body.String())
	}

	if _, err := os.Stat(filepath.Join(projectRoot, ".claude", "settings.json")); err != nil {
		t.Fatalf("expected hooks sync to continue after rules failure: %v", err)
	}
}

func TestHandleSync_DefaultsToAllManagedResources(t *testing.T) {
	s, projectRoot, sourceDir, _ := newManagedProjectServer(t, "claude")
	addSkill(t, sourceDir, "alpha")

	ruleStore := managedrules.NewStore(projectRoot)
	if _, err := ruleStore.Put(managedrules.Save{
		ID:      "claude/manual.md",
		Content: []byte("# Managed rule\n"),
	}); err != nil {
		t.Fatalf("put managed rule: %v", err)
	}

	hookStore := managedhooks.NewStore(projectRoot)
	if _, err := hookStore.Put(managedhooks.Save{
		ID:      "claude/pre-tool-use/bash.yaml",
		Tool:    "claude",
		Event:   "PreToolUse",
		Matcher: "Bash",
		Handlers: []managedhooks.Handler{{
			Type:    "command",
			Command: "./bin/check",
		}},
	}); err != nil {
		t.Fatalf("put managed hook: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{"dryRun":false}`))
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Results []struct {
			Target   string   `json:"target"`
			Resource string   `json:"resource"`
			Linked   []string `json:"linked"`
			Updated  []string `json:"updated"`
			Skipped  []string `json:"skipped"`
			Pruned   []string `json:"pruned"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Results) != 3 {
		t.Fatalf("expected 3 sync results, got %d: %#v", len(resp.Results), resp.Results)
	}

	byResource := make(map[string]struct {
		Linked  []string
		Updated []string
		Skipped []string
		Pruned  []string
	}, len(resp.Results))
	for _, result := range resp.Results {
		if result.Target != "claude" {
			t.Fatalf("result target = %q, want claude", result.Target)
		}
		byResource[result.Resource] = struct {
			Linked  []string
			Updated []string
			Skipped []string
			Pruned  []string
		}{
			Linked:  result.Linked,
			Updated: result.Updated,
			Skipped: result.Skipped,
			Pruned:  result.Pruned,
		}
	}

	if got := byResource["skills"].Linked; len(got) != 1 || got[0] != "alpha" {
		t.Fatalf("skills linked = %#v, want [alpha]", got)
	}
	if got := byResource["rules"].Updated; len(got) != 1 || got[0] != filepath.Join(projectRoot, ".claude", "rules", "manual.md") {
		t.Fatalf("rules updated = %#v, want compiled rule path", got)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".claude", "rules", "manual.md")); err != nil {
		t.Fatalf("expected synced rule file: %v", err)
	}
	if got := byResource["hooks"].Updated; len(got) != 1 || got[0] != filepath.Join(projectRoot, ".claude", "settings.json") {
		t.Fatalf("hooks updated = %#v, want compiled hook path", got)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".claude", "settings.json")); err != nil {
		t.Fatalf("expected synced hook file: %v", err)
	}
}

func TestHandleSync_HooksOnlyMaterializesEmptyCarrier(t *testing.T) {
	s, projectRoot, _, _ := newManagedProjectServer(t, "claude")

	settingsPath := filepath.Join(projectRoot, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("mkdir settings dir: %v", err)
	}
	if err := os.WriteFile(settingsPath, []byte(`{"model":"sonnet"}`), 0o644); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sync", strings.NewReader(`{"resources":["hooks"]}`))
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Results []struct {
			Target   string   `json:"target"`
			Resource string   `json:"resource"`
			Updated  []string `json:"updated"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d: %#v", len(resp.Results), resp.Results)
	}
	if resp.Results[0].Target != "claude" || resp.Results[0].Resource != "hooks" {
		t.Fatalf("result = %#v, want hooks result for claude", resp.Results[0])
	}
	if len(resp.Results[0].Updated) != 1 || resp.Results[0].Updated[0] != settingsPath {
		t.Fatalf("updated = %#v, want %q", resp.Results[0].Updated, settingsPath)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings.json: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, `"model":"sonnet"`) {
		t.Fatalf("settings.json = %q, want existing key preserved", content)
	}
	if !strings.Contains(content, `"hooks":{}`) {
		t.Fatalf("settings.json = %q, want empty hooks carrier", content)
	}
}
