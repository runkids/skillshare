package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/utils"
)

// addSkillWithTargets creates a skill with an existing targets field in frontmatter.
func addSkillWithTargets(t *testing.T, sourceDir, name string, targets []string) {
	t.Helper()
	skillDir := filepath.Join(sourceDir, name)
	os.MkdirAll(skillDir, 0755)
	targetYAML := ""
	if len(targets) > 0 {
		targetYAML = "\nmetadata:\n  targets:\n"
		for _, tgt := range targets {
			targetYAML += "    - " + tgt + "\n"
		}
	}
	content := "---\nname: " + name + targetYAML + "\n---\n# " + name
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644)
}

// addSkillNested creates a skill in a subdirectory (e.g. "folder/skill-name").
func addSkillNested(t *testing.T, sourceDir, relPath string) {
	t.Helper()
	skillDir := filepath.Join(sourceDir, filepath.FromSlash(relPath))
	os.MkdirAll(skillDir, 0755)
	name := filepath.Base(relPath)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: "+name+"\n---\n# "+name), 0644)
}

// --- Batch endpoint tests ---

func TestHandleBatchSetTargets_SetSingleTarget(t *testing.T) {
	s, src := newTestServer(t)
	addSkillNested(t, src, "frontend/skill-a")
	addSkillNested(t, src, "frontend/skill-b")

	body := `{"folder":"frontend","target":"claude"}`
	req := httptest.NewRequest(http.MethodPost, "/api/skills/batch/targets", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp batchSetTargetsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Updated != 2 {
		t.Errorf("expected updated=2, got %d", resp.Updated)
	}
	if resp.Skipped != 0 {
		t.Errorf("expected skipped=0, got %d", resp.Skipped)
	}
	if len(resp.Errors) != 0 {
		t.Errorf("expected no errors, got %v", resp.Errors)
	}

	// Verify SKILL.md was modified for both skills
	for _, name := range []string{"frontend/skill-a", "frontend/skill-b"} {
		skillMD := filepath.Join(src, filepath.FromSlash(name), "SKILL.md")
		targets := utils.ParseFrontmatterList(skillMD, "targets")
		if len(targets) != 1 || targets[0] != "claude" {
			t.Errorf("skill %s: expected targets=[claude], got %v", name, targets)
		}
	}
}

func TestHandleBatchSetTargets_RemoveTargets(t *testing.T) {
	s, src := newTestServer(t)
	addSkillWithTargets(t, src, "skill-a", []string{"claude", "cursor"})
	addSkillWithTargets(t, src, "skill-b", []string{"claude"})

	// Set target="" to remove targets (root-level folder)
	body := `{"folder":"","target":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/skills/batch/targets", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp batchSetTargetsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Updated != 2 {
		t.Errorf("expected updated=2, got %d", resp.Updated)
	}

	// Verify targets were removed
	for _, name := range []string{"skill-a", "skill-b"} {
		skillMD := filepath.Join(src, name, "SKILL.md")
		targets := utils.ParseFrontmatterList(skillMD, "targets")
		if len(targets) != 0 {
			t.Errorf("skill %s: expected no targets after removal, got %v", name, targets)
		}
	}
}

func TestHandleBatchSetTargets_PathTraversal(t *testing.T) {
	s, _ := newTestServer(t)

	body := `{"folder":"../../../etc","target":"claude"}`
	req := httptest.NewRequest(http.MethodPost, "/api/skills/batch/targets", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for path traversal, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleBatchSetTargets_EmptyFolder_RootOnly(t *testing.T) {
	s, src := newTestServer(t)
	// Root-level skills
	addSkill(t, src, "root-skill-a")
	addSkill(t, src, "root-skill-b")
	// Nested skill — should NOT be touched
	addSkillNested(t, src, "nested-folder/deep-skill")

	body := `{"folder":"","target":"cursor"}`
	req := httptest.NewRequest(http.MethodPost, "/api/skills/batch/targets", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp batchSetTargetsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Updated != 2 {
		t.Errorf("expected updated=2 (root only), got %d", resp.Updated)
	}

	// Nested skill must remain untouched
	nestedMD := filepath.Join(src, "nested-folder", "deep-skill", "SKILL.md")
	targets := utils.ParseFrontmatterList(nestedMD, "targets")
	if len(targets) != 0 {
		t.Errorf("nested skill should not be modified, but got targets %v", targets)
	}
}

// --- Single skill endpoint tests ---

func TestHandleSetSkillTargets_SetTarget(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "my-skill")

	body := `{"target":"claude"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/skills/my-skill/targets", bytes.NewBufferString(body))
	req.SetPathValue("name", "my-skill")
	rr := httptest.NewRecorder()
	s.handleSetSkillTargets(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["success"] != true {
		t.Errorf("expected success=true, got %v", resp["success"])
	}

	// Verify SKILL.md was updated
	skillMD := filepath.Join(src, "my-skill", "SKILL.md")
	targets := utils.ParseFrontmatterList(skillMD, "targets")
	if len(targets) != 1 || targets[0] != "claude" {
		t.Errorf("expected targets=[claude], got %v", targets)
	}
}

func TestHandleSetSkillTargets_RemoveTarget(t *testing.T) {
	s, src := newTestServer(t)
	addSkillWithTargets(t, src, "my-skill", []string{"claude", "cursor"})

	body := `{"target":""}`
	req := httptest.NewRequest(http.MethodPatch, "/api/skills/my-skill/targets", bytes.NewBufferString(body))
	req.SetPathValue("name", "my-skill")
	rr := httptest.NewRecorder()
	s.handleSetSkillTargets(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify targets field was removed
	skillMD := filepath.Join(src, "my-skill", "SKILL.md")
	targets := utils.ParseFrontmatterList(skillMD, "targets")
	if len(targets) != 0 {
		t.Errorf("expected no targets after removal, got %v", targets)
	}
}

func TestHandleSetSkillTargets_NotFound(t *testing.T) {
	s, _ := newTestServer(t)

	body := `{"target":"claude"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/skills/nonexistent/targets", bytes.NewBufferString(body))
	req.SetPathValue("name", "nonexistent")
	rr := httptest.NewRecorder()
	s.handleSetSkillTargets(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}
