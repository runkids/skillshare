package check

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// Each agent must be compared with the branch it was installed from, using
// its own version, even when several agents share one repository.
func TestEnrichAgentResultsWithRemote_ComparesEachAgentAtItsBranch(t *testing.T) {
	repo := t.TempDir()
	git := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s %v", args, out, err)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	git("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "old")
	oldMain := git("rev-parse", "HEAD")[:7]
	git("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "new")
	main := git("rev-parse", "HEAD")[:7]
	git("switch", "-q", "-c", "dev")
	git("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "dev")
	dev := git("rev-parse", "HEAD")[:7]
	git("switch", "-q", "main")

	results := []AgentCheckResult{
		{Name: "current", RepoURL: repo, Version: main, Status: "up_to_date"},
		{Name: "stale", RepoURL: repo, Version: oldMain, Status: "up_to_date"},
		{Name: "on-dev", RepoURL: repo, Branch: "dev", Version: dev, Status: "up_to_date"},
	}
	EnrichAgentResultsWithRemote(results, nil)

	want := map[string]string{"current": "up_to_date", "stale": "update_available", "on-dev": "up_to_date"}
	for _, r := range results {
		if r.Status != want[r.Name] {
			t.Errorf("%s: status %q, want %q (%s)", r.Name, r.Status, want[r.Name], r.Message)
		}
	}
}

// Regression for #777: an agent whose source an earlier update erased cannot
// be checked, so check must say so instead of reporting it up to date.
func TestCheckAgents_SourceErasedByEarlierUpdate(t *testing.T) {
	dir := t.TempDir()
	agentFile := filepath.Join(dir, "tutor.md")
	os.WriteFile(agentFile, []byte("# Tutor"), 0644)
	hash, _ := utils.FileHashFormatted(agentFile)

	store := install.NewMetadataStore()
	store.Set("tutor", &install.MetadataEntry{
		Source:     "/agents/tutor.md",
		Kind:       install.MetadataKindAgent,
		Type:       "unknown-subdir",
		Subdir:     "agents/tutor.md",
		Version:    "abc1234",
		FileHashes: map[string]string{"tutor.md": hash},
	})
	if err := store.Save(dir); err != nil {
		t.Fatalf("save metadata: %v", err)
	}

	results := CheckAgents(dir)
	if len(results) != 1 || results[0].Status != "error" || !strings.Contains(results[0].Message, "reinstall") {
		t.Fatalf("expected an error asking to reinstall, got %+v", results)
	}
}

// A tracked agent repo records its full commit and the branch it has checked
// out; a fresh clone must read as up to date, not as an available update.
func TestCheckAgents_TrackedRepoComparedAtItsBranch(t *testing.T) {
	root := t.TempDir()
	git := func(dir string, args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s %v", args, out, err)
		}
	}
	origin := filepath.Join(root, "origin")
	os.MkdirAll(filepath.Join(origin, "agents"), 0755)
	os.WriteFile(filepath.Join(origin, "agents", "tutor.md"), []byte("# Tutor"), 0644)
	git(origin, "init", "-q", "-b", "main")
	git(origin, "add", ".")
	git(origin, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "-m", "main")
	git(origin, "switch", "-q", "-c", "dev")
	git(origin, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "dev")
	git(origin, "switch", "-q", "main")

	agentsDir := filepath.Join(root, "agents")
	os.MkdirAll(agentsDir, 0755)
	git(agentsDir, "clone", "-q", "-b", "dev", origin, "_repo")

	results := CheckAgents(agentsDir)
	EnrichAgentResultsWithRemote(results, nil)
	if len(results) != 1 || results[0].Status != "up_to_date" || results[0].Branch != "dev" {
		t.Fatalf("expected the tracked agent up to date on dev, got %+v", results)
	}
}
