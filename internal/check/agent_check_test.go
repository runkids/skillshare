package check

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/utils"
)

func TestEnrichAgentResultsWithRemote_Failure(t *testing.T) {
	missingRepo := filepath.Join(t.TempDir(), "missing.git")
	results := []AgentCheckResult{
		{Name: "current", RepoURL: missingRepo, Version: "abc", Status: "up_to_date"},
		{Name: "edited", RepoURL: missingRepo, Version: "abc", Status: "drifted"},
	}
	EnrichAgentResultsWithRemote(results, nil)
	for _, r := range results {
		if r.Status != "error" || r.Message == "" {
			t.Errorf("remote failure must be reported: %+v", r)
		}
	}
}

func TestCheckAgents_NoAgents(t *testing.T) {
	dir := t.TempDir()
	results := CheckAgents(dir)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestCheckAgents_LocalAgent(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "tutor.md"), []byte("# Tutor"), 0644)

	results := CheckAgents(dir)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Name != "tutor" {
		t.Errorf("Name = %q, want %q", results[0].Name, "tutor")
	}
	if results[0].Status != "local" {
		t.Errorf("Status = %q, want %q", results[0].Status, "local")
	}
}

func TestCheckAgents_UpToDate(t *testing.T) {
	dir := t.TempDir()
	agentFile := filepath.Join(dir, "tutor.md")
	os.WriteFile(agentFile, []byte("# Tutor agent"), 0644)

	hash, _ := utils.FileHashFormatted(agentFile)

	meta := &install.SkillMeta{
		Source:     "test",
		Kind:       "agent",
		FileHashes: map[string]string{"tutor.md": hash},
	}
	metaData, _ := json.MarshalIndent(meta, "", "  ")
	os.WriteFile(filepath.Join(dir, "tutor.skillshare-meta.json"), metaData, 0644)

	results := CheckAgents(dir)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "up_to_date" {
		t.Errorf("Status = %q, want %q", results[0].Status, "up_to_date")
	}
}

func TestCheckAgents_Drifted(t *testing.T) {
	dir := t.TempDir()
	agentFile := filepath.Join(dir, "tutor.md")
	os.WriteFile(agentFile, []byte("# Modified content"), 0644)

	meta := &install.SkillMeta{
		Source:     "test",
		Kind:       "agent",
		FileHashes: map[string]string{"tutor.md": "sha256:0000000000000000000000000000000000000000000000000000000000000000"},
	}
	metaData, _ := json.MarshalIndent(meta, "", "  ")
	os.WriteFile(filepath.Join(dir, "tutor.skillshare-meta.json"), metaData, 0644)

	results := CheckAgents(dir)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "drifted" {
		t.Errorf("Status = %q, want %q", results[0].Status, "drifted")
	}
}

func TestCheckAgents_InvalidCentralizedMetadata(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "tutor.md"), []byte("# Tutor"), 0644)
	os.WriteFile(filepath.Join(dir, install.MetadataFileName), []byte("{invalid"), 0644)

	results := CheckAgents(dir)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Name != "tutor" {
		t.Errorf("Name = %q, want %q", results[0].Name, "tutor")
	}
	if results[0].Status != "error" {
		t.Errorf("Status = %q, want %q", results[0].Status, "error")
	}
	if results[0].Message == "" {
		t.Fatal("expected error message for invalid centralized metadata")
	}
}

func TestCheckAgents_NonExistentDir(t *testing.T) {
	results := CheckAgents("/nonexistent/path")
	if results != nil {
		t.Errorf("expected nil for nonexistent dir, got %v", results)
	}
}

func TestCheckAgents_Nested(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "demo")
	os.MkdirAll(subdir, 0755)

	agentFile := filepath.Join(subdir, "tutor.md")
	os.WriteFile(agentFile, []byte("# Tutor"), 0644)

	hash, _ := utils.FileHashFormatted(agentFile)
	meta := &install.SkillMeta{
		Source:     "https://github.com/example/repo",
		Kind:       "agent",
		FileHashes: map[string]string{"tutor.md": hash},
	}
	metaData, _ := json.MarshalIndent(meta, "", "  ")
	os.WriteFile(filepath.Join(subdir, "tutor.skillshare-meta.json"), metaData, 0644)

	results := CheckAgents(dir)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Name != "demo/tutor" {
		t.Errorf("Name = %q, want %q", results[0].Name, "demo/tutor")
	}
	if results[0].Status != "up_to_date" {
		t.Errorf("Status = %q, want %q", results[0].Status, "up_to_date")
	}
	if results[0].Source != "https://github.com/example/repo" {
		t.Errorf("Source = %q, want non-empty", results[0].Source)
	}
}

func TestCheckAgents_SkipsNonMd(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "tutor.md"), []byte("# Tutor"), 0644)
	os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("key: val"), 0644)
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)

	results := CheckAgents(dir)
	if len(results) != 1 {
		t.Fatalf("expected 1 result (only .md files), got %d", len(results))
	}
}
