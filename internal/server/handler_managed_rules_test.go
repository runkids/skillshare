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
	"skillshare/internal/inspect"
)

func TestManagedRulesCRUDAndCollect(t *testing.T) {
	s, projectRoot, _, _ := newManagedProjectServer(t, "claude")

	createReq := httptest.NewRequest(http.MethodPost, "/api/managed/rules", strings.NewReader(`{"tool":"claude","relativePath":"claude/manual.md","content":"# Managed\n"}`))
	createRR := httptest.NewRecorder()
	s.handler.ServeHTTP(createRR, createReq)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("expected 201 from create, got %d: %s", createRR.Code, createRR.Body.String())
	}

	var createResp struct {
		Rule struct {
			ID      string `json:"id"`
			Content string `json:"content"`
		} `json:"rule"`
		Previews []struct {
			Target string `json:"target"`
			Files  []struct {
				Path    string `json:"path"`
				Content string `json:"content"`
				Format  string `json:"format"`
			} `json:"files"`
		} `json:"previews"`
	}
	if err := json.Unmarshal(createRR.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("failed to decode create response: %v", err)
	}
	if createResp.Rule.ID != "claude/manual.md" {
		t.Fatalf("create response rule id = %q, want %q", createResp.Rule.ID, "claude/manual.md")
	}
	if len(createResp.Previews) != 1 || createResp.Previews[0].Target != "claude" {
		t.Fatalf("create previews = %#v, want one claude preview", createResp.Previews)
	}
	if len(createResp.Previews[0].Files) == 0 || createResp.Previews[0].Files[0].Path != filepath.Join(projectRoot, ".claude", "rules", "manual.md") {
		t.Fatalf("create preview files = %#v, want compiled claude rule output", createResp.Previews[0].Files)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/managed/rules/claude/manual.md", nil)
	getRR := httptest.NewRecorder()
	s.handler.ServeHTTP(getRR, getReq)
	if getRR.Code != http.StatusOK {
		t.Fatalf("expected 200 from get, got %d: %s", getRR.Code, getRR.Body.String())
	}

	var getResp struct {
		Rule struct {
			ID      string `json:"id"`
			Content string `json:"content"`
		} `json:"rule"`
		Previews []struct {
			Target string `json:"target"`
			Files  []struct {
				Path    string `json:"path"`
				Content string `json:"content"`
				Format  string `json:"format"`
			} `json:"files"`
		} `json:"previews"`
	}
	if err := json.Unmarshal(getRR.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("failed to decode get response: %v", err)
	}
	if getResp.Rule.Content != "# Managed\n" {
		t.Fatalf("get response content = %q, want %q", getResp.Rule.Content, "# Managed\n")
	}
	if len(getResp.Previews) != 1 || len(getResp.Previews[0].Files) == 0 {
		t.Fatalf("get previews = %#v, want compiled preview data", getResp.Previews)
	}

	updateReq := httptest.NewRequest(http.MethodPut, "/api/managed/rules/claude/manual.md", strings.NewReader(`{"tool":"claude","relativePath":"claude/manual.md","content":"# Updated\n"}`))
	updateRR := httptest.NewRecorder()
	s.handler.ServeHTTP(updateRR, updateReq)
	if updateRR.Code != http.StatusOK {
		t.Fatalf("expected 200 from update, got %d: %s", updateRR.Code, updateRR.Body.String())
	}

	var updateResp struct {
		Rule struct {
			Content string `json:"content"`
		} `json:"rule"`
	}
	if err := json.Unmarshal(updateRR.Body.Bytes(), &updateResp); err != nil {
		t.Fatalf("failed to decode update response: %v", err)
	}
	if updateResp.Rule.Content != "# Updated\n" {
		t.Fatalf("update response content = %q, want %q", updateResp.Rule.Content, "# Updated\n")
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/managed/rules/claude/manual.md", nil)
	deleteRR := httptest.NewRecorder()
	s.handler.ServeHTTP(deleteRR, deleteReq)
	if deleteRR.Code != http.StatusOK {
		t.Fatalf("expected 200 from delete, got %d: %s", deleteRR.Code, deleteRR.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/managed/rules", nil)
	listRR := httptest.NewRecorder()
	s.handler.ServeHTTP(listRR, listReq)
	if listRR.Code != http.StatusOK {
		t.Fatalf("expected 200 from list, got %d: %s", listRR.Code, listRR.Body.String())
	}
	var listResp struct {
		Rules []struct {
			ID string `json:"id"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(listRR.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}
	if len(listResp.Rules) != 0 {
		t.Fatalf("expected 0 rules after delete, got %d", len(listResp.Rules))
	}

	discoveredPath := filepath.Join(projectRoot, ".claude", "rules", "seed.md")
	if err := os.MkdirAll(filepath.Dir(discoveredPath), 0755); err != nil {
		t.Fatalf("failed to create discovered rule dir: %v", err)
	}
	if err := os.WriteFile(discoveredPath, []byte("# Seed\n"), 0644); err != nil {
		t.Fatalf("failed to write discovered rule: %v", err)
	}

	discovered, _, err := inspect.ScanRules(projectRoot)
	if err != nil {
		t.Fatalf("ScanRules() error = %v", err)
	}
	var discoveredID string
	for _, item := range discovered {
		if item.Path == discoveredPath {
			discoveredID = item.ID
			break
		}
	}
	if discoveredID == "" {
		t.Fatalf("failed to find discovered rule id for %s", discoveredPath)
	}

	collectReq := httptest.NewRequest(http.MethodPost, "/api/managed/rules/collect", strings.NewReader(`{"ids":["`+discoveredID+`"],"strategy":"overwrite"}`))
	collectRR := httptest.NewRecorder()
	s.handler.ServeHTTP(collectRR, collectReq)
	if collectRR.Code != http.StatusOK {
		t.Fatalf("expected 200 from collect, got %d: %s", collectRR.Code, collectRR.Body.String())
	}

	var collectResp struct {
		Created     []string `json:"created"`
		Overwritten []string `json:"overwritten"`
		Skipped     []string `json:"skipped"`
	}
	if err := json.Unmarshal(collectRR.Body.Bytes(), &collectResp); err != nil {
		t.Fatalf("failed to decode collect response: %v", err)
	}
	if len(collectResp.Created) != 1 {
		t.Fatalf("expected one created managed rule, got %#v", collectResp.Created)
	}

	managedPath := filepath.Join(projectRoot, ".skillshare", "rules", filepath.FromSlash(collectResp.Created[0]))
	if _, err := os.Stat(managedPath); err != nil {
		t.Fatalf("expected managed rule file at %s: %v", managedPath, err)
	}

	diffReq := httptest.NewRequest(http.MethodGet, "/api/managed/rules/diff", nil)
	diffRR := httptest.NewRecorder()
	s.handler.ServeHTTP(diffRR, diffReq)
	if diffRR.Code != http.StatusOK {
		t.Fatalf("expected 200 from diff, got %d: %s", diffRR.Code, diffRR.Body.String())
	}

	var diffResp struct {
		Diffs []struct {
			Target string `json:"target"`
			Files  []struct {
				Path    string `json:"path"`
				Content string `json:"content"`
				Format  string `json:"format"`
			} `json:"files"`
		} `json:"diffs"`
	}
	if err := json.Unmarshal(diffRR.Body.Bytes(), &diffResp); err != nil {
		t.Fatalf("failed to decode diff response: %v", err)
	}
	if len(diffResp.Diffs) != 1 || diffResp.Diffs[0].Target != "claude" {
		t.Fatalf("diff response = %#v, want one claude diff", diffResp.Diffs)
	}
	if len(diffResp.Diffs[0].Files) == 0 || diffResp.Diffs[0].Files[0].Path != filepath.Join(projectRoot, ".claude", "rules", filepath.Base(managedPath)) {
		t.Fatalf("diff files = %#v, want compiled preview output under target path", diffResp.Diffs[0].Files)
	}
}

func TestManagedRulesDetailPreviewIncludesFullCodexAggregate(t *testing.T) {
	s, projectRoot, _, _ := newManagedProjectServer(t, "codex")

	for _, body := range []string{
		`{"tool":"codex","relativePath":"codex/one.md","content":"# One\n"}`,
		`{"tool":"codex","relativePath":"codex/two.md","content":"# Two\n"}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/managed/rules", strings.NewReader(body))
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("expected 201 from create, got %d: %s", rr.Code, rr.Body.String())
		}
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/managed/rules/codex/one.md", nil)
	getRR := httptest.NewRecorder()
	s.handler.ServeHTTP(getRR, getReq)
	if getRR.Code != http.StatusOK {
		t.Fatalf("expected 200 from get, got %d: %s", getRR.Code, getRR.Body.String())
	}

	var getResp struct {
		Previews []struct {
			Target string `json:"target"`
			Files  []struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			} `json:"files"`
		} `json:"previews"`
	}
	if err := json.Unmarshal(getRR.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("failed to decode get response: %v", err)
	}

	var codexPreview *struct {
		Target string `json:"target"`
		Files  []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		} `json:"files"`
	}
	for i := range getResp.Previews {
		if getResp.Previews[i].Target == "codex" {
			codexPreview = &getResp.Previews[i]
			break
		}
	}
	if codexPreview == nil {
		t.Fatalf("expected codex preview in %#v", getResp.Previews)
	}
	if len(codexPreview.Files) != 1 {
		t.Fatalf("expected one codex compiled file, got %#v", codexPreview.Files)
	}
	if codexPreview.Files[0].Path != filepath.Join(projectRoot, "AGENTS.md") {
		t.Fatalf("codex preview path = %q, want %q", codexPreview.Files[0].Path, filepath.Join(projectRoot, "AGENTS.md"))
	}
	if !strings.Contains(codexPreview.Files[0].Content, "skillshare:codex/one.md") || !strings.Contains(codexPreview.Files[0].Content, "skillshare:codex/two.md") {
		t.Fatalf("codex preview content = %q, want aggregate output containing both codex rules", codexPreview.Files[0].Content)
	}
}

func TestManagedRulesDiffResolvesAliasTargetToClaudeProjectRuleRoot(t *testing.T) {
	s, projectRoot, _, _ := newManagedProjectServer(t, "claude-code")

	targetPath := filepath.Join(projectRoot, ".claude", "skills")
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		t.Fatalf("failed to create target dir: %v", err)
	}
	s.cfg.Targets["claude-code"] = config.TargetConfig{Path: targetPath}

	createReq := httptest.NewRequest(http.MethodPost, "/api/managed/rules", strings.NewReader(`{"tool":"claude","relativePath":"claude/manual.md","content":"# Managed\n"}`))
	createRR := httptest.NewRecorder()
	s.handler.ServeHTTP(createRR, createReq)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("expected 201 from create, got %d: %s", createRR.Code, createRR.Body.String())
	}

	diffReq := httptest.NewRequest(http.MethodGet, "/api/managed/rules/diff", nil)
	diffRR := httptest.NewRecorder()
	s.handler.ServeHTTP(diffRR, diffReq)
	if diffRR.Code != http.StatusOK {
		t.Fatalf("expected 200 from diff, got %d: %s", diffRR.Code, diffRR.Body.String())
	}

	var diffResp struct {
		Diffs []struct {
			Target string `json:"target"`
			Files  []struct {
				Path string `json:"path"`
			} `json:"files"`
			Warnings []string `json:"warnings"`
		} `json:"diffs"`
	}
	if err := json.Unmarshal(diffRR.Body.Bytes(), &diffResp); err != nil {
		t.Fatalf("failed to decode diff response: %v", err)
	}
	if len(diffResp.Diffs) != 1 || diffResp.Diffs[0].Target != "claude-code" {
		t.Fatalf("diff response = %#v, want one claude-code diff", diffResp.Diffs)
	}
	if len(diffResp.Diffs[0].Warnings) != 0 {
		t.Fatalf("diff warnings = %#v, want none", diffResp.Diffs[0].Warnings)
	}
	if len(diffResp.Diffs[0].Files) != 1 || diffResp.Diffs[0].Files[0].Path != filepath.Join(projectRoot, ".claude", "rules", "manual.md") {
		t.Fatalf("diff files = %#v, want compiled preview output under project rule root", diffResp.Diffs[0].Files)
	}
}

func TestManagedRulesPreviewCompilesSharedAgentsTargetsAtProjectRoot(t *testing.T) {
	s, projectRoot, _, _ := newManagedProjectServer(t, "universal")

	targetPath := filepath.Join(projectRoot, ".agents", "skills")
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		t.Fatalf("failed to create target dir: %v", err)
	}
	s.cfg.Targets["universal"] = config.TargetConfig{Path: targetPath}

	for _, body := range []string{
		`{"tool":"codex","relativePath":"codex/one.md","content":"# One\n"}`,
		`{"tool":"codex","relativePath":"codex/two.md","content":"# Two\n"}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/managed/rules", strings.NewReader(body))
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("expected 201 from create, got %d: %s", rr.Code, rr.Body.String())
		}
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/managed/rules/codex/one.md", nil)
	getRR := httptest.NewRecorder()
	s.handler.ServeHTTP(getRR, getReq)
	if getRR.Code != http.StatusOK {
		t.Fatalf("expected 200 from get, got %d: %s", getRR.Code, getRR.Body.String())
	}

	var getResp struct {
		Previews []struct {
			Target string `json:"target"`
			Files  []struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			} `json:"files"`
			Warnings []string `json:"warnings"`
		} `json:"previews"`
	}
	if err := json.Unmarshal(getRR.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("failed to decode get response: %v", err)
	}

	if len(getResp.Previews) != 1 || getResp.Previews[0].Target != "universal" {
		t.Fatalf("get previews = %#v, want one universal preview", getResp.Previews)
	}
	if len(getResp.Previews[0].Warnings) != 0 {
		t.Fatalf("get preview warnings = %#v, want none", getResp.Previews[0].Warnings)
	}
	if len(getResp.Previews[0].Files) != 1 || getResp.Previews[0].Files[0].Path != filepath.Join(projectRoot, "AGENTS.md") {
		t.Fatalf("get preview files = %#v, want AGENTS.md at project root", getResp.Previews[0].Files)
	}
	if !strings.Contains(getResp.Previews[0].Files[0].Content, "skillshare:codex/one.md") || !strings.Contains(getResp.Previews[0].Files[0].Content, "skillshare:codex/two.md") {
		t.Fatalf("get preview content = %q, want aggregate codex output", getResp.Previews[0].Files[0].Content)
	}
}

func TestManagedRulesDiffResolvesAliasTargetToClaudeGlobalRuleRoot(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "xdg-config"))

	targetPath := filepath.Join(homeDir, ".claude", "skills")
	s, _ := newTestServerWithTargets(t, map[string]string{"claude-code": targetPath})

	createReq := httptest.NewRequest(http.MethodPost, "/api/managed/rules", strings.NewReader(`{"tool":"claude","relativePath":"claude/manual.md","content":"# Managed\n"}`))
	createRR := httptest.NewRecorder()
	s.handler.ServeHTTP(createRR, createReq)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("expected 201 from create, got %d: %s", createRR.Code, createRR.Body.String())
	}

	diffReq := httptest.NewRequest(http.MethodGet, "/api/managed/rules/diff", nil)
	diffRR := httptest.NewRecorder()
	s.handler.ServeHTTP(diffRR, diffReq)
	if diffRR.Code != http.StatusOK {
		t.Fatalf("expected 200 from diff, got %d: %s", diffRR.Code, diffRR.Body.String())
	}

	var diffResp struct {
		Diffs []struct {
			Target string `json:"target"`
			Files  []struct {
				Path string `json:"path"`
			} `json:"files"`
			Warnings []string `json:"warnings"`
		} `json:"diffs"`
	}
	if err := json.Unmarshal(diffRR.Body.Bytes(), &diffResp); err != nil {
		t.Fatalf("failed to decode diff response: %v", err)
	}
	if len(diffResp.Diffs) != 1 || diffResp.Diffs[0].Target != "claude-code" {
		t.Fatalf("diff response = %#v, want one claude-code diff", diffResp.Diffs)
	}
	if len(diffResp.Diffs[0].Warnings) != 0 {
		t.Fatalf("diff warnings = %#v, want none", diffResp.Diffs[0].Warnings)
	}
	if len(diffResp.Diffs[0].Files) != 1 || diffResp.Diffs[0].Files[0].Path != filepath.Join(homeDir, ".claude", "rules", "manual.md") {
		t.Fatalf("diff files = %#v, want compiled output under global claude root", diffResp.Diffs[0].Files)
	}
}

func TestManagedRulesPreviewCompilesSharedAgentsTargetsAtGlobalRoot(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "xdg-config"))

	targetPath := filepath.Join(homeDir, ".agents", "skills")
	s, _ := newTestServerWithTargets(t, map[string]string{"universal": targetPath})

	for _, body := range []string{
		`{"tool":"codex","relativePath":"codex/one.md","content":"# One\n"}`,
		`{"tool":"codex","relativePath":"codex/two.md","content":"# Two\n"}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/managed/rules", strings.NewReader(body))
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("expected 201 from create, got %d: %s", rr.Code, rr.Body.String())
		}
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/managed/rules/codex/one.md", nil)
	getRR := httptest.NewRecorder()
	s.handler.ServeHTTP(getRR, getReq)
	if getRR.Code != http.StatusOK {
		t.Fatalf("expected 200 from get, got %d: %s", getRR.Code, getRR.Body.String())
	}

	var getResp struct {
		Previews []struct {
			Target string `json:"target"`
			Files  []struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			} `json:"files"`
			Warnings []string `json:"warnings"`
		} `json:"previews"`
	}
	if err := json.Unmarshal(getRR.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("failed to decode get response: %v", err)
	}
	if len(getResp.Previews) != 1 || getResp.Previews[0].Target != "universal" {
		t.Fatalf("get previews = %#v, want one universal preview", getResp.Previews)
	}
	if len(getResp.Previews[0].Warnings) != 0 {
		t.Fatalf("get preview warnings = %#v, want none", getResp.Previews[0].Warnings)
	}
	if len(getResp.Previews[0].Files) != 1 || getResp.Previews[0].Files[0].Path != filepath.Join(homeDir, ".agents", "AGENTS.md") {
		t.Fatalf("get preview files = %#v, want AGENTS.md under global .agents root", getResp.Previews[0].Files)
	}
	if !strings.Contains(getResp.Previews[0].Files[0].Content, "skillshare:codex/one.md") || !strings.Contains(getResp.Previews[0].Files[0].Content, "skillshare:codex/two.md") {
		t.Fatalf("get preview content = %q, want aggregate codex output", getResp.Previews[0].Files[0].Content)
	}
}

func TestManagedRulesCreateServerErrorOnWriteFailure(t *testing.T) {
	s, projectRoot, _, _ := newManagedProjectServer(t, "claude")

	blockingPath := filepath.Join(projectRoot, ".skillshare", "rules")
	if err := os.WriteFile(blockingPath, []byte("block"), 0644); err != nil {
		t.Fatalf("failed to create blocking rules file: %v", err)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/managed/rules", strings.NewReader(`{"tool":"claude","relativePath":"claude/manual.md","content":"# Managed\n"}`))
	createRR := httptest.NewRecorder()
	s.handler.ServeHTTP(createRR, createReq)
	if createRR.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 from create, got %d: %s", createRR.Code, createRR.Body.String())
	}
}

func TestManagedRulesCreateRejectsEscapingRelativePath(t *testing.T) {
	s, _, _, _ := newManagedProjectServer(t, "claude")

	createReq := httptest.NewRequest(http.MethodPost, "/api/managed/rules", strings.NewReader(`{"tool":"claude","relativePath":"../foo.md","content":"# Bad\n"}`))
	createRR := httptest.NewRecorder()
	s.handler.ServeHTTP(createRR, createReq)
	if createRR.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 from create, got %d: %s", createRR.Code, createRR.Body.String())
	}
}

func TestManagedRulesCreateRejectsDuplicateRule(t *testing.T) {
	s, _, _, _ := newManagedProjectServer(t, "claude")

	firstReq := httptest.NewRequest(http.MethodPost, "/api/managed/rules", strings.NewReader(`{"tool":"claude","relativePath":"claude/manual.md","content":"# First\n"}`))
	firstRR := httptest.NewRecorder()
	s.handler.ServeHTTP(firstRR, firstReq)
	if firstRR.Code != http.StatusCreated {
		t.Fatalf("expected 201 from first create, got %d: %s", firstRR.Code, firstRR.Body.String())
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/api/managed/rules", strings.NewReader(`{"tool":"claude","relativePath":"claude/manual.md","content":"# Second\n"}`))
	secondRR := httptest.NewRecorder()
	s.handler.ServeHTTP(secondRR, secondReq)
	if secondRR.Code != http.StatusConflict {
		t.Fatalf("expected 409 from duplicate create, got %d: %s", secondRR.Code, secondRR.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/managed/rules/claude/manual.md", nil)
	getRR := httptest.NewRecorder()
	s.handler.ServeHTTP(getRR, getReq)
	if getRR.Code != http.StatusOK {
		t.Fatalf("expected 200 from get, got %d: %s", getRR.Code, getRR.Body.String())
	}

	var getResp struct {
		Rule struct {
			Content string `json:"content"`
		} `json:"rule"`
	}
	if err := json.Unmarshal(getRR.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("failed to decode get response: %v", err)
	}
	if getResp.Rule.Content != "# First\n" {
		t.Fatalf("rule content after duplicate create = %q, want original content", getResp.Rule.Content)
	}
}

func TestManagedRulesCreateRejectsMissingContent(t *testing.T) {
	s, _, _, _ := newManagedProjectServer(t, "claude")

	req := httptest.NewRequest(http.MethodPost, "/api/managed/rules", strings.NewReader(`{"tool":"claude","relativePath":"claude/manual.md"}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 from create, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestManagedRulesCreateRejectsInvalidOrUnsupportedTool(t *testing.T) {
	s, _, _, _ := newManagedProjectServer(t, "claude")

	for _, tool := range []string{"foo", "foo/bar", "foo/../codex"} {
		t.Run(tool, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/managed/rules", strings.NewReader(`{"tool":"`+tool+`","relativePath":"manual.md","content":"# Bad\n"}`))
			rr := httptest.NewRecorder()
			s.handler.ServeHTTP(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("tool %q: expected 400 from create, got %d: %s", tool, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestManagedRulesCreateRejectsUnsupportedIDOnlyRule(t *testing.T) {
	s, _, _, _ := newManagedProjectServer(t, "claude")

	req := httptest.NewRequest(http.MethodPost, "/api/managed/rules", strings.NewReader(`{"id":"foo/bar.md","content":"# Bad\n"}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 from create, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestManagedRulesUpdateRejectsUnsupportedIDOnlyRule(t *testing.T) {
	s, _, _, _ := newManagedProjectServer(t, "claude")

	createReq := httptest.NewRequest(http.MethodPost, "/api/managed/rules", strings.NewReader(`{"tool":"claude","relativePath":"claude/good.md","content":"# Good\n"}`))
	createRR := httptest.NewRecorder()
	s.handler.ServeHTTP(createRR, createReq)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("expected 201 from create, got %d: %s", createRR.Code, createRR.Body.String())
	}

	updateReq := httptest.NewRequest(http.MethodPut, "/api/managed/rules/foo/bar.md", strings.NewReader(`{"id":"foo/bar.md","content":"# Bad\n"}`))
	updateRR := httptest.NewRecorder()
	s.handler.ServeHTTP(updateRR, updateReq)
	if updateRR.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 from update, got %d: %s", updateRR.Code, updateRR.Body.String())
	}
}

func TestManagedRulesCreateRejectsBareToolPrefixAndReservedTempID(t *testing.T) {
	s, _, _, _ := newManagedProjectServer(t, "claude")

	for name, body := range map[string]string{
		"bare tool prefix":   `{"id":"claude","content":"# Bad\n"}`,
		"reserved temp path": `{"tool":"claude","relativePath":"claude/.rule-tmp-manual.md","content":"# Bad\n"}`,
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/managed/rules", strings.NewReader(body))
			rr := httptest.NewRecorder()
			s.handler.ServeHTTP(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 from create, got %d: %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestManagedRulesUnsupportedTargetPreviewWarning(t *testing.T) {
	s, _, _, _ := newManagedProjectServer(t, "cursor")

	createReq := httptest.NewRequest(http.MethodPost, "/api/managed/rules", strings.NewReader(`{"tool":"claude","relativePath":"claude/manual.md","content":"# Managed\n"}`))
	createRR := httptest.NewRecorder()
	s.handler.ServeHTTP(createRR, createReq)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("expected 201 from create, got %d: %s", createRR.Code, createRR.Body.String())
	}

	var createResp struct {
		Previews []struct {
			Target   string     `json:"target"`
			Files    []struct{} `json:"files"`
			Warnings []string   `json:"warnings"`
		} `json:"previews"`
	}
	if err := json.Unmarshal(createRR.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("failed to decode create response: %v", err)
	}
	if len(createResp.Previews) != 1 || createResp.Previews[0].Target != "cursor" {
		t.Fatalf("create previews = %#v, want one cursor preview", createResp.Previews)
	}
	if len(createResp.Previews[0].Files) != 0 {
		t.Fatalf("create preview files = %#v, want empty files for unsupported target", createResp.Previews[0].Files)
	}
	if len(createResp.Previews[0].Warnings) == 0 {
		t.Fatalf("create preview warnings = %#v, want unsupported-target warning", createResp.Previews[0].Warnings)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/managed/rules/claude/manual.md", nil)
	getRR := httptest.NewRecorder()
	s.handler.ServeHTTP(getRR, getReq)
	if getRR.Code != http.StatusOK {
		t.Fatalf("expected 200 from get, got %d: %s", getRR.Code, getRR.Body.String())
	}

	var getResp struct {
		Previews []struct {
			Target   string     `json:"target"`
			Files    []struct{} `json:"files"`
			Warnings []string   `json:"warnings"`
		} `json:"previews"`
	}
	if err := json.Unmarshal(getRR.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("failed to decode get response: %v", err)
	}
	if len(getResp.Previews) != 1 || getResp.Previews[0].Target != "cursor" {
		t.Fatalf("get previews = %#v, want one cursor preview", getResp.Previews)
	}
	if len(getResp.Previews[0].Files) != 0 {
		t.Fatalf("get preview files = %#v, want empty files for unsupported target", getResp.Previews[0].Files)
	}
	if len(getResp.Previews[0].Warnings) == 0 {
		t.Fatalf("get preview warnings = %#v, want unsupported-target warning", getResp.Previews[0].Warnings)
	}
}

func TestManagedRulesCreateRejectsWindowsStyleRelativePath(t *testing.T) {
	s, _, _, _ := newManagedProjectServer(t, "claude")

	for _, relativePath := range []string{
		"C:/outside.md",
		"C:outside.md",
		"claude/C:/outside.md",
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/managed/rules", strings.NewReader(`{"tool":"claude","relativePath":"`+relativePath+`","content":"# Bad\n"}`))
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("relativePath %q: expected 400 from create, got %d: %s", relativePath, rr.Code, rr.Body.String())
		}
	}
}

func TestManagedRulesCreateRejectsInvalidRelativePathWhenIDProvided(t *testing.T) {
	s, _, _, _ := newManagedProjectServer(t, "claude")

	req := httptest.NewRequest(http.MethodPost, "/api/managed/rules", strings.NewReader(`{"id":"claude/good.md","tool":"claude","relativePath":"../escape.md","content":"# Bad\n"}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 from create, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestManagedRulesUpdateRejectsMismatchedIDAndRelativePath(t *testing.T) {
	s, _, _, _ := newManagedProjectServer(t, "claude")

	createReq := httptest.NewRequest(http.MethodPost, "/api/managed/rules", strings.NewReader(`{"tool":"claude","relativePath":"claude/good.md","content":"# Good\n"}`))
	createRR := httptest.NewRecorder()
	s.handler.ServeHTTP(createRR, createReq)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("expected 201 from create, got %d: %s", createRR.Code, createRR.Body.String())
	}

	updateReq := httptest.NewRequest(http.MethodPut, "/api/managed/rules/claude/good.md", strings.NewReader(`{"id":"claude/good.md","tool":"claude","relativePath":"claude/other.md","content":"# Bad\n"}`))
	updateRR := httptest.NewRecorder()
	s.handler.ServeHTTP(updateRR, updateReq)
	if updateRR.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 from update, got %d: %s", updateRR.Code, updateRR.Body.String())
	}
}

func TestManagedRulesUpdateRejectsMissingContent(t *testing.T) {
	s, _, _, _ := newManagedProjectServer(t, "claude")

	createReq := httptest.NewRequest(http.MethodPost, "/api/managed/rules", strings.NewReader(`{"tool":"claude","relativePath":"claude/manual.md","content":"# Managed\n"}`))
	createRR := httptest.NewRecorder()
	s.handler.ServeHTTP(createRR, createReq)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("expected 201 from create, got %d: %s", createRR.Code, createRR.Body.String())
	}

	updateReq := httptest.NewRequest(http.MethodPut, "/api/managed/rules/claude/manual.md", strings.NewReader(`{"tool":"claude","relativePath":"claude/manual.md"}`))
	updateRR := httptest.NewRecorder()
	s.handler.ServeHTTP(updateRR, updateReq)
	if updateRR.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 from update, got %d: %s", updateRR.Code, updateRR.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/managed/rules/claude/manual.md", nil)
	getRR := httptest.NewRecorder()
	s.handler.ServeHTTP(getRR, getReq)
	if getRR.Code != http.StatusOK {
		t.Fatalf("expected 200 from get, got %d: %s", getRR.Code, getRR.Body.String())
	}

	var getResp struct {
		Rule struct {
			Content string `json:"content"`
		} `json:"rule"`
	}
	if err := json.Unmarshal(getRR.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("failed to decode get response: %v", err)
	}
	if getResp.Rule.Content != "# Managed\n" {
		t.Fatalf("rule content after rejected update = %q, want original content", getResp.Rule.Content)
	}
}

func TestManagedRulesUpdateMissingRuleReturnsNotFound(t *testing.T) {
	s, _, _, _ := newManagedProjectServer(t, "claude")

	req := httptest.NewRequest(http.MethodPut, "/api/managed/rules/claude/missing.md", strings.NewReader(`{"id":"claude/missing.md","tool":"claude","relativePath":"claude/missing.md","content":"# Missing\n"}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 from update, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestManagedRulesCreateRejectsUnknownFieldsAndTrailingJSON(t *testing.T) {
	s, _, _, _ := newManagedProjectServer(t, "claude")

	for name, body := range map[string]string{
		"unknown field": `{"tool":"claude","relativePath":"claude/manual.md","content":"# Managed\n","extra":true}`,
		"trailing json": `{"tool":"claude","relativePath":"claude/manual.md","content":"# Managed\n"}{"extra":true}`,
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/managed/rules", strings.NewReader(body))
			rr := httptest.NewRecorder()
			s.handler.ServeHTTP(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 from create, got %d: %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestManagedRulesUpdateRejectsUnknownFieldsAndTrailingJSON(t *testing.T) {
	s, _, _, _ := newManagedProjectServer(t, "claude")

	createReq := httptest.NewRequest(http.MethodPost, "/api/managed/rules", strings.NewReader(`{"tool":"claude","relativePath":"claude/manual.md","content":"# Managed\n"}`))
	createRR := httptest.NewRecorder()
	s.handler.ServeHTTP(createRR, createReq)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("expected 201 from create, got %d: %s", createRR.Code, createRR.Body.String())
	}

	for name, body := range map[string]string{
		"unknown field": `{"tool":"claude","relativePath":"claude/manual.md","content":"# Updated\n","extra":true}`,
		"trailing json": `{"tool":"claude","relativePath":"claude/manual.md","content":"# Updated\n"}{"extra":true}`,
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/api/managed/rules/claude/manual.md", strings.NewReader(body))
			rr := httptest.NewRecorder()
			s.handler.ServeHTTP(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 from update, got %d: %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestManagedRulesCollectRejectsUnknownFieldsAndTrailingJSON(t *testing.T) {
	s, projectRoot, _, _ := newManagedProjectServer(t, "claude")

	discoveredPath := filepath.Join(projectRoot, ".claude", "rules", "seed.md")
	if err := os.MkdirAll(filepath.Dir(discoveredPath), 0755); err != nil {
		t.Fatalf("failed to create discovered rule dir: %v", err)
	}
	if err := os.WriteFile(discoveredPath, []byte("# Seed\n"), 0644); err != nil {
		t.Fatalf("failed to write discovered rule: %v", err)
	}

	discovered, _, err := inspect.ScanRules(projectRoot)
	if err != nil {
		t.Fatalf("ScanRules() error = %v", err)
	}
	var discoveredID string
	for _, item := range discovered {
		if item.Path == discoveredPath {
			discoveredID = item.ID
			break
		}
	}
	if discoveredID == "" {
		t.Fatalf("failed to find discovered rule id for %s", discoveredPath)
	}

	for name, body := range map[string]string{
		"unknown field": `{"ids":["` + discoveredID + `"],"strategy":"overwrite","extra":true}`,
		"trailing json": `{"ids":["` + discoveredID + `"],"strategy":"overwrite"}{"extra":true}`,
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/managed/rules/collect", strings.NewReader(body))
			rr := httptest.NewRecorder()
			s.handler.ServeHTTP(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 from collect, got %d: %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestManagedRulesCollectDedupesRepeatedIDs(t *testing.T) {
	s, projectRoot, _, _ := newManagedProjectServer(t, "claude")

	discoveredPath := filepath.Join(projectRoot, ".claude", "rules", "seed.md")
	if err := os.MkdirAll(filepath.Dir(discoveredPath), 0755); err != nil {
		t.Fatalf("failed to create discovered rule dir: %v", err)
	}
	if err := os.WriteFile(discoveredPath, []byte("# Seed\n"), 0644); err != nil {
		t.Fatalf("failed to write discovered rule: %v", err)
	}

	discovered, _, err := inspect.ScanRules(projectRoot)
	if err != nil {
		t.Fatalf("ScanRules() error = %v", err)
	}
	var discoveredID string
	for _, item := range discovered {
		if item.Path == discoveredPath {
			discoveredID = item.ID
			break
		}
	}
	if discoveredID == "" {
		t.Fatalf("failed to find discovered rule id for %s", discoveredPath)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/managed/rules/collect", strings.NewReader(`{"ids":["`+discoveredID+`","`+discoveredID+`"],"strategy":"overwrite"}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 from collect, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Created     []string `json:"created"`
		Overwritten []string `json:"overwritten"`
		Skipped     []string `json:"skipped"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode collect response: %v", err)
	}
	if len(resp.Created) != 1 || resp.Created[0] != "claude/seed.md" {
		t.Fatalf("collect created = %#v, want one created managed rule", resp.Created)
	}
	if len(resp.Overwritten) != 0 {
		t.Fatalf("collect overwritten = %#v, want none after dedupe", resp.Overwritten)
	}
	if len(resp.Skipped) != 0 {
		t.Fatalf("collect skipped = %#v, want none after dedupe", resp.Skipped)
	}
}
