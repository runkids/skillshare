package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func postBatchToggle(t *testing.T, s *Server, body string) (*httptest.ResponseRecorder, batchToggleResponse) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/resources/batch/toggle", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	var resp batchToggleResponse
	if rr.Code == http.StatusOK {
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v (body=%s)", err, rr.Body.String())
		}
	}
	return rr, resp
}

func readIgnoreLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read ignore file: %v", err)
	}
	var lines []string
	for _, l := range strings.Split(string(data), "\n") {
		if s := strings.TrimSpace(l); s != "" {
			lines = append(lines, s)
		}
	}
	return lines
}

func TestHandleBatchToggle_DisableSkills(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "skill-a")
	addSkill(t, src, "skill-b")
	addSkill(t, src, "skill-c")

	rr, resp := postBatchToggle(t, s, `{"names":["skill-a","skill-b"],"kind":"skill","enable":false}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if resp.Summary.Updated != 2 || resp.Summary.Unchanged != 0 || resp.Summary.Failed != 0 {
		t.Fatalf("unexpected summary: %+v", resp.Summary)
	}

	lines := readIgnoreLines(t, filepath.Join(src, ".skillignore"))
	got := map[string]bool{}
	for _, l := range lines {
		got[l] = true
	}
	if !got["skill-a"] || !got["skill-b"] {
		t.Errorf("expected skill-a and skill-b disabled, got %v", lines)
	}
	if got["skill-c"] {
		t.Errorf("skill-c should not be disabled, got %v", lines)
	}
}

func TestHandleBatchToggle_EnableSubset(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "skill-a")
	addSkill(t, src, "skill-b")
	if err := os.WriteFile(filepath.Join(src, ".skillignore"), []byte("skill-a\nskill-b\n"), 0644); err != nil {
		t.Fatalf("seed .skillignore: %v", err)
	}

	rr, resp := postBatchToggle(t, s, `{"names":["skill-a"],"kind":"skill","enable":true}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if resp.Summary.Updated != 1 {
		t.Fatalf("expected updated=1, got %+v", resp.Summary)
	}

	lines := readIgnoreLines(t, filepath.Join(src, ".skillignore"))
	if len(lines) != 1 || lines[0] != "skill-b" {
		t.Errorf("expected only skill-b remaining disabled, got %v", lines)
	}
}

func TestHandleBatchToggle_AlreadyDisabled_Unchanged(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "skill-a")
	if err := os.WriteFile(filepath.Join(src, ".skillignore"), []byte("skill-a\n"), 0644); err != nil {
		t.Fatalf("seed .skillignore: %v", err)
	}

	_, resp := postBatchToggle(t, s, `{"names":["skill-a"],"kind":"skill","enable":false}`)
	if resp.Summary.Unchanged != 1 || resp.Summary.Updated != 0 {
		t.Fatalf("expected unchanged=1, got %+v", resp.Summary)
	}
}

func TestHandleBatchToggle_UnknownName_Failed(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "skill-a")

	_, resp := postBatchToggle(t, s, `{"names":["skill-a","ghost"],"kind":"skill","enable":false}`)
	if resp.Summary.Updated != 1 || resp.Summary.Failed != 1 {
		t.Fatalf("expected updated=1 failed=1, got %+v", resp.Summary)
	}
	var ghost *batchToggleItemResult
	for i := range resp.Results {
		if resp.Results[i].Name == "ghost" {
			ghost = &resp.Results[i]
		}
	}
	if ghost == nil || ghost.Success || ghost.Error == "" {
		t.Fatalf("expected ghost to fail with error, got %+v", ghost)
	}
}

func TestHandleBatchToggle_Agents(t *testing.T) {
	s, _ := newTestServer(t)
	agentsDir := s.agentsSource()
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatalf("create agents dir: %v", err)
	}
	addAgent(t, agentsDir, "demo/reviewer.md")
	addAgent(t, agentsDir, "demo/planner.md")

	rr, resp := postBatchToggle(t, s, `{"names":["demo/reviewer.md","demo/planner.md"],"kind":"agent","enable":false}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if resp.Summary.Updated != 2 {
		t.Fatalf("expected updated=2, got %+v", resp.Summary)
	}

	lines := readIgnoreLines(t, filepath.Join(agentsDir, ".agentignore"))
	got := map[string]bool{}
	for _, l := range lines {
		got[l] = true
	}
	if !got["demo/reviewer.md"] || !got["demo/planner.md"] {
		t.Errorf("expected both agents disabled, got %v", lines)
	}
}

func TestHandleBatchToggle_InvalidKind(t *testing.T) {
	s, _ := newTestServer(t)
	rr, _ := postBatchToggle(t, s, `{"names":["x"],"kind":"bogus","enable":false}`)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid kind, got %d", rr.Code)
	}
}
