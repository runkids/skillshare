package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	ssync "skillshare/internal/sync"
)

func writeSkillMarkdown(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
}

func findDiscoveredSkillByRelPath(relPath string, skills []ssync.DiscoveredSkill) (ssync.DiscoveredSkill, bool) {
	for _, skill := range skills {
		if skill.RelPath == relPath {
			return skill, true
		}
	}
	return ssync.DiscoveredSkill{}, false
}

func TestHandleInstall_InvalidatesDiscoveryCache(t *testing.T) {
	s, sourceDir := newTestServer(t)

	initial, err := s.cache.Discover(sourceDir)
	if err != nil {
		t.Fatalf("prime discover: %v", err)
	}
	if len(initial) != 0 {
		t.Fatalf("expected empty source, got %d skills", len(initial))
	}

	localSource := filepath.Join(t.TempDir(), "install-source")
	writeSkillMarkdown(t, localSource, "---\nname: cache-install-skill\n---\n# install")

	payload, _ := json.Marshal(map[string]any{
		"source": localSource,
		"name":   "cache-install-skill",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/install", bytes.NewReader(payload))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	after, err := s.cache.Discover(sourceDir)
	if err != nil {
		t.Fatalf("discover after install: %v", err)
	}
	if len(after) != 1 || after[0].RelPath != "cache-install-skill" {
		t.Fatalf("expected discovered skill cache-install-skill after install, got %#v", after)
	}
}

func TestHandleCollect_InvalidatesDiscoveryCache(t *testing.T) {
	targetPath := filepath.Join(t.TempDir(), "claude-skills")
	s, sourceDir := newTestServerWithTargets(t, map[string]string{"claude": targetPath})

	initial, err := s.cache.Discover(sourceDir)
	if err != nil {
		t.Fatalf("prime discover: %v", err)
	}
	if len(initial) != 0 {
		t.Fatalf("expected empty source, got %d skills", len(initial))
	}

	localSkill := filepath.Join(targetPath, "cache-collected-skill")
	writeSkillMarkdown(t, localSkill, "---\nname: cache-collected-skill\n---\n# collected")

	payload, _ := json.Marshal(map[string]any{
		"skills": []map[string]string{
			{"name": "cache-collected-skill", "targetName": "claude"},
		},
		"force": true,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/collect", bytes.NewReader(payload))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	after, err := s.cache.Discover(sourceDir)
	if err != nil {
		t.Fatalf("discover after collect: %v", err)
	}
	if len(after) != 1 || after[0].RelPath != "cache-collected-skill" {
		t.Fatalf("expected discovered skill cache-collected-skill after collect, got %#v", after)
	}
}

func TestHandleUpdate_InvalidatesDiscoveryCache(t *testing.T) {
	s, sourceDir := newTestServer(t)

	localSource := filepath.Join(t.TempDir(), "update-source")
	writeSkillMarkdown(t, localSource, "---\nname: cache-update-skill\ntargets:\n  - claude\n---\n# v1")

	installPayload, _ := json.Marshal(map[string]any{
		"source": localSource,
		"name":   "cache-update-skill",
	})
	installReq := httptest.NewRequest(http.MethodPost, "/api/install", bytes.NewReader(installPayload))
	installRR := httptest.NewRecorder()
	s.handler.ServeHTTP(installRR, installReq)
	if installRR.Code != http.StatusOK {
		t.Fatalf("install failed: %d %s", installRR.Code, installRR.Body.String())
	}

	before, err := s.cache.Discover(sourceDir)
	if err != nil {
		t.Fatalf("prime discover before update: %v", err)
	}
	beforeSkill, found := findDiscoveredSkillByRelPath("cache-update-skill", before)
	if !found {
		t.Fatalf("expected cache-update-skill before update, got %#v", before)
	}
	if len(beforeSkill.Targets) != 1 || beforeSkill.Targets[0] != "claude" {
		t.Fatalf("expected targets [claude] before update, got %v", beforeSkill.Targets)
	}

	writeSkillMarkdown(t, localSource, "---\nname: cache-update-skill\ntargets:\n  - cursor\n---\n# v2")

	updatePayload, _ := json.Marshal(map[string]any{"name": "cache-update-skill"})
	updateReq := httptest.NewRequest(http.MethodPost, "/api/update", bytes.NewReader(updatePayload))
	updateRR := httptest.NewRecorder()
	s.handler.ServeHTTP(updateRR, updateReq)
	if updateRR.Code != http.StatusOK {
		t.Fatalf("update failed: %d %s", updateRR.Code, updateRR.Body.String())
	}

	var updateResp struct {
		Results []struct {
			Action string `json:"action"`
		} `json:"results"`
	}
	if err := json.Unmarshal(updateRR.Body.Bytes(), &updateResp); err != nil {
		t.Fatalf("parse update response: %v", err)
	}
	if len(updateResp.Results) == 0 || updateResp.Results[0].Action != "updated" {
		t.Fatalf("expected update action=updated, got %+v", updateResp.Results)
	}

	after, err := s.cache.Discover(sourceDir)
	if err != nil {
		t.Fatalf("discover after update: %v", err)
	}
	afterSkill, found := findDiscoveredSkillByRelPath("cache-update-skill", after)
	if !found {
		t.Fatalf("expected cache-update-skill after update, got %#v", after)
	}
	if len(afterSkill.Targets) != 1 || afterSkill.Targets[0] != "cursor" {
		t.Fatalf("expected targets [cursor] after update, got %v", afterSkill.Targets)
	}
}
