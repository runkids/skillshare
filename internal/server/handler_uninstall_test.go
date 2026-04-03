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

	"skillshare/internal/config"
)

func TestHandleBatchUninstall_ProjectMode_GitignorePath(t *testing.T) {
	s, _ := newTestServer(t)

	// Simulate project mode
	projectRoot := t.TempDir()
	projectSkillsDir := filepath.Join(projectRoot, ".skillshare", "skills")
	os.MkdirAll(projectSkillsDir, 0755)
	s.projectRoot = projectRoot
	s.cfg.Source = projectSkillsDir

	// Create tracked repo
	addTrackedRepo(t, projectSkillsDir, "_team-skills")

	// Write gitignore the way project install does
	gitignoreDir := filepath.Join(projectRoot, ".skillshare")
	gitignorePath := filepath.Join(gitignoreDir, ".gitignore")
	os.WriteFile(gitignorePath, []byte(
		"# BEGIN SKILLSHARE MANAGED - DO NOT EDIT\nskills/_team-skills/\n# END SKILLSHARE MANAGED\n",
	), 0644)

	s.registry = &config.Registry{
		Skills: []config.SkillEntry{
			{Name: "_team-skills", Tracked: true},
		},
	}

	body := batchUninstallRequest{Names: []string{"_team-skills"}, Force: true}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/uninstall", bytes.NewReader(b))
	rr := httptest.NewRecorder()
	s.handleBatchUninstall(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("failed to read gitignore: %v", err)
	}
	if strings.Contains(string(data), "skills/_team-skills/") {
		t.Fatal("expected skills/_team-skills/ to be removed from .skillshare/.gitignore")
	}
}

func TestHandleBatchUninstall_GlobalMode_GitignorePath(t *testing.T) {
	s, src := newTestServer(t)

	// Create tracked repo in global source
	addTrackedRepo(t, src, "_team-skills")

	// Write gitignore in source dir
	gitignorePath := filepath.Join(src, ".gitignore")
	os.WriteFile(gitignorePath, []byte(
		"# BEGIN SKILLSHARE MANAGED - DO NOT EDIT\n_team-skills/\n# END SKILLSHARE MANAGED\n",
	), 0644)

	s.registry = &config.Registry{
		Skills: []config.SkillEntry{
			{Name: "_team-skills", Tracked: true},
		},
	}

	body := batchUninstallRequest{Names: []string{"_team-skills"}, Force: true}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/uninstall", bytes.NewReader(b))
	rr := httptest.NewRecorder()
	s.handleBatchUninstall(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("failed to read gitignore: %v", err)
	}
	if strings.Contains(string(data), "_team-skills/") {
		t.Fatal("expected _team-skills/ to be removed from global source .gitignore")
	}
}
