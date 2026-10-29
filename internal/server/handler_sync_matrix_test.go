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

func TestHandleSyncMatrix_Empty(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/sync-matrix", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Entries []any `json:"entries"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(resp.Entries))
	}
}

func TestSyncMatrixUsesSharedVariantSelection(t *testing.T) {
	s, src := newTestServerWithTargets(t, map[string]string{"claude": t.TempDir(), "codex-work": t.TempDir(), "universal": t.TempDir()})
	account := s.cfg.Targets["codex-work"]
	account.Agent = "codex"
	account.ConfigDir = t.TempDir()
	account.Path = ""
	s.cfg.Targets["codex-work"] = account
	if err := s.cfg.Save(); err != nil {
		t.Fatal(err)
	}
	for _, agent := range []string{"claude", "claude-code", "codex"} {
		for _, suffix := range []string{"fix", "review/fix"} {
			addSkill(t, src, "_repo/skills/"+agent+"/"+suffix)
		}
	}
	for _, tc := range []struct {
		target string
		want   int
	}{{"claude", 0}, {"codex-work", 2}, {"universal", 0}} {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			t.Run(tc.target+"/"+method, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, "/api/sync-matrix?target="+tc.target, nil)
				if method == http.MethodPost {
					req = httptest.NewRequest(http.MethodPost, "/api/sync-matrix/preview", strings.NewReader(`{"target":"`+tc.target+`"}`))
				}
				rr := httptest.NewRecorder()
				s.handler.ServeHTTP(rr, req)
				if rr.Code != http.StatusOK {
					t.Fatal(rr.Body.String())
				}
				var response struct {
					Entries []syncMatrixEntry `json:"entries"`
				}
				if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				synced, skipped := 0, 0
				for _, row := range response.Entries {
					if row.Kind == "agent" {
						continue
					}
					if row.Status == "synced" {
						synced++
					}
					if row.Status == "skipped" && strings.Contains(row.Reason, "ambiguous") {
						skipped++
					}
				}
				if synced != tc.want || (tc.target == "claude" && skipped != 4) {
					t.Fatalf("matrix selected %d, skipped %d; want %d selected: %s", synced, skipped, tc.want, rr.Body.String())
				}
			})
		}
	}
	// Draft includes must disambiguate before the shared selector checks aliases.
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/sync-matrix/preview", strings.NewReader(`{"target":"claude","include":["_repo__skills__claude__*"]}`)))
	var response struct {
		Entries []syncMatrixEntry `json:"entries"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	synced := 0
	for _, row := range response.Entries {
		if row.Status == "synced" {
			synced++
		}
	}
	if synced != 2 {
		t.Fatalf("draft include failed to select exact variant: %s", rr.Body.String())
	}
}

func TestHandleSyncMatrix_WithSkillsAndTargets(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, sourceDir := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})
	addSkill(t, sourceDir, "frontend-design")
	addSkill(t, sourceDir, "backend-api")
	req := httptest.NewRequest(http.MethodGet, "/api/sync-matrix", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Entries []struct {
			Skill  string `json:"skill"`
			Target string `json:"target"`
			Status string `json:"status"`
		} `json:"entries"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(resp.Entries))
	}
	for _, e := range resp.Entries {
		if e.Target != "claude" {
			t.Errorf("expected target 'claude', got %q", e.Target)
		}
		if e.Status != "synced" {
			t.Errorf("skill %q: expected 'synced', got %q", e.Skill, e.Status)
		}
	}
}

func TestHandleSyncMatrix_WithFilters(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, sourceDir := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})
	addSkill(t, sourceDir, "frontend-design")
	addSkill(t, sourceDir, "backend-api")
	s.cfg.Targets["claude"] = config.TargetConfig{Path: tgtPath, Mode: "merge", Exclude: []string{"backend*"}}
	if err := s.cfg.Save(); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/sync-matrix", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	var resp struct {
		Entries []struct {
			Skill        string            `json:"skill"`
			Status       string            `json:"status"`
			Reason       string            `json:"reason"`
			ReasonCode   string            `json:"reasonCode"`
			ReasonParams map[string]string `json:"reasonParams"`
		} `json:"entries"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(resp.Entries))
	}
	statusMap := map[string]string{}
	reasonMap := map[string]string{}
	reasonCodeMap := map[string]string{}
	reasonPatternMap := map[string]string{}
	for _, e := range resp.Entries {
		statusMap[e.Skill] = e.Status
		reasonMap[e.Skill] = e.Reason
		reasonCodeMap[e.Skill] = e.ReasonCode
		if e.ReasonParams != nil {
			reasonPatternMap[e.Skill] = e.ReasonParams["pattern"]
		}
	}
	if statusMap["frontend-design"] != "synced" {
		t.Errorf("frontend-design: expected synced, got %q", statusMap["frontend-design"])
	}
	if statusMap["backend-api"] != "excluded" {
		t.Errorf("backend-api: expected excluded, got %q", statusMap["backend-api"])
	}
	if reasonMap["backend-api"] != "backend*" {
		t.Errorf("backend-api reason: expected 'backend*', got %q", reasonMap["backend-api"])
	}
	if reasonCodeMap["backend-api"] != "sync_matrix.excluded" {
		t.Errorf("backend-api reasonCode: expected sync_matrix.excluded, got %q", reasonCodeMap["backend-api"])
	}
	if reasonPatternMap["backend-api"] != "backend*" {
		t.Errorf("backend-api pattern param: expected backend*, got %q", reasonPatternMap["backend-api"])
	}
}

func TestHandleSyncMatrix_TargetQueryParam(t *testing.T) {
	tgt1 := filepath.Join(t.TempDir(), "claude-skills")
	tgt2 := filepath.Join(t.TempDir(), "cursor-skills")
	s, sourceDir := newTestServerWithTargets(t, map[string]string{"claude": tgt1, "cursor": tgt2})
	addSkill(t, sourceDir, "my-skill")
	req := httptest.NewRequest(http.MethodGet, "/api/sync-matrix?target=claude", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	var resp struct {
		Entries []struct {
			Target string `json:"target"`
		} `json:"entries"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	for _, e := range resp.Entries {
		if e.Target != "claude" {
			t.Errorf("expected only claude entries, got %q", e.Target)
		}
	}
}

func TestHandleSyncMatrixPreview_Basic(t *testing.T) {
	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, sourceDir := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})
	addSkill(t, sourceDir, "frontend-design")
	addSkill(t, sourceDir, "backend-api")
	addSkill(t, sourceDir, "legacy-tool")
	body := `{"target":"claude","include":["frontend*","backend*"],"exclude":["legacy*"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/sync-matrix/preview", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Entries []struct {
			Skill  string `json:"skill"`
			Status string `json:"status"`
		} `json:"entries"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(resp.Entries))
	}
	statusMap := map[string]string{}
	for _, e := range resp.Entries {
		statusMap[e.Skill] = e.Status
	}
	if statusMap["frontend-design"] != "synced" {
		t.Errorf("frontend-design: expected synced, got %q", statusMap["frontend-design"])
	}
	if statusMap["backend-api"] != "synced" {
		t.Errorf("backend-api: expected synced, got %q", statusMap["backend-api"])
	}
	if statusMap["legacy-tool"] != "not_included" {
		t.Errorf("legacy-tool: expected not_included, got %q", statusMap["legacy-tool"])
	}
}

func TestHandleSyncMatrixPreview_InvalidPattern(t *testing.T) {
	s, _ := newTestServer(t)
	body := `{"target":"claude","include":["[unclosed"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/sync-matrix/preview", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestHandleSyncMatrixPreview_MissingTarget(t *testing.T) {
	s, _ := newTestServer(t)
	body := `{"include":["*"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/sync-matrix/preview", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestHandleSyncMatrixPreview_IncludesAgents(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	os.MkdirAll(home, 0755)
	t.Setenv("HOME", home)

	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, sourceDir := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})
	addSkill(t, sourceDir, "my-skill")

	// Add agents to the agents source directory
	agentsSource := s.cfg.EffectiveAgentsSource()
	addAgentFile(t, agentsSource, "code-reviewer.md")
	addAgentFile(t, agentsSource, "draft-helper.md")

	body := `{"target":"claude","include":[],"exclude":[],"agent_include":[],"agent_exclude":["draft-*"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/sync-matrix/preview", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Entries []struct {
			Skill  string `json:"skill"`
			Status string `json:"status"`
			Kind   string `json:"kind"`
		} `json:"entries"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)

	// Should have 1 skill + 2 agents = 3 entries
	if len(resp.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(resp.Entries))
	}

	statusMap := map[string]string{}
	kindMap := map[string]string{}
	for _, e := range resp.Entries {
		statusMap[e.Skill] = e.Status
		kindMap[e.Skill] = e.Kind
	}

	// Skill should be synced
	if statusMap["my-skill"] != "synced" {
		t.Errorf("my-skill: expected synced, got %q", statusMap["my-skill"])
	}
	if kindMap["my-skill"] != "" {
		t.Errorf("my-skill kind: expected empty, got %q", kindMap["my-skill"])
	}

	// code-reviewer agent should be synced
	if statusMap["code-reviewer.md"] != "synced" {
		t.Errorf("code-reviewer.md: expected synced, got %q", statusMap["code-reviewer.md"])
	}
	if kindMap["code-reviewer.md"] != "agent" {
		t.Errorf("code-reviewer.md kind: expected agent, got %q", kindMap["code-reviewer.md"])
	}

	// draft-helper agent should be excluded
	if statusMap["draft-helper.md"] != "excluded" {
		t.Errorf("draft-helper.md: expected excluded, got %q", statusMap["draft-helper.md"])
	}
}

func TestHandleSyncMatrixPreview_AgentExactNameExclude(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	os.MkdirAll(home, 0755)
	t.Setenv("HOME", home)

	tgtPath := filepath.Join(t.TempDir(), "claude-skills")
	s, _ := newTestServerWithTargets(t, map[string]string{"claude": tgtPath})
	addAgentFile(t, s.cfg.EffectiveAgentsSource(), "draft-helper.md")

	// Sync matches agent patterns without the .md extension, so the matrix must too.
	body := `{"target":"claude","include":[],"exclude":[],"agent_include":[],"agent_exclude":["draft-helper"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/sync-matrix/preview", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Entries []struct {
			Skill  string `json:"skill"`
			Status string `json:"status"`
		} `json:"entries"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	for _, e := range resp.Entries {
		if e.Skill == "draft-helper.md" && e.Status != "excluded" {
			t.Errorf("draft-helper.md: expected excluded, got %q", e.Status)
		}
	}
}

func TestHandleSyncMatrixPreview_NoAgentsWhenNoAgentPath(t *testing.T) {
	// custom-tool has no agent path in builtin targets
	tgtPath := filepath.Join(t.TempDir(), "custom-skills")
	s, sourceDir := newTestServerWithTargets(t, map[string]string{"custom-tool": tgtPath})
	addSkill(t, sourceDir, "my-skill")

	// Add agents to the agents source
	agentsSource := s.cfg.EffectiveAgentsSource()
	addAgentFile(t, agentsSource, "reviewer.md")

	body := `{"target":"custom-tool","include":[],"exclude":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/sync-matrix/preview", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Entries []struct {
			Skill string `json:"skill"`
			Kind  string `json:"kind"`
		} `json:"entries"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)

	// Should have only 1 skill, no agents
	if len(resp.Entries) != 1 {
		t.Fatalf("expected 1 entry (skill only, no agents), got %d", len(resp.Entries))
	}
	if resp.Entries[0].Kind == "agent" {
		t.Error("expected no agent entries for target without agent path")
	}
}

func TestHandleSyncMatrixPreview_InvalidAgentPattern(t *testing.T) {
	s, _ := newTestServer(t)
	body := `{"target":"claude","include":[],"exclude":[],"agent_include":["[unclosed"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/sync-matrix/preview", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid agent pattern, got %d", rr.Code)
	}
}

func TestHandleSyncMatrixPreview_ProjectDraftGetsAgents(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	os.MkdirAll(home, 0755)
	t.Setenv("HOME", home)

	s, _ := newTestServerWithTargets(t, map[string]string{"claude": filepath.Join(t.TempDir(), "claude-skills")})
	addAgentFile(t, s.cfg.EffectiveAgentsSource(), "code-reviewer.md")

	// No target is called shop@claude yet: the project page previews before it saves.
	body := `{"target":"shop@claude","include":[],"exclude":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/sync-matrix/preview", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"kind":"agent"`) {
		t.Fatalf("status %d, body %s", rr.Code, rr.Body.String())
	}
}

func TestSyncMatrixAmbiguityReasonCode(t *testing.T) {
	entry := newSyncMatrixEntry("_repo__skills__claude__fix", "claude", "skipped", "ambiguous variant", "skill")
	if entry.ReasonCode != "sync_matrix.ambiguous_variant" {
		t.Fatalf("reason code = %q", entry.ReasonCode)
	}
}

func TestSyncMatrixPreviewRecomputesSharedVariantSelection(t *testing.T) {
	shared := t.TempDir()
	s, src := newTestServerWithTargets(t, map[string]string{"codex": shared, "universal": shared})
	for _, agent := range []string{"claude", "codex"} {
		addSkill(t, src, "_repo/skills/"+agent+"/fix")
	}
	for _, tc := range []struct {
		filters string
		want    string
	}{
		{``, "synced"},
		{`,"exclude":["unrelated"]`, "skill_target_mismatch"},
	} {
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/sync-matrix/preview", strings.NewReader(`{"target":"universal"`+tc.filters+`}`)))
		if rr.Code != http.StatusOK {
			t.Fatal(rr.Body.String())
		}
		var response struct {
			Entries []syncMatrixEntry `json:"entries"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		for _, row := range response.Entries {
			if row.Skill == "_repo__skills__codex__fix" && row.Status != tc.want {
				t.Fatalf("preview %s: got %s want %s", tc.filters, row.Status, tc.want)
			}
		}
	}
	if len(s.cfg.Targets["universal"].Exclude) != 0 {
		t.Fatal("preview mutated saved filters")
	}
}
