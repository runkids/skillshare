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

	"skillshare/internal/skill"
)

func TestHandleGetTemplates(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/skills/templates", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Patterns   []skill.Pattern  `json:"patterns"`
		Categories []skill.Category `json:"categories"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Patterns) != 6 {
		t.Errorf("expected 6 patterns, got %d", len(resp.Patterns))
	}
	if len(resp.Categories) != 9 {
		t.Errorf("expected 9 categories, got %d", len(resp.Categories))
	}

	// Verify first pattern has expected scaffold dirs
	if resp.Patterns[0].Name != "tool-wrapper" {
		t.Errorf("expected first pattern 'tool-wrapper', got %q", resp.Patterns[0].Name)
	}
	if len(resp.Patterns[0].ScaffoldDirs) != 1 || resp.Patterns[0].ScaffoldDirs[0] != "references" {
		t.Errorf("expected tool-wrapper scaffoldDirs [references], got %v", resp.Patterns[0].ScaffoldDirs)
	}
}

func TestHandleCreateSkill_Success(t *testing.T) {
	s, src := newTestServer(t)

	body := `{"name":"my-tool","pattern":"tool-wrapper","category":"library","scaffoldDirs":["references"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/skills", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Skill struct {
			Name       string `json:"name"`
			FlatName   string `json:"flatName"`
			RelPath    string `json:"relPath"`
			SourcePath string `json:"sourcePath"`
		} `json:"skill"`
		CreatedFiles []string `json:"createdFiles"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Skill.Name != "my-tool" {
		t.Errorf("expected name 'my-tool', got %q", resp.Skill.Name)
	}
	if resp.Skill.FlatName != "my-tool" {
		t.Errorf("expected flatName 'my-tool', got %q", resp.Skill.FlatName)
	}

	// Verify SKILL.md exists on disk
	skillMD := filepath.Join(src, "my-tool", "SKILL.md")
	data, err := os.ReadFile(skillMD)
	if err != nil {
		t.Fatalf("SKILL.md not created: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "name: my-tool") {
		t.Error("SKILL.md missing 'name: my-tool'")
	}
	if !strings.Contains(content, "pattern: tool-wrapper") {
		t.Error("SKILL.md missing 'pattern: tool-wrapper'")
	}
	if !strings.Contains(content, "category: library") {
		t.Error("SKILL.md missing 'category: library'")
	}

	// Verify scaffold dir and .gitkeep
	gitkeep := filepath.Join(src, "my-tool", "references", ".gitkeep")
	if _, err := os.Stat(gitkeep); err != nil {
		t.Errorf("references/.gitkeep not created: %v", err)
	}

	// Verify createdFiles list
	if len(resp.CreatedFiles) != 2 {
		t.Fatalf("expected 2 createdFiles, got %d: %v", len(resp.CreatedFiles), resp.CreatedFiles)
	}
	if resp.CreatedFiles[0] != "SKILL.md" {
		t.Errorf("expected createdFiles[0] 'SKILL.md', got %q", resp.CreatedFiles[0])
	}
	if resp.CreatedFiles[1] != "references/.gitkeep" {
		t.Errorf("expected createdFiles[1] 'references/.gitkeep', got %q", resp.CreatedFiles[1])
	}
}

func TestHandleCreateSkill_EmptyScaffoldDirs(t *testing.T) {
	s, src := newTestServer(t)

	body := `{"name":"simple-skill","pattern":"tool-wrapper","category":"library","scaffoldDirs":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/skills", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		CreatedFiles []string `json:"createdFiles"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Only SKILL.md, no scaffold dirs
	if len(resp.CreatedFiles) != 1 {
		t.Errorf("expected 1 createdFile, got %d: %v", len(resp.CreatedFiles), resp.CreatedFiles)
	}

	// Verify no scaffold directories were created
	entries, _ := os.ReadDir(filepath.Join(src, "simple-skill"))
	for _, e := range entries {
		if e.IsDir() {
			t.Errorf("unexpected directory created: %s", e.Name())
		}
	}
}

func TestHandleCreateSkill_InvalidName(t *testing.T) {
	s, _ := newTestServer(t)

	tests := []struct {
		name string
		body string
	}{
		{"uppercase", `{"name":"MySkill","pattern":"none","category":"","scaffoldDirs":[]}`},
		{"starts with number", `{"name":"1skill","pattern":"none","category":"","scaffoldDirs":[]}`},
		{"contains space", `{"name":"my skill","pattern":"none","category":"","scaffoldDirs":[]}`},
		{"contains dot", `{"name":"my.skill","pattern":"none","category":"","scaffoldDirs":[]}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/skills", bytes.NewBufferString(tc.body))
			rr := httptest.NewRecorder()
			s.handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
			}

			var resp map[string]string
			json.Unmarshal(rr.Body.Bytes(), &resp)
			if !strings.Contains(resp["error"], "invalid skill name") {
				t.Errorf("expected 'invalid skill name' error, got %q", resp["error"])
			}
		})
	}
}

func TestHandleCreateSkill_Duplicate(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "existing-skill")

	body := `{"name":"existing-skill","pattern":"none","category":"","scaffoldDirs":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/skills", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if !strings.Contains(resp["error"], "already exists") {
		t.Errorf("expected 'already exists' error, got %q", resp["error"])
	}
}

func TestHandleCreateSkill_InvalidScaffoldDir(t *testing.T) {
	s, _ := newTestServer(t)

	// tool-wrapper only allows "references", not "assets"
	body := `{"name":"bad-dirs","pattern":"tool-wrapper","category":"library","scaffoldDirs":["assets"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/skills", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if !strings.Contains(resp["error"], "not valid for pattern") {
		t.Errorf("expected 'not valid for pattern' error, got %q", resp["error"])
	}
}

func TestHandleCreateSkill_NonePattern(t *testing.T) {
	s, src := newTestServer(t)

	body := `{"name":"plain-skill","pattern":"none","category":"","scaffoldDirs":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/skills", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify SKILL.md has no "pattern:" field in frontmatter
	skillMD := filepath.Join(src, "plain-skill", "SKILL.md")
	data, err := os.ReadFile(skillMD)
	if err != nil {
		t.Fatalf("SKILL.md not created: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "pattern:") {
		t.Error("SKILL.md should not contain 'pattern:' for none pattern")
	}
	if strings.Contains(content, "category:") {
		t.Error("SKILL.md should not contain 'category:' for none pattern")
	}

	// Should still have name in frontmatter
	if !strings.Contains(content, "name: plain-skill") {
		t.Error("SKILL.md missing 'name: plain-skill'")
	}
}
