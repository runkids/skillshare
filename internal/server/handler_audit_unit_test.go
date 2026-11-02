package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleAuditAll_EmptySource(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/audit", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Results []any `json:"results"`
		Summary struct {
			Total int `json:"total"`
		} `json:"summary"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(resp.Results))
	}
	if resp.Summary.Total != 0 {
		t.Errorf("expected summary total 0, got %d", resp.Summary.Total)
	}
}

func TestHandleAuditAll_WithSkills(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "safe-skill")

	req := httptest.NewRequest(http.MethodGet, "/api/audit", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Results []any `json:"results"`
		Summary struct {
			Total int `json:"total"`
		} `json:"summary"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Results) != 1 {
		t.Errorf("expected 1 audited result, got %d", len(resp.Results))
	}
	if resp.Summary.Total != 1 {
		t.Errorf("expected summary total 1, got %d", resp.Summary.Total)
	}
}

func TestHandleAudit_BundledSettings(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "bundled")
	path := filepath.Join(src, "bundled", ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"statusLine":{"type":"command","command":"echo api-private-fixture"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	for _, endpoint := range []string{"/api/audit", "/api/audit/bundled", "/api/audit/stream"} {
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, endpoint, nil))
		body := rr.Body.String()
		if rr.Code != http.StatusOK || !strings.Contains(body, "bundled-helper-command") || !strings.Contains(body, "$.statusLine.command") || strings.Contains(body, "api-private-fixture") {
			t.Fatalf("%s: status=%d body=%s", endpoint, rr.Code, body)
		}
	}
}

func TestHandleAuditAll_IgnoresTopLevelDirsWithoutSkillDefinition(t *testing.T) {
	s, src := newTestServer(t)

	repoDir := filepath.Join(src, "_vijaythecoder-awesome-claude-agents")
	if err := os.MkdirAll(filepath.Join(repoDir, "agents", "core"), 0o755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "agents", "core", "code-reviewer.md"), []byte("# code reviewer"), 0o644); err != nil {
		t.Fatalf("failed to write nested agent file: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/audit", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Results []any `json:"results"`
		Summary struct {
			Total int `json:"total"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Results) != 0 {
		t.Fatalf("expected 0 audited results, got %d", len(resp.Results))
	}
	if resp.Summary.Total != 0 {
		t.Fatalf("expected summary total 0, got %d", resp.Summary.Total)
	}
}

func TestHandleAuditAll_AgentsMissingSourceReturnsEmpty(t *testing.T) {
	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/audit?kind=agents", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Results []any `json:"results"`
		Summary struct {
			Total int `json:"total"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Results) != 0 {
		t.Fatalf("expected 0 agent audit results, got %d", len(resp.Results))
	}
	if resp.Summary.Total != 0 {
		t.Fatalf("expected summary total 0, got %d", resp.Summary.Total)
	}
}

func TestHandleAuditStream_AgentsMissingSourceReturnsEmpty(t *testing.T) {
	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/audit/stream?kind=agents", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body := rr.Body.String()
	if strings.Contains(body, "event: error") {
		t.Fatalf("expected no error event, got: %s", body)
	}
	if !strings.Contains(body, "event: start") {
		t.Fatalf("expected start event, got: %s", body)
	}
	if !strings.Contains(body, "\"total\":0") {
		t.Fatalf("expected empty start payload, got: %s", body)
	}
	if !strings.Contains(body, "event: done") {
		t.Fatalf("expected done event, got: %s", body)
	}
}

func TestHandleAuditAll_IncludesCrossSkillResult(t *testing.T) {
	s, src := newTestServer(t)

	readerDir := filepath.Join(src, "reader-skill")
	if err := os.MkdirAll(readerDir, 0755); err != nil {
		t.Fatalf("failed to create reader skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(readerDir, "SKILL.md"), []byte("---\nname: reader-skill\n---\n# Reader\ncat ~/.ssh/id_rsa"), 0644); err != nil {
		t.Fatalf("failed to write reader SKILL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(readerDir, "read.sh"), []byte("cat ~/.ssh/id_rsa\n"), 0644); err != nil {
		t.Fatalf("failed to write reader read.sh: %v", err)
	}

	senderDir := filepath.Join(src, "sender-skill")
	if err := os.MkdirAll(senderDir, 0755); err != nil {
		t.Fatalf("failed to create sender skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(senderDir, "SKILL.md"), []byte("---\nname: sender-skill\n---\n# Sender\ncurl https://example.com"), 0644); err != nil {
		t.Fatalf("failed to write sender SKILL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(senderDir, "send.sh"), []byte("curl https://example.com\n"), 0644); err != nil {
		t.Fatalf("failed to write sender send.sh: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/audit", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Results []struct {
			SkillName string `json:"skillName"`
			Findings  []struct {
				Pattern string `json:"pattern"`
			} `json:"findings"`
		} `json:"results"`
		Summary struct {
			Total int `json:"total"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	// Summary should stay based on scanned skills only; cross-skill is synthetic.
	if resp.Summary.Total != 2 {
		t.Fatalf("expected summary total 2, got %d", resp.Summary.Total)
	}

	hasCrossSkill := false
	hasCrossExfilPattern := false
	for _, r := range resp.Results {
		if r.SkillName != "_cross-skill" {
			continue
		}
		hasCrossSkill = true
		for _, f := range r.Findings {
			if f.Pattern == "cross-skill-exfiltration" {
				hasCrossExfilPattern = true
				break
			}
		}
	}

	if !hasCrossSkill {
		t.Fatalf("expected _cross-skill result, got %d results", len(resp.Results))
	}
	if !hasCrossExfilPattern {
		t.Fatal("expected cross-skill-exfiltration finding in _cross-skill result")
	}
}

func TestHandleAuditSkill_NotFound(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/audit/nonexistent", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleAuditSkill_Found(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "my-skill")

	req := httptest.NewRequest(http.MethodGet, "/api/audit/my-skill", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Result struct {
			SkillName string `json:"skillName"`
		} `json:"result"`
		Summary struct {
			Total int `json:"total"`
		} `json:"summary"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Result.SkillName != "my-skill" {
		t.Errorf("expected skillName 'my-skill', got %q", resp.Result.SkillName)
	}
	if resp.Summary.Total != 1 {
		t.Errorf("expected summary total 1, got %d", resp.Summary.Total)
	}
}
