package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/trash"
)

func TestHandleListTrash_Empty(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/trash", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Items     []any `json:"items"`
		TotalSize int64 `json:"totalSize"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Items) != 0 {
		t.Errorf("expected 0 trash items, got %d", len(resp.Items))
	}
	if resp.TotalSize != 0 {
		t.Errorf("expected totalSize 0, got %d", resp.TotalSize)
	}
}

func TestHandleRestoreTrash_NotFound(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/trash/nonexistent/restore", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleRestoreTrash_AgentKind(t *testing.T) {
	s, _ := newTestServer(t)

	agentsDir := s.cfg.EffectiveAgentsSource()
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("failed to create agents dir: %v", err)
	}
	agentFile := filepath.Join(agentsDir, "tutor.md")
	if err := os.WriteFile(agentFile, []byte("# Tutor agent"), 0644); err != nil {
		t.Fatalf("failed to seed agent: %v", err)
	}
	if _, err := trash.MoveAgentToTrash(agentFile, "", "tutor", s.agentTrashBase()); err != nil {
		t.Fatalf("failed to move agent to trash: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/trash/tutor/restore?kind=agent", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if _, err := os.Stat(filepath.Join(agentsDir, "tutor.md")); err != nil {
		t.Fatalf("expected restored agent file, got: %v", err)
	}
	if entry := trash.FindByName(s.agentTrashBase(), "tutor"); entry != nil {
		t.Fatalf("expected agent trash entry to be removed after restore")
	}
}

func TestHandleRestoreTrash_KeepsRemoteSource(t *testing.T) {
	s, src := newTestServer(t)
	addSkill(t, src, "remote-skill")
	s.skillsStore.Set("remote-skill", &install.MetadataEntry{Source: "github.com/acme/remote-skill"})
	if err := s.skillsStore.Save(src); err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodDelete, "/api/resources/remote-skill", nil))
	if rr.Code != http.StatusOK || s.skillsStore.Has("remote-skill") {
		t.Fatalf("uninstall should prune the entry: %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	s.handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/trash/remote-skill/restore", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if e := s.skillsStore.Get("remote-skill"); e == nil || e.Source != "github.com/acme/remote-skill" {
		t.Errorf("cached store should hold the restored source, got %+v", e)
	}
}

func TestHandleDeleteTrash_NotFound(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/trash/nonexistent", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleEmptyTrash(t *testing.T) {
	s, _ := newTestServer(t)

	addSkill(t, s.skillsSource(), "trash-skill")
	skillDir := filepath.Join(s.skillsSource(), "trash-skill")
	if _, err := trash.MoveToTrash(skillDir, "trash-skill", s.trashBase()); err != nil {
		t.Fatalf("failed to trash skill: %v", err)
	}

	agentsDir := s.cfg.EffectiveAgentsSource()
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("failed to create agents dir: %v", err)
	}
	agentFile := filepath.Join(agentsDir, "tutor.md")
	if err := os.WriteFile(agentFile, []byte("# Tutor agent"), 0644); err != nil {
		t.Fatalf("failed to seed agent: %v", err)
	}
	if _, err := trash.MoveAgentToTrash(agentFile, "", "tutor", s.agentTrashBase()); err != nil {
		t.Fatalf("failed to trash agent: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/trash/empty", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Success bool `json:"success"`
		Removed int  `json:"removed"`
	}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if !resp.Success {
		t.Error("expected success true")
	}
	if resp.Removed != 2 {
		t.Errorf("expected 2 removed, got %d", resp.Removed)
	}
	if len(trash.List(s.trashBase())) != 0 {
		t.Errorf("expected skill trash to be empty after empty")
	}
	if len(trash.List(s.agentTrashBase())) != 0 {
		t.Errorf("expected agent trash to be empty after empty")
	}
}
