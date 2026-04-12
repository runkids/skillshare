package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/inspect"
	managedhooks "skillshare/internal/resources/hooks"
)

func canonicalManagedHookID(t *testing.T, tool, event, matcher string) string {
	t.Helper()
	id, err := managedhooks.CanonicalRelativePath(tool, event, matcher)
	if err != nil {
		t.Fatalf("failed to derive canonical managed hook id: %v", err)
	}
	return id
}

func TestManagedHooksCRUDAndDiff(t *testing.T) {
	s, projectRoot, _, _ := newManagedProjectServer(t, "claude")
	hookID := canonicalManagedHookID(t, "claude", "PreToolUse", "Bash")

	createBody := `{"tool":"claude","event":"PreToolUse","matcher":"Bash","handlers":[{"type":"command","command":"./bin/check","statusMessage":"Checking"}]}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/managed/hooks", strings.NewReader(createBody))
	createRR := httptest.NewRecorder()
	s.handler.ServeHTTP(createRR, createReq)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("expected 201 from create, got %d: %s", createRR.Code, createRR.Body.String())
	}

	var createResp struct {
		Hook struct {
			ID      string `json:"id"`
			Event   string `json:"event"`
			Matcher string `json:"matcher"`
		} `json:"hook"`
		Previews []struct {
			Target string `json:"target"`
			Files  []struct {
				Path string `json:"path"`
			} `json:"files"`
			Warnings []string `json:"warnings"`
		} `json:"previews"`
	}
	if err := json.Unmarshal(createRR.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("failed to decode create response: %v", err)
	}
	if createResp.Hook.ID != hookID {
		t.Fatalf("create response hook id = %q, want %q", createResp.Hook.ID, hookID)
	}
	if len(createResp.Previews) != 1 || createResp.Previews[0].Target != "claude" {
		t.Fatalf("create previews = %#v, want one claude preview", createResp.Previews)
	}
	if len(createResp.Previews[0].Warnings) != 0 {
		t.Fatalf("create preview warnings = %#v, want none", createResp.Previews[0].Warnings)
	}
	if len(createResp.Previews[0].Files) != 1 || createResp.Previews[0].Files[0].Path != filepath.Join(projectRoot, ".claude", "settings.json") {
		t.Fatalf("create preview files = %#v, want compiled claude settings path under project root", createResp.Previews[0].Files)
	}

	dupReq := httptest.NewRequest(http.MethodPost, "/api/managed/hooks", strings.NewReader(createBody))
	dupRR := httptest.NewRecorder()
	s.handler.ServeHTTP(dupRR, dupReq)
	if dupRR.Code != http.StatusConflict {
		t.Fatalf("expected 409 from duplicate create, got %d: %s", dupRR.Code, dupRR.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/managed/hooks/"+hookID, nil)
	getRR := httptest.NewRecorder()
	s.handler.ServeHTTP(getRR, getReq)
	if getRR.Code != http.StatusOK {
		t.Fatalf("expected 200 from get, got %d: %s", getRR.Code, getRR.Body.String())
	}

	var getResp struct {
		Hook struct {
			ID       string `json:"id"`
			Event    string `json:"event"`
			Matcher  string `json:"matcher"`
			Handlers []struct {
				Type    string `json:"type"`
				Command string `json:"command"`
			} `json:"handlers"`
		} `json:"hook"`
	}
	if err := json.Unmarshal(getRR.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("failed to decode get response: %v", err)
	}
	if getResp.Hook.ID != hookID || getResp.Hook.Event != "PreToolUse" || getResp.Hook.Matcher != "Bash" {
		t.Fatalf("get hook = %#v, want id/event/matcher round-trip", getResp.Hook)
	}
	if len(getResp.Hook.Handlers) != 1 || getResp.Hook.Handlers[0].Command != "./bin/check" {
		t.Fatalf("get handlers = %#v, want command handler", getResp.Hook.Handlers)
	}

	updateBody := `{"tool":"claude","event":"PreToolUse","matcher":"Bash","handlers":[{"type":"command","command":"./bin/updated","statusMessage":"Updated"}]}`
	updateReq := httptest.NewRequest(http.MethodPut, "/api/managed/hooks/"+hookID, strings.NewReader(updateBody))
	updateRR := httptest.NewRecorder()
	s.handler.ServeHTTP(updateRR, updateReq)
	if updateRR.Code != http.StatusOK {
		t.Fatalf("expected 200 from update, got %d: %s", updateRR.Code, updateRR.Body.String())
	}

	diffReq := httptest.NewRequest(http.MethodGet, "/api/managed/hooks/diff", nil)
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
	if len(diffResp.Diffs[0].Files) != 1 || diffResp.Diffs[0].Files[0].Path != filepath.Join(projectRoot, ".claude", "settings.json") {
		t.Fatalf("diff files = %#v, want canonical claude settings path under project root", diffResp.Diffs[0].Files)
	}
	if !strings.Contains(diffResp.Diffs[0].Files[0].Content, "./bin/updated") {
		t.Fatalf("diff content = %q, want updated command", diffResp.Diffs[0].Files[0].Content)
	}

	renameID, err := managedhooks.CanonicalRelativePath("claude", "PreToolUse", "Write")
	if err != nil {
		t.Fatalf("failed to derive renamed hook id: %v", err)
	}
	renameReq := httptest.NewRequest(http.MethodPut, "/api/managed/hooks/"+hookID, strings.NewReader(`{"tool":"claude","event":"PreToolUse","matcher":"Write","handlers":[{"type":"command","command":"./bin/renamed","statusMessage":"Updated"}]}`))
	renameRR := httptest.NewRecorder()
	s.handler.ServeHTTP(renameRR, renameReq)
	if renameRR.Code != http.StatusOK {
		t.Fatalf("expected 200 from rename update, got %d: %s", renameRR.Code, renameRR.Body.String())
	}

	var renameResp struct {
		Hook struct {
			ID      string `json:"id"`
			Matcher string `json:"matcher"`
		} `json:"hook"`
	}
	if err := json.Unmarshal(renameRR.Body.Bytes(), &renameResp); err != nil {
		t.Fatalf("failed to decode rename response: %v", err)
	}
	if renameResp.Hook.ID != renameID || renameResp.Hook.Matcher != "Write" {
		t.Fatalf("rename response hook = %#v, want id %q and matcher Write", renameResp.Hook, renameID)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".skillshare", "hooks", filepath.FromSlash(renameID))); err != nil {
		t.Fatalf("expected renamed managed hook file %s: %v", renameID, err)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".skillshare", "hooks", "claude", "pre-tool-use", "bash.yaml")); !os.IsNotExist(err) {
		t.Fatalf("expected old managed hook file to be removed, got err=%v", err)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/managed/hooks/"+renameID, nil)
	deleteRR := httptest.NewRecorder()
	s.handler.ServeHTTP(deleteRR, deleteReq)
	if deleteRR.Code != http.StatusOK {
		t.Fatalf("expected 200 from delete, got %d: %s", deleteRR.Code, deleteRR.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/managed/hooks", nil)
	listRR := httptest.NewRecorder()
	s.handler.ServeHTTP(listRR, listReq)
	if listRR.Code != http.StatusOK {
		t.Fatalf("expected 200 from list, got %d: %s", listRR.Code, listRR.Body.String())
	}
	var listResp struct {
		Hooks []struct {
			ID string `json:"id"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(listRR.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}
	if len(listResp.Hooks) != 0 {
		t.Fatalf("expected 0 hooks after delete, got %d", len(listResp.Hooks))
	}
}

func TestHandleManagedHooks_CreateAndUpdateExposeMetadata(t *testing.T) {
	s, _, _, _ := newManagedProjectServer(t, "claude")
	hookID := canonicalManagedHookID(t, "claude", "PreToolUse", "Bash")

	createReq := httptest.NewRequest(http.MethodPost, "/api/managed/hooks", strings.NewReader(`{"tool":"claude","event":"PreToolUse","matcher":"Bash","targets":["claude-work"],"sourceType":"tracked","disabled":true,"handlers":[{"type":"command","command":"./bin/check"}]}`))
	createRR := httptest.NewRecorder()
	s.handler.ServeHTTP(createRR, createReq)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("expected 201 from create, got %d: %s", createRR.Code, createRR.Body.String())
	}

	var createResp struct {
		Hook struct {
			Targets    []string `json:"targets"`
			SourceType string   `json:"sourceType"`
			Disabled   bool     `json:"disabled"`
		} `json:"hook"`
	}
	if err := json.Unmarshal(createRR.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("failed to decode create response: %v", err)
	}
	if len(createResp.Hook.Targets) != 1 || createResp.Hook.Targets[0] != "claude-work" {
		t.Fatalf("create targets = %v, want [claude-work]", createResp.Hook.Targets)
	}
	if createResp.Hook.SourceType != "tracked" {
		t.Fatalf("create sourceType = %q, want %q", createResp.Hook.SourceType, "tracked")
	}
	if !createResp.Hook.Disabled {
		t.Fatalf("create disabled = %v, want true", createResp.Hook.Disabled)
	}

	updateReq := httptest.NewRequest(http.MethodPut, "/api/managed/hooks/"+url.PathEscape(hookID), strings.NewReader(`{"tool":"claude","event":"PreToolUse","matcher":"Bash","handlers":[{"type":"command","command":"./bin/updated"}]}`))
	updateRR := httptest.NewRecorder()
	s.handler.ServeHTTP(updateRR, updateReq)
	if updateRR.Code != http.StatusOK {
		t.Fatalf("expected 200 from update, got %d: %s", updateRR.Code, updateRR.Body.String())
	}

	var updateResp struct {
		Hook struct {
			Targets    []string `json:"targets"`
			SourceType string   `json:"sourceType"`
			Disabled   bool     `json:"disabled"`
		} `json:"hook"`
	}
	if err := json.Unmarshal(updateRR.Body.Bytes(), &updateResp); err != nil {
		t.Fatalf("failed to decode update response: %v", err)
	}
	if len(updateResp.Hook.Targets) != 1 || updateResp.Hook.Targets[0] != "claude-work" {
		t.Fatalf("update targets = %v, want [claude-work]", updateResp.Hook.Targets)
	}
	if updateResp.Hook.SourceType != "tracked" {
		t.Fatalf("update sourceType = %q, want %q", updateResp.Hook.SourceType, "tracked")
	}
	if !updateResp.Hook.Disabled {
		t.Fatalf("update disabled = %v, want true", updateResp.Hook.Disabled)
	}
}

func TestHandleManagedHookDisabled_PersistsDisabledState(t *testing.T) {
	s, projectRoot, _, _ := newManagedProjectServer(t, "claude")
	store := managedhooks.NewStore(projectRoot)
	hookID := canonicalManagedHookID(t, "claude", "PreToolUse", "Bash")
	if _, err := store.Put(managedhooks.Save{
		ID:      hookID,
		Tool:    "claude",
		Event:   "PreToolUse",
		Matcher: "Bash",
		Handlers: []managedhooks.Handler{{
			Type:    "command",
			Command: "./bin/check",
		}},
	}); err != nil {
		t.Fatalf("put hook: %v", err)
	}

	req := httptest.NewRequest(http.MethodPatch, "/api/managed/hooks/"+url.PathEscape(hookID)+"/disabled", strings.NewReader(`{"disabled":true}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	got, err := store.Get(hookID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !got.Disabled {
		t.Fatal("expected hook to be disabled")
	}
}

func TestManagedHooksGeminiCreateGetAndDiffPreserveMetadata(t *testing.T) {
	s, projectRoot, _, _ := newManagedProjectServer(t, "gemini")
	targetPath := filepath.Join(projectRoot, ".gemini", "skills")
	if err := os.MkdirAll(targetPath, 0o755); err != nil {
		t.Fatalf("failed to create gemini target dir: %v", err)
	}
	s.cfg.Targets["gemini"] = config.TargetConfig{Path: targetPath}

	createBody := `{"tool":"gemini","event":"BeforeTool","matcher":"Read","sequential":true,"handlers":[{"type":"command","name":"lint-read","description":"Run read lint","command":"./bin/gemini-lint","timeout":"30000"}]}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/managed/hooks", strings.NewReader(createBody))
	createRR := httptest.NewRecorder()
	s.handler.ServeHTTP(createRR, createReq)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("expected 201 from gemini create, got %d: %s", createRR.Code, createRR.Body.String())
	}

	var createResp struct {
		Hook struct {
			ID         string `json:"id"`
			Sequential *bool  `json:"sequential"`
			Handlers   []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				Timeout     string `json:"timeout"`
			} `json:"handlers"`
		} `json:"hook"`
		Previews []struct {
			Target string `json:"target"`
			Files  []struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			} `json:"files"`
		} `json:"previews"`
	}
	if err := json.Unmarshal(createRR.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("failed to decode gemini create response: %v", err)
	}
	if createResp.Hook.Sequential == nil || !*createResp.Hook.Sequential {
		t.Fatalf("create sequential = %#v, want true", createResp.Hook.Sequential)
	}
	if len(createResp.Hook.Handlers) != 1 {
		t.Fatalf("create handlers = %#v, want one handler", createResp.Hook.Handlers)
	}
	if createResp.Hook.Handlers[0].Name != "lint-read" || createResp.Hook.Handlers[0].Description != "Run read lint" || createResp.Hook.Handlers[0].Timeout != "30000" {
		t.Fatalf("create gemini handler metadata = %#v, want preserved metadata", createResp.Hook.Handlers[0])
	}
	if len(createResp.Previews) != 1 || createResp.Previews[0].Target != "gemini" {
		t.Fatalf("create previews = %#v, want one gemini preview", createResp.Previews)
	}
	if len(createResp.Previews[0].Files) != 1 || createResp.Previews[0].Files[0].Path != filepath.Join(projectRoot, ".gemini", "settings.json") {
		t.Fatalf("create preview files = %#v, want gemini settings path", createResp.Previews[0].Files)
	}
	for _, want := range []string{`"sequential":true`, `"name":"lint-read"`, `"description":"Run read lint"`} {
		if !strings.Contains(createResp.Previews[0].Files[0].Content, want) {
			t.Fatalf("gemini preview content missing %q: %q", want, createResp.Previews[0].Files[0].Content)
		}
	}

	hookID := canonicalManagedHookID(t, "gemini", "BeforeTool", "Read")
	getReq := httptest.NewRequest(http.MethodGet, "/api/managed/hooks/"+hookID, nil)
	getRR := httptest.NewRecorder()
	s.handler.ServeHTTP(getRR, getReq)
	if getRR.Code != http.StatusOK {
		t.Fatalf("expected 200 from gemini get, got %d: %s", getRR.Code, getRR.Body.String())
	}

	diffReq := httptest.NewRequest(http.MethodGet, "/api/managed/hooks/diff", nil)
	diffRR := httptest.NewRecorder()
	s.handler.ServeHTTP(diffRR, diffReq)
	if diffRR.Code != http.StatusOK {
		t.Fatalf("expected 200 from gemini diff, got %d: %s", diffRR.Code, diffRR.Body.String())
	}
}

func TestManagedHooksCreateRejectsGeminiInvalidTimeoutEvenWhenTimeoutSecPresent(t *testing.T) {
	s, _, _, _ := newManagedProjectServer(t, "gemini")

	createReq := httptest.NewRequest(http.MethodPost, "/api/managed/hooks", strings.NewReader(`{"tool":"gemini","event":"BeforeTool","matcher":"Read","handlers":[{"type":"command","command":"./bin/check","timeout":"30s","timeoutSec":30000}]}`))
	createRR := httptest.NewRecorder()
	s.handler.ServeHTTP(createRR, createReq)
	if createRR.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 from gemini create with invalid timeout, got %d: %s", createRR.Code, createRR.Body.String())
	}
	if !strings.Contains(createRR.Body.String(), "timeout must be numeric milliseconds for gemini") {
		t.Fatalf("create error = %q, want gemini timeout validation message", createRR.Body.String())
	}
}

func TestManagedHooksGeminiCreateSupportsEmptyMatcher(t *testing.T) {
	s, projectRoot, _, _ := newManagedProjectServer(t, "gemini")
	targetPath := filepath.Join(projectRoot, ".gemini", "skills")
	if err := os.MkdirAll(targetPath, 0o755); err != nil {
		t.Fatalf("failed to create gemini target dir: %v", err)
	}
	s.cfg.Targets["gemini"] = config.TargetConfig{Path: targetPath}

	createReq := httptest.NewRequest(http.MethodPost, "/api/managed/hooks", strings.NewReader(`{"tool":"gemini","event":"Notification","handlers":[{"type":"command","command":"./bin/notify"}]}`))
	createRR := httptest.NewRecorder()
	s.handler.ServeHTTP(createRR, createReq)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("expected 201 from matcherless gemini create, got %d: %s", createRR.Code, createRR.Body.String())
	}

	var createResp struct {
		Hook struct {
			ID      string `json:"id"`
			Matcher string `json:"matcher"`
		} `json:"hook"`
		Previews []struct {
			Target string `json:"target"`
			Files  []struct {
				Content string `json:"content"`
			} `json:"files"`
		} `json:"previews"`
	}
	if err := json.Unmarshal(createRR.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("failed to decode matcherless gemini create response: %v", err)
	}
	wantID := canonicalManagedHookID(t, "gemini", "Notification", "")
	if createResp.Hook.ID != wantID {
		t.Fatalf("create hook id = %q, want %q", createResp.Hook.ID, wantID)
	}
	if createResp.Hook.Matcher != "" {
		t.Fatalf("create matcher = %q, want empty", createResp.Hook.Matcher)
	}
	if len(createResp.Previews) != 1 || len(createResp.Previews[0].Files) != 1 {
		t.Fatalf("create previews = %#v, want one gemini preview file", createResp.Previews)
	}
	if strings.Contains(createResp.Previews[0].Files[0].Content, `"matcher"`) {
		t.Fatalf("matcherless gemini preview should omit matcher field: %q", createResp.Previews[0].Files[0].Content)
	}
}

func TestManagedHooksCollectRoute(t *testing.T) {
	s, projectRoot, _, _ := newManagedProjectServer(t, "claude")

	discoveredPath := filepath.Join(projectRoot, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(discoveredPath), 0755); err != nil {
		t.Fatalf("failed to create discovered hook dir: %v", err)
	}
	if err := os.WriteFile(discoveredPath, []byte(`{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"./bin/check"}]}],"PostToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"./bin/post"}]}]}}`), 0644); err != nil {
		t.Fatalf("failed to write discovered hook config: %v", err)
	}

	discovered, _, err := inspect.ScanHooks(projectRoot)
	if err != nil {
		t.Fatalf("ScanHooks() error = %v", err)
	}

	var preGroupID, postGroupID string
	for _, item := range discovered {
		if item.Path != discoveredPath {
			continue
		}
		if item.Event == "PreToolUse" {
			preGroupID = item.GroupID
		}
		if item.Event == "PostToolUse" {
			postGroupID = item.GroupID
		}
	}
	if preGroupID == "" || postGroupID == "" {
		t.Fatalf("failed to find discovered hook groups for %s (pre=%q post=%q)", discoveredPath, preGroupID, postGroupID)
	}

	collectReq := httptest.NewRequest(http.MethodPost, "/api/managed/hooks/collect", strings.NewReader(`{"groupIds":["`+preGroupID+`","`+preGroupID+`","`+postGroupID+`"],"strategy":"overwrite"}`))
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
	if len(collectResp.Created) != 2 {
		t.Fatalf("collect created = %#v, want exactly two created managed hooks after dedupe", collectResp.Created)
	}
	if !strings.Contains(collectResp.Created[0], "/pre-tool-use/") || !strings.Contains(collectResp.Created[1], "/post-tool-use/") {
		t.Fatalf("collect created order = %#v, want first-seen group order", collectResp.Created)
	}
	if len(collectResp.Overwritten) != 0 {
		t.Fatalf("collect overwritten = %#v, want none", collectResp.Overwritten)
	}
	if len(collectResp.Skipped) != 0 {
		t.Fatalf("collect skipped = %#v, want none", collectResp.Skipped)
	}

	for _, managedID := range collectResp.Created {
		managedPath := filepath.Join(projectRoot, ".skillshare", "hooks", filepath.FromSlash(managedID))
		if _, err := os.Stat(managedPath); err != nil {
			t.Fatalf("expected managed hook file at %s: %v", managedPath, err)
		}
	}

	for name, body := range map[string]string{
		"unknown group id":  `{"groupIds":["unknown-group"],"strategy":"overwrite"}`,
		"unknown field":     `{"groupIds":["` + preGroupID + `"],"strategy":"overwrite","extra":true}`,
		"trailing json":     `{"groupIds":["` + preGroupID + `"],"strategy":"overwrite"}{"extra":true}`,
		"missing group ids": `{"strategy":"overwrite"}`,
		"invalid strategy":  `{"groupIds":["` + preGroupID + `"],"strategy":"invalid"}`,
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/managed/hooks/collect", strings.NewReader(body))
			rr := httptest.NewRecorder()
			s.handler.ServeHTTP(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 from collect, got %d: %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestManagedHooksUpdateUsesPathIDAndExistingRecord(t *testing.T) {
	s, _, _, _ := newManagedProjectServer(t, "claude")
	hookID := canonicalManagedHookID(t, "claude", "PreToolUse", "Bash")

	createReq := httptest.NewRequest(http.MethodPost, "/api/managed/hooks", strings.NewReader(`{"tool":"claude","event":"PreToolUse","matcher":"Bash","handlers":[{"type":"command","command":"./bin/check"}]}`))
	createRR := httptest.NewRecorder()
	s.handler.ServeHTTP(createRR, createReq)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("expected 201 from create, got %d: %s", createRR.Code, createRR.Body.String())
	}

	updateReq := httptest.NewRequest(http.MethodPut, "/api/managed/hooks/"+hookID, strings.NewReader(`{"tool":"claude","event":"PreToolUse","matcher":"Bash","handlers":[{"type":"command","command":"./bin/check"}]}`))
	updateRR := httptest.NewRecorder()
	s.handler.ServeHTTP(updateRR, updateReq)
	if updateRR.Code != http.StatusOK {
		t.Fatalf("expected 200 from update without body id, got %d: %s", updateRR.Code, updateRR.Body.String())
	}

	missingReq := httptest.NewRequest(http.MethodPut, "/api/managed/hooks/claude/pre-tool-use/missing.yaml", strings.NewReader(`{"tool":"claude","event":"PreToolUse","matcher":"Bash","handlers":[{"type":"command","command":"./bin/check"}]}`))
	missingRR := httptest.NewRecorder()
	s.handler.ServeHTTP(missingRR, missingReq)
	if missingRR.Code != http.StatusNotFound {
		t.Fatalf("expected 404 from update missing hook, got %d: %s", missingRR.Code, missingRR.Body.String())
	}
}

func TestManagedHooksCreateAndUpdateRequireFieldsAndStrictJSON(t *testing.T) {
	s, _, _, _ := newManagedProjectServer(t, "claude")
	hookID := canonicalManagedHookID(t, "claude", "PreToolUse", "Bash")

	for name, body := range map[string]string{
		"missing tool":     `{"event":"PreToolUse","matcher":"Bash","handlers":[{"type":"command","command":"./bin/check"}]}`,
		"missing event":    `{"tool":"claude","matcher":"Bash","handlers":[{"type":"command","command":"./bin/check"}]}`,
		"missing matcher":  `{"tool":"claude","event":"PreToolUse","handlers":[{"type":"command","command":"./bin/check"}]}`,
		"missing handlers": `{"tool":"claude","event":"PreToolUse","matcher":"Bash"}`,
		"unknown field":    `{"tool":"claude","event":"PreToolUse","matcher":"Bash","handlers":[{"type":"command","command":"./bin/check"}],"extra":true}`,
		"trailing json":    `{"tool":"claude","event":"PreToolUse","matcher":"Bash","handlers":[{"type":"command","command":"./bin/check"}]}{"extra":true}`,
	} {
		t.Run("create "+name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/managed/hooks", strings.NewReader(body))
			rr := httptest.NewRecorder()
			s.handler.ServeHTTP(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 from create, got %d: %s", rr.Code, rr.Body.String())
			}
		})
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/managed/hooks", strings.NewReader(`{"tool":"claude","event":"PreToolUse","matcher":"Bash","handlers":[{"type":"command","command":"./bin/check"}]}`))
	createRR := httptest.NewRecorder()
	s.handler.ServeHTTP(createRR, createReq)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("expected 201 from create, got %d: %s", createRR.Code, createRR.Body.String())
	}

	for name, body := range map[string]string{
		"missing handlers": `{"tool":"claude","event":"PreToolUse","matcher":"Bash"}`,
		"unknown field":    `{"tool":"claude","event":"PreToolUse","matcher":"Bash","handlers":[{"type":"command","command":"./bin/check"}],"extra":true}`,
		"trailing json":    `{"tool":"claude","event":"PreToolUse","matcher":"Bash","handlers":[{"type":"command","command":"./bin/check"}]}{"extra":true}`,
	} {
		t.Run("update "+name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/api/managed/hooks/"+hookID, strings.NewReader(body))
			rr := httptest.NewRecorder()
			s.handler.ServeHTTP(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 from update, got %d: %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestManagedHooksCreateAndUpdateRejectStoreValidationErrorsAsBadRequest(t *testing.T) {
	s, _, _, _ := newManagedProjectServer(t, "claude")
	hookID := canonicalManagedHookID(t, "claude", "PreToolUse", "Bash")

	for name, body := range map[string]string{
		"unsupported tool":         `{"tool":"cursor","event":"PreToolUse","matcher":"Bash","handlers":[{"type":"command","command":"./bin/check"}]}`,
		"missing nested command":   `{"tool":"claude","event":"PreToolUse","matcher":"Bash","handlers":[{"type":"command"}]}`,
		"missing nested prompt":    `{"tool":"claude","event":"PreToolUse","matcher":"Bash","handlers":[{"type":"prompt"}]}`,
		"missing nested webhook":   `{"tool":"claude","event":"PreToolUse","matcher":"Bash","handlers":[{"type":"http"}]}`,
		"unsupported handler type": `{"tool":"claude","event":"PreToolUse","matcher":"Bash","handlers":[{"type":"unknown","command":"./bin/check"}]}`,
	} {
		t.Run("create "+name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/managed/hooks", strings.NewReader(body))
			rr := httptest.NewRecorder()
			s.handler.ServeHTTP(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 from create, got %d: %s", rr.Code, rr.Body.String())
			}
		})
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/managed/hooks", strings.NewReader(`{"tool":"claude","event":"PreToolUse","matcher":"Bash","handlers":[{"type":"command","command":"./bin/check"}]}`))
	createRR := httptest.NewRecorder()
	s.handler.ServeHTTP(createRR, createReq)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("expected 201 from create, got %d: %s", createRR.Code, createRR.Body.String())
	}

	for name, body := range map[string]string{
		"unsupported tool":         `{"tool":"cursor","event":"PreToolUse","matcher":"Bash","handlers":[{"type":"command","command":"./bin/check"}]}`,
		"missing nested command":   `{"tool":"claude","event":"PreToolUse","matcher":"Bash","handlers":[{"type":"command"}]}`,
		"missing nested prompt":    `{"tool":"claude","event":"PreToolUse","matcher":"Bash","handlers":[{"type":"prompt"}]}`,
		"missing nested webhook":   `{"tool":"claude","event":"PreToolUse","matcher":"Bash","handlers":[{"type":"http"}]}`,
		"unsupported handler type": `{"tool":"claude","event":"PreToolUse","matcher":"Bash","handlers":[{"type":"unknown","command":"./bin/check"}]}`,
	} {
		t.Run("update "+name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/api/managed/hooks/"+hookID, strings.NewReader(body))
			rr := httptest.NewRecorder()
			s.handler.ServeHTTP(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 from update, got %d: %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestManagedHooksDiffUsesCanonicalProjectRootsForAliasAndSharedTargets(t *testing.T) {
	t.Run("claude alias target", func(t *testing.T) {
		s, projectRoot, _, _ := newManagedProjectServer(t, "claude-code")

		targetPath := filepath.Join(projectRoot, ".claude", "skills")
		if err := os.MkdirAll(targetPath, 0755); err != nil {
			t.Fatalf("failed to create target dir: %v", err)
		}
		s.cfg.Targets["claude-code"] = config.TargetConfig{Path: targetPath}

		createReq := httptest.NewRequest(http.MethodPost, "/api/managed/hooks", strings.NewReader(`{"tool":"claude","event":"PreToolUse","matcher":"Bash","handlers":[{"type":"command","command":"./bin/check"}]}`))
		createRR := httptest.NewRecorder()
		s.handler.ServeHTTP(createRR, createReq)
		if createRR.Code != http.StatusCreated {
			t.Fatalf("expected 201 from create, got %d: %s", createRR.Code, createRR.Body.String())
		}

		diffReq := httptest.NewRequest(http.MethodGet, "/api/managed/hooks/diff", nil)
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
		if len(diffResp.Diffs[0].Files) != 1 || diffResp.Diffs[0].Files[0].Path != filepath.Join(projectRoot, ".claude", "settings.json") {
			t.Fatalf("diff files = %#v, want canonical claude settings path under project root", diffResp.Diffs[0].Files)
		}
	})

	t.Run("codex shared target", func(t *testing.T) {
		s, projectRoot, _, _ := newManagedProjectServer(t, "universal")

		targetPath := filepath.Join(projectRoot, ".agents", "skills")
		if err := os.MkdirAll(targetPath, 0755); err != nil {
			t.Fatalf("failed to create target dir: %v", err)
		}
		s.cfg.Targets["universal"] = config.TargetConfig{Path: targetPath}

		createReq := httptest.NewRequest(http.MethodPost, "/api/managed/hooks", strings.NewReader(`{"tool":"codex","event":"PreToolUse","matcher":"Bash","handlers":[{"type":"command","command":"./bin/check","timeoutSec":30}]}`))
		createRR := httptest.NewRecorder()
		s.handler.ServeHTTP(createRR, createReq)
		if createRR.Code != http.StatusCreated {
			t.Fatalf("expected 201 from create, got %d: %s", createRR.Code, createRR.Body.String())
		}

		var createResp struct {
			Previews []struct {
				Target string `json:"target"`
				Files  []struct {
					Path string `json:"path"`
				} `json:"files"`
				Warnings []string `json:"warnings"`
			} `json:"previews"`
		}
		if err := json.Unmarshal(createRR.Body.Bytes(), &createResp); err != nil {
			t.Fatalf("failed to decode create response: %v", err)
		}
		if len(createResp.Previews) != 1 || createResp.Previews[0].Target != "universal" {
			t.Fatalf("create previews = %#v, want one universal preview", createResp.Previews)
		}
		if len(createResp.Previews[0].Warnings) != 0 {
			t.Fatalf("create preview warnings = %#v, want none", createResp.Previews[0].Warnings)
		}
		if len(createResp.Previews[0].Files) != 2 {
			t.Fatalf("create preview files = %#v, want codex config + hooks outputs", createResp.Previews[0].Files)
		}
		wantPaths := map[string]bool{
			filepath.Join(projectRoot, ".codex", "config.toml"): false,
			filepath.Join(projectRoot, ".codex", "hooks.json"):  false,
		}
		for _, file := range createResp.Previews[0].Files {
			_, ok := wantPaths[file.Path]
			if !ok {
				t.Fatalf("unexpected codex preview path %q in %#v", file.Path, createResp.Previews[0].Files)
			}
			wantPaths[file.Path] = true
		}
		for path, seen := range wantPaths {
			if !seen {
				t.Fatalf("missing codex preview path %q in %#v", path, createResp.Previews[0].Files)
			}
		}
	})
}

func TestManagedHooksUnsupportedTargetPreviewWarning(t *testing.T) {
	s, _, _, _ := newManagedProjectServer(t, "cursor")

	createReq := httptest.NewRequest(http.MethodPost, "/api/managed/hooks", strings.NewReader(`{"tool":"claude","event":"PreToolUse","matcher":"Bash","handlers":[{"type":"command","command":"./bin/check"}]}`))
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
}

func TestManagedHooksCreateRollsBackWhenPreviewCompilationFails(t *testing.T) {
	s, projectRoot, _, _ := newManagedProjectServer(t, "claude")

	previewConfigPath := filepath.Join(projectRoot, ".claude", "settings.json")
	if err := os.MkdirAll(previewConfigPath, 0755); err != nil {
		t.Fatalf("failed to create preview failure path: %v", err)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/managed/hooks", strings.NewReader(`{"tool":"claude","event":"PreToolUse","matcher":"Bash","handlers":[{"type":"command","command":"./bin/check"}]}`))
	createRR := httptest.NewRecorder()
	s.handler.ServeHTTP(createRR, createReq)
	if createRR.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 from create when preview compilation fails, got %d: %s", createRR.Code, createRR.Body.String())
	}

	records, err := managedhooks.NewStore(projectRoot).List()
	if err != nil {
		t.Fatalf("failed to list managed hooks after create failure: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("managed hook create was not rolled back; got records %#v", records)
	}
}

func TestManagedHooksUpdateRestoresPreviousRecordWhenPreviewCompilationFails(t *testing.T) {
	s, projectRoot, _, _ := newManagedProjectServer(t, "claude")

	hookID := canonicalManagedHookID(t, "claude", "PreToolUse", "Bash")
	createReq := httptest.NewRequest(http.MethodPost, "/api/managed/hooks", strings.NewReader(`{"tool":"claude","event":"PreToolUse","matcher":"Bash","handlers":[{"type":"command","command":"./bin/original"}]}`))
	createRR := httptest.NewRecorder()
	s.handler.ServeHTTP(createRR, createReq)
	if createRR.Code != http.StatusCreated {
		t.Fatalf("expected 201 from create, got %d: %s", createRR.Code, createRR.Body.String())
	}

	previewConfigPath := filepath.Join(projectRoot, ".claude", "settings.json")
	if err := os.MkdirAll(previewConfigPath, 0755); err != nil {
		t.Fatalf("failed to create preview failure path: %v", err)
	}

	updateReq := httptest.NewRequest(http.MethodPut, "/api/managed/hooks/"+hookID, strings.NewReader(`{"tool":"claude","event":"PreToolUse","matcher":"Write","handlers":[{"type":"command","command":"./bin/updated"}]}`))
	updateRR := httptest.NewRecorder()
	s.handler.ServeHTTP(updateRR, updateReq)
	if updateRR.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 from update when preview compilation fails, got %d: %s", updateRR.Code, updateRR.Body.String())
	}

	renamedID, err := managedhooks.CanonicalRelativePath("claude", "PreToolUse", "Write")
	if err != nil {
		t.Fatalf("failed to derive renamed hook id: %v", err)
	}
	record, err := managedhooks.NewStore(projectRoot).Get(hookID)
	if err != nil {
		t.Fatalf("failed to load hook after failed update: %v", err)
	}
	if len(record.Handlers) != 1 || record.Handlers[0].Command != "./bin/original" {
		t.Fatalf("managed hook update was not rolled back; got handlers %#v", record.Handlers)
	}
	if _, err := managedhooks.NewStore(projectRoot).Get(renamedID); err == nil {
		t.Fatalf("expected renamed hook %s to be rolled back", renamedID)
	}
}

func TestManagedHooksRejectUnsupportedFamilyBeforeStoreWrite(t *testing.T) {
	s, _, _, _ := newManagedProjectServer(t, "claude")

	req := httptest.NewRequest(http.MethodPost, "/api/managed/hooks", strings.NewReader(`{"tool":"pi","event":"PreToolUse","matcher":"Read","handlers":[{"type":"command","command":"./bin/check"}]}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 from pi hook create, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "does not support managed hooks") {
		t.Fatalf("pi hook rejection body = %s, want managed family support error", rr.Body.String())
	}
}
