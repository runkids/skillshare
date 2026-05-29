package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestHandleGitCommit_NoRemote_CreatesLocalCommit(t *testing.T) {
	s, src := newTestServer(t)
	initServerGitRepo(t, src)
	addSkill(t, src, "local-skill")

	req := httptest.NewRequest(http.MethodPost, "/api/git/commit", strings.NewReader(`{"message":"local checkpoint"}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp pushResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response: %+v", resp)
	}

	message := testutil.RunGit(t, src, "log", "-1", "--pretty=%s")
	if message != "local checkpoint" {
		t.Fatalf("commit message = %q, want %q", message, "local checkpoint")
	}

	status := testutil.RunGit(t, src, "status", "--porcelain")
	if status != "" {
		t.Fatalf("expected clean working tree, got %q", status)
	}
}

func TestHandleGitCommit_DryRun_DoesNotCreateCommit(t *testing.T) {
	s, src := newTestServer(t)
	initServerGitRepo(t, src)
	addSkill(t, src, "dry-run-skill")

	req := httptest.NewRequest(http.MethodPost, "/api/git/commit", strings.NewReader(`{"message":"dry run checkpoint","dryRun":true}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	count := testutil.RunGit(t, src, "rev-list", "--count", "HEAD")
	if count != "1" {
		t.Fatalf("commit count = %q, want 1", count)
	}
}

func TestHandleGitCommit_NoChanges(t *testing.T) {
	s, src := newTestServer(t)
	initServerGitRepo(t, src)

	req := httptest.NewRequest(http.MethodPost, "/api/git/commit", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp pushResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Message != "nothing to commit (working tree clean)" {
		t.Fatalf("message = %q, want nothing to commit", resp.Message)
	}
}

func initServerGitRepo(t *testing.T, dir string) {
	t.Helper()

	testutil.RunGit(t, dir, "init")
	testutil.ConfigureGitUser(t, dir)
	testutil.RunGit(t, dir, "commit", "--allow-empty", "-m", "initial")
}

// POST /api/git/root with a non-default scope initializes a repo at that scope
// directory and persists git_root to config.
func TestHandleSetGitRoot_SwitchToAgents_InitsAndPersists(t *testing.T) {
	s, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/git/root", strings.NewReader(`{"scope":"agents"}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["scope"] != "agents" {
		t.Fatalf("scope = %v, want agents", resp["scope"])
	}
	dir, _ := resp["gitRoot"].(string)
	if dir == "" {
		t.Fatalf("missing gitRoot in response: %+v", resp)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Errorf("expected .git initialized at agents scope %s: %v", dir, err)
	}
	if s.cfg.GitRoot != "agents" {
		t.Errorf("cfg.GitRoot = %q, want agents (persisted)", s.cfg.GitRoot)
	}
}

// POST /api/git/root with a remoteURL initializes the scope repo and wires the
// "origin" remote on it in one request (UI parity with the CLI's
// `init --git-root <scope> --remote <url>`).
func TestHandleSetGitRoot_WithRemote_WiresOrigin(t *testing.T) {
	s, _ := newTestServer(t)

	const remote = "https://example.com/skills.git"
	body := `{"scope":"agents","remoteURL":"` + remote + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/git/root", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	dir, _ := resp["gitRoot"].(string)
	if dir == "" {
		t.Fatalf("missing gitRoot in response: %+v", resp)
	}

	got := testutil.RunGit(t, dir, "remote", "get-url", "origin")
	if got != remote {
		t.Errorf("origin = %q, want %q", got, remote)
	}
}

// POST /api/git/root rejects unknown or empty scopes with 400.
func TestHandleSetGitRoot_InvalidScope_Rejected(t *testing.T) {
	s, _ := newTestServer(t)

	for _, scope := range []string{`"bogus"`, `""`} {
		req := httptest.NewRequest(http.MethodPost, "/api/git/root", strings.NewReader(`{"scope":`+scope+`}`))
		rr := httptest.NewRecorder()
		s.handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("scope %s: expected 400, got %d: %s", scope, rr.Code, rr.Body.String())
		}
	}
}

// git_root is global-only; project mode must reject the request.
func TestHandleSetGitRoot_ProjectMode_Rejected(t *testing.T) {
	s, _ := newProjectTargetServer(t, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/git/root", strings.NewReader(`{"scope":"root"}`))
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 in project mode, got %d: %s", rr.Code, rr.Body.String())
	}
}

// GET /api/git/status reports a scope mismatch when the configured root has no
// repo but a sibling scope directory does.
func TestHandleGitStatus_ScopeMismatch(t *testing.T) {
	s, _ := newTestServer(t)

	// git_root defaults to skills (the source). Plant a stray repo at the agents
	// scope dir so the configured root lacks .git but agents has one.
	agentsDir := s.cfg.EffectiveAgentsSource()
	if err := os.MkdirAll(filepath.Join(agentsDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/git/status", nil)
	rr := httptest.NewRecorder()
	s.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp gitStatusResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.ScopeMismatch {
		t.Fatalf("expected scopeMismatch=true, got: %+v", resp)
	}
	if resp.MismatchScope != "agents" {
		t.Errorf("mismatchScope = %q, want agents", resp.MismatchScope)
	}
	if resp.MismatchDir != agentsDir {
		t.Errorf("mismatchDir = %q, want %q", resp.MismatchDir, agentsDir)
	}
}
